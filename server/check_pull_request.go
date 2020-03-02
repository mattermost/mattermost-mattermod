// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v5/mlog"
)

func (s *Server) checks(pr *model.PullRequest) {
	go s.checkCLA(pr)
	go s.checkForIntegrations(pr)
}

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

	s.checks(pr)
}
