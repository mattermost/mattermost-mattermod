// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package store

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/mattermost/mattermost-mattermod/model"
)

type SQLPullRequestStore struct {
	*SQLStore
	insertQuery string
	updateQuery string
}

func NewSQLPullRequestStore(sqlStore *SQLStore) PullRequestStore {
	s := &SQLPullRequestStore{SQLStore: sqlStore}
	var b strings.Builder
	b.WriteString("INSERT INTO PullRequests ")
	b.WriteString("(RepoOwner, RepoName, FullName, Number, Username, Ref, Sha, Labels, State, BuildStatus, ")
	b.WriteString("BuildConclusion, BuildLink, URL, CreatedAt, Merged, MaintainerCanModify) ")
	b.WriteString("VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)")
	s.insertQuery = b.String()

	b.Reset()
	b.WriteString("UPDATE PullRequests SET FullName = $1, Username = $2, Ref = $3, Sha = $4, Labels = $5, ")
	b.WriteString("State = $6, BuildStatus = $7, BuildConclusion = $8, BuildLink = $9, URL = $10, CreatedAt = $11, Merged = $12, ")
	b.WriteString("MaintainerCanModify = $13 WHERE RepoOwner = $14 AND RepoName = $15 AND Number = $16")
	s.updateQuery = b.String()

	return s
}

func (s SQLPullRequestStore) Save(pr *model.PullRequest) (*model.PullRequest, error) {
	if _, err := s.dbx.Exec(s.insertQuery, pr.RepoOwner, pr.RepoName, pr.FullName, pr.Number, pr.Username, pr.Ref, pr.Sha, pr.Labels, pr.State,
		pr.BuildStatus, pr.BuildConclusion, pr.BuildLink, pr.URL, pr.CreatedAt, pr.Merged, pr.MaintainerCanModify); err != nil {
		if _, err := s.dbx.Exec(s.updateQuery, pr.FullName, pr.Username, pr.Ref, pr.Sha, pr.Labels, pr.State, pr.BuildStatus, pr.BuildConclusion,
			pr.BuildLink, pr.URL, pr.CreatedAt, pr.Merged, pr.MaintainerCanModify, pr.RepoOwner, pr.RepoName, pr.Number); err != nil {
			return nil, fmt.Errorf("could not insert or update PR: owner=%v, name=%v, number=%v, err=%w", pr.RepoOwner, pr.RepoName, pr.Number, err)
		}
	}
	return pr, nil
}

func (s SQLPullRequestStore) Get(repoOwner, repoName string, number int) (*model.PullRequest, error) {
	var pr model.PullRequest
	// s.dbx.Get()
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
	// s.dbx.Select
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
