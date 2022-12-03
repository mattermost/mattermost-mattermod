package server

import (
	"context"
	"fmt"

	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v6/shared/mlog"
)

const (
	e2eTestMobileMsgCommenterPermission = "You don't have permissions to trigger this command.\n It's only available for organization members."
	e2eTestMobileMsgCIFailing           = "The command /e2e-test requires all PR checks to pass."
	e2eTestMobileMsgUnknownPRState      = "Failed to check whether PR checks passed. E2E testing isn't triggered. Please retry later."
	e2eTestMobileMsgPRInfo              = "Failed to get the PR information required to trigger testing. Please retry later."
	e2eTestMobileMsgTrigger             = "Failed to trigger E2E mobile testing pipeline."
	e2eTestMobileMsgSuccess             = "Successfully triggered e2e mobile testing!"
	e2eTestMobileFmtSuccess             = "%v\n%v"
)

func (e *E2ETestMobileError) Error() string {
	switch e.source {
	case e2eTestMobileMsgCommenterPermission:
		return "commenter does not have permissions for e2e test mobile"
	case e2eTestMobileMsgCIFailing:
		return "PR checks needs to be passing for e2e test mobile"
	case e2eTestMobileMsgUnknownPRState:
		return "unknown PR state for e2e test mobile"
	case e2eTestMobileMsgPRInfo:
		return "could not fetch PR info for e2e test mobile"
	case e2eTestMobileMsgTrigger:
		return "could not trigger pipeline for e2e test mobile"
	default:
		panic("unhandled error type")
	}
}

type E2ETestMobileError struct {
	source string
}

type E2ETestMobileTriggerInfo struct {
	TriggerBranch string
	TriggerPR     int
	TriggerRepo   string
	TriggerSHA    string
	RefToTrigger  string
	EnvVars       map[string]string
}

func (s *Server) handleE2ETestMobile(ctx context.Context, commenter string, pr *model.PullRequest) error {
	var e2eTestErr *E2ETestMobileError
	defer func() {
		if e2eTestErr != nil {
			if err := s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, e2eTestErr.source); err != nil {
				mlog.Warn("Error while commenting", mlog.Err(err))
			}
		}
	}()
	if !s.IsOrgMember(commenter) {
		e2eTestErr = &E2ETestMobileError{source: e2eTestMobileMsgCommenterPermission}
		return e2eTestErr
	}
	prRepoOwner, prRepoName, prNumber := pr.RepoOwner, pr.RepoName, pr.Number

	isCIPassing, err := s.areChecksSuccessfulForPR(ctx, pr)
	if err != nil {
		e2eTestErr = &E2ETestMobileError{source: e2eTestMobileMsgUnknownPRState}
		return e2eTestErr
	}
	if !isCIPassing {
		e2eTestErr = &E2ETestMobileError{source: e2eTestMobileMsgCIFailing}
		return e2eTestErr
	}

	info := &E2ETestMobileTriggerInfo{
		TriggerBranch: pr.Ref,
		TriggerPR:     pr.Number,
		TriggerRepo:   pr.RepoName,
		TriggerSHA:    pr.Sha,
	}

	info.RefToTrigger = "main"

	url, err := s.triggerE2EMobileGitLabPipeline(ctx, info)
	if err != nil {
		e2eTestErr = &E2ETestMobileError{source: e2eTestMobileMsgTrigger}
		return e2eTestErr
	}
	endMsg := fmt.Sprintf(e2eTestMobileFmtSuccess, e2eTestMobileMsgSuccess, url)
	if cErr := s.sendGitHubComment(ctx, prRepoOwner, prRepoName, prNumber, endMsg); cErr != nil {
		mlog.Warn("Error while commenting", mlog.Err(cErr))
	}

	return nil
}
