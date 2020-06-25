// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package store

import (
	"github.com/mattermost/mattermost-mattermod/model"
)

type Store interface {
	PullRequest() PullRequestStore
	Issue() IssueStore
	Spinmint() SpinmintStore
	Close()
	DropAllTables()
}

type PullRequestStore interface {
	Save(pr *model.PullRequest) (*model.PullRequest, *model.AppError)
	Get(repoOwner, repoName string, number int) (*model.PullRequest, *model.AppError)
	ListOpen() ([]*model.PullRequest, *model.AppError)
}

type IssueStore interface {
	Save(issue *model.Issue) (*model.Issue, *model.AppError)
	Get(repoOwner, repoName string, number int) (*model.Issue, *model.AppError)
}

type SpinmintStore interface {
	Save(spinmint *model.Spinmint) (*model.Spinmint, *model.AppError)
	Delete(instanceID string) ([]*model.Spinmint, *model.AppError)
	Get(prNumber int, repoName string) (*model.Spinmint, *model.AppError)
	List() ([]*model.Spinmint, *model.AppError)
}
