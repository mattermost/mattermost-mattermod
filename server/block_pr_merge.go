// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"fmt"

	"github.com/google/go-github/v93/github"
	"github.com/mattermost/mattermost/server/public/shared/mlog"

	"github.com/mattermost/mattermost-mattermod/model"
)

func (s *Server) blockPRMerge(ctx context.Context, pr *model.PullRequest) error {
	if pr.State == model.StateClosed {
		return nil
	}

	mergeStatus := github.RepoStatus{
		Context:     new("merge/blocked"),
		State:       new(statePending),
		Description: new(fmt.Sprintf("Merge blocked due %s label", s.getBlockLabelFromPR(pr.Labels))),
		TargetURL:   new(""),
	}

	mlog.Info("will block PR merge status", mlog.Int("pr", pr.Number), mlog.String("repo", pr.RepoName))
	_, _, err := s.GithubClient.Repositories.CreateStatus(ctx, pr.RepoOwner, pr.RepoName, pr.Sha, mergeStatus)
	return err
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

func (s *Server) unblockPRMerge(ctx context.Context, pr *model.PullRequest) error {
	if pr.State == model.StateClosed {
		return nil
	}

	mergeStatus := github.RepoStatus{
		Context:     new("merge/blocked"),
		State:       new(stateSuccess),
		Description: new("Merged allowed"),
		TargetURL:   new(""),
	}

	mlog.Info("will unblock PR merge status", mlog.Int("pr", pr.Number), mlog.String("repo", pr.RepoName))
	_, _, err := s.GithubClient.Repositories.CreateStatus(ctx, pr.RepoOwner, pr.RepoName, pr.Sha, mergeStatus)
	return err
}
