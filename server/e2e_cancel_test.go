package server

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"testing"

	"github.com/xanzy/go-gitlab"

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
	const prNumber = 123
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
		},
		GithubClient:     &GithubClient{},
		GitLabCIClientV4: &GitLabClient{},
	}

	ctx := context.Background()

	msg := new(string)
	comment := &github.IssueComment{Body: msg}
	is := mocks.NewMockIssuesService(ctrl)
	rs := mocks.NewMockRepositoriesService(ctrl)
	glPS := mocks.NewMockPipelinesService(ctrl)
	s.GithubClient.Issues = is
	s.GithubClient.Repositories = rs
	s.GitLabCIClientV4.Pipelines = glPS

	gCtxInterface := reflect.TypeOf(gitlab.RequestOptionFunc(nil))
	ctxInterface := reflect.TypeOf((*context.Context)(nil)).Elem()
	listPipsOptsInterface := reflect.TypeOf((*gitlab.ListProjectPipelinesOptions)(nil))

	t.Run("random user", func(t *testing.T) {
		*msg = e2eCancelMsgCommenterPermission
		pr := &model.PullRequest{
			RepoOwner: userHandle,
			RepoName:  s.Config.E2EWebappReponame,
			Number:    123,
		}
		is.EXPECT().CreateComment(gomock.AssignableToTypeOf(ctxInterface), pr.RepoOwner, pr.RepoName, pr.Number, comment).Times(1).Return(nil, nil, nil)
		err := s.handleE2ECancel(ctx, "someone", pr)
		require.Error(t, err)
		require.IsType(t, &e2eCancelError{}, err)
		require.Equal(t, err.(*e2eCancelError).source, *msg)
	})
	t.Run("nothing to cancel", func(t *testing.T) {
		s.OrgMembers = make([]string, 1)
		s.OrgMembers[0] = userHandle
		pr := &model.PullRequest{
			RepoOwner: userHandle,
			RepoName:  s.Config.E2EWebappReponame,
			Number:    prNumber,
		}
		pipsC := []*gitlab.PipelineInfo{
			{
				ID:     0,
				WebURL: "https://your.gitlab.url/project/-/pipelines/53140",
				Ref:    s.Config.E2EWebappRef,
				Status: string(gitlab.Created),
			},
		}
		pipsP := []*gitlab.PipelineInfo{
			{
				ID:     1,
				WebURL: "https://your.gitlab.url/project/-/pipelines/53140",
				Ref:    s.Config.E2EWebappRef,
				Status: string(gitlab.Pending),
			},
		}
		pipsR := []*gitlab.PipelineInfo{
			{
				ID:     2,
				WebURL: "https://your.gitlab.url/project/-/pipelines/53140",
				Ref:    s.Config.E2EServerRef,
				Status: string(gitlab.Running),
			},
		}
		pipEnvs0 := []*gitlab.PipelineVariable{
			{
				Key:   envKeyPRNumber,
				Value: "120",
			},
		}
		pipEnvs1 := []*gitlab.PipelineVariable{
			{
				Key:   envKeyPRNumber,
				Value: "124",
			},
		}
		pipEnvs2 := []*gitlab.PipelineVariable{
			{
				Key:   envKeyPRNumber,
				Value: "125",
			},
		}

		*msg = e2eCancelMsgNothingToCancel
		r := &gitlab.Response{
			Response: &http.Response{
				StatusCode: http.StatusOK,
			},
		}
		is.EXPECT().CreateComment(gomock.AssignableToTypeOf(ctxInterface), pr.RepoOwner, pr.RepoName, pr.Number, comment).Times(1).Return(nil, nil, nil)
		glPS.EXPECT().ListProjectPipelines(gomock.Any(), gomock.AssignableToTypeOf(listPipsOptsInterface), gomock.AssignableToTypeOf(gCtxInterface)).Times(1).Return(pipsC, r, nil)
		glPS.EXPECT().ListProjectPipelines(gomock.Any(), gomock.AssignableToTypeOf(listPipsOptsInterface), gomock.AssignableToTypeOf(gCtxInterface)).Times(1).Return(pipsP, r, nil)
		glPS.EXPECT().ListProjectPipelines(gomock.Any(), gomock.AssignableToTypeOf(listPipsOptsInterface), gomock.AssignableToTypeOf(gCtxInterface)).Times(1).Return(pipsR, r, nil)
		glPS.EXPECT().GetPipelineVariables(gomock.Any(), pipsC[0].ID, gomock.AssignableToTypeOf(gCtxInterface)).Times(1).Return(pipEnvs0, r, nil)
		glPS.EXPECT().GetPipelineVariables(gomock.Any(), pipsP[0].ID, gomock.AssignableToTypeOf(gCtxInterface)).Times(1).Return(pipEnvs1, r, nil)
		glPS.EXPECT().GetPipelineVariables(gomock.Any(), pipsR[0].ID, gomock.AssignableToTypeOf(gCtxInterface)).Times(1).Return(pipEnvs2, r, nil)
		err := s.handleE2ECancel(ctx, userHandle, pr)
		require.Error(t, err)
		require.IsType(t, &e2eCancelError{}, err)
		require.Equal(t, err.(*e2eCancelError).source, *msg)
	})
	t.Run("pipeline canceled", func(t *testing.T) {
		s.OrgMembers = make([]string, 1)
		s.OrgMembers[0] = userHandle
		pr := &model.PullRequest{
			RepoOwner: userHandle,
			RepoName:  s.Config.E2EWebappReponame,
			Number:    prNumber,
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
		}
		pipsR := []*gitlab.PipelineInfo{
			{
				ID:     3,
				Ref:    s.Config.E2EServerRef,
				Status: string(gitlab.Running),
			},
			{
				ID:     4,
				Ref:    s.Config.E2EServerRef,
				Status: string(gitlab.Running),
			},
		}

		envs0 := []*gitlab.PipelineVariable{
			{
				Key:   envKeyPRNumber,
				Value: strconv.Itoa(prNumber),
			},
		}
		envs1 := []*gitlab.PipelineVariable{
			{
				Key:   envKeyPRNumber,
				Value: strconv.Itoa(prNumber),
			},
		}
		envs2 := []*gitlab.PipelineVariable{
			{
				Key:   envKeyPRNumber,
				Value: "126",
			},
		}
		envs3 := []*gitlab.PipelineVariable{
			{
				Key:   envKeyPRNumber,
				Value: strconv.Itoa(prNumber),
			},
		}
		envs4 := []*gitlab.PipelineVariable{
			{
				Key:   envKeyPRNumber,
				Value: strconv.Itoa(prNumber),
			},
		}
		var expectedCedPips []*gitlab.PipelineInfo
		pipsC[0].WebURL = "https://your.gitlab.com/project/-/pipelines/" + strconv.Itoa(pipsC[0].ID)
		pipsP[0].WebURL = "https://your.gitlab.com/project/-/pipelines/" + strconv.Itoa(pipsP[0].ID)
		pipsR[0].WebURL = "https://your.gitlab.com/project/-/pipelines/" + strconv.Itoa(pipsR[0].ID)
		pipsR[1].WebURL = "https://your.gitlab.com/project/-/pipelines/" + strconv.Itoa(pipsR[1].ID)
		expectedCedPips = append(expectedCedPips, pipsC[0])
		expectedCedPips = append(expectedCedPips, pipsP[0])
		expectedCedPips = append(expectedCedPips, pipsR[0])
		expectedCedPips = append(expectedCedPips, pipsR[1])
		r := &gitlab.Response{
			Response: &http.Response{
				StatusCode: http.StatusOK,
			},
		}
		var fURLs string
		for _, pip := range expectedCedPips {
			fURLs += pip.WebURL + "\n"
		}
		*msg = fmt.Sprintf("Successfully canceled pipelines:\n%v", fURLs)
		is.EXPECT().CreateComment(gomock.AssignableToTypeOf(ctxInterface), pr.RepoOwner, pr.RepoName, pr.Number, comment).Times(1).Return(nil, nil, nil)
		glPS.EXPECT().ListProjectPipelines(gomock.Any(), gomock.AssignableToTypeOf(listPipsOptsInterface), gomock.AssignableToTypeOf(gCtxInterface)).Times(1).Return(pipsC, r, nil)
		glPS.EXPECT().ListProjectPipelines(gomock.Any(), gomock.AssignableToTypeOf(listPipsOptsInterface), gomock.AssignableToTypeOf(gCtxInterface)).Times(1).Return(pipsP, r, nil)
		glPS.EXPECT().ListProjectPipelines(gomock.Any(), gomock.AssignableToTypeOf(listPipsOptsInterface), gomock.AssignableToTypeOf(gCtxInterface)).Times(1).Return(pipsR, r, nil)
		glPS.EXPECT().GetPipelineVariables(gomock.Any(), pipsC[0].ID, gomock.AssignableToTypeOf(gCtxInterface)).Times(1).Return(envs0, r, nil)
		glPS.EXPECT().GetPipelineVariables(gomock.Any(), pipsP[0].ID, gomock.AssignableToTypeOf(gCtxInterface)).Times(1).Return(envs1, r, nil)
		glPS.EXPECT().GetPipelineVariables(gomock.Any(), pipsP[1].ID, gomock.AssignableToTypeOf(gCtxInterface)).Times(1).Return(envs2, r, nil)
		glPS.EXPECT().GetPipelineVariables(gomock.Any(), pipsR[0].ID, gomock.AssignableToTypeOf(gCtxInterface)).Times(1).Return(envs3, r, nil)
		glPS.EXPECT().GetPipelineVariables(gomock.Any(), pipsR[1].ID, gomock.AssignableToTypeOf(gCtxInterface)).Times(1).Return(envs4, r, nil)
		glPS.EXPECT().CancelPipelineBuild(gomock.Any(), pipsC[0].ID, gomock.AssignableToTypeOf(gCtxInterface)).Times(1).Return(nil, r, nil)
		glPS.EXPECT().CancelPipelineBuild(gomock.Any(), pipsP[0].ID, gomock.AssignableToTypeOf(gCtxInterface)).Times(1).Return(nil, r, nil)
		glPS.EXPECT().CancelPipelineBuild(gomock.Any(), pipsR[0].ID, gomock.AssignableToTypeOf(gCtxInterface)).Times(1).Return(nil, r, nil)
		glPS.EXPECT().CancelPipelineBuild(gomock.Any(), pipsR[1].ID, gomock.AssignableToTypeOf(gCtxInterface)).Times(1).Return(nil, r, nil)
		err := s.handleE2ECancel(ctx, userHandle, pr)
		require.NoError(t, err)
	})
}
