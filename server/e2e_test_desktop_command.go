package server

import (
	"context"
	"fmt"

	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v6/shared/mlog"
)

const (
	e2eTestDesktopMsgCommenterPermission = "You don't have permissions to trigger this command.\n It's only available for organization members."
	e2eTestDesktopMsgCIFailing           = "The command /e2e-test requires all PR checks to pass."
	e2eTestDesktopMsgUnknownPRState      = "Failed to check whether PR checks passed. E2E testing isn't triggered. Please retry later."
	e2eTestDesktopMsgPRInfo              = "Failed to get the PR information required to trigger testing. Please retry later."
	e2eTestDesktopMsgTrigger             = "Failed to trigger E2E desktop testing pipeline."
	e2eTestDesktopMsgSuccess             = "Successfully triggered e2e desktop testing!"
	e2eTestDesktopFmtSuccess             = "%v\n%v"
)

func (e *E2ETestDesktopError) Error() string {
	switch e.source {
	case e2eTestDesktopMsgCommenterPermission:
		return "commenter does not have permissions for e2e test desktop"
	case e2eTestDesktopMsgCIFailing:
		return "PR checks needs to be passing for e2e test desktop"
	case e2eTestDesktopMsgUnknownPRState:
		return "unknown PR state for e2e test desktop"
	case e2eTestDesktopMsgPRInfo:
		return "could not fetch PR info for e2e test desktop"
	case e2eTestDesktopMsgTrigger:
		return "could not trigger pipeline for e2e test desktop"
	default:
		panic("unhandled error type")
	}
}

type E2ETestDesktopError struct {
	source string
}

type E2ETestDesktopTriggerInfo struct {
	TriggerBranch string
	TriggerPR     int
	TriggerRepo   string
	TriggerSHA    string
	RefToTrigger  string
	EnvVars       map[string]string
}

func (s *Server) handleE2ETestDesktop(ctx context.Context, commenter string, pr *model.PullRequest) error {
	var e2eTestErr *E2ETestDesktopError
	defer func() {
		if e2eTestErr != nil {
			if err := s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, e2eTestErr.source); err != nil {
				mlog.Warn("Error while commenting", mlog.Err(err))
			}
		}
	}()
	if !s.IsOrgMember(commenter) || s.IsInBotBlockList(commenter) {
		e2eTestErr = &E2ETestDesktopError{source: e2eTestDesktopMsgCommenterPermission}
		return e2eTestErr
	}
	prRepoOwner, prRepoName, prNumber := pr.RepoOwner, pr.RepoName, pr.Number

	isCIPassing, err := s.areChecksSuccessfulForPR(ctx, pr)
	if err != nil {
		e2eTestErr = &E2ETestDesktopError{source: e2eTestDesktopMsgUnknownPRState}
		return e2eTestErr
	}
	if !isCIPassing {
		e2eTestErr = &E2ETestDesktopError{source: e2eTestDesktopMsgCIFailing}
		return e2eTestErr
	}

	info := &E2ETestDesktopTriggerInfo{
		TriggerBranch: pr.Ref,
		TriggerPR:     pr.Number,
		TriggerRepo:   pr.RepoName,
		TriggerSHA:    pr.Sha,
	}

	info.RefToTrigger = s.Config.E2EDesktopGitLabProjectRefForPR

	url, err := s.triggerE2EDesktopGitLabPipeline(ctx, info)
	if err != nil {
		e2eTestErr = &E2ETestDesktopError{source: e2eTestDesktopMsgTrigger}
		return e2eTestErr
	}
	endMsg := fmt.Sprintf(e2eTestDesktopFmtSuccess, e2eTestDesktopMsgSuccess, url)
	if cErr := s.sendGitHubComment(ctx, prRepoOwner, prRepoName, prNumber, endMsg); cErr != nil {
		mlog.Warn("Error while commenting", mlog.Err(cErr))
	}

	return nil
}
