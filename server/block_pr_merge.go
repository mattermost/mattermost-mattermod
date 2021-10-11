// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"fmt"

	"github.com/google/go-github/v39/github"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v5/mlog"
)

func (s *Server) blockPRMerge(ctx context.Context, pr *model.PullRequest) error {
	if pr.State == model.StateClosed {
		return nil
	}

	mergeStatus := &github.RepoStatus{
		Context:     github.String("merge/blocked"),
		State:       github.String(statePending),
		Description: github.String(fmt.Sprintf("Merge blocked due %s label", s.getBlockLabelFromPR(pr.Labels))),
		TargetURL:   github.String(""),
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

	mergeStatus := &github.RepoStatus{
		Context:     github.String("merge/blocked"),
		State:       github.String(stateSuccess),
		Description: github.String("Merged allowed"),
		TargetURL:   github.String(""),
	}

	mlog.Info("will unblock PR merge status", mlog.Int("pr", pr.Number), mlog.String("repo", pr.RepoName))
	_, _, err := s.GithubClient.Repositories.CreateStatus(ctx, pr.RepoOwner, pr.RepoName, pr.Sha, mergeStatus)
	return err
}
