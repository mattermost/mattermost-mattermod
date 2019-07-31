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

	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/mlog"
	mattermostModel "github.com/mattermost/mattermost-server/model"
	"github.com/pkg/errors"
)

// The following structs are copied from the mattermost-cloud repo to allow
// mattermod to interact with provisioning servers.
//
// TODO: consider moving the structs in mattermost-cloud for these models out
// of the /internal directory so that they can be vendored and imported here.
// When doing this, we should start using semver in the mattermost-cloud repo.

// CreateClusterRequest specifies the parameters for a new cluster.
type CreateClusterRequest struct {
	Provider string
	Size     string
	Zones    []string
}

// Cluster represents a Kubernetes cluster.
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

// CreateInstallationRequest specifies the parameters for a new installation.
type CreateInstallationRequest struct {
	OwnerID  string
	Version  string
	DNS      string
	Size     string
	Affinity string
}

// Installation represents a Mattermost installation.
type Installation struct {
	ID             string
	OwnerID        string
	Version        string
	DNS            string
	Size           string
	Affinity       string
	GroupID        *string
	State          string
	CreateAt       int64
	DeleteAt       int64
	LockAcquiredBy *string
	LockAcquiredAt int64
}

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

		return
	}

	if installationID != "" {
		s.storeSpinmintInfo(&model.Spinmint{
			InstanceId: installationID,
			RepoOwner:  pr.RepoOwner,
			RepoName:   pr.RepoName,
			Number:     pr.Number,
			CreatedAt:  time.Now().UTC().Unix(),
		})
	}
}

// createSpinWick creates a SpinWick and returns an error as well as a bool
// indicating if the error should be logged to Mattermost.
func (s *Server) createSpinWick(pr *model.PullRequest, size string) (string, bool, error) {
	installationID := "n/a"

	result := <-s.Store.Spinmint().Get(pr.Number, pr.RepoName)
	if result.Err != nil {
		return installationID, true, errors.Wrap(result.Err, "unable to get the SpinWick information from database")
	}
	if result.Data != nil {
		mlog.Info("PR already has a SpinWick created", mlog.String("repo_name", pr.RepoName), mlog.Int("pr", pr.Number))
		return installationID, false, nil
	}

	mlog.Info("No SpinWick for this PR in the database. Creating a new one.")

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

	// Check the mattermod store again just in case a separate process created
	// a SpinWick while we waited for the build to finish.
	//
	// TODO: this should be improved in the future.
	result = <-s.Store.Spinmint().Get(pr.Number, pr.RepoName)
	if result.Err != nil {
		return installationID, true, errors.Wrap(result.Err, "unable to get the SpinWick information from database")
	}
	if result.Data != nil {
		return installationID, true, errors.New("More than a single process was trying to create a SpinWick")
	}

	mlog.Info("Provisioning Server - Installation request")

	spinwickID := makeSpinWickID(pr.RepoName, pr.Number)
	installationRequest := CreateInstallationRequest{
		OwnerID:  spinwickID,
		Version:  pr.Sha[0:7],
		DNS:      fmt.Sprintf("%s.%s", spinwickID, s.Config.DNSNameTestServer),
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

	var installation Installation
	err = json.NewDecoder(respReqInstallation.Body).Decode(&installation)
	if err != nil && err != io.EOF {
		return installationID, true, errors.Wrap(err, "error decoding installation")
	}
	installationID = installation.ID
	mlog.Info("Provisioner Server - installation request", mlog.String("InstallationID", installationID))

	time.Sleep(3 * time.Second)
	// Get the installaion to check if the state is creation-no-compatible-clusters
	// if is that state we need to requst a new k8s cluster
	// TODO:
	// There is no garauntee that the installation has been worked on yet. We
	// may have to wait longer for it to enter the creation-no-compatible-clusters
	// state.
	url = fmt.Sprintf("%s/api/installation/%s", s.Config.ProvisionerServer, installationID)
	resp, err := makeRequest("GET", url, nil)
	if err != nil {
		return installationID, true, errors.Wrap(err, "error getting the mattermost installation")
	}
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&installation)
	if err != nil && err != io.EOF {
		return installationID, true, errors.Wrap(err, "error decoding installation")
	}
	if installation.State == "creation-no-compatible-clusters" {
		err = s.requestK8sClusterCreation(pr)
		if err != nil {
			return installationID, true, errors.Wrap(err, "unable to create a new cluster to accommodate the installation")
		}
	}

	wait := 480
	mlog.Info("Waiting up to 480 seconds for the mattermost installation to complete...")
	ctx, cancel = context.WithTimeout(context.Background(), time.Duration(wait)*time.Second)
	defer cancel()
	err = s.waitMattermostInstallation(ctx, pr, installationID, false)
	if err != nil {
		return installationID, true, errors.Wrap(err, "error waiting for installation to become stable")
	}

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

// udpateSpinWick updates a SpinWick and returns an error as well as a bool
// indicating if the error should be logged to Mattermost.
func (s *Server) updateSpinWick(pr *model.PullRequest) (string, bool, error) {
	installationID := "n/a"
	foundLabel := false
	for _, label := range pr.Labels {
		if s.isSpinWickLabel(label) {
			mlog.Info("PR has a SpinWick label; proceeding with upgrade", mlog.Int("pr", pr.Number))
			foundLabel = true
			break
		}
	}

	if !foundLabel {
		return installationID, false, nil
	}

	result := <-s.Store.Spinmint().Get(pr.Number, pr.RepoName)
	if result.Err != nil {
		return installationID, true, errors.Wrap(result.Err, "unable to get SpinWick information from the database")
	}
	if result.Data == nil {
		return installationID, true, errors.New("no SpinWick database information found")
	}
	installationID = result.Data.(*model.Spinmint).InstanceId

	// TODO: add a new column in the db to get the previous job and wait for the new one start
	// for now will sleep some time
	mlog.Info("Sleeping a bit to wait for the build process start", mlog.Int("pr", pr.Number), mlog.String("sha", pr.Sha))
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
	mlog.Info("Build Link updated", mlog.String("buildLink", buildLink), mlog.String("OldBuildLink", pr.BuildLink))
	pr.BuildLink = buildLink
	result = <-s.Store.PullRequest().Save(pr)
	if result.Err != nil {
		return installationID, true, errors.Wrap(result.Err, "unable to save updated PR to the database")
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

	// TODO: remove this when we starting building the docker image in the sam build pipeline
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
	err = s.waitMattermostInstallation(ctx, pr, installationID, true)
	if err != nil {
		return installationID, true, errors.Wrap(err, "encountered error waiting for installation to become stable")
	}

	return installationID, false, nil
}

func (s *Server) handleDestroySpinWick(pr *model.PullRequest, installationID string) {
	mlog.Info("Destroying SpinWick", mlog.Int("pr", pr.Number), mlog.String("repo_owner", pr.RepoOwner), mlog.String("repo_name", pr.RepoName), mlog.String("installation_id", installationID))

	sendMattermostLog, err := s.destroyMMInstallation(installationID)
	if err != nil {
		mlog.Error("Failed to delete Mattermost installation", mlog.Err(err), mlog.String("repo_name", pr.RepoName), mlog.Int("pr", pr.Number), mlog.String("installation_id", installationID))
		if sendMattermostLog {
			additionalFields := map[string]string{
				"Installation ID": installationID,
			}
			s.logPrettyErrorToMattermost("[ SpinWick ] Destroy Failed", pr, err, additionalFields)
		}
	}

	s.removeTestServerFromDB(installationID)
}

// destroyMMInstallation destroys a SpinWick and returns an error as well as a bool
// indicating if the error should be logged to Mattermost.
func (s *Server) destroyMMInstallation(instanceClusterID string) (bool, error) {
	url := fmt.Sprintf("%s/api/installation/%s", s.Config.ProvisionerServer, instanceClusterID)
	resp, err := makeRequest("DELETE", url, nil)
	if err != nil {
		return true, errors.Wrap(err, "unable to make installation delete request to provisioning server")
	}
	defer resp.Body.Close()

	return false, nil
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

func (s *Server) waitMattermostInstallation(ctx context.Context, pr *model.PullRequest, installationRequestID string, upgrade bool) error {
	for {
		url := fmt.Sprintf("%s/api/installation/%s", s.Config.ProvisionerServer, installationRequestID)
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
			return fmt.Errorf("Error decoding installation: %s", err)
		}
		if installationRequest.State == "stable" {
			mmURL := fmt.Sprintf("https://%s.%s", makeSpinWickID(pr.RepoName, pr.Number), s.Config.DNSNameTestServer)
			if !upgrade {
				userErr := s.initializeMattermostTestServer(mmURL, pr.Number)
				if userErr != nil {
					s.commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, "Failed to create mattermost installation.")
					s.destroyMMInstallation(installationRequest.ID)
					return nil
				}
				userTable := "| Account Type | Username | Password |\n|---|---|---|\n| Admin | sysadmin | Sys@dmin123 |\n| User | user-1 | User-1@123 |"
				msg := fmt.Sprintf("Mattermost test server created! :tada:\n\nAccess here: %s\n\n%s", mmURL, userTable)
				s.commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, msg)
			} else {
				msg := fmt.Sprintf("Mattermost test server updated!\n\nAccess here: %s", mmURL)
				s.commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, msg)
			}
			return nil
		} else if installationRequest.State == "creation-failed" {
			s.commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, "Failed to create mattermost installation.")
			s.destroyMMInstallation(installationRequest.ID)
			return fmt.Errorf("error creating mattermost installation")
		}
		mlog.Info("Provisioner Server - installation request creating... sleep", mlog.String("InstallationID", installationRequest.ID), mlog.String("State", installationRequest.State))
		select {
		case <-ctx.Done():
			s.destroyMMInstallation(installationRequest.ID)
			s.commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, "Timed out waiting for the mattermost installation. Please check the logs.")
			return fmt.Errorf("timed out waiting for the mattermost installation complete. requesting the deletion")
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

		var clusterRequest Cluster
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
	mlog.Info("Will check if can ping the new DNS otherwise will wait for the DNS propagation for 5 minutes")
	wait := 300
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(wait)*time.Second)
	defer cancel()

	mmHost, _ := url.Parse(mmURL)
	err := checkDNS(ctx, fmt.Sprintf("%s:443", mmHost.Host))
	if err != nil {
		mlog.Info("URL not accessible")
		return err
	}

	mlog.Info("Will create the initial user")
	client := mattermostModel.NewAPIv4Client(mmURL)

	//check if Mattermost is available
	ctx, cancel = context.WithTimeout(context.Background(), time.Duration(wait)*time.Second)
	defer cancel()
	err = checkMMPing(ctx, client)
	if err != nil {
		return err
	}

	user := &mattermostModel.User{
		Username: "sysadmin",
		Email:    "sysadmin@example.mattermost.com",
		Password: "Sys@dmin123",
	}
	_, response := client.CreateUser(user)
	if response.StatusCode != 201 {
		mlog.Error("Error creating the initial user", mlog.Int("StatusCode", response.StatusCode), mlog.String("Message", response.Error.Message))
		return fmt.Errorf(response.Error.Message)
	}
	mlog.Info("Done the creation of the initial user")

	mlog.Info("Logging into MM")
	client.Logout()
	userLogged, response := client.Login("sysadmin", "Sys@dmin123")
	if response.StatusCode != 200 {
		mlog.Error("Error logging with the initial user", mlog.Int("StatusCode", response.StatusCode), mlog.String("Message", response.Error.Message))
		return fmt.Errorf(response.Error.Message)
	}
	mlog.Info("Done logging into MM")

	mlog.Info("Creating new Team")
	teamName := fmt.Sprintf("pr%d", prNumber)
	team := &mattermostModel.Team{
		Name:        teamName,
		DisplayName: teamName,
		Type:        "O",
	}
	firstTeam, response := client.CreateTeam(team)
	if response.StatusCode != 201 {
		mlog.Error("Error creating the initial team", mlog.Int("StatusCode", response.StatusCode))
	}
	mlog.Info("Done creating new Team and will update the config")

	_, response = client.AddTeamMember(firstTeam.Id, userLogged.Id)
	if response.StatusCode != 201 {
		mlog.Error("Error adding sysadmin to the initial team", mlog.Int("StatusCode", response.StatusCode))
	}

	// Create test user-1
	testUser := &mattermostModel.User{
		Username: "user-1",
		Email:    "user-1@example.mattermost.com",
		Password: "User-1@123",
	}
	testUser, response = client.CreateUser(testUser)
	if response.StatusCode != 201 {
		mlog.Error("Error creating the initial test user", mlog.Int("StatusCode", response.StatusCode), mlog.String("Message", response.Error.Message))
	}
	_, response = client.AddTeamMember(firstTeam.Id, testUser.Id)
	if response.StatusCode != 201 {
		mlog.Error("Error adding test user to the initial team", mlog.Int("StatusCode", response.StatusCode))
	}

	config, response := client.GetConfig()
	if response.StatusCode != 200 {
		mlog.Error("Error getting the config ", mlog.Int("StatusCode", response.StatusCode), mlog.String("Message", response.Error.Message))
		return fmt.Errorf(response.Error.Message)
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
		mlog.Error("Error setting the config ", mlog.Int("StatusCode", response.StatusCode), mlog.String("Message", response.Error.Message))
		return fmt.Errorf(response.Error.Message)
	}

	mlog.Info("Done update the s.Config. All good.")

	return nil
}

func (s *Server) requestK8sClusterCreation(pr *model.PullRequest) error {
	mlog.Info("Building new kubernetes cluster")

	url := fmt.Sprintf("%s/api/clusters", s.Config.ProvisionerServer)
	s.commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, "Please wait while a new kubernetes cluster is created for your SpinWick")

	clusterRequest := CreateClusterRequest{
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

	var cluster Cluster
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
			mlog.Debug("URL reachable", mlog.String("URL", url))
			return nil
		}
		select {
		case <-ctx.Done():
			mlog.Error("Timeout while checking the URL. URL not reachabled", mlog.String("URL", url))
			return fmt.Errorf("Timeout while checking the URL. URL not reachabled")
		case <-time.After(10 * time.Second):
			mlog.Debug("not reachabled, will sleep 10 seconds", mlog.String("URL", url))
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
			mlog.Error("Timeout while checking mattermost")
			return fmt.Errorf("Timeout while checking mattermost")
		case <-time.After(10 * time.Second):
			mlog.Debug("cannot get the mattermost ping, waiting a bit more")
		}
	}
}

func makeSpinWickID(repoName string, prNumber int) string {
	return strings.ToLower(fmt.Sprintf("%s-pr-%d", repoName, prNumber))
}

func (s *Server) isSpinWickLabel(label string) bool {
	return label == s.Config.SetupSpinWick || label == s.Config.SetupSpinWickHA
}
