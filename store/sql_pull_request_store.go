// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package store

import (
	"database/sql"
	"fmt"

	"github.com/mattermost/mattermost-mattermod/model"
)

type SQLPullRequestStore struct {
	*SQLStore
}

func NewSQLPullRequestStore(sqlStore *SQLStore) PullRequestStore {
	s := &SQLPullRequestStore{sqlStore}

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
		table.ColMap("CreatedAt").SetMaxSize(128)
	}

	return s
}

func (s SQLPullRequestStore) CreateIndexesIfNotExists() {
	s.CreateColumnIfNotExists("PullRequests", "Ref", "varchar(128)", "varchar(128)", "")
	s.CreateColumnIfNotExists("PullRequests", "Sha", "varchar(48)", "varchar(48)", "")
	s.CreateColumnIfNotExists("PullRequests", "State", "varchar(8)", "varchar(8)", "")
	s.CreateColumnIfNotExists("PullRequests", "BuildStatus", "varchar(20)", "varchar(20)", "")
	s.CreateColumnIfNotExists("PullRequests", "BuildConclusion", "varchar(20)", "varchar(20)", "")
	s.CreateColumnIfNotExists("PullRequests", "URL", "varchar(20)", "varchar(2083)", "")
	s.CreateColumnIfNotExists("PullRequests", "FullName", "varchar(2083)", "varchar(2083)", "")
	s.CreateColumnIfNotExists("PullRequests", "CreatedAt", "timestamp", "timestamp", "")
}

func (s SQLPullRequestStore) Save(pr *model.PullRequest) (*model.PullRequest, error) {
	if err := s.GetMaster().Insert(pr); err != nil {
		if _, err := s.GetMaster().Update(pr); err != nil {
			return nil, fmt.Errorf("could not insert or update PR: owner=%v, name=%v, number=%v, err=%w", pr.RepoOwner, pr.RepoName, pr.Number, err)
		}
	}
	return pr, nil
}

func (s SQLPullRequestStore) Get(repoOwner, repoName string, number int) (*model.PullRequest, error) {
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
			return nil, fmt.Errorf("could not get PR: owner=%v, name=%v, number=%v, err=%w", pr.RepoOwner, pr.RepoName, pr.Number, err)
		}
		return nil, nil // row not found.
	}
	return &pr, nil
}

func (s SQLPullRequestStore) ListOpen() ([]*model.PullRequest, error) {
	var prs []*model.PullRequest
	if _, err := s.GetReplica().Select(&prs,
		`SELECT
				*
			FROM
				PullRequests
			WHERE
				State = 'open'`); err != nil {
		return nil, fmt.Errorf("could not list open PRs: %w", err)
	}
	return prs, nil
}
