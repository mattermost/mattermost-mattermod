// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

type LabelResponse struct {
	Label   string
	Message string
}

type Repository struct {
	Owner                      string
	Name                       string
	BuildStatusContext         string
	JenkinsServer              string
	InstanceSetupScript        string
	InstanceSetupUpgradeScript string
	JobName                    string
}

type JenkinsCredentials struct {
	URL      string
	Username string
	ApiToken string
}

type Integration struct {
	RepositoryName  string
	Files           []string
	IntegrationLink string
	Message         string
}

type BuildMobileAppJob struct {
	JobName           string
	ExpectedArtifacts int
}

type ServerConfig struct {
	ListenAddress               string
	MattermodURL                string
	GithubAccessToken           string
	GitHubTokenReserve          int
	GithubUsername              string
	GithubAccessTokenCherryPick string
	GitHubWebhookSecret         string
	Org                         string
	Username                    string
	AutoAssignerTeam            string
	AutoAssignerTeamID          int64
	CircleCIToken               string

	TickRateMinutes int

	DriverName string
	DataSource string

	Repositories []*Repository

	BlockPRMergeLabels []string
	AutoPRMergeLabel   string

	BuildMobileAppTag           string
	BuildMobileAppInitMessage   string
	BuildMobileAppDoneMessage   string
	BuildMobileAppFailedMessage string
	BuildMobileAppBranchPrefix  string
	BuildMobileAppJobs          []*BuildMobileAppJob

	StartLoadtestTag     string
	StartLoadtestMessage string

	SignedCLAURL          string
	NeedsToSignCLAMessage string

	PrLabels    []LabelResponse
	IssueLabels []LabelResponse

	JenkinsCredentials map[string]*JenkinsCredentials

	DockerRegistryURL string
	DockerUsername    string
	DockerPassword    string

	BlacklistPaths []string
	Integrations   []*Integration

	MattermostWebhookURL    string
	MattermostWebhookFooter string

	LogSettings struct {
		EnableConsole bool
		ConsoleJson   bool
		ConsoleLevel  string
		EnableFile    bool
		FileJson      bool
		FileLevel     string
		FileLocation  string
	}

	DaysUntilStale    int
	ExemptStaleLabels []string
	StaleLabel        string
	StaleComment      string
}

func findConfigFile(fileName string) string {
	if _, err := os.Stat("/tmp/" + fileName); err == nil {
		fileName, _ = filepath.Abs("/tmp/" + fileName)
	} else if _, err := os.Stat("./config/" + fileName); err == nil {
		fileName, _ = filepath.Abs("./config/" + fileName)
	} else if _, err := os.Stat("../config/" + fileName); err == nil {
		fileName, _ = filepath.Abs("../config/" + fileName)
	} else if _, err := os.Stat(fileName); err == nil {
		fileName, _ = filepath.Abs(fileName)
	}

	return fileName
}

func GetConfig(fileName string) (*ServerConfig, error) {
	config := &ServerConfig{}
	fileName = findConfigFile(fileName)

	file, err := os.Open(fileName)
	if err != nil {
		return config, errors.Wrap(err, "unable to open config file")
	}

	decoder := json.NewDecoder(file)
	err = decoder.Decode(config)
	if err != nil {
		return config, errors.Wrap(err, "unable to decode config file")
	}

	return config, nil
}

func GetRepository(repositories []*Repository, owner, name string) (*Repository, bool) {
	for _, repo := range repositories {
		if repo.Owner == owner && repo.Name == name {
			return repo, true
		}
	}

	return nil, false
}
