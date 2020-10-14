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
	return &SQLPullRequestStore{sqlStore}
}

func (s SQLPullRequestStore) Save(pr *model.PullRequest) (*model.PullRequest, error) {
	if _, err := s.dbx.NamedExec(
		`INSERT INTO PullRequests
			(RepoOwner, RepoName, FullName, Number, Username, Ref, Sha, Labels, State, BuildStatus, BuildConclusion, BuildLink,
				URL, CreatedAt, MaintainerCanModify, Merged)
		VALUES
			(:RepoOwner, :RepoName, :FullName, :Number, :Username, :Ref, :Sha, :Labels, :State, :BuildStatus, :BuildConclusion, :BuildLink,
				:URL, :CreatedAt, :MaintainerCanModify, :Merged)`, pr); err != nil {
		if _, err := s.dbx.NamedExec(
			`UPDATE PullRequests
			 SET FullName = :FullName, Username = :Username, Ref = :Ref, Sha = :Sha, Labels = :Labels,
				 State = :State, BuildStatus = :BuildStatus, BuildConclusion = :BuildConclusion, BuildLink = :BuildLink,
				 URL = :URL, CreatedAt = :CreatedAt, MaintainerCanModify = :MaintainerCanModify, Merged = :Merged
			 WHERE RepoOwner = :RepoOwner AND RepoName = :RepoName AND Number = :Number`, pr); err != nil {
			return nil, fmt.Errorf("could not insert or update PR: owner=%v, name=%v, number=%v, err=%w", pr.RepoOwner, pr.RepoName, pr.Number, err)
		}
	}
	return pr, nil
}

func (s SQLPullRequestStore) Get(repoOwner, repoName string, number int) (*model.PullRequest, error) {
	var pr model.PullRequest
	if err := s.dbx.Get(&pr,
		`SELECT
				*
			FROM
				PullRequests
			WHERE
				RepoOwner = ?
				AND RepoName = ?
				AND Number = ?`, repoOwner, repoName, number); err != nil {
		if err != sql.ErrNoRows {
			return nil, fmt.Errorf("could not get PR: owner=%v, name=%v, number=%v, err=%w", pr.RepoOwner, pr.RepoName, pr.Number, err)
		}
		return nil, nil // row not found.
	}
	return &pr, nil
}

func (s SQLPullRequestStore) ListOpen() ([]*model.PullRequest, error) {
	var prs []*model.PullRequest
	if err := s.dbx.Select(&prs,
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
