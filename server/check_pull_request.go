// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/google/go-github/v28/github"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v5/mlog"
)

func (s *Server) handleChecks(eventIssueComment IssueComment) {
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
	s.checkForIntegrationCriticalFiles(pr)
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
	tempCLA := strings.Split(string(body), "\n")
	for _, item := range tempCLA {
		itemCLA := strings.TrimSpace(item)
		if strings.Compare(itemCLA, username) == 0 || strings.Compare(itemCLA, lowerUsername) == 0 || strings.Compare(strings.ToLower(itemCLA), lowerUsername) == 0 {
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
			return
		}
	}

	_, existComment := s.checkCLAComment(comments)
	if !existComment {
		s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number, strings.Replace(s.Config.NeedsToSignCLAMessage, "USERNAME", "@"+username, 1))
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

func (s *Server) checkForIntegrationCriticalFiles(pr *model.PullRequest) {
	integrationConfigs := s.Config.Integrations
	relevantIntegrationConfigs := getRelevantIntegrationsForPR(pr, integrationConfigs)
	if len(relevantIntegrationConfigs) > 0 {
		prFilenames, err := s.getFilenamesInPullRequest(pr)
		if err != nil {
			mlog.Error("Error listing the files from a PR", mlog.String("repo", pr.RepoName), mlog.Int("pr", pr.Number), mlog.String("Fullname", pr.FullName), mlog.Err(err))
			return
		}

		for _, config := range relevantIntegrationConfigs {
			filenames := getMatchingFilenames(prFilenames, config.Files)

			if len(filenames) > 0 {
				affectedFiles := strings.Join(filenames, ", ")
				mlog.Info("Files could affect server client integration", mlog.String("filenames", affectedFiles), mlog.String("repo", pr.RepoName), mlog.Int("pr", pr.Number), mlog.String("Fullname", pr.FullName))
				msg := fmt.Sprintf(config.Message, affectedFiles, config.IntegrationLink)
				s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number, msg)
				return
			}
		}
	}
}

func getRelevantIntegrationsForPR(pr *model.PullRequest, integrations []*Integration) []*Integration {
	var relevantIntegrations []*Integration
	for _, integration := range integrations {
		if pr.RepoName == integration.RepositoryName && pr.RepoOwner == integration.RepositoryOwner {
			relevantIntegrations = append(relevantIntegrations, integration)
		}
	}

	if len(relevantIntegrations) > 0 {
		return relevantIntegrations
	}
	return nil
}

func getMatchingFilenames(a []string, b []string) []string {
	var matches []string
	for _, aName := range a {
		for _, bName := range b {
			if aName == bName {
				matches = append(matches, bName)
			}
		}
	}

	if len(matches) > 0 {
		return matches
	}
	return nil
}
