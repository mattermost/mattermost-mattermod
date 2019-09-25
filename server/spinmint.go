// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/mlog"
	// "github.com/mattermost/mattermost-load-test/ltops"
	// "github.com/mattermost/mattermost-load-test/ltparse"
	// "github.com/mattermost/mattermost-load-test/terraform"
)

// TODO FIXME
// func waitForBuildAndSetupLoadtest(pr *model.PullRequest) {
// 	repo, ok := s.Config.GetRepository(pr.RepoOwner, pr.RepoName)
// 	if !ok || repo.JenkinsServer == "" {
// 		LogError("Unable to set up loadtest for PR %v in %v/%v without Jenkins configured for server", pr.Number, pr.RepoOwner, pr.RepoName)
// 		s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number, "Failed to setup loadtest")
// 		return
// 	}

// 	credentials, ok := s.Config.JenkinsCredentials[repo.JenkinsServer]
// 	if !ok {
// 		LogError("No Jenkins credentials for server %v required for PR %v in %v/%v", repo.JenkinsServer, pr.Number, pr.RepoOwner, pr.RepoName)
// 		s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number, "Failed to setup loadtest")
// 		return
// 	}

// 	client := jenkins.NewJenkins(&jenkins.Auth{
// 		Username: credentials.Username,
// 		ApiToken: credentials.ApiToken,
// 	}, credentials.URL)

// 	LogInfo("Waiting for Jenkins to build to set up loadtest for PR %v in %v/%v", pr.Number, pr.RepoOwner, pr.RepoName)

// 	pr = waitForBuild(client, pr)

// 	config := &ltops.ClusterConfig{
// 		Name:                  fmt.Sprintf("pr-%v", pr.Number),
// 		AppInstanceType:       "m4.xlarge",
// 		AppInstanceCount:      4,
// 		DBInstanceType:        "db.r4.xlarge",
// 		DBInstanceCount:       4,
// 		LoadtestInstanceCount: 1,
// 	}
// 	s.Config.WorkingDirectory = filepath.Join("./clusters/", s.Config.Name)

// 	LogInfo("Creating terraform cluster for loadtest for PR %v in %v/%v", pr.Number, pr.RepoOwner, pr.RepoName)
// 	cluster, err := terraform.CreateCluster(config)
// 	if err != nil {
// 		LogError("Unable to setup cluster: " + err.Error())
// 		s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number, "Failed to setup loadtest")
// 	}
// 	// Wait for the cluster to init
// 	time.Sleep(time.Minute)

// 	results := bytes.NewBuffer(nil)

// 	LogInfo("Deploying to cluster for loadtest for PR %v in %v/%v", pr.Number, pr.RepoOwner, pr.RepoName)
// 	if err := cluster.Deploy("https://releases.mattermost.com/mattermost-platform-pr/"+strconv.Itoa(pr.Number)+"/mattermost-enterprise-linux-amd64.tar.gz", "mattermod.mattermost-license"); err != nil {
// 		LogError("Unable to deploy cluster: " + err.Error())
// 		s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number, "Failed to setup loadtest")
// 		return
// 	}
// 	if err := cluster.Loadtest("https://releases.mattermost.com/mattermost-load-test/mattermost-load-test.tar.gz"); err != nil {
// 		LogError("Unable to deploy loadtests to cluster: " + err.Error())
// 		s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number, "Failed to setup loadtest")
// 		return
// 	}

// 	// Wait for the cluster restart after deploy
// 	time.Sleep(time.Minute)

// 	LogInfo("Runing loadtest for PR %v in %v/%v", pr.Number, pr.RepoOwner, pr.RepoName)
// 	if err := cluster.Loadtest(results); err != nil {
// 		LogError("Unable to loadtest cluster: " + err.Error())
// 		s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number, "Failed to setup loadtest")
// 		return
// 	}
// 	LogInfo("Destroying cluster for PR %v in %v/%v", pr.Number, pr.RepoOwner, pr.RepoName)
// 	if err := cluster.Destroy(); err != nil {
// 		LogError("Unable to destroy cluster: " + err.Error())
// 		s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number, "Failed to setup loadtest")
// 		return
// 	}

// 	githubOutput := bytes.NewBuffer(nil)
// 	cfg := ltparse.ResultsConfig{
// 		Input:     results,
// 		Output:    githubOutput,
// 		Display:   "markdown",
// 		Aggregate: false,
// 	}

// 	ltparse.ParseResults(&cfg)
// 	LogInfo("Loadtest results for PR %v in %v/%v\n%v", pr.Number, pr.RepoOwner, pr.RepoName, githubOutput.String())
// 	s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number, githubOutput.String())
// }
func waitForBuildAndSetupLoadtest(pr *model.PullRequest) {
	return
}

func (s *Server) waitForBuildAndSetupSpinmint(pr *model.PullRequest, upgradeServer bool) {
	repo, client, err := s.Builds.buildJenkinsClient(s, pr)
	if err != nil {
		mlog.Error("Error building Jenkins client", mlog.Err(err))
		s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number, s.Config.SetupSpinmintFailedMessage)
		return
	}

	mlog.Info("Waiting for Jenkins to build to set up spinmint for PR", mlog.Int("pr", pr.Number), mlog.String("repo_owner", pr.RepoOwner), mlog.String("repo_name", pr.RepoName))

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	defer cancel()

	pr, err = s.Builds.waitForBuild(ctx, s, client, pr)
	if err != nil {
		mlog.Error("Error waiting for PR build to finish", mlog.Err(err))
		s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number, s.Config.SetupSpinmintFailedMessage)
		return
	}

	var instance *ec2.Instance
	if result := <-s.Store.Spinmint().Get(pr.Number, pr.RepoName); result.Err != nil {
		mlog.Error("Unable to get the spinmint information. Will not build the spinmint", mlog.String("pr_error", result.Err.Error()))
	} else if result.Data == nil {
		mlog.Error("No spinmint for this PR in the Database. will start a fresh one.")
		var errInstance error
		instance, errInstance = s.setupSpinmint(pr, repo, upgradeServer)
		if errInstance != nil {
			s.logErrorToMattermost("Unable to set up spinmint for PR %v in %v/%v: %v", pr.Number, pr.RepoOwner, pr.RepoName, errInstance.Error())
			s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number, s.Config.SetupSpinmintFailedMessage)
			return
		}
		spinmint := &model.Spinmint{
			InstanceId: *instance.InstanceId,
			RepoOwner:  pr.RepoOwner,
			RepoName:   pr.RepoName,
			Number:     pr.Number,
			CreatedAt:  time.Now().UTC().Unix(),
		}
		s.storeSpinmintInfo(spinmint)
	} else {
		spinmint := result.Data.(*model.Spinmint)
		instance.InstanceId = aws.String(spinmint.InstanceId)
	}

	mlog.Info("Waiting for instance to come up.")
	time.Sleep(time.Minute * 2)
	publicDNS, internalIP := s.getIPsForInstance(*instance.InstanceId)

	if err := s.updateRoute53Subdomain(*instance.InstanceId, publicDNS, "CREATE"); err != nil {
		s.logErrorToMattermost("Unable to set up S3 subdomain for PR %v in %v/%v with instance %v: %v", pr.Number, pr.RepoOwner, pr.RepoName, *instance.InstanceId, err.Error())
		s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number, s.Config.SetupSpinmintFailedMessage)
		return
	}

	smLink := fmt.Sprintf("%v.%v", *instance.InstanceId, s.Config.AWSDnsSuffix)
	if s.Config.SpinmintsUseHttps {
		smLink = "https://" + smLink
	} else {
		smLink = "http://" + smLink
	}

	var message string
	if upgradeServer {
		message = s.Config.SetupSpinmintUpgradeDoneMessage
	} else {
		message = s.Config.SetupSpinmintDoneMessage
	}

	message = strings.Replace(message, SPINMINT_LINK, smLink, 1)
	message = strings.Replace(message, INSTANCE_ID, INSTANCE_ID_MESSAGE+*instance.InstanceId, 1)
	message = strings.Replace(message, INTERNAL_IP, internalIP, 1)

	s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number, message)
}

func (s *Server) waitForMobileAppsBuild(pr *model.PullRequest) {
	repo, client, err := s.Builds.buildJenkinsClient(s, pr)
	if err != nil {
		mlog.Error("Error building Jenkins client", mlog.Err(err))
		s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number, s.Config.SetupSpinmintFailedMessage)
		return
	}

	mlog.Info("Waiting for Jenkins to build to start build the mobile app for PR", mlog.Int("pr", pr.Number), mlog.String("repo_owner", pr.RepoOwner), mlog.String("repo_name", pr.RepoName))

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	defer cancel()

	//Job that will build the apps for a PR
	jobName := fmt.Sprintf("mm/job/%s", repo.JobName)
	job, err := client.GetJob(jobName)
	if err != nil {
		mlog.Error("Failed to get Jenkins job", mlog.String("job", jobName), mlog.Err(err))
		return
	}

	mlog.Info("Will start the job", mlog.String("job", jobName))
	parameters := url.Values{}
	parameters.Add("PR_NUMBER", strconv.Itoa(pr.Number))
	err = client.Build(jobName, parameters)
	if err != nil {
		mlog.Error("Failed to build Jenkins job", mlog.String("job", jobName), mlog.Err(err))
		return
	}

	job.Name = jobName
	for {
		build, err := client.GetLastBuild(job)
		if err != nil {
			mlog.Error("Failed to get the build Jenkins job", mlog.String("job", jobName), mlog.Err(err))
			return
		}
		if !build.Building && build.Result == "SUCCESS" {
			mlog.Info("build mobile app for PR succeeded!", mlog.Int("build", build.Number), mlog.Int("pr", pr.Number), mlog.String("repo_owner", pr.RepoOwner), mlog.String("repo_name", pr.RepoName))
			break
		} else if build.Result == "FAILURE" {
			mlog.Error("build has status FAILURE aborting.", mlog.Int("build", build.Number), mlog.String("result", build.Result))
			s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number, s.Config.BuildMobileAppFailedMessage)
			return
		} else {
			mlog.Info("build is running", mlog.Int("build", build.Number), mlog.Bool("building", build.Building))
		}
		time.Sleep(60 * time.Second)
	}

	prNumberStr := fmt.Sprintf("PR-%d", pr.Number)
	msgMobile := s.Config.BuildMobileAppDoneMessage
	msgMobile = strings.Replace(msgMobile, "PR_NUMBER", prNumberStr, 2)
	msgMobile = strings.Replace(msgMobile, "ANDROID_APP", prNumberStr, 1)
	msgMobile = strings.Replace(msgMobile, "IOS_APP", prNumberStr, 1)
	s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number, msgMobile)
	return
}

// Returns instance ID of instance created
func (s *Server) setupSpinmint(pr *model.PullRequest, repo *Repository, upgrade bool) (*ec2.Instance, error) {
	mlog.Info("Setting up spinmint for PR", mlog.Int("pr", pr.Number))

	svc := ec2.New(session.New(), s.GetAwsConfig())

	var setupScript string
	if upgrade {
		setupScript = repo.InstanceSetupUpgradeScript
	} else {
		setupScript = repo.InstanceSetupScript
	}

	data, err := ioutil.ReadFile(path.Join("config", setupScript))
	if err != nil {
		return nil, err
	}
	sdata := string(data)
	// with circleci if the PR is opened in upstream we dont have the PR number and we have the branch name instead.
	// so we will use the commit hash that we upload too
	partialURL := fmt.Sprintf("commit/%s", pr.Sha)
	sdata = strings.Replace(sdata, "PR-BUILD_NUMBER", partialURL, -1)
	// for server
	sdata = strings.Replace(sdata, "BUILD_NUMBER", strconv.Itoa(pr.Number), -1)
	sdata = strings.Replace(sdata, "BRANCH_NAME", pr.Ref, -1)
	mlog.Debug("Script to bootstrap the server", mlog.String("Script", sdata))
	bsdata := []byte(sdata)
	sdata = base64.StdEncoding.EncodeToString(bsdata)

	var one int64 = 1
	params := &ec2.RunInstancesInput{
		ImageId:          &s.Config.AWSImageId,
		MaxCount:         &one,
		MinCount:         &one,
		InstanceType:     &s.Config.AWSInstanceType,
		UserData:         &sdata,
		SecurityGroupIds: []*string{&s.Config.AWSSecurityGroup},
		SubnetId:         &s.Config.AWSSubNetId,
	}

	resp, err := svc.RunInstances(params)
	if err != nil {
		return nil, err
	}

	// Add tags to the created instance
	time.Sleep(time.Second * 10)
	_, errtag := svc.CreateTags(&ec2.CreateTagsInput{
		Resources: []*string{resp.Instances[0].InstanceId},
		Tags: []*ec2.Tag{
			{
				Key:   aws.String("Name"),
				Value: aws.String("Spinmint-" + pr.Ref),
			},
			{
				Key:   aws.String("Created"),
				Value: aws.String(time.Now().Format("2006-01-02/15:04:05")),
			},
			{
				Key:   aws.String("PRNumber"),
				Value: aws.String("PR-" + strconv.Itoa(pr.Number)),
			},
			{
				Key:   aws.String("RepoName"),
				Value: aws.String(pr.RepoName),
			},
		},
	})
	if errtag != nil {
		mlog.Error("Could not create tags for instance", mlog.String("instance", *resp.Instances[0].InstanceId), mlog.String("tag_error", errtag.Error()))
	}

	return resp.Instances[0], nil
}

func (s *Server) destroySpinmint(pr *model.PullRequest, instanceID string) {
	mlog.Info("Destroying spinmint for PR", mlog.String("instance", instanceID), mlog.Int("pr", pr.Number), mlog.String("repo_owner", pr.RepoOwner), mlog.String("repo_name", pr.RepoName))

	svc := ec2.New(session.New(), s.GetAwsConfig())

	params := &ec2.TerminateInstancesInput{
		InstanceIds: []*string{
			&instanceID,
		},
	}

	_, err := svc.TerminateInstances(params)
	if err != nil {
		mlog.Error("Error terminating instances", mlog.Err(err))
		return
	}

	// Remove route53 entry
	err = s.updateRoute53Subdomain(instanceID, "", "DELETE")
	if err != nil {
		mlog.Error("Error removing the Route53 entry", mlog.Err(err))
		return
	}

	s.removeTestServerFromDB(instanceID)
}

func (s *Server) getIPsForInstance(instance string) (publicIP string, privateIP string) {
	svc := ec2.New(session.New(), s.GetAwsConfig())
	params := &ec2.DescribeInstancesInput{
		InstanceIds: []*string{
			&instance,
		},
	}
	resp, err := svc.DescribeInstances(params)
	if err != nil {
		mlog.Error("Problem getting instance ip", mlog.Err(err))
		return "", ""
	}

	return *resp.Reservations[0].Instances[0].PublicIpAddress, *resp.Reservations[0].Instances[0].PrivateIpAddress
}

func (s *Server) updateRoute53Subdomain(name, target, action string) error {
	svc := route53.New(session.New(), s.GetAwsConfig())
	domainName := fmt.Sprintf("%v.%v", name, s.Config.AWSDnsSuffix)

	targetServer := target
	if target == "" && action == "DELETE" {
		targetServer, _ = s.getIPsForInstance(name)
	}

	params := &route53.ChangeResourceRecordSetsInput{
		ChangeBatch: &route53.ChangeBatch{
			Changes: []*route53.Change{
				{
					Action: aws.String(action),
					ResourceRecordSet: &route53.ResourceRecordSet{
						Name: aws.String(domainName),
						TTL:  aws.Int64(30),
						Type: aws.String("A"),
						ResourceRecords: []*route53.ResourceRecord{
							{
								Value: aws.String(targetServer),
							},
						},
					},
				},
			},
		},
		HostedZoneId: &s.Config.AWSHostedZoneId,
	}

	_, err := svc.ChangeResourceRecordSets(params)
	if err != nil {
		return err
	}

	return nil
}

// CheckTestServerLifeTime checks the age of the test server and kills if reach the limit
func (s *Server) CheckTestServerLifeTime() {
	mlog.Info("Checking Test Server lifetime...")
	testServers := []*model.Spinmint{}
	if result := <-s.Store.Spinmint().List(); result.Err != nil {
		mlog.Error("Unable to get updated PR while waiting for test server", mlog.String("testServer_error", result.Err.Error()))
	} else {
		testServers = result.Data.([]*model.Spinmint)
	}

	for _, testServer := range testServers {
		mlog.Info("Check if need destroy Test Server for PR", mlog.String("instance", testServer.InstanceId), mlog.Int("TestServer", testServer.Number), mlog.String("repo_owner", testServer.RepoOwner), mlog.String("repo_name", testServer.RepoName))
		testServerCreated := time.Unix(testServer.CreatedAt, 0)
		duration := time.Since(testServerCreated)
		if int(duration.Hours()) > s.Config.SpinmintExpirationHour {
			mlog.Info("Will destroy spinmint for PR", mlog.String("instance", testServer.InstanceId), mlog.Int("TestServer", testServer.Number), mlog.String("repo_owner", testServer.RepoOwner), mlog.String("repo_name", testServer.RepoName))
			pr := &model.PullRequest{
				RepoOwner: testServer.RepoOwner,
				RepoName:  testServer.RepoName,
				Number:    testServer.Number,
			}
			go s.destroySpinmint(pr, testServer.InstanceId)
			s.removeTestServerFromDB(testServer.InstanceId)
			s.sendGitHubComment(testServer.RepoOwner, testServer.RepoName, testServer.Number, s.Config.DestroyedExpirationSpinmintMessage)
		}
	}

	mlog.Info("Done checking Test Server lifetime.")
}

func (s *Server) storeSpinmintInfo(spinmint *model.Spinmint) {
	if result := <-s.Store.Spinmint().Save(spinmint); result.Err != nil {
		mlog.Error(result.Err.Error())
	}
}

func (s *Server) removeTestServerFromDB(instanceId string) {
	if result := <-s.Store.Spinmint().Delete(instanceId); result.Err != nil {
		mlog.Error(result.Err.Error())
	}
}

func (s *Server) isSpinMintLabel(label string) bool {
	return label == s.Config.SetupSpinmintTag || label == s.Config.SetupSpinmintUpgradeTag
}
