package store

import (
	"database/sql"
	"fmt"
	"os"
	"testing"

	"github.com/go-gorp/gorp"
	"github.com/go-sql-driver/mysql"
	"github.com/mattermost/mattermost-mattermod/model"
)

const (
	defaultMysqlDSN     = "mattermod:mattermod@tcp(localhost:3306)/mattermost_mattermod_test?charset=utf8mb4,utf8\u0026readTimeout=30s\u0026writeTimeout=30s\u0026parseTime=true"
	defaultMysqlUser    = "mattermod"
	defaultMysqlRootPWD = "mattermod"
	defaultMysqlUserPWD = "mattermod"
	defaultMysqlDB      = "mattermod_test"
)

func getTestSQLStore(t *testing.T) *SQLStore {
	t.Helper()

	createTempDB(t, defaultMysqlDB, getEnv("MYSQL_USER", defaultMysqlUser))
	t.Log("created temporary database")

	cfg, err := mysql.ParseDSN(defaultMysqlDSN)
	if err != nil {
		t.Fatal(err)
	}

	cfg.User = getEnv("MYSQL_USER", defaultMysqlUser)
	cfg.Passwd = getEnv("MYSQL_PASSWORD", defaultMysqlUserPWD)
	cfg.DBName = defaultMysqlDB

	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}

	store := &SQLStore{
		master: &gorp.DbMap{
			Db:            db,
			TypeConverter: mattermConverter{},
			Dialect: gorp.MySQLDialect{
				Engine:   "InnoDB",
				Encoding: "UTF8MB4",
			},
		},
	}
	t.Cleanup(func() {
		db.Close()
		t.Log("destroyed temporary database")
	})

	store.master.AddTableWithName(model.PullRequest{}, "PullRequests")
	if err := store.master.CreateTablesIfNotExists(); err != nil {
		t.Fatal(err)
	}

	return store
}

func createTempDB(t *testing.T, dbName, dbUser string) {
	rootPwd := getEnv("MYSQL_ROOT_PASSWORD", defaultMysqlRootPWD)
	cfg, err := mysql.ParseDSN(defaultMysqlDSN)
	if err != nil {
		t.Fatal(err)
	}

	cfg.User = "root"
	cfg.Passwd = rootPwd
	cfg.DBName = "mysql"

	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if _, err2 := db.Exec(fmt.Sprintf("DROP DATABASE %s", dbName)); err2 != nil {
			panic(fmt.Sprintf("failed to drop temporary database: %s", err2))
		}
		db.Close()
	})

	if _, err = db.Exec(fmt.Sprintf("CREATE DATABASE %s", dbName)); err != nil {
		t.Fatal(err)
	}

	if _, err = db.Exec(fmt.Sprintf("GRANT ALL PRIVILEGES ON %s.* TO '%s'", dbName, dbUser)); err != nil {
		t.Fatal(err)
	}
}

func getEnv(name, defaultValue string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return defaultValue
}
