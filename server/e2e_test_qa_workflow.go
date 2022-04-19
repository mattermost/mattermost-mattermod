package server

import (
	"context"

	"github.com/google/go-github/v39/github"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v5/mlog"
)

const (
	e2eTestQAMsgPRNotMergeable   = "E2E tests not automatically triggered, because the PR is not in a mergeable state. Please update the branch with the base branch and resolve outstanding conflicts."
	e2eTestQAMsgPRHasNoApprovals = "E2E tests not automatically triggered, because PR has no approval yet. Please ask a developer to review and then try again to attach the QA label. "
)

func (e *E2ETestQAError) Error() string {
	switch e.source {
	case e2eTestQAMsgPRNotMergeable:
		return "e2e not running, pr not in mergeable state"
	case e2eTestQAMsgPRHasNoApprovals:
		return "e2e not running, pr has no approval"
	default:
		panic("unhandled error type")
	}
}

type E2ETestQAError struct {
	source string
}

func (s *Server) triggerE2ETestFromPRChange(ctx context.Context, pr *model.PullRequest) error {
	var e2eTestQAErr *E2ETestQAError
	defer func() {
		if e2eTestQAErr != nil {
			if err := s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, e2eTestQAErr.source); err != nil {
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
		return err
	}
	if !hasMoreThanOneApproval(prReviews) {
		e2eTestQAErr = &E2ETestQAError{source: e2eTestQAMsgPRHasNoApprovals}
		return e2eTestQAErr
	}

	ghPR, _, err := s.GithubClient.PullRequests.Get(ctx, pr.RepoOwner, pr.RepoName, pr.Number)
	if err != nil {
		mlog.Error("Error in getting the PR info",
			mlog.Int("pr", pr.Number),
			mlog.String("repo", pr.RepoName),
			mlog.Err(err))
		return err
	}
	if !isMergeable(ghPR) {
		e2eTestQAErr = &E2ETestQAError{source: e2eTestQAMsgPRNotMergeable}
		return e2eTestQAErr
	}

	mlog.Debug("Determined that the event should trigger the E2E test",
		mlog.Int("pr", pr.Number),
		mlog.String("repo", pr.RepoName))
	err = s.handleE2ETest(ctx, s.Config.Username, pr, "")
	if err != nil {
		mlog.Error("Error in triggering the E2E test from PR event",
			mlog.Int("pr", pr.Number),
			mlog.String("repo", pr.RepoName),
			mlog.Err(err))
		return err
	}
	return nil
}

func hasMoreThanOneApproval(reviews []*github.PullRequestReview) bool {
	for _, review := range reviews {
		if *review.State == "approved" {
			return true
		}
	}
	return false
}

func isMergeable(pr *github.PullRequest) bool {
	if pr.GetState() == "open" && pr.GetMergeableState() == "clean" {
		return true
	}
	return false
}
