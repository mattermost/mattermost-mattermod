package server

import (
	"bytes"
        "context"
	"encoding/json"
        "net/http"
        "net/http/httptest"
        "reflect"
        "testing"
	"time"

        "github.com/golang/mock/gomock"
        "github.com/google/go-github/v39/github"
        "github.com/stretchr/testify/require"

        "github.com/mattermost/mattermost-mattermod/model"
        "github.com/mattermost/mattermost-mattermod/server/mocks"
        stmock "github.com/mattermost/mattermost-mattermod/store/mocks"

)

func TestE2EQAWorkflow(t *testing.T) {
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
	s.Config.Org = "mattertest"
	s.Config.E2ETriggerLabel = "QA review/deferred"

        rs := mocks.NewMockRepositoriesService(ctrl)
        s.GithubClient.Repositories = rs

        cs := mocks.NewMockChecksService(ctrl)
        s.GithubClient.Checks = cs

        is := mocks.NewMockIssuesService(ctrl)
        s.GithubClient.Issues = is

        prs := mocks.NewMockPullRequestsService(ctrl)
        s.GithubClient.PullRequests = prs

        ctxInterface := reflect.TypeOf((*context.Context)(nil)).Elem()

        prStoreMock := stmock.NewMockPullRequestStore(ctrl)

        ss := stmock.NewMockStore(ctrl)
        ss.EXPECT().
                PullRequest().
                Return(prStoreMock).
                AnyTimes()

        s.Store = ss

        ts := httptest.NewServer(http.HandlerFunc(s.pullRequestEventHandler))
        defer ts.Close()

        t.Run("Event should trigger E2E test", func(t *testing.T) {
		eventAction := "labeled"
		event := pullRequestEvent{
		        Action: eventAction,
			Label: &github.Label{
				Name: &s.Config.E2ETriggerLabel,
			},
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
			Sender: &pullRequestEventSender{
				Login: "ghUser",
			},
		}

                rs.EXPECT().
                        GetCombinedStatus(gomock.AssignableToTypeOf(ctxInterface), "mattertest", "mattermod", "sha", nil).
			AnyTimes().
                        Return(&github.CombinedStatus{
                                Statuses: []*github.RepoStatus{
                                        {
                                                Context: github.String("something"),
                                        },
                                },
                        }, nil, nil)

                cs.EXPECT().
                        ListCheckRunsForRef(gomock.AssignableToTypeOf(ctxInterface), "mattertest", "mattermod", "sha", nil).
                        Return(&github.ListCheckRunsResults{}, nil, nil)

                is.EXPECT().
                        ListLabelsByIssue(gomock.AssignableToTypeOf(ctxInterface), "mattertest", "mattermod", 1, nil).
                        Return([]*github.Label{}, nil, nil)

		prState := "open"
		prMergeableState := "clean"
		prApprovalState := "approved"
		prApprovalReviews := []*github.PullRequestReview{
			&github.PullRequestReview{
                                State: &prApprovalState,
                        },
		}
		prGhModel := github.PullRequest{
			Labels: []*github.Label{event.Label},
			State: &prState,
			MergeableState: &prMergeableState,
		}
		prs.EXPECT().
			ListReviews(gomock.AssignableToTypeOf(ctxInterface), "mattertest", "mattermod", 1, nil).
			Return(prApprovalReviews, nil, nil)
		prs.EXPECT().
			Get(gomock.AssignableToTypeOf(ctxInterface), "mattertest", "mattermod", 1).
			Return(&prGhModel, nil, nil)

                prStoreMock.EXPECT().Save(gomock.AssignableToTypeOf(&model.PullRequest{})).
                        Return(nil, nil)

                prStoreMock.EXPECT().Get("mattertest", "mattermod", 1).
                        Return(&model.PullRequest{
                        RepoOwner:           "mattertest",
                        RepoName:            "mattermod",
                        CreatedAt:           time.Time{},
                        Labels:              []string{"old-label"},
                        Sha:                 "sha",
                        MaintainerCanModify: NewBool(false),
                        Merged:              NewBool(false),
                }, nil)

                prStoreMock.EXPECT().Save(gomock.AssignableToTypeOf(&model.PullRequest{})).
                        Return(nil, nil)

                b, err := json.Marshal(event)
                require.NoError(t, err)

                req, err := http.NewRequest("POST", ts.URL, bytes.NewReader(b))
                require.NoError(t, err)
                resp, err := http.DefaultClient.Do(req)
                require.NoError(t, err)
                defer resp.Body.Close()
                require.Equal(t, http.StatusOK, resp.StatusCode)
        })
}
