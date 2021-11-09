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
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v5/mlog"
	"github.com/pkg/errors"
)

type issueEvent struct {
	Label  *github.Label      `json:"label"`
	Repo   *github.Repository `json:"repository"`
	Issue  *github.Issue      `json:"issue"`
	Action string             `json:"action"`
}

func (s *Server) issueEventHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout*time.Second)
	defer cancel()

	event, err := issueEventFromJSON(r.Body)
	if err != nil {
		mlog.Error("could not parse issue event", mlog.Err(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	mlog.Info("handle issue event",
		mlog.String("repoUrl", event.Issue.GetHTMLURL()),
		mlog.String("Action", event.Action),
		mlog.Int("Issue number", event.Issue.GetNumber()))

	issue, err := s.GetIssueFromGithub(ctx, event.Issue)
	if err != nil {
		mlog.Error("could not get the issue from GitHub", mlog.Err(err))
		http.Error(w, "could not get the issue from GitHub", http.StatusInternalServerError)
		return
	}

	if err := s.checkIssueForChanges(ctx, issue); err != nil {
		mlog.Error("could not check issue for changes", mlog.Err(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) checkIssueForChanges(ctx context.Context, issue *model.Issue) error {
	oldIssue, err := s.Store.Issue().Get(issue.RepoOwner, issue.RepoName, issue.Number)
	if err != nil {
		return err
	}

	if oldIssue == nil {
		// TODO: since there is no old entity, we are simply saving the issue and
		// returning here. However, this logic should be reviewed: MM-27307
		_, err = s.Store.Issue().Save(issue)
		return err
	}

	hasChanges := oldIssue.State != issue.State

	for _, label := range issue.Labels {
		hadLabel := false

		for _, oldLabel := range oldIssue.Labels {
			if label == oldLabel {
				hadLabel = true
				break
			}
		}

		if !hadLabel {
			mlog.Info("issue added label", mlog.Int("issue", issue.Number), mlog.String("label", label))
			if err = s.handleIssueLabeled(ctx, issue, label); err != nil {
				return fmt.Errorf("could not handle issue label added: %w", err)
			}
			hasChanges = true
		}
	}

	if hasChanges {
		mlog.Info("issue has changes", mlog.Int("issue", issue.Number))
		_, err = s.Store.Issue().Save(issue)
		return err
	}

	return nil
}

func (s *Server) handleIssueLabeled(ctx context.Context, issue *model.Issue, addedLabel string) error {
	// Must be sure the comment is created before we let another request test MM-27284
	s.commentLock.Lock()
	defer s.commentLock.Unlock()

	comments, err := s.getComments(ctx, issue.RepoOwner, issue.RepoName, issue.Number)
	if err != nil {
		return fmt.Errorf("could not get issue from GitHub: %w", err)
	}

	for _, label := range s.Config.IssueLabels {
		finalMessage := strings.ReplaceAll(label.Message, "USERNAME", issue.Username)
		if label.Label == addedLabel && !messageByUserContains(comments, s.Config.Username, finalMessage) {
			mlog.Info("Posted message for label on PR", mlog.String("label", label.Label), mlog.Int("issue", issue.Number))
			if err = s.sendGitHubComment(ctx, issue.RepoOwner, issue.RepoName, issue.Number, finalMessage); err != nil {
				mlog.Warn("Error while commenting", mlog.Err(err))
			}
		}
	}
	return nil
}

func issueEventFromJSON(data io.Reader) (*issueEvent, error) {
	decoder := json.NewDecoder(data)
	var event issueEvent
	if err := decoder.Decode(&event); err != nil {
		return nil, err
	}

	if event.Issue == nil {
		return nil, errors.New("github issue is missing from body")
	}
	if event.Repo == nil {
		return nil, errors.New("github repo is missing from body")
	}

	return &event, nil
}
