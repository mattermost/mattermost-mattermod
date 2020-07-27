// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"errors"
	"fmt"

	"github.com/mattermost/mattermost-server/v5/mlog"
)

func (s *Server) handleIssueEvent(ctx context.Context, event *PullRequestEvent) error {
	if event == nil || event.Issue == nil {
		return errors.New("could not handle issue event: either event or issue is nil")
	}

	mlog.Info("handle issue event", mlog.String("repoUrl", *event.Issue.HTMLURL), mlog.String("Action", event.Action), mlog.Int("PRNumber", event.PRNumber))
	issue, err := s.GetIssueFromGithub(ctx, event.Issue)
	if err != nil {
		return fmt.Errorf("error getting the issue from GitHub: %w", err)
	}

	return s.checkIssueForChanges(ctx, issue)
}
