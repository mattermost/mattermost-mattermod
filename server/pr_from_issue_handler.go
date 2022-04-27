// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"net/http"

	"github.com/mattermost/mattermost-server/v6/shared/mlog"
)

func (s *Server) prFromIssueHandler(event *issueEvent, w http.ResponseWriter) {
	// Happens if the PR is new _and_ has a milestone. So sometimes the milestone info
	// is not up to date.
	if event.Issue.GetMilestone() == nil {
		return
	}

	oldPR, err := s.Store.PullRequest().Get(event.Repo.GetOwner().GetLogin(),
		event.Repo.GetName(),
		event.Issue.GetNumber())
	if err != nil {
		mlog.Error("Error in getting PR from DB", mlog.Err(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// PR does not exist in DB.
	if oldPR == nil {
		return
	}

	// We update the milestone that we have from the issue event and merge it with the PR.
	// This is necessary to work around caching issues with GitHub.
	oldPR.MilestoneNumber = NewInt64(int64(event.Issue.GetMilestone().GetNumber()))
	oldPR.MilestoneTitle = NewString(event.Issue.GetMilestone().GetTitle())

	_, err = s.Store.PullRequest().Save(oldPR)
	if err != nil {
		mlog.Error("Error in saving PR to DB", mlog.Err(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
