// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"time"

	"github.com/mattermost/mattermost-mattermod/store/migrations"

	"github.com/go-gorp/gorp"
	_ "github.com/go-sql-driver/mysql" // Load MySQL Driver
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/mysql"
	bindata "github.com/golang-migrate/migrate/v4/source/go_bindata"
	"github.com/mattermost/mattermost-server/v5/mlog"
)

const (
	exitDBOpen = 101
	exitPing   = 102
)

type SQLStore struct {
	master        *gorp.DbMap
	pullRequest   PullRequestStore
	issue         IssueStore
	spinmint      SpinmintStore
	SchemaVersion string
}

func initConnection(driverName, dataSource string) *SQLStore {
	sqlStore := &SQLStore{}

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

	sqlStore.master = &gorp.DbMap{
		Db:            db,
		TypeConverter: mattermConverter{},
		Dialect: gorp.MySQLDialect{
			Engine:   "InnoDB",
			Encoding: "UTF8MB4",
		},
	}

	return sqlStore
}

func NewSQLStore(driverName, dataSource string) Store {
	sqlStore := initConnection(driverName, dataSource)

	sqlStore.pullRequest = NewSQLPullRequestStore(sqlStore)
	sqlStore.issue = NewSQLIssueStore(sqlStore)
	sqlStore.spinmint = NewSQLSpinmintStore(sqlStore)

	runMigrations(sqlStore.master.Db)

	return sqlStore
}

func (ss *SQLStore) GetMaster() *gorp.DbMap {
	return ss.master
}

func (ss *SQLStore) GetReplica() *gorp.DbMap {
	return ss.master
}

func (ss *SQLStore) GetAllConns() []*gorp.DbMap {
	all := make([]*gorp.DbMap, 1)
	all[0] = ss.master
	return all
}

func (ss *SQLStore) Close() {
	mlog.Info("closing db")
	ss.master.Db.Close()
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
	err := ss.master.TruncateTables()
	if err != nil {
		mlog.Error("failed to drop all tables", mlog.Err(err))
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
	if err != nil && err != migrate.ErrNoChange && err != os.ErrNotExist {
		mlog.Critical("Failed to migrate DB", mlog.Err(err))
		os.Exit(1)
	}
}

type mattermConverter struct{}

func (me mattermConverter) ToDb(val interface{}) (interface{}, error) {
	if _, ok := val.([]string); ok {
		b, err := json.Marshal(val)
		if err != nil {
			return nil, err
		}

		return string(b), nil
	}
	return val, nil
}

func (me mattermConverter) FromDb(target interface{}) (gorp.CustomScanner, bool) {
	if _, ok := target.(*[]string); ok {
		binder := func(holder, target interface{}) error {
			s, ok := holder.(*string)
			if !ok {
				return errors.New("could not deserialize pointer to string from db field")
			}

			if s == nil {
				return nil
			}

			b := []byte(*s)
			return json.Unmarshal(b, target)
		}
		return gorp.CustomScanner{Holder: new(string), Target: target, Binder: binder}, true
	}

	return gorp.CustomScanner{}, false
}
