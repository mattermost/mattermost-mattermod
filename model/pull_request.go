// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package model

import (
	"encoding/json"
	"io"
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
	MaintainerCanModify bool
}

func (o *PullRequest) ToJSON() (string, error) {
	b, err := json.Marshal(o)
	if err != nil {
		return "", err
	}

	return string(b), nil
}

func PullRequestFromJSON(data io.Reader) (*PullRequest, error) {
	var pr PullRequest
	err := json.NewDecoder(data).Decode(&pr)
	if err != nil {
		return nil, err
	}

	return &pr, nil
}
