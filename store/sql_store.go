// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package store

import (
	dbsql "database/sql"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/go-gorp/gorp"
	_ "github.com/go-sql-driver/mysql"
	"github.com/mattermost/mattermost-server/mlog"
)

const (
	INDEX_TYPE_FULL_TEXT = "full_text"
	INDEX_TYPE_DEFAULT   = "default"
	MAX_DB_CONN_LIFETIME = 15
)

const (
	EXIT_CREATE_TABLE                = 100
	EXIT_DB_OPEN                     = 101
	EXIT_PING                        = 102
	EXIT_NO_DRIVER                   = 103
	EXIT_TABLE_EXISTS                = 104
	EXIT_TABLE_EXISTS_MYSQL          = 105
	EXIT_COLUMN_EXISTS               = 106
	EXIT_DOES_COLUMN_EXISTS_POSTGRES = 107
	EXIT_DOES_COLUMN_EXISTS_MYSQL    = 108
	EXIT_DOES_COLUMN_EXISTS_MISSING  = 109
	EXIT_CREATE_COLUMN_POSTGRES      = 110
	EXIT_CREATE_COLUMN_MYSQL         = 111
	EXIT_CREATE_COLUMN_MISSING       = 112
	EXIT_REMOVE_COLUMN               = 113
	EXIT_RENAME_COLUMN               = 114
	EXIT_MAX_COLUMN                  = 115
	EXIT_ALTER_COLUMN                = 116
	EXIT_CREATE_INDEX_POSTGRES       = 117
	EXIT_CREATE_INDEX_MYSQL          = 118
	EXIT_CREATE_INDEX_FULL_MYSQL     = 119
	EXIT_CREATE_INDEX_MISSING        = 120
	EXIT_REMOVE_INDEX_POSTGRES       = 121
	EXIT_REMOVE_INDEX_MYSQL          = 122
	EXIT_REMOVE_INDEX_MISSING        = 123
)

type SqlStore struct {
	master        *gorp.DbMap
	replicas      []*gorp.DbMap
	pullRequest   PullRequestStore
	issue         IssueStore
	spinmint      SpinmintStore
	SchemaVersion string
}

func initConnection(driverName, dataSource string) *SqlStore {
	sqlStore := &SqlStore{}

	db, err := dbsql.Open(driverName, dataSource)
	if err != nil {
		mlog.Critical("failed to open db connection", mlog.Err(err))
		time.Sleep(time.Second)
		os.Exit(EXIT_DB_OPEN)
	}

	mlog.Info("pinging db")
	err = db.Ping()
	if err != nil {
		mlog.Critical("could not ping db", mlog.Err(err))
		time.Sleep(time.Second)
		os.Exit(EXIT_PING)
	}

	sqlStore.master = &gorp.DbMap{Db: db, TypeConverter: mattermConverter{}, Dialect: gorp.MySQLDialect{Engine: "InnoDB", Encoding: "UTF8MB4"}}

	return sqlStore
}

func NewSqlStore(driverName, dataSource string) Store {
	sqlStore := initConnection(driverName, dataSource)

	sqlStore.pullRequest = NewSqlPullRequestStore(sqlStore)
	sqlStore.issue = NewSqlIssueStore(sqlStore)
	sqlStore.spinmint = NewSqlSpinmintStore(sqlStore)

	if err := sqlStore.master.CreateTablesIfNotExists(); err != nil {
		mlog.Critical("error creating tables", mlog.Err(err))
		time.Sleep(time.Second)
		os.Exit(EXIT_CREATE_TABLE)
	}

	// UpgradeDatabase(sqlStore)

	sqlStore.pullRequest.(*SqlPullRequestStore).CreateIndexesIfNotExists()
	sqlStore.issue.(*SqlIssueStore).CreateIndexesIfNotExists()
	sqlStore.spinmint.(*SqlSpinmintStore).CreateIndexesIfNotExists()

	return sqlStore
}

func (ss *SqlStore) DoesTableExist(tableName string) bool {
	count, err := ss.GetMaster().SelectInt(
		`SELECT
	    COUNT(0) AS table_exists
		FROM
		    information_schema.TABLES
		WHERE
		    TABLE_SCHEMA = DATABASE()
		        AND TABLE_NAME = ?
	    `,
		tableName,
	)

	if err != nil {
		mlog.Critical("failed to check if table exists", mlog.Err(err))
		time.Sleep(time.Second)
		os.Exit(EXIT_TABLE_EXISTS_MYSQL)
	}

	return count > 0
}

func (ss *SqlStore) DoesColumnExist(tableName string, columnName string) bool {
	count, err := ss.GetMaster().SelectInt(
		`SELECT
		    COUNT(0) AS column_exists
		FROM
		    information_schema.COLUMNS
		WHERE
		    TABLE_SCHEMA = DATABASE()
		        AND TABLE_NAME = ?
		        AND COLUMN_NAME = ?`,
		tableName,
		columnName,
	)

	if err != nil {
		mlog.Critical("failed to check if column exists", mlog.Err(err))
		time.Sleep(time.Second)
		os.Exit(EXIT_DOES_COLUMN_EXISTS_MYSQL)
	}

	return count > 0
}

func (ss *SqlStore) CreateColumnIfNotExists(tableName string, columnName string, mySqlColType string, postgresColType string, defaultValue string) bool {
	if ss.DoesColumnExist(tableName, columnName) {
		return false
	}

	_, err := ss.GetMaster().Exec("ALTER TABLE " + tableName + " ADD " + columnName + " " + mySqlColType + " DEFAULT '" + defaultValue + "'")
	if err != nil {
		mlog.Critical("failed to create column if not exists: %v", mlog.Err(err))
		time.Sleep(time.Second)
		os.Exit(EXIT_CREATE_COLUMN_MYSQL)
	}

	return true
}

func (ss *SqlStore) RemoveColumnIfExists(tableName string, columnName string) bool {

	if !ss.DoesColumnExist(tableName, columnName) {
		return false
	}

	_, err := ss.GetMaster().Exec("ALTER TABLE " + tableName + " DROP COLUMN " + columnName)
	if err != nil {
		mlog.Critical("failed to remove column if exists", mlog.Err(err))
		time.Sleep(time.Second)
		os.Exit(EXIT_REMOVE_COLUMN)
	}

	return true
}

func (ss *SqlStore) RenameColumnIfExists(tableName string, oldColumnName string, newColumnName string, colType string) bool {
	if !ss.DoesColumnExist(tableName, oldColumnName) {
		return false
	}

	_, err := ss.GetMaster().Exec("ALTER TABLE " + tableName + " CHANGE " + oldColumnName + " " + newColumnName + " " + colType)

	if err != nil {
		mlog.Critical("failed to rename column if exists", mlog.Err(err))
		time.Sleep(time.Second)
		os.Exit(EXIT_RENAME_COLUMN)
	}

	return true
}

func (ss *SqlStore) GetMaxLengthOfColumnIfExists(tableName string, columnName string) string {
	if !ss.DoesColumnExist(tableName, columnName) {
		return ""
	}

	result, err := ss.GetMaster().SelectStr("SELECT CHARACTER_MAXIMUM_LENGTH FROM information_schema.columns WHERE table_name = '" + tableName + "' AND COLUMN_NAME = '" + columnName + "'")

	if err != nil {
		mlog.Critical("failed to get max length of column if exists", mlog.Err(err))
		time.Sleep(time.Second)
		os.Exit(EXIT_MAX_COLUMN)
	}

	return result
}

func (ss *SqlStore) AlterColumnTypeIfExists(tableName string, columnName string, mySqlColType string, postgresColType string) bool {
	if !ss.DoesColumnExist(tableName, columnName) {
		return false
	}

	_, err := ss.GetMaster().Exec("ALTER TABLE " + tableName + " MODIFY " + columnName + " " + mySqlColType)

	if err != nil {
		mlog.Critical("failed to alter column type if exists", mlog.Err(err))
		time.Sleep(time.Second)
		os.Exit(EXIT_ALTER_COLUMN)
	}

	return true
}

func (ss *SqlStore) CreateUniqueIndexIfNotExists(indexName string, tableName string, columnName string) bool {
	return ss.createIndexIfNotExists(indexName, tableName, columnName, INDEX_TYPE_DEFAULT, true)
}

func (ss *SqlStore) CreateIndexIfNotExists(indexName string, tableName string, columnName string) bool {
	return ss.createIndexIfNotExists(indexName, tableName, columnName, INDEX_TYPE_DEFAULT, false)
}

func (ss *SqlStore) CreateFullTextIndexIfNotExists(indexName string, tableName string, columnName string) bool {
	return ss.createIndexIfNotExists(indexName, tableName, columnName, INDEX_TYPE_FULL_TEXT, false)
}

func (ss *SqlStore) createIndexIfNotExists(indexName string, tableName string, columnName string, indexType string, unique bool) bool {

	uniqueStr := ""
	if unique {
		uniqueStr = "UNIQUE "
	}

	count, err := ss.GetMaster().SelectInt("SELECT COUNT(0) AS index_exists FROM information_schema.statistics WHERE TABLE_SCHEMA = DATABASE() and table_name = ? AND index_name = ?", tableName, indexName)
	if err != nil {
		mlog.Critical("can't check for index", mlog.Err(err))
		time.Sleep(time.Second)
		os.Exit(EXIT_CREATE_INDEX_MYSQL)
	}

	if count > 0 {
		return false
	}

	fullTextIndex := ""
	if indexType == INDEX_TYPE_FULL_TEXT {
		fullTextIndex = " FULLTEXT "
	}

	_, err = ss.GetMaster().Exec("CREATE  " + uniqueStr + fullTextIndex + " INDEX " + indexName + " ON " + tableName + " (" + columnName + ")")
	if err != nil {
		mlog.Critical("failed to create index if not exists", mlog.Err(err))
		time.Sleep(time.Second)
		os.Exit(EXIT_CREATE_INDEX_FULL_MYSQL)
	}

	return true
}

func (ss *SqlStore) RemoveIndexIfExists(indexName string, tableName string) bool {
	count, err := ss.GetMaster().SelectInt("SELECT COUNT(0) AS index_exists FROM information_schema.statistics WHERE TABLE_SCHEMA = DATABASE() and table_name = ? AND index_name = ?", tableName, indexName)
	if err != nil {
		mlog.Critical("can't check index to remove", mlog.Err(err))
		time.Sleep(time.Second)
		os.Exit(EXIT_REMOVE_INDEX_MYSQL)
	}

	if count <= 0 {
		return false
	}

	_, err = ss.GetMaster().Exec("DROP INDEX " + indexName + " ON " + tableName)
	if err != nil {
		mlog.Critical("failed to remove index if exists", mlog.Err(err))
		time.Sleep(time.Second)
		os.Exit(EXIT_REMOVE_INDEX_MYSQL)
	}

	return true
}

func IsUniqueConstraintError(err string, indexName []string) bool {
	unique := strings.Contains(err, "unique constraint") || strings.Contains(err, "Duplicate entry")
	field := false
	for _, contain := range indexName {
		if strings.Contains(err, contain) {
			field = true
			break
		}
	}

	return unique && field
}

func (ss *SqlStore) GetMaster() *gorp.DbMap {
	return ss.master
}

func (ss *SqlStore) GetReplica() *gorp.DbMap {
	return ss.master
}

func (ss *SqlStore) GetAllConns() []*gorp.DbMap {
	all := make([]*gorp.DbMap, 1)
	all[0] = ss.master
	return all
}

func (ss *SqlStore) Close() {
	mlog.Info("closing db")
	ss.master.Db.Close()
}

func (ss *SqlStore) PullRequest() PullRequestStore {
	return ss.pullRequest
}

func (ss *SqlStore) Issue() IssueStore {
	return ss.issue
}

func (ss *SqlStore) Spinmint() SpinmintStore {
	return ss.spinmint
}

func (ss *SqlStore) DropAllTables() {
	ss.master.TruncateTables()
}

type mattermConverter struct{}

func (me mattermConverter) ToDb(val interface{}) (interface{}, error) {
	switch val.(type) {
	case []string:
		if b, err := json.Marshal(val); err != nil {
			return nil, err
		} else {
			return string(b), nil
		}
	}
	return val, nil
}

func (me mattermConverter) FromDb(target interface{}) (gorp.CustomScanner, bool) {
	switch target.(type) {
	case *[]string:
		binder := func(holder, target interface{}) error {
			s, ok := holder.(*string)
			if !ok {
				return errors.New("could not deserialize pointer to string from db field")
			}

			if s == nil {
				target = []string{}
				return nil
			}

			b := []byte(*s)
			return json.Unmarshal(b, target)
		}
		return gorp.CustomScanner{new(string), target, binder}, true
	}

	return gorp.CustomScanner{}, false
}

func convertMySQLFullTextColumnsToPostgres(columnNames string) string {
	columns := strings.Split(columnNames, ", ")
	concatenatedColumnNames := ""
	for i, c := range columns {
		concatenatedColumnNames += c
		if i < len(columns)-1 {
			concatenatedColumnNames += " || ' ' || "
		}
	}

	return concatenatedColumnNames
}
