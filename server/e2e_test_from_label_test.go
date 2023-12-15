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

	"github.com/mattermost/mattermost-mattermod/model"
	stmock "github.com/mattermost/mattermost-mattermod/store/mocks"

	"github.com/golang/mock/gomock"
	"github.com/google/go-github/v39/github"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-mattermod/server/mocks"
)

func TestE2EFromLabelWorkflow(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repoOwner := "mattertest"
	repo := "mattermost-webapp"
	sha := "sha"
	s := &Server{
		GithubClient: &GithubClient{},
		Config: &Config{
			Repositories: []*Repository{
				{
					Name:               repo,
					Owner:              repoOwner,
					BuildStatusContext: "something",
				},
			},
			Org:               repoOwner,
			E2ETriggerLabel:   []string{"Run E2E Testing"},
			E2EWebappReponame: repo,
			E2EServerReponame: repo,
		},
	}
	s.OrgMembers = make([]string, 1)
	allowedUser := "mattermod"
	s.OrgMembers[0] = allowedUser

	eventAction := prEventLabeled
	event := pullRequestEvent{
		Action:   eventAction,
		Label:    &github.Label{},
		PRNumber: 1,
		PullRequest: &github.PullRequest{
			Number: github.Int(1),
			Base: &github.PullRequestBranch{
				Repo: &github.Repository{
					Owner: &github.User{
						Login: github.String(repoOwner),
					},
					Name: github.String(repo),
				},
			},
			Head: &github.PullRequestBranch{
				SHA: github.String(sha),
			},
		},
	}
	prState := "open"
	prGhModel := &github.PullRequest{
		Labels: []*github.Label{event.Label},
		State:  &prState,
	}

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

	setUpCommonMocks := func() {
		rs.EXPECT().
			GetCombinedStatus(gomock.AssignableToTypeOf(ctxInterface), repoOwner, repo, sha, nil).
			AnyTimes().
			Return(&github.CombinedStatus{
				Statuses: []*github.RepoStatus{
					{
						Context: github.String("something"),
						State:   github.String(stateSuccess),
					},
				},
			}, nil, nil)

		cs.EXPECT().
			ListCheckRunsForRef(gomock.AssignableToTypeOf(ctxInterface), repoOwner, repo, sha, nil).
			Return(&github.ListCheckRunsResults{}, nil, nil)

		is.EXPECT().
			ListLabelsByIssue(gomock.AssignableToTypeOf(ctxInterface), repoOwner, repo, 1, nil).
			Return([]*github.Label{}, nil, nil)

		prStoreMock.EXPECT().Save(gomock.AssignableToTypeOf(&model.PullRequest{})).
			Return(nil, nil)

		prStoreMock.EXPECT().Get(repoOwner, repo, 1).
			Return(&model.PullRequest{
				RepoOwner:           repoOwner,
				RepoName:            repo,
				CreatedAt:           time.Time{},
				Labels:              []string{"old-label"},
				Sha:                 sha,
				MaintainerCanModify: NewBool(false),
				Merged:              NewBool(false),
			}, nil)

		prStoreMock.EXPECT().Save(gomock.AssignableToTypeOf(&model.PullRequest{})).
			Return(nil, nil)
	}
	runTestEvent := func() {
		b, err := json.Marshal(event)
		require.NoError(t, err)

		req, err := http.NewRequest("POST", ts.URL, bytes.NewReader(b))
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		// The handler function is asynchronous, so give it time to run.
		time.Sleep(200 * time.Millisecond)
	}

	t.Run("happy path", func(t *testing.T) {
		msgHappyPath := e2eTestMsgCommenterPermission
		commentHappyPath := &github.IssueComment{Body: &msgHappyPath}
		event.Label.Name = github.String(s.Config.E2ETriggerLabel[0])
		prGhModel.MergeableState = github.String("clean")

		setUpCommonMocks()

		prs.EXPECT().Get(gomock.AssignableToTypeOf(ctxInterface), repoOwner, repo, event.PRNumber).
			Return(prGhModel, nil, nil).
			Times(1)

		// Let's not repeat all the e2e_test_command_test.go happy path steps and instead
		// throw a permission denied error, so we know handleE2ETest can be called
		s.Config.Username = "wronguser"
		is.EXPECT().
			CreateComment(gomock.AssignableToTypeOf(ctxInterface), repoOwner, repo, 1, commentHappyPath).
			Times(1).
			Return(nil, nil, nil)
		runTestEvent()
	})
	t.Run("event has wrong label", func(t *testing.T) {
		event.Label.Name = github.String("NotTheValidLabel")

		setUpCommonMocks()

		runTestEvent()
	})
	t.Run("PR not mergeable", func(t *testing.T) {
		msgNotMergeable := e2eTestFromLabelMsgPRNotMergeable
		commentNotMergeable := &github.IssueComment{Body: &msgNotMergeable}
		event.Label.Name = github.String(s.Config.E2ETriggerLabel[0])
		prGhModel.MergeableState = github.String("unclean")

		setUpCommonMocks()

		prs.EXPECT().Get(gomock.AssignableToTypeOf(ctxInterface), repoOwner, repo, event.PRNumber).
			Return(prGhModel, nil, nil).
			Times(1)

		is.EXPECT().
			CreateComment(gomock.AssignableToTypeOf(ctxInterface), repoOwner, repo, 1, commentNotMergeable).
			Times(1)

		runTestEvent()
	})
}
