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

func TestHandleE2ETestingMobile(t *testing.T) {
	ctrl := gomock.NewController(t)

	const userHandle = "user"
	const organization = "mattertest"
	s := Server{
		Config: &Config{
			Org:                    organization,
			E2EMobileGitLabProject: "qa%2Fmobile-e2e-testing-ee",
			E2EMobileCoreReponame:  "mattermost-mobile",
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

	t.Run("happy trigger from mobile core without options", func(t *testing.T) {
		s.OrgMembers = make([]string, 1)
		s.OrgMembers[0] = userHandle
		pr := &model.PullRequest{
			RepoOwner: userHandle,
			RepoName:  s.Config.E2EMobileCoreReponame,
			Number:    prNumber,
			Ref:       eBranch,
			Sha:       eSHA,
		}
		pr.FullName = organization + "/" + userHandle
		rs.EXPECT().
			GetCombinedStatus(gomock.AssignableToTypeOf(ctxInterface), s.Config.Org, s.Config.E2EMobileCoreReponame, gomock.Any(), nil).
			Times(1).
			Return(&github.CombinedStatus{
				State: github.String(statePending),
			}, nil, nil)

		p := &gitlab.Pipeline{WebURL: "https://your.gitlab.com/project/-/pipelines/54004"}
		glPS.EXPECT().CreatePipeline(s.Config.E2EMobileGitLabProject, gomock.Any(), gomock.AssignableToTypeOf(gCtxInterface)).Times(1).Return(p, nil, nil)
		commentEnd := &github.IssueComment{Body: github.String(fmt.Sprintf(e2eTestMobileFmtSuccess, e2eTestMobileMsgSuccess, p.WebURL))}
		is.EXPECT().CreateComment(gomock.AssignableToTypeOf(ctxInterface), pr.RepoOwner, pr.RepoName, pr.Number, commentEnd).Times(1).Return(nil, nil, nil)
		err := s.handleE2ETestMobile(ctx, userHandle, pr)
		require.NoError(t, err)
	})
	t.Run("random user", func(t *testing.T) {
		*msg = e2eTestMobileMsgCommenterPermission
		pr := &model.PullRequest{
			RepoOwner: userHandle,
			RepoName:  s.Config.E2EMobileCoreReponame,
			Number:    123,
		}
		is.EXPECT().CreateComment(gomock.AssignableToTypeOf(ctxInterface), pr.RepoOwner, pr.RepoName, pr.Number, comment).Times(1).Return(nil, nil, nil)
		err := s.handleE2ETestMobile(ctx, "someone", pr)
		require.Error(t, err)
		require.IsType(t, &E2ETestMobileError{}, err)
		require.Equal(t, err.(*E2ETestMobileError).source, *msg)
	})
	t.Run("pr not ready", func(t *testing.T) {
		*msg = e2eTestMobileMsgCIFailing
		s.OrgMembers = make([]string, 1)
		s.OrgMembers[0] = userHandle
		pr := &model.PullRequest{
			RepoOwner: userHandle,
			RepoName:  s.Config.E2EMobileCoreReponame,
			Number:    123,
		}
		pr.FullName = organization + "/" + userHandle
		rs.EXPECT().
			GetCombinedStatus(gomock.AssignableToTypeOf(ctxInterface), s.Config.Org, s.Config.E2EMobileCoreReponame, gomock.Any(), nil).
			Times(1).
			Return(&github.CombinedStatus{
				State: github.String(stateError), // statePending can run e2e-test (block-pr-merge.go)
			}, nil, nil)
		is.EXPECT().CreateComment(gomock.AssignableToTypeOf(ctxInterface), pr.RepoOwner, pr.RepoName, pr.Number, comment).Times(1).Return(nil, nil, nil)
		err := s.handleE2ETestMobile(ctx, userHandle, pr)
		require.Error(t, err)
		require.IsType(t, &E2ETestMobileError{}, err)
		require.Equal(t, err.(*E2ETestMobileError).source, *msg)
	})
}
