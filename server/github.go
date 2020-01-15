// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"net/http"
	"regexp"
	"strconv"

	"github.com/google/go-github/v28/github"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v5/mlog"
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
		FullName:  "",
		Ref:       *pullRequest.Head.Ref,
		Sha:       *pullRequest.Head.SHA,
		State:     *pullRequest.State,
		URL:       *pullRequest.URL,
		CreatedAt: pullRequest.GetCreatedAt(),
	}

	if pullRequest.Head.Repo != nil {
		pr.FullName = *pullRequest.Head.Repo.FullName
	}

	client := NewGithubClient(s.Config.GithubAccessToken)

	repo, ok := GetRepository(s.Config.Repositories, pr.RepoOwner, pr.RepoName)
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

	if result := <-s.Store.PullRequest().Save(pr); result.Err != nil {
		mlog.Error(result.Err.Error())
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

func (s *Server) sendGitHubComment(repoOwner, repoName string, number int, comment string) {
	mlog.Debug("Sending GitHub comment", mlog.Int("issue", number), mlog.String("comment", comment))
	client := NewGithubClient(s.Config.GithubAccessToken)
	_, _, err := client.Issues.CreateComment(context.Background(), repoOwner, repoName, number, &github.IssueComment{Body: &comment})
	if err != nil {
		mlog.Error("Error commenting", mlog.Err(err))
	}
}

func (s *Server) removeLabel(repoOwner, repoName string, number int, label string) {
	mlog.Info("Removing label on issue", mlog.Int("issue", number), mlog.String("label", label))
	client := NewGithubClient(s.Config.GithubAccessToken)
	_, err := client.Issues.RemoveLabelForIssue(context.Background(), repoOwner, repoName, number, label)
	if err != nil {
		mlog.Error("Error removing the label", mlog.Err(err))
	}
	mlog.Info("Finished removing the label")
}

func (s *Server) getComments(repoOwner, repoName string, number int) ([]*github.IssueComment, error) {
	client := NewGithubClient(s.Config.GithubAccessToken)
	comments, _, err := client.Issues.ListComments(context.Background(), repoOwner, repoName, number, nil)
	if err != nil {
		mlog.Error("pr_error", mlog.Err(err))
		return nil, err
	}
	return comments, nil
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

func (s *Server) checkIfRefExists(pr *model.PullRequest, ref string) (bool, error) {
	client := NewGithubClient(s.Config.GithubAccessToken)
	_, response, err := client.Git.GetRef(context.Background(), pr.RepoOwner, pr.RepoName, ref)
	if err != nil {
		mlog.Error("Unable to check if reference exists. ", mlog.Err(err))
	}

	if response.StatusCode == 200 {
		mlog.Info("Reference found. ", mlog.Int("pr", pr.Number), mlog.String("ref", ref))
		return true, nil
	} else if response.StatusCode == 404 {
		mlog.Info("Unable to find reference. ", mlog.Int("pr", pr.Number), mlog.String("ref", ref))
		return false, nil
	} else {
		mlog.Info("Unknown response code while trying to check for reference. ", mlog.Int("pr", pr.Number), mlog.Int("response_code", response.StatusCode), mlog.String("ref", ref))
		return false, nil
	}
}

func (s *Server) createRefWithPrefixFromPr(pr *model.PullRequest, prefix string) {
	client := NewGithubClient(s.Config.GithubAccessToken)
	_, _, err := client.Git.CreateRef(
		context.Background(),
		pr.RepoOwner,
		pr.RepoName,
		&github.Reference{
			Ref: github.String("refs/heads/" + prefix + strconv.Itoa(pr.Number)),
			Object: &github.GitObject{
				SHA: github.String(pr.Sha),
			},
		})

	if err != nil {
		mlog.Error("Error creating reference", mlog.Err(err))
	}
}

func (s *Server) deleteRefWhereCombinedStateEqualsSuccess(repoOwner string, repoName string, ref string) {
	client := NewGithubClient(s.Config.GithubAccessToken)

	cStatus, _, _ := client.Repositories.GetCombinedStatus(context.Background(), repoOwner, repoName, ref, nil)
	if cStatus.GetState() == "success" {
		r, err := client.Git.DeleteRef(context.Background(), repoOwner, repoName, "refs/heads/"+ref)
		if err != nil {
			mlog.Error("Error deleting branch", mlog.String("branch", ref), mlog.Err(err))
		}
		if r.StatusCode == http.StatusNoContent {
			mlog.Info("Successfully deleted branch", mlog.String("branch", ref))
		}
	}
}

func (s *Server) deleteRef(repoOwner string, repoName string, ref string) error {
	client := NewGithubClient(s.Config.GithubAccessToken)

	r, err := client.Git.DeleteRef(context.Background(), repoOwner, repoName, "refs/heads/"+ref)
	if err != nil {
		mlog.Error("Error deleting branch", mlog.String("branch", ref), mlog.Err(err))
		return err
	}
	if r.StatusCode == http.StatusNoContent {
		mlog.Info("Successfully deleted branch", mlog.String("branch", ref))
	}
	return nil
}

func (s *Server) isCombinedStatusSuccessForPR(pr *model.PullRequest) bool {
	client := NewGithubClient(s.Config.GithubAccessToken)
	cStatus, _, _ := client.Repositories.GetCombinedStatus(context.Background(), pr.RepoOwner, pr.RepoName, pr.Ref, nil)
	mlog.Info(cStatus.GetState())
	if cStatus.GetState() == "success" || cStatus.GetState() == "" {
		return true
	}
	return false
}

func isBranchPrefix(regexp *regexp.Regexp, branchName string) bool {
	if regexp.MatchString(branchName) {
		return true
	}
	return false
}
