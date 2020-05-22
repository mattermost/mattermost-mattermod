// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/mattermost/mattermost-mattermod/server"
	"github.com/mattermost/mattermost-server/v5/mlog"
	"github.com/pkg/errors"
	"github.com/robfig/cron/v3"
)

func main() {
	var configFile string
	flag.StringVar(&configFile, "config", "config-mattermod.json", "")
	flag.Parse()

	config, err := server.GetConfig(configFile)
	if err != nil {
		err = errors.Wrap(err, "unable to load server config")
		mlog.Error("unable to load server config", mlog.Err(err))
		panic(err)
	}
	server.SetupLogging(config)

	mlog.Info("Loaded config", mlog.String("filename", configFile))

	s, err := server.New(config)
	if err != nil {
		panic("failed creating server")
	}

	s.Start()
	defer s.Stop()

	c := cron.New()

	_, err = c.AddFunc("0 1 * * *", s.CheckPRActivity)
	if err != nil {
		mlog.Error("failed adding CheckPRActivity cron", mlog.Err(err))
	}

	_, err = c.AddFunc("0 2 * * *", s.RefreshMembers)
	if err != nil {
		mlog.Error("failed adding RefreshMembers cron", mlog.Err(err))
	}

	_, err = c.AddFunc("0 3 * * *", s.CleanOutdatedPRs)
	if err != nil {
		mlog.Error("failed adding CleanOutdatedPRs cron", mlog.Err(err))
	}

	_, err = c.AddFunc("@every 2h", s.CheckTestServerLifeTime)
	if err != nil {
		mlog.Error("failed adding CheckTestServerLifeTime cron", mlog.Err(err))
	}
	_, err = c.AddFunc("@every 30m", s.AutoMergePR)
	if err != nil {
		mlog.Error("failed adding AutoMergePR cron", mlog.Err(err))
	}

	cronTicker := fmt.Sprintf("@every %dm", s.Config.TickRateMinutes)
	_, err = c.AddFunc(cronTicker, s.Tick)
	if err != nil {
		mlog.Error("failed adding Ticker cron", mlog.Err(err))
	}

	c.Start()
	sig := make(chan os.Signal)
	signal.Notify(sig, os.Interrupt, os.Kill)
	<-sig
}
