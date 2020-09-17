// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package main

import (
	"flag"
	"os"

	"github.com/mattermost/mattermost-mattermod/server"
	"github.com/mattermost/mattermost-server/v5/mlog"
)

var (
	configFile     string
	migrateVersion int
)

func init() {
	flag.StringVar(&configFile, "config", "config-mattermod.json", "")
	flag.IntVar(&migrateVersion, "migration_version", -1, "Specify the target version to migrate to.")
}

func main() {
	flag.Parse()

	config, err := server.GetConfig(configFile)
	if err != nil {
		mlog.Error("unable to load server config", mlog.Err(err), mlog.String("file", configFile))
		os.Exit(1)
	}
	if err = server.SetupLogging(config); err != nil {
		mlog.Error("unable to configure logging", mlog.Err(err))
		os.Exit(1)
	}

	if migrateVersion != -1 {
		err = runMigrations(config.DriverName, config.DataSource, migrateVersion)
		if err != nil {
			mlog.Error("Failed to run migrations", mlog.Err(err))
			os.Exit(1)
		}
	}
}
