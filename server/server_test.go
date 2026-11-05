// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/google/go-github/v39/github"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-mattermod/server/mocks"
	stmock "github.com/mattermost/mattermost-mattermod/store/mocks"
	"github.com/stretchr/testify/require"
)

func TestPing(t *testing.T) {
	s := &Server{
		StartTime: time.Now(),
	}

	ts := httptest.NewServer(http.HandlerFunc(s.ping))
	defer ts.Close()

	res, err := http.Get(ts.URL)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, res.StatusCode)
	defer res.Body.Close()

	var ping pingResponse

	err = json.NewDecoder(res.Body).Decode(&ping)
	require.NoError(t, err)
	require.NotZero(t, ping.Uptime)
	_, err = time.ParseDuration(ping.Uptime)
	require.NoError(t, err)
}

type panicHandler struct {
}

func (ph panicHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	panic("bad handler")
}

func TestWithRecovery(t *testing.T) {
	s := Server{}
	defer func() {
		if x := recover(); x != nil {
			require.Fail(t, "got panic")
		}
	}()

	ph := panicHandler{}
	handler := s.withRecovery(ph)

	req := httptest.NewRequest("GET", "http://random", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.Body != nil {
		defer resp.Body.Close()
		_, err := io.Copy(io.Discard, resp.Body)
		require.NoError(t, err)
	}
}

func TestWithDurationMiddleware(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	metricsMock := mocks.NewMockMetricsProvider(ctrl)
	s := &Server{
		StartTime: time.Now(),
		Metrics:   metricsMock,
	}

	h := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}

	handler := s.withRequestDuration(http.HandlerFunc(h))
	ts := httptest.NewServer(handler)
	defer ts.Close()

	t.Run("Should observe http request", func(t *testing.T) {
		metricsMock.EXPECT().ObserveHTTPRequestDuration(
			gomock.Eq("POST"),
			gomock.Eq("/"),
			gomock.Eq("201"),
			gomock.Any(),
		).Times(1)

		req, err := http.NewRequest("POST", ts.URL, nil)
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		defer resp.Body.Close()
	})
}

func TestTickSkipsRepositoryOnListError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	prs := mocks.NewMockPullRequestsService(ctrl)
	issues := mocks.NewMockIssuesService(ctrl)
	metrics := mocks.NewMockMetricsProvider(ctrl)
	metrics.EXPECT().IncreaseCronTaskErrors("tick").AnyTimes()
	metrics.EXPECT().ObserveCronTaskDuration("tick", gomock.Any()).AnyTimes()

	listErr := errors.New("context deadline exceeded")
	for _, repo := range []string{"repo-a", "repo-b"} {
		// Exactly one call per repository: a failed page must not be retried.
		prs.EXPECT().List(gomock.Any(), "mattermost", repo, gomock.Any()).Return(nil, nil, listErr).Times(1)
		issues.EXPECT().ListByRepo(gomock.Any(), "mattermost", repo, gomock.Any()).Return(nil, nil, listErr).Times(1)
	}

	s := &Server{
		Config: &Config{Repositories: []*Repository{
			{Owner: "mattermost", Name: "repo-a"},
			{Owner: "mattermost", Name: "repo-b"},
		}},
		GithubClient: &GithubClient{PullRequests: prs, Issues: issues},
		Metrics:      metrics,
	}

	done := make(chan struct{})
	go func() {
		s.Tick()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		require.Fail(t, "Tick did not return after list errors")
	}
}

func TestTickUsesListItemsAndChecksLastPage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	label := func(name string) *github.Label { return &github.Label{Name: github.String(name)} }
	owner, repo := "mattermost", "repo-a"

	ghPR := &github.PullRequest{
		Number: github.Int(1),
		State:  github.String("open"),
		Base:   &github.PullRequestBranch{Repo: &github.Repository{Name: github.String(repo), Owner: &github.User{Login: github.String(owner)}}},
		Head:   &github.PullRequestBranch{Ref: github.String("feature"), SHA: github.String("abc")},
		Labels: []*github.Label{label("Do Not Merge")},
	}
	ghIssue := &github.Issue{
		Number:  github.Int(2),
		State:   github.String("open"),
		HTMLURL: github.String("https://github.com/mattermost/repo-a/issues/2"),
		Labels:  []*github.Label{label("Bug")},
	}
	ghIssueForPR := &github.Issue{Number: github.Int(1), PullRequestLinks: &github.PullRequestLinks{}}

	// A single page (NextPage == 0) must still be checked. No per-item ListLabelsByIssue
	// calls are expected: gomock fails the test on any call without an expectation.
	prs := mocks.NewMockPullRequestsService(ctrl)
	prs.EXPECT().List(gomock.Any(), owner, repo, gomock.Any()).
		Return([]*github.PullRequest{ghPR}, &github.Response{}, nil).Times(1)
	issues := mocks.NewMockIssuesService(ctrl)
	issues.EXPECT().ListByRepo(gomock.Any(), owner, repo, gomock.Any()).
		Return([]*github.Issue{ghIssue, ghIssueForPR}, &github.Response{}, nil).Times(1)

	// The stored PR matches GitHub: nothing changed, so nothing is saved.
	prStore := stmock.NewMockPullRequestStore(ctrl)
	prStore.EXPECT().Get(owner, repo, 1).Return(&model.PullRequest{
		RepoOwner: owner, RepoName: repo, Number: 1, State: "open", Ref: "feature", Sha: "abc",
		Labels: []string{"Do Not Merge"}, Merged: github.Bool(false), MaintainerCanModify: github.Bool(false),
		MilestoneNumber: github.Int64(0), MilestoneTitle: github.String(""),
	}, nil).Times(1)
	// The issue is new to the store: it is saved with the labels from the list item.
	issueStore := stmock.NewMockIssueStore(ctrl)
	issueStore.EXPECT().Get(owner, repo, 2).Return(nil, nil).Times(1)
	issueStore.EXPECT().Save(gomock.Any()).DoAndReturn(func(i *model.Issue) (*model.Issue, error) {
		require.Equal(t, model.StringArray{"Bug"}, i.Labels)
		return i, nil
	}).Times(1)
	st := stmock.NewMockStore(ctrl)
	st.EXPECT().PullRequest().Return(prStore).AnyTimes()
	st.EXPECT().Issue().Return(issueStore).AnyTimes()

	metrics := mocks.NewMockMetricsProvider(ctrl)
	metrics.EXPECT().ObserveCronTaskDuration("tick", gomock.Any()).AnyTimes()

	s := &Server{
		Config:       &Config{Repositories: []*Repository{{Owner: owner, Name: repo}}},
		GithubClient: &GithubClient{PullRequests: prs, Issues: issues},
		Store:        st,
		Metrics:      metrics,
	}
	s.Tick()
}
