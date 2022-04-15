// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package store

import (
	"context"

	"github.com/mattermost/mattermost-mattermod/model"
)

type Store interface {
	PullRequest() PullRequestStore
	Issue() IssueStore
	Close()
	DropAllTables()
	Mutex() LockStore
}

type PullRequestStore interface {
	Save(pr *model.PullRequest) (*model.PullRequest, error)
	Get(repoOwner, repoName string, number int) (*model.PullRequest, error)
	ListOpen() ([]*model.PullRequest, error)
}

type IssueStore interface {
	Save(issue *model.Issue) (*model.Issue, error)
	Get(repoOwner, repoName string, number int) (*model.Issue, error)
}

type LockStore interface {
	Lock(ctx context.Context) error
	Unlock() error
}
