// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package main

import (
	"database/sql"
	"fmt"

	"github.com/mattermost/mattermost-mattermod/store/migrations"

	_ "github.com/go-sql-driver/mysql" // Load MySQL Driver
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/mysql"
	bindata "github.com/golang-migrate/migrate/v4/source/go_bindata"
)

func runMigrations(driverName, dataSource string, migrateVersion int) error {
	if migrateVersion <= 0 {
		return fmt.Errorf("invalid migration version: %d", migrateVersion)
	}

	db, err := sql.Open(driverName, dataSource)
	if err != nil {
		return err
	}

	// Create database driver
	dbDriver, err := mysql.WithInstance(db, &mysql.Config{})
	if err != nil {
		return fmt.Errorf("failed to create migration driver: %v", err)
	}
	// Create source driver
	s := bindata.Resource(migrations.AssetNames(), migrations.Asset)

	srcDriver, err := bindata.WithInstance(s)
	if err != nil {
		return fmt.Errorf("failed to create source instance: %v", err)
	}

	m, err := migrate.NewWithInstance("go-bindata", srcDriver, "mysql", dbDriver)
	if err != nil {
		return fmt.Errorf("failed to create db instance: %v", err)
	}
	defer m.Close()

	err = m.Migrate(uint(migrateVersion))
	if err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migration: %v", err)
	}
	return nil
}
