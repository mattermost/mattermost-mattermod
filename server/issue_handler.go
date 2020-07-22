// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v5/mlog"
)

func (s *Server) issueEventHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout*time.Second)
	defer cancel()

	event, err := PullRequestEventFromJSON(r.Body)
	if err != nil {
		mlog.Error("could not parse issue event", mlog.Err(err))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	parts := strings.Split(event.Issue.GetHTMLURL(), "/")
	if len(parts) < 4 {
		mlog.Error("incorrect pattern for issue url")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	mlog.Info("handle issue event", mlog.String("repoUrl", *event.Issue.HTMLURL), mlog.String("Action", event.Action), mlog.Int("PRNumber", event.PRNumber))
	issue, err := s.GetIssueFromGithub(ctx, parts[len(parts)-4], parts[len(parts)-3], event.Issue)
	if err != nil {
		mlog.Error("could not get the issue from GitHub", mlog.Err(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := s.checkIssueForChanges(ctx, issue); err != nil {
		mlog.Error("could not check issue for changes", mlog.Err(err))
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (s *Server) checkIssueForChanges(ctx context.Context, issue *model.Issue) error {
	oldIssue, err := s.Store.Issue().Get(issue.RepoOwner, issue.RepoName, issue.Number)
	if err != nil {
		return err
	}

	if oldIssue == nil {
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
	// Must be sure the comment is created before we let anouther request test
	s.commentLock.Lock()
	defer s.commentLock.Unlock()

	comments, _, err := s.GithubClient.Issues.ListComments(ctx, issue.RepoOwner, issue.RepoName, issue.Number, nil)
	if err != nil {
		return fmt.Errorf("could not get issue from GitHub: %w", err)
	}

	for _, label := range s.Config.IssueLabels {
		finalMessage := strings.Replace(label.Message, "USERNAME", issue.Username, -1)
		if label.Label == addedLabel && !messageByUserContains(comments, s.Config.Username, finalMessage) {
			mlog.Info("Posted message for label on PR", mlog.String("label", label.Label), mlog.Int("issue", issue.Number))
			s.sendGitHubComment(ctx, issue.RepoOwner, issue.RepoName, issue.Number, finalMessage)
		}
	}
	return nil
}
