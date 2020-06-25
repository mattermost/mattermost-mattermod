// Copyright (c) 2019-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"fmt"

	"github.com/google/go-github/v31/github"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v5/mlog"
)

func (s *Server) AutoMergePR() {
	mlog.Info("Starting the process to auto merge PRs")
	prs, err := s.Store.PullRequest().ListOpen()
	if err != nil {
		mlog.Error(err.Error())
		return
	}

	for _, pr := range prs {
		if !s.isAutoMergeLabelInLabels(pr.Labels) {
			mlog.Info("No auto merge label for this PR", mlog.Int("pr", pr.Number), mlog.String("repo", pr.RepoName))
			continue
		}

		prToMerge, _, err := s.GithubClient.PullRequests.Get(context.Background(), pr.RepoOwner, pr.RepoName, pr.Number)
		if err != nil {
			mlog.Error("Error in getting the PR info", mlog.Int("pr", pr.Number), mlog.String("repo", pr.RepoName), mlog.Err(err))
			continue
		}

		if prToMerge.GetState() == model.StateClosed {
			continue
		}

		if prToMerge.GetMergeableState() != "clean" {
			mlog.Info("PR is not ready to merge, does not have a good merge state", mlog.Int("pr", pr.Number), mlog.String("repo", pr.RepoName), mlog.String("mergeableState", prToMerge.GetMergeableState()))
			continue
		}

		// Get the Statuses
		PRStatus, _, err := s.GithubClient.Repositories.GetCombinedStatus(context.Background(), pr.RepoOwner, pr.RepoName, prToMerge.Head.GetSHA(), nil)
		if err != nil {
			mlog.Error("Error in getting the PR Status", mlog.Int("pr", pr.Number), mlog.String("repo", pr.RepoName), mlog.Err(err))
			continue
		}

		if prToMerge.Head.GetSHA() != PRStatus.GetSHA() {
			mlog.Error("PR is not ready to merge, missmatch SHA", mlog.Int("pr", pr.Number), mlog.String("repo", pr.RepoName), mlog.String("SHAFromPR", prToMerge.Head.GetSHA()), mlog.String("SHAFromStatus", PRStatus.GetSHA()))
			continue
		}

		if PRStatus.GetState() != stateSuccess {
			mlog.Error("PR is not ready to merge, not ready state", mlog.Int("pr", pr.Number), mlog.String("repo", pr.RepoName), mlog.String("state", PRStatus.GetState()))
			continue
		}

		// Check if all reviewers did the review
		prReviewers, _, err := s.GithubClient.PullRequests.ListReviewers(context.Background(), pr.RepoOwner, pr.RepoName, pr.Number, nil)
		if err != nil {
			mlog.Error("Error to get the Reviewers for a PR", mlog.Int("pr", pr.Number), mlog.String("repo", pr.RepoName), mlog.Err(err))
			continue
		}

		if len(prReviewers.Users) != 0 || len(prReviewers.Teams) != 0 {
			mlog.Info("Still some pending reviewers", mlog.Int("pr", pr.Number), mlog.String("repo", pr.RepoName))
			continue
		}

		msg := "Trying to auto merge this PR."
		s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number, msg)

		// All good to merge
		opt := &github.PullRequestOptions{
			SHA:         prToMerge.Head.GetSHA(),
			MergeMethod: "squash",
		}

		merged, _, err := s.GithubClient.PullRequests.Merge(context.Background(), pr.RepoOwner, pr.RepoName, pr.Number, "Automatic Merge", opt)
		if err != nil {
			errMsg := fmt.Sprintf("Error while trying to automerge the PR\nErr %s", err.Error())
			s.removeLabel(pr.RepoOwner, pr.RepoName, pr.Number, s.Config.AutoPRMergeLabel)
			s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number, errMsg)
			continue
		}

		msg = fmt.Sprintf("%s\nSHA: %s", merged.GetMessage(), merged.GetSHA())
		s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number, msg)
		s.removeLabel(pr.RepoOwner, pr.RepoName, pr.Number, s.Config.AutoPRMergeLabel)
	}

	mlog.Info("Done the process to auto merge PRs")
}

func (s *Server) isAutoMergeLabelInLabels(labels []string) bool {
	for _, label := range labels {
		if label == s.Config.AutoPRMergeLabel {
			return true
		}
	}
	return false
}
