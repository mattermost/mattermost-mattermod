// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v5/mlog"
)

func (s *Server) waitForBuildAndSetupSpinmint(pr *model.PullRequest, upgradeServer bool) {
	// This needs its own context because is executing a heavy job
	ctx, cancel := context.WithTimeout(context.Background(), defaultBuildMobileTimeout*time.Second)
	defer cancel()
	repo, client, err := s.Builds.buildJenkinsClient(s, pr)
	if err != nil {
		mlog.Error("Error building Jenkins client", mlog.Err(err))
		s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, s.Config.SetupSpinmintFailedMessage)
		return
	}

	mlog.Info("Waiting for Jenkins to build to set up spinmint for PR", mlog.Int("pr", pr.Number), mlog.String("repo_owner", pr.RepoOwner), mlog.String("repo_name", pr.RepoName))

	pr, err = s.Builds.waitForBuild(ctx, s, client, pr)
	if err != nil {
		mlog.Error("Error waiting for PR build to finish", mlog.Err(err))
		s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, s.Config.SetupSpinmintFailedMessage)
		return
	}

	var instance *ec2.Instance
	spinmint, appErr := s.Store.Spinmint().Get(pr.Number, pr.RepoName)
	if appErr != nil {
		mlog.Error("Unable to get the spinmint information. Will not build the spinmint", mlog.String("pr_error", appErr.Error()))
		return
	}

	if spinmint == nil {
		mlog.Error("No spinmint for this PR in the Database. will start a fresh one.")
		var errInstance error
		instance, errInstance = s.setupSpinmint(ctx, pr, repo, upgradeServer)
		if errInstance != nil {
			s.logToMattermost("Unable to set up spinmint for PR %v in %v/%v: %v", pr.Number, pr.RepoOwner, pr.RepoName, errInstance.Error())
			s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, s.Config.SetupSpinmintFailedMessage)
			return
		}
		spinmint = &model.Spinmint{
			InstanceID: *instance.InstanceId,
			RepoOwner:  pr.RepoOwner,
			RepoName:   pr.RepoName,
			Number:     pr.Number,
			CreatedAt:  time.Now().UTC().Unix(),
		}
		s.storeSpinmintInfo(spinmint)
	} else {
		instance = &ec2.Instance{
			InstanceId: aws.String(spinmint.InstanceID),
		}
	}

	mlog.Info("Waiting for instance to come up.")
	time.Sleep(time.Minute * 2)
	publicDNS, internalIP := s.getIPsForInstance(ctx, *instance.InstanceId)

	if err := s.updateRoute53Subdomain(ctx, *instance.InstanceId, publicDNS, "CREATE"); err != nil {
		s.logToMattermost("Unable to set up S3 subdomain for PR %v in %v/%v with instance %v: %v", pr.Number, pr.RepoOwner, pr.RepoName, *instance.InstanceId, err.Error())
		s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, s.Config.SetupSpinmintFailedMessage)
		return
	}

	smLink := fmt.Sprintf("%v.%v", *instance.InstanceId, s.Config.AWSDnsSuffix)
	if s.Config.SpinmintsUseHTTPS {
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

	message = strings.Replace(message, templateSpinmintLink, smLink, 1)
	message = strings.Replace(message, templateInstanceID, instanceIDMessage+*instance.InstanceId, 1)
	message = strings.Replace(message, templateInternalIP, internalIP, 1)

	s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, message)
}

// Returns instance ID of instance created
func (s *Server) setupSpinmint(ctx context.Context, pr *model.PullRequest, repo *Repository, upgrade bool) (*ec2.Instance, error) {
	mlog.Info("Setting up spinmint for PR", mlog.Int("pr", pr.Number))

	svc := ec2.New(s.awsSession, s.GetAwsConfig())

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
	// with circleci if the PR is opened in upstream we don't have the PR number and we have the branch name instead.
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
		ImageId:          &s.Config.AWSImageID,
		MaxCount:         &one,
		MinCount:         &one,
		InstanceType:     &s.Config.AWSInstanceType,
		UserData:         &sdata,
		SecurityGroupIds: []*string{&s.Config.AWSSecurityGroup},
		SubnetId:         &s.Config.AWSSubNetID,
	}

	resp, err := svc.RunInstancesWithContext(ctx, params)
	if err != nil {
		return nil, err
	}

	// Add tags to the created instance
	time.Sleep(time.Second * 10)
	_, errtag := svc.CreateTagsWithContext(ctx, &ec2.CreateTagsInput{
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
	ctx, cancel := context.WithTimeout(context.Background(), defaultBuildSpinmintTimeout*time.Second)
	defer cancel()
	mlog.Info("Destroying spinmint for PR", mlog.String("instance", instanceID), mlog.Int("pr", pr.Number), mlog.String("repo_owner", pr.RepoOwner), mlog.String("repo_name", pr.RepoName))

	svc := ec2.New(s.awsSession, s.GetAwsConfig())

	params := &ec2.TerminateInstancesInput{
		InstanceIds: []*string{
			&instanceID,
		},
	}

	_, err := svc.TerminateInstancesWithContext(ctx, params)
	if err != nil {
		mlog.Error("Error terminating instances", mlog.Err(err))
		return
	}

	// Remove route53 entry
	err = s.updateRoute53Subdomain(ctx, instanceID, "", "DELETE")
	if err != nil {
		mlog.Error("Error removing the Route53 entry", mlog.Err(err))
		return
	}

	s.removeTestServerFromDB(instanceID)
}

func (s *Server) getIPsForInstance(ctx context.Context, instance string) (publicIP string, privateIP string) {
	svc := ec2.New(s.awsSession, s.GetAwsConfig())
	params := &ec2.DescribeInstancesInput{
		InstanceIds: []*string{
			&instance,
		},
	}
	resp, err := svc.DescribeInstancesWithContext(ctx, params)
	if err != nil {
		mlog.Error("Problem getting instance ip", mlog.Err(err))
		return "", ""
	}

	return *resp.Reservations[0].Instances[0].PublicIpAddress, *resp.Reservations[0].Instances[0].PrivateIpAddress
}

func (s *Server) updateRoute53Subdomain(ctx context.Context, name, target, action string) error {
	svc := route53.New(s.awsSession, s.GetAwsConfig())
	domainName := fmt.Sprintf("%v.%v", name, s.Config.AWSDnsSuffix)

	targetServer := target
	if target == "" && action == "DELETE" {
		targetServer, _ = s.getIPsForInstance(ctx, name)
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
		HostedZoneId: &s.Config.AWSHostedZoneID,
	}

	if _, err := svc.ChangeResourceRecordSetsWithContext(ctx, params); err != nil {
		return err
	}

	return nil
}

// CheckTestServerLifeTime checks the age of the test server and kills if reach the limit
func (s *Server) CheckTestServerLifeTime() {
	mlog.Info("Checking Test Server lifetime...")

	ctx, cancel := context.WithTimeout(context.Background(), defaultCronTaskTimeout*time.Second)
	defer cancel()
	testServers, err := s.Store.Spinmint().List()
	if err != nil {
		mlog.Error("Unable to get updated PR while waiting for test server", mlog.String("testServer_error", err.Error()))
		return
	}

	for _, testServer := range testServers {
		mlog.Info("Check if need destroy Test Server for PR", mlog.String("instance", testServer.InstanceID), mlog.Int("TestServer", testServer.Number), mlog.String("repo_owner", testServer.RepoOwner), mlog.String("repo_name", testServer.RepoName))
		testServerCreated := time.Unix(testServer.CreatedAt, 0)
		duration := time.Since(testServerCreated)
		if int(duration.Hours()) > s.Config.SpinmintExpirationHour {
			mlog.Info("Will destroy spinmint for PR", mlog.String("instance", testServer.InstanceID), mlog.Int("TestServer", testServer.Number), mlog.String("repo_owner", testServer.RepoOwner), mlog.String("repo_name", testServer.RepoName))
			pr := &model.PullRequest{
				RepoOwner: testServer.RepoOwner,
				RepoName:  testServer.RepoName,
				Number:    testServer.Number,
			}
			go s.destroySpinmint(pr, testServer.InstanceID)
			s.removeTestServerFromDB(testServer.InstanceID)
			s.sendGitHubComment(ctx, testServer.RepoOwner, testServer.RepoName, testServer.Number, s.Config.DestroyedExpirationSpinmintMessage)
		}
	}

	mlog.Info("Done checking Test Server lifetime.")
}

func (s *Server) storeSpinmintInfo(spinmint *model.Spinmint) {
	if _, err := s.Store.Spinmint().Save(spinmint); err != nil {
		mlog.Error(err.Error())
	}
}

func (s *Server) removeTestServerFromDB(instanceID string) {
	if err := s.Store.Spinmint().Delete(instanceID); err != nil {
		mlog.Error(err.Error())
	}
}

func (s *Server) isSpinMintLabel(label string) bool {
	return label == s.Config.SetupSpinmintTag || label == s.Config.SetupSpinmintUpgradeTag
}
