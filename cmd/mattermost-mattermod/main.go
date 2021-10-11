// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mattermost/mattermost-mattermod/metrics"
	"github.com/mattermost/mattermost-mattermod/server"
	"github.com/mattermost/mattermost-server/v5/mlog"
)

var (
	configFile string
)

func init() {
	flag.StringVar(&configFile, "config", "config-mattermod.json", "")
}

func main() {
	var exitCode = 1
	defer func() {
		os.Exit(exitCode)
	}()

	flag.Parse()

	config, err := server.GetConfig(configFile)
	if err != nil {
		mlog.Error("unable to load server config", mlog.Err(err), mlog.String("file", configFile))
		exitCode = 1
		return
	}
	if err = server.SetupLogging(config); err != nil {
		mlog.Error("unable to configure logging", mlog.Err(err))
		exitCode = 1
		return
	}

	// Metrics system
	metricsProvider := metrics.NewPrometheusProvider()
	metricsServer := metrics.NewServer(config.MetricsServerPort, metricsProvider.Handler(), true)
	metricsServer.Start()
	defer metricsServer.Stop()

	mlog.Info("Loaded config", mlog.String("filename", configFile))
	s, err := server.New(config, metricsProvider)
	if err != nil {
		mlog.Error("unable to start server", mlog.Err(err))
		exitCode = 1
		return
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
		if err2 := mlog.ShutdownAdvancedLogging(ctx); err2 != nil {
			mlog.Error("error while shutting logging", mlog.Err(err2))
			code = 1
		}
		if code != 0 {
			exitCode = code
			return
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	<-sig
}
