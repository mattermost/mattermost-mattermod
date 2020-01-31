package server

import (
	"github.com/mattermost/mattermost-mattermod/model"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetRelevantIntegrationsForPR(t *testing.T) {
	aIntegration := &Integration{
		RepositoryOwner: "mattermost",
		RepositoryName:  "mattermost-server",
		Files:           nil,
		IntegrationLink: "",
		Message:         "First",
	}
	bIntegration := &Integration{
		RepositoryOwner: "mattermost",
		RepositoryName:  "mattermost-server",
		Files:           nil,
		IntegrationLink: "",
		Message:         "Second",
	}
	integrations := []*Integration{aIntegration, bIntegration}

	pr := &model.PullRequest{
		RepoOwner:       "mattermost",
		RepoName:        "mattermost-server",
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
	assert.Equal(t, "mattermost", configs[0].RepositoryOwner)
	assert.Equal(t, "mattermost", configs[1].RepositoryOwner)
	assert.Equal(t, "mattermost-server", configs[0].RepositoryName)
	assert.Equal(t, "mattermost-server", configs[1].RepositoryName)
	assert.Equal(t, "First", configs[0].Message)
	assert.Equal(t, "Second", configs[1].Message)

	aIntegration = &Integration{
		RepositoryOwner: "mattermost",
		RepositoryName:  "mattermost-server",
		Files:           nil,
		IntegrationLink: "",
		Message:         "",
	}
	bIntegration = &Integration{
		RepositoryOwner: "mattermost",
		RepositoryName:  "mmctl",
		Files:           nil,
		IntegrationLink: "",
		Message:         "",
	}
	integrations = []*Integration{aIntegration, bIntegration}

	pr = &model.PullRequest{
		RepoOwner:       "mattermost",
		RepoName:        "mattermost-server",
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
	assert.Equal(t, "mattermost", configs[0].RepositoryOwner)
	assert.Equal(t, "mattermost-server", configs[0].RepositoryName)

	aIntegration = &Integration{
		RepositoryOwner: "mattermost",
		RepositoryName:  "mmctl",
		Files:           nil,
		IntegrationLink: "",
		Message:         "",
	}
	bIntegration = &Integration{
		RepositoryOwner: "mattermost",
		RepositoryName:  "mmctl",
		Files:           nil,
		IntegrationLink: "",
		Message:         "",
	}
	integrations = []*Integration{aIntegration, bIntegration}

	pr = &model.PullRequest{
		RepoOwner:       "mattermost",
		RepoName:        "mattermost-server",
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
	aNames := []string{"config.yaml", "config.yml"}
	bNames := []string{"config.yaml", "config.yml"}
	matches := getMatchingFilenames(aNames, bNames)
	assert.Equal(t, 2, len(matches))

	aNames = []string{"config.yaml"}
	bNames = []string{"config.yaml", "config.yml"}
	matches = getMatchingFilenames(aNames, bNames)
	assert.Equal(t, 1, len(matches))

	aNames = []string{}
	bNames = []string{"config.yaml", "config.yml"}
	matches = getMatchingFilenames(aNames, bNames)
	assert.Equal(t, 0, len(matches))
}
