// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	// "bytes"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/url"
	"path"

	// "path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/route53"
	jenkins "github.com/cpanato/golang-jenkins"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/mlog"
	// "github.com/mattermost/mattermost-load-test/ltops"
	// "github.com/mattermost/mattermost-load-test/ltparse"
	// "github.com/mattermost/mattermost-load-test/terraform"
)

// TODO FIXME
// func waitForBuildAndSetupLoadtest(pr *model.PullRequest) {
// 	repo, ok := Config.GetRepository(pr.RepoOwner, pr.RepoName)
// 	if !ok || repo.JenkinsServer == "" {
// 		LogError("Unable to set up loadtest for PR %v in %v/%v without Jenkins configured for server", pr.Number, pr.RepoOwner, pr.RepoName)
// 		commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, "Failed to setup loadtest")
// 		return
// 	}

// 	credentials, ok := Config.JenkinsCredentials[repo.JenkinsServer]
// 	if !ok {
// 		LogError("No Jenkins credentials for server %v required for PR %v in %v/%v", repo.JenkinsServer, pr.Number, pr.RepoOwner, pr.RepoName)
// 		commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, "Failed to setup loadtest")
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
// 	config.WorkingDirectory = filepath.Join("./clusters/", config.Name)

// 	LogInfo("Creating terraform cluster for loadtest for PR %v in %v/%v", pr.Number, pr.RepoOwner, pr.RepoName)
// 	cluster, err := terraform.CreateCluster(config)
// 	if err != nil {
// 		LogError("Unable to setup cluster: " + err.Error())
// 		commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, "Failed to setup loadtest")
// 	}
// 	// Wait for the cluster to init
// 	time.Sleep(time.Minute)

// 	results := bytes.NewBuffer(nil)

// 	LogInfo("Deploying to cluster for loadtest for PR %v in %v/%v", pr.Number, pr.RepoOwner, pr.RepoName)
// 	if err := cluster.Deploy("https://releases.mattermost.com/mattermost-platform-pr/"+strconv.Itoa(pr.Number)+"/mattermost-enterprise-linux-amd64.tar.gz", "mattermod.mattermost-license"); err != nil {
// 		LogError("Unable to deploy cluster: " + err.Error())
// 		commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, "Failed to setup loadtest")
// 		return
// 	}
// 	if err := cluster.Loadtest("https://releases.mattermost.com/mattermost-load-test/mattermost-load-test.tar.gz"); err != nil {
// 		LogError("Unable to deploy loadtests to cluster: " + err.Error())
// 		commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, "Failed to setup loadtest")
// 		return
// 	}

// 	// Wait for the cluster restart after deploy
// 	time.Sleep(time.Minute)

// 	LogInfo("Runing loadtest for PR %v in %v/%v", pr.Number, pr.RepoOwner, pr.RepoName)
// 	if err := cluster.Loadtest(results); err != nil {
// 		LogError("Unable to loadtest cluster: " + err.Error())
// 		commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, "Failed to setup loadtest")
// 		return
// 	}
// 	LogInfo("Destroying cluster for PR %v in %v/%v", pr.Number, pr.RepoOwner, pr.RepoName)
// 	if err := cluster.Destroy(); err != nil {
// 		LogError("Unable to destroy cluster: " + err.Error())
// 		commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, "Failed to setup loadtest")
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
// 	commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, githubOutput.String())
// }
func waitForBuildAndSetupLoadtest(pr *model.PullRequest) {
	return
}

func waitForBuildAndSetupSpinmint(pr *model.PullRequest, upgradeServer bool) {
	repo, ok := Config.GetRepository(pr.RepoOwner, pr.RepoName)
	if !ok || repo.JenkinsServer == "" {
		mlog.Error("Unable to set up spintmint for PR without Jenkins configured for server", mlog.Int("pr", pr.Number), mlog.String("repo_owner", pr.RepoOwner), mlog.String("repo_name", pr.RepoName))
		commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, Config.SetupSpinmintFailedMessage)
		return
	}

	credentials, ok := Config.JenkinsCredentials[repo.JenkinsServer]
	if !ok {
		mlog.Error("No Jenkins credentials for server required for PR", mlog.String("jenkins", repo.JenkinsServer), mlog.Int("pr", pr.Number), mlog.String("repo_owner", pr.RepoOwner), mlog.String("repo_name", pr.RepoName))
		commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, Config.SetupSpinmintFailedMessage)
		return
	}

	client := jenkins.NewJenkins(&jenkins.Auth{
		Username: credentials.Username,
		ApiToken: credentials.ApiToken,
	}, credentials.URL)

	mlog.Info("Waiting for Jenkins to build to set up spinmint for PR", mlog.Int("pr", pr.Number), mlog.String("repo_owner", pr.RepoOwner), mlog.String("repo_name", pr.RepoName))

	pr, errr := waitForBuild(client, pr)
	if errr == false || pr == nil {
		commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, Config.SetupSpinmintFailedMessage)
		return
	}

	instance, err := setupSpinmint(pr.Number, pr.Ref, repo, upgradeServer)
	if err != nil {
		LogErrorToMattermost("Unable to set up spinmint for PR %v in %v/%v: %v", pr.Number, pr.RepoOwner, pr.RepoName, err.Error())
		commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, Config.SetupSpinmintFailedMessage)
		return
	}

	mlog.Info("Waiting for instance to come up.")
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

	spinmint := &model.Spinmint{
		InstanceId: *instance.InstanceId,
		RepoOwner:  pr.RepoOwner,
		RepoName:   pr.RepoName,
		Number:     pr.Number,
		CreatedAt:  time.Now().UTC().Unix(),
	}
	storeSpinmintInfo(spinmint)
}

func waitForMobileAppsBuild(pr *model.PullRequest) {
	repo, ok := Config.GetRepository(pr.RepoOwner, pr.RepoName)
	if !ok || repo.JenkinsServer == "" {
		mlog.Error("Unable to build the mobile app for PR %v in %v/%v without Jenkins configured for server", mlog.Int("pr", pr.Number), mlog.String("repo_owner", pr.RepoOwner), mlog.String("repo_name", pr.RepoName))
		commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, Config.BuildMobileAppFailedMessage)
		return
	}

	credentials, ok := Config.JenkinsCredentials[repo.JenkinsServer]
	if !ok {
		mlog.Error("No Jenkins credentials for server required for PR", mlog.String("jenkins", repo.JenkinsServer), mlog.Int("pr", pr.Number), mlog.String("repo_owner", pr.RepoOwner), mlog.String("repo_name", pr.RepoName))
		commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, Config.BuildMobileAppFailedMessage)
		return
	}

	client := jenkins.NewJenkins(&jenkins.Auth{
		Username: credentials.Username,
		ApiToken: credentials.ApiToken,
	}, credentials.URL)

	mlog.Info("Waiting for Jenkins to build to start build the mobile app for PR", mlog.Int("pr", pr.Number), mlog.String("repo_owner", pr.RepoOwner), mlog.String("repo_name", pr.RepoName))

	pr, errr := waitForBuild(client, pr)
	if errr == false || pr == nil {
		commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, Config.BuildMobileAppFailedMessage)
		return
	}

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
			commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, Config.BuildMobileAppFailedMessage)
			return
		} else {
			mlog.Info("build is running", mlog.Int("build", build.Number), mlog.Bool("building", build.Building))
		}
		time.Sleep(60 * time.Second)
	}

	prNumberStr := fmt.Sprintf("PR-%d", pr.Number)
	msgMobile := Config.BuildMobileAppDoneMessage
	msgMobile = strings.Replace(msgMobile, "PR_NUMBER", prNumberStr, 2)
	msgMobile = strings.Replace(msgMobile, "ANDROID_APP", prNumberStr, 1)
	msgMobile = strings.Replace(msgMobile, "IOS_APP", prNumberStr, 1)
	commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, msgMobile)
	return
}

func waitForBuild(client *jenkins.Jenkins, pr *model.PullRequest) (*model.PullRequest, bool) {
	for {
		if result := <-Srv.Store.PullRequest().Get(pr.RepoOwner, pr.RepoName, pr.Number); result.Err != nil {
			mlog.Error("Unable to get updated PR while waiting for spinmint", mlog.String("build_error", result.Err.Error()))
			return nil, false
		} else {
			// Update the PR in case the build link has changed because of a new commit
			pr = result.Data.(*model.PullRequest)
		}

		if pr.BuildLink != "" {
			mlog.Info("BuildLink for PR", mlog.Int("pr", pr.Number), mlog.String("repo_owner", pr.RepoOwner), mlog.String("repo_name", pr.RepoName), mlog.String("buildlink", pr.BuildLink))
			// Doing this because the lib we are using does not support folders :(
			var jobNumber int64
			var jobName string

			parts := strings.Split(pr.BuildLink, "/")
			// Doing this because the lib we are using does not support folders :(
			if pr.RepoName == "mattermost-server" {
				jobNumber, _ = strconv.ParseInt(parts[len(parts)-3], 10, 32)
				jobName = parts[len(parts)-6]     //mattermost-server
				subJobName := parts[len(parts)-4] //PR-XXXX

				jobName = "mp/job/" + jobName + "/job/" + subJobName
				mlog.Info("Job name for server", mlog.String("job", jobName))
			} else if pr.RepoName == "mattermost-mobile" {
				jobNumber, _ = strconv.ParseInt(parts[len(parts)-2], 10, 32)
				jobName = parts[len(parts)-3] //mattermost-mobile
				jobName = "mm/job/" + jobName
				mlog.Info("Job name for mobile", mlog.String("job", jobName))
			} else if pr.RepoName == "mattermost-webapp" {
				jobNumber, _ = strconv.ParseInt(parts[len(parts)-3], 10, 32)
				jobName = parts[len(parts)-6]     //mattermost-webapp
				subJobName := parts[len(parts)-4] //PR-XXXX

				jobName = "mw/job/" + jobName + "/job/" + subJobName
				mlog.Info("Job name for webapp", mlog.String("job", jobName))
			} else {
				mlog.Error("Did not know this repository. Aborting.", mlog.String("repo_name", pr.RepoName))
				return pr, false
			}

			job, err := client.GetJob(jobName)
			if err != nil {
				mlog.Error("Failed to get Jenkins job", mlog.String("job", jobName), mlog.Err(err))
				return pr, false
			}

			// Doing this because the lib we are using does not support folders :(
			// This time is in the Jenkins job Name because it returns just the name
			job.Name = jobName

			build, err := client.GetBuild(job, int(jobNumber))
			if err != nil {
				LogErrorToMattermost("Failed to get build %v for PR %v in %v/%v: %v", jobNumber, pr.Number, pr.RepoOwner, pr.RepoName, err)
				return pr, false
			}

			if !build.Building && build.Result == "SUCCESS" {
				mlog.Info("build for PR succeeded!", mlog.Int64("jobnumber", jobNumber), mlog.Int("pr", pr.Number), mlog.String("repo_owner", pr.RepoOwner), mlog.String("repo_name", pr.RepoName))
				return pr, true
			} else if build.Result == "FAILURE" {
				mlog.Error("build has status FAILURE. Aborting.", mlog.Int("build", build.Number), mlog.String("build_error", build.Result))
				return pr, false
			} else {
				mlog.Info("build is running", mlog.Int("build", build.Number), mlog.Bool("building", build.Building))
			}
		} else {
			mlog.Error("Unable to find build link for PR", mlog.Int("pr", pr.Number))
		}

		mlog.Info("Sleeping a bit....Will re-check the Jenkins Build...")
		time.Sleep(30 * time.Second)
	}
}

// Returns instance ID of instance created
func setupSpinmint(prNumber int, prRef string, repo *Repository, upgrade bool) (*ec2.Instance, error) {
	mlog.Info("Setting up spinmint for PR", mlog.Int("pr", prNumber))

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
		SubnetId:         &Config.AWSSubNetId,
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
		mlog.Error("Could not create tags for instance", mlog.String("instance", *resp.Instances[0].InstanceId), mlog.String("tag_error", errtag.Error()))
	}

	return resp.Instances[0], nil
}

func destroySpinmint(pr *model.PullRequest, instanceId string) {
	mlog.Info("Destroying spinmint for PR", mlog.String("instance", instanceId), mlog.Int("pr", pr.Number), mlog.String("repo_owner", pr.RepoOwner), mlog.String("repo_name", pr.RepoName))

	svc := ec2.New(session.New(), Config.GetAwsConfig())

	params := &ec2.TerminateInstancesInput{
		InstanceIds: []*string{
			&instanceId,
		},
	}

	_, err := svc.TerminateInstances(params)
	if err != nil {
		mlog.Error("Error terminating instances", mlog.Err(err))
		return
	}

	// Remove route53 entry
	err = updateRoute53Subdomain(instanceId, "", "DELETE")
	if err != nil {
		mlog.Error("Error removing the Route53 entry", mlog.Err(err))
		return
	}

	// Remove from the local db
	removeSpinmintInfo(instanceId)
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
		mlog.Error("Problem getting instance ip", mlog.Err(err))
		return ""
	}

	return *resp.Reservations[0].Instances[0].PublicIpAddress
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
		HostedZoneId: &Config.AWSHostedZoneId,
	}

	_, err := svc.ChangeResourceRecordSets(params)
	if err != nil {
		return err
	}

	return nil
}

func CheckSpinmintLifeTime() {
	mlog.Info("Checking Spinmint lifetime...")
	spinmints := []*model.Spinmint{}
	if result := <-Srv.Store.Spinmint().List(); result.Err != nil {
		mlog.Error("Unable to get updated PR while waiting for spinmint", mlog.String("spinmint_error", result.Err.Error()))
	} else {
		spinmints = result.Data.([]*model.Spinmint)
	}

	for _, spinmint := range spinmints {
		mlog.Info("Check if need destroy spinmint for PR", mlog.String("instance", spinmint.InstanceId), mlog.Int("spinmint", spinmint.Number), mlog.String("repo_owner", spinmint.RepoOwner), mlog.String("repo_name", spinmint.RepoName))
		spinmintCreated := time.Unix(spinmint.CreatedAt, 0)
		duration := time.Since(spinmintCreated)
		if int(duration.Hours()) > Config.SpinmintExpirationHour {
			mlog.Info("Will destroy spinmint for PR", mlog.String("instance", spinmint.InstanceId), mlog.Int("spinmint", spinmint.Number), mlog.String("repo_owner", spinmint.RepoOwner), mlog.String("repo_name", spinmint.RepoName))
			pr := &model.PullRequest{
				RepoOwner: spinmint.RepoOwner,
				RepoName:  spinmint.RepoName,
				Number:    spinmint.Number,
			}
			go destroySpinmint(pr, spinmint.InstanceId)
			removeSpinmintInfo(spinmint.InstanceId)
			commentOnIssue(spinmint.RepoOwner, spinmint.RepoName, spinmint.Number, Config.DestroyedExpirationSpinmintMessage)
		}
	}

	mlog.Info("Done checking Spinmint lifetime.")
}

func storeSpinmintInfo(spinmint *model.Spinmint) {
	if result := <-Srv.Store.Spinmint().Save(spinmint); result.Err != nil {
		mlog.Error(result.Err.Error())
	}
}

func removeSpinmintInfo(instanceId string) {
	if result := <-Srv.Store.Spinmint().Delete(instanceId); result.Err != nil {
		mlog.Error(result.Err.Error())
	}
}
