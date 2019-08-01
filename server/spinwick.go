// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"

	"time"

	"github.com/mattermost/mattermost-mattermod/cloud"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/mlog"
	mattermostModel "github.com/mattermost/mattermost-server/model"
	"github.com/pkg/errors"
)

func (s *Server) handleCreateSpinWick(pr *model.PullRequest, size string) {
	installationID, sendMattermostLog, err := s.createSpinWick(pr, size)
	if err != nil {
		mlog.Error("Failed to create SpinWick", mlog.Err(err), mlog.String("repo_name", pr.RepoName), mlog.Int("pr", pr.Number), mlog.String("installation_id", installationID))
		s.commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, s.Config.SetupSpinmintFailedMessage)
		if sendMattermostLog {
			additionalFields := map[string]string{
				"Installation ID": installationID,
			}
			s.logPrettyErrorToMattermost("[ SpinWick ] Creation Failed", pr, err, additionalFields)
		}
	}
}

// createSpinwick creates a SpinWick with the following behavior:
// - no cloud installation found = installation is created
// - cloud installation found = actual ID string and no error
// - any errors = error is returned
func (s *Server) createSpinWick(pr *model.PullRequest, size string) (string, bool, error) {
	installationID := "n/a"
	ownerID := makeSpinWickID(pr.RepoName, pr.Number)
	id, err := cloud.GetInstallationIDFromOwnerID(s.Config.ProvisionerServer, ownerID)
	if err != nil {
		return installationID, true, err
	}
	if id != "" {
		return id, false, nil
	}

	mlog.Info("No SpinWick found for this PR. Creating a new one.")

	_, client, err := s.buildJenkinsClient(pr)
	if err != nil {
		return installationID, true, errors.Wrap(err, "unable to build Jenkins client")
	}

	mlog.Info("Waiting for build to finish to set up SpinWick", mlog.Int("pr", pr.Number), mlog.String("repo_owner", pr.RepoOwner), mlog.String("repo_name", pr.RepoName), mlog.String("build_link", pr.BuildLink))

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	defer cancel()

	pr, err = s.waitForBuild(ctx, client, pr)
	if err != nil {
		return installationID, false, errors.Wrap(err, "error waiting for PR build to finish")
	}

	mlog.Info("Provisioning Server - Installation request")

	installationRequest := cloud.CreateInstallationRequest{
		OwnerID:  ownerID,
		Version:  pr.Sha[0:7],
		DNS:      fmt.Sprintf("%s.%s", ownerID, s.Config.DNSNameTestServer),
		Size:     size,
		Affinity: "multitenant",
	}

	b, err := json.Marshal(installationRequest)
	if err != nil {
		return installationID, true, errors.Wrap(err, "unable to marshal the installation request")
	}

	url := fmt.Sprintf("%s/api/installations", s.Config.ProvisionerServer)
	respReqInstallation, err := makeRequest("POST", url, bytes.NewBuffer(b))
	if err != nil {
		return installationID, true, errors.Wrap(err, "unable to make the installation creation request to the provisioning server")
	}
	defer respReqInstallation.Body.Close()

	var installation cloud.Installation
	err = json.NewDecoder(respReqInstallation.Body).Decode(&installation)
	if err != nil && err != io.EOF {
		return installationID, true, errors.Wrap(err, "error decoding installation")
	}
	installationID = installation.ID
	mlog.Info("Provisioner Server - installation request", mlog.String("InstallationID", installationID))

	wait := 480
	mlog.Info("Waiting up to 480 seconds for the mattermost installation to complete...")
	ctx, cancel = context.WithTimeout(context.Background(), time.Duration(wait)*time.Second)
	defer cancel()
	err = s.waitForMattermostInstallationStable(ctx, pr, installationID)
	if err != nil {
		return installationID, true, errors.Wrap(err, "error waiting for installation to become stable")
	}

	spinwickURL := fmt.Sprintf("https://%s.%s", makeSpinWickID(pr.RepoName, pr.Number), s.Config.DNSNameTestServer)
	err = s.initializeMattermostTestServer(spinwickURL, pr.Number)
	if err != nil {
		return installationID, true, errors.Wrap(err, "failed to initialize the SpinWick")
	}
	userTable := "| Account Type | Username | Password |\n|---|---|---|\n| Admin | sysadmin | Sys@dmin123 |\n| User | user-1 | User-1@123 |"
	msg := fmt.Sprintf("Mattermost test server created! :tada:\n\nAccess here: %s\n\n%s", spinwickURL, userTable)
	s.commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, msg)

	return installationID, false, nil
}

func (s *Server) handleUpdateSpinWick(pr *model.PullRequest) {
	installationID, sendMattermostLog, err := s.updateSpinWick(pr)
	if err != nil {
		mlog.Error("Error trying to update SpinWick", mlog.Err(err), mlog.String("repo_name", pr.RepoName), mlog.Int("pr", pr.Number), mlog.String("installation_id", installationID))
		s.commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, s.Config.SetupSpinmintFailedMessage)
		if sendMattermostLog {
			additionalFields := map[string]string{
				"Installation ID": installationID,
			}
			s.logPrettyErrorToMattermost("[ SpinWick ] Update Failed", pr, err, additionalFields)
		}
	}
}

// updateSpinWick updates a SpinWick with the following behavior:
// - no cloud installation found = error is returned
// - cloud installation found and updated = actual ID string and no error
// - any errors = error is returned
func (s *Server) updateSpinWick(pr *model.PullRequest) (string, bool, error) {
	installationID := "n/a"

	ownerID := makeSpinWickID(pr.RepoName, pr.Number)
	id, err := cloud.GetInstallationIDFromOwnerID(s.Config.ProvisionerServer, ownerID)
	if err != nil {
		return installationID, true, err
	}
	if id == "" {
		return installationID, true, errors.New("no installation found matching this PR")
	}
	installationID = id

	mlog.Info("Sleeping a bit to wait for the build process to start", mlog.Int("pr", pr.Number), mlog.String("sha", pr.Sha))
	time.Sleep(60 * time.Second)

	wait := 480
	mlog.Info("Waiting to get the up-to-date build link", mlog.Int("wait_seconds", wait))
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(wait)*time.Second)
	defer cancel()

	// need to do this workaroud here because when push a new commit the build link
	// is not updated and can be blank for some time
	buildLink, err := s.checkBuildLink(ctx, pr)
	if err != nil || buildLink == "" {
		return installationID, true, errors.Wrap(err, "error waiting for build link")
	}

	s.commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, "New commit detected. SpinWick upgrade will occur after the build is successful.")

	_, client, err := s.buildJenkinsClient(pr)
	if err != nil {
		return installationID, true, errors.Wrap(err, "unable to build Jenkins client")
	}

	mlog.Info("Waiting for build to finish to set up SpinWick", mlog.Int("pr", pr.Number), mlog.String("repo_owner", pr.RepoOwner), mlog.String("repo_name", pr.RepoName), mlog.String("build_link", pr.BuildLink))

	ctx, cancel = context.WithTimeout(context.Background(), 45*time.Minute)
	defer cancel()

	pr, err = s.waitForBuild(ctx, client, pr)
	if err != nil {
		return installationID, false, errors.Wrap(err, "error waiting for PR build to finish")
	}

	// TODO: remove this when we starting building the docker image in the same build pipeline
	time.Sleep(60 * time.Second)

	mlog.Info("Provisioning Server - Upgrade request", mlog.String("SHA", pr.Sha))
	shortCommit := pr.Sha[0:7]
	payload := fmt.Sprintf("{\n\"version\": \"%s\"}", shortCommit)
	var mmStr = []byte(payload)
	url := fmt.Sprintf("%s/api/installation/%s/mattermost", s.Config.ProvisionerServer, installationID)
	resp, err := makeRequest("PUT", url, bytes.NewBuffer(mmStr))
	if err != nil {
		return installationID, true, errors.Wrap(err, "encountered error making upgrade request to provisioning server")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 202 {
		return installationID, true, fmt.Errorf("upgrade request not accepted by the provisioning server: status code %d", resp.StatusCode)
	}

	mlog.Info("Waiting for mattermost installation to become stable", mlog.Int("wait_seconds", wait))
	ctx, cancel = context.WithTimeout(context.Background(), time.Duration(wait)*time.Second)
	defer cancel()
	err = s.waitForMattermostInstallationStable(ctx, pr, installationID)
	if err != nil {
		return installationID, true, errors.Wrap(err, "encountered error waiting for installation to become stable")
	}

	mmURL := fmt.Sprintf("https://%s.%s", makeSpinWickID(pr.RepoName, pr.Number), s.Config.DNSNameTestServer)
	msg := fmt.Sprintf("Mattermost test server updated!\n\nAccess here: %s", mmURL)
	s.commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, msg)

	return installationID, false, nil
}

func (s *Server) handleDestroySpinWick(pr *model.PullRequest) {
	installationID, sendMattermostLog, err := s.destroySpinWick(pr)
	if err != nil {
		mlog.Error("Failed to delete Mattermost installation", mlog.Err(err), mlog.String("repo_name", pr.RepoName), mlog.Int("pr", pr.Number), mlog.String("installation_id", installationID))
		if sendMattermostLog {
			additionalFields := map[string]string{
				"Installation ID": installationID,
			}
			s.logPrettyErrorToMattermost("[ SpinWick ] Destroy Failed", pr, err, additionalFields)
		}
	}
}

// destroySpinwick destroys a SpinWick with the following behavior:
// - no cloud installation found = empty ID string and no error
// - cloud installation found and deleted = actual ID string and no error
// - any errors = error is returned
func (s *Server) destroySpinWick(pr *model.PullRequest) (string, bool, error) {
	installationID := "n/a"

	ownerID := makeSpinWickID(pr.RepoName, pr.Number)
	id, err := cloud.GetInstallationIDFromOwnerID(s.Config.ProvisionerServer, ownerID)
	if err != nil {
		return installationID, true, err
	}
	if id == "" {
		return installationID, false, nil
	}
	installationID = id

	mlog.Info("Destroying SpinWick", mlog.Int("pr", pr.Number), mlog.String("repo_owner", pr.RepoOwner), mlog.String("repo_name", pr.RepoName), mlog.String("installation_id", installationID))

	url := fmt.Sprintf("%s/api/installation/%s", s.Config.ProvisionerServer, installationID)
	resp, err := makeRequest("DELETE", url, nil)
	if err != nil {
		return installationID, true, errors.Wrap(err, "unable to make installation delete request to provisioning server")
	}
	defer resp.Body.Close()

	// Old comments created by Mattermod user will be deleted here.
	s.commentLock.Lock()
	defer s.commentLock.Unlock()

	comments, _, err := NewGithubClient(s.Config.GithubAccessToken).Issues.ListComments(context.Background(), pr.RepoOwner, pr.RepoName, pr.Number, nil)
	if err != nil {
		return installationID, true, errors.Wrap(err, "unable to get list of old comments")
	}
	s.removeOldComments(comments, pr)

	s.commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, s.Config.DestroyedSpinmintMessage)

	return installationID, false, nil
}

func (s *Server) checkBuildLink(ctx context.Context, pr *model.PullRequest) (string, error) {
	client := NewGithubClient(s.Config.GithubAccessToken)
	repo, _ := s.GetRepository(pr.RepoOwner, pr.RepoName)
	for {
		combined, _, err := client.Repositories.GetCombinedStatus(context.Background(), pr.RepoOwner, pr.RepoName, pr.Sha, nil)
		if err != nil {
			return "", err
		}
		for _, status := range combined.Statuses {
			if *status.Context == repo.BuildStatusContext {
				if *status.TargetURL != "" {
					return *status.TargetURL, nil
				}
			}
		}

		// for the repos using circleci we have the checks now
		checks, _, err := client.Checks.ListCheckRunsForRef(context.Background(), pr.RepoOwner, pr.RepoName, pr.Sha, nil)
		if err != nil {
			return "", err
		}
		for _, status := range checks.CheckRuns {
			if *status.Name == repo.BuildStatusContext {
				return status.GetHTMLURL(), nil
			}
		}

		select {
		case <-ctx.Done():
			s.commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, "Timed out waiting for build link. Please check the logs.")
			return "", fmt.Errorf("timed out waiting the build link")
		case <-time.After(10 * time.Second):
		}
	}
}

func (s *Server) waitForMattermostInstallationStable(ctx context.Context, pr *model.PullRequest, installationID string) error {
	for {
		installation, err := cloud.GetInstallation(s.Config.ProvisionerServer, installationID)
		if err != nil {
			return err
		}

		switch installation.State {
		case "stable":
			return nil
		case "creation-failed":
			return errors.New("the installation creation failed")
		case "creation-no-compatible-clusters":
			err = s.requestK8sClusterCreation(pr)
			if err != nil {
				return errors.Wrap(err, "unable to create a new cluster to accommodate the installation")
			}
			// This sleep is a bit hacky, but is intended to ensure that the
			// installation has time to be worked on before we check its state
			// again so we don't create another cluster needlessly.
			time.Sleep(30 * time.Second)
		}

		mlog.Info("Waiting for installation to stabilize", mlog.String("installation_id", installation.ID), mlog.String("state", installation.State))
		select {
		case <-ctx.Done():
			return errors.New("timed out waiting for the mattermost installation to stabilize")
		case <-time.After(10 * time.Second):
		}
	}
}

func (s *Server) waitK8sCluster(ctx context.Context, pr *model.PullRequest, clusterRequestID string) error {
	for {
		url := fmt.Sprintf("%s/api/cluster/%s", s.Config.ProvisionerServer, clusterRequestID)
		resp, err := makeRequest("GET", url, nil)
		if err != nil {
			mlog.Error("Error making the post request to create the k8s cluster", mlog.Err(err))
			return err
		}
		defer resp.Body.Close()

		var clusterRequest cloud.Cluster
		err = json.NewDecoder(resp.Body).Decode(&clusterRequest)
		if err != nil && err != io.EOF {
			mlog.Error("Error decoding cluster response", mlog.Err(err))
		}
		if clusterRequest.State == "stable" {
			s.commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, "Kubernetes cluster created. Now will deploy Mattermost... Hang on!")
			return nil
		} else if clusterRequest.State == "creation-failed" {
			s.commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, "Failed to create the k8s cluster.")
			return fmt.Errorf("error creating k8s cluster")
		}
		mlog.Info("Provisioner Server - cluster request creating... sleep", mlog.String("ClusterID", clusterRequest.ID), mlog.String("State", clusterRequest.State))
		time.Sleep(20 * time.Second)
		select {
		case <-ctx.Done():
			s.commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, "Timed out waiting for the kubernetes cluster. Please check the logs.")
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

func (s *Server) initializeMattermostTestServer(mmURL string, prNumber int) error {
	mlog.Info("Initializing Mattermost installation")

	wait := 300
	mlog.Info("Waiting up to 300 seconds for DNS to propagate")
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(wait)*time.Second)
	defer cancel()

	mmHost, _ := url.Parse(mmURL)
	err := checkDNS(ctx, fmt.Sprintf("%s:443", mmHost.Host))
	if err != nil {
		return errors.Wrap(err, "timed out waiting for DNS to propagate for installation")
	}

	client := mattermostModel.NewAPIv4Client(mmURL)

	//check if Mattermost is available
	ctx, cancel = context.WithTimeout(context.Background(), time.Duration(wait)*time.Second)
	defer cancel()
	err = checkMMPing(ctx, client)
	if err != nil {
		return errors.Wrap(err, "failed to get mattermost ping response")
	}

	user := &mattermostModel.User{
		Username: "sysadmin",
		Email:    "sysadmin@example.mattermost.com",
		Password: "Sys@dmin123",
	}
	_, response := client.CreateUser(user)
	if response.StatusCode != 201 {
		return fmt.Errorf("error creating the initial mattermost user: status code = %d, message = %s", response.StatusCode, response.Error.Message)
	}

	client.Logout()
	userLogged, response := client.Login("sysadmin", "Sys@dmin123")
	if response.StatusCode != 200 {
		return fmt.Errorf("error logging in with initial mattermost user: status code = %d, message = %s", response.StatusCode, response.Error.Message)
	}

	teamName := fmt.Sprintf("pr%d", prNumber)
	team := &mattermostModel.Team{
		Name:        teamName,
		DisplayName: teamName,
		Type:        "O",
	}
	firstTeam, response := client.CreateTeam(team)
	if response.StatusCode != 201 {
		return fmt.Errorf("error creating the initial team: status code = %d, message = %s", response.StatusCode, response.Error.Message)
	}

	_, response = client.AddTeamMember(firstTeam.Id, userLogged.Id)
	if response.StatusCode != 201 {
		return fmt.Errorf("error adding sysadmin to the initial team: status code = %d, message = %s", response.StatusCode, response.Error.Message)
	}

	testUser := &mattermostModel.User{
		Username: "user-1",
		Email:    "user-1@example.mattermost.com",
		Password: "User-1@123",
	}
	testUser, response = client.CreateUser(testUser)
	if response.StatusCode != 201 {
		return fmt.Errorf("error creating the standard test user: status code = %d, message = %s", response.StatusCode, response.Error.Message)
	}
	_, response = client.AddTeamMember(firstTeam.Id, testUser.Id)
	if response.StatusCode != 201 {
		return fmt.Errorf("error adding standard test user to the initial team: status code = %d, message = %s", response.StatusCode, response.Error.Message)
	}

	config, response := client.GetConfig()
	if response.StatusCode != 200 {
		return fmt.Errorf("error getting mattermost config: status code = %d, message = %s", response.StatusCode, response.Error.Message)
	}

	config.TeamSettings.EnableOpenServer = NewBool(true)
	config.TeamSettings.ExperimentalViewArchivedChannels = NewBool(true)
	config.PluginSettings.EnableUploads = NewBool(true)
	config.ServiceSettings.EnableTesting = NewBool(true)
	config.ServiceSettings.ExperimentalLdapGroupSync = NewBool(true)
	config.ServiceSettings.EnableDeveloper = NewBool(true)
	config.LogSettings.FileLevel = NewString("INFO")
	config.EmailSettings.FeedbackName = NewString("SpinWick Feedback")
	config.EmailSettings.FeedbackEmail = NewString("feedback@mattermost.com")
	config.EmailSettings.ReplyToAddress = NewString("feedback@mattermost.com")
	config.EmailSettings.SMTPUsername = NewString(s.Config.AWSEmailAccessKey)
	config.EmailSettings.SMTPPassword = NewString(s.Config.AWSEmailSecretKey)
	config.EmailSettings.SMTPServer = NewString(s.Config.AWSEmailEndpoint)
	config.EmailSettings.SMTPPort = NewString("465")
	config.EmailSettings.EnableSMTPAuth = NewBool(true)
	config.EmailSettings.ConnectionSecurity = NewString("TLS")
	config.EmailSettings.SendEmailNotifications = NewBool(true)
	config.LdapSettings.Enable = NewBool(true)
	config.LdapSettings.EnableSync = NewBool(true)
	config.LdapSettings.LdapServer = NewString("ldap.forumsys.com")
	config.LdapSettings.BaseDN = NewString("dc=example,dc=com")
	config.LdapSettings.BindUsername = NewString("cn=read-only-admin,dc=example,dc=com")
	config.LdapSettings.BindPassword = NewString("password")
	config.LdapSettings.GroupDisplayNameAttribute = NewString("cn")
	config.LdapSettings.GroupIdAttribute = NewString("entryUUID")
	config.LdapSettings.EmailAttribute = NewString("mail")
	config.LdapSettings.UsernameAttribute = NewString("uid")
	config.LdapSettings.IdAttribute = NewString("uid")
	config.LdapSettings.LoginIdAttribute = NewString("uid")

	// UpdateConfig
	_, response = client.UpdateConfig(config)
	if response.StatusCode != 200 {
		return fmt.Errorf("error updating mattermost config: status code = %d, message = %s", response.StatusCode, response.Error.Message)
	}

	mlog.Info("Mattermost configuration complete")

	return nil
}

func (s *Server) requestK8sClusterCreation(pr *model.PullRequest) error {
	mlog.Info("Building new kubernetes cluster")

	url := fmt.Sprintf("%s/api/clusters", s.Config.ProvisionerServer)
	s.commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, "Please wait while a new kubernetes cluster is created for your SpinWick")

	clusterRequest := cloud.CreateClusterRequest{
		Size: "SizeAlef1000",
	}
	b, err := json.Marshal(clusterRequest)
	if err != nil {
		mlog.Error("Error trying to marshal the cluster request", mlog.Err(err))
		return err
	}

	respReqCluster, err := makeRequest("POST", url, bytes.NewBuffer(b))
	if err != nil {
		mlog.Error("Error trying to send the k8s-cluster-creation request", mlog.Err(err))
		return err
	}
	defer respReqCluster.Body.Close()

	var cluster cloud.Cluster
	err = json.NewDecoder(respReqCluster.Body).Decode(&cluster)
	if err != nil && err != io.EOF {
		mlog.Error("Error decoding cluster", mlog.Err(err))
		return fmt.Errorf("Error decoding cluster: %s", err)
	}
	mlog.Info("Provisioner Server - cluster request", mlog.String("ClusterID", cluster.ID))

	wait := 900
	mlog.Info("Waiting up to 900 seconds for the k8s cluster creation to complete...")
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(wait)*time.Second)
	defer cancel()

	return s.waitK8sCluster(ctx, pr, cluster.ID)
}

func checkDNS(ctx context.Context, url string) error {
	for {
		timeout := time.Duration(2 * time.Second)
		_, err := net.DialTimeout("tcp", url, timeout)
		if err == nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for %s to become reachable", url)
		case <-time.After(10 * time.Second):
		}
	}
}

func checkMMPing(ctx context.Context, client *mattermostModel.Client4) error {
	for {
		status, response := client.GetPing()
		if response.StatusCode == 200 && status == "OK" {
			return nil
		}

		select {
		case <-ctx.Done():
			return errors.New("timed out waiting for ok response")
		case <-time.After(10 * time.Second):
		}
	}
}

func makeSpinWickID(repoName string, prNumber int) string {
	return strings.ToLower(fmt.Sprintf("%s-pr-%d", repoName, prNumber))
}

func (s *Server) isSpinWickLabel(label string) bool {
	return label == s.Config.SetupSpinWick || label == s.Config.SetupSpinWickHA
}

func (s *Server) isSpinWickLabelInLabels(labels []string) bool {
	for _, label := range labels {
		if s.isSpinWickLabel(label) {
			return true
		}
	}

	return false
}
