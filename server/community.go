// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"time"

	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v5/mlog"
)

func (s *Server) addHacktoberfestLabel(pr *model.PullRequest) {
	if pr.State == "closed" {
		return
	}

	// Ignore PRs not created in october
	if pr.CreatedAt.Month() != time.October {
		return
	}

	isContributorOrgMember, err := s.isOrgMember(s.Config.Org, pr.Username)
	if err != nil {
		mlog.Error("Error getting org membership", mlog.Err(err), mlog.Int("PR", pr.Number), mlog.String("Repo", pr.RepoName))
		return
	}
	// Don't apply label if the contributors is a core committer
	if isContributorOrgMember {
		return
	}

	client := NewGithubClient(s.Config.GithubAccessToken)
	_, _, err = client.Issues.AddLabelsToIssue(context.Background(), pr.RepoOwner, pr.RepoName, pr.Number, []string{"Hacktoberfest"})
	if err != nil {
		mlog.Error("Error applying Hacktoberfest label", mlog.Err(err), mlog.Int("PR", pr.Number), mlog.String("Repo", pr.RepoName))
		return
	}
}
