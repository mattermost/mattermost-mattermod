package server

import (
	"context"
	"net/http"
	"strings"

	"github.com/mattermost/mattermost-mattermod/model"

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
	var e *updateError
	defer func() {
		if e != nil {
			s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, e.source)
		}
	}()

	// If the commenter is not the PR submitter, check if the PR submitter is an org member
	if commenter != pr.Username && !s.IsOrgMember(commenter) {
		e = &updateError{msgCommenterPermission}
		return e
	}

	repoInfo := strings.Split(pr.FullName, "/")
	if repoInfo[0] != s.Config.Org {
		if !pr.MaintainerCanModify.Valid || !pr.MaintainerCanModify.Bool {
			e = &updateError{msgOrganizationPermission}
			return e
		}
	}

	opt := &github.PullRequestBranchUpdateOptions{
		ExpectedHeadSHA: github.String(pr.Sha),
	}

	_, resp, err := s.GithubClient.PullRequests.UpdateBranch(ctx, pr.RepoOwner, pr.RepoName, pr.Number, opt)
	if resp != nil && resp.StatusCode != http.StatusAccepted {
		e = &updateError{msgUpdatePullRequest}
		return e
	}
	if err != nil && !strings.Contains("job scheduled on GitHub side; try again later", err.Error()) {
		e = &updateError{msgUpdatePullRequest}
		return err
	}

	return nil
}
