// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

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
		mlog.Error("unable to load server config", mlog.Err(errors.Wrap(err, "unable to load server config")))
		os.Exit(1)
	}
	server.SetupLogging(config)

	mlog.Info("Loaded config", mlog.String("filename", configFile))
	s := server.New(config)

	mlog.Info("Starting Mattermod Server")
	errs := s.Start()

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
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sig:
		mlog.Info("Stopping Mattermod Server")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.Stop(ctx); err != nil {
			mlog.Error("Error while shutting down server", mlog.Err(err))
			os.Exit(1)
		}
	case err := <-errs:
		mlog.Error("Server exited with error", mlog.Err(err))
		os.Exit(1)
	}
}
