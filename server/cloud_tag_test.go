// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/google/go-github/v39/github"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-mattermod/server/mocks"
	stmock "github.com/mattermost/mattermost-mattermod/store/mocks"
	"github.com/stretchr/testify/require"
)

const (
	cloudRepo1 = "mattermost-server"
	member     = "member-a"
)

func TestCreateCloudTag(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

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
			CloudRepositories: []*CloudRepository{{Name: cloudRepo1}},
		},
		OrgMembers: []string{
			member,
		},
	}
	ctxInterface := reflect.TypeOf((*context.Context)(nil)).Elem()

	lockStoreMock := stmock.NewMockLockStore(ctrl)
	lockStoreMock.EXPECT().Lock(context.Background()).Return(nil).AnyTimes()
	lockStoreMock.EXPECT().Unlock().Return(nil).AnyTimes()

	ss := stmock.NewMockStore(ctrl)
	ss.EXPECT().
		Mutex().
		Return(lockStoreMock).
		AnyTimes()

	s.Store = ss

	t.Run("Closed issue, do nothing and return", func(t *testing.T) {
		issue := &model.Issue{
			State: model.StateClosed,
		}

		ctx := context.Background()

		_, err := s.createCloudTag(ctx, issue, "/cloud-tag", member)
		require.NoError(t, err)
	})

	t.Run("Not org member, do nothing and return", func(t *testing.T) {
		issue := &model.Issue{
			State: model.StateClosed,
		}

		ctx := context.Background()

		_, err := s.createCloudTag(ctx, issue, "/cloud-tag", "contributor-a")
		require.NoError(t, err)
	})

	t.Run("Cloud branch exist, will create the tag", func(t *testing.T) {
		issue := &model.Issue{
			State:     model.StateOpen,
			RepoName:  "mattermost-server",
			RepoOwner: "mattermosttest",
		}
		gs := mocks.NewMockGitService(ctrl)
		s.GithubClient.Git = gs
		gs.EXPECT().GetRef(gomock.AssignableToTypeOf(ctxInterface), s.Config.Org, cloudRepo1, cloudBranchName).Times(1).Return(&github.Reference{
			Ref: github.String("heads/cloud"),
			Object: &github.GitObject{
				SHA: github.String("some-random-sha"),
			},
		}, nil, nil)
		gs.EXPECT().CreateRef(gomock.AssignableToTypeOf(ctxInterface), s.Config.Org, cloudRepo1, &github.Reference{
			Ref: github.String("tags/cloud-" + time.Now().Format("2006-01-02") + "-1"),
			Object: &github.GitObject{
				Type: github.String("tag"),
				SHA:  github.String("some-random-sha"),
			},
		}).Times(1).Return(&github.Reference{
			Ref: github.String("tags/cloud-" + time.Now().Format("2006-01-02") + "-1"),
			Object: &github.GitObject{
				Type: github.String("tag"),
				SHA:  github.String("some-random-sha"),
			},
		}, nil, nil)

		ctx := context.Background()
		res, err := s.createCloudTag(ctx, issue, "/cloud-tag", member)
		require.NoError(t, err)
		require.Len(t, res.Tagged, 1)
		require.Len(t, res.Skipped, 0)
	})

	t.Run("Tag already exist, will delete then create the new tag", func(t *testing.T) {
		issue := &model.Issue{
			State:     model.StateOpen,
			RepoName:  "mattermost-server",
			RepoOwner: "mattermosttest",
		}
		gs := mocks.NewMockGitService(ctrl)
		s.GithubClient.Git = gs
		gs.EXPECT().GetRef(gomock.AssignableToTypeOf(ctxInterface), s.Config.Org, cloudRepo1, cloudBranchName).Times(1).Return(&github.Reference{
			Ref: github.String("heads/cloud"),
			Object: &github.GitObject{
				SHA: github.String("some-random-sha"),
			},
		}, nil, nil)
		gs.EXPECT().DeleteRef(gomock.AssignableToTypeOf(ctxInterface), s.Config.Org, cloudRepo1, "tags/cloud-"+time.Now().Format("2006-01-02")+"-1").Times(1).Return(nil, nil)
		gs.EXPECT().CreateRef(gomock.AssignableToTypeOf(ctxInterface), s.Config.Org, cloudRepo1, &github.Reference{
			Ref: github.String("tags/cloud-" + time.Now().Format("2006-01-02") + "-1"),
			Object: &github.GitObject{
				Type: github.String("tag"),
				SHA:  github.String("some-random-sha"),
			},
		}).Times(1).Return(&github.Reference{
			Ref: github.String("tags/cloud-" + time.Now().Format("2006-01-02") + "-1"),
			Object: &github.GitObject{
				Type: github.String("tag"),
				SHA:  github.String("some-random-sha"),
			},
		}, nil, nil)

		ctx := context.Background()
		res, err := s.createCloudTag(ctx, issue, "/cloud-tag --force", member)
		require.NoError(t, err)
		require.Len(t, res.Tagged, 1)
		require.Len(t, res.Skipped, 0)
	})

	t.Run("Dry run, will just return", func(t *testing.T) {
		issue := &model.Issue{
			State:     model.StateOpen,
			RepoName:  "mattermost-server",
			RepoOwner: "mattermosttest",
		}
		gs := mocks.NewMockGitService(ctrl)
		s.GithubClient.Git = gs
		gs.EXPECT().GetRef(gomock.AssignableToTypeOf(ctxInterface), s.Config.Org, cloudRepo1, cloudBranchName).Times(1).Return(&github.Reference{
			Ref: github.String("heads/cloud"),
			Object: &github.GitObject{
				SHA: github.String("some-random-sha"),
			},
		}, nil, nil)

		ctx := context.Background()
		res, err := s.createCloudTag(ctx, issue, "/cloud-tag --dry-run", member)
		require.NoError(t, err)
		require.Len(t, res.Tagged, 1)
		require.Len(t, res.Skipped, 0)
	})
}
