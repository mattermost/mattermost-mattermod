// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"strings"
	"time"

	"github.com/google/go-github/github"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/mlog"
)

func (s *Server) handleIssueEvent(event *PullRequestEvent) {
	mlog.Info("Handle Issue event", mlog.Any("Issue HTMLURL", *event.Issue))
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
	var oldIssue *model.Issue
	if result := <-s.Store.Issue().Get(issue.RepoOwner, issue.RepoName, issue.Number); result.Err != nil {
		mlog.Error(result.Err.Error())
		return
	} else if result.Data == nil {
		if resultSave := <-s.Store.Issue().Save(issue); resultSave.Err != nil {
			mlog.Error(resultSave.Err.Error())
		}
		return
	} else {
		oldIssue = result.Data.(*model.Issue)
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

		if result := <-s.Store.Issue().Save(issue); result.Err != nil {
			mlog.Error(result.Err.Error())
			return
		}
	}
}

func (s *Server) handleIssueLabeled(issue *model.Issue, addedLabel string) {
	client := NewGithubClient(s.Config.GithubAccessToken)

	// Must be sure the comment is created before we let anouther request test
	s.commentLock.Lock()
	defer s.commentLock.Unlock()

	comments, _, err := client.Issues.ListComments(context.Background(), issue.RepoOwner, issue.RepoName, issue.Number, nil)
	if err != nil {
		mlog.Error("issue_error", mlog.Err(err))
		return
	}

	for _, label := range s.Config.IssueLabels {
		finalMessage := strings.Replace(label.Message, "USERNAME", issue.Username, -1)
		if label.Label == addedLabel && !messageByUserContains(comments, s.Config.Username, finalMessage) {
			mlog.Info("Posted message for label on PR", mlog.String("label", label.Label), mlog.Int("issue", issue.Number))
			s.commentOnIssue(issue.RepoOwner, issue.RepoName, issue.Number, finalMessage)
		}
	}
}

func (s *Server) CleanOutdatedIssues() {
	mlog.Info("Cleaning outdated issues in the mattermod database....")

	var issues []*model.Issue
	if result := <-s.Store.Issue().ListOpen(); result.Err != nil {
		mlog.Error(result.Err.Error())
		return
	} else {
		issues = result.Data.([]*model.Issue)
	}

	mlog.Info("Will process the Issues", mlog.Int("Issues Count", len(issues)))

	client := NewGithubClient(s.Config.GithubAccessToken)
	for _, issue := range issues {
		ghIssue, _, errIssue := client.Issues.Get(context.Background(), issue.RepoOwner, issue.RepoName, issue.Number)
		if errIssue != nil {
			mlog.Error("Error getting Pull Request", mlog.String("RepoOwner", issue.RepoOwner), mlog.String("RepoName", issue.RepoName), mlog.Int("PRNumber", issue.Number), mlog.Err(errIssue))
			if _, ok := errIssue.(*github.RateLimitError); ok {
				mlog.Error("GitHub rate limit reached")
				s.CheckLimitRateAndSleep()
			}
		}

		if *ghIssue.State == "closed" {
			mlog.Info("Issue is closed, updating the status in the database", mlog.String("RepoOwner", issue.RepoOwner), mlog.String("RepoName", issue.RepoName), mlog.Int("IssueNumber", issue.Number))
			issue.State = *ghIssue.State
			if result := <-s.Store.Issue().Save(issue); result.Err != nil {
				mlog.Error(result.Err.Error())
			}
		} else {
			mlog.Info("Nothing do to", mlog.String("RepoOwner", issue.RepoOwner), mlog.String("RepoName", issue.RepoName), mlog.Int("IssueNumber", issue.Number))
		}
		time.Sleep(5 * time.Second)
	}
	mlog.Info("Finished update the outdated issues in the mattermod database....")
}
