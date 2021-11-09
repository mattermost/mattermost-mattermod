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
	Merged              *bool
	MaintainerCanModify *bool
	MilestoneNumber     *int64
	MilestoneTitle      *string
	CreatedAt           time.Time
	RepoOwner           string
	RepoName            string
	FullName            string
	Username            string
	Ref                 string
	Sha                 string
	State               string
	BuildStatus         string
	BuildConclusion     string
	BuildLink           string
	URL                 string
	MergeCommitSHA      string `db:"-"`
	Labels              StringArray
	Number              int
}

// GetMerged returns the Merged field if it's non-nil, zero value otherwise.
func (pr *PullRequest) GetMerged() bool {
	if pr == nil || pr.Merged == nil {
		return false
	}
	return *pr.Merged
}

// GetMaintainerCanModify returns the MaintainerCanModify field if it's non-nil, zero value otherwise.
func (pr *PullRequest) GetMaintainerCanModify() bool {
	if pr == nil || pr.MaintainerCanModify == nil {
		return false
	}
	return *pr.MaintainerCanModify
}

// GetMilestoneNumber returns the MilestoneNumber field if it's non-nil, zero value otherwise.
func (pr *PullRequest) GetMilestoneNumber() int64 {
	if pr == nil || pr.MilestoneNumber == nil {
		return 0
	}
	return *pr.MilestoneNumber
}

// GetMilestoneTitle returns the MilestoneTitle field if it's non-nil, zero value otherwise.
func (pr *PullRequest) GetMilestoneTitle() string {
	if pr == nil || pr.MilestoneTitle == nil {
		return ""
	}
	return *pr.MilestoneTitle
}
