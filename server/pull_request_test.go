package server_test

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/google/go-github/v28/github"

	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-mattermod/server"
	"github.com/mattermost/mattermost-mattermod/server/mocks"
)

func TestCleanUpLabels(t *testing.T) {
	pr := &model.PullRequest{
		RepoOwner: "owner",
		RepoName:  "repoName",
		Number:    123,
	}

	for name, test := range map[string]struct {
		SetupClient func(*gomock.Controller) *server.GithubClient
	}{
		"no label has to be removed": {
			SetupClient: func(ctrl *gomock.Controller) *server.GithubClient {
				issueMocks := mocks.NewMockIssuesService(ctrl)
				client := &server.GithubClient{
					Issues: issueMocks,
				}

				ctx := context.Background()
				labels := []*github.Label{{
					Name: github.String("abc"),
				}, {
					Name: github.String("def"),
				}}

				issueMocks.EXPECT().ListLabelsByIssue(gomock.Eq(ctx), gomock.Eq(pr.RepoOwner), gomock.Eq(pr.RepoName), gomock.Eq(pr.Number), nil).Return(labels, nil, nil)

				return client
			},
		},
		"all labels have to be removed": {
			SetupClient: func(ctrl *gomock.Controller) *server.GithubClient {
				issueMocks := mocks.NewMockIssuesService(ctrl)
				client := &server.GithubClient{
					Issues: issueMocks,
				}

				ctx := context.Background()
				labels := []*github.Label{{
					Name: github.String("AutoMerge"),
				}, {
					Name: github.String("Do Not Merge"),
				}, {
					Name: github.String("Work In Progress"),
				}}

				issueMocks.EXPECT().ListLabelsByIssue(gomock.Eq(ctx), gomock.Eq(pr.RepoOwner), gomock.Eq(pr.RepoName), gomock.Eq(pr.Number), nil).Return(labels, nil, nil)
				issueMocks.EXPECT().RemoveLabelForIssue(gomock.Eq(ctx), gomock.Eq(pr.RepoOwner), gomock.Eq(pr.RepoName), gomock.Eq(pr.Number), gomock.Eq("AutoMerge")).Return(nil, nil)
				issueMocks.EXPECT().RemoveLabelForIssue(gomock.Eq(ctx), gomock.Eq(pr.RepoOwner), gomock.Eq(pr.RepoName), gomock.Eq(pr.Number), gomock.Eq("Do Not Merge")).Return(nil, nil)
				issueMocks.EXPECT().RemoveLabelForIssue(gomock.Eq(ctx), gomock.Eq(pr.RepoOwner), gomock.Eq(pr.RepoName), gomock.Eq(pr.Number), gomock.Eq("Work In Progress")).Return(nil, nil)

				return client
			},
		},
		"some labels have to be removed": {
			SetupClient: func(ctrl *gomock.Controller) *server.GithubClient {
				issueMocks := mocks.NewMockIssuesService(ctrl)
				client := &server.GithubClient{
					Issues: issueMocks,
				}

				ctx := context.Background()
				labels := []*github.Label{{
					Name: github.String("Work In Progress"),
				}, {
					Name: github.String("abc"),
				}, {
					Name: github.String("AutoMerge"),
				}, {
					Name: github.String("def"),
				}}

				issueMocks.EXPECT().ListLabelsByIssue(gomock.Eq(ctx), gomock.Eq(pr.RepoOwner), gomock.Eq(pr.RepoName), gomock.Eq(pr.Number), nil).Return(labels, nil, nil)
				issueMocks.EXPECT().RemoveLabelForIssue(gomock.Eq(ctx), gomock.Eq(pr.RepoOwner), gomock.Eq(pr.RepoName), gomock.Eq(pr.Number), gomock.Eq("AutoMerge")).Return(nil, nil)
				issueMocks.EXPECT().RemoveLabelForIssue(gomock.Eq(ctx), gomock.Eq(pr.RepoOwner), gomock.Eq(pr.RepoName), gomock.Eq(pr.Number), gomock.Eq("Work In Progress")).Return(nil, nil)

				return client
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			defer ctrl.Finish()

			s := &server.Server{
				Config: &server.ServerConfig{
					IssueLabelsToCleanUp: []string{"AutoMerge", "Do Not Merge", "Work In Progress"},
				},
				GithubClient: test.SetupClient(ctrl),
			}
			s.CleanUpLabels(pr)
		})
	}
}
