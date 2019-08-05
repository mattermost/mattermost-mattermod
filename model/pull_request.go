// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package model

import (
	"encoding/json"
	"io"
)

const (
	STATE_OPEN   = "open"
	STATE_CLOSED = "closed"
)

type PullRequest struct {
	RepoOwner       string
	RepoName        string
	Number          int
	Username        string
	Ref             string
	Sha             string
	Labels          []string
	State           string
	BuildStatus     string
	BuildConclusion string
	BuildLink       string
	URL             string
}

func (o *PullRequest) ToJson() (string, error) {
	if b, err := json.Marshal(o); err != nil {
		return "", err
	} else {
		return string(b), nil
	}
}

func PullRequestFromJson(data io.Reader) (*PullRequest, error) {
	var pr PullRequest

	if err := json.NewDecoder(data).Decode(&pr); err != nil {
		return nil, err
	} else {
		return &pr, nil
	}
}
