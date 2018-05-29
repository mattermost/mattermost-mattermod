// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package model

import (
	"encoding/json"
	"io"
)

type Spinmint struct {
	InstanceId string
	RepoOwner  string
	RepoName   string
	Number     int
	CreatedAt  int64
}

func (o *Spinmint) ToJson() (string, error) {
	if b, err := json.Marshal(o); err != nil {
		return "", err
	} else {
		return string(b), nil
	}
}

func SpinmintFromJson(data io.Reader) (*Spinmint, error) {
	var pr Spinmint

	if err := json.NewDecoder(data).Decode(&pr); err != nil {
		return nil, err
	} else {
		return &pr, nil
	}
}
