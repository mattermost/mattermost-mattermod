// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/go-github/v33/github"
	"github.com/mattermost/mattermost-server/v5/mlog"
	"github.com/pkg/errors"
)

type issueCommentEvent struct {
	Action     string                     `json:"action"`
	Comment    *github.PullRequestComment `json:"comment"`
	Issue      *github.Issue              `json:"issue"`
	Repository *github.Repository         `json:"repository"`
}

func (s *Server) issueCommentEventHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout*time.Second)
	defer cancel()

	ev, err := issueCommentEventFromJSON(r.Body)
	if err != nil {
		mlog.Error("could not parse pr comment event", mlog.Err(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// We ignore comments from issues.
	if !ev.Issue.IsPullRequest() {
		return
	}

	// We ignore deletion events for now.
	if ev.Action == "deleted" {
		return
	}

	pr, err := s.getPRFromIssueCommentEvent(ctx, ev)
	if err != nil {
		mlog.Error("Error getting PR from Comment", mlog.Err(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	commenter := ev.Comment.GetUser().GetLogin()

	errs := make([]error, 0)

	if ev.HasCheckCLA() {
		s.Metrics.IncreaseWebhookRequest("check_cla")
		if _, err := s.handleCheckCLA(ctx, pr); err != nil {
			s.Metrics.IncreaseWebhookErrors("check_cla")
			errs = append(errs, fmt.Errorf("error checking CLA: %w", err))
		}
	}

	if ev.HasCherryPick() {
		s.Metrics.IncreaseWebhookRequest(CHERRY_PICK)
		if err := s.handleCommandRequest(ctx, commenter, CHERRY_PICK, ev.Comment.GetBody(), pr); err != nil {
			s.Metrics.IncreaseWebhookErrors(CHERRY_PICK)
			errs = append(errs, fmt.Errorf("error cherry picking: %w", err))
		}
	}

	if ev.HasAutoAssign() {
		s.Metrics.IncreaseWebhookRequest("auto_assign")
		if err := s.handleAutoAssign(ctx, ev.Comment.GetHTMLURL(), pr); err != nil {
			s.Metrics.IncreaseWebhookErrors("auto_assign")
			errs = append(errs, fmt.Errorf("error auto assigning: %w", err))
		}
	}

	if ev.HasUpdateBranch() {
		s.Metrics.IncreaseWebhookRequest("update_branch")
		if err := s.handleUpdateBranch(ctx, commenter, pr); err != nil {
			s.Metrics.IncreaseWebhookErrors("update_branch")
			errs = append(errs, fmt.Errorf("error updating branch: %w", err))
		}
	}

	if ev.HasLocalImports() {
		s.Metrics.IncreaseWebhookRequest("goimports-local")
		if err := s.handleCommandRequest(ctx, commenter, "goimports-local", ev.Comment.GetBody(), pr); err != nil {
			s.Metrics.IncreaseWebhookErrors("goimports-local")
			errs = append(errs, fmt.Errorf("error running goimports-local: %w", err))
		}
	}

	for _, err := range errs {
		mlog.Error("Error handling PR comment", mlog.Err(err))
	}

	if len(errs) > 0 {
		http.Error(w, "Error handling PR comment", http.StatusInternalServerError)
	}
}

func issueCommentEventFromJSON(data io.Reader) (*issueCommentEvent, error) {
	decoder := json.NewDecoder(data)
	var pr issueCommentEvent
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
func (e *issueCommentEvent) HasCheckCLA() bool {
	return strings.Contains(strings.TrimSpace(e.Comment.GetBody()), "/check-cla")
}

// HasCherryPick is true if body contains "/cherry-pick"
func (e *issueCommentEvent) HasCherryPick() bool {
	return strings.Contains(strings.TrimSpace(e.Comment.GetBody()), "/cherry-pick")
}

// HasAutoAssign is true if body contains "/autoassign"
func (e *issueCommentEvent) HasAutoAssign() bool {
	return strings.Contains(strings.TrimSpace(e.Comment.GetBody()), "/autoassign")
}

// HasUpdateBranch is true if body contains "/update-branch"
func (e *issueCommentEvent) HasUpdateBranch() bool {
	return strings.Contains(strings.TrimSpace(e.Comment.GetBody()), "/update-branch")
}

// HasUpdateBranch is true if body contains "/goimports-local"
func (e *issueCommentEvent) HasLocalImports() bool {
	return strings.Contains(strings.TrimSpace(e.Comment.GetBody()), "/goimports-local")
}
