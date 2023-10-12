// Copyright (c) 2019-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"fmt"
	"time"

	"github.com/mattermost/mattermost-mattermod/model"

	"github.com/google/go-github/v39/github"
	"github.com/mattermost/mattermost-server/v6/shared/mlog"
)

func (s *Server) AutoMergePR() error {
	mlog.Info("Starting the process to auto merge PRs")
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), defaultCronTaskTimeout*time.Second)
	defer cancel()
	defer func() {
		elapsed := float64(time.Since(start)) / float64(time.Second)
		s.Metrics.ObserveCronTaskDuration("auto_merger_pr", elapsed)
	}()
	prs, err := s.Store.PullRequest().ListOpen()
	if err != nil {
		return fmt.Errorf("error while listing open PRs %w", err)
	}

	for _, pr := range prs {
		var autoMergePr = s.hasAutoMerge(pr.Labels)
		var translationPr = s.isTranslationPr(pr) && s.hasTranslationMergeLabel(pr.Labels)
		if !autoMergePr && !translationPr {
			continue
		}

		ghPR, _, err := s.GithubClient.PullRequests.Get(ctx, pr.RepoOwner, pr.RepoName, pr.Number)
		if err != nil {
			mlog.Error("Error in getting the PR info",
				mlog.Int("pr", pr.Number),
				mlog.String("repo", pr.RepoName),
				mlog.Err(err))
			continue
		}

		if ghPR.GetState() == model.StateClosed {
			continue
		}

		if ghPR.GetMergeableState() != model.MergeableStateClean {
			mlog.Debug("PR is not ready to merge; unclean merge state",
				mlog.Int("pr", pr.Number),
				mlog.String("repo", pr.RepoName),
				mlog.String("mergeableState", ghPR.GetMergeableState()))
			continue
		}

		// Get the Statuses
		prStatus, _, err := s.GithubClient.Repositories.GetCombinedStatus(ctx, pr.RepoOwner, pr.RepoName, ghPR.Head.GetSHA(), nil)
		if err != nil {
			mlog.Error("Error in getting the PR Status",
				mlog.Int("pr", pr.Number),
				mlog.String("repo", pr.RepoName),
				mlog.Err(err))
			continue
		}

		if ghPR.Head.GetSHA() != prStatus.GetSHA() {
			mlog.Error("PR is not ready to merge; mismatch in SHA",
				mlog.Int("pr", pr.Number),
				mlog.String("repo", pr.RepoName),
				mlog.String("SHAFromPR", ghPR.Head.GetSHA()),
				mlog.String("SHAFromStatus", prStatus.GetSHA()))
			continue
		}

		if prStatus.GetState() != stateSuccess {
			for _, status := range prStatus.Statuses {
				mlog.Debug("status",
					mlog.Int("pr", pr.Number),
					mlog.String("repo", pr.RepoName),
					mlog.String("state", status.GetState()),
					mlog.String("description", status.GetDescription()),
					mlog.String("context", status.GetContext()),
					mlog.String("target_url", status.GetTargetURL()),
				)
			}

			mlog.Error("PR is not ready to merge; combined status state is not success",
				mlog.Int("pr", pr.Number),
				mlog.String("repo", pr.RepoName),
				mlog.String("state", prStatus.GetState()))
			continue
		}

		// Check if all reviewers did the review
		prReviewers, _, err := s.GithubClient.PullRequests.ListReviewers(ctx, pr.RepoOwner, pr.RepoName, pr.Number, nil)
		if err != nil {
			mlog.Error("Error to get the reviewers for the PR",
				mlog.Int("pr", pr.Number),
				mlog.String("repo", pr.RepoName),
				mlog.Err(err))
			continue
		}

		if len(prReviewers.Users) != 0 || len(prReviewers.Teams) != 0 {
			mlog.Debug("PR is not ready to merge; pending reviewers",
				mlog.Int("pr", pr.Number),
				mlog.String("repo", pr.RepoName))
			continue
		}

		msg := "Trying to auto merge this PR."
		if err = s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, msg); err != nil {
			mlog.Warn("Error while commenting", mlog.Err(err))
		}

		// All good to merge
		opt := &github.PullRequestOptions{
			SHA:         ghPR.Head.GetSHA(),
			MergeMethod: "squash",
		}

		if translationPr {
			opt.MergeMethod = s.Config.TranslationsMergePolicy
		}

		merged, _, err := s.GithubClient.PullRequests.Merge(ctx, pr.RepoOwner, pr.RepoName, pr.Number, "Automatic Merge", opt)
		if err != nil {
			errMsg := fmt.Sprintf("Error while trying to automerge the PR\nErr %s", err.Error())
			if err = s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, errMsg); err != nil {
				mlog.Warn("Error while commenting", mlog.Err(err))
			}
			if translationPr {
				if err = s.removeTranslationLabel(ctx, pr); err != nil {
					mlog.Warn("Error while removing translation label", mlog.Err(err))
				}
				if err = s.sendTranslationWebhookMessage(ctx, pr, s.Config.TranslationsMergeFailureMessage); err != nil {
					mlog.Warn("Error while sending failure message to mattermost", mlog.Err(err))
				}
			}
			continue
		}

		msg = fmt.Sprintf("%s\nSHA: %s", merged.GetMessage(), merged.GetSHA())
		if err = s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, msg); err != nil {
			mlog.Warn("Error while commenting", mlog.Err(err))
		}

		if translationPr {
			if err = s.removeTranslationLabel(ctx, pr); err != nil {
				mlog.Warn("Error while removing translation label", mlog.Err(err))
			}
			if err = s.sendTranslationWebhookMessage(ctx, pr, s.Config.TranslationsMergedMessage); err != nil {
				mlog.Warn("Error while sending translations merged message to mattermost", mlog.Err(err))
			}
		}
	}

	mlog.Info("Done with the process to auto merge PRs")
	return nil
}

func (s *Server) hasAutoMerge(labels []string) bool {
	for _, label := range labels {
		if label == s.Config.AutoPRMergeLabel {
			return true
		}
	}
	return false
}
