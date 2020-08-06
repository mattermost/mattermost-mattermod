// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v5/mlog"
	"github.com/pkg/errors"

	"github.com/mattermost/go-circleci"
)

func (s *Server) triggerCircleCiIfNeeded(ctx context.Context, pr *model.PullRequest) {
	mlog.Info("Checking if need trigger circleci", mlog.String("repo", pr.RepoName), mlog.Int("pr", pr.Number), mlog.String("fullname", pr.FullName))
	repoInfo := strings.Split(pr.FullName, "/")
	if repoInfo[0] == s.Config.Org {
		// It is from upstream mattermost repo don't need to trigger the circleci because org members
		// have permissions
		mlog.Info("Don't need to trigger circleci", mlog.String("repo", pr.RepoName), mlog.Int("pr", pr.Number), mlog.String("fullname", pr.FullName))
		return
	}

	// Checking if the repo have circleci setup
	builds, err := s.CircleCiClient.ListRecentBuildsForProjectWithContext(ctx, circleci.VcsTypeGithub, pr.RepoOwner, pr.RepoName, "master", "", 5, 0)
	if err != nil {
		mlog.Error("listing the circleci project", mlog.String("repo", pr.RepoName), mlog.Int("pr", pr.Number), mlog.String("Fullname", pr.FullName), mlog.Err(err))
		return
	}
	// If builds are 0 means no build ran for master and most probably this is not setup, so skipping.
	if len(builds) == 0 {
		mlog.Debug("looks like there is not circleci setup or master never ran. Skipping")
		return
	}

	// List the files that was modified or added in the PullRequest
	prFiles, _, err := s.GithubClient.PullRequests.ListFiles(ctx, pr.RepoOwner, pr.RepoName, pr.Number, nil)
	if err != nil {
		mlog.Error("Error listing the files from a PR", mlog.String("repo", pr.RepoName), mlog.Int("pr", pr.Number), mlog.String("Fullname", pr.FullName), mlog.Err(err))
		return
	}

	for _, prFile := range prFiles {
		for _, blockListPath := range s.Config.BlacklistPaths {
			if prFile.GetFilename() == blockListPath {
				mlog.Error("File is on the blocklist and will not retrigger circleci to give the contexts", mlog.String("repo", pr.RepoName), mlog.Int("pr", pr.Number), mlog.String("Fullname", pr.FullName))
				msg := fmt.Sprintf("The file `%s` is in the blocklist and should not be modified from external contributors, please if you are part of the Mattermost Org submit this PR in the upstream.\n /cc @mattermost/core-security @mattermost/core-build-engineers", prFile.GetFilename())
				s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, msg)
				return
			}
		}
	}

	opts := map[string]interface{}{
		"revision": pr.Sha,
		"branch":   fmt.Sprintf("pull/%d", pr.Number),
	}

	err = s.CircleCiClient.BuildByProjectWithContext(ctx, circleci.VcsTypeGithub, pr.RepoOwner, pr.RepoName, opts)
	if err != nil {
		mlog.Error("Error triggering circleci", mlog.String("repo", pr.RepoName), mlog.Int("pr", pr.Number), mlog.String("Fullname", pr.FullName), mlog.Err(err))
		return
	}
	mlog.Info("Triggered circleci", mlog.String("repo", pr.RepoName), mlog.Int("pr", pr.Number), mlog.String("fullname", pr.FullName))
}

func (s *Server) requestEETriggering(ctx context.Context, pr *model.PullRequest, info *EETriggerInfo) error {
	r, err := s.triggerEnterprisePipeline(ctx, pr, info)
	if err != nil {
		return err
	}

	workflowID, err := s.waitForWorkflowID(ctx, r.ID, s.Config.EnterpriseWorkflowName)
	if err != nil {
		return err
	}

	buildLink := "https://app.circleci.com/pipelines/github/" + s.Config.Org + "/" + s.Config.EnterpriseReponame + "/" + strconv.Itoa(r.Number) + "/workflows/" + workflowID
	mlog.Debug("EE tests wf found", mlog.Int("pr", pr.Number), mlog.String("sha", pr.Sha), mlog.String("link", buildLink))

	err = s.waitForStatus(ctx, pr, s.Config.EnterpriseGithubStatusContext, stateSuccess)
	if err != nil {
		s.createEnterpriseTestsErrorStatus(ctx, pr, err)
		return err
	}

	s.updateBuildStatus(ctx, pr, s.Config.EnterpriseGithubStatusEETests, buildLink)
	return nil
}

func (s *Server) triggerEnterprisePipeline(ctx context.Context, pr *model.PullRequest, info *EETriggerInfo) (*circleci.Pipeline, error) {

	params := map[string]interface{}{
		"tbs_sha":           pr.Sha,
		"tbs_pr":            strconv.Itoa(pr.Number),
		"tbs_server_owner":  info.ServerOwner,
		"tbs_server_branch": info.ServerBranch,
		"tbs_webapp_owner":  info.WebappOwner,
		"tbs_webapp_branch": info.WebappBranch,
	}
	pip, err := s.CircleCiClientV2.TriggerPipelineWithContext(ctx, circleci.VcsTypeGithub, s.Config.Org, s.Config.EnterpriseReponame, info.EEBranch, "", params)
	if err != nil {
		return nil, err
	}

	mlog.Debug("EE triggered",
		mlog.Int("pr", pr.Number),
		mlog.String("sha", pr.Sha),
		mlog.String("EEBranch", info.EEBranch),
		mlog.String("ServerOwner", info.ServerOwner),
		mlog.String("ServerBranch", info.ServerBranch),
		mlog.String("WebappOwner", info.WebappOwner),
		mlog.String("WebappBranch", info.WebappBranch),
	)

	return pip, nil
}

func (s *Server) waitForWorkflowID(ctx context.Context, id string, workflowName string) (string, error) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return "", errors.New("timed out trying to fetch workflow")
		case <-ticker.C:
			wfList, err := s.CircleCiClientV2.GetPipelineWorkflowWithContext(ctx, id, "")
			if err != nil {
				return "", err
			}

			workflowID := ""
			for _, wf := range wfList.Items {
				if wf.Name == workflowName {
					workflowID = wf.ID
					break
				}
			}

			if workflowID == "" {
				return "", errors.Errorf("workflow for pip %s not found", id)
			}

			return workflowID, nil
		}
	}
}

func (s *Server) waitForJobs(ctx context.Context, pr *model.PullRequest, org string, branch string, expectedJobNames []string) ([]*circleci.Build, error) {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, errors.New("timed out waiting for build")
		case <-ticker.C:
			mlog.Debug("Waiting for jobs", mlog.Int("pr", pr.Number), mlog.Int("expected", len(expectedJobNames)))
			var builds []*circleci.Build
			var err error
			builds, err = s.CircleCiClient.ListRecentBuildsForProjectWithContext(ctx, circleci.VcsTypeGithub, org, pr.RepoName, branch, "running", len(expectedJobNames), 0)
			if err != nil {
				return nil, err
			}

			if len(builds) == 0 {
				builds, err = s.CircleCiClient.ListRecentBuildsForProjectWithContext(ctx, circleci.VcsTypeGithub, org, pr.RepoName, branch, "", len(expectedJobNames), 0)
				if err != nil {
					return nil, err
				}
			}

			if !areAllExpectedJobs(builds, expectedJobNames) {
				continue
			}

			mlog.Debug("Started building", mlog.Int("pr", pr.Number))
			return builds, nil
		}
	}
}

func (s *Server) waitForArtifacts(ctx context.Context, pr *model.PullRequest, org string, buildNumber int, expectedArtifacts int) ([]*circleci.Artifact, error) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, errors.New("timed out waiting for links to artifacts")
		case <-ticker.C:
			mlog.Debug("Trying to fetch artifacts", mlog.Int("build", buildNumber))
			artifacts, err := s.CircleCiClient.ListBuildArtifactsWithContext(ctx, circleci.VcsTypeGithub, org, pr.RepoName, buildNumber)
			if err != nil {
				return nil, err
			}

			if len(artifacts) < expectedArtifacts {
				continue
			}

			return artifacts, nil
		}
	}
}

func areAllExpectedJobs(builds []*circleci.Build, jobNames []string) bool {
	c := 0
	for _, build := range builds {
		for _, jobName := range jobNames {
			if build.Workflows.JobName == jobName {
				c++
			}
		}
	}

	return len(jobNames) == c
}
