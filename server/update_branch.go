package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v5/mlog"

	"github.com/google/go-github/v32/github"
)

const (
	msgCommenterPermission    = "Looks like you don't have permissions to trigger this command.\n Only available for the PR submitter and org members"
	msgOrganizationPermission = "We don't have permissions to update this PR, please contact the submitter to apply the update."
	msgUpdatePullRequest      = "Error trying to update the PR.\nPlease do it manually."
)

type updateError struct {
	source string
}

func (e *updateError) Error() string {
	switch e.source {
	case msgCommenterPermission:
		return "commenter does not have permissions"
	case msgOrganizationPermission:
		return "we don't have permissions"
	case msgUpdatePullRequest:
		return "could not update pull request"
	default:
		panic("unhandled error type")
	}
}

func (s *Server) handleUpdateBranch(ctx context.Context, commenter string, pr *model.PullRequest) error {
	var uerr *updateError
	defer func() {
		if uerr != nil {
			if err := s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, uerr.source); err != nil {
				mlog.Warn("Error while commenting", mlog.Err(err))
			}
		}
	}()

	// If the commenter is not the PR submitter, check if the PR submitter is an org member
	if commenter != pr.Username && !s.IsOrgMember(commenter) {
		uerr = &updateError{source: msgCommenterPermission}
		return uerr
	}

	repoInfo := strings.Split(pr.FullName, "/")
	if repoInfo[0] != s.Config.Org {
		if !pr.GetMaintainerCanModify() {
			uerr = &updateError{source: msgOrganizationPermission}
			return uerr
		}
	}

	opt := &github.PullRequestBranchUpdateOptions{
		ExpectedHeadSHA: github.String(pr.Sha),
	}

	_, resp, err := s.GithubClient.PullRequests.UpdateBranch(ctx, pr.RepoOwner, pr.RepoName, pr.Number, opt)
	if resp != nil && resp.StatusCode != http.StatusAccepted {
		uerr = &updateError{source: msgUpdatePullRequest}
		return uerr
	}
	if err != nil && !strings.Contains("job scheduled on GitHub side; try again later", err.Error()) {
		uerr = &updateError{source: msgUpdatePullRequest}
		return fmt.Errorf("%s: %w", uerr, err)
	}

	return nil
}
