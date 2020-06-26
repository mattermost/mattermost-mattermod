// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"strconv"

	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v5/mlog"
	"github.com/metanerd/go-circleci"
)

func (s *Server) buildMobileApp(pr *model.PullRequest) {
	// This needs its own context because is executing a heavy job
	ctx, cancel := context.WithTimeout(context.Background(), buildMobileTimeout)
	defer cancel()

	prRepoOwner, prRepoName, prNumber := pr.RepoOwner, pr.RepoName, pr.Number
	ref := "refs/heads/" + s.Config.BuildMobileAppBranchPrefix + strconv.Itoa(prNumber)

	isReadyToBeBuilt, err := s.areChecksSuccessfulForPr(ctx, pr, s.Config.Org)
	if err != nil {
		s.sendGitHubComment(ctx, prRepoOwner, prRepoName, prNumber,
			"Failed to retrieve the status of the PR. Error:  \n```"+err.Error()+"```")
		return
	}

	if isReadyToBeBuilt {
		exists, err := s.checkIfRefExists(ctx, pr, s.Config.Org, ref)
		if err != nil {
			s.sendGitHubComment(ctx, prRepoOwner, prRepoName, prNumber,
				"Failed to check ref. @mattermost/core-build-engineers have been notified. Error:  \n```"+err.Error()+"```")
			return
		}

		if exists {
			err = s.deleteRef(ctx, s.Config.Org, prRepoName, ref)
			if err != nil {
				s.sendGitHubComment(ctx, prRepoOwner, prRepoName, prNumber,
					"Failed to delete already existing build branch. @mattermost/core-build-engineers have been notified. Error:  \n```"+err.Error()+"```")
				return
			}
		}

		s.createRef(ctx, pr, ref)
		s.sendGitHubComment(ctx, prRepoOwner, prRepoName, prNumber, s.Config.BuildMobileAppInitMessage)
		s.build(ctx, pr, s.Config.Org)

		err = s.deleteRefWhereCombinedStateEqualsSuccess(ctx, s.Config.Org, prRepoName, ref)
		if err != nil {
			s.sendGitHubComment(ctx, prRepoOwner, prRepoName, prNumber,
				"Failed to delete ref. @mattermost/core-build-engineers have been notified. Error:  \n```"+err.Error()+"```")
		}
	} else {
		s.sendGitHubComment(ctx, prRepoOwner, prRepoName, prNumber,
			"Not triggering the mobile app build workflow, because PR checks are failing. ")
	}
}

func (s *Server) build(ctx context.Context, pr *model.PullRequest, org string) {
	prRepoOwner, prRepoName, prNumber := pr.RepoOwner, pr.RepoName, pr.Number
	branch := s.Config.BuildMobileAppBranchPrefix + strconv.Itoa(pr.Number)

	expectedJobNames := getExpectedJobNames(s.Config.BuildMobileAppJobs)

	builds, err := s.waitForJobs(ctx, pr, org, branch, expectedJobNames)
	if err != nil {
		mlog.Err(err)
		s.sendGitHubComment(ctx, prRepoOwner, prRepoName, prNumber,
			"Failed retrieving build links. @mattermost/core-build-engineers have been notified. Error:  \n```"+err.Error()+"```")
		return
	}

	linksBuilds := ""
	for _, build := range builds {
		linksBuilds += build.BuildURL + "  \n"
	}
	s.sendGitHubComment(ctx, prRepoOwner, prRepoName, prNumber, "Successfully building:  \n"+linksBuilds)

	var artifacts []*circleci.Artifact
	for _, build := range builds {
		expectedArtifacts := getExpectedArtifacts(s.Config.BuildMobileAppJobs, build.Workflows.JobName)
		buildArtifacts, err := s.waitForArtifacts(ctx, pr, s.Config.Org, build.BuildNum, expectedArtifacts)
		if err != nil {
			s.sendGitHubComment(ctx, prRepoOwner, prRepoName, prNumber,
				"Failed retrieving artifact links. @mattermost/core-build-engineers have been notified. Error:  \n```"+err.Error()+"```")
			return
		}
		artifacts = append(artifacts, buildArtifacts...)
	}

	if len(artifacts) < len(expectedJobNames) {
		s.sendGitHubComment(ctx, prRepoOwner, prRepoName, prNumber,
			"Failed retrieving artifact links. @mattermost/core-build-engineers have been notified. ")
	}

	linksArtifacts := ""
	for _, artifact := range artifacts {
		linksArtifacts += artifact.URL + "  \n"
	}
	s.sendGitHubComment(ctx, prRepoOwner, prRepoName, prNumber, "Artifact links:  \n"+linksArtifacts)
}

func getExpectedArtifacts(jobs []*BuildMobileAppJob, buildJobName string) int {
	for _, job := range jobs {
		if buildJobName == job.JobName {
			return job.ExpectedArtifacts
		}
	}
	return 0
}

func getExpectedJobNames(jobs []*BuildMobileAppJob) []string {
	var expectedJobNames []string
	for _, job := range jobs {
		expectedJobNames = append(expectedJobNames, job.JobName)
	}
	return expectedJobNames
}
