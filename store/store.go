// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package store

import (
	"time"

	l4g "github.com/alecthomas/log4go"
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
		l4g.Close()
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
	Delete(instanceId string) StoreChannel
	Get(prNumber int) StoreChannel
	List() StoreChannel
}
