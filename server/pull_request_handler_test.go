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
	"time"

	"github.com/golang/mock/gomock"
	"github.com/google/go-github/v32/github"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-mattermod/server/mocks"
	stmock "github.com/mattermost/mattermost-mattermod/store/mocks"
)

func TestPullRequestEventHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	s := &Server{
		GithubClient: &GithubClient{},
		Config: &Config{
			Repositories: []*Repository{
				{
					Name:               "mattermod",
					Owner:              "mattertest",
					BuildStatusContext: "something",
				},
			},
		},
	}
	rs := mocks.NewMockRepositoriesService(ctrl)
	s.GithubClient.Repositories = rs

	cs := mocks.NewMockChecksService(ctrl)
	s.GithubClient.Checks = cs

	is := mocks.NewMockIssuesService(ctrl)
	s.GithubClient.Issues = is

	ctxInterface := reflect.TypeOf((*context.Context)(nil)).Elem()

	prStoreMock := stmock.NewMockPullRequestStore(ctrl)
	ss := stmock.NewMockStore(ctrl)
	ss.EXPECT().
		PullRequest().
		Return(prStoreMock).
		AnyTimes()

	s.Store = ss

	event := pullRequestEvent{
		Action:   "",
		PRNumber: 1,
		PullRequest: &github.PullRequest{
			Number: github.Int(1),
			Base: &github.PullRequestBranch{
				Repo: &github.Repository{
					Owner: &github.User{
						Login: github.String("mattertest"),
					},
					Name: github.String("mattermod"),
				},
			},
			Head: &github.PullRequestBranch{
				SHA: github.String("sha"),
			},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(s.pullRequestEventHandler))
	defer ts.Close()

	t.Run("Should fail with no body", func(t *testing.T) {
		req, err := http.NewRequest("POST", ts.URL, nil)
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("Should fail on not finding the PR from GitHub", func(t *testing.T) {
		rs.EXPECT().
			GetCombinedStatus(gomock.AssignableToTypeOf(ctxInterface), "mattertest", "mattermod", "sha", nil).
			Times(1).
			Return(nil, nil, errors.New("some-error"))

		b, err := json.Marshal(event)
		require.NoError(t, err)

		req, err := http.NewRequest("POST", ts.URL, bytes.NewReader(b))
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("Should be able to get PR from GitHub (new PR)", func(t *testing.T) {
		rs.EXPECT().
			GetCombinedStatus(gomock.AssignableToTypeOf(ctxInterface), "mattertest", "mattermod", "sha", nil).
			Times(1).
			Return(&github.CombinedStatus{
				Statuses: []*github.RepoStatus{
					{
						Context: github.String("something"),
					},
				},
			}, nil, nil)

		cs.EXPECT().
			ListCheckRunsForRef(gomock.AssignableToTypeOf(ctxInterface), "mattertest", "mattermod", "sha", nil).
			Times(1).
			Return(&github.ListCheckRunsResults{}, nil, nil)

		is.EXPECT().
			ListLabelsByIssue(gomock.AssignableToTypeOf(ctxInterface), "mattertest", "mattermod", 1, nil).
			Times(1).
			Return([]*github.Label{}, nil, nil)

		prStoreMock.EXPECT().Save(gomock.AssignableToTypeOf(&model.PullRequest{})).
			Times(1).Return(nil, nil)

		prStoreMock.EXPECT().Get("mattertest", "mattermod", 1).
			Times(1).Return(nil, nil)

		prStoreMock.EXPECT().Save(gomock.AssignableToTypeOf(&model.PullRequest{})).
			Times(1).Return(nil, nil)

		b, err := json.Marshal(event)
		require.NoError(t, err)

		req, err := http.NewRequest("POST", ts.URL, bytes.NewReader(b))
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("Error when checking PR for changes", func(t *testing.T) {
		rs.EXPECT().
			GetCombinedStatus(gomock.AssignableToTypeOf(ctxInterface), "mattertest", "mattermod", "sha", nil).
			Times(1).
			Return(&github.CombinedStatus{
				Statuses: []*github.RepoStatus{
					{
						Context: github.String("something"),
					},
				},
			}, nil, nil)

		cs.EXPECT().
			ListCheckRunsForRef(gomock.AssignableToTypeOf(ctxInterface), "mattertest", "mattermod", "sha", nil).
			Times(1).
			Return(&github.ListCheckRunsResults{}, nil, nil)

		is.EXPECT().
			ListLabelsByIssue(gomock.AssignableToTypeOf(ctxInterface), "mattertest", "mattermod", 1, nil).
			Times(1).
			Return([]*github.Label{}, nil, nil)

		prStoreMock.EXPECT().Save(gomock.AssignableToTypeOf(&model.PullRequest{})).
			Times(1).Return(nil, nil)

		prStoreMock.EXPECT().Get("mattertest", "mattermod", 1).
			Times(1).Return(nil, errors.New("some-error"))

		b, err := json.Marshal(event)
		require.NoError(t, err)

		req, err := http.NewRequest("POST", ts.URL, bytes.NewReader(b))
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("PR has changes", func(t *testing.T) {
		rs.EXPECT().
			GetCombinedStatus(gomock.AssignableToTypeOf(ctxInterface), "mattertest", "mattermod", "sha", nil).
			Times(1).
			Return(&github.CombinedStatus{
				Statuses: []*github.RepoStatus{
					{
						Context: github.String("something"),
					},
				},
			}, nil, nil)

		cs.EXPECT().
			ListCheckRunsForRef(gomock.AssignableToTypeOf(ctxInterface), "mattertest", "mattermod", "sha", nil).
			Times(1).
			Return(&github.ListCheckRunsResults{}, nil, nil)

		is.EXPECT().
			ListLabelsByIssue(gomock.AssignableToTypeOf(ctxInterface), "mattertest", "mattermod", 1, nil).
			Times(1).
			Return([]*github.Label{}, nil, nil)

		is.EXPECT().
			ListComments(gomock.AssignableToTypeOf(ctxInterface), "mattertest", "mattermod", 1, gomock.AssignableToTypeOf(&github.IssueListCommentsOptions{})).
			Times(1).
			Return([]*github.IssueComment{}, &github.Response{
				NextPage: 0,
				Response: &http.Response{
					StatusCode: http.StatusOK,
				},
			}, nil)

		prStoreMock.EXPECT().Save(gomock.AssignableToTypeOf(&model.PullRequest{})).
			Times(1).Return(nil, nil)

		prStoreMock.EXPECT().Get("mattertest", "mattermod", 1).
			Times(1).Return(&model.PullRequest{
			RepoOwner:           "mattertest",
			RepoName:            "mattermod",
			CreatedAt:           time.Time{},
			Labels:              []string{"old-label"},
			Sha:                 "sha",
			MaintainerCanModify: NewBool(false),
			Merged:              NewBool(false),
		}, nil)

		prStoreMock.EXPECT().Save(gomock.AssignableToTypeOf(&model.PullRequest{})).
			Times(1).Return(nil, nil)

		b, err := json.Marshal(event)
		require.NoError(t, err)

		req, err := http.NewRequest("POST", ts.URL, bytes.NewReader(b))
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})

	testPRHasChanges := func(t *testing.T, modelPR *model.PullRequest, githubPR *github.PullRequest, expectedSaveCalls int) {
		t.Helper()
		rs.EXPECT().
			GetCombinedStatus(gomock.AssignableToTypeOf(ctxInterface), "mattertest", "mattermod", "sha", nil).
			Times(1).
			Return(&github.CombinedStatus{
				Statuses: []*github.RepoStatus{
					{
						Context: github.String("something"),
					},
				},
			}, nil, nil)

		cs.EXPECT().
			ListCheckRunsForRef(gomock.AssignableToTypeOf(ctxInterface), "mattertest", "mattermod", "sha", nil).
			Times(1).
			Return(&github.ListCheckRunsResults{}, nil, nil)

		is.EXPECT().
			ListLabelsByIssue(gomock.AssignableToTypeOf(ctxInterface), "mattertest", "mattermod", 1, nil).
			Times(1).
			Return([]*github.Label{{Name: NewString("old-label")}}, nil, nil)

		prStoreMock.EXPECT().Get("mattertest", "mattermod", 1).
			Times(1).Return(modelPR, nil)

		prStoreMock.EXPECT().Save(gomock.AssignableToTypeOf(&model.PullRequest{})).
			Times(expectedSaveCalls).Return(nil, nil)

		e := pullRequestEvent{
			Action:      "",
			PRNumber:    modelPR.Number,
			PullRequest: githubPR,
		}
		b, err := json.Marshal(e)
		require.NoError(t, err)

		req, err := http.NewRequest("POST", ts.URL, bytes.NewReader(b))
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
	}

	modelPR := &model.PullRequest{
		Number:              1,
		RepoOwner:           "mattertest",
		RepoName:            "mattermod",
		Labels:              []string{"old-label"},
		Sha:                 "sha",
		Merged:              NewBool(true),
		MilestoneNumber:     NewInt64(0),
		MilestoneTitle:      NewString(""),
		MaintainerCanModify: NewBool(true),
	}

	t.Run("PR doesn't have changes if all the values are the same", func(t *testing.T) {
		githubPR := &github.PullRequest{
			Number: &modelPR.Number,
			Base: &github.PullRequestBranch{
				Repo: &github.Repository{
					Owner: &github.User{Login: &modelPR.RepoOwner},
					Name:  &modelPR.RepoName,
				},
			},
			Merged: modelPR.Merged,
			Head:   &github.PullRequestBranch{SHA: &modelPR.Sha},
			Milestone: &github.Milestone{
				Number: NewInt(int(modelPR.GetMilestoneNumber())),
				Title:  modelPR.MilestoneTitle,
			},
			MaintainerCanModify: modelPR.MaintainerCanModify,
		}

		testPRHasChanges(t, modelPR, githubPR, 1)
	})

	t.Run("PR has changes if the values of MaintainerCanModify are different", func(t *testing.T) {
		githubPR := &github.PullRequest{
			Number: &modelPR.Number,
			Base: &github.PullRequestBranch{
				Repo: &github.Repository{
					Owner: &github.User{Login: &modelPR.RepoOwner},
					Name:  &modelPR.RepoName,
				},
			},
			Merged: modelPR.Merged,
			Head:   &github.PullRequestBranch{SHA: &modelPR.Sha},
			Milestone: &github.Milestone{
				Number: NewInt(int(modelPR.GetMilestoneNumber())),
				Title:  modelPR.MilestoneTitle,
			},
			MaintainerCanModify: NewBool(!modelPR.GetMaintainerCanModify()),
		}

		testPRHasChanges(t, modelPR, githubPR, 2)
	})

	t.Run("PR has changes if the values of Merged are different", func(t *testing.T) {
		githubPR := &github.PullRequest{
			Number: &modelPR.Number,
			Base: &github.PullRequestBranch{
				Repo: &github.Repository{
					Owner: &github.User{Login: &modelPR.RepoOwner},
					Name:  &modelPR.RepoName,
				},
			},
			Merged: NewBool(!modelPR.GetMerged()),
			Head:   &github.PullRequestBranch{SHA: &modelPR.Sha},
			Milestone: &github.Milestone{
				Number: NewInt(int(modelPR.GetMilestoneNumber())),
				Title:  modelPR.MilestoneTitle,
			},
			MaintainerCanModify: modelPR.MaintainerCanModify,
		}

		testPRHasChanges(t, modelPR, githubPR, 2)
	})

	t.Run("PR has changes if the values of MilestoneNumber are different", func(t *testing.T) {
		githubPR := &github.PullRequest{
			Number: &modelPR.Number,
			Base: &github.PullRequestBranch{
				Repo: &github.Repository{
					Owner: &github.User{Login: &modelPR.RepoOwner},
					Name:  &modelPR.RepoName,
				},
			},
			Merged: modelPR.Merged,
			Head:   &github.PullRequestBranch{SHA: &modelPR.Sha},
			Milestone: &github.Milestone{
				Number: NewInt(int(modelPR.GetMilestoneNumber() + 1)),
				Title:  modelPR.MilestoneTitle,
			},
			MaintainerCanModify: modelPR.MaintainerCanModify,
		}

		testPRHasChanges(t, modelPR, githubPR, 2)
	})

	t.Run("PR has changes if the values of MilestoneTitle are different", func(t *testing.T) {
		githubPR := &github.PullRequest{
			Number: &modelPR.Number,
			Base: &github.PullRequestBranch{
				Repo: &github.Repository{
					Owner: &github.User{Login: &modelPR.RepoOwner},
					Name:  &modelPR.RepoName,
				},
			},
			Merged: modelPR.Merged,
			Head:   &github.PullRequestBranch{SHA: &modelPR.Sha},
			Milestone: &github.Milestone{
				Number: NewInt(int(modelPR.GetMilestoneNumber())),
				Title:  NewString(modelPR.GetMilestoneTitle() + "moretext"),
			},
			MaintainerCanModify: modelPR.MaintainerCanModify,
		}

		testPRHasChanges(t, modelPR, githubPR, 2)
	})
}

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
				issueMocks := mocks.NewMockIssuesService(ctrl)
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
				issueMocks := mocks.NewMockIssuesService(ctrl)
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
				issueMocks := mocks.NewMockIssuesService(ctrl)
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
	metricsMock := mocks.NewMockMetricsProvider(ctrl)
	metricsMock.EXPECT().ObserveCronTaskDuration(gomock.Any(), gomock.Any()).AnyTimes()
	metricsMock.EXPECT().IncreaseCronTaskErrors(gomock.Any()).AnyTimes()
	cfg := &Config{
		DaysUntilStale: 1,
	}

	t.Run("should check for activity but not mark PRs as stale", func(t *testing.T) {
		prServiceMock := mocks.NewMockPullRequestsService(ctrl)
		issuesServiceMock := mocks.NewMockIssuesService(ctrl)
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
		prServiceMock := mocks.NewMockPullRequestsService(ctrl)
		issuesServiceMock := mocks.NewMockIssuesService(ctrl)
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
		prServiceMock := mocks.NewMockPullRequestsService(ctrl)
		issuesServiceMock := mocks.NewMockIssuesService(ctrl)
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
		prServiceMock := mocks.NewMockPullRequestsService(ctrl)
		issuesServiceMock := mocks.NewMockIssuesService(ctrl)
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
