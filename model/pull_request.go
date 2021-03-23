// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package model

import (
	"time"
)

const (
	StateOpen   = "open"
	StateClosed = "closed"
)

type PullRequest struct {
	RepoOwner           string
	RepoName            string
	FullName            string
	Number              int
	Username            string
	Ref                 string
	Sha                 string
	Labels              StringArray
	State               string
	BuildStatus         string
	BuildConclusion     string
	BuildLink           string
	URL                 string
	CreatedAt           time.Time
	Merged              *bool
	MergeCommitSHA      string `db:"-"`
	MaintainerCanModify *bool
	MilestoneNumber     *int64
	MilestoneTitle      *string
}
