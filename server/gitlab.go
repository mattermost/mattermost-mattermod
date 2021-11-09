package server

import (
	"context"
	"strconv"

	"github.com/xanzy/go-gitlab"
)

const (
	envKeyPRNumber = "PR_NUMBER"
	envKeyBuildTag = "BUILD_TAG"
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
	createOpts := &gitlab.CreatePipelineOptions{
		Ref: &info.RefToTrigger,
		Variables: []*gitlab.PipelineVariable{
			{
				Key:   "REF_MATTERMOST_WEBAPP",
				Value: info.WebappBranch,
			},
			{
				Key:   "SHA_MATTERMOST_WEBAPP",
				Value: info.WebappSHA,
			},
			{
				Key:   "REF_MATTERMOST_SERVER",
				Value: info.ServerBranch,
			},
			{
				Key:   "SHA_MATTERMOST_SERVER",
				Value: info.WebappSHA,
			},
			{
				Key:   envKeyBuildTag,
				Value: info.BuildTag,
			},
			{
				Key:   envKeyPRNumber,
				Value: strconv.Itoa(info.TriggerPR),
			},
		},
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

func hasSameEnvs(info *E2ETestTriggerInfo, glVars []*gitlab.PipelineVariable) (bool, error) {
	if info.EnvVars == nil {
		matching := 2
		i := 0
		for _, glVar := range glVars {
			if glVar.Key == envKeyPRNumber {
				pr, err := strconv.Atoi(glVar.Value)
				if err != nil {
					return false, err
				}
				if pr != info.TriggerPR {
					return false, nil
				}
				i++
			}
			if glVar.Key == envKeyBuildTag {
				if glVar.Value != info.BuildTag {
					return false, nil
				}
				i++
			}
		}
		if matching == i {
			return true, nil
		}
		return false, nil
	}
	i := 0
	matching := len(info.EnvVars)
	for k, v := range info.EnvVars {
		for _, glVar := range glVars {
			if glVar.Key == envKeyPRNumber {
				pr, err := strconv.Atoi(glVar.Value)
				if err != nil {
					return false, err
				}
				if pr != info.TriggerPR {
					return false, nil
				}
			}
			if glVar.Key == envKeyBuildTag && glVar.Value != info.BuildTag {
				return false, nil
			}

			if k == glVar.Key && v == glVar.Value {
				i++
			}
		}
	}
	if matching == i {
		return true, nil
	}
	return false, nil
}

func (s *Server) cancelPipelinesForPR(ctx context.Context, prRef *string, prNumber *int) ([]*string, error) { // pending, created, running
	pipInfosC, err := s.findCancellablePipelines(ctx, gitlab.Created, prRef, prNumber)
	if err != nil {
		return nil, err
	}
	pipInfosP, err := s.findCancellablePipelines(ctx, gitlab.Pending, prRef, prNumber)
	if err != nil {
		return nil, err
	}
	pipInfosR, err := s.findCancellablePipelines(ctx, gitlab.Running, prRef, prNumber)
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

func (s *Server) findCancellablePipelines(ctx context.Context, state gitlab.BuildStateValue, prRef *string, prNumber *int) ([]*gitlab.PipelineInfo, error) {
	listOpts := &gitlab.ListProjectPipelinesOptions{
		Status: gitlab.BuildState(state),
		Ref:    prRef,
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
