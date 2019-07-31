// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/mattermost/mattermost-server/mlog"
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

type ServerConfig struct {
	ListenAddress               string
	GithubAccessToken           string
	GitHubTokenReserve          int
	GithubUsername              string
	GithubAccessTokenCherryPick string
	Username                    string

	TickRateMinutes        int
	SpinmintExpirationHour int

	DriverName string
	DataSource string

	Repositories []*Repository

	SetupSpinWick            string
	SetupSpinWickHA          string
	ProvisionerServer        string
	DNSNameTestServer        string
	AWSEmailAccessKey        string
	AWSEmailSecretKey        string
	AWSEmailEndpoint         string
	TokenToDeleteTestServers string

	SetupSpinmintTag                   string
	SetupSpinmintMessage               string
	SetupSpinmintDoneMessage           string
	SetupSpinmintFailedMessage         string
	DestroyedSpinmintMessage           string
	DestroyedExpirationSpinmintMessage string
	SpinmintsUseHttps                  bool

	SetupSpinmintUpgradeTag         string
	SetupSpinmintUpgradeMessage     string
	SetupSpinmintUpgradeDoneMessage string

	BuildMobileAppTag           string
	BuildMobileAppInitMessage   string
	BuildMobileAppDoneMessage   string
	BuildMobileAppFailedMessage string

	StartLoadtestTag     string
	StartLoadtestMessage string

	SignedCLAURL          string
	NeedsToSignCLAMessage string

	PrLabels    []LabelResponse
	IssueLabels []LabelResponse

	JenkinsCredentials map[string]*JenkinsCredentials

	AWSCredentials struct {
		Id     string
		Secret string
		Token  string
	}

	AWSRegion        string
	AWSImageId       string
	AWSKeyName       string
	AWSInstanceType  string
	AWSHostedZoneId  string
	AWSSecurityGroup string
	AWSDnsSuffix     string
	AWSSubNetId      string

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

func getConfig(fileName string) (*ServerConfig, error) {
	config := &ServerConfig{}
	fileName = findConfigFile(fileName)
	mlog.Info("Loading config", mlog.String("filename", fileName))

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

func (s *Server) GetRepository(owner, name string) (*Repository, bool) {
	for _, repo := range s.Config.Repositories {
		if repo.Owner == owner && repo.Name == name {
			return repo, true
		}
	}

	return nil, false
}

func (s *Server) GetAwsConfig() *aws.Config {
	var creds *credentials.Credentials = nil
	if s.Config.AWSCredentials.Id != "" {
		creds = credentials.NewStaticCredentials(
			s.Config.AWSCredentials.Id,
			s.Config.AWSCredentials.Secret,
			s.Config.AWSCredentials.Token,
		)
	}

	return &aws.Config{
		Credentials: creds,
		Region:      &s.Config.AWSRegion,
	}
}
