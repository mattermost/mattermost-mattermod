// Copyright (c) 2019-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"fmt"
	"time"

	"github.com/mattermost/mattermost-mattermod/model"

	"github.com/google/go-github/v32/github"
	"github.com/mattermost/mattermost-server/v5/mlog"
)

func (s *Server) AutoMergePR() error {
	mlog.Info("Starting the process to auto merge PRs")
	ctx, cancel := context.WithTimeout(context.Background(), defaultCronTaskTimeout*time.Second)
	defer cancel()
	prs, err := s.Store.PullRequest().ListOpen()
	if err != nil {
		return fmt.Errorf("error while listing open PRs %w", err)
	}

	for _, pr := range prs {
		if !s.hasAutoMerge(pr.Labels) {
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

		if ghPR.GetMergeableState() != "clean" {
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
		s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, msg)

		// All good to merge
		opt := &github.PullRequestOptions{
			SHA:         ghPR.Head.GetSHA(),
			MergeMethod: "squash",
		}

		merged, _, err := s.GithubClient.PullRequests.Merge(ctx, pr.RepoOwner, pr.RepoName, pr.Number, "Automatic Merge", opt)
		if err != nil {
			errMsg := fmt.Sprintf("Error while trying to automerge the PR\nErr %s", err.Error())
			s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, errMsg)
			continue
		}

		msg = fmt.Sprintf("%s\nSHA: %s", merged.GetMessage(), merged.GetSHA())
		s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, msg)
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
