package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/xanzy/go-gitlab"

	"github.com/stretchr/testify/assert"

	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-mattermod/server/mocks"

	"github.com/golang/mock/gomock"
	"github.com/google/go-github/v39/github"
	"github.com/stretchr/testify/require"
)

const (
	commandE2ETestBase     = "/e2e-test"
	commandE2ETestAdvanced = "/e2e-test MM_ENV=\"MM_FEATUREFLAGS_GLOBALHEADER=true,MM_OTHER_FLAG=true\" INCLUDE_FILE=\"new_message_spec.js\" EXCLUDE_FILE=\"something_to_exclude_spec.js\"\nOther commenting after command \n Even other comment"
	prNumber               = 123
	eSHA                   = "abcdefg"
	eBranch                = "branchA"
	ghBranchNotFoundError  = "throwing a GitHub error when branch not found"
)

func TestParseE2ETestCommentForOpts(t *testing.T) {
	t.Run("command with newline", func(t *testing.T) {
		commentBody := "/e2e-test\nOther commenting after command \n Even other comment"
		aOpts := parseE2ETestCommentForOpts(commentBody)
		assert.Nil(t, aOpts)

		commentBody = "/e2e-test INCLUDE_FILE=\"new_message_spec.js\" EXCLUDE_FILE=\"something_to_exclude_spec.js\"\nOther commenting after command \n Even other comment"
		aOpts = parseE2ETestCommentForOpts(commentBody)
		eOpts := &map[string]string{
			"INCLUDE_FILE": "new_message_spec.js",
			"EXCLUDE_FILE": "something_to_exclude_spec.js",
		}
		assert.Equal(t, 2, len(*aOpts))
		assert.EqualValues(t, eOpts, aOpts)

		commentBody = commandE2ETestAdvanced
		aOpts = parseE2ETestCommentForOpts(commentBody)
		eOpts = &map[string]string{
			"MM_ENV":       "MM_FEATUREFLAGS_GLOBALHEADER=true,MM_OTHER_FLAG=true",
			"INCLUDE_FILE": "new_message_spec.js",
			"EXCLUDE_FILE": "something_to_exclude_spec.js",
		}
		assert.Equal(t, 3, len(*aOpts))
		assert.EqualValues(t, eOpts, aOpts)
	})
	t.Run("command with space at end", func(t *testing.T) {
		commentBody := "/e2e-test "
		aOpts := parseE2ETestCommentForOpts(commentBody)
		assert.Nil(t, aOpts)

		commentBody = "/e2e-test INCLUDE_FILE=\"new_message_spec.js\" EXCLUDE_FILE=\"something_to_exclude_spec.js\" "
		aOpts = parseE2ETestCommentForOpts(commentBody)
		eOpts := &map[string]string{
			"INCLUDE_FILE": "new_message_spec.js",
			"EXCLUDE_FILE": "something_to_exclude_spec.js",
		}
		assert.Equal(t, 2, len(*aOpts))
		assert.EqualValues(t, eOpts, aOpts)

		commentBody = "/e2e-test MM_ENV=\"MM_FEATUREFLAGS_GLOBALHEADER=true,MM_OTHER_FLAG=true\" INCLUDE_FILE=\"new_message_spec.js\" EXCLUDE_FILE=\"something_to_exclude_spec.js\" "
		aOpts = parseE2ETestCommentForOpts(commentBody)
		eOpts = &map[string]string{
			"MM_ENV":       "MM_FEATUREFLAGS_GLOBALHEADER=true,MM_OTHER_FLAG=true",
			"INCLUDE_FILE": "new_message_spec.js",
			"EXCLUDE_FILE": "something_to_exclude_spec.js",
		}
		assert.Equal(t, 3, len(*aOpts))
		assert.EqualValues(t, eOpts, aOpts)
	})
}

func TestHandleE2ETesting(t *testing.T) {
	t.Run("happy trigger from webapp", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		const userHandle = "user"
		const organization = "mattertest"
		s := Server{
			Config: &Config{
				Org:                    organization,
				E2EGitLabProject:       "qa%2Fcypress",
				E2EWebappReponame:      "mattermost-webapp",
				E2EServerReponame:      "mattermost-server",
				E2EWebappRef:           "e2e-webapp-pr",
				E2EServerRef:           "e2e-server-pr",
				E2EDockerRepo:          "mattermost/mm-ee-test",
				E2EServerStatusContext: "ee/circleci",
				E2EWebappStatusContext: "ci/circleci: build-docker",
				E2ETestDeadline:        5 * time.Second,
				E2EGitHubRateLimitHack: 1 * time.Second,
			},
			GithubClient:     &GithubClient{},
			GitLabCIClientV4: &GitLabClient{},
		}

		ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
		defer cancel()
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

		commentBody := commandE2ETestBase
		s.OrgMembers = make([]string, 1)
		s.OrgMembers[0] = userHandle
		eBuildTag := s.Config.E2EDockerRepo + eSHA[0:7]
		pr := &model.PullRequest{
			RepoOwner: userHandle,
			RepoName:  s.Config.E2EWebappReponame,
			Number:    prNumber,
			Ref:       eBranch,
			Sha:       eSHA,
		}
		pipsC := []*gitlab.PipelineInfo{
			{
				ID:     0,
				Ref:    s.Config.E2EWebappRef,
				Status: string(gitlab.Created),
			},
		}
		pipsP := []*gitlab.PipelineInfo{
			{
				ID:     1,
				Ref:    s.Config.E2EWebappRef,
				Status: string(gitlab.Pending),
			},
		}
		notSameEnvs0 := []*gitlab.PipelineVariable{
			{
				Key:   envKeyPRNumber,
				Value: "124",
			},
			{
				Key:   envKeyBuildTag,
				Value: eBuildTag,
			},
		}
		notSameEnvs1 := []*gitlab.PipelineVariable{
			{
				Key:   envKeyPRNumber,
				Value: strconv.Itoa(prNumber),
			},
			{
				Key:   envKeyBuildTag,
				Value: s.Config.E2EDockerRepo + ":" + "otherSHA"[0:7],
			},
		}
		p := &gitlab.Pipeline{WebURL: "https://your.gitlab.com/project/-/pipelines/54004"}
		pr.FullName = organization + "/" + userHandle
		rs.EXPECT().ListStatuses(gomock.AssignableToTypeOf(ctxInterface), pr.RepoOwner, pr.RepoName, pr.Sha, nil).Times(1).Return([]*github.RepoStatus{
			{
				State:   github.String(stateSuccess),
				Context: &s.Config.E2EWebappStatusContext,
			},
		}, nil, nil)
		rs.EXPECT().GetBranch(gomock.AssignableToTypeOf(ctxInterface), s.Config.Org, s.Config.E2EServerReponame, pr.Ref, false).Times(1).Return(
			nil,
			&github.Response{Response: &http.Response{StatusCode: http.StatusNotFound}},
			errors.New(ghBranchNotFoundError),
		)
		prs.EXPECT().Get(gomock.AssignableToTypeOf(ctxInterface), s.Config.Org, pr.RepoName, pr.Number).Times(1).Return(
			&github.PullRequest{
				Number: &pr.Number,
				Base: &github.PullRequestBranch{
					Ref: github.String("master"),
				},
			},
			&github.Response{Response: &http.Response{StatusCode: http.StatusOK}},
			nil,
		)
		glPS.EXPECT().ListProjectPipelines(s.Config.E2EGitLabProject, gomock.Any(), gomock.AssignableToTypeOf(gCtxInterface)).Times(1).Return(pipsC, nil, nil)
		glPS.EXPECT().ListProjectPipelines(s.Config.E2EGitLabProject, gomock.Any(), gomock.AssignableToTypeOf(gCtxInterface)).Times(1).Return(pipsP, nil, nil)
		glPS.EXPECT().ListProjectPipelines(s.Config.E2EGitLabProject, gomock.Any(), gomock.AssignableToTypeOf(gCtxInterface)).Times(1).Return(nil, nil, nil)
		glPS.EXPECT().GetPipelineVariables(s.Config.E2EGitLabProject, gomock.Any(), gomock.AssignableToTypeOf(gCtxInterface)).Times(1).Return(notSameEnvs0, nil, nil)
		glPS.EXPECT().GetPipelineVariables(s.Config.E2EGitLabProject, gomock.Any(), gomock.AssignableToTypeOf(gCtxInterface)).Times(1).Return(notSameEnvs1, nil, nil)

		glPS.EXPECT().CreatePipeline(s.Config.E2EGitLabProject, gomock.Any(), gomock.AssignableToTypeOf(gCtxInterface)).Times(1).Return(p, nil, nil)
		commentEnd := &github.IssueComment{Body: github.String(fmt.Sprintf(e2eTestFmtSuccess, e2eTestMsgSuccess, p.WebURL))}
		is.EXPECT().CreateComment(gomock.AssignableToTypeOf(ctxInterface), pr.RepoOwner, pr.RepoName, pr.Number, commentEnd).Times(1).Return(nil, nil, nil)
		err := s.handleE2ETest(ctx, userHandle, pr, commentBody)
		require.NoError(t, err)
	})
	t.Run("happy trigger from webapp with options", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		const userHandle = "user"
		const organization = "mattertest"
		s := Server{
			Config: &Config{
				Org:                    organization,
				E2EGitLabProject:       "qa%2Fcypress",
				E2EWebappReponame:      "mattermost-webapp",
				E2EServerReponame:      "mattermost-server",
				E2EWebappRef:           "e2e-webapp-pr",
				E2EServerRef:           "e2e-server-pr",
				E2EDockerRepo:          "mattermost/mm-ee-test",
				E2EServerStatusContext: "ee/circleci",
				E2EWebappStatusContext: "ci/circleci: build-docker",
				E2ETestDeadline:        5 * time.Second,
				E2EGitHubRateLimitHack: 1 * time.Second,
			},
			GithubClient:     &GithubClient{},
			GitLabCIClientV4: &GitLabClient{},
		}

		ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
		defer cancel()
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

		commentBody := commandE2ETestAdvanced
		s.OrgMembers = make([]string, 1)
		s.OrgMembers[0] = userHandle
		eBuildTag := s.Config.E2EDockerRepo + eSHA[0:7]
		pr := &model.PullRequest{
			RepoOwner: userHandle,
			RepoName:  s.Config.E2EWebappReponame,
			Number:    prNumber,
			Ref:       eBranch,
			Sha:       eSHA,
		}
		pipsC := []*gitlab.PipelineInfo{
			{
				ID:     0,
				Ref:    s.Config.E2EWebappRef,
				Status: string(gitlab.Created),
			},
		}
		pipsP := []*gitlab.PipelineInfo{
			{
				ID:     1,
				Ref:    s.Config.E2EWebappRef,
				Status: string(gitlab.Pending),
			},
		}
		notSameEnvs0 := []*gitlab.PipelineVariable{
			{
				Key:   envKeyPRNumber,
				Value: "124",
			},
			{
				Key:   envKeyBuildTag,
				Value: eBuildTag,
			},
		}
		notSameEnvs1 := []*gitlab.PipelineVariable{
			{
				Key:   envKeyPRNumber,
				Value: strconv.Itoa(prNumber),
			},
			{
				Key:   envKeyBuildTag,
				Value: s.Config.E2EDockerRepo + ":" + "otherSHA"[0:7],
			},
		}
		p := &gitlab.Pipeline{WebURL: "https://your.gitlab.com/project/-/pipelines/54004"}
		pr.FullName = organization + "/" + userHandle
		first := rs.EXPECT().ListStatuses(gomock.AssignableToTypeOf(ctxInterface), pr.RepoOwner, pr.RepoName, pr.Sha, nil).Times(1).Return([]*github.RepoStatus{
			{
				State:   github.String(stateError),
				Context: &s.Config.E2EWebappStatusContext,
			},
		}, nil, nil)
		second := rs.EXPECT().ListStatuses(gomock.AssignableToTypeOf(ctxInterface), pr.RepoOwner, pr.RepoName, pr.Sha, nil).Times(1).Return([]*github.RepoStatus{
			{
				State:   github.String(stateSuccess),
				Context: &s.Config.E2EWebappStatusContext,
			},
		}, nil, nil)
		gomock.InOrder(first, second)
		rs.EXPECT().GetBranch(gomock.AssignableToTypeOf(ctxInterface), s.Config.Org, s.Config.E2EServerReponame, pr.Ref, false).Times(1).Return(
			nil,
			&github.Response{Response: &http.Response{StatusCode: http.StatusNotFound}},
			errors.New(ghBranchNotFoundError),
		)
		prs.EXPECT().Get(gomock.AssignableToTypeOf(ctxInterface), s.Config.Org, pr.RepoName, pr.Number).Times(1).Return(
			&github.PullRequest{
				Number: &pr.Number,
				Base: &github.PullRequestBranch{
					Ref: github.String("master"),
				},
			},
			&github.Response{Response: &http.Response{StatusCode: http.StatusOK}},
			nil,
		)
		glPS.EXPECT().ListProjectPipelines(s.Config.E2EGitLabProject, gomock.Any(), gomock.AssignableToTypeOf(gCtxInterface)).Times(1).Return(pipsC, nil, nil)
		glPS.EXPECT().ListProjectPipelines(s.Config.E2EGitLabProject, gomock.Any(), gomock.AssignableToTypeOf(gCtxInterface)).Times(1).Return(pipsP, nil, nil)
		glPS.EXPECT().ListProjectPipelines(s.Config.E2EGitLabProject, gomock.Any(), gomock.AssignableToTypeOf(gCtxInterface)).Times(1).Return(nil, nil, nil)
		glPS.EXPECT().GetPipelineVariables(s.Config.E2EGitLabProject, gomock.Any(), gomock.AssignableToTypeOf(gCtxInterface)).Times(1).Return(notSameEnvs0, nil, nil)
		glPS.EXPECT().GetPipelineVariables(s.Config.E2EGitLabProject, gomock.Any(), gomock.AssignableToTypeOf(gCtxInterface)).Times(1).Return(notSameEnvs1, nil, nil)

		opts := &map[string]string{
			"EXCLUDE_FILE": "something_to_exclude_spec.js",
			"INCLUDE_FILE": "new_message_spec.js",
			"MM_ENV":       "MM_FEATUREFLAGS_GLOBALHEADER=true,MM_OTHER_FLAG=true",
		}
		commentInit := &github.IssueComment{Body: github.String(fmt.Sprintf(e2eTestFmtOpts, e2eTestMsgOpts, opts))}
		is.EXPECT().CreateComment(gomock.AssignableToTypeOf(ctxInterface), pr.RepoOwner, pr.RepoName, pr.Number, commentInit).Times(1).Return(nil, nil, nil)
		glPS.EXPECT().CreatePipeline(s.Config.E2EGitLabProject, gomock.Any(), gomock.AssignableToTypeOf(gCtxInterface)).Times(1).Return(p, nil, nil)
		commentEnd := &github.IssueComment{Body: github.String(fmt.Sprintf(e2eTestFmtSuccess, e2eTestMsgSuccess, p.WebURL))}
		is.EXPECT().CreateComment(gomock.AssignableToTypeOf(ctxInterface), pr.RepoOwner, pr.RepoName, pr.Number, commentEnd).Times(1).Return(nil, nil, nil)
		err := s.handleE2ETest(ctx, userHandle, pr, commentBody)
		require.NoError(t, err)
	})
	t.Run("happy trigger from server", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		const userHandle = "user"
		const organization = "mattertest"
		s := Server{
			Config: &Config{
				Org:                    organization,
				E2EGitLabProject:       "qa%2Fcypress",
				E2EWebappReponame:      "mattermost-webapp",
				E2EServerReponame:      "mattermost-server",
				E2EWebappRef:           "e2e-webapp-pr",
				E2EServerRef:           "e2e-server-pr",
				E2EDockerRepo:          "mattermost/mm-ee-test",
				E2EServerStatusContext: "ee/circleci",
				E2EWebappStatusContext: "ci/circleci: build-docker",
				E2ETestDeadline:        5 * time.Second,
				E2EGitHubRateLimitHack: 1 * time.Second,
			},
			GithubClient:     &GithubClient{},
			GitLabCIClientV4: &GitLabClient{},
		}

		ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
		defer cancel()
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

		commentBody := commandE2ETestBase
		s.OrgMembers = make([]string, 1)
		s.OrgMembers[0] = userHandle
		eBuildTag := s.Config.E2EDockerRepo + eSHA[0:7]
		pr := &model.PullRequest{
			RepoOwner: userHandle,
			RepoName:  s.Config.E2EServerReponame,
			Number:    prNumber,
			Ref:       eBranch,
			Sha:       eSHA,
		}
		pipsC := []*gitlab.PipelineInfo{
			{
				ID:     0,
				Ref:    s.Config.E2EServerRef,
				Status: string(gitlab.Created),
			},
		}
		pipsP := []*gitlab.PipelineInfo{
			{
				ID:     1,
				Ref:    s.Config.E2EServerRef,
				Status: string(gitlab.Pending),
			},
		}
		notSameEnvs0 := []*gitlab.PipelineVariable{
			{
				Key:   envKeyPRNumber,
				Value: "124",
			},
			{
				Key:   envKeyBuildTag,
				Value: eBuildTag,
			},
		}
		notSameEnvs1 := []*gitlab.PipelineVariable{
			{
				Key:   envKeyPRNumber,
				Value: strconv.Itoa(prNumber),
			},
			{
				Key:   envKeyBuildTag,
				Value: s.Config.E2EDockerRepo + ":" + "otherSHA"[0:7],
			},
		}
		p := &gitlab.Pipeline{WebURL: "https://your.gitlab.com/project/-/pipelines/54004"}
		pr.FullName = organization + "/" + userHandle
		rs.EXPECT().
			ListStatuses(gomock.AssignableToTypeOf(ctxInterface), pr.RepoOwner, pr.RepoName, pr.Sha, nil).
			Times(1).
			Return([]*github.RepoStatus{
				{
					State:   github.String(stateSuccess),
					Context: &s.Config.E2EServerStatusContext,
				},
			}, nil, nil)
		rs.EXPECT().
			GetBranch(
				gomock.AssignableToTypeOf(ctxInterface),
				s.Config.Org,
				s.Config.E2EWebappReponame,
				pr.Ref,
				false,
			).
			Times(1).
			Return(
				nil,
				&github.Response{Response: &http.Response{StatusCode: http.StatusNotFound}},
				errors.New(ghBranchNotFoundError),
			)
		prs.EXPECT().
			Get(gomock.AssignableToTypeOf(ctxInterface), s.Config.Org, pr.RepoName, pr.Number).
			Times(1).
			Return(
				&github.PullRequest{
					Number: &pr.Number,
					Base: &github.PullRequestBranch{
						Ref: github.String("master"),
					},
				},
				&github.Response{Response: &http.Response{StatusCode: http.StatusOK}},
				nil,
			)
		glPS.EXPECT().
			ListProjectPipelines(s.Config.E2EGitLabProject, gomock.Any(), gomock.AssignableToTypeOf(gCtxInterface)).
			Times(1).
			Return(pipsC, nil, nil)
		glPS.EXPECT().
			ListProjectPipelines(s.Config.E2EGitLabProject, gomock.Any(), gomock.AssignableToTypeOf(gCtxInterface)).
			Times(1).
			Return(pipsP, nil, nil)
		glPS.EXPECT().
			ListProjectPipelines(s.Config.E2EGitLabProject, gomock.Any(), gomock.AssignableToTypeOf(gCtxInterface)).
			Times(1).
			Return(nil, nil, nil)
		glPS.EXPECT().
			GetPipelineVariables(s.Config.E2EGitLabProject, gomock.Any(), gomock.AssignableToTypeOf(gCtxInterface)).
			Times(1).
			Return(notSameEnvs0, nil, nil)
		glPS.EXPECT().
			GetPipelineVariables(s.Config.E2EGitLabProject, gomock.Any(), gomock.AssignableToTypeOf(gCtxInterface)).
			Times(1).
			Return(notSameEnvs1, nil, nil)

		commentInit := &github.IssueComment{Body: github.String(fmt.Sprintf(e2eTestFmtOpts, e2eTestMsgOpts, nil))}
		is.EXPECT().
			CreateComment(gomock.AssignableToTypeOf(ctxInterface),
				pr.RepoOwner,
				pr.RepoName,
				pr.Number,
				commentInit,
			).
			Times(1).
			Return(nil, nil, nil)
		glPS.EXPECT().
			CreatePipeline(s.Config.E2EGitLabProject, gomock.Any(), gomock.AssignableToTypeOf(gCtxInterface)).
			Times(1).
			Return(p, nil, nil)
		commentEnd := &github.IssueComment{Body: github.String(fmt.Sprintf(e2eTestFmtSuccess, e2eTestMsgSuccess, p.WebURL))}
		is.EXPECT().
			CreateComment(gomock.AssignableToTypeOf(ctxInterface), pr.RepoOwner, pr.RepoName, pr.Number, commentEnd).
			Times(1).
			Return(nil, nil, nil)
		err := s.handleE2ETest(ctx, userHandle, pr, commentBody)
		require.NoError(t, err)
	})
	t.Run("random user", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		const userHandle = "user"
		const organization = "mattertest"
		s := Server{
			Config: &Config{
				Org:                    organization,
				E2EGitLabProject:       "qa%2Fcypress",
				E2EWebappReponame:      "mattermost-webapp",
				E2EServerReponame:      "mattermost-server",
				E2EWebappRef:           "e2e-webapp-pr",
				E2EServerRef:           "e2e-server-pr",
				E2EDockerRepo:          "mattermost/mm-ee-test",
				E2EServerStatusContext: "ee/circleci",
				E2EWebappStatusContext: "ci/circleci: build-docker",
				E2ETestDeadline:        5 * time.Second,
				E2EGitHubRateLimitHack: 1 * time.Second,
			},
			GithubClient:     &GithubClient{},
			GitLabCIClientV4: &GitLabClient{},
		}

		ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
		defer cancel()
		is := mocks.NewMockIssuesService(ctrl)
		rs := mocks.NewMockRepositoriesService(ctrl)
		prs := mocks.NewMockPullRequestsService(ctrl)
		glPS := mocks.NewMockPipelinesService(ctrl)
		s.GithubClient.Issues = is
		s.GithubClient.Repositories = rs
		s.GithubClient.PullRequests = prs
		s.GitLabCIClientV4.Pipelines = glPS

		ctxInterface := reflect.TypeOf((*context.Context)(nil)).Elem()

		s.OrgMembers = make([]string, 1)
		s.OrgMembers[0] = userHandle
		commentBody := commandE2ETestBase
		pr := &model.PullRequest{
			RepoOwner: userHandle,
			RepoName:  s.Config.E2EWebappReponame,
			Number:    prNumber,
			Ref:       eBranch,
			Sha:       eSHA,
		}
		is.EXPECT().
			CreateComment(
				gomock.AssignableToTypeOf(ctxInterface),
				pr.RepoOwner,
				pr.RepoName,
				pr.Number,
				&github.IssueComment{Body: github.String(e2eTestMsgCommenterPermission)},
			).
			Times(1).
			Return(nil, nil, nil)
		err := s.handleE2ETest(ctx, "someone", pr, commentBody)
		require.Error(t, err)
		require.IsType(t, &E2ETestError{}, err)
		require.Equal(t, err.(*E2ETestError).source, e2eTestMsgCommenterPermission)
	})
	t.Run("pr not ready", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		const userHandle = "user"
		const organization = "mattertest"
		s := Server{
			Config: &Config{
				Org:                    organization,
				E2EGitLabProject:       "qa%2Fcypress",
				E2EWebappReponame:      "mattermost-webapp",
				E2EServerReponame:      "mattermost-server",
				E2EWebappRef:           "e2e-webapp-pr",
				E2EServerRef:           "e2e-server-pr",
				E2EDockerRepo:          "mattermost/mm-ee-test",
				E2EServerStatusContext: "ee/circleci",
				E2EWebappStatusContext: "ci/circleci: build-docker",
				E2ETestDeadline:        5 * time.Second,
				E2EGitHubRateLimitHack: 1 * time.Second,
			},
			GithubClient:     &GithubClient{},
			GitLabCIClientV4: &GitLabClient{},
		}

		is := mocks.NewMockIssuesService(ctrl)
		rs := mocks.NewMockRepositoriesService(ctrl)
		prs := mocks.NewMockPullRequestsService(ctrl)
		glPS := mocks.NewMockPipelinesService(ctrl)
		s.GithubClient.Issues = is
		s.GithubClient.Repositories = rs
		s.GithubClient.PullRequests = prs
		s.GitLabCIClientV4.Pipelines = glPS

		ctxInterface := reflect.TypeOf((*context.Context)(nil)).Elem()

		commentBody := commandE2ETestBase
		s.OrgMembers = make([]string, 1)
		s.OrgMembers[0] = userHandle
		pr := &model.PullRequest{
			RepoOwner: userHandle,
			RepoName:  s.Config.E2EWebappReponame,
			Number:    prNumber,
			Ref:       eBranch,
			Sha:       eSHA,
		}
		pr.FullName = organization + "/" + userHandle
		rs.EXPECT().
			ListStatuses(gomock.AssignableToTypeOf(ctxInterface), pr.RepoOwner, pr.RepoName, pr.Sha, nil).
			Times(5).
			Return([]*github.RepoStatus{
				{
					State:   github.String(stateError),
					Context: &s.Config.E2EWebappStatusContext,
				},
			}, nil, nil)
		is.EXPECT().
			CreateComment(
				gomock.AssignableToTypeOf(ctxInterface),
				pr.RepoOwner,
				pr.RepoName,
				pr.Number,
				&github.IssueComment{Body: github.String(e2eTestMsgCIFailing)},
			).
			Times(1).
			Return(nil, nil, nil)
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(s.Config.E2ETestDeadline))
		defer cancel()
		err := s.handleE2ETest(ctx, userHandle, pr, commentBody)
		require.Error(t, err)
		require.IsType(t, &E2ETestError{}, err)
		require.Equal(t, err.(*E2ETestError).source, e2eTestMsgCIFailing)
	})
	t.Run("webapp do not trigger with same envs", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		const userHandle = "user"
		const organization = "mattertest"
		s := Server{
			Config: &Config{
				Org:                    organization,
				E2EGitLabProject:       "qa%2Fcypress",
				E2EWebappReponame:      "mattermost-webapp",
				E2EServerReponame:      "mattermost-server",
				E2EWebappRef:           "e2e-webapp-pr",
				E2EServerRef:           "e2e-server-pr",
				E2EDockerRepo:          "mattermost/mm-ee-test",
				E2EServerStatusContext: "ee/circleci",
				E2EWebappStatusContext: "ci/circleci: build-docker",
				E2ETestDeadline:        5 * time.Second,
				E2EGitHubRateLimitHack: 1 * time.Second,
			},
			GithubClient:     &GithubClient{},
			GitLabCIClientV4: &GitLabClient{},
		}

		ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
		defer cancel()
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
		eEnvKey0 := "MM_ENV"
		eEnvValue0 := "MM_FEATUREFLAGS_GLOBALHEADER=true,MM_OTHER_FLAG=true"
		eEnvKey1 := "INCLUDE_FILE"
		eEnvValue1 := "new_message_spec.js"
		eEnvKey2 := "EXCLUDE_FILE"
		eEnvValue2 := "something_to_exclude_spec.js"
		commentBody := fmt.Sprintf("/e2e-test %s=\"%s\" %s=\"%s\" %s=\"%s\"\nOther commenting after command \n Even other comment", eEnvKey0, eEnvValue0, eEnvKey1, eEnvValue1, eEnvKey2, eEnvValue2)
		s.OrgMembers = make([]string, 1)
		s.OrgMembers[0] = userHandle
		eBuildTag := s.Config.E2EDockerRepo + eSHA[0:7]
		pr := &model.PullRequest{
			RepoOwner: userHandle,
			RepoName:  s.Config.E2EWebappReponame,
			Number:    prNumber,
			Ref:       eBranch,
			Sha:       eSHA,
		}
		pipsC := []*gitlab.PipelineInfo{
			{
				ID:     0,
				Ref:    s.Config.E2EWebappRef,
				Status: string(gitlab.Created),
			},
		}
		pipsP := []*gitlab.PipelineInfo{
			{
				ID:     1,
				Ref:    s.Config.E2EWebappRef,
				Status: string(gitlab.Pending),
			},
			{
				ID:     2,
				Ref:    s.Config.E2EWebappRef,
				Status: string(gitlab.Pending),
			},
			{
				ID:     3,
				Ref:    s.Config.E2EWebappRef,
				Status: string(gitlab.Pending),
			},
		}
		notSameEnvs0 := []*gitlab.PipelineVariable{
			{
				Key:   envKeyPRNumber,
				Value: "124",
			},
			{
				Key:   envKeyBuildTag,
				Value: eBuildTag,
			},
			{
				Key:   eEnvKey0,
				Value: eEnvValue0,
			},
			{
				Key:   eEnvKey1,
				Value: eEnvValue1,
			},
			{
				Key:   eEnvKey2,
				Value: eEnvValue2,
			},
		}
		NotSameEnvs1 := []*gitlab.PipelineVariable{
			{
				Key:   envKeyPRNumber,
				Value: strconv.Itoa(prNumber),
			},
			{
				Key:   envKeyBuildTag,
				Value: s.Config.E2EDockerRepo + ":" + "otherSHA"[0:7],
			},
			{
				Key:   eEnvKey0,
				Value: eEnvValue0,
			},
			{
				Key:   eEnvKey1,
				Value: eEnvValue1,
			},
			{
				Key:   eEnvKey2,
				Value: eEnvValue2,
			},
		}
		NotSameEnvs2 := []*gitlab.PipelineVariable{
			{
				Key:   envKeyPRNumber,
				Value: strconv.Itoa(prNumber),
			},
			{
				Key:   envKeyBuildTag,
				Value: eBuildTag,
			},
			{
				Key:   eEnvKey0 + "0",
				Value: eEnvValue0,
			},
			{
				Key:   eEnvKey1,
				Value: eEnvValue1,
			},
			{
				Key:   eEnvKey2,
				Value: eEnvValue2,
			},
		}
		sameEnvs := []*gitlab.PipelineVariable{
			{
				Key:   envKeyPRNumber,
				Value: strconv.Itoa(prNumber),
			},
			{
				Key:   envKeyBuildTag,
				Value: eBuildTag,
			},
			{
				Key:   eEnvKey0,
				Value: eEnvValue0,
			},
			{
				Key:   eEnvKey1,
				Value: eEnvValue1,
			},
			{
				Key:   eEnvKey2,
				Value: eEnvValue2,
			},
		}
		pr.FullName = organization + "/" + userHandle
		rs.EXPECT().ListStatuses(gomock.AssignableToTypeOf(ctxInterface), pr.RepoOwner, pr.RepoName, pr.Sha, nil).Times(1).Return([]*github.RepoStatus{
			{
				State:   github.String(stateSuccess),
				Context: &s.Config.E2EWebappStatusContext,
			},
		}, nil, nil)
		rs.EXPECT().GetBranch(gomock.AssignableToTypeOf(ctxInterface), s.Config.Org, s.Config.E2EServerReponame, pr.Ref, false).Times(1).Return(
			nil,
			&github.Response{Response: &http.Response{StatusCode: http.StatusNotFound}},
			errors.New(ghBranchNotFoundError),
		)
		prs.EXPECT().Get(gomock.AssignableToTypeOf(ctxInterface), s.Config.Org, pr.RepoName, pr.Number).Times(1).Return(
			&github.PullRequest{
				Number: &pr.Number,
				Base: &github.PullRequestBranch{
					Ref: github.String("master"),
				},
			},
			&github.Response{Response: &http.Response{StatusCode: http.StatusOK}},
			nil,
		)
		glPS.EXPECT().ListProjectPipelines(s.Config.E2EGitLabProject, gomock.Any(), gomock.AssignableToTypeOf(gCtxInterface)).Times(1).Return(pipsC, nil, nil)
		glPS.EXPECT().ListProjectPipelines(s.Config.E2EGitLabProject, gomock.Any(), gomock.AssignableToTypeOf(gCtxInterface)).Times(1).Return(pipsP, nil, nil)
		glPS.EXPECT().ListProjectPipelines(s.Config.E2EGitLabProject, gomock.Any(), gomock.AssignableToTypeOf(gCtxInterface)).Times(1).Return(nil, nil, nil)
		glPS.EXPECT().GetPipelineVariables(s.Config.E2EGitLabProject, gomock.Any(), gomock.AssignableToTypeOf(gCtxInterface)).Times(1).Return(notSameEnvs0, nil, nil)
		glPS.EXPECT().GetPipelineVariables(s.Config.E2EGitLabProject, gomock.Any(), gomock.AssignableToTypeOf(gCtxInterface)).Times(1).Return(NotSameEnvs1, nil, nil)
		glPS.EXPECT().GetPipelineVariables(s.Config.E2EGitLabProject, gomock.Any(), gomock.AssignableToTypeOf(gCtxInterface)).Times(1).Return(NotSameEnvs2, nil, nil)
		glPS.EXPECT().GetPipelineVariables(s.Config.E2EGitLabProject, gomock.Any(), gomock.AssignableToTypeOf(gCtxInterface)).Times(1).Return(sameEnvs, nil, nil)

		is.EXPECT().CreateComment(
			gomock.AssignableToTypeOf(ctxInterface),
			pr.RepoOwner,
			pr.RepoName,
			pr.Number,
			&github.IssueComment{Body: github.String("Triggering e2e testing with options:\n```&map[EXCLUDE_FILE:something_to_exclude_spec.js INCLUDE_FILE:new_message_spec.js MM_ENV:MM_FEATUREFLAGS_GLOBALHEADER=true,MM_OTHER_FLAG=true]```")},
		).Times(1).Return(nil, nil, nil)
		is.EXPECT().CreateComment(
			gomock.AssignableToTypeOf(ctxInterface),
			pr.RepoOwner,
			pr.RepoName,
			pr.Number,
			&github.IssueComment{Body: github.String(e2eTestMsgSameEnvs)},
		).Times(1).Return(nil, nil, nil)
		err := s.handleE2ETest(ctx, userHandle, pr, commentBody)
		require.Error(t, err)
		require.IsType(t, &E2ETestError{}, err)
		require.Equal(t, err.(*E2ETestError).source, e2eTestMsgSameEnvs)
	})
}
