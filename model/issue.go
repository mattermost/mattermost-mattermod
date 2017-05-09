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

func (o *Issue) ToJson() (string, error) {
	if b, err := json.Marshal(o); err != nil {
		return "", err
	} else {
		return string(b), nil
	}
}

func IssueFromJson(data io.Reader) (*Issue, error) {
	var issue Issue

	if err := json.NewDecoder(data).Decode(&issue); err != nil {
		return nil, err
	} else {
		return &issue, nil
	}
}
