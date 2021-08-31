// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/mattermost/go-circleci"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v5/mlog"
)

func (s *Server) buildApp(pr *model.PullRequest) {
	// This needs its own context because is executing a heavy job
	ctx, cancel := context.WithTimeout(context.Background(), defaultBuildAppTimeout*time.Second)
	defer cancel()

	prRepoOwner, prRepoName, prNumber := pr.RepoOwner, pr.RepoName, pr.Number
	// will generate the string refs/heads/build-pr-1222-8bfcb54
	ref := fmt.Sprintf("refs/heads/%s%d-%s", s.Config.BuildAppBranchPrefix, prNumber, pr.Sha[0:7])
	isReadyToBeBuilt, err := s.areChecksSuccessfulForPr(ctx, pr, s.Config.Org)
	if err != nil {
		msg := fmt.Sprintf("Failed to retrieve the status of the PR. Error:  \n```%s```", err.Error())
		if cErr := s.sendGitHubComment(ctx, prRepoOwner, prRepoName, prNumber, msg); cErr != nil {
			mlog.Warn("Error while commenting", mlog.Err(cErr))
		}
		return
	}

	if isReadyToBeBuilt {
		exists, err := s.checkIfRefExists(ctx, pr, s.Config.Org, ref)
		if err != nil {
			msg := fmt.Sprintf("Failed to check ref. @mattermost/core-build-engineers have been notified. Error \n```%s```", err.Error())
			if cErr := s.sendGitHubComment(ctx, prRepoOwner, prRepoName, prNumber, msg); cErr != nil {
				mlog.Warn("Error while commenting", mlog.Err(cErr))
			}
			return
		}

		if exists {
			err = s.deleteRef(ctx, s.Config.Org, prRepoName, ref)
			if err != nil {
				msg := fmt.Sprintf("Failed to delete already existing build branch. @mattermost/core-build-engineers have been notified. Error \n```%s```", err.Error())
				if cErr := s.sendGitHubComment(ctx, prRepoOwner, prRepoName, prNumber, msg); cErr != nil {
					mlog.Warn("Error while commenting", mlog.Err(cErr))
				}
				return
			}
		}

		s.createRef(ctx, pr, ref)
		if cErr := s.sendGitHubComment(ctx, prRepoOwner, prRepoName, prNumber, s.Config.BuildAppInitMessage); cErr != nil {
			mlog.Warn("Error while commenting", mlog.Err(cErr))
		}
		s.build(ctx, pr, s.Config.Org)

		err = s.deleteRefWhereCombinedStateEqualsSuccess(ctx, s.Config.Org, prRepoName, ref)
		if err != nil {
			msg := fmt.Sprintf("Failed to delete ref. @mattermost/core-build-engineers have been notified. Error \n```%s```", err.Error())
			if cErr := s.sendGitHubComment(ctx, prRepoOwner, prRepoName, prNumber, msg); cErr != nil {
				mlog.Warn("Error while commenting", mlog.Err(cErr))
			}
		}
	} else {
		msg := "Not triggering the mobile app build workflow, because PR checks are failing. "
		if cErr := s.sendGitHubComment(ctx, prRepoOwner, prRepoName, prNumber, msg); cErr != nil {
			mlog.Warn("Error while commenting", mlog.Err(cErr))
		}
	}
}

func (s *Server) build(ctx context.Context, pr *model.PullRequest, org string) {
	prRepoOwner, prRepoName, prNumber := pr.RepoOwner, pr.RepoName, pr.Number
	branch := s.Config.BuildAppBranchPrefix + strconv.Itoa(pr.Number)

	expectedJobNames := getExpectedJobNames(s.Config.BuildAppJobs, prRepoName)

	builds, err := s.waitForJobs(ctx, pr, org, branch, expectedJobNames)
	if err != nil {
		mlog.Err(err)
		msg := fmt.Sprintf("Failed retrieving build links. @mattermost/core-build-engineers have been notified. Error:  \n```%s```", err.Error())
		if cErr := s.sendGitHubComment(ctx, prRepoOwner, prRepoName, prNumber, msg); cErr != nil {
			mlog.Warn("Error while commenting", mlog.Err(cErr))
		}
		return
	}

	linksBuilds := ""
	for _, build := range builds {
		linksBuilds += build.BuildURL + "  \n"
	}
	comment := fmt.Sprintf("Successfully building:  \n%s", linksBuilds)
	if cErr := s.sendGitHubComment(ctx, prRepoOwner, prRepoName, prNumber, comment); cErr != nil {
		mlog.Warn("Error while commenting", mlog.Err(cErr))
	}

	var artifacts []*circleci.Artifact
	for _, build := range builds {
		expectedArtifacts := getExpectedArtifacts(s.Config.BuildAppJobs, build.Workflows.JobName, prRepoName)
		buildArtifacts, err := s.waitForArtifacts(ctx, pr, s.Config.Org, build.BuildNum, expectedArtifacts)
		if err != nil {
			msg := fmt.Sprintf("Failed retrieving artifact links. @mattermost/core-build-engineers have been notified. Error:  \n```%s```", err.Error())
			if cErr := s.sendGitHubComment(ctx, prRepoOwner, prRepoName, prNumber, msg); cErr != nil {
				mlog.Warn("Error while commenting", mlog.Err(cErr))
			}
		}
		artifacts = append(artifacts, buildArtifacts...)
	}

	if len(artifacts) < len(expectedJobNames) {
		msg := "Failed retrieving artifact links. @mattermost/core-build-engineers have been notified. "
		if cErr := s.sendGitHubComment(ctx, prRepoOwner, prRepoName, prNumber, msg); cErr != nil {
			mlog.Warn("Error while commenting", mlog.Err(cErr))
		}
	}

	linksArtifacts := ""
	for _, artifact := range artifacts {
		linksArtifacts += artifact.URL + "  \n"
	}
	comment = fmt.Sprintf("Artifact links:  \n%s", linksArtifacts)
	if cErr := s.sendGitHubComment(ctx, prRepoOwner, prRepoName, prNumber, comment); cErr != nil {
		mlog.Warn("Error while commenting", mlog.Err(cErr))
	}
}

func getExpectedArtifacts(jobs []*BuildAppJob, buildJobName, prRepoName string) int {
	for _, job := range jobs {
		if buildJobName == job.JobName && prRepoName == job.RepoName {
			return job.ExpectedArtifacts
		}
	}

	return 0
}

func getExpectedJobNames(jobs []*BuildAppJob, prRepoName string) []string {
	var expectedJobNames []string
	for _, job := range jobs {
		if prRepoName == job.RepoName {
			expectedJobNames = append(expectedJobNames, job.JobName)
		}
	}

	return expectedJobNames
}
