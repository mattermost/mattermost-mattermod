// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/mlog"

	"github.com/cpanato/go-circleci"
)

func (s *Server) triggerCircleCiIfNeeded(pr *model.PullRequest) {
	client := &circleci.Client{Token: s.Config.CircleCIToken}
	clientGitHub := NewGithubClient(s.Config.GithubAccessToken)

	mlog.Info("Checking if need trigger circleci", mlog.String("repo", pr.RepoName), mlog.Int("pr", pr.Number), mlog.String("fullname", pr.FullName))
	repoInfo := strings.Split(pr.FullName, "/")
	if repoInfo[0] == "mattermost" {
		// It is from upstream mattermost repo dont need to trigger the circleci because org members
		// have permissions
		mlog.Info("Dont need to trigger circleci", mlog.String("repo", pr.RepoName), mlog.Int("pr", pr.Number), mlog.String("fullname", pr.FullName))
		return
	}

	// Checking if the repo have circleci setup
	builds, err := client.ListRecentBuildsForProject("github", pr.RepoOwner, pr.RepoName, "master", "", 5, 0)
	if err != nil {
		mlog.Error("listing the circleci project", mlog.String("repo", pr.RepoName), mlog.Int("pr", pr.Number), mlog.String("Fullname", pr.FullName), mlog.Err(err))
		return
	}
	// if builds are 0 means no build ran for master and most problaby this is not setup, so skipping.
	if len(builds) == 0 {
		mlog.Debug("looks like there is not circleci setup or master never ran. Skipping")
		return
	}

	prCommits, _, err := clientGitHub.PullRequests.ListCommits(context.Background(), pr.RepoOwner, pr.RepoName, pr.Number, nil)
	if err != nil {
		mlog.Error("Error listing the commits from a PR", mlog.String("repo", pr.RepoName), mlog.Int("pr", pr.Number), mlog.String("Fullname", pr.FullName), mlog.Err(err))
		return
	}

	for _, commit := range prCommits {
		prCommit, _, errCommit := clientGitHub.Repositories.GetCommit(context.Background(), pr.RepoOwner, pr.RepoName, commit.GetSHA())
		if errCommit != nil {
			mlog.Error("Error getting the commits from a PR", mlog.String("repo", pr.RepoName), mlog.Int("pr", pr.Number), mlog.String("Fullname", pr.FullName), mlog.Err(errCommit))
			return
		}
		for _, file := range prCommit.Files {
			for _, blackListPath := range s.Config.BlacklistPaths {
				if file.GetFilename() == blackListPath {
					mlog.Error("File is on the blacklist and will not retrigger circleci to give the contexts", mlog.String("repo", pr.RepoName), mlog.Int("pr", pr.Number), mlog.String("Fullname", pr.FullName))
					msg := fmt.Sprintf("The file `%s` is in the blacklist and should not be modified from external contributors, please if you are part of the Mattermost Org submit this PR in the upstream.\n /cc @mattermost/core-security", file.GetFilename())
					s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number, msg)
					return
				}
			}
		}
	}

	opts := map[string]interface{}{
		"revision": pr.Sha,
		"branch":   fmt.Sprintf("pull/%d", pr.Number),
	}

	err = client.BuildByProject("github", pr.RepoOwner, pr.RepoName, opts)
	if err != nil {
		mlog.Error("Error triggering circleci", mlog.String("repo", pr.RepoName), mlog.Int("pr", pr.Number), mlog.String("Fullname", pr.FullName), mlog.Err(err))
		return
	}
	mlog.Info("Triggered circleci", mlog.String("repo", pr.RepoName), mlog.Int("pr", pr.Number), mlog.String("fullname", pr.FullName))
}
