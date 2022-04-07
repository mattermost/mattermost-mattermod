// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/google/go-github/v39/github"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-mattermod/server/mocks"
	"github.com/stretchr/testify/require"
)

func TestPerformFastForwardProcess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cloudRepo1 := "mattermost-server"

	s := &Server{
		GithubClient: &GithubClient{},
		Config: &Config{
			Repositories: []*Repository{
				{
					Name:               "mattermost-server",
					Owner:              "mattermosttest",
					BuildStatusContext: "something",
				},
			},
			Org:               "mattermosttest",
			CloudRepositories: []string{cloudRepo1},
		},
	}

	ctxInterface := reflect.TypeOf((*context.Context)(nil)).Elem()

	t.Run("Closed issue, do nothing and return", func(t *testing.T) {
		issue := &model.Issue{
			State: model.StateClosed,
		}

		ctx := context.Background()

		err := s.performFastForwardProcess(ctx, issue, "/cloud-ff")
		require.NoError(t, err)
	})

	t.Run("Not enough args, don't return error but add a comment", func(t *testing.T) {
		issue := &model.Issue{
			State:     model.StateOpen,
			RepoName:  "mattermost-server",
			RepoOwner: "mattermosttest",
		}
		is := mocks.NewMockIssuesService(ctrl)
		is.EXPECT().CreateComment(gomock.AssignableToTypeOf(ctxInterface), issue.RepoOwner, issue.RepoName, issue.Number, gomock.AssignableToTypeOf(reflect.TypeOf(&github.IssueComment{}))).
			Times(1).Return(nil, nil, nil)
		s.GithubClient.Issues = is

		ctx := context.Background()
		err := s.performFastForwardProcess(ctx, issue, "/cloud-ff")
		require.NoError(t, err)
	})

	t.Run("Cloud branch does not exist, will not create the backup but still going to create a new one", func(t *testing.T) {
		issue := &model.Issue{
			State:     model.StateOpen,
			RepoName:  "mattermost-server",
			RepoOwner: "mattermosttest",
		}
		gs := mocks.NewMockGitService(ctrl)
		gs.EXPECT().GetRef(gomock.AssignableToTypeOf(ctxInterface), s.Config.Org, cloudRepo1, cloudBranchName).Times(1).Return(nil, nil, errors.New("some-error"))
		gs.EXPECT().DeleteRef(gomock.AssignableToTypeOf(ctxInterface), s.Config.Org, cloudRepo1, cloudBranchName).Times(1).Return(nil, errors.New("some-error"))
		gs.EXPECT().GetRef(gomock.AssignableToTypeOf(ctxInterface), s.Config.Org, cloudRepo1, mainBranchName).Times(1).Return(&github.Reference{
			Object: &github.GitObject{
				SHA: github.String("some-random-sha"),
			},
		}, nil, nil)
		gs.EXPECT().CreateRef(gomock.AssignableToTypeOf(ctxInterface), s.Config.Org, cloudRepo1, &github.Reference{
			Ref: github.String(cloudBranchName),
			Object: &github.GitObject{
				SHA: github.String("some-random-sha"),
			},
		}).Times(1).Return(nil, nil, nil)
		s.GithubClient.Git = gs

		is := mocks.NewMockIssuesService(ctrl)
		is.EXPECT().CreateComment(gomock.AssignableToTypeOf(ctxInterface), issue.RepoOwner, issue.RepoName, issue.Number, gomock.AssignableToTypeOf(reflect.TypeOf(&github.IssueComment{}))).
			Times(1).Return(nil, nil, nil)
		s.GithubClient.Issues = is

		ctx := context.Background()
		err := s.performFastForwardProcess(ctx, issue, "/cloud-ff 2022-04-05")
		require.NoError(t, err)
	})

	t.Run("Cloud branch do exist, will create the backup but and going to finish fast forward process", func(t *testing.T) {
		issue := &model.Issue{
			State:     model.StateOpen,
			RepoName:  "mattermost-server",
			RepoOwner: "mattermosttest",
		}
		gs := mocks.NewMockGitService(ctrl)
		gs.EXPECT().GetRef(gomock.AssignableToTypeOf(ctxInterface), s.Config.Org, cloudRepo1, cloudBranchName).Times(1).Return(&github.Reference{
			Object: &github.GitObject{
				SHA: github.String("some-random-sha"),
			},
		}, nil, nil)
		gs.EXPECT().CreateRef(gomock.AssignableToTypeOf(ctxInterface), s.Config.Org, cloudRepo1, &github.Reference{
			Ref: github.String(cloudBranchName + "-2022-04-05-backup"),
			Object: &github.GitObject{
				SHA: github.String("some-random-sha"),
			},
		}).Times(1).Return(nil, nil, nil)
		gs.EXPECT().DeleteRef(gomock.AssignableToTypeOf(ctxInterface), s.Config.Org, cloudRepo1, cloudBranchName).Times(1).Return(nil, nil)
		gs.EXPECT().GetRef(gomock.AssignableToTypeOf(ctxInterface), s.Config.Org, cloudRepo1, mainBranchName).Times(1).Return(&github.Reference{
			Object: &github.GitObject{
				SHA: github.String("some-random-sha"),
			},
		}, nil, nil)
		gs.EXPECT().CreateRef(gomock.AssignableToTypeOf(ctxInterface), s.Config.Org, cloudRepo1, &github.Reference{
			Ref: github.String(cloudBranchName),
			Object: &github.GitObject{
				SHA: github.String("some-random-sha"),
			},
		}).Times(1).Return(nil, nil, nil)
		s.GithubClient.Git = gs

		is := mocks.NewMockIssuesService(ctrl)
		is.EXPECT().CreateComment(gomock.AssignableToTypeOf(ctxInterface), issue.RepoOwner, issue.RepoName, issue.Number, gomock.AssignableToTypeOf(reflect.TypeOf(&github.IssueComment{}))).
			Times(1).Return(nil, nil, nil)
		s.GithubClient.Issues = is

		ctx := context.Background()
		err := s.performFastForwardProcess(ctx, issue, "/cloud-ff 2022-04-05")
		require.NoError(t, err)
	})
}
