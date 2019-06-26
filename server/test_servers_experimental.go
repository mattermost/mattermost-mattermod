// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"time"

	jenkins "github.com/cpanato/golang-jenkins"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/mlog"
)

type Cluster struct {
	ID                  string
	Provider            string
	Provisioner         string
	ProviderMetadata    []byte `json:",omitempty"`
	ProvisionerMetadata []byte `json:",omitempty"`
	AllowInstallations  bool
	Size                string
	State               string
	CreateAt            int64
	DeleteAt            int64
	LockAcquiredBy      *string
	LockAcquiredAt      int64
}

type Installation struct {
	ID             string
	OwnerID        string
	Version        string
	DNS            string
	Affinity       string
	GroupID        *string
	State          string
	CreateAt       int64
	DeleteAt       int64
	LockAcquiredBy *string
	LockAcquiredAt int64
}

func waitForBuildAndSetupSpinmintExperimental(pr *model.PullRequest) {
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

	var installation string
	if result := <-Srv.Store.Spinmint().Get(pr.Number); result.Err != nil {
		mlog.Error("Unable to get the spinmint information. Will not build the spinmint", mlog.String("pr_error", result.Err.Error()))
	} else if result.Data == nil {
		mlog.Error("No spinmint for this PR in the Database. will start a fresh one.")
		var errInstance error
		installation, errInstance = setupSpinmintExperimental(pr)
		if errInstance != nil {
			LogErrorToMattermost("Unable to set up spinmint for PR %v in %v/%v: %v", pr.Number, pr.RepoOwner, pr.RepoName, errInstance.Error())
			commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, Config.SetupSpinmintFailedMessage)
			return
		}
		spinmint := &model.Spinmint{
			InstanceId: installation,
			RepoOwner:  pr.RepoOwner,
			RepoName:   pr.RepoName,
			Number:     pr.Number,
			CreatedAt:  time.Now().UTC().Unix(),
		}
		storeSpinmintInfo(spinmint)
	}
}

func setupSpinmintExperimental(pr *model.PullRequest) (string, error) {
	mlog.Info("Setting up spinmint experimental for PR", mlog.Int("pr", pr.Number))
	url := fmt.Sprintf("%s/api/clusters", Config.ProvisionerServer)
	mlog.Info("Provisioner Server ", mlog.String("Server", url))

	// Get cluster list
	mlog.Info("Provisioner Server getting clusters")
	resp, err := makeRequest("GET", url, nil)
	if err != nil {
		mlog.Error("Error making the post request to check the k8s cluster", mlog.Err(err))
		return "", err
	}
	defer resp.Body.Close()

	var createClusterList []Cluster
	err = json.NewDecoder(resp.Body).Decode(&createClusterList)
	if err != nil && err != io.EOF {
		mlog.Error("Error decoding cluster message", mlog.Err(err))
		return "", err
	}

	clusterCount := 0
	for _, cluster := range createClusterList {
		if cluster.State == "stable" {
			clusterCount++
			mlog.Info("Provisioner Server counting", mlog.Int("clusterCount", clusterCount))
		}
	}

	// Get cluster list
	mlog.Info("Provisioner Server getting installations")
	urlInstallation := fmt.Sprintf("%s/api/installations", Config.ProvisionerServer)
	respInstallation, err := makeRequest("GET", urlInstallation, nil)
	if err != nil {
		mlog.Error("Error making the post request to check the installations", mlog.Err(err))
		return "", err
	}
	defer respInstallation.Body.Close()

	var createInstallationList []Installation
	err = json.NewDecoder(respInstallation.Body).Decode(&createClusterList)
	if err != nil && err != io.EOF {
		mlog.Error("Error decoding installation message", mlog.Err(err))
		return "", err
	}

	installationCount := 0
	for _, installation := range createInstallationList {
		if installation.State == "stable" {
			installationCount++
			mlog.Info("Provisioner Server counting MM Installations", mlog.Int("installationCount", installationCount))
		}
	}

	mlog.Info("will check if need create a cluster")
	if clusterCount == 0 || installationCount/clusterCount > 5 {
		mlog.Info("Need to spin a new k8s cluster", mlog.Int("clusterCount", installationCount), mlog.Int("clusterCount", clusterCount))
		commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, "Will spin a Kubernetes Cluster. This may take up to 600 seconds.")
		payloadCluster := fmt.Sprint("{\n\"size\":\"SizeAlef1000\"\n}")
		var jsonStr = []byte(payloadCluster)
		respCluster, errCluster := makeRequest("POST", url, bytes.NewBuffer(jsonStr))
		if errCluster != nil {
			mlog.Error("Error making the post request to create the k8s cluster", mlog.Err(errCluster))
			return "", err
		}
		defer respCluster.Body.Close()

		var createClusterRequest Cluster
		errCluster = json.NewDecoder(respCluster.Body).Decode(&createClusterRequest)
		if errCluster != nil && errCluster != io.EOF {
			mlog.Error("Error decoding", mlog.Err(errCluster))
			return "", err
		}
		mlog.Info("Provisioner Server - cluster request", mlog.String("ClusterID", createClusterRequest.ID))

		wait := 900
		mlog.Info("Waiting up to 900 seconds for the k8s cluster installation to complete...")
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(wait)*time.Second)
		defer cancel()
		err = waitK8sCluster(ctx, pr, createClusterRequest.ID)
		if err != nil {
			return "", err
		}

	} else {
		mlog.Info("not needed to create a cluster")
		commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, "We don't need a new Kubernetes cluster, will reuse an existing one. Requesting to deploy Mattermost.")
	}

	mlog.Info("Provisioner Server - Installation request")
	shortCommit := pr.Sha[0:7]
	payload := fmt.Sprintf("{\n\"ownerId\":\"%s-PR-%d\",\n\"dns\": \"pr-%d.%s\",\n\"version\": \"%s\",\n\"affinity\":\"multitenant\"}", pr.RepoName, pr.Number, pr.Number, Config.DNSNameTestServer, shortCommit)
	var mmStr = []byte(payload)
	url = fmt.Sprintf("%s/api/installations", Config.ProvisionerServer)
	respReqInstallation, err := makeRequest("POST", url, bytes.NewBuffer(mmStr))
	if err != nil {
		mlog.Error("Error making the post request to create the mm cluster", mlog.Err(err))
		return "", err
	}
	defer respReqInstallation.Body.Close()

	var createInstallationRequest Installation
	err = json.NewDecoder(respReqInstallation.Body).Decode(&createInstallationRequest)
	if err != nil && err != io.EOF {
		mlog.Error("Error decoding installation request", mlog.Err(err))
		return "", err
	}
	mlog.Info("Provisioner Server - installation request", mlog.String("InstallationID", createInstallationRequest.ID))

	wait := 480
	mlog.Info("Waiting up to 480 seconds for the mattermost installation to complete...")
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(wait)*time.Second)
	defer cancel()
	err = waitMattermostInstallation(ctx, pr, createInstallationRequest.ID)
	if err != nil {
		return "", err
	}

	return createInstallationRequest.ID, nil
}

func destroySpinmintExperimental(pr *model.PullRequest, instanceClusterID string) {
	mlog.Info("Destroying spinmint experimental for PR", mlog.String("instance", instanceClusterID), mlog.Int("pr", pr.Number), mlog.String("repo_owner", pr.RepoOwner), mlog.String("repo_name", pr.RepoName))

	destroyMMInstallation(instanceClusterID)
	// Remove from the local db
	removeSpinmintInfo(instanceClusterID)
}

func destroyMMInstallation(instanceClusterID string) {
	url := fmt.Sprintf("%s/api/installation/%s", Config.ProvisionerServer, instanceClusterID)
	resp, err := makeRequest("DELETE", url, nil)
	if err != nil {
		mlog.Error("Error deleting the installation", mlog.Err(err))
	}
	defer resp.Body.Close()
}

func waitMattermostInstallation(ctx context.Context, pr *model.PullRequest, installationRequestID string) error {
	for {
		url := fmt.Sprintf("%s/api/installation/%s", Config.ProvisionerServer, installationRequestID)
		resp, err := makeRequest("GET", url, nil)
		if err != nil {
			mlog.Error("Error making the post request to create the mm installation", mlog.Err(err))
			return err
		}
		defer resp.Body.Close()
		var installationRequest Installation
		err = json.NewDecoder(resp.Body).Decode(&installationRequest)
		if err != nil && err != io.EOF {
			mlog.Error("Error decoding installation", mlog.Err(err))
		}
		if installationRequest.State == "stable" {
			msg := fmt.Sprintf("Mattermost test server created! :tada:\nAccess here: https://pr-%d.%s", pr.Number, Config.DNSNameTestServer)
			commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, msg)
			return nil
		} else if installationRequest.State == "creation-failed" {
			commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, "Failed to create mattermost installation.")
			return fmt.Errorf("error creating mattermost installation")
		}
		mlog.Info("Provisioner Server - installation request creating... sleep", mlog.String("InstallationID", installationRequest.ID), mlog.String("State", installationRequest.State))
		select {
		case <-ctx.Done():
			destroyMMInstallation(installationRequest.ID)
			commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, "Timeouted the installation. Aborted the test server. Please check the logs.")
			return fmt.Errorf("timed out waiting for the mattermost installation complete. requesting the deletion.")
		case <-time.After(10 * time.Second):
		}
	}
}

func waitK8sCluster(ctx context.Context, pr *model.PullRequest, clusterRequestID string) error {
	for {
		url := fmt.Sprintf("%s/api/cluster/%s", Config.ProvisionerServer, clusterRequestID)
		resp, err := makeRequest("GET", url, nil)
		if err != nil {
			mlog.Error("Error making the post request to create the k8s cluster", mlog.Err(err))
			return err
		}
		defer resp.Body.Close()

		var clusterRequest Cluster
		err = json.NewDecoder(resp.Body).Decode(&clusterRequest)
		if err != nil && err != io.EOF {
			mlog.Error("Error decoding cluster response", mlog.Err(err))
		}
		if clusterRequest.State == "stable" {
			commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, "Kubernetes cluster created. Now will deploy Mattermost... Hang on!")
			return nil
		} else if clusterRequest.State == "creation-failed" {
			commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, "Failed to create the k8s cluster.")
			return fmt.Errorf("error creating k8s cluster")
		}
		mlog.Info("Provisioner Server - cluster request creating... sleep", mlog.String("ClusterID", clusterRequest.ID), mlog.String("State", clusterRequest.State))
		time.Sleep(20 * time.Second)
		select {
		case <-ctx.Done():
			commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, "Timeouted the k8s cluster installation. Aborted the test server. Please check the logs.")
			return fmt.Errorf("timed out waiting for the cluster installation complete")
		case <-time.After(10 * time.Second):
		}
	}
}

func makeRequest(method, url string, payload io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, url, payload)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
