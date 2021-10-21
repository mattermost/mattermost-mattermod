package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/google/go-github/v33/github"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v5/mlog"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
	"sigs.k8s.io/release-utils/command"
	"sigs.k8s.io/release-utils/util"
)

const (
	gitCommand      = "git"
	rebaseMagic     = ".git/rebase-apply"
	newBranchSlug   = "automated-cherry-pick-of-"
	MMREBASE        = "rebase"
	MMMERGE         = "merge"
	MMSQUASH        = "squash"
	prTitleTemplate = "Automated cherry pick of #%d on %s"
	prBodyTemplate  = `Automated cherry pick of #%d on %s

Cherry pick of #%d on %s.

/cc  @%s

` + "```release-note\nNONE\n```\n"
)

// CherryPicker captures the cherry-pick creation logic in go
type CherryPicker struct {
	impl    cherryPickerImplementation
	options CPOptions
	state   CPState
}

// NewCherryPicker returns a cherrypicker with default opts
func NewCherryPicker() *CherryPicker {
	return NewCherryPickerWithOptions(defaultCherryPickerOpts)
}

// NewCherryPicker returns a cherrypicker with default opts
func NewCherryPickerWithOptions(opts CPOptions) *CherryPicker {
	if opts.Remote == "" {
		opts.Remote = defaultCherryPickerOpts.Remote
	}
	if opts.RepoPath == "" {
		opts.RepoPath = defaultCherryPickerOpts.RepoPath
	}
	return &CherryPicker{
		options: opts,
		state:   CPState{},
		impl:    &defaultCPImplementation{},
	}
}

// SetGitHubClient sets a previously configured github client to interact with GH
func (cp *CherryPicker) SetGitHubClient(ghclient *github.Client) {
	cp.state.github = ghclient
}

type CPOptions struct {
	RepoPath  string // Local path to the repository
	RepoOwner string // Org of the repo we are using
	RepoName  string // Name of the repository
	ForkOwner string
	Remote    string
}

var defaultCherryPickerOpts = CPOptions{
	RepoPath:  ".",
	Remote:    "origin",
	ForkOwner: "",
}

type CPState struct {
	Repository *git.Repository // Repo object to
	github     *github.Client  // go-github client
}

// Actual implementation of the CP interfaces
type cherryPickerImplementation interface {
	initialize(context.Context, *CPState, *CPOptions) error
	readPRcommits(context.Context, *CPState, *CPOptions, *model.PullRequest) ([]*github.RepositoryCommit, error)
	createBranch(*CPState, *CPOptions, string, *model.PullRequest) (string, error)
	cherrypickCommits(*CPState, *CPOptions, string, []string) error
	cherrypickMergeCommit(*CPState, *CPOptions, string, []string, int) error
	getPRMergeMode(context.Context, *CPState, *CPOptions, *model.PullRequest, []*github.RepositoryCommit) (string, error)
	findCommitPatchTree(context.Context, *CPState, *CPOptions, *model.PullRequest, []*github.RepositoryCommit) (int, error)
	GetRebaseCommits(context.Context, *CPState, *CPOptions, *model.PullRequest, []*github.RepositoryCommit) ([]string, error)
	getPullRequest(context.Context, *CPState, *CPOptions, int) (*model.PullRequest, error)
	createPullRequest(context.Context, *CPState, *CPOptions, *model.PullRequest, string, string) (*github.PullRequest, error)
	pushFeatureBranch(*CPState, *CPOptions, string) error
}

// Initialize checks the environment and populates the state
func (impl *defaultCPImplementation) initialize(ctx context.Context, state *CPState, opts *CPOptions) error {
	if state.github == nil {
		if token := os.Getenv("GITHUB_TOKEN"); token == "" {
			return errors.New("unable to cherry-pick, github token not found")
		}

		state.github = github.NewClient(oauth2.NewClient(ctx, oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")},
		)))
	}

	// Check the repository path exists
	if util.Exists(filepath.Join(opts.RepoPath, rebaseMagic)) {
		return errors.New("there is a rebase in progress, unable to cherry pick at this time")
	}

	// Open the repo
	repo, err := git.PlainOpen(opts.RepoPath)
	if err != nil {
		return errors.Wrapf(err, "opening repository from %s", opts.RepoPath)
	}

	// And add it to the state
	state.Repository = repo
	return nil
}

// CreateCherryPickPR creates a cherry-pick PR to the the given branch
func (cp *CherryPicker) CreateCherryPickPR(prNumber int, branch string) error {
	return cp.CreateCherryPickPRWithContext(context.Background(), prNumber, branch)
}

// CreateCherryPickPR creates a cherry-pick PR to the the given branch
func (cp *CherryPicker) CreateCherryPickPRWithContext(ctx context.Context, prNumber int, branch string) (err error) {
	if err := cp.impl.initialize(ctx, &cp.state, &cp.options); err != nil {
		return errors.Wrap(err, "verifying environment")
	}

	// Fetch the pull request
	pr, err := cp.impl.getPullRequest(ctx, &cp.state, &cp.options, prNumber)
	if err != nil {
		return errors.Wrapf(err, "getting pull request %d", prNumber)
	}

	// The first thing we need to create the CPs is to pull the commits
	// from the pull request
	commits, err := cp.impl.readPRcommits(ctx, &cp.state, &cp.options, pr)
	if err != nil {
		return errors.Wrapf(err, "reading commits from PR #%d", pr.Number)
	}

	// Next step: Find out how the PR was merged
	mergeMode, err := cp.impl.getPRMergeMode(ctx, &cp.state, &cp.options, pr, commits)
	if err != nil {
		return errors.Wrapf(err, "getting merge mode for PR #%d", pr.Number)
	}

	// Create the CP branch
	featureBranch, err := cp.impl.createBranch(&cp.state, &cp.options, branch, pr)
	if err != nil {
		return errors.Wrap(err, "creating the feature branch")
	}

	var cpError error

	// The easiest case: PR was squashed. In this case we only need to CP
	// the sha returned in merge_commit_sha
	if mergeMode == MMSQUASH {
		cpError = cp.impl.cherrypickCommits(
			&cp.state, &cp.options, branch, []string{pr.MergeCommitSHA},
		)
	}

	// Next, if the PR resulted in a merge commit, we only need to cherry-pick
	// the `merge_commit_sha` but we have to find out which parent's tree we want
	// to generate the diff from:
	if mergeMode == MMMERGE {
		parent, err := cp.impl.findCommitPatchTree(ctx, &cp.state, &cp.options, pr, commits)
		if err != nil {
			return errors.Wrap(err, "searching for parent patch tree")
		}
		cpError = cp.impl.cherrypickMergeCommit(
			&cp.state, &cp.options, branch, []string{pr.MergeCommitSHA}, parent,
		)
	}

	// Last case. We are dealing with a rebase. In this case we have to take the
	// merge commit and go back in the git log to find the previous trees and
	// CP the commits where they merged
	if mergeMode == MMREBASE {
		rebaseCommits, err := cp.impl.GetRebaseCommits(ctx, &cp.state, &cp.options, pr, commits)
		if err != nil {
			return errors.Wrapf(err, "while getting commits in rebase from PR #%d", pr.Number)
		}

		if len(rebaseCommits) == 0 {
			return errors.Errorf("empty commit list while searching from commits from PR#%d", pr.Number)
		}

		cpError = cp.impl.cherrypickCommits(
			&cp.state, &cp.options, branch, rebaseCommits,
		)
	}

	if cpError != nil {
		return errors.Errorf("while cherrypicking pull request %d of type %s", pr.Number, mergeMode)
	}

	if err := cp.impl.pushFeatureBranch(&cp.state, &cp.options, featureBranch); err != nil {
		return errors.Wrap(err, "pushing branch to git remote")
	}

	// Create the pull request
	pullrequest, err := cp.impl.createPullRequest(
		ctx, &cp.state, &cp.options, pr, featureBranch, branch,
	)
	if err != nil {
		return errors.Wrap(err, "creating pull request in github")
	}

	mlog.Info(fmt.Sprintf("Successfully created pull request #%d", pullrequest.GetNumber()))

	return nil
}

type defaultCPImplementation struct{}

// readPRcommits returns the SHAs of all commits in a PR
func (impl *defaultCPImplementation) readPRcommits(
	ctx context.Context, state *CPState, opts *CPOptions, pr *model.PullRequest,
) (commitList []*github.RepositoryCommit, err error) {
	// Fixme read response and add retries
	commitList, _, err = state.github.PullRequests.ListCommits(
		ctx, pr.RepoOwner, pr.RepoName, pr.Number, &github.ListOptions{PerPage: 100},
	)
	if err != nil {
		return nil, errors.Wrapf(err, "querying GitHub for commits in PR %d", pr.Number)
	}

	mlog.Info(fmt.Sprintf("Read %d commits from PR %d", len(commitList), pr.Number))
	return commitList, nil
}

// createBranch creates the new branch for the cherry pick and
// switches to it. The new branch is created frp, sourceBranch.
func (impl *defaultCPImplementation) createBranch(
	state *CPState, opts *CPOptions, sourceBranch string, pr *model.PullRequest,
) (branchName string, err error) {
	// The new name of the branch, we append the date to make it unique
	branchName = newBranchSlug + fmt.Sprintf("%d", pr.Number) + "-" + fmt.Sprintf("%d", (time.Now().Unix()))

	// Switch to the sourceBranch, this ensures it exists and from there we branch
	if err := command.NewWithWorkDir(
		opts.RepoPath, gitCommand, "checkout", sourceBranch).RunSilentSuccess(); err != nil {
		return "", errors.Wrapf(err, "switching to source branch %s", sourceBranch)
	}

	// Create the new branch:
	if err := command.NewWithWorkDir(
		opts.RepoPath, gitCommand, "branch", branchName).RunSilentSuccess(); err != nil {
		return "", errors.Wrap(err, "creating CP branch")
	}

	// Create the new branch:
	if err := command.NewWithWorkDir(
		opts.RepoPath, gitCommand, "checkout", branchName).RunSilentSuccess(); err != nil {
		return "", errors.Wrap(err, "creating CP branch")
	}

	mlog.Info("created cherry-pick feature branch " + branchName)
	return branchName, nil
}

// cherrypickCommits calls the git command via the shell to cherry-pick the list of
// commits passed into the current repository path.
func (impl *defaultCPImplementation) cherrypickCommits(
	state *CPState, opts *CPOptions, branch string, commits []string,
) (err error) {
	mlog.Info(fmt.Sprintf("Cherry picking %d commits to branch %s", len(commits), branch))
	cmd := command.NewWithWorkDir(opts.RepoPath, gitCommand, append([]string{"cherry-pick"}, commits...)...)
	if _, err = cmd.RunSilent(); err != nil {
		return errors.Wrap(err, "running git cherry-pick")
	}

	// Check if the cp was halted due to unmerged commits
	output, err := command.NewWithWorkDir(
		opts.RepoPath, gitCommand, "status", "--porcelain",
	).RunSuccessOutput()
	if err != nil {
		return errors.Wrap(err, "while trying to look for merge conflicts")
	}
	for _, line := range strings.Split(output.Output(), "\n") {
		if strings.HasPrefix(line, "U") {
			return errors.Errorf("conflicts detected, cannot merge:\n%s", output.Output())
		}
	}
	return nil
}

func (impl *defaultCPImplementation) cherrypickMergeCommit(
	state *CPState, opts *CPOptions, branch string, commits []string, parent int,
) (err error) {
	cmd := command.NewWithWorkDir(
		opts.RepoPath, gitCommand, append([]string{"cherry-pick", "-m", fmt.Sprintf("%d", parent)}, commits...)...,
	)
	if err = cmd.RunSuccess(); err != nil {
		return errors.Wrap(err, "running git cherry-pick")
	}
	return nil
}

// getPRMergeMode implements an algo to try and determine how the PR was
// merged. It should work for most cases except in single commit PRs
// which have been squashed or rebased, but for practical purposes this
// edge case in non relevant.
//
// The PR commits must be fetched beforehand and passed to this function
// to be able to mock it properly.
func (impl *defaultCPImplementation) getPRMergeMode(
	ctx context.Context, state *CPState, opts *CPOptions,
	pr *model.PullRequest, commits []*github.RepositoryCommit,
) (mode string, err error) {
	// Fetch the PR data from the github API
	mergeCommit, _, err := state.github.Repositories.GetCommit(ctx, pr.RepoOwner, pr.RepoName, pr.MergeCommitSHA)
	if err != nil {
		return "", errors.Wrapf(err, "querying GitHub for merge commit %s", pr.MergeCommitSHA)
	}
	if mergeCommit == nil {
		return "", errors.Errorf("commit returned empty when querying sha %s", pr.MergeCommitSHA)
	}

	// If the SHA commit has more than one parent, it is definitely a merge commit.
	if len(mergeCommit.Parents) > 1 {
		mlog.Info(fmt.Sprintf("PR #%d merged via a merge commit", pr.Number))
		return MMMERGE, nil
	}

	// A special case: if the PR only has one commit, we cannot tell if it was rebased or
	// squashed. We return "squash" preemptibly to avoid recomputing trees unnecessarily.
	if len(commits) == 1 {
		mlog.Info(fmt.Sprintf("Considering PR #%d as squash as it only has one commit", pr.Number))
		return MMSQUASH, nil
	}

	// Now, to be able to determine if the PR was squashed, we have to compare the trees
	// of `merge_commit_sha` and the last commit in the PR.
	//
	// In both cases (squashed and rebased) the sha in that field *is not a merge commit*:
	//  * If the PR was squashed, the sha will point to the single resulting commit.
	//  * If the PR was rebased, it will point to the last commit in the sequence
	//
	// If we compare the tree in `merge_commit_sha` and it matches the tree in the last
	// commit in the PR, then we are looking at a rebase.
	//
	// If the tree in the `merge_commit_sha` commit is different from the last commit,
	// then the PR was squashed (thus generating a new tree of al commits combined).

	// Fetch trees from both the merge commit and the last commit in the PR
	mergeTree := mergeCommit.GetCommit().GetTree()
	prTree := commits[len(commits)-1].GetCommit().GetTree()

	mlog.Info(fmt.Sprintf("Merge tree: %s - PR tree: %s", mergeTree.GetSHA(), prTree.GetSHA()))

	// Compare the tree shas...
	if mergeTree.GetSHA() == prTree.GetSHA() {
		// ... if they match the PR was rebased
		mlog.Info(fmt.Sprintf("PR #%d was merged via rebase", pr.Number))
		return MMREBASE, nil
	}

	// Otherwise it was squashed
	mlog.Info(fmt.Sprintf("PR #%d was merged via squash", pr.Number))
	return MMSQUASH, nil
}

// findCommitPatchTree analyzes the parents of a merge commit and
// returns the parent ID whose treee will be used to generate the
// diff for the cherry pick.
func (impl defaultCPImplementation) findCommitPatchTree(
	ctx context.Context, state *CPState, opts *CPOptions,
	pr *model.PullRequest, commits []*github.RepositoryCommit,
) (parentNr int, err error) {
	if len(commits) == 0 {
		return 0, errors.New("unable to find patch tree, commit list is empty")
	}
	// They way to find out which tree to use is to search the tree from
	// the last commit in the PR. The tree sha in the PR commit will match
	// the tree in the PR parent

	// Get the commit information
	mergeCommit, _, err := state.github.Repositories.GetCommit(ctx, pr.RepoOwner, pr.RepoName, pr.MergeCommitSHA)
	if err != nil {
		return 0, errors.Wrapf(err, "querying GitHub for merge commit %s", pr.MergeCommitSHA)
	}
	if mergeCommit == nil {
		return 0, errors.Errorf("commit returned empty when querying sha %s", pr.MergeCommitSHA)
	}

	// First, get the tree hash from the last commit in the PR
	prTree := commits[len(commits)-1].GetCommit().GetTree()
	prSHA := prTree.GetSHA()

	// Now, cycle the parents, fetch their commits and see which one matches
	// the tree hash extracted from the commit
	for pn, parent := range mergeCommit.Parents {
		parentCommit, _, err := state.github.Repositories.GetCommit(ctx, pr.RepoOwner, pr.RepoName, parent.GetSHA())
		if err != nil {
			return 0, errors.Wrapf(err, "querying GitHub for parent commit %s", parent.GetSHA())
		}
		if parentCommit == nil {
			return 0, errors.Errorf("commit returned empty when querying sha %s", parent.GetSHA())
		}

		parentTree := parentCommit.Commit.GetTree()
		mlog.Info(fmt.Sprintf("PR: %s - Parent: %s", prSHA, parentTree.GetSHA()))
		if parentTree.GetSHA() == prSHA {
			mlog.Info(fmt.Sprintf("Cherry pick to be performed diffing the parent #%d tree ", pn))
			return pn, nil
		}
	}

	// If not found, we return an error to make sure we don't use 0
	return 0, errors.Errorf(
		"unable to find patch tree of merge commit among %d parents", len(mergeCommit.Parents),
	)
}

// GetRebaseCommits searches for the commits in the branch history
// that match the modifications in the pull request
func (impl *defaultCPImplementation) GetRebaseCommits(
	ctx context.Context, state *CPState, opts *CPOptions,
	pr *model.PullRequest, prCommits []*github.RepositoryCommit) (commitSHAs []string, err error) {
	// To find the commits, we take the last commit from the PR.
	// The patch should match the commit int the pr `merge_commit_sha` field.
	// From there we navigate backwards in the history ensuring all commits match
	// patches from all commits.

	// First, the merge_commit_sha commit:
	branchCommit, err := impl.getCommit(ctx, state, pr.RepoOwner, pr.RepoName, pr.MergeCommitSHA)
	if err != nil {
		return nil, errors.Wrapf(err, "querying GitHub for merge commit %s", pr.MergeCommitSHA)
	}

	commitSHAs = []string{}

	// Now, lets cycle and make sure we have the right SHAs
	for i := len(prCommits); i > 0; i-- {
		// Get the shas from the trees. They should match
		prTreeSHA := prCommits[i-1].GetCommit().GetTree().GetSHA()
		branchTreeSha := branchCommit.GetCommit().GetTree().GetSHA()
		if prTreeSHA != branchTreeSha {
			return nil, errors.Errorf(
				"Mismatch in PR and branch hashed in commit #%d PR:%s vs Branch:%s",
				i, prTreeSHA, branchTreeSha,
			)
		}

		mlog.Info(fmt.Sprintf("Match #%d PR:%s vs Branch:%s", i, prTreeSHA, branchTreeSha))

		// Append the commit sha to the list (note not to use the *tree hash* here)
		commitSHAs = append(commitSHAs, branchCommit.GetSHA())

		// While we traverse the PR commits linearly, we follow
		// the git graph to get the neext commit int th branch
		branchCommit, err = impl.getCommit(
			ctx, state, pr.RepoOwner, pr.RepoName, branchCommit.Parents[0].GetSHA(),
		)
		if err != nil {
			return nil, errors.Wrapf(
				err, "while fetching branch commit #%d - %s", i, branchCommit.Parents[0].GetSHA(),
			)
		}
	}

	// Reverse the list of shas to preserve the PR order
	for i, j := 0, len(commitSHAs)-1; i < j; i, j = i+1, j-1 {
		commitSHAs[i], commitSHAs[j] = commitSHAs[j], commitSHAs[i]
	}

	return commitSHAs, nil
}

// getCommit gets info about a commit from the github API
func (impl *defaultCPImplementation) getCommit(
	ctx context.Context, state *CPState, owner, repo, commitSHA string,
) (cmt *github.RepositoryCommit, err error) {
	// Get the commit from the API:
	cmt, _, err = state.github.Repositories.GetCommit(ctx, owner, repo, commitSHA)
	if err != nil {
		return nil, errors.Wrapf(err, "querying GitHub for commit %s", commitSHA)
	}
	if cmt == nil {
		return nil, errors.Errorf("commit returned empty when querying sha %s", commitSHA)
	}

	return cmt, nil
}

// getPullRequest fetches a pull request from GitHub
func (impl *defaultCPImplementation) getPullRequest(
	ctx context.Context, state *CPState, opts *CPOptions, prNumber int,
) (*model.PullRequest, error) {
	pullRequest, _, err := state.github.PullRequests.Get(
		ctx, opts.RepoOwner, opts.RepoName, prNumber)
	if err != nil {
		return nil, errors.Wrapf(err, "getting pull request %d from GitHub", prNumber)
	}
	return &model.PullRequest{
		RepoOwner:           pullRequest.GetBase().GetRepo().GetOwner().GetLogin(),
		RepoName:            pullRequest.GetBase().GetRepo().GetName(),
		Number:              pullRequest.GetNumber(),
		Username:            pullRequest.GetUser().GetLogin(),
		FullName:            pullRequest.GetHead().GetRepo().GetFullName(),
		Ref:                 pullRequest.GetHead().GetRef(),
		Sha:                 pullRequest.GetHead().GetSHA(),
		State:               pullRequest.GetState(),
		URL:                 pullRequest.GetURL(),
		CreatedAt:           pullRequest.GetCreatedAt(),
		Merged:              NewBool(pullRequest.GetMerged()),
		MergeCommitSHA:      pullRequest.GetMergeCommitSHA(),
		MaintainerCanModify: NewBool(pullRequest.GetMaintainerCanModify()),
		MilestoneNumber:     NewInt64(int64(pullRequest.GetMilestone().GetNumber())),
		MilestoneTitle:      NewString(pullRequest.GetMilestone().GetTitle()),
	}, nil
}

// pushFeatureBranch pushes thw new branch with the CPs to the remote
func (impl *defaultCPImplementation) pushFeatureBranch(
	state *CPState, opts *CPOptions, featureBranch string,
) error {
	// Push the feature branch to the specified remote
	if err := command.NewWithWorkDir(
		opts.RepoPath, gitCommand, "push", opts.Remote, featureBranch,
	).RunSilentSuccess(); err != nil {
		return errors.Wrapf(err, "pushing branch %s to remote %s", featureBranch, opts.Remote)
	}
	mlog.Info(fmt.Sprintf("Successfully pushed %s to remote %s", featureBranch, opts.Remote))
	return nil
}

// createPullRequest cresates the cherry-picks pull request
func (impl *defaultCPImplementation) createPullRequest(
	ctx context.Context, state *CPState, opts *CPOptions, pr *model.PullRequest, featureBranch, baseBranch string,
) (*github.PullRequest, error) {
	// We will pass the branchname to git
	headBranchName := featureBranch
	// Unless a fork is defined in the options. IN this case we append the fork Org
	// to the branch and use that as the head branch
	if opts.ForkOwner != "" {
		headBranchName = fmt.Sprintf("%s:%s", opts.ForkOwner, featureBranch)
	}
	newPullRequest := &github.NewPullRequest{
		Title:               NewString(fmt.Sprintf(prTitleTemplate, pr.Number, baseBranch)),
		Head:                NewString(headBranchName),
		Base:                &baseBranch,
		Body:                NewString(fmt.Sprintf(prBodyTemplate, pr.Number, baseBranch, pr.Number, baseBranch, pr.Username)),
		MaintainerCanModify: github.Bool(true),
	}

	// Send the PR to GItHub:
	pullrequest, _, err := state.github.PullRequests.Create(ctx, opts.RepoOwner, opts.RepoName, newPullRequest)
	if err != nil {
		return pullrequest, errors.Wrap(err, "creating pull request")
	}

	return pullrequest, nil
}
