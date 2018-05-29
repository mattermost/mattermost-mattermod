// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
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
}

type JenkinsCredentials struct {
	URL      string
	Username string
	ApiToken string
}

type PRServerConfig struct {
	ListenAddress     string
	GithubAccessToken string
	Username          string

	TickRate               int
	SpinmintExpirationHour int

	DriverName string
	DataSource string

	Repositories []*Repository

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

	MattermostWebhookURL    string
	MattermostWebhookFooter string
}

var Config *PRServerConfig = &PRServerConfig{}

func FindConfigFile(fileName string) string {
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

func LoadConfig(fileName string) {
	fileName = FindConfigFile(fileName)
	LogInfo("Loading " + fileName)

	file, err := os.Open(fileName)
	if err != nil {
		LogCritical("Error opening config file=" + fileName + ", err=" + err.Error())
	}

	decoder := json.NewDecoder(file)
	err = decoder.Decode(Config)
	if err != nil {
		LogCritical("Error decoding config file=" + fileName + ", err=" + err.Error())
	}
}

func (config *PRServerConfig) GetRepository(owner, name string) (*Repository, bool) {
	for _, repo := range config.Repositories {
		if repo.Owner == owner && repo.Name == name {
			return repo, true
		}
	}

	return nil, false
}

func (config *PRServerConfig) GetAwsConfig() *aws.Config {
	var creds *credentials.Credentials = nil
	if config.AWSCredentials.Id != "" {
		creds = credentials.NewStaticCredentials(
			config.AWSCredentials.Id,
			config.AWSCredentials.Secret,
			config.AWSCredentials.Token,
		)
	}

	return &aws.Config{
		Credentials: creds,
		Region:      &config.AWSRegion,
	}
}
