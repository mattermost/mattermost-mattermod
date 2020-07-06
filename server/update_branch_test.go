// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/google/go-github/v32/github"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-mattermod/server/mocks"
	"github.com/stretchr/testify/require"
)

func TestCheckUpdatePullRequestPermissions(t *testing.T) {
	const userHandle = "user"
	const organization = "some-organization"
	s := Server{
		Config: &Config{
			Org: organization,
		},
	}

	t.Run("random user", func(t *testing.T) {
		err := s.checkUpdatePullRequestPermissions("someone", &model.PullRequest{
			RepoOwner: userHandle,
		})
		require.Error(t, err)
		require.EqualError(t, ErrCommenterPermission, err.Error())
	})

	t.Run("not org. member", func(t *testing.T) {
		err := s.checkUpdatePullRequestPermissions(userHandle, &model.PullRequest{
			RepoOwner: userHandle,
		})
		require.Error(t, err)
		require.EqualError(t, ErrCommenterPermission, err.Error())
	})

	t.Run("app does not have permissions", func(t *testing.T) {
		s.OrgMembers = make([]string, 1)
		s.OrgMembers[0] = userHandle

		err := s.checkUpdatePullRequestPermissions(userHandle, &model.PullRequest{
			RepoOwner: userHandle,
		})
		require.Error(t, err)
		require.EqualError(t, ErrOrganizationPermission, err.Error())
	})

	t.Run("organization member", func(t *testing.T) {
		s.OrgMembers = make([]string, 1)
		s.OrgMembers[0] = organization + "/" + userHandle

		err := s.checkUpdatePullRequestPermissions(organization+"/"+userHandle, &model.PullRequest{
			RepoOwner: organization + "/" + userHandle,
			Username:  organization + "/" + userHandle,
		})
		require.NoError(t, err)
	})

	t.Run("non-organization member, maintainer can modify", func(t *testing.T) {
		s.OrgMembers = make([]string, 1)
		s.OrgMembers[0] = userHandle

		err := s.checkUpdatePullRequestPermissions(userHandle, &model.PullRequest{
			RepoOwner:           userHandle,
			MaintainerCanModify: true,
		})
		require.NoError(t, err)
	})
}

func TestUpdatePullRequest(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userHandle := "user"
	repoName := "repo-name"
	prNumber := 123

	opt := &github.PullRequestBranchUpdateOptions{
		ExpectedHeadSHA: github.String("123abc345efg"),
	}
	s := Server{
		GithubClient: &GithubClient{},
	}

	ctx := context.Background()

	t.Run("err from github api", func(t *testing.T) {
		prs := mocks.NewMockPullRequestsService(ctrl)
		prs.EXPECT().UpdateBranch(ctx, userHandle, repoName, prNumber, gomock.AssignableToTypeOf(opt)).Return(nil, nil, errors.New("some-error"))
		s.GithubClient.PullRequests = prs

		err := s.updatePullRequest(context.Background(), &model.PullRequest{
			RepoOwner: userHandle,
			RepoName:  repoName,
			Number:    prNumber,
		})
		require.Error(t, err)
		require.EqualError(t, ErrUpdatePullRequest, err.Error())
	})

	t.Run("non-OK status code from github", func(t *testing.T) {
		resp := &github.Response{
			Response: &http.Response{
				StatusCode: http.StatusNotFound,
			},
		}

		prs := mocks.NewMockPullRequestsService(ctrl)
		prs.EXPECT().UpdateBranch(ctx, userHandle, repoName, prNumber, gomock.AssignableToTypeOf(opt)).Return(nil, resp, nil)
		s.GithubClient.PullRequests = prs

		err := s.updatePullRequest(context.Background(), &model.PullRequest{
			RepoOwner: userHandle,
			RepoName:  repoName,
			Number:    prNumber,
		})
		require.Error(t, err)
		require.EqualError(t, ErrUpdatePullRequest, err.Error())
	})

	t.Run("accepted by github", func(t *testing.T) {
		resp := &github.Response{
			Response: &http.Response{
				StatusCode: http.StatusAccepted,
			},
		}

		prs := mocks.NewMockPullRequestsService(ctrl)
		prs.EXPECT().UpdateBranch(ctx, userHandle, repoName, prNumber, gomock.AssignableToTypeOf(opt)).Return(nil, resp, nil)
		s.GithubClient.PullRequests = prs

		err := s.updatePullRequest(context.Background(), &model.PullRequest{
			RepoOwner: userHandle,
			RepoName:  repoName,
			Number:    prNumber,
		})
		require.NoError(t, err)
	})
}
