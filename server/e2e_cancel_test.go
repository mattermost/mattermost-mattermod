package server

import (
	"context"
	"reflect"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/google/go-github/v39/github"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-mattermod/server/mocks"
	"github.com/stretchr/testify/require"
)

func TestHandleE2ECanceling(t *testing.T) {
	ctrl := gomock.NewController(t)

	const userHandle = "user"
	const organization = "mattertest"
	s := Server{
		Config: &Config{
			Org:                   organization,
			E2EWebappReponame:     "mattermost-webapp",
			E2EServerReponame:     "mattermost-server",
			E2EEnterpriseReponame: "enterprise",
		},
		GithubClient: &GithubClient{},
	}

	ctx := context.Background()

	msg := new(string)
	comment := &github.IssueComment{Body: msg}
	is := mocks.NewMockIssuesService(ctrl)
	rs := mocks.NewMockRepositoriesService(ctrl)
	s.GithubClient.Issues = is
	s.GithubClient.Repositories = rs

	ctxInterface := reflect.TypeOf((*context.Context)(nil)).Elem()

	t.Run("random user", func(t *testing.T) {
		*msg = e2eCancelMsgCommenterPermission
		commentBody := "/e2e-cancel"
		pr := &model.PullRequest{
			RepoOwner: userHandle,
			RepoName:  s.Config.E2EWebappReponame,
			Number:    123,
		}
		is.EXPECT().CreateComment(gomock.AssignableToTypeOf(ctxInterface), pr.RepoOwner, pr.RepoName, pr.Number, comment).Times(1).Return(nil, nil, nil)
		err := s.handleE2ECancel(ctx, "someone", pr, commentBody)
		require.Error(t, err)
		require.IsType(t, &e2eCancelError{}, err)
		require.Equal(t, err.(*e2eCancelError).source, *msg)
	})
	t.Run("nothing to cancel", func(t *testing.T) {
		// todo
	})
	t.Run("pipeline canceled", func(t *testing.T) {
		// todo
	})
}
