// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"github.com/mattermost/mattermost-mattermod/model"
)

func (s *Server) buildMobileApp(pr *model.PullRequest) {
	if s.isCombinedStatusSuccessForPR(pr) {
		s.createRefWithPrefixFromPr(pr, s.Config.BuildMobileAppBranchPrefix)
	} else {
		s.sendGitHubComment(
			pr.RepoOwner,
			pr.RepoName,
			pr.Number,
			"Not triggering the mobile app build workflow, because PR checks are not successful. ",
		)
	}
}

func (s *Server) cleanupBuiltMobileAppBranches() {
	mobileRepoOwner := s.Config.BuildMobileAppCleanupRepo.Owner
	mobileRepoName := s.Config.BuildMobileAppCleanupRepo.Name

	s.deleteRefsWithPrefixWhereCombinedStateEqualsSuccess(
		mobileRepoOwner, mobileRepoName, s.Config.BuildMobileAppBranchPrefix,
	)
}
