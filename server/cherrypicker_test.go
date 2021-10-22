package server

import (
	"context"
	"os"
	"strings"
	"testing"

	git "github.com/go-git/go-git/v5"
	"github.com/google/go-github/v33/github"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v5/mlog"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/release-utils/command"
)

func createTestRepo(t *testing.T) string {
	dir, err := os.MkdirTemp("", "test-repo-")
	require.Nil(t, err)

	require.Nil(t, command.NewWithWorkDir(dir, gitCommand, "init", "--initial-branch=main").RunSuccess())
	require.Nil(t, command.NewWithWorkDir(dir, gitCommand, "config", "user.email", "user@example.com").RunSuccess())
	require.Nil(t, command.NewWithWorkDir(dir, gitCommand, "config", "user.name", "Example Users").RunSuccess())
	require.Nil(t, command.NewWithWorkDir(dir, gitCommand, "commit", "--allow-empty", "-m", "First Commit").RunSuccess())
	return dir
}

func TestCreateBranch(t *testing.T) {
	repoDir := createTestRepo(t)
	defer os.RemoveAll(repoDir)

	// Open the repo and plug it into the state
	repo, err := git.PlainOpen(repoDir)
	require.Nil(t, err)

	state := &CPState{
		Repository: repo,
	}

	impl := defaultCPImplementation{}
	pr := &model.PullRequest{
		RepoOwner:      "mattermost",
		RepoName:       "mattermost-server",
		Number:         18746,
		MergeCommitSHA: "f68ba02e325002d7982936860f202b0524ee33bb",
	}
	branchName, err := impl.createBranch(state, &CPOptions{RepoPath: repoDir}, "main", pr)
	require.Nil(t, err)
	mlog.Info("Created branch " + branchName)

	// Ensure the branch was created
	cmd := command.NewWithWorkDir(repoDir, "git", "branch")
	output, err := cmd.RunSuccessOutput()
	require.Nil(t, err)

	lines := strings.Split(output.OutputTrimNL(), "\n")
	require.Len(t, lines, 2)
	found := false
	for _, line := range lines {
		if strings.TrimSpace(strings.TrimPrefix(line, "* ")) == branchName {
			found = true
		}
	}
	require.True(t, found, "checking if new branch was created")
}

func TestGetPRMergeMode(t *testing.T) {
	impl := defaultCPImplementation{}
	ctx := context.Background()
	opts := &CPOptions{}
	state := &CPState{
		github: github.NewClient(nil),
	}

	for _, tc := range []struct {
		MergeCommitSHA string
		ExpectedMode   string
		PrNumber       int
		ExpectedLength int
	}{
		{
			PrNumber:       18746, // This PR has 10 commits, and was rebased
			ExpectedLength: 10,
			MergeCommitSHA: "f68ba02e325002d7982936860f202b0524ee33bb",
			ExpectedMode:   "rebase",
		},
		{
			PrNumber:       18759, // PR resulted in a merge commit , pointing to two commits
			ExpectedLength: 2,
			MergeCommitSHA: "bc19bb33b0590a7c5699d9a2618911adfd7c7d7c",
			ExpectedMode:   "merge",
		},
		{
			PrNumber:       18698, // Two commits, squashed
			ExpectedLength: 2,
			MergeCommitSHA: "e6f36f064959261f588c11f91aeb2fcb8164d70b",
			ExpectedMode:   "squash",
		},
		{
			PrNumber:       18733, // Single commit, unless merged should return "squash"
			ExpectedLength: 1,
			MergeCommitSHA: "2a07d4641abfef5327249c380edb8b1292337319",
			ExpectedMode:   "squash",
		},
	} {
		pr := &model.PullRequest{
			RepoOwner:      "mattermost",
			RepoName:       "mattermost-server",
			Number:         tc.PrNumber,
			MergeCommitSHA: tc.MergeCommitSHA,
		}

		// Perhaps we should precache the commits here. Maybe later
		commits, err := impl.readPRcommits(ctx, state, opts, pr)
		require.Nil(t, err, "fetching commits")
		require.Len(t, commits, tc.ExpectedLength)

		mode, err := impl.getPRMergeMode(ctx, state, opts, pr, commits)
		require.Nil(t, err)
		require.Equal(t, tc.ExpectedMode, mode)
	}
}

func TestReadPRcommits(t *testing.T) {
	impl := defaultCPImplementation{}
	state := &CPState{
		github: github.NewClient(nil),
	}

	for _, tc := range []struct {
		PrNumber       int
		ExpectedLength int
	}{
		{
			PrNumber:       18746, // This is a PR merged rebased
			ExpectedLength: 10,
		},
		{
			PrNumber:       18722, // Merge commit
			ExpectedLength: 2,
		},
	} {
		// Cicle some test PRs which we know
		pr := &model.PullRequest{
			RepoOwner: "mattermost",
			RepoName:  "mattermost-server",
			Number:    tc.PrNumber,
			// MergeCommitSHA:      "",
		}

		commits, err := impl.readPRcommits(context.Background(), state, &CPOptions{}, pr)
		require.Nil(t, err, "reading PR commits")
		require.Len(t, commits, tc.ExpectedLength)
	}
}

func TestFindCommitPatchTree(t *testing.T) {
	impl := defaultCPImplementation{}
	ctx := context.Background()
	opts := &CPOptions{}
	state := &CPState{
		github: github.NewClient(nil),
	}
	pr := &model.PullRequest{
		RepoOwner:      "mattermost",
		RepoName:       "mattermost-server",
		Number:         18759,
		MergeCommitSHA: "bc19bb33b0590a7c5699d9a2618911adfd7c7d7c",
	}
	// Get the comits, they are required
	commits, err := impl.readPRcommits(ctx, state, opts, pr)
	require.Nil(t, err, "fetching commits")
	require.Len(t, commits, 2)

	// In Github, generally parent #0 points to the branch history, while
	// parent #1 points to the commit list in the PR
	parentID, err := impl.findCommitPatchTree(ctx, state, opts, pr, commits)
	require.Nil(t, err)
	require.Equal(t, 1, parentID)
}

func TestGetRebaseCommits(t *testing.T) {
	impl := defaultCPImplementation{}
	ctx := context.Background()
	opts := &CPOptions{}
	state := &CPState{
		github: github.NewClient(nil),
	}

	pr := &model.PullRequest{
		RepoOwner:      "mattermost",
		RepoName:       "mattermost-server",
		Number:         18746,
		MergeCommitSHA: "f68ba02e325002d7982936860f202b0524ee33bb",
	}

	// Get the comits, they are required
	commits, err := impl.readPRcommits(ctx, state, opts, pr)
	require.Nil(t, err, "fetching commits")
	require.Len(t, commits, 10)

	//
	commitList, err := impl.GetRebaseCommits(ctx, state, opts, pr, commits)
	require.Nil(t, err, "getting rebase commits")
	require.Len(t, commitList, 10)

	require.Equal(t, "f68ba02e325002d7982936860f202b0524ee33bb", commitList[9])
	require.Equal(t, "125767e905e06779c36dd97bc405fd73d1e18f5f", commitList[8])
	require.Equal(t, "ca6e387e7eb7ee95d80c61540b5bf9840ee15255", commitList[7])
	require.Equal(t, "2a18f5e31364faf48de617de2011c14124de90a1", commitList[6])
	require.Equal(t, "e5caaf33c0c4c500308fbc3f8e803481c7494bad", commitList[5])
	require.Equal(t, "676cebd459c7e30e9444e692693f44b483b6dc26", commitList[4])
	require.Equal(t, "c3569b7c6b43a483a9910851afb36f44cbfdff28", commitList[3])
	require.Equal(t, "e6528fdcc4af928407a96e83004bc4d19f1bc797", commitList[2])
	require.Equal(t, "ecd49172414b819632dc59adcd5bb6e480ee759e", commitList[1])
	require.Equal(t, "ec9f8df72de730cb3b61c72678cdc050e93f925d", commitList[0])
}
