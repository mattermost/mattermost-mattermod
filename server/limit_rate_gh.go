// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"time"

	"github.com/mattermost/mattermost-server/v5/mlog"
)

func (s *Server) sleepUntilRateLimitAboveTokenReserve() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rate, _, err := s.GithubClient.RateLimits(ctx)
	for err != nil || rate.Core.Remaining <= s.Config.GitHubTokenReserve {
		if err != nil {
			mlog.Error("Error getting the rate limit", mlog.Err(err))
			time.Sleep(2 * time.Minute)
		} else {
			mlog.Info("--Rate Limit-- Reached minimum threshold. Reset in", mlog.Int("Minimum", s.Config.GitHubTokenReserve), mlog.Any("Sleep time", rate.Core.Reset.UTC()))
			time.Sleep(time.Until(rate.Core.Reset.UTC()))
		}
		rate, _, err = s.GithubClient.RateLimits(ctx)
	}
}

func (s *Server) shouldStopRequests() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rate, _, err := s.GithubClient.RateLimits(ctx)
	for err != nil {
		time.Sleep(2 * time.Minute)
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
