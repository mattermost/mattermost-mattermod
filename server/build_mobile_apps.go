// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v5/mlog"
	"strconv"
	"time"
)

func (s *Server) buildMobileApp(pr *model.PullRequest) {
	prRepoOwner, prRepoName, prNumber := pr.RepoOwner, pr.RepoName, pr.Number
	ref := "refs/heads/"+s.Config.BuildMobileAppBranchPrefix + strconv.Itoa(prNumber)

	isReadyToBeBuilt, err := s.areChecksSuccessfulForPr(pr)
	if err != nil {
		s.sendGitHubComment(prRepoOwner, prRepoName, prNumber,"Failed to retrieve the status of the PR. Error:  \n```"+err.Error()+"```",)
		return
	}

	if isReadyToBeBuilt {
		exists, _ := s.checkIfRefExists(pr, s.Config.Username, ref)
		if exists {
			err := s.deleteRef(s.Config.Username, prRepoName, ref)
			if err != nil {
				s.sendGitHubComment(prRepoOwner, prRepoName, prNumber,"Failed to delete already existing build branch. @mattermost/core-build-engineers have been notified. Error:  \n```"+err.Error()+"```",)
				return
			}
		}

		s.createRef(pr, ref)
		s.sendGitHubComment(prRepoOwner, prRepoName, prNumber, s.Config.BuildMobileAppInitMessage,)

		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
		defer cancel()

		buildLink, buildNumber, err := s.waitForBuildLink(ctx, pr, s.Config.Username)
		if err != nil {
			mlog.Err(err)
			s.sendGitHubComment(prRepoOwner, prRepoName, prNumber,"Failed retrieving build link. @mattermost/core-build-engineers have been notified. Error:  \n```"+err.Error()+"```",)
			return
		}
		s.sendGitHubComment(prRepoOwner, prRepoName, prNumber, "Successfully building: " + buildLink,)

		artifactLinks, err := s.waitForArtifactLinks(ctx, pr, s.Config.Username, buildNumber)
		if err != nil {
			s.sendGitHubComment(prRepoOwner, prRepoName, prNumber,"Failed retrieving artifact links. @mattermost/core-build-engineers have been notified. Error:  \n```"+err.Error()+"```",)
			return
		}
		s.sendGitHubComment(prRepoOwner, prRepoName, prNumber, "Artifact links: " + artifactLinks,)

		_ = s.deleteRefWhereCombinedStateEqualsSuccess(s.Config.Username, prRepoName, ref,)
	} else {
		s.sendGitHubComment(prRepoOwner, prRepoName, prNumber,"Not triggering the mobile app build workflow, because PR checks are failing. ",)
	}
}
