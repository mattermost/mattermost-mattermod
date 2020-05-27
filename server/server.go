// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/braintree/manners"
	"github.com/google/go-github/v31/github"
	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost-mattermod/store"
	"github.com/mattermost/mattermost-server/v5/mlog"
	"github.com/mattermost/mattermost-server/v5/utils/fileutils"
)

// Server is the mattermod server.
type Server struct {
	Config       *Config
	Store        store.Store
	Router       *mux.Router
	GithubClient *GithubClient
	OrgMembers   []string
	Builds       buildsInterface
	commentLock  sync.Mutex
	StartTime    time.Time
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

func New(config *Config) (server *Server, err error) {
	s := &Server{
		Config:    config,
		Store:     store.NewSQLStore(config.DriverName, config.DataSource),
		Router:    mux.NewRouter(),
		StartTime: time.Now(),
	}

	s.GithubClient = NewGithubClient(s.Config.GithubAccessToken)

	s.Builds = &Builds{}
	if os.Getenv(buildOverride) != "" {
		mlog.Warn("Using mocked build tools")
		s.Builds = &MockedBuilds{
			Version: os.Getenv(buildOverride),
		}
	}

	return s, nil
}

// Start starts a server
func (s *Server) Start() {
	mlog.Info("Starting Mattermod Server")

	rand.Seed(time.Now().Unix())

	s.initializeRouter()
	s.RefreshMembers()

	var handler http.Handler = s.Router
	go func() {
		mlog.Info("Listening on", mlog.String("address", s.Config.ListenAddress))
		err := manners.ListenAndServe(s.Config.ListenAddress, handler)
		if err != nil {
			s.logToMattermost(err.Error())
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

	msg := "#Members " + time.Now().Format(time.RFC850) + "\n  "
	for _, member := range s.OrgMembers {
		msg += "- " + member + "\n  "
	}
	s.logToMattermost(msg)
}

// Tick runs a check on objects in the database
func (s *Server) Tick() {
	mlog.Info("tick")

	aboveLimit := s.hasReachedRateLimit()
	if aboveLimit {
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

func (s *Server) initializeRouter() {
	s.Router.HandleFunc("/", s.ping).Methods("GET")
	s.Router.HandleFunc("/pr_event", s.githubEvent).Methods("POST")
}

func (s *Server) ping(w http.ResponseWriter, r *http.Request) {
	msg := fmt.Sprintf("{\"uptime\": \"%v\"}", time.Since(s.StartTime))
	w.Header().Set("Content-Type", "application/json")

	_, err := w.Write([]byte(msg))
	if err != nil {
		mlog.Error("Failed to write ping", mlog.Err(err))
	}
}

func (s *Server) githubEvent(w http.ResponseWriter, r *http.Request) {
	overLimit := s.hasReachedRateLimit()
	if overLimit {
		return
	}

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
