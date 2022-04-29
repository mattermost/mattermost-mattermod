// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
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

func TestPerformFastForwardProcess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cloudRepo1 := "mattermost-server"
	member := "member-a"

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

		_, err := s.performFastForwardProcess(ctx, issue, "/cloud-ff", member)
		require.NoError(t, err)
	})

	t.Run("Not org member, do nothing and return", func(t *testing.T) {
		issue := &model.Issue{
			State: model.StateClosed,
		}

		ctx := context.Background()

		_, err := s.performFastForwardProcess(ctx, issue, "/cloud-ff", "contributor-a")
		require.NoError(t, err)
	})

	t.Run("Cloud branch does not exist, will not create the backup but still going to create a new one", func(t *testing.T) {
		issue := &model.Issue{
			State:     model.StateOpen,
			RepoName:  "mattermost-server",
			RepoOwner: "mattermosttest",
		}
		gs := mocks.NewMockGitService(ctrl)
		s.GithubClient.Git = gs
		gs.EXPECT().ListMatchingRefs(gomock.AssignableToTypeOf(ctxInterface), s.Config.Org, cloudRepo1, &github.ReferenceListOptions{
			Ref: cloudBranchName,
		}).Times(1).Return(nil, nil, nil)
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

		ctx := context.Background()
		res, err := s.performFastForwardProcess(ctx, issue, "/cloud-ff", member)
		require.NoError(t, err)
		require.Len(t, res.Backup, 0)
		require.Len(t, res.FastForwarded, 1)
	})

	t.Run("Cloud branch do exist, will not create the backup and skip", func(t *testing.T) {
		issue := &model.Issue{
			State:     model.StateOpen,
			RepoName:  "mattermost-server",
			RepoOwner: "mattermosttest",
		}
		gs := mocks.NewMockGitService(ctrl)
		s.GithubClient.Git = gs
		gs.EXPECT().ListMatchingRefs(gomock.AssignableToTypeOf(ctxInterface), s.Config.Org, cloudRepo1, &github.ReferenceListOptions{
			Ref: cloudBranchName,
		}).Times(1).Return([]*github.Reference{
			{
				Ref: github.String("heads/cloud-" + time.Now().Format("2006-01-02") + "-backup"),
				Object: &github.GitObject{
					SHA: github.String("some-random-sha"),
				},
			},
		}, nil, nil)
		now := time.Now()
		gs.EXPECT().GetCommit(gomock.AssignableToTypeOf(ctxInterface), s.Config.Org, cloudRepo1, "some-random-sha").Times(1).Return(&github.Commit{
			Author: &github.CommitAuthor{
				Date: &now,
			},
		}, nil, nil)

		ctx := context.Background()
		res, err := s.performFastForwardProcess(ctx, issue, "/cloud-ff", member)
		require.NoError(t, err)
		require.Len(t, res.Backup, 0)
		require.Len(t, res.Skipped, 1)
		require.Len(t, res.FastForwarded, 0)
	})

	t.Run("Provided dry-run flag, won't call any create ref methods", func(t *testing.T) {
		issue := &model.Issue{
			State:     model.StateOpen,
			RepoName:  "mattermost-server",
			RepoOwner: "mattermosttest",
		}
		gs := mocks.NewMockGitService(ctrl)
		s.GithubClient.Git = gs
		gs.EXPECT().ListMatchingRefs(gomock.AssignableToTypeOf(ctxInterface), s.Config.Org, cloudRepo1, &github.ReferenceListOptions{
			Ref: cloudBranchName,
		}).Times(1).Return([]*github.Reference{
			{
				Ref: github.String("heads/cloud-" + time.Now().Format("2006-01-02") + "-backup"),
				Object: &github.GitObject{
					SHA: github.String("some-random-sha"),
				},
			},
		}, nil, nil)
		now := time.Now().Add(-1 * 6 * 24 * time.Hour)
		gs.EXPECT().GetCommit(gomock.AssignableToTypeOf(ctxInterface), s.Config.Org, cloudRepo1, "some-random-sha").Times(1).Return(&github.Commit{
			Author: &github.CommitAuthor{
				Date: &now,
			},
		}, nil, nil)
		gs.EXPECT().GetRef(gomock.AssignableToTypeOf(ctxInterface), s.Config.Org, cloudRepo1, cloudBranchName).Times(1).Return(&github.Reference{
			Object: &github.GitObject{
				SHA: github.String("some-random-sha"),
			},
		}, nil, nil)

		gs.EXPECT().GetRef(gomock.AssignableToTypeOf(ctxInterface), s.Config.Org, cloudRepo1, mainBranchName).Times(1).Return(&github.Reference{
			Object: &github.GitObject{
				SHA: github.String("some-random-sha"),
			},
		}, nil, nil)

		ctx := context.Background()
		fmt.Println("1")
		res, err := s.performFastForwardProcess(ctx, issue, "/cloud-ff --dry-run", member)
		require.NoError(t, err)
		require.Len(t, res.Backup, 1)
		require.Len(t, res.Skipped, 0)
		require.Len(t, res.FastForwarded, 1)
		require.Equal(t, res.DryRun, true)
	})

	t.Run("Cloud branch do exist, force flag provided will create the backup and fast forward", func(t *testing.T) {
		issue := &model.Issue{
			State:     model.StateOpen,
			RepoName:  "mattermost-server",
			RepoOwner: "mattermosttest",
		}
		gs := mocks.NewMockGitService(ctrl)
		s.GithubClient.Git = gs
		gs.EXPECT().ListMatchingRefs(gomock.AssignableToTypeOf(ctxInterface), s.Config.Org, cloudRepo1, &github.ReferenceListOptions{
			Ref: cloudBranchName,
		}).Times(1).Return([]*github.Reference{
			{
				Ref: github.String("heads/cloud-" + time.Now().Format("2006-01-02") + "-backup"),
				Object: &github.GitObject{
					SHA: github.String("some-random-sha"),
				},
			},
		}, nil, nil)
		now := time.Now()
		gs.EXPECT().GetCommit(gomock.AssignableToTypeOf(ctxInterface), s.Config.Org, cloudRepo1, "some-random-sha").Times(1).Return(&github.Commit{
			Author: &github.CommitAuthor{
				Date: &now,
			},
		}, nil, nil)
		gs.EXPECT().GetRef(gomock.AssignableToTypeOf(ctxInterface), s.Config.Org, cloudRepo1, cloudBranchName).Times(1).Return(&github.Reference{
			Object: &github.GitObject{
				SHA: github.String("some-random-sha"),
			},
		}, nil, nil)
		gs.EXPECT().CreateRef(gomock.AssignableToTypeOf(ctxInterface), s.Config.Org, cloudRepo1, &github.Reference{
			Ref: github.String(cloudBranchName + "-" + time.Now().Format("2006-01-02") + "-backup"),
			Object: &github.GitObject{
				SHA: github.String("some-random-sha"),
			},
		}).Times(1).Return(nil, &github.Response{Response: &http.Response{
			StatusCode: http.StatusUnprocessableEntity,
		}}, fmt.Errorf("some-error"))
		gs.EXPECT().DeleteRef(gomock.AssignableToTypeOf(ctxInterface), s.Config.Org, cloudRepo1, cloudBranchName+"-"+time.Now().Format("2006-01-02")+"-backup").Times(1).Return(nil, nil)
		gs.EXPECT().CreateRef(gomock.AssignableToTypeOf(ctxInterface), s.Config.Org, cloudRepo1, &github.Reference{
			Ref: github.String(cloudBranchName + "-" + time.Now().Format("2006-01-02") + "-backup"),
			Object: &github.GitObject{
				SHA: github.String("some-random-sha"),
			},
		}).Times(1).Return(&github.Reference{
			Ref: github.String(cloudBranchName + "-" + time.Now().Format("2006-01-02") + "-backup"),
			Object: &github.GitObject{
				SHA: github.String("some-random-sha"),
			},
		}, nil, nil)
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

		ctx := context.Background()
		res, err := s.performFastForwardProcess(ctx, issue, "/cloud-ff --force", member)
		require.NoError(t, err)
		require.Len(t, res.Backup, 1)
		require.Len(t, res.Skipped, 0)
		require.Len(t, res.FastForwarded, 1)
	})
}
