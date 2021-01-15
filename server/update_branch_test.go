// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"testing"

	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-mattermod/server/mocks"

	"github.com/golang/mock/gomock"
	"github.com/google/go-github/v33/github"
	"github.com/stretchr/testify/require"
)

func TestHandeUpdateBranch(t *testing.T) {
	ctrl := gomock.NewController(t)

	const userHandle = "user"
	const organization = "some-organization"

	s := Server{
		Config: &Config{
			Org: organization,
		},
		GithubClient: &GithubClient{},
	}

	ctx := context.Background()

	pr := &model.PullRequest{
		RepoOwner: userHandle,
		RepoName:  "repo-name",
		Number:    123,
		Sha:       "some-sha",
	}

	opt := &github.PullRequestBranchUpdateOptions{
		ExpectedHeadSHA: github.String(pr.Sha),
	}

	msg := new(string)
	comment := &github.IssueComment{Body: msg}
	is := mocks.NewMockIssuesService(ctrl)
	is.EXPECT().CreateComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, comment).Times(5).Return(nil, nil, nil)
	s.GithubClient.Issues = is

	t.Run("random user", func(t *testing.T) {
		*msg = msgCommenterPermission

		err := s.handleUpdateBranch(ctx, "someone", pr)
		require.Error(t, err)
		require.IsType(t, &updateError{}, err)
		require.Equal(t, err.(*updateError).source, *msg)
	})

	t.Run("not org. member", func(t *testing.T) {
		*msg = msgCommenterPermission

		err := s.handleUpdateBranch(ctx, userHandle, pr)
		require.Error(t, err)
		require.IsType(t, &updateError{}, err)
		require.Equal(t, err.(*updateError).source, *msg)
	})

	t.Run("app does not have permissions", func(t *testing.T) {
		s.OrgMembers = make([]string, 1)
		s.OrgMembers[0] = userHandle

		*msg = msgOrganizationPermission

		err := s.handleUpdateBranch(ctx, userHandle, pr)
		require.Error(t, err)
		require.IsType(t, &updateError{}, err)
		require.Equal(t, err.(*updateError).source, *msg)
	})

	t.Run("err from github api", func(t *testing.T) {
		s.OrgMembers = make([]string, 1)
		s.OrgMembers[0] = userHandle
		pr.FullName = organization + "/" + userHandle

		*msg = msgUpdatePullRequest
		expectedErr := errors.New("some-error")

		prs := mocks.NewMockPullRequestsService(ctrl)
		prs.EXPECT().UpdateBranch(ctx, pr.RepoOwner, pr.RepoName, pr.Number, gomock.AssignableToTypeOf(opt)).Return(nil, nil, expectedErr)
		s.GithubClient.PullRequests = prs

		err := s.handleUpdateBranch(ctx, userHandle, pr)
		require.Error(t, err)
		require.True(t, errors.Is(err, expectedErr))
	})

	t.Run("non-OK status code from github", func(t *testing.T) {
		resp := &github.Response{
			Response: &http.Response{
				StatusCode: http.StatusNotFound,
			},
		}

		s.OrgMembers = make([]string, 1)
		s.OrgMembers[0] = userHandle
		pr.FullName = organization + "/" + userHandle

		*msg = msgUpdatePullRequest

		prs := mocks.NewMockPullRequestsService(ctrl)
		prs.EXPECT().UpdateBranch(ctx, pr.RepoOwner, pr.RepoName, pr.Number, gomock.AssignableToTypeOf(opt)).Return(nil, resp, nil)
		s.GithubClient.PullRequests = prs

		err := s.handleUpdateBranch(ctx, userHandle, pr)
		require.Error(t, err)
		require.IsType(t, &updateError{}, err)
		require.Equal(t, err.(*updateError).source, *msg)
	})

	t.Run("maintainer can modify and accepted by github", func(t *testing.T) {
		resp := &github.Response{
			Response: &http.Response{
				StatusCode: http.StatusAccepted,
			},
		}
		pr.MaintainerCanModify = sql.NullBool{Bool: true, Valid: true}

		prs := mocks.NewMockPullRequestsService(ctrl)
		prs.EXPECT().UpdateBranch(ctx, pr.RepoOwner, pr.RepoName, pr.Number, gomock.AssignableToTypeOf(opt)).Return(nil, resp, nil)
		s.GithubClient.PullRequests = prs

		err := s.handleUpdateBranch(ctx, userHandle, pr)
		require.Nil(t, err)
	})

	t.Run("job scheduled on GitHub", func(t *testing.T) {
		s.OrgMembers = make([]string, 1)
		s.OrgMembers[0] = userHandle
		pr.FullName = organization + "/" + userHandle

		*msg = ""

		prs := mocks.NewMockPullRequestsService(ctrl)
		prs.EXPECT().UpdateBranch(ctx, pr.RepoOwner, pr.RepoName, pr.Number, gomock.AssignableToTypeOf(opt)).Return(nil, nil, errors.New("job scheduled on GitHub side; try again later"))
		s.GithubClient.PullRequests = prs

		err := s.handleUpdateBranch(ctx, userHandle, pr)
		require.Nil(t, err)
	})
}
