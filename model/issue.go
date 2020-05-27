// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package model

import (
	"encoding/json"
	"io"
)

type Issue struct {
	RepoOwner string
	RepoName  string
	Number    int
	Username  string
	State     string
	Labels    []string
}

func (o *Issue) ToJSON() (string, error) {
	b, err := json.Marshal(o)
	if err != nil {
		return "", err
	}

	return string(b), nil
}

func IssueFromJSON(data io.Reader) (*Issue, error) {
	var issue Issue
	err := json.NewDecoder(data).Decode(&issue)
	if err != nil {
		return nil, err
	}

	return &issue, nil
}
