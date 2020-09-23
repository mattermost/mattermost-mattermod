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

func (s SQLIssueStore) Save(issue *model.Issue) (*model.Issue, error) {
	if err := s.GetMaster().Insert(issue); err != nil {
		if _, err := s.GetMaster().Update(issue); err != nil {
			return nil, fmt.Errorf("could not insert or update issue: owner=%v, name=%v, number=%v, err=%w", issue.RepoOwner, issue.RepoName, issue.Number, err)
		}
	}
	return issue, nil
}

func (s SQLIssueStore) Get(repoOwner, repoName string, number int) (*model.Issue, error) {
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
			return nil, fmt.Errorf("could not get issue: owner=%v, name=%v, number=%v, err=%w", issue.RepoOwner, issue.RepoName, issue.Number, err)
		}
		return nil, nil // row not found.
	}
	return &issue, nil
}
