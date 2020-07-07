// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/google/go-github/v32/github"
	"github.com/gorilla/mux"
	"github.com/mattermost/go-circleci"
	"github.com/mattermost/mattermost-mattermod/store"
	"github.com/mattermost/mattermost-server/v5/mlog"
	"github.com/mattermost/mattermost-server/v5/utils/fileutils"
)

// Server is the mattermod server.
type Server struct {
	Config               *Config
	Store                store.Store
	GithubClient         *GithubClient
	CircleCiClient       *circleci.Client
	OrgMembers           []string
	Builds               buildsInterface
	commentLock          sync.Mutex
	StartTime            time.Time
	awsSession           *session.Session
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

func New(config *Config) (*Server, error) {
	s := &Server{
		Config:               config,
		Store:                store.NewSQLStore(config.DriverName, config.DataSource),
		StartTime:            time.Now(),
		hasReportedRateLimit: false,
	}

	s.GithubClient = NewGithubClient(s.Config.GithubAccessToken)
	s.CircleCiClient = &circleci.Client{Token: s.Config.CircleCIToken}
	awsSession, err := session.NewSession()
	if err != nil {
		return nil, err
	}
	s.awsSession = awsSession

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

	return s, nil
}

// Start starts a server
func (s *Server) Start() {
	s.RefreshMembers()
	mlog.Info("Listening on", mlog.String("address", s.Config.ListenAddress))
	go func() {
		err := s.server.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			return
		}
		mlog.Error("Server exited with error", mlog.Err(err))
		os.Exit(1)
	}()
}

// Stop stops a server
func (s *Server) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return s.server.Shutdown(ctx)
}

func (s *Server) RefreshMembers() {
	ctx, cancel := context.WithTimeout(context.Background(), defaultCronTaskTimeout*time.Second)
	defer cancel()
	members, err := s.getMembers(ctx)
	if err != nil {
		mlog.Error("failed to refresh org members", mlog.Err(err))
		s.logToMattermost(ctx, "refresh failed, using org members of previous day\n"+err.Error())
		return
	}

	if members == nil {
		err = errors.New("no members found")
		mlog.Error("failed to refresh org members", mlog.Err(err))
		s.logToMattermost(ctx, "refresh failed, using org members of previous day\n"+err.Error())
		return
	}

	s.OrgMembers = members
}

// Tick runs a check on objects in the database
func (s *Server) Tick() {
	mlog.Info("tick")
	ctx, cancel := context.WithTimeout(context.Background(), defaultCronTaskTimeout*time.Second)
	defer cancel()
	stopRequests, _ := s.shouldStopRequests(ctx)
	if stopRequests {
		return
	}

	for _, repository := range s.Config.Repositories {
		ghPullRequests, _, err := s.GithubClient.PullRequests.List(ctx, repository.Owner, repository.Name, &github.PullRequestListOptions{
			State: "open",
		})
		if err != nil {
			mlog.Error("Failed to get PRs", mlog.Err(err), mlog.String("repo_owner", repository.Owner), mlog.String("repo_name", repository.Name))
			continue
		}

		for _, ghPullRequest := range ghPullRequests {
			pullRequest, errPR := s.GetPullRequestFromGithub(ctx, ghPullRequest)
			if errPR != nil {
				mlog.Error("failed to convert PR", mlog.Int("pr", *ghPullRequest.Number), mlog.Err(errPR))
				continue
			}

			s.checkPullRequestForChanges(ctx, pullRequest)
		}

		issues, _, err := s.GithubClient.Issues.ListByRepo(ctx, repository.Owner, repository.Name, &github.IssueListByRepoOptions{
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

			issue, err := s.GetIssueFromGithub(ctx, repository.Owner, repository.Name, ghIssue)
			if err != nil {
				mlog.Error("failed to convert issue", mlog.Int("issue", *ghIssue.Number), mlog.Err(err))
				continue
			}

			s.checkIssueForChanges(ctx, issue)
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
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout*time.Second)
	defer cancel()
	stopRequests, timeUntilReset := s.shouldStopRequests(ctx)
	if stopRequests {
		if !s.hasReportedRateLimit && timeUntilReset != nil {
			s.logToMattermost(ctx, ":warning: Hit rate limit. Time until reset: "+timeUntilReset.String())
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
		s.handlePullRequestEvent(ctx, event)
		return
	}

	pr, err := s.getPRFromComment(ctx, *eventIssueComment)
	if err != nil {
		mlog.Error(err.Error())
		return
	}
	var commenter string
	if eventIssueComment.Comment != nil {
		commenter = eventIssueComment.Comment.User.GetLogin()
	}

	if eventIssueComment != nil && eventIssueComment.Action == "created" {
		if strings.Contains(strings.TrimSpace(*eventIssueComment.Comment.Body), "/check-cla") {
			s.handleCheckCLA(ctx, *eventIssueComment)
		}
		if strings.Contains(strings.TrimSpace(*eventIssueComment.Comment.Body), "/cherry-pick") {
			s.handleCherryPick(ctx, *eventIssueComment)
		}
		if strings.Contains(strings.TrimSpace(*eventIssueComment.Comment.Body), "/autoassign") {
			s.handleAutoassign(ctx, *eventIssueComment)
		}
		if strings.Contains(strings.TrimSpace(*eventIssueComment.Comment.Body), "/update-branch") {
			if err := s.handleUpdateBranch(ctx, commenter, pr); err != nil {
				mlog.Error("Error updating branch", mlog.Err(err))
			}
		}
		return
	}

	if pingEvent != nil {
		mlog.Info("ping event", mlog.Int64("HookID", pingEvent.GetHookID()))
		return
	}

	s.handleIssueEvent(ctx, event)
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

func closeBody(r *http.Response) {
	if r.Body != nil {
		_, _ = io.Copy(ioutil.Discard, r.Body)
		_ = r.Body.Close()
	}
}
