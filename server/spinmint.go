// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/mattermost/mattermost-mattermod/model"
	jenkins "github.com/yosida95/golang-jenkins"

	ltops "github.com/mattermost/mattermost-load-test-ops"
	"github.com/mattermost/mattermost-load-test-ops/terraform"
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

	// Remove route53 entry
	err = updateRoute53Subdomain(instanceId, "", "DELETE")
	if err != nil {
		LogError("Error removing the Route53 entry: " + err.Error())
		return
	}

}

func waitForBuildAndSetupLoadtest(pr *model.PullRequest) {
	repo, ok := Config.GetRepository(pr.RepoOwner, pr.RepoName)
	if !ok || repo.JenkinsServer == "" {
		LogError("Unable to set up loadtest for PR %v in %v/%v without Jenkins configured for server", pr.Number, pr.RepoOwner, pr.RepoName)
		commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, "Failed to setup loadtest")
		return
	}

	credentials, ok := Config.JenkinsCredentials[repo.JenkinsServer]
	if !ok {
		LogError("No Jenkins credentials for server %v required for PR %v in %v/%v", repo.JenkinsServer, pr.Number, pr.RepoOwner, pr.RepoName)
		commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, "Failed to setup loadtest")
		return
	}

	client := jenkins.NewJenkins(&jenkins.Auth{
		Username: credentials.Username,
		ApiToken: credentials.ApiToken,
	}, credentials.URL)

	LogInfo("Waiting for Jenkins to build to set up loadtest for PR %v in %v/%v", pr.Number, pr.RepoOwner, pr.RepoName)

	pr = waitForBuild(client, pr)

	config := &ltops.ClusterConfig{
		Name:                  fmt.Sprintf("pr-%v", pr.Number),
		AppInstanceType:       "m4.xlarge",
		AppInstanceCount:      4,
		DBInstanceType:        "db.r4.xlarge",
		DBInstanceCount:       4,
		LoadtestInstanceCount: 1,
	}
	config.WorkingDirectory = filepath.Join("./clusters/", config.Name)

	LogInfo("Creating terraform cluster for loadtest for PR %v in %v/%v", pr.Number, pr.RepoOwner, pr.RepoName)
	cluster, err := terraform.CreateCluster(config)
	if err != nil {
		LogError("Unable to setup cluster: " + err.Error())
		commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, "Failed to setup loadtest")
	}
	// Wait for the cluster to init
	time.Sleep(time.Minute)

	results := bytes.NewBuffer(nil)

	LogInfo("Deploying to cluster for loadtest for PR %v in %v/%v", pr.Number, pr.RepoOwner, pr.RepoName)
	if err := cluster.DeployMattermost("https://releases.mattermost.com/mattermost-platform-pr/"+strconv.Itoa(pr.Number)+"/mattermost-enterprise-linux-amd64.tar.gz", "mattermod.mattermost-license"); err != nil {
		LogError("Unable to deploy cluster: " + err.Error())
		commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, "Failed to setup loadtest")
		return
	}
	if err := cluster.DeployLoadtests("https://releases.mattermost.com/mattermost-load-test/mattermost-load-test.tar.gz"); err != nil {
		LogError("Unable to deploy loadtests to cluster: " + err.Error())
		commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, "Failed to setup loadtest")
		return
	}

	// Wait for the cluster restart after deploy
	time.Sleep(time.Minute)

	LogInfo("Runing loadtest for PR %v in %v/%v", pr.Number, pr.RepoOwner, pr.RepoName)
	if err := cluster.Loadtest(results); err != nil {
		LogError("Unable to loadtest cluster: " + err.Error())
		commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, "Failed to setup loadtest")
		return
	}
	LogInfo("Destroying cluster for PR %v in %v/%v", pr.Number, pr.RepoOwner, pr.RepoName)
	if err := cluster.Destroy(); err != nil {
		LogError("Unable to destroy cluster: " + err.Error())
		commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, "Failed to setup loadtest")
		return
	}

	commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, results.String())
}

func waitForBuildAndSetupSpinmint(pr *model.PullRequest, upgradeServer bool) {
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

	pr = waitForBuild(client, pr)

	instance, err := setupSpinmint(pr.Number, pr.Ref, repo, upgradeServer)
	if err != nil {
		LogErrorToMattermost("Unable to set up spinmint for PR %v in %v/%v: %v", pr.Number, pr.RepoOwner, pr.RepoName, err.Error())
		commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, Config.SetupSpinmintFailedMessage)
		return
	}

	LogInfo("Waiting for instance to come up.")
	time.Sleep(time.Minute * 2)
	publicdns := getPublicDnsName(*instance.InstanceId)

	if err := updateRoute53Subdomain(*instance.InstanceId, publicdns, "CREATE"); err != nil {
		LogErrorToMattermost("Unable to set up S3 subdomain for PR %v in %v/%v with instance %v: %v", pr.Number, pr.RepoOwner, pr.RepoName, *instance.InstanceId, err.Error())
		commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, Config.SetupSpinmintFailedMessage)
		return
	}

	smLink := fmt.Sprintf("%v.%v", *instance.InstanceId, Config.AWSDnsSuffix)
	if Config.SpinmintsUseHttps {
		smLink = "https://" + smLink
	} else {
		smLink = "http://" + smLink
	}

	var message string
	if upgradeServer {
		message = Config.SetupSpinmintUpgradeDoneMessage
	} else {
		message = Config.SetupSpinmintDoneMessage
	}

	message = strings.Replace(message, SPINMINT_LINK, smLink, 1)
	message = strings.Replace(message, INSTANCE_ID, INSTANCE_ID_MESSAGE+*instance.InstanceId, 1)

	commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, message)
}

func waitForBuild(client *jenkins.Jenkins, pr *model.PullRequest) *model.PullRequest {
	for {
		if result := <-Srv.Store.PullRequest().Get(pr.RepoOwner, pr.RepoName, pr.Number); result.Err != nil {
			LogError("Unable to get updated PR while waiting for spinmint: %v", result.Err.Error())
		} else {
			// Update the PR in case the build link has changed because of a new commit
			pr = result.Data.(*model.PullRequest)
		}

		if pr.BuildLink != "" {
			LogInfo("BuildLink for %v in %v/%v is %v", pr.Number, pr.RepoOwner, pr.RepoName, pr.BuildLink)
			// Doing this because the lib we are using does not support folders :(
			var jobNumber int64
			var jobName string

			parts := strings.Split(pr.BuildLink, "/")
			// Doing this because the lib we are using does not support folders :(
			if pr.RepoName == "mattermost-server" {
				jobNumber, _ = strconv.ParseInt(parts[len(parts)-2], 10, 32)
				jobName = parts[len(parts)-3] //mattermost-server
				jobName = "mp/job/" + jobName
			} else if pr.RepoName == "mattermost-webapp" {
				jobNumber, _ = strconv.ParseInt(parts[len(parts)-3], 10, 32)
				jobName = parts[len(parts)-6]     //mattermost-webapp
				subJobName := parts[len(parts)-4] //PR-XXXX

				jobName = jobName + "/job/" + subJobName
			} else {
				LogError("Did not know this repository: %v. Aborting.", pr.RepoName)
				break
			}

			job, err := client.GetJob(jobName)
			if err != nil {
				LogError("Failed to get Jenkins job %v: %v", jobName, err)
				break
			}

			// Doing this because the lib we are using does not support folders :(
			// This time is in the Jenkins job Name because it returns just the name
			job.Name = jobName

			build, err := client.GetBuild(job, int(jobNumber))
			if err != nil {
				LogErrorToMattermost("Failed to get build %v for PR %v in %v/%v: %v", jobNumber, pr.Number, pr.RepoOwner, pr.RepoName, err)
				break
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

		LogInfo("Sleeping a bit....Will re-check the Jenkins Build...")
		time.Sleep(30 * time.Second)
	}
	return pr
}

// Returns instance ID of instance created
func setupSpinmint(prNumber int, prRef string, repo *Repository, upgrade bool) (*ec2.Instance, error) {
	LogInfo("Setting up spinmint for PR: " + strconv.Itoa(prNumber))

	svc := ec2.New(session.New(), Config.GetAwsConfig())

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
	sdata = strings.Replace(sdata, "BUILD_NUMBER", strconv.Itoa(prNumber), -1)
	sdata = strings.Replace(sdata, "BRANCH_NAME", prRef, -1)
	bsdata := []byte(sdata)
	sdata = base64.StdEncoding.EncodeToString(bsdata)

	var one int64 = 1
	params := &ec2.RunInstancesInput{
		ImageId:          &Config.AWSImageId,
		MaxCount:         &one,
		MinCount:         &one,
		InstanceType:     &Config.AWSInstanceType,
		UserData:         &sdata,
		SecurityGroupIds: []*string{&Config.AWSSecurityGroup},
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
				Value: aws.String("Spinmint-" + prRef),
			},
			{
				Key:   aws.String("Created"),
				Value: aws.String(time.Now().Format("2006-01-02/15:04:05")),
			},
		},
	})
	if errtag != nil {
		LogError("Could not create tags for instance: " + *resp.Instances[0].InstanceId + " Error: " + errtag.Error())
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

func updateRoute53Subdomain(name, target, action string) error {
	svc := route53.New(session.New(), Config.GetAwsConfig())
	domainName := fmt.Sprintf("%v.%v", name, Config.AWSDnsSuffix)

	targetServer := target
	if target == "" && action == "DELETE" {
		targetServer = getPublicDnsName(name)
	}

	params := &route53.ChangeResourceRecordSetsInput{
		ChangeBatch: &route53.ChangeBatch{
			Changes: []*route53.Change{
				{
					Action: aws.String(action),
					ResourceRecordSet: &route53.ResourceRecordSet{
						Name: aws.String(domainName),
						TTL:  aws.Int64(30),
						Type: aws.String("CNAME"),
						ResourceRecords: []*route53.ResourceRecord{
							{
								Value: aws.String(targetServer),
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
		return err
	}

	return nil
}
