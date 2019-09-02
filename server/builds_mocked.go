package server

import (
	"context"

	jenkins "github.com/cpanato/golang-jenkins"
	"github.com/heroku/docker-registry-client/registry"
	"github.com/mattermost/mattermost-mattermod/model"
)

// MockedBuilds implements buildsInterface but returns hardcoded information.
// This is used for local development and/or testing.
type MockedBuilds struct {
	Version string
}

func (b *MockedBuilds) getInstallationVersion(pr *model.PullRequest) string {
	return b.Version
}

func (b *MockedBuilds) buildJenkinsClient(s *Server, pr *model.PullRequest) (*Repository, *jenkins.Jenkins, error) {
	return nil, nil, nil
}

func (b *MockedBuilds) dockerRegistryClient(s *Server) (*registry.Registry, error) {
	return nil, nil
}

func (b *MockedBuilds) waitForImage(ctx context.Context, s *Server, reg *registry.Registry, pr *model.PullRequest) (*model.PullRequest, error) {
	return pr, nil
}

func (b *MockedBuilds) waitForBuild(ctx context.Context, s *Server, client *jenkins.Jenkins, pr *model.PullRequest) (*model.PullRequest, error) {
	return pr, nil
}

func (b *MockedBuilds) checkBuildLink(ctx context.Context, s *Server, pr *model.PullRequest) (string, error) {
	return "mocked", nil
}
