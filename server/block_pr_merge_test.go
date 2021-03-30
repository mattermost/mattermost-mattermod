// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/google/go-github/v33/github"
	srmock "github.com/mattermost/mattermost-mattermod/server/mocks"
	"github.com/stretchr/testify/require"

	"github.com/golang/mock/gomock"
	"github.com/mattermost/mattermost-mattermod/model"
)

func TestBlockPRMerge(t *testing.T) {
	ctrl := gomock.NewController(t)
	ctxInterface := reflect.TypeOf((*context.Context)(nil)).Elem()
	repoStatusType := reflect.TypeOf((*github.RepoStatus)(nil))
	repoMock := srmock.NewMockRepositoriesService(ctrl)
	client := &GithubClient{
		Repositories: repoMock,
	}
	cfg := &Config{
		BlockPRMergeLabels: []string{"test-block"},
	}
	s := Server{
		GithubClient: client,
		Config:       cfg,
	}

	t.Run("Should not change status if the PR is closed", func(t *testing.T) {
		pr := createExamplePR(model.StateClosed, []string{})
		repoMock.EXPECT().CreateStatus(
			gomock.AssignableToTypeOf(ctxInterface),
			gomock.Eq("testuser"),
			gomock.Eq("testrepo"),
			gomock.Eq("testsha"),
			gomock.AssignableToTypeOf(repoStatusType),
		).Times(0)
		err := s.blockPRMerge(context.TODO(), pr)
		require.NoError(t, err)
	})

	t.Run("Should change commit status to pending", func(t *testing.T) {
		pr := createExamplePR(model.StateOpen, []string{"test-block"})
		repoMock.EXPECT().CreateStatus(
			gomock.AssignableToTypeOf(ctxInterface),
			gomock.Eq("testuser"),
			gomock.Eq("testrepo"),
			gomock.Eq("testsha"),
			gomock.Eq(createRepoStatus(
				statePending,
			)),
		).Times(1).Return(&github.RepoStatus{}, &github.Response{}, nil)
		err := s.blockPRMerge(context.TODO(), pr)
		require.NoError(t, err)
	})

	t.Run("Should return error if not possible to change status", func(t *testing.T) {
		pr := createExamplePR(model.StateOpen, []string{"test-block"})
		repoMock.EXPECT().CreateStatus(
			gomock.AssignableToTypeOf(ctxInterface),
			gomock.Eq("testuser"),
			gomock.Eq("testrepo"),
			gomock.Eq("testsha"),
			gomock.Eq(createRepoStatus(
				statePending,
			)),
		).Times(1).Return(&github.RepoStatus{}, &github.Response{}, errors.New("error setting status"))
		err := s.blockPRMerge(context.TODO(), pr)
		require.Error(t, err)
	})
}

func TestUnblockPRMerge(t *testing.T) {
	ctrl := gomock.NewController(t)
	ctxInterface := reflect.TypeOf((*context.Context)(nil)).Elem()
	repoStatusType := reflect.TypeOf((*github.RepoStatus)(nil))
	repoMock := srmock.NewMockRepositoriesService(ctrl)
	client := &GithubClient{
		Repositories: repoMock,
	}
	cfg := &Config{
		BlockPRMergeLabels: []string{"test-block"},
	}
	s := Server{
		GithubClient: client,
		Config:       cfg,
	}

	t.Run("Should not change status if the PR is closed", func(t *testing.T) {
		pr := createExamplePR(model.StateClosed, []string{})
		repoMock.EXPECT().CreateStatus(
			gomock.AssignableToTypeOf(ctxInterface),
			gomock.Eq("testuser"),
			gomock.Eq("testrepo"),
			gomock.Eq("testsha"),
			gomock.AssignableToTypeOf(repoStatusType),
		).Times(0)
		err := s.unblockPRMerge(context.TODO(), pr)
		require.NoError(t, err)
	})

	t.Run("Should change commit status to success", func(t *testing.T) {
		pr := createExamplePR(model.StateOpen, []string{})
		repoMock.EXPECT().CreateStatus(
			gomock.AssignableToTypeOf(ctxInterface),
			gomock.Eq("testuser"),
			gomock.Eq("testrepo"),
			gomock.Eq("testsha"),
			gomock.Eq(createRepoStatus(
				stateSuccess,
			)),
		).Times(1).Return(&github.RepoStatus{}, &github.Response{}, nil)
		err := s.unblockPRMerge(context.TODO(), pr)
		require.NoError(t, err)
	})

	t.Run("Should return error if not possible to change status", func(t *testing.T) {
		pr := createExamplePR(model.StateOpen, []string{})
		repoMock.EXPECT().CreateStatus(
			gomock.AssignableToTypeOf(ctxInterface),
			gomock.Eq("testuser"),
			gomock.Eq("testrepo"),
			gomock.Eq("testsha"),
			gomock.Eq(createRepoStatus(
				stateSuccess,
			)),
		).Times(1).Return(&github.RepoStatus{}, &github.Response{}, errors.New("error setting status"))
		err := s.unblockPRMerge(context.TODO(), pr)
		require.Error(t, err)
	})
}

func createExamplePR(state string, labels []string) *model.PullRequest {
	return &model.PullRequest{
		RepoOwner: "testuser",
		RepoName:  "testrepo",
		Sha:       "testsha",
		Labels:    labels,
		State:     state,
	}
}

func createRepoStatus(state string) *github.RepoStatus {
	var description string
	if state == statePending {
		description = fmt.Sprintf("Merge blocked due %s label", "test-block")
	} else {
		description = "Merged allowed"
	}
	return &github.RepoStatus{
		Context:     github.String("merge/blocked"),
		State:       github.String(state),
		Description: github.String(description),
		TargetURL:   github.String(""),
	}
}
