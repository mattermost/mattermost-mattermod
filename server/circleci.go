// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"fmt"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v5/mlog"
	"github.com/pkg/errors"
	"strconv"
	"strings"
	"time"

	"github.com/metanerd/go-circleci"
)

func (s *Server) triggerCircleCiIfNeeded(pr *model.PullRequest) {
	client := &circleci.Client{Token: s.Config.CircleCIToken}
	clientGitHub := NewGithubClient(s.Config.GithubAccessToken)

	mlog.Info("Checking if need trigger circleci", mlog.String("repo", pr.RepoName), mlog.Int("pr", pr.Number), mlog.String("fullname", pr.FullName))
	repoInfo := strings.Split(pr.FullName, "/")
	if repoInfo[0] == s.Config.Org {
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

	// List th files that was modified or added in the PullRequest
	prFiles, _, err := clientGitHub.PullRequests.ListFiles(context.Background(), pr.RepoOwner, pr.RepoName, pr.Number, nil)
	if err != nil {
		mlog.Error("Error listing the files from a PR", mlog.String("repo", pr.RepoName), mlog.Int("pr", pr.Number), mlog.String("Fullname", pr.FullName), mlog.Err(err))
		return
	}

	for _, prFile := range prFiles {
		for _, blackListPath := range s.Config.BlacklistPaths {
			if prFile.GetFilename() == blackListPath {
				mlog.Error("File is on the blacklist and will not retrigger circleci to give the contexts", mlog.String("repo", pr.RepoName), mlog.Int("pr", pr.Number), mlog.String("Fullname", pr.FullName))
				msg := fmt.Sprintf("The file `%s` is in the blacklist and should not be modified from external contributors, please if you are part of the Mattermost Org submit this PR in the upstream.\n /cc @mattermost/core-security @mattermost/core-build-engineers", prFile.GetFilename())
				s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number, msg)
				return
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

func (s *Server) waitForBuildLink(ctx context.Context, pr *model.PullRequest, orgUsername string) (string, int, error) {
	ticker := time.NewTicker(10 * time.Second)
	for {
		select {
		case <-ctx.Done():
			ticker.Stop()
			return "", 0, errors.New("timed out waiting for build link")
		case <-ticker.C:
			branch := s.Config.BuildMobileAppBranchPrefix + strconv.Itoa(pr.Number)
			client := &circleci.Client{Token: s.Config.CircleCIToken}

			builds, err := client.ListRecentBuildsForProject(circleci.VcsTypeGithub, orgUsername, pr.RepoName, branch, "pending", 1, 0)
			if err != nil {
				return "", 0, err
			}

			if len(builds) == 0 {
				return "", 0, errors.New("could not retrieve any builds")
			}

			buildUrl := builds[0].BuildURL
			buildNumber := builds[0].BuildNum

			mlog.Debug("Started building", mlog.Int("buildNumber", buildNumber), mlog.Int("pr", pr.Number), mlog.String("org", orgUsername), mlog.String("repo_name", pr.RepoName))
			ticker.Stop()
			return buildUrl, buildNumber, nil
		}
	}
}

func (s *Server) waitForArtifactLinks(ctx context.Context, pr *model.PullRequest, orgUsername string, buildNumber int, expected int) (string, error) {
	ticker := time.NewTicker(1 * time.Minute)
	for {
		select {
		case <-ctx.Done():
			ticker.Stop()
			return "", errors.New("timed out waiting for links to artifacts")
		case <-ticker.C:
			client := &circleci.Client{Token: s.Config.CircleCIToken}
			mlog.Debug("Trying to fetch artifacts", mlog.String("org", orgUsername), mlog.String("repoName", pr.RepoName), mlog.Int("build", buildNumber))
			artifacts, err := client.ListBuildArtifacts(circleci.VcsTypeGithub, orgUsername, pr.RepoName, buildNumber)
			if err != nil {
				return "", err
			}

			if len(artifacts) < expected {
				continue
			}

			artifactLinks := ""
			for _, artifact := range artifacts {
				artifactLinks += artifact.URL + "  \n"
			}
			mlog.Debug("Successfully retrieved artifacts links", mlog.Int("buildNumber", buildNumber), mlog.Int("pr", pr.Number), mlog.String("org", orgUsername), mlog.String("repo_name", pr.RepoName), mlog.String("artifactLinks", artifactLinks))
			ticker.Stop()
			return artifactLinks, nil
		}
	}
}
