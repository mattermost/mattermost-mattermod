package server

import (
	"context"
	"strconv"

	"github.com/xanzy/go-gitlab"
)

const (
	envKeyPRNumber            = "PR_NUMBER"
	envKeyRefMattermostServer = "REF_MATTERMOST_SERVER"
	envKeyShaMattermostServer = "SHA_MATTERMOST_SERVER"
	envKeyRefMattermostWebapp = "REF_MATTERMOST_WEBAPP"
	envKeyShaMattermostWebapp = "SHA_MATTERMOST_WEBAPP"
	variableTypeEnvVar        = "env_var"
)

type GitLabClient struct {
	client *gitlab.Client

	Pipelines PipelinesService
}

func NewGitLabClient(accessToken string, baseURL string) (*GitLabClient, error) {
	c, err := gitlab.NewClient(accessToken, gitlab.WithBaseURL(baseURL))
	if err != nil {
		return nil, err
	}

	return &GitLabClient{
		client:    c,
		Pipelines: c.Pipelines,
	}, nil
}

// PipelinesService exposes an interface of GitLabCI client.
// Useful to mock in tests.
type PipelinesService interface {
	CancelPipelineBuild(pid interface{}, pipeline int, options ...gitlab.RequestOptionFunc) (*gitlab.Pipeline, *gitlab.Response, error)
	CreatePipeline(pid interface{}, opt *gitlab.CreatePipelineOptions, options ...gitlab.RequestOptionFunc) (*gitlab.Pipeline, *gitlab.Response, error)
	GetPipeline(pid interface{}, pipeline int, options ...gitlab.RequestOptionFunc) (*gitlab.Pipeline, *gitlab.Response, error)
	GetPipelineVariables(pid interface{}, pipeline int, options ...gitlab.RequestOptionFunc) ([]*gitlab.PipelineVariable, *gitlab.Response, error)
	ListProjectPipelines(pid interface{}, opt *gitlab.ListProjectPipelinesOptions, options ...gitlab.RequestOptionFunc) ([]*gitlab.PipelineInfo, *gitlab.Response, error)
}

func (s *Server) triggerE2EGitLabPipeline(ctx context.Context, info *E2ETestTriggerInfo) (string, error) {
	defaultEnvs := []*gitlab.PipelineVariable{
		{
			Key:          envKeyRefMattermostServer,
			Value:        info.ServerBranch,
			VariableType: variableTypeEnvVar,
		},
		{
			Key:          envKeyShaMattermostServer,
			Value:        info.ServerSHA,
			VariableType: variableTypeEnvVar,
		},
		{
			Key:          envKeyRefMattermostWebapp,
			Value:        info.WebappBranch,
			VariableType: variableTypeEnvVar,
		},
		{
			Key:          envKeyShaMattermostWebapp,
			Value:        info.WebappSHA,
			VariableType: variableTypeEnvVar,
		},
		{
			Key:          envKeyPRNumber,
			Value:        strconv.Itoa(info.TriggerPR),
			VariableType: variableTypeEnvVar,
		},
	}
	var customEnvs []*gitlab.PipelineVariable
	if info.EnvVars != nil {
		for k, v := range info.EnvVars {
			customEnvs = append(customEnvs, &gitlab.PipelineVariable{
				Key:          k,
				Value:        v,
				VariableType: variableTypeEnvVar,
			})
		}
	}
	createOpts := &gitlab.CreatePipelineOptions{
		Ref:       &info.RefToTrigger,
		Variables: append(defaultEnvs, customEnvs...),
	}
	pip, _, err := s.GitLabCIClientV4.Pipelines.CreatePipeline(s.Config.E2EGitLabProject, createOpts, gitlab.WithContext(ctx))
	if err != nil {
		return "", err
	}

	return pip.WebURL, nil
}

func (s *Server) checkForPipelinesWithSameEnvs(ctx context.Context, info *E2ETestTriggerInfo) (bool, error) {
	hasC, err := s.checkPipelinesForSameEnvs(ctx, info, gitlab.Created)
	if err != nil {
		return false, err
	}
	hasP, err := s.checkPipelinesForSameEnvs(ctx, info, gitlab.Pending)
	if err != nil {
		return false, err
	}
	hasR, err := s.checkPipelinesForSameEnvs(ctx, info, gitlab.Running)
	if err != nil {
		return false, err
	}
	if hasC || hasP || hasR {
		return true, nil
	}
	return false, nil
}

func (s *Server) checkPipelinesForSameEnvs(ctx context.Context, info *E2ETestTriggerInfo, state gitlab.BuildStateValue) (bool, error) {
	listOpts := &gitlab.ListProjectPipelinesOptions{
		Status: gitlab.BuildState(state),
		Ref:    &info.RefToTrigger,
	}
	pips, _, err := s.GitLabCIClientV4.Pipelines.ListProjectPipelines(s.Config.E2EGitLabProject, listOpts, gitlab.WithContext(ctx))
	if err != nil {
		return false, err
	}
	for _, pip := range pips {
		glVars, _, err2 := s.GitLabCIClientV4.Pipelines.GetPipelineVariables(s.Config.E2EGitLabProject, pip.ID, gitlab.WithContext(ctx))
		if err2 != nil {
			return false, err2
		}
		has, err2 := hasSameEnvs(info, glVars)
		if err2 != nil {
			return false, err2
		}
		if has {
			return true, nil
		}
	}
	return false, nil
}

// The following is a sample response of retrieving pipeline environment variables of one of our e2e testing pipelines via api.
// The first element are custom options, the rest of the elements, every pipeline has.
// [
// {
// "key": "MM_ENV",
// "value": "MM_FEATUREFLAGS_GRAPHQL=true"
// },
// {
// "key": "PR_NUMBER",
// "value": "10857"
// },
// {
// "key": "REF_MATTERMOST_SERVER",
// "value": "master"
// },
// {
// "key": "REF_MATTERMOST_WEBAPP",
// "value": "update-cypress-report"
// },
// {
// "key": "SHA_MATTERMOST_SERVER",
// "value": ""
// },
// {
// "key": "SHA_MATTERMOST_WEBAPP",
// "value": "c4a1b4cc4a7fdce4e007e95c840e59e0d976e3f3"
// }
// ]
func hasSameEnvs(info *E2ETestTriggerInfo, glVars []*gitlab.PipelineVariable) (bool, error) {
	glRequiredEnvVars := map[string]bool{
		envKeyPRNumber:            true,
		envKeyRefMattermostServer: true,
		envKeyShaMattermostServer: true,
		envKeyRefMattermostWebapp: true,
		envKeyShaMattermostWebapp: true,
	}

	// Pack the gitlab env_vars in a map
	glEnvVars := make(map[string]string)
	for _, glVar := range glVars {
		glEnvVars[glVar.Key] = glVar.Value
	}

	// Check: if the PR number differs, it's not the same pipeline
	glPRNumber, err := strconv.Atoi(glEnvVars[envKeyPRNumber])
	if err != nil {
		return false, err
	}
	if glPRNumber != info.TriggerPR {
		return false, nil
	}

	// Check: if the options are not exactly equal, it's not the same pipeline
	for requiredVar, _ := range glRequiredEnvVars {
		delete(glEnvVars, requiredVar) // It's fine even if keys that are not there
	}
	if len(glEnvVars) != len(info.EnvVars) {
		return false, nil
	} else {
		for envVar, envVarValue := range info.EnvVars {
			glEnvVarValue, glEnvVarExists := glEnvVars[envVar]
			if !glEnvVarExists || glEnvVarValue != envVarValue {
				return false, nil
			}
		}
	}

	return true, nil
}

func (s *Server) cancelPipelinesForPR(ctx context.Context, e2eProjectRef *string, prNumber *int) ([]*string, error) { // pending, created, running
	pipInfosC, err := s.findCancellablePipelines(ctx, gitlab.Created, e2eProjectRef, prNumber)
	if err != nil {
		return nil, err
	}
	pipInfosP, err := s.findCancellablePipelines(ctx, gitlab.Pending, e2eProjectRef, prNumber)
	if err != nil {
		return nil, err
	}
	pipInfosR, err := s.findCancellablePipelines(ctx, gitlab.Running, e2eProjectRef, prNumber)
	if err != nil {
		return nil, err
	}

	var urls []*string
	urlsC, err := s.cancelPipeline(ctx, pipInfosC)
	if err != nil {
		return nil, err
	}
	if urlsC != nil {
		urls = append(urls, urlsC...)
	}
	urlsP, err := s.cancelPipeline(ctx, pipInfosP)
	if err != nil {
		return nil, err
	}
	if urlsP != nil {
		urls = append(urls, urlsP...)
	}
	urlsR, err := s.cancelPipeline(ctx, pipInfosR)
	if err != nil {
		return nil, err
	}
	if urlsR != nil {
		urls = append(urls, urlsR...)
	}

	if len(urls) == 0 {
		return nil, nil
	}
	return urls, nil
}

func (s *Server) findCancellablePipelines(ctx context.Context, state gitlab.BuildStateValue, e2eProjectRef *string, prNumber *int) ([]*gitlab.PipelineInfo, error) {
	listOpts := &gitlab.ListProjectPipelinesOptions{
		Status: gitlab.BuildState(state),
		Ref:    e2eProjectRef,
	}
	pips, _, err := s.GitLabCIClientV4.Pipelines.ListProjectPipelines(s.Config.E2EGitLabProject, listOpts, gitlab.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	var matches []*gitlab.PipelineInfo
	for _, pip := range pips {
		glVars, _, err2 := s.GitLabCIClientV4.Pipelines.GetPipelineVariables(s.Config.E2EGitLabProject, pip.ID, gitlab.WithContext(ctx))
		if err2 != nil {
			return nil, err
		}
		hasPRNumber, err2 := isPRPipeline(prNumber, glVars)
		if err2 != nil {
			return nil, err
		}
		if hasPRNumber {
			matches = append(matches, pip)
		}
	}
	return matches, nil
}

func isPRPipeline(prNumber *int, glVars []*gitlab.PipelineVariable) (bool, error) {
	for _, glVar := range glVars {
		if glVar.Key == envKeyPRNumber {
			envPRNumber, err := strconv.Atoi(glVar.Value)
			if err != nil {
				return false, err
			}
			if envPRNumber == *prNumber {
				return true, nil
			}
		}
	}
	return false, nil
}

func (s *Server) cancelPipeline(ctx context.Context, infos []*gitlab.PipelineInfo) ([]*string, error) {
	if len(infos) == 0 {
		return nil, nil
	}
	var urls []*string
	for _, info := range infos {
		_, _, err := s.GitLabCIClientV4.Pipelines.CancelPipelineBuild(info.ProjectID, info.ID, gitlab.WithContext(ctx))
		if err != nil {
			return nil, err
		}
		urls = append(urls, &info.WebURL)
	}
	return urls, nil
}
