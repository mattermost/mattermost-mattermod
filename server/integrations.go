// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v5/mlog"
)

func (s *Server) handleCheckIntegrations(eventIssueComment IssueComment) {
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

	s.checkForIntegrations(pr)
}

func (s *Server) checkForIntegrations(pr *model.PullRequest) {
	if pr.State == "closed" {
		return
	}

	integrationConfigs := s.Config.Integrations
	relevantIntegrationConfigs := getRelevantIntegrationsForPR(pr, integrationConfigs)
	if len(relevantIntegrationConfigs) > 0 {
		prFilenames, err := s.getFilenamesInPullRequest(pr)
		if err != nil {
			mlog.Error("Error listing the files from a pr",
				mlog.String("repo", pr.RepoName),
				mlog.Int("pr", pr.Number), mlog.String("user", pr.Username),
				mlog.Err(err))
			return
		}

		for _, config := range relevantIntegrationConfigs {
			filenames := getMatchingFilenames(prFilenames, config.Files)

			if len(filenames) > 0 {
				affectedFiles := strings.Join(filenames, ", ")
				mlog.Info("Files could affect integration", mlog.String("filenames", affectedFiles), mlog.String("repo", pr.RepoName), mlog.Int("pr", pr.Number), mlog.String("Fullname", pr.FullName))
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
		if pr.RepoName == integration.RepositoryName {
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
