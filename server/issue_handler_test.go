// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-mattermod/server/mocks"
	stmock "github.com/mattermost/mattermost-mattermod/store/mocks"

	"github.com/golang/mock/gomock"
	"github.com/google/go-github/v32/github"
	"github.com/stretchr/testify/require"
)

func TestIssueEventHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	s := &Server{
		GithubClient: &GithubClient{},
		Config: &Config{
			IssueLabels: []LabelResponse{
				{
					Label:   "label-1",
					Message: "some-message",
				},
			},
			Username: "mattermost",
		},
	}
	is := mocks.NewMockIssuesService(ctrl)
	s.GithubClient.Issues = is

	isStoreMock := stmock.NewMockIssueStore(ctrl)
	ss := stmock.NewMockStore(ctrl)
	ss.EXPECT().
		Issue().
		Return(isStoreMock).
		AnyTimes()

	s.Store = ss

	url := "https://api.github.com/repos/mattermost/mattermost-server/issues/1"
	number := 1
	state := "state"
	login := "mattermost"
	name := "mattermost-server"

	event := issueEvent{
		Issue: &github.Issue{
			Repository: &github.Repository{
				Owner: &github.User{
					Login: &login,
				},
				Name: &name,
			},
			Number:  &number,
			HTMLURL: &url,
			State:   &state,
			User: &github.User{
				Login: &login,
			},
		},
	}

	issue := &model.Issue{
		RepoOwner: login,
		RepoName:  name,
		Username:  login,
		State:     state,
		Labels:    []string{},
		Number:    1,
	}

	ts := httptest.NewServer(http.HandlerFunc(s.issueEventHandler))
	defer ts.Close()

	ctxInterface := reflect.TypeOf((*context.Context)(nil)).Elem()

	t.Run("Should fail with no body", func(t *testing.T) {
		req, err := http.NewRequest("POST", ts.URL, nil)
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("Should fail for not getting issue from github", func(t *testing.T) {
		is.EXPECT().ListLabelsByIssue(gomock.AssignableToTypeOf(ctxInterface), issue.RepoOwner, issue.RepoName, issue.Number, nil).
			Times(1).
			Return(nil, nil, errors.New("error"))

		b, err := json.Marshal(&event)
		require.NoError(t, err)

		req, err := http.NewRequest("POST", ts.URL, bytes.NewReader(b))
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})

	t.Run("Issue does not have changes", func(t *testing.T) {
		is.EXPECT().ListLabelsByIssue(gomock.AssignableToTypeOf(ctxInterface), issue.RepoOwner, issue.RepoName, issue.Number, nil).
			Times(1).
			Return([]*github.Label{}, nil, nil)

		isStoreMock.EXPECT().
			Get(issue.RepoOwner, issue.RepoName, issue.Number).
			Return(issue, nil).
			Times(1)

		b, err := json.Marshal(&event)
		require.NoError(t, err)

		req, err := http.NewRequest("POST", ts.URL, bytes.NewReader(b))
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("Issue has changes", func(t *testing.T) {
		is.EXPECT().ListLabelsByIssue(gomock.AssignableToTypeOf(ctxInterface), issue.RepoOwner, issue.RepoName, issue.Number, nil).
			Times(1).
			Return([]*github.Label{}, nil, nil)

		oldIssue := *issue
		oldIssue.Labels = []string{"labeled"}
		oldIssue.State = "old-state"

		isStoreMock.EXPECT().
			Get(oldIssue.RepoOwner, oldIssue.RepoName, oldIssue.Number).
			Return(&oldIssue, nil).
			Times(1)

		isStoreMock.EXPECT().
			Save(issue).
			Return(nil, nil).
			AnyTimes()

		b, err := json.Marshal(&event)
		require.NoError(t, err)

		req, err := http.NewRequest("POST", ts.URL, bytes.NewReader(b))
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("Issue labeled", func(t *testing.T) {
		label := "label-1"
		body := "some-message"
		login := "somebody"
		is.EXPECT().ListLabelsByIssue(gomock.AssignableToTypeOf(ctxInterface), issue.RepoOwner, issue.RepoName, issue.Number, nil).
			Times(1).
			Return([]*github.Label{
				{
					Name: &label,
				},
			}, nil, nil)

		is.EXPECT().ListComments(gomock.AssignableToTypeOf(ctxInterface), issue.RepoOwner, issue.RepoName, issue.Number, nil).
			Times(1).Return(
			[]*github.IssueComment{
				{
					Body: &body,
					User: &github.User{ //somebody else is commented
						Login: &login,
					},
				},
			}, nil, nil)

		is.EXPECT().CreateComment(gomock.AssignableToTypeOf(ctxInterface), issue.RepoOwner, issue.RepoName, issue.Number, &github.IssueComment{Body: &body}).
			Times(1).Return(nil, nil, nil)

		newIssue := *issue
		newIssue.Labels = []string{label}

		isStoreMock.EXPECT().
			Get(issue.RepoOwner, issue.RepoName, issue.Number).
			Return(issue, nil).
			Times(1)

		isStoreMock.EXPECT().
			Save(&newIssue).
			Return(nil, nil).
			AnyTimes()

		b, err := json.Marshal(&event)
		require.NoError(t, err)

		req, err := http.NewRequest("POST", ts.URL, bytes.NewReader(b))
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})
}
