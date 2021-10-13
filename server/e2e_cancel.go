package server

import (
	"context"
	"time"

	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v5/mlog"
)

const (
	e2eCancelMsgCommenterPermission = "Looks like you don't have permissions to trigger this command.\n Only available for org members"
)

type e2eCancelError struct {
	source string
}

func (e *e2eCancelError) Error() string {
	switch e.source {
	case e2eCancelMsgCommenterPermission:
		return commenterNoPermissions
	default:
		panic("unhandled error type")
	}
}

func (s *Server) handleE2ECancel(ctx context.Context, commenter string, pr *model.PullRequest, commentBody string) error {
	var e2eErr *e2eCancelError
	defer func() {
		if e2eErr != nil {
			if err := s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, e2eErr.source); err != nil {
				mlog.Warn("Error while commenting", mlog.Err(err))
			}
		}
	}()
	if !s.IsOrgMember(commenter) {
		mlog.Warn("E2E triggering tried by non org member")
		e2eErr = &e2eCancelError{source: e2eCancelMsgCommenterPermission}
		return e2eErr
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout*time.Second)
	defer cancel()
	// prRepoOwner, prRepoName, prNumber := pr.RepoOwner, pr.RepoName, pr.Number

	return nil
}
