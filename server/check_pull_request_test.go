package server

import (
	"github.com/mattermost/mattermost-mattermod/model"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetRelevantIntegrationsForPR(t *testing.T) {
	owner := "mattermost"
	repoServer := "mattermost-server"
	repoClient := "mmmctl"

	aIntegration := &Integration{
		RepositoryOwner: owner,
		RepositoryName:  repoServer,
		Files:           nil,
		IntegrationLink: "",
		Message:         "First",
	}
	bIntegration := &Integration{
		RepositoryOwner: owner,
		RepositoryName:  repoServer,
		Files:           nil,
		IntegrationLink: "",
		Message:         "Second",
	}
	integrations := []*Integration{aIntegration, bIntegration}

	pr := &model.PullRequest{
		RepoOwner:       owner,
		RepoName:        repoServer,
		FullName:        "",
		Number:          0,
		Username:        "",
		Ref:             "",
		Sha:             "",
		Labels:          nil,
		State:           "",
		BuildStatus:     "",
		BuildConclusion: "",
		BuildLink:       "",
		URL:             "",
		CreatedAt:       time.Time{},
	}
	configs := getRelevantIntegrationsForPR(pr, integrations)
	assert.Equal(t, 2, len(configs))
	assert.Equal(t, owner, configs[0].RepositoryOwner)
	assert.Equal(t, owner, configs[1].RepositoryOwner)
	assert.Equal(t, repoServer, configs[0].RepositoryName)
	assert.Equal(t, repoServer, configs[1].RepositoryName)
	assert.Equal(t, "First", configs[0].Message)
	assert.Equal(t, "Second", configs[1].Message)

	aIntegration = &Integration{
		RepositoryOwner: owner,
		RepositoryName:  repoServer,
		Files:           nil,
		IntegrationLink: "",
		Message:         "",
	}
	bIntegration = &Integration{
		RepositoryOwner: owner,
		RepositoryName:  "mmctl",
		Files:           nil,
		IntegrationLink: "",
		Message:         "",
	}
	integrations = []*Integration{aIntegration, bIntegration}

	pr = &model.PullRequest{
		RepoOwner:       owner,
		RepoName:        repoServer,
		FullName:        "",
		Number:          0,
		Username:        "",
		Ref:             "",
		Sha:             "",
		Labels:          nil,
		State:           "",
		BuildStatus:     "",
		BuildConclusion: "",
		BuildLink:       "",
		URL:             "",
		CreatedAt:       time.Time{},
	}
	configs = getRelevantIntegrationsForPR(pr, integrations)
	assert.Equal(t, 1, len(configs))
	assert.Equal(t, owner, configs[0].RepositoryOwner)
	assert.Equal(t, repoServer, configs[0].RepositoryName)

	aIntegration = &Integration{
		RepositoryOwner: owner,
		RepositoryName:  repoClient,
		Files:           nil,
		IntegrationLink: "",
		Message:         "",
	}
	bIntegration = &Integration{
		RepositoryOwner: owner,
		RepositoryName:  repoClient,
		Files:           nil,
		IntegrationLink: "",
		Message:         "",
	}
	integrations = []*Integration{aIntegration, bIntegration}

	pr = &model.PullRequest{
		RepoOwner:       owner,
		RepoName:        repoServer,
		FullName:        "",
		Number:          0,
		Username:        "",
		Ref:             "",
		Sha:             "",
		Labels:          nil,
		State:           "",
		BuildStatus:     "",
		BuildConclusion: "",
		BuildLink:       "",
		URL:             "",
		CreatedAt:       time.Time{},
	}
	configs = getRelevantIntegrationsForPR(pr, integrations)
	assert.Nil(t, configs)
}

func TestGetMatchingFilenames(t *testing.T) {
	a := []string{"config.yaml", "config.yml"}
	b := []string{"config.yaml", "config.yml"}
	matches := getMatchingFilenames(a, b)
	assert.Equal(t, 2, len(matches))

	a = []string{"config.yaml"}
	b = []string{"config.yaml", "config.yml"}
	matches = getMatchingFilenames(a, b)
	assert.Equal(t, 1, len(matches))

	a = []string{}
	b = []string{"config.yaml", "config.yml"}
	matches = getMatchingFilenames(a, b)
	assert.Equal(t, 0, len(matches))
}
