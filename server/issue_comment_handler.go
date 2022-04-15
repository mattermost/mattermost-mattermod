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

	"github.com/google/go-github/v39/github"
	"github.com/mattermost/mattermost-server/v5/mlog"
	"github.com/pkg/errors"
)

const (
	commenterNoPermissions = "commenter does not have permissions"
)

type issueCommentEvent struct {
	Comment    *github.PullRequestComment `json:"comment"`
	Issue      *github.Issue              `json:"issue"`
	Repository *github.Repository         `json:"repository"`
	Action     string                     `json:"action"`
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

	if ev.HasCloudFF() && ev.Action == "created" {
		issue, err2 := s.GetIssueFromGithub(ctx, ev.Issue)
		if err2 != nil {
			mlog.Error("could not get the issue from GitHub", mlog.Err(err2))
			http.Error(w, "could not get the issue from GitHub", http.StatusInternalServerError)
			return
		}
		var res *fastForwardResult
		res, err = s.performFastForwardProcess(ctx, issue, ev.Comment.GetBody(), *ev.Comment.GetUser().Login)
		if err != nil {
			mlog.Error("error while fast forwarding process", mlog.Err(err), mlog.Int("issue", issue.Number), mlog.String("Repo", issue.RepoName))
			_, _, err = s.GithubClient.Issues.CreateComment(ctx, issue.RepoOwner, issue.RepoName, issue.Number, &github.IssueComment{
				Body: github.String("Could not complete the fast-forward process w/o errors. Please perform a manual check on repositores."),
			})
			if err != nil {
				mlog.Error("error creating comment", mlog.Err(err), mlog.Int("issue", issue.Number), mlog.String("Repo", issue.RepoName))
				return
			}
		} else if res != nil {
			var comment string
			comment, err = executeFFSummary(res)
			if err != nil {
				mlog.Error("error creating summary", mlog.Err(err), mlog.Int("issue", issue.Number), mlog.String("Repo", issue.RepoName))
				return
			}
			_, _, err = s.GithubClient.Issues.CreateComment(ctx, issue.RepoOwner, issue.RepoName, issue.Number, &github.IssueComment{
				Body: github.String(comment),
			})
			if err != nil {
				mlog.Error("error creating comment", mlog.Err(err), mlog.Int("issue", issue.Number), mlog.String("Repo", issue.RepoName))
				return
			}
			return
		}
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
		s.Metrics.IncreaseWebhookRequest("cherry_pick")
		if err := s.handleCherryPick(ctx, commenter, ev.Comment.GetBody(), pr); err != nil {
			s.Metrics.IncreaseWebhookErrors("cherry_pick")
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

	if ev.HasE2ETest() {
		s.Metrics.IncreaseWebhookRequest("e2e_test")
		if err := s.handleE2ETest(ctx, commenter, pr, ev.Comment.GetBody()); err != nil {
			s.Metrics.IncreaseWebhookErrors("e2e_test")
			errs = append(errs, fmt.Errorf("error e2e test: %w", err))
		}
	}

	if ev.HasE2ECancel() {
		s.Metrics.IncreaseWebhookRequest("e2e_cancel")
		if err := s.handleE2ECancel(ctx, commenter, pr); err != nil {
			s.Metrics.IncreaseWebhookErrors("e2e_cancel")
			errs = append(errs, fmt.Errorf("error e2e cancel: %w", err))
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

// HasE2ETest is true if body is prefixed with "/e2e-test"
func (e *issueCommentEvent) HasE2ETest() bool {
	return strings.HasPrefix(strings.TrimSpace(e.Comment.GetBody()), "/e2e-test")
}

// HasE2ECancel is true if body is prefixed with "/e2e-cancel"
func (e *issueCommentEvent) HasE2ECancel() bool {
	return strings.HasPrefix(strings.TrimSpace(e.Comment.GetBody()), "/e2e-cancel")
}
