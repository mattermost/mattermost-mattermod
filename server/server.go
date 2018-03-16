// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"github.com/braintree/manners"
	"github.com/google/go-github/github"
	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-mattermod/store"
)

type Server struct {
	Store  store.Store
	Router *mux.Router
}

const (
	INSTANCE_ID_MESSAGE = "Instance ID: "
)

var (
	Srv *Server

	commentLock sync.Mutex

	INSTANCE_ID_PATTERN = regexp.MustCompile(INSTANCE_ID_MESSAGE + "(i-[a-z0-9]+)")
	INSTANCE_ID         = "INSTANCE_ID"
	SPINMINT_LINK       = "SPINMINT_LINK"
)

func Start() {
	LogInfo("Starting pr manager")

	Srv = &Server{
		Store:  store.NewSqlStore(Config.DriverName, Config.DataSource),
		Router: mux.NewRouter(),
	}

	addApis(Srv.Router)

	var handler http.Handler = Srv.Router
	go func() {
		LogInfo("Listening on " + Config.ListenAddress)
		err := manners.ListenAndServe(Config.ListenAddress, handler)
		if err != nil {
			LogErrorToMattermost(err.Error())
			LogCritical(err.Error())
		}
	}()
}

func Tick() {
	LogInfo("tick")

	client := NewGithubClient()

	for _, repository := range Config.Repositories {
		ghPullRequests, _, err := client.PullRequests.List(repository.Owner, repository.Name, &github.PullRequestListOptions{
			State: "open",
		})
		if err != nil {
			LogError("failed to get prs " + repository.Owner + "/" + repository.Name)
			LogError(err.Error())
			continue
		}

		for _, ghPullRequest := range ghPullRequests {
			pullRequest, err := GetPullRequestFromGithub(ghPullRequest)
			if err != nil {
				LogError("failed to convert PR for %v: %v", ghPullRequest.Number, err)
				continue
			}

			checkPullRequestForChanges(pullRequest)
		}

		issues, _, err := client.Issues.ListByRepo(repository.Owner, repository.Name, &github.IssueListByRepoOptions{
			State: "open",
		})
		if err != nil {
			LogError("failed to get issues " + repository.Owner + "/" + repository.Name)
			LogError(err.Error())
			continue
		}

		for _, ghIssue := range issues {
			if ghIssue.PullRequestLinks != nil {
				// This is a PR so we've already checked it
				continue
			}

			issue, err := GetIssueFromGithub(repository.Owner, repository.Name, ghIssue)
			if err != nil {
				LogError("failed to convert issue for %v: %v", ghIssue.Number, err)
				continue
			}

			checkIssueForChanges(issue)
		}
	}
}

func Stop() {
	LogInfo("Stopping pr manager")
	manners.Close()
}

func addApis(r *mux.Router) {
	r.HandleFunc("/pr_event", prEvent).Methods("POST")
	r.HandleFunc("/list_prs", listPrs).Methods("GET")
	r.HandleFunc("/list_issues", listIssues).Methods("GET")
}

func prEvent(w http.ResponseWriter, r *http.Request) {
	buf, _ := ioutil.ReadAll(r.Body)
	event := PullRequestEventFromJson(ioutil.NopCloser(bytes.NewBuffer(buf)))

	if event.PRNumber != 0 {
		LogInfo(fmt.Sprintf("pr event %v %v", event.PRNumber, event.Action))
		handlePullRequestEvent(event)
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
		LogError(result.Err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	} else {
		prs = result.Data.([]*model.PullRequest)
	}

	if b, err := json.Marshal(prs); err != nil {
		LogError(err.Error())
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		w.Write(b)
	}
}

func listIssues(w http.ResponseWriter, r *http.Request) {
	var issues []*model.Issue
	if result := <-Srv.Store.Issue().List(); result.Err != nil {
		LogError(result.Err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	} else {
		issues = result.Data.([]*model.Issue)
	}

	if b, err := json.Marshal(issues); err != nil {
		LogError(err.Error())
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		w.Write(b)
	}
}
