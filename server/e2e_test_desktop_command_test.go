package server

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/xanzy/go-gitlab"

	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-mattermod/server/mocks"

	"github.com/golang/mock/gomock"
	"github.com/google/go-github/v39/github"
	"github.com/stretchr/testify/require"
)

func TestHandleE2ETestingDesktop(t *testing.T) {
	ctrl := gomock.NewController(t)

	const userHandle = "user"
	const organization = "mattertest"
	s := Server{
		Config: &Config{
			Org:                     organization,
			E2EDesktopGitLabProject: "qa%2Fdesktop-e2e-testing",
			E2EDesktopCoreReponame:  "desktop",
		},
		GithubClient:     &GithubClient{},
		GitLabCIClientV4: &GitLabClient{},
	}

	ctx := context.Background()

	msg := new(string)
	comment := &github.IssueComment{Body: msg}
	is := mocks.NewMockIssuesService(ctrl)
	rs := mocks.NewMockRepositoriesService(ctrl)
	prs := mocks.NewMockPullRequestsService(ctrl)
	glPS := mocks.NewMockPipelinesService(ctrl)
	s.GithubClient.Issues = is
	s.GithubClient.Repositories = rs
	s.GithubClient.PullRequests = prs
	s.GitLabCIClientV4.Pipelines = glPS

	gCtxInterface := reflect.TypeOf(gitlab.RequestOptionFunc(nil))
	ctxInterface := reflect.TypeOf((*context.Context)(nil)).Elem()

	t.Run("happy trigger from desktop core without options", func(t *testing.T) {
		s.OrgMembers = make([]string, 1)
		s.OrgMembers[0] = userHandle
		pr := &model.PullRequest{
			RepoOwner: userHandle,
			RepoName:  s.Config.E2EDesktopCoreReponame,
			Number:    prNumber,
			Ref:       eBranch,
			Sha:       eSHA,
		}
		pr.FullName = organization + "/" + userHandle
		rs.EXPECT().
			GetCombinedStatus(gomock.AssignableToTypeOf(ctxInterface), s.Config.Org, s.Config.E2EDesktopCoreReponame, gomock.Any(), nil).
			Times(1).
			Return(&github.CombinedStatus{
				State: github.String(statePending),
			}, nil, nil)

		p := &gitlab.Pipeline{WebURL: "https://your.gitlab.com/project/-/pipelines/54004"}
		glPS.EXPECT().CreatePipeline(s.Config.E2EDesktopGitLabProject, gomock.Any(), gomock.AssignableToTypeOf(gCtxInterface)).Times(1).Return(p, nil, nil)
		commentEnd := &github.IssueComment{Body: github.String(fmt.Sprintf(e2eTestDesktopFmtSuccess, e2eTestDesktopMsgSuccess, p.WebURL))}
		is.EXPECT().CreateComment(gomock.AssignableToTypeOf(ctxInterface), pr.RepoOwner, pr.RepoName, pr.Number, commentEnd).Times(1).Return(nil, nil, nil)
		err := s.handleE2ETestDesktop(ctx, userHandle, pr)
		require.NoError(t, err)
	})
	t.Run("random user", func(t *testing.T) {
		*msg = e2eTestDesktopMsgCommenterPermission
		pr := &model.PullRequest{
			RepoOwner: userHandle,
			RepoName:  s.Config.E2EDesktopCoreReponame,
			Number:    123,
		}
		is.EXPECT().CreateComment(gomock.AssignableToTypeOf(ctxInterface), pr.RepoOwner, pr.RepoName, pr.Number, comment).Times(1).Return(nil, nil, nil)
		err := s.handleE2ETestDesktop(ctx, "someone", pr)
		require.Error(t, err)
		require.IsType(t, &E2ETestDesktopError{}, err)
		require.Equal(t, err.(*E2ETestDesktopError).source, *msg)
	})
	t.Run("pr not ready", func(t *testing.T) {
		*msg = e2eTestDesktopMsgCIFailing
		s.OrgMembers = make([]string, 1)
		s.OrgMembers[0] = userHandle
		pr := &model.PullRequest{
			RepoOwner: userHandle,
			RepoName:  s.Config.E2EDesktopCoreReponame,
			Number:    123,
		}
		pr.FullName = organization + "/" + userHandle
		rs.EXPECT().
			GetCombinedStatus(gomock.AssignableToTypeOf(ctxInterface), s.Config.Org, s.Config.E2EDesktopCoreReponame, gomock.Any(), nil).
			Times(1).
			Return(&github.CombinedStatus{
				State: github.String(stateError), // statePending can run e2e-test (block-pr-merge.go)
			}, nil, nil)
		is.EXPECT().CreateComment(gomock.AssignableToTypeOf(ctxInterface), pr.RepoOwner, pr.RepoName, pr.Number, comment).Times(1).Return(nil, nil, nil)
		err := s.handleE2ETestDesktop(ctx, userHandle, pr)
		require.Error(t, err)
		require.IsType(t, &E2ETestDesktopError{}, err)
		require.Equal(t, err.(*E2ETestDesktopError).source, *msg)
	})
}
