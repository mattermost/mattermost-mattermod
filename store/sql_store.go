// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package store

import (
	"database/sql"
	"errors"
	"os"
	"time"

	"github.com/mattermost/mattermost-mattermod/store/migrations"

	_ "github.com/go-sql-driver/mysql" // Load MySQL Driver
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/mysql"
	bindata "github.com/golang-migrate/migrate/v4/source/go_bindata"
	"github.com/jmoiron/sqlx"
	"github.com/mattermost/mattermost-server/v5/mlog"
)

const (
	exitDBOpen = 101
	exitPing   = 102
)

type SQLStore struct {
	dbx           *sqlx.DB
	db            *sql.DB
	pullRequest   PullRequestStore
	issue         IssueStore
	spinmint      SpinmintStore
	SchemaVersion string
}

func initConnection(driverName, dataSource string) *SQLStore {
	db, err := sql.Open(driverName, dataSource)
	if err != nil {
		mlog.Critical("failed to open db connection", mlog.Err(err))
		time.Sleep(time.Second)
		os.Exit(exitDBOpen)
	}

	mlog.Info("pinging db")
	err = db.Ping()
	if err != nil {
		mlog.Critical("could not ping db", mlog.Err(err))
		time.Sleep(time.Second)
		os.Exit(exitPing)
	}

	sqlStore := &SQLStore{
		dbx: sqlx.NewDb(db, driverName),
		db:  db,
	}
	sqlStore.dbx.MapperFunc(func(s string) string { return s })

	return sqlStore
}

func NewSQLStore(driverName, dataSource string) Store {
	sqlStore := initConnection(driverName, dataSource)

	sqlStore.pullRequest = NewSQLPullRequestStore(sqlStore)
	sqlStore.issue = NewSQLIssueStore(sqlStore)
	sqlStore.spinmint = NewSQLSpinmintStore(sqlStore)

	runMigrations(sqlStore.db)

	return sqlStore
}

func (ss *SQLStore) Close() {
	mlog.Info("closing db")
	ss.dbx.Close()
}

func (ss *SQLStore) PullRequest() PullRequestStore {
	return ss.pullRequest
}

func (ss *SQLStore) Issue() IssueStore {
	return ss.issue
}

func (ss *SQLStore) Spinmint() SpinmintStore {
	return ss.spinmint
}

func (ss *SQLStore) DropAllTables() {
	stmts := []string{"TRUNCATE TABLE Issues", "TRUNCATE TABLE PullRequests", "TRUNCATE TABLE Spinmint"}
	for _, s := range stmts {
		_, err := ss.dbx.Exec(s)
		if err != nil {
			mlog.Error("failed to drop table", mlog.Err(err), mlog.String("table", s))
		}
	}
}

func runMigrations(db *sql.DB) {
	// Create database driver
	dbDriver, err := mysql.WithInstance(db, &mysql.Config{})
	if err != nil {
		mlog.Critical("Failed to create migration driver", mlog.Err(err))
		os.Exit(1)
	}
	// Create source driver
	s := bindata.Resource(migrations.AssetNames(), migrations.Asset)

	srcDriver, err := bindata.WithInstance(s)
	if err != nil {
		mlog.Critical("Failed to create source instance", mlog.Err(err))
		os.Exit(1)
	}

	m, err := migrate.NewWithInstance("go-bindata", srcDriver, "mysql", dbDriver)
	if err != nil {
		mlog.Critical("Failed to create db instance", mlog.Err(err))
		os.Exit(1)
	}
	// Run migration
	err = m.Up()
	// We ignore if there is no change and if file does not exist.
	// The latter occurs if we have rolled back to older code without running down migrations.
	// This is not ideal, but not critical enough to bail execution.
	if err != nil && err != migrate.ErrNoChange && !errors.Is(err, os.ErrNotExist) {
		mlog.Critical("Failed to migrate DB", mlog.Err(err))
		os.Exit(1)
	}
}
