// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/mattermost/mattermost-server/v5/mlog"
	"github.com/pkg/errors"
)

const (
	// In seconds
	defaultRequestTimeout       = 60
	defaultEETaskTimeout        = 300
	defaultCronTaskTimeout      = 600
	defaultBuildMobileTimeout   = 7200
	defaultBuildSpinmintTimeout = 2700
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
	APIToken string
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

type Config struct {
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

	TickRateMinutes        int
	SpinmintExpirationHour int

	DriverName string
	DataSource string

	Repositories []*Repository

	BlockPRMergeLabels []string
	AutoPRMergeLabel   string

	SetupSpinmintTag                   string
	SetupSpinmintMessage               string
	SetupSpinmintDoneMessage           string
	SetupSpinmintFailedMessage         string
	DestroyedSpinmintMessage           string
	DestroyedExpirationSpinmintMessage string
	SpinmintsUseHTTPS                  bool

	SetupSpinmintUpgradeTag         string
	SetupSpinmintUpgradeMessage     string
	SetupSpinmintUpgradeDoneMessage string

	BuildMobileAppTag           string
	BuildMobileAppInitMessage   string
	BuildMobileAppDoneMessage   string
	BuildMobileAppFailedMessage string
	BuildMobileAppBranchPrefix  string
	BuildMobileAppJobs          []*BuildMobileAppJob

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

	StartLoadtestTag     string
	StartLoadtestMessage string

	CLAExclusionsList      []string
	CLAGithubStatusContext string

	SignedCLAURL          string
	NeedsToSignCLAMessage string

	PrLabels    []LabelResponse
	IssueLabels []LabelResponse

	IssueLabelsToCleanUp []string

	JenkinsCredentials map[string]*JenkinsCredentials

	DockerRegistryURL string
	DockerUsername    string
	DockerPassword    string

	BlockListPathsGlobal  []string
	BlockListPathsPerRepo map[string][]string // BlockListPathsPerRepo is a per repository list of blocked files

	AWSCredentials struct {
		ID     string
		Secret string
		Token  string
	}

	AWSRegion        string
	AWSImageID       string
	AWSKeyName       string
	AWSInstanceType  string
	AWSHostedZoneID  string
	AWSSecurityGroup string
	AWSDnsSuffix     string
	AWSSubNetID      string

	MattermostWebhookURL    string
	MattermostWebhookFooter string

	LogSettings struct {
		EnableConsole   bool
		ConsoleJSON     bool
		ConsoleLevel    string
		EnableFile      bool
		FileJSON        bool
		FileLevel       string
		FileLocation    string
		AdvancedLogging mlog.LogTargetCfg
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

func (s *Server) GetAwsConfig() *aws.Config {
	var creds *credentials.Credentials = nil
	if s.Config.AWSCredentials.ID != "" {
		creds = credentials.NewStaticCredentials(
			s.Config.AWSCredentials.ID,
			s.Config.AWSCredentials.Secret,
			s.Config.AWSCredentials.Token,
		)
	}

	return &aws.Config{
		Credentials: creds,
		Region:      &s.Config.AWSRegion,
	}
}
