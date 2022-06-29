package server

import (
	"context"
	"time"

	"github.com/google/go-github/v39/github"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v6/shared/mlog"
)

const (
	e2eTestFromLabelMsgPRNotMergeable   = "E2E tests not automatically triggered, because the PR is not in a mergeable state. Please update the branch with the base branch and resolve outstanding conflicts."
	e2eTestFromLabelMsgPRHasNoApprovals = "E2E tests not automatically triggered, because PR has no approval yet. Please ask a developer to review and then try again to attach the QA label. "
)

func (e *E2ETestFromLabelError) Error() string {
	switch e.source {
	case e2eTestFromLabelMsgPRNotMergeable:
		return "e2e not running, pr not in mergeable state"
	case e2eTestFromLabelMsgPRHasNoApprovals:
		return "e2e not running, pr has no approval"
	default:
		panic("unhandled error type")
	}
}

type E2ETestFromLabelError struct {
	source string
}

func (s *Server) triggerE2ETestFromLabel(pr *model.PullRequest) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout*time.Second)
	defer cancel()
	var e2eTestFromLabelErr *E2ETestFromLabelError
	defer func() {
		if e2eTestFromLabelErr != nil {
			if err := s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, e2eTestFromLabelErr.source); err != nil {
				mlog.Warn("Error while commenting", mlog.Err(err))
			}
		}
	}()

	prReviews, _, err := s.GithubClient.PullRequests.ListReviews(ctx, pr.RepoOwner, pr.RepoName, pr.Number, nil)
	if err != nil {
		mlog.Error("Error getting reviews for the PR",
			mlog.Int("pr", pr.Number),
			mlog.String("repo", pr.RepoName),
			mlog.Err(err))
		return
	}
	if !hasAtLeastOneApproval(prReviews) {
		e2eTestFromLabelErr = &E2ETestFromLabelError{source: e2eTestFromLabelMsgPRHasNoApprovals}
		mlog.Warn("Not triggering E2E test, due to missing required approvals",
			mlog.Int("pr", pr.Number),
			mlog.String("repo", pr.RepoName),
			mlog.Err(e2eTestFromLabelErr))
		return
	}

	ghPR, _, err := s.GithubClient.PullRequests.Get(ctx, pr.RepoOwner, pr.RepoName, pr.Number)
	if err != nil {
		mlog.Error("Error in getting the PR info",
			mlog.Int("pr", pr.Number),
			mlog.String("repo", pr.RepoName),
			mlog.Err(err))
		return
	}
	if !isMergeable(ghPR) {
		e2eTestFromLabelErr = &E2ETestFromLabelError{source: e2eTestFromLabelMsgPRNotMergeable}
		mlog.Warn("Not triggering E2E test, the PR is not mergeable",
			mlog.Int("pr", pr.Number),
			mlog.String("repo", pr.RepoName),
			mlog.Err(e2eTestFromLabelErr))
		return
	}

	mlog.Debug("Determined that the event should trigger the E2E test",
		mlog.Int("pr", pr.Number),
		mlog.String("repo", pr.RepoName))
	err = s.handleE2ETest(ctx, s.Config.Username, pr, "")
	if err != nil {
		mlog.Error("Error in triggering the E2E test from PR's label event",
			mlog.Int("pr", pr.Number),
			mlog.String("repo", pr.RepoName),
			mlog.Err(err))
	}
}

func hasAtLeastOneApproval(reviews []*github.PullRequestReview) bool {
	for _, review := range reviews {
		mlog.Debug("Checking review state",
			mlog.String("pr", review.GetPullRequestURL()),
			mlog.String("state", review.GetState()))
		if review.GetState() == prReviewApproved {
			return true
		}
	}
	return false
}

func isMergeable(pr *github.PullRequest) bool {
	if pr.GetState() == model.StateOpen && pr.GetMergeableState() == model.MergeableStateClean {
		return true
	}
	return false
}
