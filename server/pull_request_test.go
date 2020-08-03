package server

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/google/go-github/v32/github"

	"github.com/mattermost/mattermost-mattermod/model"
	srvmock "github.com/mattermost/mattermost-mattermod/server/mocks"
	stmock "github.com/mattermost/mattermost-mattermod/store/mocks"
)

func TestCleanUpLabels(t *testing.T) {
	pr := &model.PullRequest{
		RepoOwner: "owner",
		RepoName:  "repoName",
		Number:    123,
	}

	ctxInterface := reflect.TypeOf((*context.Context)(nil)).Elem()

	for name, test := range map[string]struct {
		SetupClient func(*gomock.Controller) *GithubClient
	}{
		"no label has to be removed": {
			SetupClient: func(ctrl *gomock.Controller) *GithubClient {
				issueMocks := srvmock.NewMockIssuesService(ctrl)
				client := &GithubClient{
					Issues: issueMocks,
				}

				labels := []*github.Label{{
					Name: github.String("abc"),
				}, {
					Name: github.String("def"),
				}}

				issueMocks.EXPECT().ListLabelsByIssue(gomock.AssignableToTypeOf(ctxInterface),
					gomock.Eq(pr.RepoOwner), gomock.Eq(pr.RepoName),
					gomock.Eq(pr.Number), nil).Return(labels, nil, nil)

				return client
			},
		},
		"all labels have to be removed": {
			SetupClient: func(ctrl *gomock.Controller) *GithubClient {
				issueMocks := srvmock.NewMockIssuesService(ctrl)
				client := &GithubClient{
					Issues: issueMocks,
				}

				labels := []*github.Label{{
					Name: github.String("AutoMerge"),
				}, {
					Name: github.String("Do Not Merge"),
				}, {
					Name: github.String("Work In Progress"),
				}}

				issueMocks.EXPECT().ListLabelsByIssue(gomock.AssignableToTypeOf(ctxInterface), gomock.Eq(pr.RepoOwner), gomock.Eq(pr.RepoName), gomock.Eq(pr.Number), nil).Return(labels, nil, nil)
				issueMocks.EXPECT().RemoveLabelForIssue(gomock.AssignableToTypeOf(ctxInterface), gomock.Eq(pr.RepoOwner), gomock.Eq(pr.RepoName), gomock.Eq(pr.Number), gomock.Eq("AutoMerge")).Return(nil, nil)
				issueMocks.EXPECT().RemoveLabelForIssue(gomock.AssignableToTypeOf(ctxInterface), gomock.Eq(pr.RepoOwner), gomock.Eq(pr.RepoName), gomock.Eq(pr.Number), gomock.Eq("Do Not Merge")).Return(nil, nil)
				issueMocks.EXPECT().RemoveLabelForIssue(gomock.AssignableToTypeOf(ctxInterface), gomock.Eq(pr.RepoOwner), gomock.Eq(pr.RepoName), gomock.Eq(pr.Number), gomock.Eq("Work In Progress")).Return(nil, nil)

				return client
			},
		},
		"some labels have to be removed": {
			SetupClient: func(ctrl *gomock.Controller) *GithubClient {
				issueMocks := srvmock.NewMockIssuesService(ctrl)
				client := &GithubClient{
					Issues: issueMocks,
				}

				labels := []*github.Label{{
					Name: github.String("Work In Progress"),
				}, {
					Name: github.String("abc"),
				}, {
					Name: github.String("AutoMerge"),
				}, {
					Name: github.String("def"),
				}}

				issueMocks.EXPECT().ListLabelsByIssue(gomock.AssignableToTypeOf(ctxInterface), gomock.Eq(pr.RepoOwner), gomock.Eq(pr.RepoName), gomock.Eq(pr.Number), nil).Return(labels, nil, nil)
				issueMocks.EXPECT().RemoveLabelForIssue(gomock.AssignableToTypeOf(ctxInterface), gomock.Eq(pr.RepoOwner), gomock.Eq(pr.RepoName), gomock.Eq(pr.Number), gomock.Eq("AutoMerge")).Return(nil, nil)
				issueMocks.EXPECT().RemoveLabelForIssue(gomock.AssignableToTypeOf(ctxInterface), gomock.Eq(pr.RepoOwner), gomock.Eq(pr.RepoName), gomock.Eq(pr.Number), gomock.Eq("Work In Progress")).Return(nil, nil)

				return client
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			defer ctrl.Finish()

			s := &Server{
				Config: &Config{
					IssueLabelsToCleanUp: []string{"AutoMerge", "Do Not Merge", "Work In Progress"},
				},
				GithubClient: test.SetupClient(ctrl),
			}
			s.CleanUpLabels(pr)
		})
	}
}

func TestCheckPRActivity(t *testing.T) {
	pr1 := &model.PullRequest{
		RepoOwner: "owner",
		RepoName:  "repoName",
		Number:    123,
	}
	pr2 := &model.PullRequest{
		RepoOwner: "owner",
		RepoName:  "repoName",
		Number:    456,
	}
	updatedAt := time.Now()
	ghPR := &github.PullRequest{
		State:     github.String("open"),
		UpdatedAt: &updatedAt,
	}
	ghPR2 := &github.PullRequest{
		State:     github.String("open"),
		UpdatedAt: &updatedAt,
	}
	ctrl := gomock.NewController(t)
	ctxInterface := reflect.TypeOf((*context.Context)(nil)).Elem()
	defer ctrl.Finish()
	prStoreMock := stmock.NewMockPullRequestStore(ctrl)
	prStoreMock.EXPECT().ListOpen().AnyTimes().Return([]*model.PullRequest{pr1, pr2}, nil)
	ss := stmock.NewMockStore(ctrl)
	ss.EXPECT().
		PullRequest().
		Return(prStoreMock).
		AnyTimes()
	metricsMock := srvmock.NewMockMetricsProvider(ctrl)
	metricsMock.EXPECT().ObserveCronTaskDuration(gomock.Any(), gomock.Any()).AnyTimes()
	metricsMock.EXPECT().IncreaseCronTaskErrors(gomock.Any()).AnyTimes()
	cfg := &Config{
		DaysUntilStale: 1,
	}

	t.Run("should check for activity but not mark PRs as stale", func(t *testing.T) {
		prServiceMock := srvmock.NewMockPullRequestsService(ctrl)
		issuesServiceMock := srvmock.NewMockIssuesService(ctrl)
		client := &GithubClient{
			PullRequests: prServiceMock,
			Issues:       issuesServiceMock,
		}
		s := &Server{
			GithubClient: client,
			Config:       cfg,
			Store:        ss,
			Metrics:      metricsMock,
		}
		prServiceMock.EXPECT().Get(gomock.AssignableToTypeOf(ctxInterface),
			"owner",
			"repoName",
			123).Times(1).Return(ghPR, nil, nil)
		prServiceMock.EXPECT().Get(gomock.AssignableToTypeOf(ctxInterface),
			"owner",
			"repoName",
			456).Times(1).Return(ghPR2, nil, nil)
		issuesServiceMock.EXPECT().AddLabelsToIssue(gomock.AssignableToTypeOf(ctxInterface),
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
			gomock.Any()).Times(0)
		s.CheckPRActivity()
	})

	t.Run("should continue with the other PRs if one of them fails", func(t *testing.T) {
		prServiceMock := srvmock.NewMockPullRequestsService(ctrl)
		issuesServiceMock := srvmock.NewMockIssuesService(ctrl)
		client := &GithubClient{
			PullRequests: prServiceMock,
			Issues:       issuesServiceMock,
		}
		s := &Server{
			GithubClient: client,
			Config:       cfg,
			Store:        ss,
			Metrics:      metricsMock,
		}
		prServiceMock.EXPECT().Get(gomock.AssignableToTypeOf(ctxInterface),
			"owner",
			"repoName",
			123).Times(1).Return(nil, nil, errors.New("fails"))
		prServiceMock.EXPECT().Get(gomock.AssignableToTypeOf(ctxInterface),
			"owner",
			"repoName",
			456).Times(1).Return(ghPR2, nil, nil)
		issuesServiceMock.EXPECT().AddLabelsToIssue(gomock.AssignableToTypeOf(ctxInterface),
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
			gomock.Any()).Times(0)
		s.CheckPRActivity()
	})

	t.Run("should set the stale label ever if one PR fails", func(t *testing.T) {
		prServiceMock := srvmock.NewMockPullRequestsService(ctrl)
		issuesServiceMock := srvmock.NewMockIssuesService(ctrl)
		client := &GithubClient{
			PullRequests: prServiceMock,
			Issues:       issuesServiceMock,
		}
		cfg := &Config{
			DaysUntilStale: 0,
		}
		s := &Server{
			GithubClient: client,
			Config:       cfg,
			Store:        ss,
			Metrics:      metricsMock,
		}
		prServiceMock.EXPECT().Get(gomock.AssignableToTypeOf(ctxInterface),
			"owner",
			"repoName",
			123).Times(1).Return(ghPR, nil, nil)
		prServiceMock.EXPECT().Get(gomock.AssignableToTypeOf(ctxInterface),
			"owner",
			"repoName",
			456).Times(1).Return(ghPR2, nil, nil)
		issuesServiceMock.EXPECT().ListLabelsByIssue(gomock.AssignableToTypeOf(ctxInterface),
			gomock.Any(),
			gomock.Any(),
			123,
			nil).Times(1).Return(nil, nil, errors.New("fails"))
		issuesServiceMock.EXPECT().ListLabelsByIssue(gomock.AssignableToTypeOf(ctxInterface),
			gomock.Any(),
			gomock.Any(),
			456,
			nil).Times(1).Return([]*github.Label{}, nil, nil)
		issuesServiceMock.EXPECT().AddLabelsToIssue(gomock.AssignableToTypeOf(ctxInterface),
			gomock.Any(),
			gomock.Any(),
			456,
			gomock.Any()).Times(1)
		issuesServiceMock.EXPECT().CreateComment(gomock.AssignableToTypeOf(ctxInterface),
			"owner",
			"repoName",
			456,
			gomock.Any()).Times(1)
		s.CheckPRActivity()
	})

	t.Run("should not mark as stale if the exempt stale label is set", func(t *testing.T) {
		prServiceMock := srvmock.NewMockPullRequestsService(ctrl)
		issuesServiceMock := srvmock.NewMockIssuesService(ctrl)
		client := &GithubClient{
			PullRequests: prServiceMock,
			Issues:       issuesServiceMock,
		}
		cfg := &Config{
			DaysUntilStale:    0,
			ExemptStaleLabels: []string{"nostale"},
		}
		s := &Server{
			GithubClient: client,
			Config:       cfg,
			Store:        ss,
			Metrics:      metricsMock,
		}
		prServiceMock.EXPECT().Get(gomock.AssignableToTypeOf(ctxInterface),
			"owner",
			"repoName",
			123).Times(1).Return(ghPR, nil, nil)
		prServiceMock.EXPECT().Get(gomock.AssignableToTypeOf(ctxInterface),
			"owner",
			"repoName",
			456).Times(1).Return(ghPR2, nil, nil)
		issuesServiceMock.EXPECT().ListLabelsByIssue(gomock.AssignableToTypeOf(ctxInterface),
			gomock.Any(),
			gomock.Any(),
			123,
			nil).Times(1).Return([]*github.Label{}, nil, nil)
		issuesServiceMock.EXPECT().ListLabelsByIssue(gomock.AssignableToTypeOf(ctxInterface),
			gomock.Any(),
			gomock.Any(),
			456,
			nil).Times(1).Return([]*github.Label{{Name: github.String("nostale")}}, nil, nil)
		issuesServiceMock.EXPECT().AddLabelsToIssue(gomock.AssignableToTypeOf(ctxInterface),
			gomock.Any(),
			gomock.Any(),
			123,
			gomock.Any()).Times(1)
		issuesServiceMock.EXPECT().CreateComment(gomock.AssignableToTypeOf(ctxInterface),
			"owner",
			"repoName",
			123,
			gomock.Any()).Times(1)
		s.CheckPRActivity()
	})
}
