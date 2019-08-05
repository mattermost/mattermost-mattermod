// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/mattermost/mattermost-mattermod/server"
	"gopkg.in/robfig/cron.v3"
)

func main() {
	var configFile string
	flag.StringVar(&configFile, "config", "config-mattermod.json", "")
	flag.Parse()

	s, err := server.New(configFile)
	if err != nil {
		panic(err)
	}
	s.Start()
	defer s.Stop()

	c := cron.New()
	c.AddFunc("@daily", s.CheckPRActivity)
	c.AddFunc("@midnight", s.CleanOutdatedPRs)
	c.AddFunc("@every 2h", s.CheckTestServerLifeTime)

	cronTicker := fmt.Sprintf("@every %dm", s.Config.TickRateMinutes)
	c.AddFunc(cronTicker, s.Tick)

	c.Start()
	sig := make(chan os.Signal)
	signal.Notify(sig, os.Interrupt, os.Kill)
	<-sig
}
