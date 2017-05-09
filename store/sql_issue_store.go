// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package store

import (
	"database/sql"
	"fmt"

	"github.com/mattermost/mattermost-mattermod/model"
)

type SqlIssueStore struct {
	*SqlStore
}

func NewSqlIssueStore(sqlStore *SqlStore) IssueStore {
	s := &SqlIssueStore{sqlStore}

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

func (s SqlIssueStore) CreateIndexesIfNotExists() {
	s.CreateColumnIfNotExists("Issues", "State", "varchar(8)", "varchar(8)", "")
}

func (s SqlIssueStore) Save(issue *model.Issue) StoreChannel {
	storeChannel := make(StoreChannel)

	go func() {
		result := StoreResult{}

		if err := s.GetMaster().Insert(issue); err != nil {
			if _, err := s.GetMaster().Update(issue); err != nil {
				result.Err = model.NewLocAppError("SqlIssueStore.Save", "Could not insert or update issue", nil,
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

func (s SqlIssueStore) Get(repoOwner, repoName string, number int) StoreChannel {
	storeChannel := make(StoreChannel)

	go func() {
		result := StoreResult{}

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

func (s SqlIssueStore) List() StoreChannel {
	storeChannel := make(StoreChannel)

	go func() {
		result := StoreResult{}

		var issues []*model.Issue
		if _, err := s.GetReplica().Select(&issues,
			`SELECT
				*
			FROM
				Issues`); err != nil {
			result.Err = model.NewLocAppError("SqlIssueStore.List", "Could not list issues", nil, err.Error())
		} else {
			result.Data = issues
		}

		storeChannel <- result
		close(storeChannel)
	}()

	return storeChannel
}

func (s SqlIssueStore) ListOpen() StoreChannel {
	storeChannel := make(StoreChannel)

	go func() {
		result := StoreResult{}

		var issues []*model.Issue
		if _, err := s.GetReplica().Select(&issues,
			`SELECT
				*
			FROM
				Issues
			WHERE
				State = 'open'`); err != nil {
			result.Err = model.NewLocAppError("SqlIssueStore.ListOpen", "Could not list open issues", nil, err.Error())
		} else {
			result.Data = issues
		}

		storeChannel <- result
		close(storeChannel)
	}()

	return storeChannel
}
