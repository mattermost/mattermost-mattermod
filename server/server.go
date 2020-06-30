// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/v31/github"
	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost-mattermod/store"
	"github.com/mattermost/mattermost-server/v5/mlog"
	"github.com/mattermost/mattermost-server/v5/utils/fileutils"
)

// Server is the mattermod server.
type Server struct {
	Config               *Config
	Store                store.Store
	GithubClient         *GithubClient
	OrgMembers           []string
	Builds               buildsInterface
	commentLock          sync.Mutex
	StartTime            time.Time
	hasReportedRateLimit bool

	server *http.Server
}

type pingResponse struct {
	Uptime string `json:"uptime"`
}

const (
	instanceIDMessage = "Instance ID: "
	logFilename       = "mattermod.log"

	// buildOverride overrides the buildsInterface of the server for development
	// and testing.
	buildOverride = "MATTERMOD_BUILD_OVERRIDE"

	templateSpinmintLink = "SPINMINT_LINK"
	templateInstanceID   = "INSTANCE_ID"
	templateInternalIP   = "INTERNAL_IP"
)

func New(config *Config) *Server {
	s := &Server{
		Config:               config,
		Store:                store.NewSQLStore(config.DriverName, config.DataSource),
		StartTime:            time.Now(),
		hasReportedRateLimit: false,
	}

	s.GithubClient = NewGithubClient(s.Config.GithubAccessToken)

	s.Builds = &Builds{}
	if os.Getenv(buildOverride) != "" {
		mlog.Warn("Using mocked build tools")
		s.Builds = &MockedBuilds{
			Version: os.Getenv(buildOverride),
		}
	}

	r := mux.NewRouter()
	r.HandleFunc("/", s.ping).Methods(http.MethodGet)
	r.HandleFunc("/pr_event", s.githubEvent).Methods(http.MethodPost)

	s.server = &http.Server{
		Addr:         s.Config.ListenAddress,
		Handler:      r,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	return s
}

// Start starts a server
func (s *Server) Start() error {
	s.RefreshMembers()

	mlog.Info("Listening on", mlog.String("address", s.Config.ListenAddress))
	err := s.server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

// Stop stops a server
func (s *Server) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return s.server.Shutdown(ctx)
}

func (s *Server) RefreshMembers() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	members, err := s.getMembers(ctx)
	if err != nil {
		mlog.Error("failed to refresh org members", mlog.Err(err))
		s.logToMattermost("refresh failed, using org members of previous day\n" + err.Error())
		return
	}

	if members == nil {
		err = errors.New("no members found")
		mlog.Error("failed to refresh org members", mlog.Err(err))
		s.logToMattermost("refresh failed, using org members of previous day\n" + err.Error())
		return
	}

	s.OrgMembers = members
}

// Tick runs a check on objects in the database
func (s *Server) Tick() {
	mlog.Info("tick")

	stopRequests, _ := s.shouldStopRequests()
	if stopRequests {
		return
	}

	for _, repository := range s.Config.Repositories {
		ghPullRequests, _, err := s.GithubClient.PullRequests.List(context.Background(), repository.Owner, repository.Name, &github.PullRequestListOptions{
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

		issues, _, err := s.GithubClient.Issues.ListByRepo(context.Background(), repository.Owner, repository.Name, &github.IssueListByRepoOptions{
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

func (s *Server) ping(w http.ResponseWriter, r *http.Request) {
	uptime := fmt.Sprintf("%v", time.Since(s.StartTime))
	err := json.NewEncoder(w).Encode(pingResponse{Uptime: uptime})
	if err != nil {
		mlog.Error("Failed to write ping", mlog.Err(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (s *Server) githubEvent(w http.ResponseWriter, r *http.Request) {
	stopRequests, timeUntilReset := s.shouldStopRequests()
	if stopRequests {
		if !s.hasReportedRateLimit && timeUntilReset != nil {
			s.logToMattermost(":warning: Hit rate limit. Time until reset: " + timeUntilReset.String())
		}
		return
	}

	s.hasReportedRateLimit = false

	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		mlog.Error("Failed to read body", mlog.Err(err))
		return
	}

	receivedHash := strings.SplitN(r.Header.Get("X-Hub-Signature"), "=", 2)
	if receivedHash[0] != "sha1" {
		mlog.Error("Invalid webhook hash signature: SHA1")
		return
	}

	err = ValidateSignature(receivedHash, buf, s.Config.GitHubWebhookSecret)
	if err != nil {
		mlog.Error(err.Error())
		return
	}

	var pingEvent *github.PingEvent
	if r.Header.Get("X-GitHub-Event") == "ping" {
		pingEvent = PingEventFromJSON(ioutil.NopCloser(bytes.NewBuffer(buf)))
	}

	event := PullRequestEventFromJSON(ioutil.NopCloser(bytes.NewBuffer(buf)))
	eventIssueComment := IssueCommentFromJSON(ioutil.NopCloser(bytes.NewBuffer(buf)))

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
		if strings.Contains(strings.TrimSpace(*eventIssueComment.Comment.Body), "/autoassign") {
			s.handleAutoassign(*eventIssueComment)
		}
		if strings.Contains(strings.TrimSpace(*eventIssueComment.Comment.Body), "/update-branch") {
			s.handleUpdateBranch(*eventIssueComment)
		}
		return
	}

	if pingEvent != nil {
		mlog.Info("ping event", mlog.Int64("HookID", pingEvent.GetHookID()))
		return
	}

	s.handleIssueEvent(event)
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

	return filepath.Join(fileLocation, logFilename)
}

func SetupLogging(config *Config) {
	loggingConfig := &mlog.LoggerConfiguration{
		EnableConsole: config.LogSettings.EnableConsole,
		ConsoleJson:   config.LogSettings.ConsoleJSON,
		ConsoleLevel:  strings.ToLower(config.LogSettings.ConsoleLevel),
		EnableFile:    config.LogSettings.EnableFile,
		FileJson:      config.LogSettings.FileJSON,
		FileLevel:     strings.ToLower(config.LogSettings.FileLevel),
		FileLocation:  GetLogFileLocation(config.LogSettings.FileLocation),
	}

	logger := mlog.NewLogger(loggingConfig)
	mlog.RedirectStdLog(logger)
	mlog.InitGlobalLogger(logger)
}
