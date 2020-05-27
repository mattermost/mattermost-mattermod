// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package store

import (
	"time"

	"github.com/mattermost/mattermost-mattermod/model"
)

type Result struct {
	Data interface{}
	Err  *model.AppError
}

type Channel chan Result

func Must(sc Channel) interface{} {
	r := <-sc
	if r.Err != nil {
		time.Sleep(time.Second)
		panic(r.Err)
	}

	return r.Data
}

type Store interface {
	PullRequest() PullRequestStore
	Issue() IssueStore
	Spinmint() SpinmintStore
	Close()
	DropAllTables()
}

type PullRequestStore interface {
	Save(pr *model.PullRequest) Channel
	Get(repoOwner, repoName string, number int) Channel
	ListOpen() Channel
}

type IssueStore interface {
	Save(issue *model.Issue) Channel
	Get(repoOwner, repoName string, number int) Channel
}

type SpinmintStore interface {
	Save(spinmint *model.Spinmint) Channel
	Delete(instanceID string) Channel
	Get(prNumber int, repoName string) Channel
	List() Channel
}
