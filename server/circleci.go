// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"fmt"
	"strings"

	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/mlog"

	"github.com/cpanato/go-circleci"
)

func (s *Server) triggerCircleCiIfNeeded(pr *model.PullRequest) {
	client := &circleci.Client{Token: s.Config.CircleCIToken}

	if strings.Contains(pr.FullName, "mattermost/") {
		// It is from upstream mattermost repo dont need to trigger the circleci because org members
		// have permissions
		return
	}

	// Checking if the repo have circleci setup
	builds, err := client.ListRecentBuildsForProject(pr.RepoOwner, pr.RepoName, "master", "", 5, 0)
	if err != nil {
		mlog.Error("listing the circleci project", mlog.String("repo", pr.RepoName), mlog.Int("pr", pr.Number), mlog.String("Fullname", pr.FullName))
		return
	}
	// if builds are 0 means no build ran for master and most problaby this is not setup, so skipping.
	if len(builds) == 0 {
		mlog.Debug("looks like there is not circleci setup or master never ran. Skipping")
		return
	}

	opts := map[string]interface{}{
		"revision": pr.Sha,
		"branch":   fmt.Sprintf("pull/%d", pr.Number),
	}

	err = client.BuildByProject("github", pr.RepoOwner, pr.RepoName, opts)
	if err != nil {
		mlog.Error("Error triggering circleci", mlog.String("repo", pr.RepoName), mlog.Int("pr", pr.Number), mlog.String("Fullname", pr.FullName))
	}
}
