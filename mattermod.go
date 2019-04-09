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
	var flagConfigFile string
	flag.StringVar(&flagConfigFile, "config", "config-mattermod.json", "")
	flag.Parse()

	server.LoadConfig(flagConfigFile)
	server.Start()

	//server.CleanOutdatedPRs()
	//server.CleanOutdatedIssues()

	c := cron.New()
	c.AddFunc("@daily", server.CheckPRActivity)
	c.AddFunc("@midnight", server.CleanOutdatedPRs)
	c.AddFunc("@every 2h", server.CheckSpinmintLifeTime)

	cronTicker := fmt.Sprintf("@every %dm", server.Config.TickRateMinutes)
	c.AddFunc(cronTicker, server.Tick)

	c.Start()
	sig := make(chan os.Signal)
	signal.Notify(sig, os.Interrupt, os.Kill)
	<-sig

	server.Stop()
}
