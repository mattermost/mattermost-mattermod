// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"encoding/json"
	"io"
	"strings"

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

type EventData struct {
	Action     string                     `json:"action"`
	Comment    *github.PullRequestComment `json:"comment"`
	Issue      *github.Issue              `json:"issue"`
	Repository *github.Repository         `json:"repository"`
}

// CheckCLA is true if body contains "/check-cla"
func (d *EventData) CheckCLA() bool {
	if d.Comment == nil || d.Comment.Body == nil {
		return false
	}
	return strings.Contains(strings.TrimSpace(*d.Comment.Body), "/check-cla")
}

// CherryPick is true if body contains "/cherry-pick"
func (d *EventData) CherryPick() bool {
	if d.Comment == nil || d.Comment.Body == nil {
		return false
	}
	return strings.Contains(strings.TrimSpace(*d.Comment.Body), "/cherry-pick")
}

// AutoAssign is true if body contains "/autoassign"
func (d *EventData) AutoAssign() bool {
	if d.Comment == nil || d.Comment.Body == nil {
		return false
	}
	return strings.Contains(strings.TrimSpace(*d.Comment.Body), "/autoassign")
}

// UpdateBranch is true if body contains "/update-branch"
func (d *EventData) UpdateBranch() bool {
	if d.Comment == nil || d.Comment.Body == nil {
		return false
	}
	return strings.Contains(strings.TrimSpace(*d.Comment.Body), "/update-branch")
}

func PullRequestEventFromJSON(data io.Reader) *PullRequestEvent {
	decoder := json.NewDecoder(data)
	var event PullRequestEvent
	if err := decoder.Decode(&event); err != nil {
		return nil
	}

	return &event
}

func EventDataFromJSON(data io.Reader) *EventData {
	decoder := json.NewDecoder(data)
	var event EventData
	if err := decoder.Decode(&event); err != nil {
		return nil
	}

	return &event
}

func PingEventFromJSON(data io.Reader) *github.PingEvent {
	decoder := json.NewDecoder(data)
	var event github.PingEvent
	if err := decoder.Decode(&event); err != nil {
		return nil
	}

	return &event
}
