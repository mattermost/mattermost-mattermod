// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package store

import (
	"database/sql"
	"fmt"

	"github.com/mattermost/mattermost-mattermod/model"
)

type SQLIssueStore struct {
	*SQLStore
}

func NewSQLIssueStore(sqlStore *SQLStore) IssueStore {
	s := &SQLIssueStore{sqlStore}

	for _, db := range sqlStore.GetAllConns() {
		table := db.AddTableWithName(model.Issue{}, "Issues").SetKeys(false, "RepoOwner", "RepoName", "Number")
		table.ColMap("RepoOwner").SetMaxSize(128)
		table.ColMap("RepoName").SetMaxSize(128)
		table.ColMap("Username").SetMaxSize(128)
		table.ColMap("State").SetMaxSize(8)
		table.ColMap("Labels").SetMaxSize(1024)
	}

	return s
}

func (s SQLIssueStore) CreateIndexesIfNotExists() {
	s.CreateColumnIfNotExists("Issues", "State", "varchar(8)", "varchar(8)", "")
}

func (s SQLIssueStore) Save(issue *model.Issue) Channel {
	storeChannel := make(Channel)

	go func() {
		result := Result{}

		if err := s.GetMaster().Insert(issue); err != nil {
			if _, err := s.GetMaster().Update(issue); err != nil {
				result.Err = model.NewLocAppError("SQLIssueStore.Save", "Could not insert or update issue", nil,
					fmt.Sprintf("owner=%v, name=%v, number=%v, err=%v", issue.RepoOwner, issue.RepoName, issue.Number, err.Error()))
			}
		}

		if result.Err == nil {
			result.Data = issue
		}

		storeChannel <- result
		close(storeChannel)
	}()

	return storeChannel
}

func (s SQLIssueStore) Get(repoOwner, repoName string, number int) Channel {
	storeChannel := make(Channel)

	go func() {
		result := Result{}

		var issue model.Issue
		if err := s.GetReplica().SelectOne(&issue,
			`SELECT
				*
			FROM
				Issues
			WHERE
				RepoOwner = :RepoOwner
				AND RepoName = :RepoName
				AND Number = :Number`, map[string]interface{}{"Number": number, "RepoOwner": repoOwner, "RepoName": repoName}); err != nil {
			if err != sql.ErrNoRows {
				result.Err = model.NewLocAppError("SqlPrStore.Get", "Could not get issue", nil,
					fmt.Sprintf("owner=%v, name=%v, number=%v, err=%v", issue.RepoOwner, issue.RepoName, issue.Number, err.Error()))
			} else {
				result.Data = nil
			}
		} else {
			result.Data = &issue
		}

		storeChannel <- result
		close(storeChannel)
	}()

	return storeChannel
}
