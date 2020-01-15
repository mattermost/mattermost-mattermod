// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"github.com/mattermost/mattermost-mattermod/model"
	"strconv"
)

func (s *Server) buildMobileApp(pr *model.PullRequest) {
	if s.isCombinedStatusSuccessForPR(pr) {
		ref := s.Config.BuildMobileAppBranchPrefix + strconv.Itoa(pr.Number)
		exists, err := s.checkIfRefExists(pr, ref)
		if err != nil {
			s.sendGitHubComment(
				pr.RepoOwner,
				pr.RepoName,
				pr.Number,
				"Failed checking for existing reference. @mattermost/core-build-engineers have been notified. ",
			)

			return
		}

		if exists {

		}

		s.createRefWithPrefixFromPr(pr, s.Config.BuildMobileAppBranchPrefix)

		buildLink, buildNumber, err := s.waitForBuildLink(pr)
		if err != nil {
			s.sendGitHubComment(
				pr.RepoOwner,
				pr.RepoName,
				pr.Number,
				"Failed building. @mattermost/core-build-engineers have been notified. ",
			)

			return
		}

		s.sendGitHubComment(
			pr.RepoOwner,
			pr.RepoName,
			pr.Number,
			"Successfully building: " + buildLink,
		)

		artifactLinks, err := s.waitForArtifactLinks(pr, buildNumber)
		if err != nil {
			s.sendGitHubComment(
				pr.RepoOwner,
				pr.RepoName,
				pr.Number,
				"Failed retrieving artifact links. @mattermost/core-build-engineers have been notified. ",
			)

			return
		}

		s.sendGitHubComment(
			pr.RepoOwner,
			pr.RepoName,
			pr.Number,
			"Artifact links: " + artifactLinks,
		)

		s.deleteRefWhereCombinedStateEqualsSuccess(
			 pr.RepoOwner, pr.RepoName, ref,
		)

	} else {
		s.sendGitHubComment(
			pr.RepoOwner,
			pr.RepoName,
			pr.Number,
			"Not triggering the mobile app build workflow, because PR checks are not successful. ",
		)
	}
}
