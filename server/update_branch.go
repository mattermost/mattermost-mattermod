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
	ErrCommenterPermission    = errors.New("commenter does not have permissions")
	ErrOrganizationPermission = errors.New("we don't have permissions")
	ErrUpdatePullRequest      = errors.New("could not update pull request")
)

const (
	MsgCommenterPermission    = "Looks like you don't have permissions to trigger this command.\n Only available for the PR submitter and org members"
	MsgOrganizationPermission = "We don't have permissions to update this PR, please contact the submitter to apply the update."
	MsgUpdatePullRequest      = "Error trying to update the PR.\nPlease do it manually."
)

func (s *Server) handleUpdateBranch(ctx context.Context, commenter string, pr *model.PullRequest) error {
	var err error
	var msg string
	defer func(e *error, m *string) {
		if *e != nil {
			s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, *m)
		}
	}(&err, &msg)

	// If the commenter is not the PR submitter, check if the PR submitter is an org member
	if commenter != pr.Username && !s.IsOrgMember(commenter) {
		msg = MsgCommenterPermission
		err = ErrCommenterPermission
		return err
	}

	repoInfo := strings.Split(pr.FullName, "/")
	if repoInfo[0] != s.Config.Org {
		if !pr.MaintainerCanModify {
			msg = MsgOrganizationPermission
			err = ErrOrganizationPermission
			return err
		}
	}

	opt := &github.PullRequestBranchUpdateOptions{
		ExpectedHeadSHA: github.String(pr.Sha),
	}

	_, resp, err2 := s.GithubClient.PullRequests.UpdateBranch(ctx, pr.RepoOwner, pr.RepoName, pr.Number, opt)
	if resp != nil && resp.StatusCode != http.StatusAccepted {
		msg = MsgUpdatePullRequest
		err = ErrUpdatePullRequest
		return err
	}
	if err2 != nil && !strings.Contains("job scheduled on GitHub side; try again later", err2.Error()) {
		msg = MsgUpdatePullRequest
		err = ErrUpdatePullRequest
		return fmt.Errorf("%s: %w", err, err2)
	}

	return nil
}
