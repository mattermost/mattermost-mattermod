// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mattermost/mattermost-mattermod/server"
	"github.com/mattermost/mattermost-server/v5/mlog"
	"github.com/robfig/cron/v3"
	"golang.org/x/net/context"
)

func main() {
	var configFile string
	flag.StringVar(&configFile, "config", "config-mattermod.json", "")
	flag.Parse()

	config, err := server.GetConfig(configFile)
	if err != nil {
		mlog.Error("unable to load server config", mlog.Err(err))
		os.Exit(1)
	}
	if err = server.SetupLogging(config); err != nil {
		mlog.Error("unable to configure logging", mlog.Err(err))
		os.Exit(1)
	}

	mlog.Info("Loaded config", mlog.String("filename", configFile))
	s, err := server.New(config)
	if err != nil {
		mlog.Error("unable to start server", mlog.Err(err))
		os.Exit(1)
	}

	mlog.Info("Starting Mattermod Server")
	s.Start()

	defer func() {
		mlog.Info("Stopping Mattermod Server")
		code := 0
		if err2 := s.Stop(); err2 != nil {
			mlog.Error("error while shutting down server", mlog.Err(err2))
			code = 1
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		if err3 := mlog.ShutdownAdvancedLogging(ctx); err != nil {
			mlog.Error("error while shutting logging", mlog.Err(err3))
			code = 1
		}
		if code != 0 {
			os.Exit(code)
		}
	}()

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
	_, err = c.AddFunc("@every 30m", func() {
		err2 := s.AutoMergePR()
		if err2 != nil {
			mlog.Error("Error from AutoMergePR", mlog.Err(err2))
		}
	})
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

	<-sig
}
