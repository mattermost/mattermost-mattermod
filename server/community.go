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

	isOrgMember := s.IsOrgMember(pr.Username)
	// Don't apply label if the contributors is a core committer
	if isOrgMember {
		return
	}

	_, _, err := s.GithubClient.Issues.AddLabelsToIssue(context.Background(), pr.RepoOwner, pr.RepoName, pr.Number, []string{"Hacktoberfest"})
	if err != nil {
		mlog.Error("Error applying Hacktoberfest label", mlog.Err(err), mlog.Int("PR", pr.Number), mlog.String("Repo", pr.RepoName))
		return
	}
}
