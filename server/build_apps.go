// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"fmt"
	"time"

	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v6/shared/mlog"
)

func (s *Server) buildApp(pr *model.PullRequest) {
	// This needs its own context because is executing a heavy job
	ctx, cancel := context.WithTimeout(context.Background(), defaultBuildAppTimeout*time.Second)
	defer cancel()

	prRepoOwner, prRepoName, prNumber := pr.RepoOwner, pr.RepoName, pr.Number
	// will generate the string refs/heads/build-pr-1222-8bfcb54
	ref := fmt.Sprintf("refs/heads/%s%d-%s", s.Config.BuildAppBranchPrefix, prNumber, pr.Sha[0:7])
	isReadyToBeBuilt, err := s.areChecksSuccessfulForPR(ctx, pr)
	if err != nil {
		msg := fmt.Sprintf("Failed to retrieve the status of the PR. Error:  \n```%s```", err.Error())
		if cErr := s.sendGitHubComment(ctx, prRepoOwner, prRepoName, prNumber, msg); cErr != nil {
			mlog.Warn("Error while commenting", mlog.Err(cErr))
		}
		return
	}

	if isReadyToBeBuilt {
		exists, err := s.checkIfRefExists(ctx, pr, s.Config.Org, ref)
		if err != nil {
			msg := fmt.Sprintf("Failed to check ref. @mattermost/core-build-engineers have been notified. Error \n```%s```", err.Error())
			if cErr := s.sendGitHubComment(ctx, prRepoOwner, prRepoName, prNumber, msg); cErr != nil {
				mlog.Warn("Error while commenting", mlog.Err(cErr))
			}
			return
		}

		if exists {
			err = s.deleteRef(ctx, s.Config.Org, prRepoName, ref)
			if err != nil {
				msg := fmt.Sprintf("Failed to delete already existing build branch. @mattermost/core-build-engineers have been notified. Error \n```%s```", err.Error())
				if cErr := s.sendGitHubComment(ctx, prRepoOwner, prRepoName, prNumber, msg); cErr != nil {
					mlog.Warn("Error while commenting", mlog.Err(cErr))
				}
				return
			}
		}

		s.createRef(ctx, pr, ref)
		if cErr := s.sendGitHubComment(ctx, prRepoOwner, prRepoName, prNumber, s.Config.BuildAppInitMessage); cErr != nil {
			mlog.Warn("Error while commenting", mlog.Err(cErr))
		}

		err = s.deleteRefWhereCombinedStateEqualsSuccess(ctx, s.Config.Org, prRepoName, ref)
		if err != nil {
			msg := fmt.Sprintf("Failed to delete ref. @mattermost/core-build-engineers have been notified. Error \n```%s```", err.Error())
			if cErr := s.sendGitHubComment(ctx, prRepoOwner, prRepoName, prNumber, msg); cErr != nil {
				mlog.Warn("Error while commenting", mlog.Err(cErr))
			}
		}
	} else {
		msg := "Not triggering the mobile app build workflow, because PR checks are failing. "
		if cErr := s.sendGitHubComment(ctx, prRepoOwner, prRepoName, prNumber, msg); cErr != nil {
			mlog.Warn("Error while commenting", mlog.Err(cErr))
		}
	}
}
