// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"time"

	"github.com/mattermost/mattermost-server/v5/mlog"
)

func (s *Server) shouldStopRequests(ctx context.Context) (shouldStop bool, timeUntilReset *time.Duration) {
	rate, _, err := s.GithubClient.RateLimits(ctx)
	for err != nil {
		mlog.Error("error getting rate limit")
		return true, nil
	}

	rateReset := time.Until(rate.Core.Reset.UTC())
	mlog.Info("Current rate limit", mlog.Int("Remaining Rate", rate.Core.Remaining), mlog.Int("Limit Rate", rate.Core.Limit))
	if rate.Core.Remaining <= s.Config.GitHubTokenReserve {
		mlog.Debug("Request will be aborted...")

		return true, &rateReset
	}
	return false, &rateReset
}
