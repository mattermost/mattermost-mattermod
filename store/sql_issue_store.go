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
	return &SQLIssueStore{sqlStore}
}

func (s SQLIssueStore) Save(issue *model.Issue) (*model.Issue, error) {
	if _, err := s.dbx.NamedExec(
		`INSERT INTO Issues
			(RepoOwner, RepoName, Number, Username, State, Labels)
		VALUES
			(:RepoOwner, :RepoName, :Number, :Username, :State, :Labels)`, issue); err != nil {
		if _, err := s.dbx.NamedExec(
			`UPDATE Issues
			 SET Username = :Username, State = :State, Labels = :Labels
			 WHERE RepoOwner = :RepoOwner AND RepoName = :RepoName AND Number = :Number`, issue); err != nil {
			return nil, fmt.Errorf("could not insert or update issue: owner=%v, name=%v, number=%v, err=%w", issue.RepoOwner, issue.RepoName, issue.Number, err)
		}
	}
	return issue, nil
}

func (s SQLIssueStore) Get(repoOwner, repoName string, number int) (*model.Issue, error) {
	var issue model.Issue
	if err := s.dbx.Get(&issue,
		`SELECT
				*
			FROM
				Issues
			WHERE
				RepoOwner = ?
				AND RepoName = ?
				AND Number = ?`, repoOwner, repoName, number); err != nil {
		if err != sql.ErrNoRows {
			return nil, fmt.Errorf("could not get issue: owner=%v, name=%v, number=%v, err=%w", issue.RepoOwner, issue.RepoName, issue.Number, err)
		}
		return nil, nil // row not found.
	}
	return &issue, nil
}
