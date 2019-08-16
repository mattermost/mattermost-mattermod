// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/braintree/manners"
	"github.com/google/go-github/github"
	"github.com/gorilla/mux"
	cloudModel "github.com/mattermost/mattermost-cloud/model"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-mattermod/store"
	"github.com/mattermost/mattermost-server/mlog"
	"github.com/mattermost/mattermost-server/utils/fileutils"
)

// Server is the mattermod server.
type Server struct {
	Config *ServerConfig
	Store  store.Store
	Router *mux.Router

	webhookChannelsLock sync.Mutex
	webhookChannels     map[string]chan cloudModel.WebhookPayload

	Builds buildsInterface

	commentLock sync.Mutex

	StartTime time.Time
}

const (
	INSTANCE_ID_MESSAGE = "Instance ID: "
	LOG_FILENAME        = "mattermod.log"

	// buildOverride overrides the buildsInterface of the server for development
	// and testing.
	buildOverride = "MATTERMOD_BUILD_OVERRIDE"
)

var (
	INSTANCE_ID_PATTERN = regexp.MustCompile(INSTANCE_ID_MESSAGE + "(i-[a-z0-9]+)")
	INSTANCE_ID         = "INSTANCE_ID"
	INTERNAL_IP         = "INTERNAL_IP"
	SPINMINT_LINK       = "SPINMINT_LINK"
)

// New returns a new server with the desired configuration
func New(config *ServerConfig) *Server {
	s := &Server{
		Config:          config,
		Store:           store.NewSqlStore(config.DriverName, config.DataSource),
		Router:          mux.NewRouter(),
		webhookChannels: make(map[string]chan cloudModel.WebhookPayload),
		StartTime:       time.Now(),
	}

	s.Builds = &Builds{}
	if os.Getenv(buildOverride) != "" {
		mlog.Warn("Using mocked build tools")
		s.Builds = &MockedBuilds{
			Version: os.Getenv(buildOverride),
		}
	}

	return s
}

// Start starts a server
func (s *Server) Start() {
	mlog.Info("Starting Mattermod Server")

	rand.Seed(time.Now().Unix())

	s.initializeRouter()

	var handler http.Handler = s.Router
	go func() {
		mlog.Info("Listening on", mlog.String("address", s.Config.ListenAddress))
		err := manners.ListenAndServe(s.Config.ListenAddress, handler)
		if err != nil {
			s.logErrorToMattermost(err.Error())
			mlog.Critical("server_error", mlog.Err(err))
			panic(err.Error())
		}
	}()
}

// Stop stops a server
func (s *Server) Stop() {
	mlog.Info("Stopping Mattermod")
	manners.Close()
}

// Tick runs a check on objects in the database
func (s *Server) Tick() {
	mlog.Info("tick")

	aboveLimit := s.CheckLimitRateAndAbortRequest()
	if aboveLimit {
		return
	}

	client := NewGithubClient(s.Config.GithubAccessToken)

	for _, repository := range s.Config.Repositories {
		ghPullRequests, _, err := client.PullRequests.List(context.Background(), repository.Owner, repository.Name, &github.PullRequestListOptions{
			State: "open",
		})
		if err != nil {
			mlog.Error("Failed to get PRs", mlog.Err(err), mlog.String("repo_owner", repository.Owner), mlog.String("repo_name", repository.Name))
			continue
		}

		for _, ghPullRequest := range ghPullRequests {
			pullRequest, errPR := s.GetPullRequestFromGithub(ghPullRequest)
			if errPR != nil {
				mlog.Error("failed to convert PR", mlog.Int("pr", *ghPullRequest.Number), mlog.Err(errPR))
				continue
			}

			s.checkPullRequestForChanges(pullRequest)
		}

		issues, _, err := client.Issues.ListByRepo(context.Background(), repository.Owner, repository.Name, &github.IssueListByRepoOptions{
			State: "open",
		})
		if err != nil {
			mlog.Error("Failed to get issues", mlog.Err(err), mlog.String("repo_owner", repository.Owner), mlog.String("repo_name", repository.Name))
			continue
		}

		for _, ghIssue := range issues {
			if ghIssue.PullRequestLinks != nil {
				// This is a PR so we've already checked it
				continue
			}

			issue, err := s.GetIssueFromGithub(repository.Owner, repository.Name, ghIssue)
			if err != nil {
				mlog.Error("failed to convert issue", mlog.Int("issue", *ghIssue.Number), mlog.Err(err))
				continue
			}

			s.checkIssueForChanges(issue)
		}
	}
}

func (s *Server) initializeRouter() {
	s.Router.HandleFunc("/", s.ping).Methods("GET")
	s.Router.HandleFunc("/pr_event", s.githubEvent).Methods("POST")
	s.Router.HandleFunc("/cloud_webhooks", s.handleCloudWebhook).Methods("POST")
	s.Router.HandleFunc("/list_prs", s.listPrs).Methods("GET")
	s.Router.HandleFunc("/list_issues", s.listIssues).Methods("GET")
	s.Router.HandleFunc("/list_spinmints", s.listTestServers).Methods("GET")
	s.Router.HandleFunc("/delete_test_server", s.deleteTestServer).Methods("DELETE")
	s.Router.HandleFunc("/shrug_wick", s.serveShrugWick).Methods("GET")
}

func (s *Server) ping(w http.ResponseWriter, r *http.Request) {
	msg := fmt.Sprintf("{\"uptime\": \"%v\"}", time.Since(s.StartTime))
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(msg))
}

func (s *Server) githubEvent(w http.ResponseWriter, r *http.Request) {
	overLimit := s.CheckLimitRateAndAbortRequest()
	if overLimit {
		return
	}

	buf, _ := ioutil.ReadAll(r.Body)
	event := PullRequestEventFromJson(ioutil.NopCloser(bytes.NewBuffer(buf)))
	eventIssueComment := IssueCommentFromJson(ioutil.NopCloser(bytes.NewBuffer(buf)))

	if event != nil && event.PRNumber != 0 {
		mlog.Info("pr event", mlog.Int("pr", event.PRNumber), mlog.String("action", event.Action))
		s.handlePullRequestEvent(event)
		return
	}

	if eventIssueComment != nil && eventIssueComment.Action == "created" {
		if strings.Contains(strings.TrimSpace(*eventIssueComment.Comment.Body), "/check-cla") {
			s.handleCheckCLA(*eventIssueComment)
		}
		if strings.Contains(strings.TrimSpace(*eventIssueComment.Comment.Body), "/cherry-pick") {
			s.handleCherryPick(*eventIssueComment)
		}
		if strings.Contains(strings.TrimSpace(*eventIssueComment.Comment.Body), "/shrugwick") {
			s.handleShrugWick(*eventIssueComment)
		}
		if strings.Contains(strings.TrimSpace(*eventIssueComment.Comment.Body), "/autoassign") {
			s.handleAutoassign(*eventIssueComment)
		}
		return
	}

	s.handleIssueEvent(event)
}

func (s *Server) handleCloudWebhook(w http.ResponseWriter, r *http.Request) {
	payload, err := cloudModel.WebhookPayloadFromReader(r.Body)
	if err != nil {
		mlog.Error("Received webhook event, but couldn't parse the payload")
		return
	}
	defer r.Body.Close()

	payloadClone := *payload

	s.webhookChannelsLock.Lock()
	mlog.Debug("Received cloud webhook payload", mlog.Int("channels", len(s.webhookChannels)), mlog.String("payload", fmt.Sprintf("%+v", payloadClone)))
	for _, channel := range s.webhookChannels {
		go func(ch chan cloudModel.WebhookPayload, p cloudModel.WebhookPayload) {
			select {
			case ch <- p:
			case <-time.After(5 * time.Second):
			}
		}(channel, payloadClone)
	}
	s.webhookChannelsLock.Unlock()
}

func (s *Server) listPrs(w http.ResponseWriter, r *http.Request) {
	result := <-s.Store.PullRequest().List()
	if result.Err != nil {
		mlog.Error("Error getting list of pull requests", mlog.Err(result.Err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	prs := result.Data.([]*model.PullRequest)

	b, err := json.Marshal(prs)
	if err != nil {
		mlog.Error("Error marshalling pull requests", mlog.Err(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
}

func (s *Server) listIssues(w http.ResponseWriter, r *http.Request) {
	result := <-s.Store.Issue().List()
	if result.Err != nil {
		mlog.Error("Error getting list of github issues", mlog.Err(result.Err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	issues := result.Data.([]*model.Issue)

	b, err := json.Marshal(issues)
	if err != nil {
		mlog.Error("Error marshalling github issues", mlog.Err(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
}

func (s *Server) listTestServers(w http.ResponseWriter, r *http.Request) {
	result := <-s.Store.Spinmint().List()
	if result.Err != nil {
		mlog.Error("Error getting list of test servers", mlog.Err(result.Err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	spinmints := result.Data.([]*model.Spinmint)

	b, err := json.Marshal(spinmints)
	if err != nil {
		mlog.Error("spinmint_error", mlog.Err(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
}

func (s *Server) deleteTestServer(w http.ResponseWriter, r *http.Request) {
	instance := r.FormValue("instance")
	token := r.FormValue("token")

	if token != s.Config.TokenToDeleteTestServers {
		mlog.Error("Error deleting test server - invalid token")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	result := <-s.Store.Spinmint().GetTestServer(instance)
	if result.Err != nil {
		mlog.Error("Error deleting test server", mlog.Err(result.Err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	testServer := result.Data.(*model.Spinmint)

	result = <-s.Store.PullRequest().Get(testServer.RepoOwner, testServer.RepoName, testServer.Number)
	if result.Err != nil {
		mlog.Error("Error deleting test server", mlog.Err(result.Err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	pr := result.Data.(*model.PullRequest)

	if strings.Contains(testServer.InstanceId, "i-") {
		s.destroySpinmint(pr, testServer.InstanceId)
	}

	s.handleDestroySpinWick(pr)

	w.WriteHeader(http.StatusOK)
}

func messageByUserContains(comments []*github.IssueComment, username string, text string) bool {
	for _, comment := range comments {
		if *comment.User.Login == username && strings.Contains(*comment.Body, text) {
			return true
		}
	}

	return false
}

func GetLogFileLocation(fileLocation string) string {
	if fileLocation == "" {
		fileLocation, _ = fileutils.FindDir("logs")
	}

	return filepath.Join(fileLocation, LOG_FILENAME)
}

func SetupLogging(config *ServerConfig) {
	loggingConfig := &mlog.LoggerConfiguration{
		EnableConsole: config.LogSettings.EnableConsole,
		ConsoleJson:   config.LogSettings.ConsoleJson,
		ConsoleLevel:  strings.ToLower(config.LogSettings.ConsoleLevel),
		EnableFile:    config.LogSettings.EnableFile,
		FileJson:      config.LogSettings.FileJson,
		FileLevel:     strings.ToLower(config.LogSettings.FileLevel),
		FileLocation:  GetLogFileLocation(config.LogSettings.FileLocation),
	}

	logger := mlog.NewLogger(loggingConfig)
	mlog.RedirectStdLog(logger)
	mlog.InitGlobalLogger(logger)
}
