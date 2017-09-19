// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/mattermost/mattermost-mattermod/model"
	jenkins "github.com/yosida95/golang-jenkins"
)

func destroySpinmint(pr *model.PullRequest, instanceId string) {
	LogInfo("Destroying spinmint %v for PR %v in %v/%v", instanceId, pr.Number, pr.RepoOwner, pr.RepoName)

	svc := ec2.New(session.New(), Config.GetAwsConfig())

	params := &ec2.TerminateInstancesInput{
		InstanceIds: []*string{
			&instanceId,
		},
	}

	_, err := svc.TerminateInstances(params)
	if err != nil {
		LogError("Error terminating instances: " + err.Error())
		return
	}
}

func waitForBuildAndSetupSpinmint(pr *model.PullRequest) {
	repo, ok := Config.GetRepository(pr.RepoOwner, pr.RepoName)
	if !ok || repo.JenkinsServer == "" {
		LogError("Unable to set up spintmint for PR %v in %v/%v without Jenkins configured for server", pr.Number, pr.RepoOwner, pr.RepoName)
		commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, Config.SetupSpinmintFailedMessage)
		return
	}

	credentials, ok := Config.JenkinsCredentials[repo.JenkinsServer]
	if !ok {
		LogError("No Jenkins credentials for server %v required for PR %v in %v/%v", repo.JenkinsServer, pr.Number, pr.RepoOwner, pr.RepoName)
		commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, Config.SetupSpinmintFailedMessage)
		return
	}

	client := jenkins.NewJenkins(&jenkins.Auth{
		Username: credentials.Username,
		ApiToken: credentials.ApiToken,
	}, credentials.URL)

	LogInfo("Waiting for Jenkins to build to set up spinmint for PR %v in %v/%v", pr.Number, pr.RepoOwner, pr.RepoName)

	for {
		if result := <-Srv.Store.PullRequest().Get(pr.RepoOwner, pr.RepoName, pr.Number); result.Err != nil {
			LogError("Unable to get updated PR while waiting for spinmint: %v", result.Err.Error())
		} else {
			// Update the PR in case the build link has changed because of a new commit
			pr = result.Data.(*model.PullRequest)
		}

		if pr.BuildLink != "" {
			parts := strings.Split(pr.BuildLink, "/")
			jobNumber, _ := strconv.ParseInt(parts[len(parts)-2], 10, 32)
			jobName := parts[len(parts)-3]

			job, err := client.GetJob(jobName)
			if err != nil {
				LogError("Failed to get Jenkins job %v: %v", jobName, err)
				return
			}

			build, err := client.GetBuild(job, int(jobNumber))
			if err != nil {
				LogError("Failed to get build %v for PR %v in %v/%v: %v", jobNumber, pr.Number, pr.RepoOwner, pr.RepoName, err)
				return
			}

			if !build.Building && build.Result == "SUCCESS" {
				LogInfo("build %v for PR %v in %v/%v succeeded!", jobNumber, pr.Number, pr.RepoOwner, pr.RepoName)
				break
			} else {
				LogInfo("build %v has status %v %v", jobNumber, build.Result, build.Building)
			}
		} else {
			LogError("Unable to find build link for PR %v", pr.Number)
		}

		time.Sleep(10 * time.Second)
	}

	instance, err := setupSpinmint(pr.Number)
	if err != nil {
		LogError("Unable to set up spinmint for PR %v in %v/%v: %v", pr.Number, pr.RepoOwner, pr.RepoName, err.Error())
		commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, Config.SetupSpinmintFailedMessage)
		return
	}

	LogInfo("Waiting for instance to come up.")
	time.Sleep(time.Minute * 2)
	publicdns := getPublicDnsName(*instance.InstanceId)

	err2 := createRoute53Subdomain(*instance.InstanceId, publicdns)
	if err2 != nil {
		LogError("Unable to set up S3 subdomain for PR %v in %v/%v with instance %v: %v", pr.Number, pr.RepoOwner, pr.RepoName, *instance.InstanceId, err2.Error())
		commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, Config.SetupSpinmintFailedMessage)
		return
	}
	smLink := "http://" + *instance.InstanceId + ".spinmint.com:8065" + "/pr" + strconv.Itoa(pr.Number)
	commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, strings.Replace(Config.SetupSpinmintDoneMessage+INSTANCE_ID_MESSAGE+*instance.InstanceId, SPINMINT_LINK, smLink, 1))
}

// Returns instance ID of instance created
func setupSpinmint(prNumber int) (*ec2.Instance, error) {
	LogInfo("Setting up spinmint for PR: " + strconv.Itoa(prNumber))

	svc := ec2.New(session.New(), Config.GetAwsConfig())

	data, err := ioutil.ReadFile("config/instance-setup.sh")
	if err != nil {
		return nil, err
	}
	sdata := string(data)
	sdata = strings.Replace(sdata, "BUILD_NUMBER", strconv.Itoa(prNumber), -1)
	bsdata := []byte(sdata)
	sdata = base64.StdEncoding.EncodeToString(bsdata)

	var one int64 = 1
	params := &ec2.RunInstancesInput{
		ImageId:          &Config.AWSImageId,
		MaxCount:         &one,
		MinCount:         &one,
		KeyName:          &Config.AWSKeyName,
		InstanceType:     &Config.AWSInstanceType,
		UserData:         &sdata,
		SecurityGroupIds: []*string{&Config.AWSSecurityGroup},
	}

	resp, err := svc.RunInstances(params)
	if err != nil {
		LogError("Error creating instances: " + err.Error())
		return nil, err
	}

	return resp.Instances[0], nil
}

func getPublicDnsName(instance string) string {
	svc := ec2.New(session.New(), Config.GetAwsConfig())
	params := &ec2.DescribeInstancesInput{
		InstanceIds: []*string{
			&instance,
		},
	}
	resp, err := svc.DescribeInstances(params)
	if err != nil {
		LogError("Problem getting instance ip: " + err.Error())
		return ""
	}

	return *resp.Reservations[0].Instances[0].PublicDnsName
}

func createRoute53Subdomain(name string, target string) error {
	svc := route53.New(session.New(), Config.GetAwsConfig())

	create := "CREATE"
	var threehundred int64 = 300
	cname := "CNAME"
	domainName := fmt.Sprintf("%v.%v", name, "spinmint.com")
	params := &route53.ChangeResourceRecordSetsInput{
		ChangeBatch: &route53.ChangeBatch{
			Changes: []*route53.Change{
				{
					Action: &create,
					ResourceRecordSet: &route53.ResourceRecordSet{
						Name: &domainName,
						TTL:  &threehundred,
						Type: &cname,
						ResourceRecords: []*route53.ResourceRecord{
							{
								Value: &target,
							},
						},
					},
				},
			},
		},
		HostedZoneId: &Config.AWSHostedZoneId,
	}

	_, err := svc.ChangeResourceRecordSets(params)
	if err != nil {
		LogError("Error creating instances: " + err.Error())
		return err
	}

	return nil
}