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

	"github.com/golang/mock/gomock"
	"github.com/google/go-github/v43/github"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-mattermod/server/mocks"
	stmock "github.com/mattermost/mattermost-mattermod/store/mocks"
	"github.com/stretchr/testify/require"
)

func TestIssueCommentEventHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	s := &Server{
		GithubClient: &GithubClient{},
		Config: &Config{
			Repositories: []*Repository{},
		},
	}
	prs := mocks.NewMockPullRequestsService(ctrl)
	s.GithubClient.PullRequests = prs

	is := mocks.NewMockIssuesService(ctrl)
	s.GithubClient.Issues = is

	prStoreMock := stmock.NewMockPullRequestStore(ctrl)
	ss := stmock.NewMockStore(ctrl)
	ss.EXPECT().
		PullRequest().
		Return(prStoreMock).
		AnyTimes()

	s.Store = ss

	event := issueCommentEvent{
		Repository: &github.Repository{
			Owner: &github.User{
				Login: github.String("mattertest"),
			},
			Name: github.String("mattermod"),
		},
		Comment: &github.PullRequestComment{
			Body: github.String("some-text"),
		},
		Issue: &github.Issue{
			Repository: &github.Repository{
				Owner: &github.User{
					Login: github.String("mattertest"),
				},
				Name: github.String("mattermod"),
			},
			Number: github.Int(1),
			State:  github.String("a-state"),
			User: &github.User{
				Login: github.String("mattertest"),
			},
			PullRequestLinks: &github.PullRequestLinks{},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(s.issueCommentEventHandler))
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

	t.Run("Missing entities in body", func(t *testing.T) {
		b, err := json.Marshal(issueCommentEvent{
			Action:  "new",
			Comment: &github.PullRequestComment{},
		})
		require.NoError(t, err)

		req, err := http.NewRequest("POST", ts.URL, bytes.NewReader(b))
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("Not a pull request comment, should not fail", func(t *testing.T) {
		b, err := json.Marshal(issueCommentEvent{
			Issue:      &github.Issue{},
			Repository: &github.Repository{},
			Comment:    &github.PullRequestComment{},
		})
		require.NoError(t, err)

		req, err := http.NewRequest("POST", ts.URL, bytes.NewReader(b))
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("Deletion event, should not fail", func(t *testing.T) {
		b, err := json.Marshal(issueCommentEvent{
			Action:     "deleted",
			Issue:      &github.Issue{},
			Repository: &github.Repository{},
			Comment:    &github.PullRequestComment{},
		})
		require.NoError(t, err)

		req, err := http.NewRequest("POST", ts.URL, bytes.NewReader(b))
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("Should fail on getting the PR", func(t *testing.T) {
		prs.EXPECT().Get(gomock.AssignableToTypeOf(ctxInterface), event.Repository.GetOwner().GetLogin(), event.Repository.GetName(), event.Issue.GetNumber()).
			Times(1).
			Return(nil, nil, errors.New("some-error"))

		b, err := json.Marshal(&event)
		require.NoError(t, err)

		req, err := http.NewRequest("POST", ts.URL, bytes.NewReader(b))
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})

	t.Run("Should fallthrough the handler", func(t *testing.T) {
		prs.EXPECT().Get(gomock.AssignableToTypeOf(ctxInterface), event.Repository.GetOwner().GetLogin(), event.Repository.GetName(), event.Issue.GetNumber()).
			Times(1).
			Return(nil, nil, nil)

		is.EXPECT().ListLabelsByIssue(gomock.AssignableToTypeOf(ctxInterface), "", "", 0, nil).
			Times(1).
			Return([]*github.Label{}, nil, nil)

		prStoreMock.EXPECT().Save(gomock.AssignableToTypeOf(&model.PullRequest{})).
			Times(1).
			Return(nil, nil)

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
