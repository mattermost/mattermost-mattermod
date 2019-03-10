// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"time"

	"github.com/mattermost/mattermost-server/mlog"
)

func CheckLimitRateAndSleep() {
	mlog.Info("Checking the rate limit on Github and will sleep if need...")

	client := NewGithubClient()
	rate, _, err := client.RateLimit()
	if err != nil {
		mlog.Error("Error getting the rate limit", mlog.Err(err))
		time.Sleep(30 * time.Second)
		return
	}
	mlog.Info("Current rate limit", mlog.Int("Remaining Rate", rate.Remaining), mlog.Int("Limit Rate", rate.Limit))
	if rate.Remaining <= Config.GitHubTokenReserve {
		sleepDuration := time.Until(rate.Reset.Time) + (time.Second * 10)
		if sleepDuration > 0 {
			mlog.Error("--Rate Limiting-- Tokens reached minimum reserve. Sleeping until reset in", mlog.Int("Minimun", Config.GitHubTokenReserve), mlog.Any("Sleep time", sleepDuration))
			time.Sleep(sleepDuration)
		}
	}
}

func CheckLimitRateAndAbortRequest() bool {
	mlog.Info("Checking the rate limit on Github and will sleep if need...")

	client := NewGithubClient()
	rate, _, err := client.RateLimit()
	if err != nil {
		mlog.Error("Error getting the rate limit", mlog.Err(err))
		time.Sleep(30 * time.Second)
		return
	}
	mlog.Info("Current rate limit", mlog.Int("Remaining Rate", rate.Remaining), mlog.Int("Limit Rate", rate.Limit))
	if rate.Remaining <= Config.GitHubTokenReserve {
		return true
	}
	return false
}
