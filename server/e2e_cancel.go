package server

import (
	"context"
	"fmt"

	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v5/mlog"
)

const (
	e2eCancelMsgCommenterPermission = "Looks like you don't have permissions to trigger this command.\n Only available for org members"
	e2eCancelMsgFailedCancellation  = "Looks like cancellation failed. Sorry about that."
	e2eCancelMsgNothingToCancel     = "Looks like nothing had to be canceled. "
)

type E2ECancelError struct {
	source string
}

func (e *E2ECancelError) Error() string {
	switch e.source {
	case e2eCancelMsgCommenterPermission:
		return commenterNoPermissions
	case e2eCancelMsgFailedCancellation:
		return "could not cancel"
	case e2eCancelMsgNothingToCancel:
		return "no pipeline to cancel"
	default:
		panic("unhandled error type")
	}
}

func (s *Server) handleE2ECancel(ctx context.Context, commenter string, pr *model.PullRequest) error {
	var e2eErr *E2ECancelError
	defer func() {
		if e2eErr != nil {
			if err := s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, e2eErr.source); err != nil {
				mlog.Warn("Error while commenting", mlog.Err(err))
			}
		}
	}()
	prRepoOwner, prRepoName, prNumber := pr.RepoOwner, pr.RepoName, pr.Number
	if !s.IsOrgMember(commenter) {
		mlog.Warn("E2E cancellation tried by non org member")
		e2eErr = &E2ECancelError{source: e2eCancelMsgCommenterPermission}
		return e2eErr
	}
	var e2eProjectRef string
	switch pr.RepoName {
	case s.Config.E2EWebappReponame:
		e2eProjectRef = s.Config.E2EWebappRef
	case s.Config.E2EServerReponame:
		e2eProjectRef = s.Config.E2EWebappRef
	}
	urls, err := s.cancelPipelinesForPR(ctx, &e2eProjectRef, &pr.Number)
	if err != nil {
		mlog.Warn("E2E cancellation failed")
		e2eErr = &E2ECancelError{source: e2eCancelMsgFailedCancellation}
		return e2eErr
	}

	if urls == nil {
		mlog.Warn("E2E cancellation has no cancellable pipeline")
		e2eErr = &E2ECancelError{source: e2eCancelMsgNothingToCancel}
		return e2eErr
	}
	var fURLs string
	for _, url := range urls {
		fURLs += *url + "\n"
	}
	endMsg := fmt.Sprintf("Successfully canceled pipelines:\n%v", fURLs)
	if cErr := s.sendGitHubComment(ctx, prRepoOwner, prRepoName, prNumber, endMsg); cErr != nil {
		mlog.Warn("Error while commenting", mlog.Err(cErr))
	}

	return nil
}
