package server

import (
	"context"
	"database/sql"
	"reflect"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/google/go-github/v32/github"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-mattermod/server/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleCherryPick(t *testing.T) {
	ctrl := gomock.NewController(t)

	s := Server{
		Config: &Config{
			Org: "some-organization",
		},
		OrgMembers: []string{
			"org-member",
		},
		GithubClient: &GithubClient{},
	}

	pr := &model.PullRequest{
		RepoOwner: "user",
		RepoName:  "repo-name",
		Number:    123,
		Sha:       "some-sha",
		Merged:    sql.NullBool{Valid: true},
	}

	ctxInterface := reflect.TypeOf((*context.Context)(nil)).Elem()

	msg := new(string)
	comment := &github.IssueComment{Body: msg}
	is := mocks.NewMockIssuesService(ctrl)
	is.EXPECT().CreateComment(gomock.AssignableToTypeOf(ctxInterface), pr.RepoOwner, pr.RepoName, pr.Number, comment).AnyTimes().Return(nil, nil, nil)
	s.GithubClient.Issues = is

	t.Run("should ignore for non org members", func(t *testing.T) {
		*msg = msgCommenterPermission

		err := s.handleCherryPick(context.Background(), "non-org-member", "/cherry-pick release-5.28", pr)
		require.NoError(t, err)
	})

	t.Run("should ignore not merged PRs", func(t *testing.T) {
		err := s.handleCherryPick(context.Background(), "org-member", "/cherry-pick release-5.28", pr)
		require.NoError(t, err)
	})

	t.Run("should ignore when server is closing", func(t *testing.T) {
		s.cherryPickStopChan = make(chan struct{})
		s.cherryPickRequests = make(chan *cherryPickRequest, 1)
		pr.Merged.Bool = true
		close(s.cherryPickStopChan)
		close(s.cherryPickRequests)

		err := s.handleCherryPick(context.Background(), "org-member", "/cherry-pick release-5.28", pr)
		require.EqualError(t, err, "server is closing")
	})

	t.Run("should fail on too many cherry pick tasks", func(t *testing.T) {
		s.cherryPickStopChan = make(chan struct{})
		s.cherryPickRequests = make(chan *cherryPickRequest, 1)
		pr.Merged.Bool = true

		*msg = cherryPickScheduledMsg

		err := s.handleCherryPick(context.Background(), "org-member", "/cherry-pick release-5.28", pr)
		require.NoError(t, err)

		*msg = tooManyCherryPickMsg

		err = s.handleCherryPick(context.Background(), "org-member", "/cherry-pick release-5.28", pr)
		require.EqualError(t, err, "too many requests")
	})

	t.Run("should not panic on empty requests", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				require.Failf(t, "recovered from panic", "%v", r)
			}
		}()

		err := s.handleCherryPick(context.Background(), "org-member", "/cherry-pick", pr)
		require.NoError(t, err)
	})
}

func TestGetMilestone(t *testing.T) {
	title := "v5.20.0"
	milestone := getMilestone(title)
	assert.Equal(t, "release-5.20", milestone)

	title = "v5.1.0"
	milestone = getMilestone(title)
	assert.Equal(t, "release-5.1", milestone)
}
