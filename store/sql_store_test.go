package store

import (
	"database/sql"
	"fmt"
	"os"
	"testing"

	"github.com/go-sql-driver/mysql"
)

const (
	defaultMysqlDSN         = "mattermod:mattermod@tcp(localhost:3306)/mattermost_mattermod_test?charset=utf8mb4,utf8&readTimeout=30s&writeTimeout=30s&parseTime=true&multiStatements=true"
	defaultMysqlRootUser    = "root"
	defaultMysqlRootUserPWD = "mattermod"
	defaultMysqlUser        = "mattermod"
	defaultMysqlUserPWD     = "mattermod"
	defaultMysqlDB          = "mattermod_test"
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

	store := initConnection("mysql", cfg.FormatDSN())
	runMigrations(store.db)

	t.Cleanup(func() {
		if err := store.db.Close(); err != nil {
			t.Fatal(err)
		}
		t.Log("destroyed temporary database")
	})

	return store
}

func createTempDB(t *testing.T, dbName, dbUser string) {
	rootUser := getEnv("MYSQL_ROOT_USER", defaultMysqlRootUser)
	rootPwd := getEnv("MYSQL_ROOT_PASSWORD", defaultMysqlRootUserPWD)
	cfg, err := mysql.ParseDSN(defaultMysqlDSN)
	if err != nil {
		t.Fatal(err)
	}

	cfg.User = rootUser
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
