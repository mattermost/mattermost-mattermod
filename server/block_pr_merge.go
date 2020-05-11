// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"fmt"

	"github.com/google/go-github/v28/github"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v5/mlog"
)

func (s *Server) blockPRMerge(pr *model.PullRequest) {
	if pr.State == "closed" {
		return
	}

	mergeStatus := &github.RepoStatus{
		Context:     github.String("merge/blocked"),
		State:       github.String("pending"),
		Description: github.String(fmt.Sprintf("Merge blocked due %s label", s.getBlockLabelFromPR(pr.Labels))),
		TargetURL:   github.String(""),
	}

	mlog.Info("will block PR merge status", mlog.Int("pr", pr.Number), mlog.String("repo", pr.RepoName))
	_, _, errStatus := s.GithubClient.Repositories.CreateStatus(context.Background(), pr.RepoOwner, pr.RepoName, pr.Sha, mergeStatus)
	if errStatus != nil {
		mlog.Error("Unable to create the github status for for PR", mlog.Int("pr", pr.Number), mlog.Err(errStatus))
		return
	}
}

func (s *Server) getBlockLabelFromPR(prLabels []string) string {
	for _, blockLabel := range s.Config.BlockPRMergeLabels {
		for _, prLabel := range prLabels {
			if prLabel == blockLabel {
				return prLabel
			}
		}
	}
	return ""
}

func (s *Server) unblockPRMerge(pr *model.PullRequest) {
	if pr.State == "closed" {
		return
	}

	mergeStatus := &github.RepoStatus{
		Context:     github.String("merge/blocked"),
		State:       github.String("success"),
		Description: github.String("Merged allowed"),
		TargetURL:   github.String(""),
	}

	mlog.Info("will unblock PR merge status", mlog.Int("pr", pr.Number), mlog.String("repo", pr.RepoName))
	_, _, errStatus := s.GithubClient.Repositories.CreateStatus(context.Background(), pr.RepoOwner, pr.RepoName, pr.Sha, mergeStatus)
	if errStatus != nil {
		mlog.Error("Unable to create the github status for for PR", mlog.Int("pr", pr.Number), mlog.Err(errStatus))
		return
	}
}
