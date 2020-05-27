package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/mattermost/mattermost-server/v5/mlog"

	"github.com/google/go-github/v31/github"
)

func (s *Server) handleUpdateBranch(eventIssueComment IssueComment) {
	prGitHub, _, err := s.GithubClient.PullRequests.Get(context.Background(), *eventIssueComment.Repository.Owner.Login, *eventIssueComment.Repository.Name, *eventIssueComment.Issue.Number)
	if err != nil {
		mlog.Error("Error getting the latest PR information from github", mlog.Err(err))
		return
	}
	pr, err := s.GetPullRequestFromGithub(prGitHub)
	if err != nil {
		mlog.Error("Error Updating the PR in the DB", mlog.Err(err))
		return
	}

	commenter := eventIssueComment.Comment.User.GetLogin()
	// If the commenter is not the PR submitter, check if the PR submitter is an org member
	if commenter != pr.Username && !s.IsOrgMember(commenter) {
		mlog.Debug("not org member", mlog.String("user", commenter))
		s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number, "Looks like you don't have permissions to trigger this command.\n Only available for the PR submitter and org members")
		return
	}

	repoInfo := strings.Split(pr.FullName, "/")
	if repoInfo[0] != s.Config.Org {
		if !prGitHub.GetMaintainerCanModify() {
			s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number, "We don't have permissions to update this PR, please contact the submitter to apply the update.")
			return
		}
	}

	opt := &github.PullRequestBranchUpdateOptions{
		ExpectedHeadSHA: github.String(pr.Sha),
	}

	_, resp, err := s.GithubClient.PullRequests.UpdateBranch(context.Background(), pr.RepoOwner, pr.RepoName, pr.Number, opt)
	if resp != nil && resp.StatusCode != http.StatusAccepted {
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
