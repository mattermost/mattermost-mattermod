// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v32/github"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v5/mlog"
	"github.com/pkg/errors"

	"github.com/mattermost/go-circleci"
)

// CircleCIService exposes an interface of CircleCI client.
// Useful to mock in tests.
type CircleCIService interface {
	// ListRecentBuildsForProject fetches the list of recent builds for the given repository
	// The status and branch parameters are used to further filter results if non-empty
	// If limit is -1, fetches all builds.
	ListRecentBuildsForProjectWithContext(ctx context.Context, vcsType circleci.VcsType, account, repo, branch, status string, limit, offset int) ([]*circleci.Build, error)
	// BuildByProjectWithContext triggers a build by project.
	BuildByProjectWithContext(ctx context.Context, vcsType circleci.VcsType, account, repo string, opts map[string]interface{}) error
	// ListBuildArtifactsWithContext fetches the build artifacts for the given build.
	ListBuildArtifactsWithContext(ctx context.Context, vcsType circleci.VcsType, account, repo string, buildNum int) ([]*circleci.Artifact, error)
	// TriggerPipeline triggers a new pipeline for the given project for the given branch or tag.
	TriggerPipelineWithContext(ctx context.Context, vcsType circleci.VcsType, account, repo, branch, tag string, params map[string]interface{}) (*circleci.Pipeline, error)
	// GetPipelineWorkflowWithContext returns a list of paginated workflows by pipeline ID
	GetPipelineWorkflowWithContext(ctx context.Context, pipelineID, pageToken string) (*circleci.WorkflowList, error)
}

func (s *Server) triggerCircleCIIfNeeded(ctx context.Context, pr *model.PullRequest) error {
	mlog.Info("Checking if need trigger CircleCI", mlog.String("repo", pr.RepoName), mlog.Int("pr", pr.Number), mlog.String("fullname", pr.FullName))
	repoInfo := strings.Split(pr.FullName, "/")
	if repoInfo[0] == s.Config.Org {
		// It is from upstream mattermost repo don't need to trigger the circleci because org members
		// have permissions
		return nil
	}

	// Checking if the repo have CircleCI setup
	builds, err := s.CircleCiClient.ListRecentBuildsForProjectWithContext(ctx, circleci.VcsTypeGithub, pr.RepoOwner, pr.RepoName, "master", "", 5, 0)
	if err != nil {
		return fmt.Errorf("could not list the CircleCI builds for project: %w", err)
	}

	// If builds are 0 means no build ran for master and most probably this is not setup, so skipping.
	if len(builds) == 0 {
		return nil
	}

	// List the files that was modified or added in the PullRequest
	prFiles, err := s.getFiles(ctx, pr.RepoOwner, pr.RepoName, pr.Number)
	if err != nil {
		return fmt.Errorf("could not list the files for the #%d: %w", pr.Number, err)
	}

	err = s.validateBlockPaths(pr.RepoName, prFiles)
	var blockError *BlockPathValidationError
	if err != nil && errors.As(err, &blockError) {
		mlog.Info("Files found in the block list", mlog.Err(err))
		if cErr := s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, blockError.ReportBlockFiles()); cErr != nil {
			mlog.Warn("Error while commenting", mlog.Err(cErr))
		}
		return err
	}

	opts := map[string]interface{}{
		"revision": pr.Sha,
		"branch":   fmt.Sprintf("pull/%d", pr.Number),
	}

	err = s.CircleCiClient.BuildByProjectWithContext(ctx, circleci.VcsTypeGithub, pr.RepoOwner, pr.RepoName, opts)
	if err != nil {
		return fmt.Errorf("could not trigger circleci: %w", err)
	}

	return nil
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

type BlockPathValidationError struct {
	files []string
}

// Error implements the error interface.
func (e *BlockPathValidationError) Error() string {
	return "files in the Block List " + strings.Join(e.files, ",")
}

// BlockListFiles return an array of block files
func (e *BlockPathValidationError) BlockListFiles() []string {
	return e.files
}

// ReportBlockFiles return a message based on how many files are in the block list
// to be send out
func (e *BlockPathValidationError) ReportBlockFiles() string {
	var msg string
	if len(e.files) > 1 {
		msg = fmt.Sprintf("The files `%s` are in the blocklist for external contributors. Hence, these changes are not tested by the CI pipeline active until the build is re-triggered by a core committer or the PR is merged. Please be careful when reviewing it.\n/cc @mattermost/core-security @mattermost/core-build-engineers", strings.Join(e.files, ", "))
	} else {
		msg = fmt.Sprintf("The file `%s` is in the blocklist for external contributors. Hence, these changes are not tested by the CI pipeline active until the build is re-triggered by a core committer or the PR is merged. Please be careful when reviewing it.\n/cc @mattermost/core-security @mattermost/core-build-engineers", e.files[0])
	}
	return msg
}

func newBlockPathValidationError(files []string) *BlockPathValidationError {
	return &BlockPathValidationError{
		files: files,
	}
}

func (s *Server) validateBlockPaths(repo string, prFiles []*github.CommitFile) error {
	blockList := s.Config.BlockListPathsGlobal
	repoBlockList, ok := s.Config.BlockListPathsPerRepo[repo]
	if ok {
		blockList = append(blockList, repoBlockList...)
	}

	var matches []string
	for _, prFile := range prFiles {
		for _, blockListPath := range blockList {
			if matched, err := filepath.Match(blockListPath, prFile.GetFilename()); err != nil {
				mlog.Error("failed to match the file", mlog.String("blockPathPattern", blockListPath), mlog.String("filename", prFile.GetFilename()), mlog.Err(err))

				continue
			} else if matched {
				matches = append(matches, prFile.GetFilename())
			}
		}
	}

	if len(matches) > 0 {
		return newBlockPathValidationError(matches)
	}

	return nil
}

func (s *Server) waitForWorkflowID(ctx context.Context, id string, workflowName string) (string, error) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return "", errors.New("timed out trying to fetch workflow")
		case <-ticker.C:
			token := ""
			workflowID := ""
			for {
				wfList, err := s.CircleCiClientV2.GetPipelineWorkflowWithContext(ctx, id, token)
				if err != nil {
					var apiError *circleci.APIError
					if errors.As(err, &apiError) && apiError.HTTPStatusCode >= 400 && apiError.HTTPStatusCode < 500 {
						// We retry if it's a client side issue
						continue
					}
					return "", err
				}

				for _, wf := range wfList.Items {
					if wf.Name == workflowName {
						workflowID = wf.ID
						break
					}
				}

				if workflowID != "" {
					return workflowID, nil
				}

				if wfList.NextPageToken == "" {
					break
				}
				token = wfList.NextPageToken
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
