// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mattermost/mattermost-mattermod/server"
)

func main() {
	var flagConfigFile string
	flag.StringVar(&flagConfigFile, "config", "config-mattermod.json", "")
	flag.Parse()

	server.LoadConfig(flagConfigFile)
	server.Start()

	go server.CleanOutdatedPRs()

	stopChan := make(chan os.Signal)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(time.Duration(server.Config.TickRate) * time.Second)

	run := true
	for run {
		server.Tick()
		select {
		case <-ticker.C:
			continue
		case <-stopChan:
			run = false
		}
	}

	server.Stop()
}
