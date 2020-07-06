package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/mattermost/mattermost-mattermod/model"

	"github.com/google/go-github/v32/github"
)

var (
	ErrCommenterPermission    error = errors.New("commenter does not have permissions")
	ErrOrganizationPermission error = errors.New("we don't have permissions")
	ErrUpdatePullRequest      error = errors.New("could not update pull request")
)

func (s *Server) handleUpdateBranch(ctx context.Context, eventIssueComment IssueComment) error {
	prGitHub, _, err := s.GithubClient.PullRequests.Get(ctx,
		*eventIssueComment.Repository.Owner.Login,
		*eventIssueComment.Repository.Name,
		*eventIssueComment.Issue.Number,
	)
	if err != nil {
		return fmt.Errorf("could not get the latest PR information from github: %w", err)
	}

	pr, err := s.GetPullRequestFromGithub(ctx, prGitHub)
	if err != nil {
		return fmt.Errorf("error updating the PR in the DB: %w", err)
	}

	commenter := eventIssueComment.Comment.User.GetLogin()
	if err := s.checkUpdatePullRequestPermissions(commenter, pr); err != nil {
		var msg string
		switch err {
		case ErrCommenterPermission:
			msg = "Looks like you don't have permissions to trigger this command.\n Only available for the PR submitter and org members"
		case ErrOrganizationPermission:
			msg = "We don't have permissions to update this PR, please contact the submitter to apply the update."
		}
		s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, msg)
		return err
	}

	if err := s.updatePullRequest(ctx, pr); err != nil {
		s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, "Error trying to update the PR.\nPlease do it manually.")
		return err
	}

	return nil
}

func (s *Server) checkUpdatePullRequestPermissions(commenter string, pr *model.PullRequest) error {
	// If the commenter is not the PR submitter, check if the PR submitter is an org member
	if commenter != pr.Username && !s.IsOrgMember(commenter) {
		return ErrCommenterPermission
	}

	repoInfo := strings.Split(pr.Username, "/")
	if repoInfo[0] != s.Config.Org {
		if !pr.MaintainerCanModify {
			return ErrOrganizationPermission
		}
	}
	return nil
}

func (s *Server) updatePullRequest(ctx context.Context, pr *model.PullRequest) error {
	opt := &github.PullRequestBranchUpdateOptions{
		ExpectedHeadSHA: github.String(pr.Sha),
	}

	_, resp, err := s.GithubClient.PullRequests.UpdateBranch(ctx, pr.RepoOwner, pr.RepoName, pr.Number, opt)
	if resp != nil && resp.StatusCode != http.StatusAccepted {
		return ErrUpdatePullRequest
	}
	if err != nil && !strings.Contains("job scheduled on GitHub side; try again later", err.Error()) {
		return ErrUpdatePullRequest
	}
	return nil
}
