// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package store

import (
	"database/sql"
	"fmt"

	"github.com/mattermost/mattermost-mattermod/model"
)

type SqlPullRequestStore struct {
	*SqlStore
}

func NewSqlPullRequestStore(sqlStore *SqlStore) PullRequestStore {
	s := &SqlPullRequestStore{sqlStore}

	for _, db := range sqlStore.GetAllConns() {
		table := db.AddTableWithName(model.PullRequest{}, "PullRequests").SetKeys(false, "RepoOwner", "RepoName", "Number")
		table.ColMap("RepoOwner").SetMaxSize(128)
		table.ColMap("RepoName").SetMaxSize(128)
		table.ColMap("Username").SetMaxSize(128)
		table.ColMap("FullName").SetMaxSize(2083)
		table.ColMap("Ref").SetMaxSize(128)
		table.ColMap("Sha").SetMaxSize(48)
		table.ColMap("State").SetMaxSize(8)
		table.ColMap("Labels").SetMaxSize(1024)
		table.ColMap("BuildStatus").SetMaxSize(8)
		table.ColMap("BuildLink").SetMaxSize(256)
		table.ColMap("BuildConclusion").SetMaxSize(256)
		table.ColMap("URL").SetMaxSize(2083)
	}

	return s
}

func (s SqlPullRequestStore) CreateIndexesIfNotExists() {
	s.CreateColumnIfNotExists("PullRequests", "Ref", "varchar(128)", "varchar(128)", "")
	s.CreateColumnIfNotExists("PullRequests", "Sha", "varchar(48)", "varchar(48)", "")
	s.CreateColumnIfNotExists("PullRequests", "State", "varchar(8)", "varchar(8)", "")
	s.CreateColumnIfNotExists("PullRequests", "BuildStatus", "varchar(20)", "varchar(20)", "")
	s.CreateColumnIfNotExists("PullRequests", "BuildConclusion", "varchar(20)", "varchar(20)", "")
	s.CreateColumnIfNotExists("PullRequests", "URL", "varchar(20)", "varchar(2083)", "")
	s.CreateColumnIfNotExists("PullRequests", "FullName", "varchar(2083)", "varchar(2083)", "")
}

func (s SqlPullRequestStore) Save(pr *model.PullRequest) StoreChannel {
	storeChannel := make(StoreChannel)

	go func() {
		result := StoreResult{}

		if err := s.GetMaster().Insert(pr); err != nil {
			if _, err := s.GetMaster().Update(pr); err != nil {
				result.Err = model.NewLocAppError("SqlPullRequestStore.Save", "Could not insert or update PR", nil,
					fmt.Sprintf("owner=%v, name=%v, number=%v, err=%v", pr.RepoOwner, pr.RepoName, pr.Number, err.Error()))
			}
		}

		if result.Err == nil {
			result.Data = pr
		}

		storeChannel <- result
		close(storeChannel)
	}()

	return storeChannel
}

func (s SqlPullRequestStore) Get(repoOwner, repoName string, number int) StoreChannel {
	storeChannel := make(StoreChannel)

	go func() {
		result := StoreResult{}

		var pr model.PullRequest
		if err := s.GetReplica().SelectOne(&pr,
			`SELECT
				*
			FROM
				PullRequests
			WHERE
				RepoOwner = :RepoOwner
				AND RepoName = :RepoName
				AND Number = :Number`, map[string]interface{}{"Number": number, "RepoOwner": repoOwner, "RepoName": repoName}); err != nil {
			if err != sql.ErrNoRows {
				result.Err = model.NewLocAppError("SqlPullRequestStore.Get", "Could not get PR", nil,
					fmt.Sprintf("owner=%v, name=%v, number=%v, err=%v", pr.RepoOwner, pr.RepoName, pr.Number, err.Error()))
			} else {
				result.Data = nil
			}
		} else {
			result.Data = &pr
		}

		storeChannel <- result
		close(storeChannel)
	}()

	return storeChannel
}

func (s SqlPullRequestStore) List() StoreChannel {
	storeChannel := make(StoreChannel)

	go func() {
		result := StoreResult{}

		var prs []*model.PullRequest
		if _, err := s.GetReplica().Select(&prs,
			`SELECT
				*
			FROM
				PullRequests`); err != nil {
			result.Err = model.NewLocAppError("SqlPullRequestStore.List", "Could not list PRs", nil, err.Error())
		} else {
			result.Data = prs
		}

		storeChannel <- result
		close(storeChannel)
	}()

	return storeChannel
}

func (s SqlPullRequestStore) ListOpen() StoreChannel {
	storeChannel := make(StoreChannel)

	go func() {
		result := StoreResult{}

		var prs []*model.PullRequest
		if _, err := s.GetReplica().Select(&prs,
			`SELECT
				*
			FROM
				PullRequests
			WHERE
				State = 'open'`); err != nil {
			result.Err = model.NewLocAppError("SqlPullRequestStore.ListOpen", "Could not list openPRs", nil, err.Error())
		} else {
			result.Data = prs
		}

		storeChannel <- result
		close(storeChannel)
	}()

	return storeChannel
}
