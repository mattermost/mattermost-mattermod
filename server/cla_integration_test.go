package server_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/google/go-github/v31/github"

	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-mattermod/server"
	"github.com/mattermost/mattermost-mattermod/server/mocks"
	"github.com/stretchr/testify/assert"
)

const (
	claContext   = "cla"
	otherContext = "other"

	statePending = "pending"
	stateSuccess = "success"
)

func TestIsAlreadySigned(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repoMocks := mocks.NewMockRepositoriesService(ctrl)
	mockedClient := &server.GithubClient{
		Repositories: repoMocks,
	}

	statuses := make([]*github.RepoStatus, 2)
	statuses[0] = &github.RepoStatus{
		State:       github.String(statePending),
		TargetURL:   github.String(""),
		Description: github.String(""),
		Context:     github.String(otherContext),
	}
	statuses[1] = &github.RepoStatus{
		State:       github.String(stateSuccess),
		TargetURL:   github.String(""),
		Description: github.String(""),
		Context:     github.String(claContext),
	}

	pr := &model.PullRequest{
		Number:    1,
		Username:  "userName",
		RepoOwner: "repoOwner",
		RepoName:  "repoName",
		Sha:       "sha",
	}

	r := &http.Response{StatusCode: http.StatusOK}
	ghR := &github.Response{
		Response: r,
		NextPage: 0,
	}

	ctx := context.Background()
	repoMocks.EXPECT().ListStatuses(gomock.Eq(ctx), gomock.Eq(pr.RepoOwner), gomock.Eq(pr.RepoName), gomock.Eq(pr.Sha), nil).Return(statuses, ghR, nil)

	s := &server.Server{
		Config: &server.Config{
			CLAGithubStatusContext: claContext,
		},
		GithubClient: mockedClient,
	}

	assert.True(t, s.IsAlreadySigned(context.Background(), pr))
}

func TestIsNotAlreadySigned(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repoMocks := mocks.NewMockRepositoriesService(ctrl)
	mockedClient := &server.GithubClient{
		Repositories: repoMocks,
	}

	statuses := make([]*github.RepoStatus, 2)
	statuses[0] = &github.RepoStatus{
		State:       github.String(stateSuccess),
		TargetURL:   github.String(""),
		Description: github.String(""),
		Context:     github.String(otherContext),
	}
	statuses[1] = &github.RepoStatus{
		State:       github.String(statePending),
		TargetURL:   github.String(""),
		Description: github.String(""),
		Context:     github.String(claContext),
	}

	pr := &model.PullRequest{
		Number:    1,
		Username:  "userName",
		RepoOwner: "repoOwner",
		RepoName:  "repoName",
		Sha:       "sha",
	}

	r := &http.Response{StatusCode: http.StatusOK}
	ghR := &github.Response{
		Response: r,
		NextPage: 0,
	}

	ctx := context.Background()
	repoMocks.EXPECT().ListStatuses(gomock.Eq(ctx), gomock.Eq(pr.RepoOwner), gomock.Eq(pr.RepoName), gomock.Eq(pr.Sha), nil).Return(statuses, ghR, nil)

	s := &server.Server{
		Config: &server.Config{
			CLAGithubStatusContext: claContext,
		},
		GithubClient: mockedClient,
	}

	assert.False(t, s.IsAlreadySigned(context.Background(), pr))
}
