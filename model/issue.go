// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package model

import (
	"github.com/mattermost/mattermost-mattermod/types"
)

type Issue struct {
	RepoOwner string
	RepoName  string
	Number    int
	Username  string
	State     string
	Labels    types.JSONText
}
