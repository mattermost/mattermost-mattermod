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

func (s *Server) handleCheckCLA(eventIssueComment IssueComment) {
	client := NewGithubClient(s.Config.GithubAccessToken)
	prGitHub, _, err := client.PullRequests.Get(context.Background(), *eventIssueComment.Repository.Owner.Login, *eventIssueComment.Repository.Name, *eventIssueComment.Issue.Number)
	pr, err := s.GetPullRequestFromGithub(prGitHub)
	if err != nil {
		mlog.Error("pr_error", mlog.Err(err))
		return
	}

	if *prGitHub.State == "closed" {
		return
	}

	s.checkCLA(pr)
}

func (s *Server) checkCLA(pr *model.PullRequest) {
	if pr.State == "closed" {
		return
	}

	username := pr.Username
	mlog.Info("Will check the CLA for user", mlog.String("user", username),
		mlog.String("repo", pr.RepoOwner), mlog.String("reponame", pr.RepoName),
		mlog.Int("pr n", pr.Number))

	resp, err := http.Get(s.Config.SignedCLAURL)
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

	client := NewGithubClient(s.Config.GithubAccessToken)
	claStatus := &github.RepoStatus{
		TargetURL: github.String(s.Config.SignedCLAURL),
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
		_, existComment := s.checkCLAComment(comments)
		if !existComment {
			s.commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, strings.Replace(s.Config.NeedsToSignCLAMessage, "USERNAME", "@"+username, 1))
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
	commentToRemove, existComment := s.checkCLAComment(comments)
	if existComment {
		mlog.Info("Removing old comment with ID", mlog.Int64("ID", commentToRemove))
		_, err := client.Issues.DeleteComment(context.Background(), pr.RepoOwner, pr.RepoName, commentToRemove)
		if err != nil {
			mlog.Error("Unable to remove old Mattermod comment", mlog.Err(err))
		}
	}
}

func (s *Server) checkCLAComment(comments []*github.IssueComment) (int64, bool) {
	for _, comment := range comments {
		if *comment.User.Login == s.Config.Username {
			if strings.Contains(*comment.Body, "Please help complete the Mattermost") {
				return *comment.ID, true
			}
		}
	}
	return 0, false
}
