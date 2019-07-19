// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package store

import (
	"time"

	"github.com/mattermost/mattermost-mattermod/model"
)

type StoreResult struct {
	Data interface{}
	Err  *model.AppError
}

type StoreChannel chan StoreResult

func Must(sc StoreChannel) interface{} {
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
	Save(pr *model.PullRequest) StoreChannel
	Get(repoOwner, repoName string, number int) StoreChannel
	List() StoreChannel
	ListOpen() StoreChannel
}

type IssueStore interface {
	Save(issue *model.Issue) StoreChannel
	Get(repoOwner, repoName string, number int) StoreChannel
	List() StoreChannel
	ListOpen() StoreChannel
}

type SpinmintStore interface {
	Save(spinmint *model.Spinmint) StoreChannel
	Delete(instanceID string) StoreChannel
	Get(prNumber int, repoName string) StoreChannel
	GetTestServer(instanceID string) StoreChannel
	List() StoreChannel
}
