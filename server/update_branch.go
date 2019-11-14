package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/mattermost/mattermost-server/mlog"

	"github.com/google/go-github/v28/github"
)

func (s *Server) handleUpdateBranch(eventIssueComment IssueComment) {
	client := NewGithubClient(s.Config.GithubAccessToken)
	prGitHub, _, err := client.PullRequests.Get(context.Background(), *eventIssueComment.Repository.Owner.Login, *eventIssueComment.Repository.Name, *eventIssueComment.Issue.Number)
	if err != nil {
		mlog.Error("Error getting the latest PR information from github", mlog.Err(err))
		return
	}
	pr, err := s.GetPullRequestFromGithub(prGitHub)
	if err != nil {
		mlog.Error("Error Updating the PR in the DB", mlog.Err(err))
		return
	}

	userComment := *eventIssueComment.Comment.User
	if !s.checkUserPermission(userComment.GetLogin(), pr.RepoOwner) {
		s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number, "Looks like you dont have permissions to trigger this command.\n Only available for Org members")
		return
	}

	if !prGitHub.GetMaintainerCanModify() {
		s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number, "We dont have permissions to update this PR, please contact the submitter to apply the update.")
		return
	}

	opt := &github.PullReqestBranchUpdateOptions{
		ExpectedHeadSHA: github.String(pr.Sha),
	}

	_, resp, err := client.PullRequests.UpdateBranch(context.Background(), pr.RepoOwner, pr.RepoName, pr.Number, opt)
	if resp.StatusCode != http.StatusAccepted {
		s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number, "Error trying to update the PR.\nPlease do it manually.")
		return
	}
	if err != nil {
		if !strings.Contains("job scheduled on GitHub side; try again later", err.Error()) {
			msg := fmt.Sprintf("Error trying to update the PR.\nPlease do it manually.\nError: %s", err.Error())
			s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number, msg)
		}
	}
}
