// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/braintree/manners"
	"github.com/google/go-github/github"
	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-mattermod/store"
	"github.com/mattermost/mattermost-server/mlog"
	"github.com/mattermost/mattermost-server/utils/fileutils"
)

type Server struct {
	Store  store.Store
	Router *mux.Router
}

const (
	INSTANCE_ID_MESSAGE = "Instance ID: "
	LOG_FILENAME        = "mattermod.log"
)

var (
	Srv *Server

	commentLock sync.Mutex

	INSTANCE_ID_PATTERN = regexp.MustCompile(INSTANCE_ID_MESSAGE + "(i-[a-z0-9]+)")
	INSTANCE_ID         = "INSTANCE_ID"
	SPINMINT_LINK       = "SPINMINT_LINK"
)

func Start() {
	SetupLogging()
	mlog.Info("Starting pr manager")

	Srv = &Server{
		Store:  store.NewSqlStore(Config.DriverName, Config.DataSource),
		Router: mux.NewRouter(),
	}

	addApis(Srv.Router)

	var handler http.Handler = Srv.Router
	go func() {
		mlog.Info("Listening on", mlog.String("address", Config.ListenAddress))
		err := manners.ListenAndServe(Config.ListenAddress, handler)
		if err != nil {
			LogErrorToMattermost(err.Error())
			mlog.Critical("server_error", mlog.Err(err))
			panic(err.Error())
		}
	}()
}

func Tick() {
	mlog.Info("tick")

	abortTick := CheckLimitRateAndAbortRequest()
	if abortTick {
		return
	}

	client := NewGithubClient()

	for _, repository := range Config.Repositories {
		ghPullRequests, _, err := client.PullRequests.List(repository.Owner, repository.Name, &github.PullRequestListOptions{
			State: "open",
		})
		if err != nil {
			mlog.Error("failed to get PRs", mlog.String("repo_owner", repository.Owner), mlog.String("repo_name", repository.Name))
			mlog.Error("pr_error", mlog.Err(err))
			continue
		}

		for _, ghPullRequest := range ghPullRequests {
			pullRequest, errPR := GetPullRequestFromGithub(ghPullRequest)
			if errPR != nil {
				mlog.Error("failed to convert PR", mlog.Int("pr", *ghPullRequest.Number), mlog.Err(errPR))
				continue
			}

			checkPullRequestForChanges(pullRequest)
		}

		issues, _, err := client.Issues.ListByRepo(repository.Owner, repository.Name, &github.IssueListByRepoOptions{
			State: "open",
		})
		if err != nil {
			mlog.Error("failed to get issues", mlog.String("repo_owner", repository.Owner), mlog.String("repo_name", repository.Name))
			mlog.Error("issue_error", mlog.Err(err))
			continue
		}

		for _, ghIssue := range issues {
			if ghIssue.PullRequestLinks != nil {
				// This is a PR so we've already checked it
				continue
			}

			issue, err := GetIssueFromGithub(repository.Owner, repository.Name, ghIssue)
			if err != nil {
				mlog.Error("failed to convert issue", mlog.Int("issue", *ghIssue.Number), mlog.Err(err))
				continue
			}

			checkIssueForChanges(issue)
		}
	}

	checkSpinmintLifeTime()
}

func Stop() {
	mlog.Info("Stopping pr manager")
	manners.Close()
}

func addApis(r *mux.Router) {
	r.HandleFunc("/pr_event", prEvent).Methods("POST")
	r.HandleFunc("/list_prs", listPrs).Methods("GET")
	r.HandleFunc("/list_issues", listIssues).Methods("GET")
	r.HandleFunc("/list_spinmints", listSpinmints).Methods("GET")
}

func prEvent(w http.ResponseWriter, r *http.Request) {
	buf, _ := ioutil.ReadAll(r.Body)
	event := PullRequestEventFromJson(ioutil.NopCloser(bytes.NewBuffer(buf)))
	eventIssueComment := IssueCommentFromJson(ioutil.NopCloser(bytes.NewBuffer(buf)))

	abortTick := CheckLimitRateAndAbortRequest()
	if abortTick {
		return
	}

	if event.PRNumber != 0 {
		mlog.Info("pr event", mlog.Int("pr", event.PRNumber), mlog.String("action", event.Action))
		handlePullRequestEvent(event)
	} else if eventIssueComment.Action == "created" {
		if strings.Contains(strings.TrimSpace(*eventIssueComment.Comment.Body), "/check-cla") {
			handleCheckCLA(*eventIssueComment)
		}
	} else {
		handleIssueEvent(event)
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

func listPrs(w http.ResponseWriter, r *http.Request) {
	var prs []*model.PullRequest
	if result := <-Srv.Store.PullRequest().List(); result.Err != nil {
		mlog.Error(result.Err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	} else {
		prs = result.Data.([]*model.PullRequest)
	}

	if b, err := json.Marshal(prs); err != nil {
		mlog.Error("pr_error", mlog.Err(err))
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		w.Write(b)
	}
}

func listIssues(w http.ResponseWriter, r *http.Request) {
	var issues []*model.Issue
	if result := <-Srv.Store.Issue().List(); result.Err != nil {
		mlog.Error(result.Err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	} else {
		issues = result.Data.([]*model.Issue)
	}

	if b, err := json.Marshal(issues); err != nil {
		mlog.Error("issue_error", mlog.Err(err))
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		w.Write(b)
	}
}

func listSpinmints(w http.ResponseWriter, r *http.Request) {
	var spinmints []*model.Spinmint
	if result := <-Srv.Store.Spinmint().List(); result.Err != nil {
		mlog.Error(result.Err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	} else {
		spinmints = result.Data.([]*model.Spinmint)
	}

	if b, err := json.Marshal(spinmints); err != nil {
		mlog.Error("spinmint_error", mlog.Err(err))
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		w.Write(b)
	}
}

func GetLogFileLocation(fileLocation string) string {
	if fileLocation == "" {
		fileLocation, _ = fileutils.FindDir("logs")
	}

	return filepath.Join(fileLocation, LOG_FILENAME)
}

func SetupLogging() {
	loggingConfig := &mlog.LoggerConfiguration{
		EnableConsole: Config.LogSettings.EnableConsole,
		ConsoleJson:   Config.LogSettings.ConsoleJson,
		ConsoleLevel:  strings.ToLower(Config.LogSettings.ConsoleLevel),
		EnableFile:    Config.LogSettings.EnableFile,
		FileJson:      Config.LogSettings.FileJson,
		FileLevel:     strings.ToLower(Config.LogSettings.FileLevel),
		FileLocation:  GetLogFileLocation(Config.LogSettings.FileLocation),
	}

	logger := mlog.NewLogger(loggingConfig)
	mlog.RedirectStdLog(logger)
	mlog.InitGlobalLogger(logger)
}
