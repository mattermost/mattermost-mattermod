// Copyright (c) 2019-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"reflect"
	"testing"

	"github.com/mattermost/mattermost-mattermod/model"
	srmock "github.com/mattermost/mattermost-mattermod/server/mocks"
	stmock "github.com/mattermost/mattermost-mattermod/store/mocks"

	"github.com/google/go-github/v93/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestAutoMergePR(t *testing.T) {
	ctrl := gomock.NewController(t)
	ctxInterface := reflect.TypeOf((*context.Context)(nil)).Elem()

	prs := []*model.PullRequest{
		{
			RepoOwner: "admin",
			RepoName:  "mattermod",
			Number:    42,
			Labels:    []string{"auto-merge"},
		},
	}
	prStoreMock := stmock.NewMockPullRequestStore(ctrl)
	prStoreMock.EXPECT().
		ListOpen().
		Return(prs, nil).
		AnyTimes()

	ss := stmock.NewMockStore(ctrl)
	ss.EXPECT().
		PullRequest().
		Return(prStoreMock).
		AnyTimes()

	metricsMock := srmock.NewMockMetricsProvider(ctrl)
	metricsMock.EXPECT().ObserveCronTaskDuration(gomock.Any(), gomock.Any()).AnyTimes()
	metricsMock.EXPECT().IncreaseCronTaskErrors(gomock.Any()).AnyTimes()

	t.Run("Basic", func(t *testing.T) {
		ghPR := &github.PullRequest{
			State:          new("open"),
			MergeableState: new("clean"),
			Head: &github.PullRequestBranch{
				SHA: new("sha"),
			},
		}

		ghStatus := &github.CombinedStatus{
			SHA:   new(ghPR.Head.GetSHA()),
			State: new("success"),
		}

		ghReviewers := &github.Reviewers{
			Users: []*github.User{},
			Teams: []*github.Team{},
		}

		prMergeResult := &github.PullRequestMergeResult{
			Message: new("merge message"),
			SHA:     new("sha"),
		}

		repoMock := srmock.NewMockRepositoriesService(ctrl)
		repoMock.EXPECT().
			GetCombinedStatus(gomock.AssignableToTypeOf(ctxInterface),
				gomock.Eq(prs[0].RepoOwner),
				gomock.Eq(prs[0].RepoName),
				gomock.Eq(ghPR.Head.GetSHA()),
				nil).
			Return(ghStatus, &github.Response{}, nil)

		prMock := srmock.NewMockPullRequestsService(ctrl)
		prMock.EXPECT().
			Get(gomock.AssignableToTypeOf(ctxInterface),
				gomock.Eq(prs[0].RepoOwner),
				gomock.Eq(prs[0].RepoName),
				gomock.Eq(prs[0].Number)).
			Return(ghPR, &github.Response{}, nil)
		prMock.EXPECT().
			ListReviewers(gomock.AssignableToTypeOf(ctxInterface),
				gomock.Eq(prs[0].RepoOwner),
				gomock.Eq(prs[0].RepoName),
				gomock.Eq(prs[0].Number)).
			Return(ghReviewers, &github.Response{}, nil)
		prOpts := &github.PullRequestOptions{}
		prMock.EXPECT().
			Merge(gomock.AssignableToTypeOf(ctxInterface),
				gomock.Eq(prs[0].RepoOwner),
				gomock.Eq(prs[0].RepoName),
				gomock.Eq(prs[0].Number),
				gomock.Eq("Automatic Merge"),
				gomock.AssignableToTypeOf(prOpts)).
			Return(prMergeResult, &github.Response{}, nil)

		issueMock := srmock.NewMockIssuesService(ctrl)
		comment := github.IssueCommentRequest{}
		issueMock.EXPECT().
			CreateComment(gomock.AssignableToTypeOf(ctxInterface),
				gomock.Eq(prs[0].RepoOwner),
				gomock.Eq(prs[0].RepoName),
				gomock.Eq(prs[0].Number),
				gomock.AssignableToTypeOf(comment)).
			MaxTimes(2)

		client := &GithubClient{
			PullRequests: prMock,
			Repositories: repoMock,
			Issues:       issueMock,
		}

		cfg := &Config{
			AutoPRMergeLabel: "auto-merge",
		}
		s := Server{
			GithubClient: client,
			Store:        ss,
			Config:       cfg,
			Metrics:      metricsMock,
		}

		err := s.AutoMergePR()
		require.NoError(t, err)
	})

	t.Run("Closed", func(t *testing.T) {
		ghPR := &github.PullRequest{
			State:          new("closed"),
			MergeableState: new("clean"),
			Head: &github.PullRequestBranch{
				SHA: new("sha"),
			},
		}

		prMock := srmock.NewMockPullRequestsService(ctrl)
		prMock.EXPECT().
			Get(gomock.AssignableToTypeOf(ctxInterface),
				gomock.Eq(prs[0].RepoOwner),
				gomock.Eq(prs[0].RepoName),
				gomock.Eq(prs[0].Number)).
			Return(ghPR, &github.Response{}, nil)

		client := &GithubClient{
			PullRequests: prMock,
		}

		cfg := &Config{
			AutoPRMergeLabel: "auto-merge",
		}
		s := Server{
			GithubClient: client,
			Store:        ss,
			Config:       cfg,
			Metrics:      metricsMock,
		}

		err := s.AutoMergePR()
		require.NoError(t, err)
	})

	// TODO: verify the log line once MM-26709 is done.
	t.Run("Unclean", func(t *testing.T) {
		ghPR := &github.PullRequest{
			State:          new("open"),
			MergeableState: new("unclean"),
			Head: &github.PullRequestBranch{
				SHA: new("sha"),
			},
		}

		prMock := srmock.NewMockPullRequestsService(ctrl)
		prMock.EXPECT().
			Get(gomock.AssignableToTypeOf(ctxInterface),
				gomock.Eq(prs[0].RepoOwner),
				gomock.Eq(prs[0].RepoName),
				gomock.Eq(prs[0].Number)).
			Return(ghPR, &github.Response{}, nil)

		client := &GithubClient{
			PullRequests: prMock,
		}

		cfg := &Config{
			AutoPRMergeLabel: "auto-merge",
		}
		s := Server{
			GithubClient: client,
			Store:        ss,
			Config:       cfg,
			Metrics:      metricsMock,
		}

		err := s.AutoMergePR()
		require.NoError(t, err)
	})

	// TODO: verify the log line once MM-26709 is done.
	t.Run("PendingReviewers", func(t *testing.T) {
		ghPR := &github.PullRequest{
			State:          new("open"),
			MergeableState: new("clean"),
			Head: &github.PullRequestBranch{
				SHA: new("sha"),
			},
		}

		ghStatus := &github.CombinedStatus{
			SHA:   new(ghPR.Head.GetSHA()),
			State: new("success"),
		}

		ghReviewers := &github.Reviewers{
			Users: []*github.User{{Name: new("name")}},
			Teams: []*github.Team{},
		}

		repoMock := srmock.NewMockRepositoriesService(ctrl)
		repoMock.EXPECT().
			GetCombinedStatus(gomock.AssignableToTypeOf(ctxInterface),
				gomock.Eq(prs[0].RepoOwner),
				gomock.Eq(prs[0].RepoName),
				gomock.Eq(ghPR.Head.GetSHA()),
				nil).
			Return(ghStatus, &github.Response{}, nil)

		prMock := srmock.NewMockPullRequestsService(ctrl)
		prMock.EXPECT().
			Get(gomock.AssignableToTypeOf(ctxInterface),
				gomock.Eq(prs[0].RepoOwner),
				gomock.Eq(prs[0].RepoName),
				gomock.Eq(prs[0].Number)).
			Return(ghPR, &github.Response{}, nil)
		prMock.EXPECT().
			ListReviewers(gomock.AssignableToTypeOf(ctxInterface),
				gomock.Eq(prs[0].RepoOwner),
				gomock.Eq(prs[0].RepoName),
				gomock.Eq(prs[0].Number)).
			Return(ghReviewers, &github.Response{}, nil)

		client := &GithubClient{
			PullRequests: prMock,
			Repositories: repoMock,
		}

		cfg := &Config{
			AutoPRMergeLabel: "auto-merge",
		}
		s := Server{
			GithubClient: client,
			Store:        ss,
			Config:       cfg,
			Metrics:      metricsMock,
		}

		err := s.AutoMergePR()
		require.NoError(t, err)
	})

	prs[0].Labels = []string{}

	t.Run("NoAuto-Merge", func(t *testing.T) {
		cfg := &Config{
			AutoPRMergeLabel: "auto-merge",
		}
		s := Server{
			Store:   ss,
			Config:  cfg,
			Metrics: metricsMock,
		}

		err := s.AutoMergePR()
		require.NoError(t, err)
	})
}

func TestHasAutoMerge(t *testing.T) {
	s := Server{
		Config: &Config{
			AutoPRMergeLabel: "label",
		},
	}

	t.Run("Positive", func(t *testing.T) {
		assert.True(t, s.hasAutoMerge([]string{"badlabel", s.Config.AutoPRMergeLabel}))
	})

	t.Run("Negative", func(t *testing.T) {
		assert.False(t, s.hasAutoMerge([]string{"badlabel"}))
	})
}
