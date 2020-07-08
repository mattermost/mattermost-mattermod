// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package model

import (
	"database/sql"
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
	Labels              []string
	State               string
	BuildStatus         string
	BuildConclusion     string
	BuildLink           string
	URL                 string
	CreatedAt           time.Time
	MaintainerCanModify sql.NullBool
}
