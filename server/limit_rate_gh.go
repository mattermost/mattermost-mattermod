// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"time"

	"github.com/mattermost/mattermost-server/v5/mlog"
)

func (s *Server) shouldStopRequests() bool {
	intervalBetweenRateLimitChecks := 2 * time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), 2*intervalBetweenRateLimitChecks+timeoutRequestGithub)
	defer cancel()

	rate, _, err := s.GithubClient.RateLimits(ctx)
	for err != nil {
		time.Sleep(intervalBetweenRateLimitChecks)
		mlog.Error("Error getting the rate limit", mlog.Err(err))
		rate, _, err = s.GithubClient.RateLimits(ctx)
	}

	mlog.Info("Current rate limit", mlog.Int("Remaining Rate", rate.Core.Remaining), mlog.Int("Limit Rate", rate.Core.Limit))
	if rate.Core.Remaining <= s.Config.GitHubTokenReserve {
		mlog.Info("Request will be aborted...")
		return true
	}
	return false
}
