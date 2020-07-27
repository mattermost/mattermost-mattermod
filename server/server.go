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
	"strconv"
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
	Config         *Config
	Store          store.Store
	GithubClient   *GithubClient
	CircleCiClient *circleci.Client
	OrgMembers     []string
	Builds         buildsInterface
	commentLock    sync.Mutex
	StartTime      time.Time
	awsSession     *session.Session
	Metrics        MetricsProvider

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

func New(config *Config, metrics MetricsProvider) (*Server, error) {
	s := &Server{
		Config:    config,
		Store:     store.NewSQLStore(config.DriverName, config.DataSource),
		StartTime: time.Now(),
		Metrics:   metrics,
	}

	ghClient, err := NewGithubClient(s.Config.GithubAccessToken, s.Config.GitHubTokenReserve, s.Metrics)
	if err != nil {
		return nil, err
	}
	s.GithubClient = ghClient
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

	webhooks := r.PathPrefix("/webhooks").Subrouter()
	webhooks.HandleFunc("/issue", s.issueEventHandler).Methods(http.MethodPost)

	r.HandleFunc("/healthz", s.ping).Methods(http.MethodGet)
	r.HandleFunc("/pr_event", s.githubEvent).Methods(http.MethodPost)
	r.Use(s.withRecovery)
	r.Use(s.withRequestDuration)
	r.Use(s.withValidation)

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
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), defaultCronTaskTimeout*time.Second)
	defer cancel()
	defer func() {
		elapsed := float64(time.Since(start)) / float64(time.Second)
		s.Metrics.ObserveCronTaskDuration("refresh_members", elapsed)
	}()
	members, err := s.getMembers(ctx)
	if err != nil {
		mlog.Error("failed to refresh org members", mlog.Err(err))
		s.logToMattermost(ctx, "refresh failed, using org members of previous day\n"+err.Error())
		s.Metrics.IncreaseCronTaskErrors("refresh_members")
		return
	}

	if members == nil {
		err = errors.New("no members found")
		mlog.Error("failed to refresh org members", mlog.Err(err))
		s.logToMattermost(ctx, "refresh failed, using org members of previous day\n"+err.Error())
		s.Metrics.IncreaseCronTaskErrors("refresh_members")
		return
	}

	s.OrgMembers = members
}

func (s *Server) withRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if x := recover(); x != nil {
				mlog.Error("recovered from a panic", mlog.String("url", r.URL.String()), mlog.Any("error", x))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) withRequestDuration(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		w = newWrappedWriter(w)

		next.ServeHTTP(w, r)

		elapsed := float64(time.Since(start)) / float64(time.Second)
		statusCode := strconv.Itoa(w.(*responseWriterWrapper).StatusCode())
		s.Metrics.ObserveHTTPRequestDuration(r.Method, r.URL.Path, statusCode, elapsed)
	})
}

// Tick runs a check on objects in the database
func (s *Server) Tick() {
	start := time.Now()
	mlog.Info("tick")
	ctx, cancel := context.WithTimeout(context.Background(), defaultCronTaskTimeout*time.Second)
	defer cancel()
	defer func() {
		elapsed := float64(time.Since(start)) / float64(time.Second)
		s.Metrics.ObserveCronTaskDuration("tick", elapsed)
	}()

	for _, repository := range s.Config.Repositories {
		ghPullRequests, _, err := s.GithubClient.PullRequests.List(ctx, repository.Owner, repository.Name, &github.PullRequestListOptions{
			State: "open",
		})
		if err != nil {
			mlog.Error("Failed to get PRs", mlog.Err(err), mlog.String("repo_owner", repository.Owner), mlog.String("repo_name", repository.Name))
			s.Metrics.IncreaseCronTaskErrors("tick")
			continue
		}

		for _, ghPullRequest := range ghPullRequests {
			pullRequest, errPR := s.GetPullRequestFromGithub(ctx, ghPullRequest)
			if errPR != nil {
				mlog.Error("failed to convert PR", mlog.Int("pr", *ghPullRequest.Number), mlog.Err(errPR))
				s.Metrics.IncreaseCronTaskErrors("tick")
				continue
			}

			s.checkPullRequestForChanges(ctx, pullRequest)
		}

		issues, _, err := s.GithubClient.Issues.ListByRepo(ctx, repository.Owner, repository.Name, &github.IssueListByRepoOptions{
			State: "open",
		})
		if err != nil {
			mlog.Error("Failed to get issues", mlog.Err(err), mlog.String("repo_owner", repository.Owner), mlog.String("repo_name", repository.Name))
			s.Metrics.IncreaseCronTaskErrors("tick")
			continue
		}

		for _, ghIssue := range issues {
			if ghIssue.PullRequestLinks != nil {
				// This is a PR so we've already checked it
				continue
			}

			issue, err := s.GetIssueFromGithub(ctx, ghIssue)
			if err != nil {
				mlog.Error("failed to convert issue", mlog.Int("issue", *ghIssue.Number), mlog.Err(err))
				s.Metrics.IncreaseCronTaskErrors("tick")
				continue
			}

			if err := s.checkIssueForChanges(ctx, issue); err != nil {
				mlog.Error("could not check issue for changes", mlog.Err(err))
			}
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

	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		mlog.Error("Failed to read body", mlog.Err(err))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if r.Header.Get("X-GitHub-Event") == "ping" {
		if pingEvent := PingEventFromJSON(ioutil.NopCloser(bytes.NewBuffer(buf))); pingEvent != nil {
			mlog.Info("ping event", mlog.Int64("HookID", pingEvent.GetHookID()))
			return
		}
	}

	// TODO: remove this after migration complete; MM-27283
	event, err := PullRequestEventFromJSON(ioutil.NopCloser(bytes.NewBuffer(buf)))
	if err != nil || event.PRNumber != 0 {
		mlog.Info("pr event", mlog.Int("pr", event.PRNumber), mlog.String("action", event.Action))
		s.handlePullRequestEvent(ctx, event)
		return
	}

	eventData := EventDataFromJSON(ioutil.NopCloser(bytes.NewBuffer(buf)))
	if eventData == nil || eventData.Action != "created" {
		if err = s.handleIssueEvent(ctx, event); err != nil {
			mlog.Error(err.Error())
		}
		return
	}

	pr, err := s.getPRFromEvent(ctx, *eventData)
	if err != nil {
		mlog.Error("Error getting PR from Comment", mlog.Err(err))
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var commenter string
	if eventData.Comment != nil && eventData.Comment.User != nil {
		commenter = eventData.Comment.User.GetLogin()
	}

	if eventData.HasCheckCLA() {
		s.Metrics.IncreaseWebhookRequest("check_cla")
		if err := s.handleCheckCLA(ctx, pr); err != nil {
			s.Metrics.IncreaseWebhookErrors("check_cla")
			mlog.Error("Error checking CLA", mlog.Err(err))
			w.WriteHeader(http.StatusInternalServerError)
		}
	}

	if eventData.HasCherryPick() {
		s.Metrics.IncreaseWebhookRequest("cherry_pick")
		if err := s.handleCherryPick(ctx, commenter, *eventData.Comment.Body, pr); err != nil {
			s.Metrics.IncreaseWebhookErrors("cherry_pick")
			mlog.Error("Error cherry picking", mlog.Err(err))
			w.WriteHeader(http.StatusInternalServerError)
		}
	}

	if eventData.HasAutoAssign() {
		s.Metrics.IncreaseWebhookRequest("auto_assign")
		if err := s.handleAutoAssign(ctx, eventData.Comment.GetHTMLURL(), pr); err != nil {
			s.Metrics.IncreaseWebhookErrors("auto_assign")
			mlog.Error("Error auto assigning", mlog.Err(err))
			w.WriteHeader(http.StatusInternalServerError)
		}
	}

	if eventData.HasUpdateBranch() {
		s.Metrics.IncreaseWebhookRequest("update_branch")
		if err := s.handleUpdateBranch(ctx, commenter, pr); err != nil {
			s.Metrics.IncreaseWebhookErrors("update_branch")
			mlog.Error("Error updating branch", mlog.Err(err))
			w.WriteHeader(http.StatusInternalServerError)
		}
	}
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

func SetupLogging(config *Config) error {
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

	if config.LogSettings.AdvancedLogging != nil {
		err := logger.ConfigAdvancedLogging(config.LogSettings.AdvancedLogging)
		if err != nil {
			return err
		}
	}
	return nil
}

func closeBody(r *http.Response) {
	if r.Body != nil {
		_, _ = io.Copy(ioutil.Discard, r.Body)
		_ = r.Body.Close()
	}
}
