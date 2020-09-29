// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package model

type Spinmint struct {
	InstanceID string `db:"InstanceId"`
	RepoOwner  string
	RepoName   string
	Number     int
	CreatedAt  int64
}
