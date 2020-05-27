// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package model

import (
	"encoding/json"
	"io"
)

type Spinmint struct {
	InstanceID string
	RepoOwner  string
	RepoName   string
	Number     int
	CreatedAt  int64
}

func (o *Spinmint) ToJSON() (string, error) {
	b, err := json.Marshal(o)
	if err != nil {
		return "", err
	}

	return string(b), nil
}

func SpinmintFromJSON(data io.Reader) (*Spinmint, error) {
	var pr Spinmint
	err := json.NewDecoder(data).Decode(&pr)
	if err != nil {
		return nil, err
	}

	return &pr, nil
}
