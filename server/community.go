// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"strings"
	"time"

	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v5/mlog"
)

func (s *Server) addHacktoberfestLabel(ctx context.Context, pr *model.PullRequest) {
	if pr.State == model.StateClosed {
		return
	}

	// Ignore PRs not created in october
	if pr.CreatedAt.Month() != time.October {
		return
	}

	// Don't apply label if the contributors is a core committer
	if s.IsOrgMember(pr.Username) {
		return
	}

	_, _, err := s.GithubClient.Issues.AddLabelsToIssue(ctx, pr.RepoOwner, pr.RepoName, pr.Number, []string{"Hacktoberfest"})
	if err != nil {
		mlog.Error("Error applying Hacktoberfest label", mlog.Err(err), mlog.Int("PR", pr.Number), mlog.String("Repo", pr.RepoName))
		return
	}
}

func (s *Server) postPRWelcomeMessage(ctx context.Context, pr *model.PullRequest, claCommentNeeded bool) error {
	// Only post welcome Message for community member
	if s.IsOrgMember(pr.Username) {
		return nil
	}

	msg := s.Config.PRWelcomeMessage
	if claCommentNeeded {
		msg += "\n\n" + s.Config.NeedsToSignCLAMessage
	}

	msg = strings.ReplaceAll(msg, "USERNAME", "@"+pr.Username)

	err := s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, msg)
	if err != nil {
		return err
	}

	return nil
}
