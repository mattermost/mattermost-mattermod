// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"encoding/json"
	"errors"
	"io"

	"github.com/google/go-github/v32/github"
)

type PullRequestEvent struct {
	Action        string              `json:"action"`
	PRNumber      int                 `json:"number"`
	PullRequest   *github.PullRequest `json:"pull_request"`
	Issue         *github.Issue       `json:"issue"`
	Label         *github.Label       `json:"label"`
	Repo          *github.Repository  `json:"repository"`
	RepositoryURL string              `json:"repository_url"`
}

func PullRequestEventFromJSON(data io.Reader) (*PullRequestEvent, error) {
	decoder := json.NewDecoder(data)
	var event PullRequestEvent
	if err := decoder.Decode(&event); err != nil {
		return nil, err
	}

	if event.Issue == nil {
		return nil, errors.New("event issue is missing from body")
	}

	return &event, nil
}

func PingEventFromJSON(data io.Reader) *github.PingEvent {
	decoder := json.NewDecoder(data)
	var event github.PingEvent
	if err := decoder.Decode(&event); err != nil {
		return nil
	}

	return &event
}
