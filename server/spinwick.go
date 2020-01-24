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

	"github.com/google/go-github/v28/github"
	cloudModel "github.com/mattermost/mattermost-cloud/model"
	"github.com/mattermost/mattermost-mattermod/internal/cloudtools"
	"github.com/mattermost/mattermost-mattermod/internal/spinwick"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v5/mlog"
	mattermostModel "github.com/mattermost/mattermost-server/v5/model"
	"github.com/pkg/errors"
)

func (s *Server) handleCreateSpinWick(pr *model.PullRequest, size string, withLicense bool) {
	request := s.createSpinWick(pr, size, withLicense)
	if request.Error != nil {
		if request.Aborted {
			mlog.Warn("Aborted creation of SpinWick", mlog.String("abort_message", request.Error.Error()), mlog.String("repo_name", pr.RepoName), mlog.Int("pr", pr.Number), mlog.String("installation_id", request.InstallationID))
		} else {
			mlog.Error("Failed to create SpinWick", mlog.Err(request.Error), mlog.String("repo_name", pr.RepoName), mlog.Int("pr", pr.Number), mlog.String("installation_id", request.InstallationID))
		}
		comments, err := s.getComments(pr.RepoOwner, pr.RepoName, pr.Number)
		if err != nil {
			mlog.Error("Error getting comments", mlog.Err(err))
		} else {
			s.removeOldComments(comments, pr)
		}
		for _, label := range pr.Labels {
			if s.isSpinWickLabel(label) {
				s.removeLabel(pr.RepoOwner, pr.RepoName, pr.Number, label)
			}
		}
		s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number, s.Config.SetupSpinmintFailedMessage)

		if request.ReportError {
			additionalFields := map[string]string{
				"Installation ID": request.InstallationID,
			}
			s.logPrettyErrorToMattermost("[ SpinWick ] Creation Failed", pr, request.Error, additionalFields)
		}
	}
}

// createSpinwick creates a SpinWick with the following behavior:
// - no cloud installation found = installation is created
// - cloud installation found = actual ID string and no error
// - any errors = error is returned
func (s *Server) createSpinWick(pr *model.PullRequest, size string, withLicense bool) *spinwick.Request {
	request := &spinwick.Request{
		InstallationID: "n/a",
		Error:          nil,
		ReportError:    false,
		Aborted:        false,
	}
	ownerID := makeSpinWickID(pr.RepoName, pr.Number)
	id, err := cloudtools.GetInstallationIDFromOwnerID(s.Config.ProvisionerServer, s.Config.AwsAPIKey, ownerID)
	if err != nil {
		return request.WithError(err).ShouldReportError()
	}
	if id != "" {
		return request.WithInstallationID(id).WithError(fmt.Errorf("Already found a installation belonging to %s", ownerID)).IntentionalAbort()
	}
	request.InstallationID = id

	mlog.Info("No SpinWick found for this PR. Creating a new one.")

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	defer cancel()
	// set the version to master
	version := "master"
	// if is server or webapp then set version to the PR git commit hash
	if pr.RepoName == "mattermost-server" || pr.RepoName == "mattermost-webapp" {
		reg, errDocker := s.Builds.dockerRegistryClient(s)
		if errDocker != nil {
			return request.WithError(errors.Wrap(errDocker, "unable to get docker registry client")).ShouldReportError()
		}

		mlog.Info("Waiting for docker image to set up SpinWick", mlog.Int("pr", pr.Number), mlog.String("repo_owner", pr.RepoOwner), mlog.String("repo_name", pr.RepoName), mlog.String("build_link", pr.BuildLink))

		prNew, errImage := s.Builds.waitForImage(ctx, s, reg, pr)
		if errImage != nil {
			return request.WithError(errors.Wrap(errImage, "error waiting for the docker image. Aborting")).IntentionalAbort()
		}

		version = s.Builds.getInstallationVersion(prNew)
	}

	mlog.Info("Provisioning Server - Installation request")

	installationRequest := &cloudModel.CreateInstallationRequest{
		OwnerID:  ownerID,
		Version:  version,
		DNS:      fmt.Sprintf("%s.%s", ownerID, s.Config.DNSNameTestServer),
		Size:     size,
		Affinity: "multitenant",
	}
	if withLicense {
		installationRequest.License = s.Config.SpinWickHALicense
	}

	headers := map[string]string{
		"x-api-key": s.Config.AwsAPIKey,
	}
	cloudClient := cloudModel.NewClientWithHeaders(s.Config.ProvisionerServer, headers)
	installation, err := cloudClient.CreateInstallation(installationRequest)
	if err != nil {
		return request.WithError(errors.Wrap(err, "unable to make the installation creation request to the provisioning server")).ShouldReportError()
	}
	request.InstallationID = installation.ID
	mlog.Info("Provisioner Server - installation request", mlog.String("InstallationID", request.InstallationID))

	wait := 1200
	mlog.Info("Waiting for mattermost installation to become stable", mlog.Int("wait_seconds", wait))
	ctx, cancel = context.WithTimeout(context.Background(), time.Duration(wait)*time.Second)
	defer cancel()

	s.waitForInstallationStable(ctx, pr, request)
	if request.Error != nil {
		return request.WithError(errors.Wrap(request.Error, "error waiting for installation to become stable"))
	}

	spinwickURL := fmt.Sprintf("https://%s.%s", makeSpinWickID(pr.RepoName, pr.Number), s.Config.DNSNameTestServer)
	err = s.initializeMattermostTestServer(spinwickURL, pr.Number)
	if err != nil {
		return request.WithError(errors.Wrap(err, "failed to initialize the Installation")).ShouldReportError()
	}
	userTable := "| Account Type | Username | Password |\n|---|---|---|\n| Admin | sysadmin | Sys@dmin123 |\n| User | user-1 | User-1@123 |"
	msg := fmt.Sprintf("Mattermost test server created! :tada:\n\nAccess here: %s\n\n%s", spinwickURL, userTable)
	s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number, msg)

	return request
}

func (s *Server) handleUpdateSpinWick(pr *model.PullRequest, withLicense bool) {
	// other repos we are not updating
	if pr.RepoName != "mattermost-server" && pr.RepoName != "mattermost-webapp" {
		return
	}

	request := s.updateSpinWick(pr, withLicense)
	if request.Error != nil {
		if request.Aborted {
			mlog.Warn("Aborted update of SpinWick", mlog.String("abort_message", request.Error.Error()), mlog.String("repo_name", pr.RepoName), mlog.Int("pr", pr.Number), mlog.String("installation_id", request.InstallationID))
		} else {
			mlog.Error("Failed to update SpinWick", mlog.Err(request.Error), mlog.String("repo_name", pr.RepoName), mlog.Int("pr", pr.Number), mlog.String("installation_id", request.InstallationID))
		}
		s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number, s.Config.SetupSpinmintFailedMessage)
		if request.ReportError {
			additionalFields := map[string]string{
				"Installation ID": request.InstallationID,
			}
			s.logPrettyErrorToMattermost("[ SpinWick ] Update Failed", pr, request.Error, additionalFields)
		}
	}
}

// updateSpinWick updates a SpinWick with the following behavior:
// - no cloud installation found = error is returned
// - cloud installation found and updated = actual ID string and no error
// - any errors = error is returned
func (s *Server) updateSpinWick(pr *model.PullRequest, withLicense bool) *spinwick.Request {
	request := &spinwick.Request{
		InstallationID: "n/a",
		Error:          nil,
		ReportError:    false,
		Aborted:        false,
	}

	ownerID := makeSpinWickID(pr.RepoName, pr.Number)
	id, err := cloudtools.GetInstallationIDFromOwnerID(s.Config.ProvisionerServer, s.Config.AwsAPIKey, ownerID)
	if err != nil {
		return request.WithError(err).ShouldReportError()
	}
	if id == "" {
		return request.WithError(fmt.Errorf("no installation found with owner %s", ownerID)).ShouldReportError()
	}
	request.InstallationID = id

	mlog.Info("Sleeping a bit to wait for the build process to start", mlog.Int("pr", pr.Number), mlog.String("sha", pr.Sha))
	time.Sleep(60 * time.Second)

	// Remove old message to reduce the amount of similar messages and avoid confusion
	serverNewCommitMessages := []string{
		"New commit detected.",
	}
	comments, errComments := s.getComments(pr.RepoOwner, pr.RepoName, pr.Number)
	if errComments != nil {
		mlog.Error("pr_error", mlog.Err(err))
	} else {
		s.removeCommentsWithSpecificMessages(comments, serverNewCommitMessages, pr)
	}
	s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number, "New commit detected. SpinWick will upgrade if the updated docker image is available.")

	reg, err := s.Builds.dockerRegistryClient(s)
	if err != nil {
		return request.WithError(errors.Wrap(err, "unable to get docker registry client")).ShouldReportError()
	}

	mlog.Info("Waiting for docker image to update SpinWick", mlog.Int("pr", pr.Number), mlog.String("repo_owner", pr.RepoOwner), mlog.String("repo_name", pr.RepoName))

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	defer cancel()

	pr, err = s.Builds.waitForImage(ctx, s, reg, pr)
	if err != nil {
		return request.WithError(errors.Wrap(err, "error waiting for the docker image. Aborting")).IntentionalAbort()
	}

	upgradeRequest := &cloudModel.UpgradeInstallationRequest{
		Version: s.Builds.getInstallationVersion(pr),
	}
	if withLicense {
		upgradeRequest.License = s.Config.SpinWickHALicense
	}

	// Final upgrade check
	// Let's get the installation state one last time. If the version matches
	// what we want then another process already updated it.
	headers := map[string]string{
		"x-api-key": s.Config.AwsAPIKey,
	}
	cloudClient := cloudModel.NewClientWithHeaders(s.Config.ProvisionerServer, headers)
	installation, err := cloudClient.GetInstallation(request.InstallationID)
	if err != nil {
		return request.WithError(errors.Wrap(err, "unable to get installation")).ShouldReportError()
	}
	if installation.Version == upgradeRequest.Version {
		return request.WithError(errors.New("another process already updated the installation version. Aborting")).IntentionalAbort()
	}

	mlog.Info("Provisioning Server - Upgrade request", mlog.String("SHA", pr.Sha))

	err = cloudClient.UpgradeInstallation(request.InstallationID, upgradeRequest)
	if err != nil {
		return request.WithError(errors.Wrap(err, "unable to make upgrade request to provisioning server")).ShouldReportError()
	}

	wait := 600
	mlog.Info("Waiting for mattermost installation to become stable", mlog.Int("wait_seconds", wait))
	ctx, cancel = context.WithTimeout(context.Background(), time.Duration(wait)*time.Second)
	defer cancel()

	s.waitForInstallationStable(ctx, pr, request)
	if request.Error != nil {
		return request.WithError(errors.Wrap(request.Error, "error waiting for installation to become stable"))
	}

	// Remove old message to reduce the amount of similar messages and avoid confusion
	if errComments == nil {
		serverUpdateMessage := []string{
			"Mattermost test server updated",
		}
		s.removeCommentsWithSpecificMessages(comments, serverUpdateMessage, pr)
	}

	mmURL := fmt.Sprintf("https://%s.%s", makeSpinWickID(pr.RepoName, pr.Number), s.Config.DNSNameTestServer)
	msg := fmt.Sprintf("Mattermost test server updated with git commit `%s`.\n\nAccess here: %s", pr.Sha, mmURL)
	s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number, msg)

	return request
}

func (s *Server) handleDestroySpinWick(pr *model.PullRequest) {
	request := s.destroySpinWick(pr)
	if request.Error != nil {
		if request.Aborted {
			mlog.Warn("Aborted deletion of SpinWick", mlog.String("abort_message", request.Error.Error()), mlog.String("repo_name", pr.RepoName), mlog.Int("pr", pr.Number), mlog.String("installation_id", request.InstallationID))
		} else {
			mlog.Error("Failed to delete SpinWick", mlog.Err(request.Error), mlog.String("repo_name", pr.RepoName), mlog.Int("pr", pr.Number), mlog.String("installation_id", request.InstallationID))
		}
		if request.ReportError {
			additionalFields := map[string]string{
				"Installation ID": request.InstallationID,
			}
			s.logPrettyErrorToMattermost("[ SpinWick ] Destroy Failed", pr, request.Error, additionalFields)
		}
	}
}

// destroySpinwick destroys a SpinWick with the following behavior:
// - no cloud installation found = empty ID string and no error
// - cloud installation found and deleted = actual ID string and no error
// - any errors = error is returned
func (s *Server) destroySpinWick(pr *model.PullRequest) *spinwick.Request {
	request := &spinwick.Request{
		InstallationID: "n/a",
		Error:          nil,
		ReportError:    false,
		Aborted:        false,
	}

	ownerID := makeSpinWickID(pr.RepoName, pr.Number)
	id, err := cloudtools.GetInstallationIDFromOwnerID(s.Config.ProvisionerServer, s.Config.AwsAPIKey, ownerID)
	if err != nil {
		return request.WithError(err).ShouldReportError()
	}
	if id == "" {
		return request.WithInstallationID(id).WithError(errors.New("No SpinWick found for this PR. Skipping deletion")).IntentionalAbort()
	}
	request.InstallationID = id

	mlog.Info("Destroying SpinWick", mlog.Int("pr", pr.Number), mlog.String("repo_owner", pr.RepoOwner), mlog.String("repo_name", pr.RepoName), mlog.String("installation_id", request.InstallationID))

	headers := map[string]string{
		"x-api-key": s.Config.AwsAPIKey,
	}
	cloudClient := cloudModel.NewClientWithHeaders(s.Config.ProvisionerServer, headers)
	err = cloudClient.DeleteInstallation(request.InstallationID)
	if err != nil {
		return request.WithError(errors.Wrap(err, "unable to make installation delete request to provisioning server")).ShouldReportError()
	}

	// Old comments created by Mattermod user will be deleted here.
	s.commentLock.Lock()
	defer s.commentLock.Unlock()

	comments, _, err := NewGithubClient(s.Config.GithubAccessToken).Issues.ListComments(context.Background(), pr.RepoOwner, pr.RepoName, pr.Number, nil)
	if err != nil {
		return request.WithError(errors.Wrap(err, "unable to get list of old comments")).ShouldReportError()
	}
	s.removeOldComments(comments, pr)

	s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number, s.Config.DestroyedSpinmintMessage)

	return request
}

func (s *Server) waitForInstallationStable(ctx context.Context, pr *model.PullRequest, request *spinwick.Request) {
	channel, err := s.requestCloudWebhookChannel(request.InstallationID)
	if err != nil {
		request.WithError(err).ShouldReportError()
		return
	}
	defer s.removeCloudWebhookChannel(request.InstallationID)

	for {
		select {
		case <-ctx.Done():
			request.WithError(errors.New("timed out waiting for the mattermost installation to stabilize")).ShouldReportError()
			return
		case payload := <-channel:
			if payload.ID != request.InstallationID {
				continue
			}

			mlog.Info("Installation changed state", mlog.String("installation", request.InstallationID), mlog.String("state", payload.NewState))

			switch payload.NewState {
			case cloudModel.InstallationStateStable:
				return
			case cloudModel.InstallationStateCreationFailed:
				request.WithError(errors.New("the installation creation failed")).ShouldReportError()
				return
			case cloudModel.InstallationStateDeletionRequested,
				cloudModel.InstallationStateDeletionInProgress,
				cloudModel.InstallationStateDeleted:
				// Another process may have deleted the installation. Let's check.
				pr, err = s.GetUpdateChecks(pr.RepoOwner, pr.RepoName, pr.Number)
				if err != nil {
					request.WithError(errors.Wrapf(err, "received state update %s, but was unable to check PR labels", payload.NewState)).ShouldReportError()
					return
				}
				if !s.isSpinWickLabelInLabels(pr.Labels) {
					request.WithError(errors.New("the SpinWick label has been removed. Aborting")).IntentionalAbort()
					return
				}
			case cloudModel.InstallationStateCreationNoCompatibleClusters:
				err := s.requestK8sClusterCreation(pr)
				if err != nil {
					request.WithError(errors.Wrap(err, "unable to create a new cluster to accommodate the installation")).ShouldReportError()
					return
				}
				// This sleep is a bit hacky, but is intended to ensure that the
				// installation has time to be worked on before we check its state
				// again so we don't create another cluster needlessly.
				time.Sleep(30 * time.Second)
			}
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

		var clusterRequest cloudModel.Cluster
		err = json.NewDecoder(resp.Body).Decode(&clusterRequest)
		if err != nil && err != io.EOF {
			mlog.Error("Error decoding cluster response", mlog.Err(err))
		}
		if clusterRequest.State == "stable" {
			s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number, "Kubernetes cluster created. Now will deploy Mattermost... Hang on!")
			return nil
		} else if clusterRequest.State == "creation-failed" {
			s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number, "Failed to create the k8s cluster.")
			return errors.New("error creating k8s cluster")
		}
		mlog.Info("Provisioner Server - cluster request creating... sleep", mlog.String("ClusterID", clusterRequest.ID), mlog.String("State", clusterRequest.State))
		time.Sleep(20 * time.Second)
		select {
		case <-ctx.Done():
			s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number, "Timed out waiting for the kubernetes cluster. Please check the logs.")
			return errors.New("timed out waiting for the cluster installation complete")
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

	wait := 600
	mlog.Info("Waiting up to 600 seconds for DNS to propagate")
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
	// removed in 5.16
	//config.ServiceSettings.ExperimentalLdapGroupSync = NewBool(true)
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
	s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number, "Please wait while a new kubernetes cluster is created for your SpinWick")

	clusterRequest := cloudModel.CreateClusterRequest{
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

	var cluster cloudModel.Cluster
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

func (s *Server) isSpinWickHALabel(labels []string) bool {
	for _, label := range labels {
		if label == s.Config.SetupSpinWickHA {
			return true
		}
	}
	return false
}

func (s *Server) removeCommentsWithSpecificMessages(comments []*github.IssueComment, serverMessages []string, pr *model.PullRequest) {
	mlog.Info("Removing old spinwick Mattermod comments")
	for _, comment := range comments {
		if *comment.User.Login == s.Config.Username {
			for _, message := range serverMessages {
				if strings.Contains(*comment.Body, message) {
					mlog.Info("Removing old spinwick comment with ID", mlog.Int64("ID", *comment.ID))
					_, err := NewGithubClient(s.Config.GithubAccessToken).Issues.DeleteComment(context.Background(), pr.RepoOwner, pr.RepoName, *comment.ID)
					if err != nil {
						mlog.Error("Unable to remove old spinwick Mattermod comment", mlog.Err(err))
					}
					break
				}
			}
		}
	}
}
