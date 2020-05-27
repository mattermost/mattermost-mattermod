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
	_ "github.com/go-sql-driver/mysql" // Load MySQL Driver
	"github.com/mattermost/mattermost-server/v5/mlog"
)

const (
	indexTypeFullText = "full_text"
	indexTypeDefault  = "default"
)

const (
	exitCreateTable          = 100
	exitDBOpen               = 101
	exitPing                 = 102
	exitTableExistsMySQL     = 105
	exitColumExistsMySQL     = 108
	exitCreateColumMySQL     = 111
	exitRemoveColumn         = 113
	exitRenameColumn         = 114
	exitMaxColumn            = 115
	exitAlterColumn          = 116
	exitCreateIndexMySQL     = 118
	exitCreateIndexFullMySQL = 119
	exitRemoveIndexMySQL     = 122
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

	db, err := dbsql.Open(driverName, dataSource)
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

	sqlStore.master = &gorp.DbMap{Db: db, TypeConverter: mattermConverter{}, Dialect: gorp.MySQLDialect{Engine: "InnoDB", Encoding: "UTF8MB4"}}

	return sqlStore
}

func NewSQLStore(driverName, dataSource string) Store {
	sqlStore := initConnection(driverName, dataSource)

	sqlStore.pullRequest = NewSQLPullRequestStore(sqlStore)
	sqlStore.issue = NewSQLIssueStore(sqlStore)
	sqlStore.spinmint = NewSQLSpinmintStore(sqlStore)

	if err := sqlStore.master.CreateTablesIfNotExists(); err != nil {
		mlog.Critical("error creating tables", mlog.Err(err))
		time.Sleep(time.Second)
		os.Exit(exitCreateTable)
	}

	// UpgradeDatabase(sqlStore)

	sqlStore.pullRequest.(*SQLPullRequestStore).CreateIndexesIfNotExists()
	sqlStore.issue.(*SQLIssueStore).CreateIndexesIfNotExists()
	sqlStore.spinmint.(*SQLSpinmintStore).CreateIndexesIfNotExists()

	return sqlStore
}

func (ss *SQLStore) DoesTableExist(tableName string) bool {
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
		os.Exit(exitTableExistsMySQL)
	}

	return count > 0
}

func (ss *SQLStore) DoesColumnExist(tableName string, columnName string) bool {
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
		os.Exit(exitColumExistsMySQL)
	}

	return count > 0
}

func (ss *SQLStore) CreateColumnIfNotExists(tableName string, columnName string, mySQLColType string, postgresColType string, defaultValue string) bool {
	if ss.DoesColumnExist(tableName, columnName) {
		return false
	}

	_, err := ss.GetMaster().Exec("ALTER TABLE " + tableName + " ADD " + columnName + " " + mySQLColType + " DEFAULT '" + defaultValue + "'")
	if err != nil {
		mlog.Critical("failed to create column if not exists: %v", mlog.Err(err))
		time.Sleep(time.Second)
		os.Exit(exitCreateColumMySQL)
	}

	return true
}

func (ss *SQLStore) RemoveColumnIfExists(tableName string, columnName string) bool {
	if !ss.DoesColumnExist(tableName, columnName) {
		return false
	}

	_, err := ss.GetMaster().Exec("ALTER TABLE " + tableName + " DROP COLUMN " + columnName)
	if err != nil {
		mlog.Critical("failed to remove column if exists", mlog.Err(err))
		time.Sleep(time.Second)
		os.Exit(exitRemoveColumn)
	}

	return true
}

func (ss *SQLStore) RenameColumnIfExists(tableName string, oldColumnName string, newColumnName string, colType string) bool {
	if !ss.DoesColumnExist(tableName, oldColumnName) {
		return false
	}

	_, err := ss.GetMaster().Exec("ALTER TABLE " + tableName + " CHANGE " + oldColumnName + " " + newColumnName + " " + colType)

	if err != nil {
		mlog.Critical("failed to rename column if exists", mlog.Err(err))
		time.Sleep(time.Second)
		os.Exit(exitRenameColumn)
	}

	return true
}

func (ss *SQLStore) GetMaxLengthOfColumnIfExists(tableName string, columnName string) string {
	if !ss.DoesColumnExist(tableName, columnName) {
		return ""
	}

	result, err := ss.GetMaster().SelectStr("SELECT CHARACTER_MAXIMUM_LENGTH FROM information_schema.columns WHERE table_name = '?' AND COLUMN_NAME = '?'", tableName, columnName)
	if err != nil {
		mlog.Critical("failed to get max length of column if exists", mlog.Err(err))
		time.Sleep(time.Second)
		os.Exit(exitMaxColumn)
	}

	return result
}

func (ss *SQLStore) AlterColumnTypeIfExists(tableName string, columnName string, mySQLColType string, postgresColType string) bool {
	if !ss.DoesColumnExist(tableName, columnName) {
		return false
	}

	_, err := ss.GetMaster().Exec("ALTER TABLE " + tableName + " MODIFY " + columnName + " " + mySQLColType)

	if err != nil {
		mlog.Critical("failed to alter column type if exists", mlog.Err(err))
		time.Sleep(time.Second)
		os.Exit(exitAlterColumn)
	}

	return true
}

func (ss *SQLStore) CreateUniqueIndexIfNotExists(indexName string, tableName string, columnName string) bool {
	return ss.createIndexIfNotExists(indexName, tableName, columnName, indexTypeDefault, true)
}

func (ss *SQLStore) CreateIndexIfNotExists(indexName string, tableName string, columnName string) bool {
	return ss.createIndexIfNotExists(indexName, tableName, columnName, indexTypeDefault, false)
}

func (ss *SQLStore) CreateFullTextIndexIfNotExists(indexName string, tableName string, columnName string) bool {
	return ss.createIndexIfNotExists(indexName, tableName, columnName, indexTypeFullText, false)
}

func (ss *SQLStore) createIndexIfNotExists(indexName string, tableName string, columnName string, indexType string, unique bool) bool {
	uniqueStr := ""
	if unique {
		uniqueStr = "UNIQUE "
	}

	count, err := ss.GetMaster().SelectInt("SELECT COUNT(0) AS index_exists FROM information_schema.statistics WHERE TABLE_SCHEMA = DATABASE() and table_name = ? AND index_name = ?", tableName, indexName)
	if err != nil {
		mlog.Critical("can't check for index", mlog.Err(err))
		time.Sleep(time.Second)
		os.Exit(exitCreateIndexMySQL)
	}

	if count > 0 {
		return false
	}

	fullTextIndex := ""
	if indexType == indexTypeFullText {
		fullTextIndex = " FULLTEXT "
	}

	_, err = ss.GetMaster().Exec("CREATE  " + uniqueStr + fullTextIndex + " INDEX " + indexName + " ON " + tableName + " (" + columnName + ")")
	if err != nil {
		mlog.Critical("failed to create index if not exists", mlog.Err(err))
		time.Sleep(time.Second)
		os.Exit(exitCreateIndexFullMySQL)
	}

	return true
}

func (ss *SQLStore) RemoveIndexIfExists(indexName string, tableName string) bool {
	count, err := ss.GetMaster().SelectInt("SELECT COUNT(0) AS index_exists FROM information_schema.statistics WHERE TABLE_SCHEMA = DATABASE() and table_name = ? AND index_name = ?", tableName, indexName)
	if err != nil {
		mlog.Critical("can't check index to remove", mlog.Err(err))
		time.Sleep(time.Second)
		os.Exit(exitRemoveIndexMySQL)
	}

	if count <= 0 {
		return false
	}

	_, err = ss.GetMaster().Exec("DROP INDEX " + indexName + " ON " + tableName)
	if err != nil {
		mlog.Critical("failed to remove index if exists", mlog.Err(err))
		time.Sleep(time.Second)
		os.Exit(exitRemoveIndexMySQL)
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
		mlog.Error("failed to drop all tabels", mlog.Err(err))
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
