// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"strings"

	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/mlog"
)

func handleIssueEvent(event *PullRequestEvent) {
	parts := strings.Split(event.RepositoryUrl, "/")

	mlog.Info("handle issue event", mlog.String("repoUrl", event.RepositoryUrl), mlog.String("Action", event.Action),
		mlog.Int("PRNumber", event.PRNumber), mlog.String("PullRequest URL", *event.PullRequest.URL))
	issue, err := GetIssueFromGithub(parts[len(parts)-2], parts[len(parts)-1], event.Issue)
	if err != nil {
		mlog.Error("issue_error", mlog.Err(err))
		return
	}

	checkIssueForChanges(issue)
}

func checkIssueForChanges(issue *model.Issue) {
	var oldIssue *model.Issue
	if result := <-Srv.Store.Issue().Get(issue.RepoOwner, issue.RepoName, issue.Number); result.Err != nil {
		mlog.Error(result.Err.Error())
		return
	} else if result.Data == nil {
		if resultSave := <-Srv.Store.Issue().Save(issue); resultSave.Err != nil {
			mlog.Error(resultSave.Err.Error())
		}
		return
	} else {
		oldIssue = result.Data.(*model.Issue)
	}

	hasChanges := false

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
			handleIssueLabeled(issue, label)
			hasChanges = true
		}
	}

	if hasChanges {
		mlog.Info("issue has changes", mlog.Int("issue", issue.Number))

		if result := <-Srv.Store.Issue().Save(issue); result.Err != nil {
			mlog.Error(result.Err.Error())
			return
		}
	}
}

func handleIssueLabeled(issue *model.Issue, addedLabel string) {
	client := NewGithubClient()

	// Must be sure the comment is created before we let anouther request test
	commentLock.Lock()
	defer commentLock.Unlock()

	comments, _, err := client.Issues.ListComments(issue.RepoOwner, issue.RepoName, issue.Number, nil)
	if err != nil {
		mlog.Error("issue_error", mlog.Err(err))
		return
	}

	for _, label := range Config.IssueLabels {
		finalMessage := strings.Replace(label.Message, "USERNAME", issue.Username, -1)
		if label.Label == addedLabel && !messageByUserContains(comments, Config.Username, finalMessage) {
			mlog.Info("Posted message for label on PR", mlog.String("label", label.Label), mlog.Int("issue", issue.Number))
			commentOnIssue(issue.RepoOwner, issue.RepoName, issue.Number, finalMessage)
		}
	}
}
