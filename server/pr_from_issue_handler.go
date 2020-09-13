// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"net/http"
	"time"

	"github.com/mattermost/mattermost-server/v5/mlog"
)

func (s *Server) prFromIssueHandler(event *issueEvent, w http.ResponseWriter) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout*time.Second)
	defer cancel()

	prGitHub, _, err := s.GithubClient.PullRequests.Get(ctx,
		event.Repo.GetOwner().GetLogin(),
		event.Repo.GetName(),
		event.Issue.GetNumber())
	if err != nil {
		mlog.Error("Error in getting PR from GitHub", mlog.Err(err),
			mlog.String("owner", event.Repo.GetOwner().GetLogin()),
			mlog.String("repoName", event.Repo.GetName()),
			mlog.Int("prNumber", event.Issue.GetNumber()),
		)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// We update the milestone that we have from the issue event and merge it with the PR.
	// This is necessary to work around caching issues with GitHub.
	prGitHub.Milestone = event.Issue.GetMilestone()

	_, err = s.GetPullRequestFromGithub(ctx, prGitHub)
	if err != nil {
		mlog.Error("Error in saving the PR", mlog.Err(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
