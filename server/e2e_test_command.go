package server

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v5/mlog"
)

const (
	e2eTestMsgCommenterPermission = "Looks like you don't have permissions to trigger this command.\n Only available for org members"
	e2eTestMsgCIFailing           = "Command e2e-test requires PR checks to pass."
	e2eTestMsgUnknownPRState      = "Failed to check if PR checks passed. Will not try to trigger e2e testing. Please retry in a bit."
)

type e2eTestError struct {
	source string
}

func (e *e2eTestError) Error() string {
	switch e.source {
	case e2eTestMsgCommenterPermission:
		return "commenter does not have permissions"
	case e2eTestMsgCIFailing:
		return "PR checks needs to be passing"
	case e2eTestMsgUnknownPRState:
		return "unknown PR state"
	default:
		panic("unhandled error type")
	}
}

func (s *Server) handleE2ETest(ctx context.Context, commenter string, pr *model.PullRequest, commentBody string) error {
	var e2eTestErr *e2eTestError
	defer func() {
		if e2eTestErr != nil {
			if err := s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, e2eTestErr.source); err != nil {
				mlog.Warn("Error while commenting", mlog.Err(err))
			}
		}
	}()
	if !s.IsOrgMember(commenter) {
		e2eTestErr = &e2eTestError{source: e2eTestMsgCommenterPermission}
		return e2eTestErr
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultE2ETestTimeout*time.Second)
	defer cancel()
	prRepoOwner, prRepoName, prNumber := pr.RepoOwner, pr.RepoName, pr.Number

	isCIPassing, err := s.areChecksSuccessfulForPR(ctx, pr)
	if err != nil {
		e2eTestErr = &e2eTestError{source: e2eTestMsgUnknownPRState}
		return fmt.Errorf("%s: %w", e2eTestErr, err)
	}
	if !isCIPassing {
		e2eTestErr = &e2eTestError{source: e2eTestMsgCIFailing}
		return e2eTestErr
	}

	opts := parseE2ETestCommentForOpts(commentBody)
	var optMsg string
	for _, m := range opts {
		for k, v := range m {
			optMsg += fmt.Sprintf("%v=%v\n", k, v)
		}
	}
	msg := fmt.Sprintf("Triggering e2e testing with options: ```\n%v\n```", optMsg)
	if cErr := s.sendGitHubComment(ctx, prRepoOwner, prRepoName, prNumber, msg); cErr != nil {
		mlog.Warn("Error while commenting", mlog.Err(cErr))
	}
	return nil
}

func parseE2ETestCommentForOpts(commentBody string) map[string]string {
	commentBody = strings.Split(commentBody, "\n")[0]
	commentBody = strings.TrimSuffix(commentBody, " ")

	if !strings.Contains(commentBody, " ") && !strings.Contains(commentBody, "=") {
		mlog.Debug("E2E comment does not contain options")
		return nil
	}

	var opts = make(map[string]string)
	for _, envAssignment := range strings.Split(commentBody, " ")[1:] {
		sAssignment := strings.SplitAfterN(envAssignment, string('='), 2)
		sAssignment[0] = strings.TrimSuffix(sAssignment[0], "=")
		if _, exists := opts[sAssignment[0]]; exists {
			break
		}
		envVar, envValue := sAssignment[0], sAssignment[1]
		opts[envVar] = envValue
	}

	return opts
}
