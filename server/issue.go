// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"strings"

	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v5/mlog"
)

func (s *Server) handleIssueEvent(event *PullRequestEvent) {
	if event == nil || event.Issue == nil {
		return
	}
	parts := strings.Split(*event.Issue.HTMLURL, "/")

	mlog.Info("handle issue event", mlog.String("repoUrl", *event.Issue.HTMLURL), mlog.String("Action", event.Action), mlog.Int("PRNumber", event.PRNumber))
	issue, err := s.GetIssueFromGithub(parts[len(parts)-4], parts[len(parts)-3], event.Issue)
	if err != nil {
		mlog.Error("Error getting the issue from Github", mlog.Err(err))
		return
	}

	s.checkIssueForChanges(issue)
}

func (s *Server) checkIssueForChanges(issue *model.Issue) {
	oldIssue, err := s.Store.Issue().Get(issue.RepoOwner, issue.RepoName, issue.Number)
	if err != nil {
		mlog.Error(err.Error())
		return
	}

	if oldIssue == nil {
		if _, err := s.Store.Issue().Save(issue); err != nil {
			mlog.Error(err.Error())
		}
		return
	}

	hasChanges := false

	if oldIssue.State != issue.State {
		hasChanges = true
	}

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
			s.handleIssueLabeled(issue, label)
			hasChanges = true
		}
	}

	if hasChanges {
		mlog.Info("issue has changes", mlog.Int("issue", issue.Number))

		if _, err := s.Store.Issue().Save(issue); err != nil {
			mlog.Error(err.Error())
			return
		}
	}
}

func (s *Server) handleIssueLabeled(issue *model.Issue, addedLabel string) {
	// Must be sure the comment is created before we let anouther request test
	s.commentLock.Lock()
	defer s.commentLock.Unlock()

	comments, _, err := s.GithubClient.Issues.ListComments(context.Background(), issue.RepoOwner, issue.RepoName, issue.Number, nil)
	if err != nil {
		mlog.Error("issue_error", mlog.Err(err))
		return
	}

	for _, label := range s.Config.IssueLabels {
		finalMessage := strings.Replace(label.Message, "USERNAME", issue.Username, -1)
		if label.Label == addedLabel && !messageByUserContains(comments, s.Config.Username, finalMessage) {
			mlog.Info("Posted message for label on PR", mlog.String("label", label.Label), mlog.Int("issue", issue.Number))
			s.sendGitHubComment(issue.RepoOwner, issue.RepoName, issue.Number, finalMessage)
		}
	}
}
