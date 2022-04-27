// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/mattermost/mattermost-mattermod/metrics"
	"github.com/mattermost/mattermost-mattermod/server"
	"github.com/mattermost/mattermost-server/v6/shared/mlog"
)

var (
	configFile string
)

func init() {
	flag.StringVar(&configFile, "config", "config-mattermod.json", "")
}

func main() {
	flag.Parse()

	config, err := server.GetConfig(configFile)
	if err != nil {
		mlog.Error("unable to load server config", mlog.Err(err), mlog.String("file", configFile))
		return
	}
	if err = server.SetupLogging(config); err != nil {
		mlog.Error("unable to configure logging", mlog.Err(err))
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
		return
	}

	mlog.Info("Starting Mattermod Server")
	s.Start()

	defer func() {
		mlog.Info("Stopping Mattermod Server")
		if err2 := s.Stop(); err2 != nil {
			mlog.Error("error while shutting down server", mlog.Err(err2))
			os.Exit(1)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	<-sig
	mlog.Info("Stopped Mattermod Server")
}
