// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"time"

	"github.com/google/go-github/github"
	"github.com/mattermost/mattermost-server/mlog"
)

func limitRate(r *github.Rate) {
	mlog.Info("Current rate limit", mlog.Int("Remaining Rate", r.Remaining), mlog.Int("Limit Rate", r.Limit))
	if r.Remaining <= Config.GitHubTokenReserve {
		sleepDuration := time.Until(r.Reset.Time) + (time.Second * 10)
		if sleepDuration > 0 {
			mlog.Error("--Rate Limiting-- Tokens reached minimum reserve. Sleeping until reset in", mlog.Int("Minimun", Config.GitHubTokenReserve), mlog.Any("Sleep time", sleepDuration))
			time.Sleep(sleepDuration)
		}
	}
}

func CheckLimitRateGH() {
	mlog.Info("Checking the rate limit on Github and will sleep if need...")

	client := NewGithubClient()
	rate, _, err := client.RateLimit()
	if err != nil {
		mlog.Error("Error getting the rate limit", mlog.Err(err))
		time.Sleep(30 * time.Second)
		return
	}
	limitRate(rate)
}
