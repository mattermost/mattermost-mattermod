// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"database/sql"
	"net/http"

	"github.com/mattermost/mattermost-server/v5/mlog"
)

func (s *Server) prFromIssueHandler(event *issueEvent, w http.ResponseWriter) {
	oldPR, err := s.Store.PullRequest().Get(event.Repo.GetOwner().GetLogin(),
		event.Repo.GetName(),
		event.Issue.GetNumber())
	if err != nil {
		mlog.Error("Error in getting PR from DB", mlog.Err(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// We update the milestone that we have from the issue event and merge it with the PR.
	// This is necessary to work around caching issues with GitHub.
	oldPR.MilestoneNumber = sql.NullInt64{Int64: int64(event.Issue.GetMilestone().GetNumber()), Valid: true}
	oldPR.MilestoneTitle = sql.NullString{String: event.Issue.GetMilestone().GetTitle(), Valid: true}

	_, err = s.Store.PullRequest().Save(oldPR)
	if err != nil {
		mlog.Error("Error in saving PR to DB", mlog.Err(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
