package server

import (
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetRelevantIntegrationsForPR(t *testing.T) {
	repoServer := "mattermost-server"
	repoClient := "mmmctl"

	aIntegration := &Integration{
		RepositoryName:  repoServer,
		Files:           nil,
		IntegrationLink: "",
		Message:         "First",
	}
	bIntegration := &Integration{
		RepositoryName:  repoServer,
		Files:           nil,
		IntegrationLink: "",
		Message:         "Second",
	}
	integrations := []*Integration{aIntegration, bIntegration}

	pr := &model.PullRequest{
		RepoName:        repoServer,
	}
	configs := getRelevantIntegrationsForPR(pr, integrations)
	assert.Equal(t, 2, len(configs))
	assert.Equal(t, repoServer, configs[0].RepositoryName)
	assert.Equal(t, repoServer, configs[1].RepositoryName)
	assert.Equal(t, "First", configs[0].Message)
	assert.Equal(t, "Second", configs[1].Message)

	aIntegration = &Integration{
		RepositoryName:  repoServer,
		Files:           nil,
		IntegrationLink: "",
		Message:         "",
	}
	bIntegration = &Integration{
		RepositoryName:  "mmctl",
		Files:           nil,
		IntegrationLink: "",
		Message:         "",
	}
	integrations = []*Integration{aIntegration, bIntegration}

	pr = &model.PullRequest{
		RepoName:        repoServer,
	}
	configs = getRelevantIntegrationsForPR(pr, integrations)
	assert.Equal(t, 1, len(configs))
	assert.Equal(t, repoServer, configs[0].RepositoryName)

	aIntegration = &Integration{
		RepositoryName:  repoClient,
		Files:           nil,
		IntegrationLink: "",
		Message:         "",
	}
	bIntegration = &Integration{
		RepositoryName:  repoClient,
		Files:           nil,
		IntegrationLink: "",
		Message:         "",
	}
	integrations = []*Integration{aIntegration, bIntegration}

	pr = &model.PullRequest{
		RepoName:        repoServer,
	}
	configs = getRelevantIntegrationsForPR(pr, integrations)
	assert.Nil(t, configs)
}

func TestGetMatchingFilenames(t *testing.T) {
	x := "go.go"
	y := "go_test.go"
	a := []string{x, y}
	b := []string{x, y}
	matches := getMatchingFilenames(a, b)
	assert.Equal(t, 2, len(matches))

	a = []string{x}
	b = []string{x, y}
	matches = getMatchingFilenames(a, b)
	assert.Equal(t, 1, len(matches))

	a = []string{}
	b = []string{x, y}
	matches = getMatchingFilenames(a, b)
	assert.Equal(t, 0, len(matches))
}
