// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/google/go-github/github"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/mlog"
)

func handleCheckCLA(eventIssueComment IssueComment) {
	client := NewGithubClient()
	prGitHub, _, err := client.PullRequests.Get(context.Background(), *eventIssueComment.Repository.Owner.Login, *eventIssueComment.Repository.Name, *eventIssueComment.Issue.Number)
	pr, err := GetPullRequestFromGithub(prGitHub)
	if err != nil {
		mlog.Error("pr_error", mlog.Err(err))
		return
	}

	if *prGitHub.State == "closed" {
		return
	}

	checkCLA(pr)
}

func checkCLA(pr *model.PullRequest) {
	if pr.State == "closed" {
		return
	}

	username := pr.Username
	mlog.Info("Will check the CLA for user", mlog.String("user", username),
		mlog.String("repo", pr.RepoOwner), mlog.String("reponame", pr.RepoName),
		mlog.Int("pr n", pr.Number))

	resp, err := http.Get(Config.SignedCLAURL)
	if err != nil {
		mlog.Error("Unable to get CLA list", mlog.Err(err))
		return
	}

	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		mlog.Error("Unable to read response body", mlog.Err(err))
		return
	}

	client := NewGithubClient()
	claStatus := &github.RepoStatus{
		TargetURL: github.String(Config.SignedCLAURL),
		Context:   github.String("cla/mattermost"),
	}

	// Get Github comments
	comments, _, err := client.Issues.ListComments(context.Background(), pr.RepoOwner, pr.RepoName, pr.Number, nil)
	if err != nil {
		mlog.Error("pr_error", mlog.Err(err))
		return
	}

	lowerUsername := strings.ToLower(username)
	if !strings.Contains(string(body), username) && !strings.Contains(string(body), lowerUsername) {
		_, existComment := checkCLAComment(comments)
		if !existComment {
			commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, strings.Replace(Config.NeedsToSignCLAMessage, "USERNAME", "@"+username, 1))
		}
		claStatus.State = github.String("error")
		userMsg := fmt.Sprintf("%s needs to sign the CLA", username)
		claStatus.Description = github.String(userMsg)
		mlog.Info("will post error on CLA", mlog.String("user", username))
		_, _, errStatus := client.Repositories.CreateStatus(context.Background(), pr.RepoOwner, pr.RepoName, pr.Sha, claStatus)
		if errStatus != nil {
			mlog.Error("Unable to create the github status for for PR", mlog.Int("pr", pr.Number), mlog.Err(errStatus))
			return
		}
		return
	}

	mlog.Info("will post success on CLA", mlog.String("user", username))
	claStatus.State = github.String("success")
	userMsg := fmt.Sprintf("%s authorized", username)
	claStatus.Description = github.String(userMsg)
	_, _, errStatus := client.Repositories.CreateStatus(context.Background(), pr.RepoOwner, pr.RepoName, pr.Sha, claStatus)
	if errStatus != nil {
		mlog.Error("Unable to create the github status for for PR", mlog.Int("pr", pr.Number), mlog.Err(errStatus))
		return
	}
	mlog.Info("will clean some comments regarding the CLA")
	commentToRemove, existComment := checkCLAComment(comments)
	if existComment {
		mlog.Info("Removing old comment with ID", mlog.Int64("ID", commentToRemove))
		_, err := client.Issues.DeleteComment(context.Background(), pr.RepoOwner, pr.RepoName, commentToRemove)
		if err != nil {
			mlog.Error("Unable to remove old Mattermod comment", mlog.Err(err))
		}
	}
}

func checkCLAComment(comments []*github.IssueComment) (int64, bool) {
	for _, comment := range comments {
		if *comment.User.Login == Config.Username {
			if strings.Contains(*comment.Body, "Please help complete the Mattermost") {
				return *comment.ID, true
			}
		}
	}
	return 0, false
}
