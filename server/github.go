// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/go-github/v39/github"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v6/shared/mlog"
	"github.com/pkg/errors"
)

const (
	statePending       = "pending"
	stateSuccess       = "success"
	stateError         = "error"
	prEventOpened      = "opened"
	prEventReOpened    = "reopened"
	prEventLabeled     = "labeled"
	prEventUnLabeled   = "unlabeled"
	prEventSynchronize = "synchronize"
	prEventClosed      = "closed"
)

func (s *Server) GetPullRequestFromGithub(ctx context.Context, pullRequest *github.PullRequest, action string) (*model.PullRequest, error) {
	pr := &model.PullRequest{
		RepoOwner:           pullRequest.GetBase().GetRepo().GetOwner().GetLogin(),
		RepoName:            pullRequest.GetBase().GetRepo().GetName(),
		Number:              pullRequest.GetNumber(),
		Username:            pullRequest.GetUser().GetLogin(),
		FullName:            "",
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
	}

	pr.FullName = pullRequest.GetHead().GetRepo().GetFullName()

	repo, ok := GetRepository(s.Config.Repositories, pr.RepoOwner, pr.RepoName)
	if ok && repo.BuildStatusContext != "" {
		combined, _, err := s.GithubClient.Repositories.GetCombinedStatus(ctx, pr.RepoOwner, pr.RepoName, pr.Sha, nil)
		if err != nil {
			return nil, err
		}

		for _, status := range combined.Statuses {
			if status.GetContext() == repo.BuildStatusContext {
				pr.BuildStatus = status.GetState()
				pr.BuildLink = status.GetTargetURL()
				break
			}
		}

		// for the repos using circleci we have the checks now
		checks, _, err := s.GithubClient.Checks.ListCheckRunsForRef(ctx, pr.RepoOwner, pr.RepoName, pr.Sha, nil)
		if err != nil {
			return nil, err
		}

		for _, status := range checks.CheckRuns {
			if status.GetName() == repo.BuildStatusContext {
				pr.BuildStatus = status.GetStatus()
				pr.BuildConclusion = status.GetConclusion()
				pr.BuildLink = status.GetHTMLURL()
				break
			}
		}
	}

	// if is opened it might not have any label yet
	if action != prEventOpened {
		labels, _, err := s.GithubClient.Issues.ListLabelsByIssue(ctx, pr.RepoOwner, pr.RepoName, pr.Number, nil)
		if err != nil {
			return nil, err
		}

		pr.Labels = labelsToStringArray(labels)
	}

	if _, err := s.Store.PullRequest().Save(pr); err != nil {
		return nil, err
	}

	return pr, nil
}

func (s *Server) GetIssueFromGithub(ctx context.Context, ghIssue *github.Issue) (*model.Issue, error) {
	issue := &model.Issue{
		RepoOwner: ghIssue.GetRepository().GetOwner().GetLogin(),
		RepoName:  ghIssue.GetRepository().GetName(),
		Number:    ghIssue.GetNumber(),
		Username:  ghIssue.GetUser().GetLogin(),
		State:     ghIssue.GetState(),
	}

	if issue.RepoOwner == "" || issue.RepoName == "" {
		// URL is expected to be in the form of https://github.com/{repoOwner}/{repoName}/pull/{number}
		parts := strings.Split(ghIssue.GetHTMLURL(), "/")
		if len(parts) < 5 {
			return nil, fmt.Errorf("could not get repo owner or repo name from url: %q", ghIssue.GetHTMLURL())
		}
		issue.RepoOwner = parts[3]
		issue.RepoName = parts[4]
	}

	labels, _, err := s.GithubClient.Issues.ListLabelsByIssue(ctx, issue.RepoOwner, issue.RepoName, issue.Number, nil)
	if err != nil {
		return nil, err
	}
	issue.Labels = labelsToStringArray(labels)

	return issue, nil
}

func labelsToStringArray(labels []*github.Label) []string {
	out := make([]string, len(labels))

	for i, label := range labels {
		out[i] = *label.Name
	}

	return out
}

func (s *Server) sendGitHubComment(ctx context.Context, repoOwner, repoName string, number int, comment string) error {
	mlog.Debug("Sending GitHub comment", mlog.Int("issue", number), mlog.String("comment", comment))
	_, _, err := s.GithubClient.Issues.CreateComment(ctx, repoOwner, repoName, number, &github.IssueComment{Body: &comment})
	return err
}

func (s *Server) removeLabel(ctx context.Context, repoOwner, repoName string, number int, label string) {
	mlog.Info("Removing label on issue", mlog.Int("issue", number), mlog.String("label", label))

	_, err := s.GithubClient.Issues.RemoveLabelForIssue(ctx, repoOwner, repoName, number, label)
	if err != nil {
		mlog.Error("Error removing the label", mlog.Err(err))
	}
	mlog.Info("Finished removing the label")
}

func (s *Server) getComments(ctx context.Context, repoOwner, repoName string, issueNumber int) ([]*github.IssueComment, error) {
	opts := &github.IssueListCommentsOptions{
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}
	var allComments []*github.IssueComment
	for {
		commentsPerPage, r, err := s.GithubClient.Issues.ListComments(ctx, repoOwner, repoName, issueNumber, opts)
		if err != nil {
			return nil, err
		}
		allComments = append(allComments, commentsPerPage...)
		if r != nil && r.StatusCode != http.StatusOK {
			return nil, errors.Errorf("failed fetching comments: got http status %s", r.Status)
		}
		if r.NextPage == 0 {
			break
		}
		opts.Page = r.NextPage
	}
	return allComments, nil
}

func (s *Server) getFiles(ctx context.Context, repoOwner, repoName string, issueNumber int) ([]*github.CommitFile, error) {
	opts := &github.ListOptions{
		PerPage: 100,
	}
	var allFiles []*github.CommitFile

	for {
		files, r, err := s.GithubClient.PullRequests.ListFiles(ctx, repoOwner, repoName, issueNumber, opts)
		if err != nil {
			return nil, err
		}
		allFiles = append(allFiles, files...)
		if r != nil && r.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed fetching files: got http status: %s", r.Status)
		}
		if r.NextPage == 0 {
			break
		}
		opts.Page = r.NextPage
	}
	return allFiles, nil
}

func (s *Server) GetUpdateChecks(ctx context.Context, owner, repoName string, prNumber int) (*model.PullRequest, error) {
	prGitHub, _, err := s.GithubClient.PullRequests.Get(ctx, owner, repoName, prNumber)
	if err != nil {
		mlog.Error("Failed to get PR for update check", mlog.Err(err))
		return nil, err
	}

	pr, err := s.GetPullRequestFromGithub(ctx, prGitHub, "")
	if err != nil {
		mlog.Error("pr_error", mlog.Err(err))
		return nil, err
	}

	return pr, nil
}

func (s *Server) getMembers(ctx context.Context) (orgMembers []string, err error) {
	opts := &github.ListMembersOptions{
		ListOptions: github.ListOptions{},
	}
	var allUsers []*github.User
	for {
		users, r, err := s.GithubClient.Organizations.ListMembers(ctx, s.Config.Org, opts)
		if err != nil {
			return nil, err
		}
		allUsers = append(allUsers, users...)
		if r != nil && r.StatusCode != http.StatusOK {
			return nil, errors.Errorf("failed listing org members: got http status %s", r.Status)
		}
		if r.NextPage == 0 {
			break
		}
		opts.Page = r.NextPage
	}

	members := make([]string, len(allUsers))
	for i, user := range allUsers {
		members[i] = user.GetLogin()
	}

	return members, nil
}

func (s *Server) IsOrgMember(user string) bool {
	for _, member := range s.OrgMembers {
		if user == member {
			return true
		}
	}
	return false
}

func (s *Server) IsInBotBlockList(user string) bool {
	for _, member := range s.Config.BlockListBots {
		if user == member {
			return true
		}
	}
	return false
}

func (s *Server) IsBotUserFromCLAExclusionsList(user string) bool {
	for _, claExcludedUser := range s.Config.CLAExclusionsList {
		if user == claExcludedUser {
			return true
		}
	}
	return false
}

func (s *Server) createRepoStatus(ctx context.Context, pr *model.PullRequest, status *github.RepoStatus) error {
	_, _, err := s.GithubClient.Repositories.CreateStatus(ctx, pr.RepoOwner, pr.RepoName, pr.Sha, status)
	if err != nil {
		return err
	}
	return nil
}
