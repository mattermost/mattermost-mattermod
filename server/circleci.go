// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v5/mlog"
	"github.com/pkg/errors"

	"github.com/metanerd/go-circleci"
)

func (s *Server) triggerCircleCiIfNeeded(pr *model.PullRequest) {
	client := &circleci.Client{Token: s.Config.CircleCIToken}
	mlog.Info("Checking if need trigger circleci", mlog.String("repo", pr.RepoName), mlog.Int("pr", pr.Number), mlog.String("fullname", pr.FullName))
	repoInfo := strings.Split(pr.FullName, "/")
	if repoInfo[0] == s.Config.Org {
		// It is from upstream mattermost repo don't need to trigger the circleci because org members
		// have permissions
		mlog.Info("Don't need to trigger circleci", mlog.String("repo", pr.RepoName), mlog.Int("pr", pr.Number), mlog.String("fullname", pr.FullName))
		return
	}

	// Checking if the repo have circleci setup
	builds, err := client.ListRecentBuildsForProject("github", pr.RepoOwner, pr.RepoName, "master", "", 5, 0)
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
	prFiles, _, err := s.GithubClient.PullRequests.ListFiles(context.Background(), pr.RepoOwner, pr.RepoName, pr.Number, nil)
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

type PipelineTriggeredResponse struct {
	Number    int       `json:"number"`
	State     string    `json:"state"`
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Server) triggerEnterprisePipeline(ctx context.Context, pr *model.PullRequest, info *EETriggerInfo) (*PipelineTriggeredResponse, error) {
	body := strings.NewReader(
		`branch=` + info.EEBranch +
			`&parameters[tbs_sha]=` + pr.Sha +
			`&parameters[tbs_pr]=` + strconv.Itoa(pr.Number) +
			`&parameters[tbs_server_owner]=` + info.ServerOwner +
			`&parameters[tbs_server_branch]=` + info.ServerBranch +
			`&parameters[tbs_webapp_owner]=` + info.WebappOwner +
			`&parameters[tbs_webapp_branch]=` + info.WebappBranch)
	req, err := http.NewRequestWithContext(ctx, "POST", "https://circleci.com/api/v2/project/gh/"+s.Config.Org+"/"+s.Config.EnterpriseReponame+"/pipeline", body)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(os.ExpandEnv(s.Config.CircleCIToken), "")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	mlog.Debug("EE triggered",
		mlog.Int("pr", pr.Number),
		mlog.String("sha", pr.Sha),
		mlog.String("EEBranch", info.EEBranch),
		mlog.String("ServerOwner", info.ServerOwner),
		mlog.String("ServerBranch", info.ServerBranch),
		mlog.String("WebappOwner", info.WebappOwner),
		mlog.String("WebappBranch", info.WebappBranch),
	)
	if err != nil {
		return nil, err
	}
	r := PipelineTriggeredResponse{}
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&r)
	if err != nil {
		return nil, err
	}

	return &r, err
}

type PipelineItem struct {
	StoppedAt   time.Time `json:"stopped_at"`
	Number      int       `json:"pipeline_number"`
	Status      string    `json:"status"`
	WorkflowID  string    `json:"id"`
	Name        string    `json:"name"`
	ProjectSlug string    `json:"project_slug"`
	CreatedAt   time.Time `json:"created_at"`
	ID          string    `json:"pipeline_id"`
}

type PipelineWorkflowResponse struct {
	Pipelines     []PipelineItem `json:"items"`
	NextPageToken string         `json:"next_page_token"`
}

func (s *Server) waitForWorkflowID(ctx context.Context, id string, workflowName string) (string, error) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return "", errors.New("timed out trying to fetch workflow")
		case <-ticker.C:
			req, err := http.NewRequestWithContext(ctx, "GET", "https://circleci.com/api/v2/pipeline/"+id+"/workflow", nil)
			if err != nil {
				return "", err
			}
			req.SetBasicAuth(os.ExpandEnv(s.Config.CircleCIToken), "")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return "", err
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				continue
			}
			r := PipelineWorkflowResponse{}
			err = json.NewDecoder(resp.Body).Decode(&r)
			if err != nil {
				return "", err
			}

			workflowID := ""
			for _, pip := range r.Pipelines {
				if pip.Name == workflowName {
					workflowID = pip.WorkflowID
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
			client := &circleci.Client{Token: s.Config.CircleCIToken}
			var builds []*circleci.Build
			var err error
			builds, err = client.ListRecentBuildsForProject(circleci.VcsTypeGithub, org, pr.RepoName, branch, "running", len(expectedJobNames), 0)
			if err != nil {
				return nil, err
			}

			if len(builds) == 0 {
				builds, err = client.ListRecentBuildsForProject(circleci.VcsTypeGithub, org, pr.RepoName, branch, "", len(expectedJobNames), 0)
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
			client := &circleci.Client{Token: s.Config.CircleCIToken}
			mlog.Debug("Trying to fetch artifacts", mlog.Int("build", buildNumber))
			artifacts, err := client.ListBuildArtifacts(circleci.VcsTypeGithub, org, pr.RepoName, buildNumber)
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
