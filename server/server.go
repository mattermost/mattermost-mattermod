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
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/v43/github"
	"github.com/gorilla/mux"
	"github.com/mattermost/go-circleci"
	"github.com/mattermost/mattermost-mattermod/store"
	"github.com/mattermost/mattermost-mattermod/version"
	"github.com/mattermost/mattermost-server/v5/mlog"
	"github.com/mattermost/mattermost-server/v5/utils/fileutils"
)

// Server is the mattermod server.
// nolint:govet
type Server struct {
	Config                *Config
	Store                 store.Store
	GithubClient          *GithubClient
	CircleCiClient        CircleCIService
	CircleCiClientV2      CircleCIService
	GitLabCIClientV4      *GitLabClient
	OrgMembers            []string
	commentLock           sync.Mutex
	StartTime             time.Time
	Metrics               MetricsProvider
	cherryPickRequests    chan *cherryPickRequest
	cherryPickStopChan    chan struct{}
	cherryPickStoppedChan chan struct{}

	server *http.Server
}

type pingResponse struct {
	Info   *version.Info `json:"info"`
	Uptime string        `json:"uptime"`
}

const (
	logFilename = "mattermod.log"

	serverRepoName = "mattermost-server"
)

func New(config *Config, metrics MetricsProvider) (*Server, error) {
	s := &Server{
		Config:                config,
		Store:                 store.NewSQLStore(config.DriverName, config.DataSource),
		StartTime:             time.Now(),
		Metrics:               metrics,
		cherryPickRequests:    make(chan *cherryPickRequest, 20),
		cherryPickStopChan:    make(chan struct{}),
		cherryPickStoppedChan: make(chan struct{}),
	}

	ghClient, err := NewGithubClient(s.Config.GithubAccessToken, s.Config.GitHubTokenReserve, s.Metrics)
	if err != nil {
		return nil, err
	}
	s.GithubClient = ghClient
	s.CircleCiClient, err = circleci.NewClient(s.Config.CircleCIToken, circleci.APIVersion11)
	if err != nil {
		return nil, err
	}
	s.CircleCiClientV2, err = circleci.NewClient(s.Config.CircleCIToken, circleci.APIVersion2)
	if err != nil {
		return nil, err
	}
	s.GitLabCIClientV4, err = NewGitLabClient(s.Config.GitLabInternalToken, s.Config.GitLabInternalURL)
	if err != nil {
		return nil, err
	}

	r := mux.NewRouter()
	r.HandleFunc("/", s.ping).Methods(http.MethodGet)

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

	go s.listenCherryPickRequests()
}

// Stop stops a server
func (s *Server) Stop() error {
	s.finishCherryPickRequests()

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
				mlog.Error("recovered from a panic",
					mlog.String("url", r.URL.String()),
					mlog.Any("error", x),
					mlog.String("stack", string(debug.Stack())))
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
		prListOpts := &github.PullRequestListOptions{
			State:       "open",
			ListOptions: github.ListOptions{PerPage: 50},
		}
		// We sleep in between requests to remain within rate limits.
		// While we do have a rate limiter in the HTTP transport itself, that's a general limit for the entire application.
		// In this scenario, just during listing issues and PRs,
		// we need to throttle the rate a bit more.
		for {
			ghPullRequests, resp, err := s.GithubClient.PullRequests.List(ctx, repository.Owner, repository.Name, prListOpts)
			if err != nil {
				mlog.Error("Failed to get PRs", mlog.Err(err), mlog.String("repo_owner", repository.Owner), mlog.String("repo_name", repository.Name))
				s.Metrics.IncreaseCronTaskErrors("tick")
				if resp.NextPage == 0 {
					break
				}
				prListOpts.Page = resp.NextPage
				continue
			}
			if resp.NextPage == 0 {
				break
			}
			prListOpts.Page = resp.NextPage
			time.Sleep(200 * time.Millisecond)

			for _, ghPullRequest := range ghPullRequests {
				pullRequest, errPR := s.GetPullRequestFromGithub(ctx, ghPullRequest, "")
				if errPR != nil {
					mlog.Error("failed to convert PR", mlog.Int("pr", *ghPullRequest.Number), mlog.Err(errPR))
					s.Metrics.IncreaseCronTaskErrors("tick")
					continue
				}

				changed, err2 := s.checkPullRequestForChanges(ctx, pullRequest)
				if err2 != nil {
					mlog.Error("Could not check changes for PR", mlog.Err(err2))
				} else if changed {
					mlog.Info("pr has changes", mlog.Int("pr", pullRequest.Number))
				}
			}
		}

		time.Sleep(time.Second)

		issueListOpts := &github.IssueListByRepoOptions{
			State:       "open",
			ListOptions: github.ListOptions{PerPage: 50},
		}

		for {
			issues, resp, err := s.GithubClient.Issues.ListByRepo(ctx, repository.Owner, repository.Name, issueListOpts)
			if err != nil {
				mlog.Error("Failed to get issues", mlog.Err(err), mlog.String("repo_owner", repository.Owner), mlog.String("repo_name", repository.Name))
				s.Metrics.IncreaseCronTaskErrors("tick")
				if resp.NextPage == 0 {
					break
				}
				issueListOpts.Page = resp.NextPage
				continue
			}
			if resp.NextPage == 0 {
				break
			}
			issueListOpts.Page = resp.NextPage

			time.Sleep(200 * time.Millisecond)

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
}

func (s *Server) ping(w http.ResponseWriter, r *http.Request) {
	uptime := fmt.Sprintf("%v", time.Since(s.StartTime))
	err := json.NewEncoder(w).Encode(pingResponse{Uptime: uptime, Info: version.Full()})
	if err != nil {
		mlog.Error("Failed to write ping", mlog.Err(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (s *Server) githubEvent(w http.ResponseWriter, r *http.Request) {
	switch event := r.Header.Get("X-GitHub-Event"); event {
	case "ping":
		pingEvent := PingEventFromJSON(r.Body)
		if pingEvent == nil {
			http.Error(w, "could not parse ping event", http.StatusBadRequest)
			return
		}
		mlog.Info("ping event", mlog.Int64("HookID", pingEvent.GetHookID()))
	case "issues":
		buf, err := ioutil.ReadAll(r.Body)
		if err != nil {
			mlog.Error("Failed to read body", mlog.Err(err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		err = r.Body.Close()
		if err != nil {
			mlog.Error("Error closing body", mlog.Err(err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		event, err := issueEventFromJSON(bytes.NewReader(buf))
		if err != nil {
			mlog.Error("Could not parse issue event", mlog.Err(err))
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		r.Body = ioutil.NopCloser(bytes.NewBuffer(buf))
		// An issue can be both an issue or a PR. So we need to differentiate between the two.
		if event.Issue.IsPullRequest() {
			mlog.Info("A PR event is found from an issue. Updating DB.", mlog.String("link", event.Issue.GetPullRequestLinks().GetHTMLURL()))
			s.prFromIssueHandler(event, w)
		} else {
			s.issueEventHandler(w, r)
		}
	case "issue_comment":
		s.issueCommentEventHandler(w, r)
	case "pull_request":
		s.pullRequestEventHandler(w, r)
	default:
		http.Error(w, "unhandled event type", http.StatusNotImplemented)
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

func PingEventFromJSON(data io.Reader) *github.PingEvent {
	decoder := json.NewDecoder(data)
	var event github.PingEvent
	if err := decoder.Decode(&event); err != nil {
		return nil
	}

	return &event
}
