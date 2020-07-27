// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/go-github/v32/github"
	"github.com/mattermost/mattermost-server/v5/mlog"
	"github.com/pkg/errors"
)

type prCommentEvent struct {
	Action     string                     `json:"action"`
	Comment    *github.PullRequestComment `json:"comment"`
	Issue      *github.Issue              `json:"issue"`
	Repository *github.Repository         `json:"repository"`
}

func (s *Server) prCommentEventHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout*time.Second)
	defer cancel()

	ev, err := prCommentEventFromJSON(r.Body)
	if err != nil {
		mlog.Error("could not parse pr comment event", mlog.Err(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	pr, err := s.getPRFromEvent(ctx, ev)
	if err != nil {
		mlog.Error("Error getting PR from Comment", mlog.Err(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var commenter string
	if ev.Comment != nil {
		commenter = ev.Comment.GetUser().GetLogin()
	}

	if ev.HasCheckCLA() {
		s.Metrics.IncreaseWebhookRequest("check_cla")
		if err := s.handleCheckCLA(ctx, pr); err != nil {
			s.Metrics.IncreaseWebhookErrors("check_cla")
			mlog.Error("Error checking CLA", mlog.Err(err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}

	if ev.HasCherryPick() {
		s.Metrics.IncreaseWebhookRequest("cherry_pick")
		if err := s.handleCherryPick(ctx, commenter, ev.Comment.GetBody(), pr); err != nil {
			s.Metrics.IncreaseWebhookErrors("cherry_pick")
			mlog.Error("Error cherry picking", mlog.Err(err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}

	if ev.HasAutoAssign() {
		s.Metrics.IncreaseWebhookRequest("auto_assign")
		if err := s.handleAutoAssign(ctx, ev.Comment.GetHTMLURL(), pr); err != nil {
			s.Metrics.IncreaseWebhookErrors("auto_assign")
			mlog.Error("Error auto assigning", mlog.Err(err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}

	if ev.HasUpdateBranch() {
		s.Metrics.IncreaseWebhookRequest("update_branch")
		if err := s.handleUpdateBranch(ctx, commenter, pr); err != nil {
			s.Metrics.IncreaseWebhookErrors("update_branch")
			mlog.Error("Error updating branch", mlog.Err(err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func prCommentEventFromJSON(data io.Reader) (*prCommentEvent, error) {
	decoder := json.NewDecoder(data)
	var pr prCommentEvent
	if err := decoder.Decode(&pr); err != nil {
		return nil, err
	}

	if pr.Comment == nil {
		return nil, errors.New("comment is missing from body")
	}
	if pr.Issue == nil {
		return nil, errors.New("issue is missing from body")
	}
	if pr.Repository == nil {
		return nil, errors.New("repository is missing from body")
	}

	return &pr, nil
}

// HasCheckCLA is true if body contains "/check-cla"
func (e *prCommentEvent) HasCheckCLA() bool {
	return strings.Contains(strings.TrimSpace(e.Comment.GetBody()), "/check-cla")
}

// HasCherryPick is true if body contains "/cherry-pick"
func (e *prCommentEvent) HasCherryPick() bool {
	return strings.Contains(strings.TrimSpace(e.Comment.GetBody()), "/cherry-pick")
}

// HasAutoAssign is true if body contains "/autoassign"
func (e *prCommentEvent) HasAutoAssign() bool {
	return strings.Contains(strings.TrimSpace(e.Comment.GetBody()), "/autoassign")
}

// HasUpdateBranch is true if body contains "/update-branch"
func (e *prCommentEvent) HasUpdateBranch() bool {
	return strings.Contains(strings.TrimSpace(e.Comment.GetBody()), "/update-branch")
}
