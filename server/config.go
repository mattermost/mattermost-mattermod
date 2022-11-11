// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

const (
	// In seconds
	defaultRequestTimeout  = 60
	defaultEETaskTimeout   = 300
	defaultCronTaskTimeout = 600
	defaultBuildAppTimeout = 7200
)

type LabelResponse struct {
	Label   string
	Message string
}

type Repository struct {
	Owner                      string
	Name                       string
	BuildStatusContext         string
	InstanceSetupScript        string
	InstanceSetupUpgradeScript string
	JobName                    string
	GreetingTeam               string   // GreetingTeam is the GitHub team responsible for triaging non-member PRs for this repo.
	GreetingLabels             []string // GreetingLabels are the labels applied automatically to non-member PRs for this repo.
}

type CloudRepository struct {
	Name       string
	MainBranch string
}

type Integration struct {
	RepositoryName  string
	IntegrationLink string
	Message         string
	Files           []string
}

type BuildAppJob struct {
	JobName           string
	RepoName          string
	ExpectedArtifacts int
}

// nolint:govet
type Config struct {
	ListenAddress               string
	MattermodURL                string
	GithubAccessToken           string
	GitHubTokenReserve          int
	GithubUsername              string
	GithubEmail                 string
	GithubAccessTokenCherryPick string
	GitHubWebhookSecret         string
	Org                         string
	Username                    string
	AutoAssignerTeam            string
	AutoAssignerTeamID          int64
	CircleCIToken               string
	GitLabInternalURL           string
	GitLabInternalToken         string

	TickRateMinutes int

	DriverName string
	DataSource string

	Repositories      []*Repository
	CloudRepositories []*CloudRepository

	BlockPRMergeLabels []string
	AutoPRMergeLabel   string

	BuildAppTag           string
	BuildAppInitMessage   string
	BuildAppDoneMessage   string
	BuildAppFailedMessage string
	BuildAppBranchPrefix  string
	BuildAppJobs          []*BuildAppJob

	E2EDockerRepo          string
	E2EGitLabProject       string
	E2EWebappRef           string
	E2EServerRef           string
	E2EWebappReponame      string
	E2EServerReponame      string
	E2EWebappStatusContext string
	E2EServerStatusContext string
	E2ETriggerLabel        []string

	E2EMobileCoreReponame  string
	E2EMobileGitLabProject string

	EnterpriseReponame            string
	EnterpriseTriggerReponame     string
	EnterpriseWebappReponame      string
	EnterpriseTriggerLabel        string
	EnterpriseGithubStatusContext string
	EnterpriseGithubStatusTETests string
	EnterpriseGithubStatusEETests string
	EnterpriseWorkflowName        string

	TranslationsMattermostWebhookURL string
	TranslationsMattermostMessage    string
	TranslationsBot                  string
	TranslationsDoNotMergeLabel      string
	TranslationsMergedMessage        string
	TranslationsMergeFailureMessage  string
	TranslationsMergePolicy          string

	StartLoadtestTag     string
	StartLoadtestMessage string

	CLAExclusionsList      []string
	CLAGithubStatusContext string

	SignedCLAURL     string
	PRWelcomeMessage string

	PrLabels    []LabelResponse
	IssueLabels []LabelResponse

	IssueLabelsToCleanUp []string

	BlockListPathsGlobal  []string
	BlockListPathsPerRepo map[string][]string // BlockListPathsPerRepo is a per repository list of blocked files

	MattermostWebhookURL    string
	MattermostWebhookFooter string

	LogSettings struct {
		ConsoleLevel  string
		FileLevel     string
		FileLocation  string
		EnableConsole bool
		EnableColor   bool
		ConsoleJSON   bool
		EnableFile    bool
		FileJSON      bool
	}

	DaysUntilStale    int
	ExemptStaleLabels []string
	StaleLabel        string
	StaleComment      string

	MetricsServerPort string

	RepoFolder    string // folder containing local checkouts of repositories for cherry-picking
	ScriptsFolder string // folder containing the cherry-pick.sh script
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

func GetConfig(fileName string) (*Config, error) {
	config := &Config{}
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
