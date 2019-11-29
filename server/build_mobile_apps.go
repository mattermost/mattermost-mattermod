// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"github.com/mattermost/mattermost-mattermod/model"
)

func (s *Server) buildMobileApp(pr *model.PullRequest) {
	s.createRefWithPrefixFromPr(pr, s.Config.BuildMobileAppBranchPrefix)
}

func (s *Server) cleanupBuiltMobileAppBranches() {
	mobileRepoOwner := s.Config.BuildMobileAppCleanupRepo.Owner
	mobileRepoName := s.Config.BuildMobileAppCleanupRepo.Name

	s.deleteRefsWithPrefixWhereCombinedStateEqualsSuccess(
		mobileRepoOwner, mobileRepoName, s.Config.BuildMobileAppBranchPrefix,
	)
}
