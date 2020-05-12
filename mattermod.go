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
	"github.com/robfig/cron/v3"
)

func main() {
	var configFile string
	flag.StringVar(&configFile, "config", "config-mattermod.json", "")
	flag.Parse()

	config, err := server.GetConfig(configFile)
	if err != nil {
		panic(err)
	}
	server.SetupLogging(config)

	mlog.Info("Loaded config", mlog.String("filename", configFile))

	s := server.New(config)

	s.Start()
	defer s.Stop()

	c := cron.New()
	c.AddFunc("@daily", s.CheckPRActivity)
	c.AddFunc("@midnight", s.CleanOutdatedPRs)
	c.AddFunc("@every 2h", s.CheckTestServerLifeTime)
	c.AddFunc("@every 30m", s.AutoMergePR)

	cronTicker := fmt.Sprintf("@every %dm", s.Config.TickRateMinutes)
	c.AddFunc(cronTicker, s.Tick)

	c.Start()
	sig := make(chan os.Signal)
	signal.Notify(sig, os.Interrupt, os.Kill)
	<-sig
}
