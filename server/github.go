// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"

	"github.com/google/go-github/github"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/mlog"
	"golang.org/x/oauth2"
)

func NewGithubClient(token string) *github.Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(oauth2.NoContext, ts)

	return github.NewClient(tc)
}

func (s *Server) GetPullRequestFromGithub(pullRequest *github.PullRequest) (*model.PullRequest, error) {
	pr := &model.PullRequest{
		RepoOwner: *pullRequest.Base.Repo.Owner.Login,
		RepoName:  *pullRequest.Base.Repo.Name,
		Number:    *pullRequest.Number,
		Username:  *pullRequest.User.Login,
		Ref:       *pullRequest.Head.Ref,
		Sha:       *pullRequest.Head.SHA,
		State:     *pullRequest.State,
		URL:       *pullRequest.URL,
	}

	client := NewGithubClient(s.Config.GithubAccessToken)

	repo, ok := s.GetRepository(pr.RepoOwner, pr.RepoName)
	if ok && repo.BuildStatusContext != "" {
		if combined, _, err := client.Repositories.GetCombinedStatus(context.Background(), pr.RepoOwner, pr.RepoName, pr.Sha, nil); err != nil {
			return nil, err
		} else {
			for _, status := range combined.Statuses {
				if *status.Context == repo.BuildStatusContext {
					pr.BuildStatus = *status.State
					pr.BuildLink = *status.TargetURL
					break
				}
			}
		}

		// for the repos using circleci we have the checks now
		if checks, _, err := client.Checks.ListCheckRunsForRef(context.Background(), pr.RepoOwner, pr.RepoName, pr.Sha, nil); err != nil {
			return nil, err
		} else {
			for _, status := range checks.CheckRuns {
				if *status.Name == repo.BuildStatusContext {
					pr.BuildStatus = status.GetStatus()
					pr.BuildConclusion = status.GetConclusion()
					pr.BuildLink = status.GetHTMLURL()
					break
				}
			}
		}
	}

	if labels, _, err := client.Issues.ListLabelsByIssue(context.Background(), pr.RepoOwner, pr.RepoName, pr.Number, nil); err != nil {
		return nil, err
	} else {
		pr.Labels = labelsToStringArray(labels)
	}

	return pr, nil
}

func (s *Server) GetIssueFromGithub(repoOwner, repoName string, ghIssue *github.Issue) (*model.Issue, error) {
	issue := &model.Issue{
		RepoOwner: repoOwner,
		RepoName:  repoName,
		Number:    *ghIssue.Number,
		Username:  *ghIssue.User.Login,
		State:     *ghIssue.State,
	}

	if labels, _, err := NewGithubClient(s.Config.GithubAccessToken).Issues.ListLabelsByIssue(context.Background(), issue.RepoOwner, issue.RepoName, issue.Number, nil); err != nil {
		return nil, err
	} else {
		issue.Labels = labelsToStringArray(labels)
	}

	return issue, nil
}

func labelsToStringArray(labels []*github.Label) []string {
	out := make([]string, len(labels))

	for i, label := range labels {
		out[i] = *label.Name
	}

	return out
}

func (s *Server) commentOnIssue(repoOwner, repoName string, number int, comment string) {
	mlog.Info("Commenting on issue", mlog.Int("issue", number), mlog.String("comment", comment))
	client := NewGithubClient(s.Config.GithubAccessToken)
	_, _, err := client.Issues.CreateComment(context.Background(), repoOwner, repoName, number, &github.IssueComment{Body: &comment})
	if err != nil {
		mlog.Error("Error", mlog.Err(err))
	}
	mlog.Info("Finished commenting")
}

func (s *Server) GetUpdateChecks(owner, repoName string, prNumber int) (*model.PullRequest, error) {
	client := NewGithubClient(s.Config.GithubAccessToken)
	prGitHub, _, err := client.PullRequests.Get(context.Background(), owner, repoName, prNumber)
	pr, err := s.GetPullRequestFromGithub(prGitHub)
	if err != nil {
		mlog.Error("pr_error", mlog.Err(err))
		return nil, err
	}

	if result := <-s.Store.PullRequest().Save(pr); result.Err != nil {
		mlog.Error(result.Err.Error())
	}

	return pr, nil
}

func (s *Server) checkUserPermission(user, repoOwner string) bool {
	client := NewGithubClient(s.Config.GithubAccessToken)

	_, resp, err := client.Organizations.GetOrgMembership(context.Background(), user, repoOwner)
	if resp.StatusCode == 404 {
		mlog.Info("User is not part of the ORG", mlog.String("User", user))
		return false
	}
	if err != nil {
		return false
	}

	return true
}
