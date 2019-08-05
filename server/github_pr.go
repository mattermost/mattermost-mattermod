// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"encoding/json"
	"io"

	"github.com/google/go-github/github"
)

type PullRequestEvent struct {
	Action        string              `json:"action"`
	PRNumber      int                 `json:"number"`
	PullRequest   *github.PullRequest `json:"pull_request"`
	Issue         *github.Issue       `json:"issue"`
	Label         *github.Label       `json:"label"`
	Repo          *github.Repository  `json:"repository"`
	RepositoryUrl string              `json:"repository_url"`
}

type IssueComment struct {
	Action     string                     `json:"action"`
	Comment    *github.PullRequestComment `json:"comment"`
	Issue      *github.Issue              `json:"issue"`
	Repository *github.Repository         `json:"repository"`
}

func PullRequestEventFromJson(data io.Reader) *PullRequestEvent {
	decoder := json.NewDecoder(data)
	var event PullRequestEvent
	if err := decoder.Decode(&event); err != nil {
		return nil
	}

	return &event
}

func IssueCommentFromJson(data io.Reader) *IssueComment {
	decoder := json.NewDecoder(data)
	var event IssueComment
	if err := decoder.Decode(&event); err != nil {
		return nil
	}

	return &event
}
