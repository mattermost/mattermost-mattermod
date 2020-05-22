// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"time"

	"github.com/mattermost/mattermost-server/v5/mlog"
)

func (s *Server) CheckLimitRateAndSleep() {
	mlog.Info("Checking the rate limit on Github and will sleep if need...")
	rate, _, err := s.GithubClient.RateLimits(context.Background())
	if err != nil {
		mlog.Error("Error getting the rate limit", mlog.Err(err))
		time.Sleep(30 * time.Second)
		return
	}
	mlog.Info("Current rate limit", mlog.Int("Remaining Rate", rate.Core.Remaining), mlog.Int("Limit Rate", rate.Core.Limit))
	if rate.Core.Remaining <= s.Config.GitHubTokenReserve {
		sleepDuration := time.Until(rate.Core.Reset.Time) + (time.Second * 10)
		if sleepDuration > 0 {
			mlog.Error("--Rate Limiting-- Tokens reached minimum reserve. Sleeping until reset in", mlog.Int("Minimun", s.Config.GitHubTokenReserve), mlog.Any("Sleep time", sleepDuration))
			time.Sleep(sleepDuration)
		}
	}
}

func (s *Server) hasReachedRateLimit() bool {
	mlog.Info("Checking the rate limit on Github and will abort request if need...")
	rate, _, err := s.GithubClient.RateLimits(context.Background())
	if err != nil {
		mlog.Error("Error getting the rate limit", mlog.Err(err))
		time.Sleep(30 * time.Second)
		return false
	}
	mlog.Info("Current rate limit", mlog.Int("Remaining Rate", rate.Core.Remaining), mlog.Int("Limit Rate", rate.Core.Limit))
	if rate.Core.Remaining <= s.Config.GitHubTokenReserve {
		mlog.Info("Request will be aborted...")
		return true
	}
	return false
}
