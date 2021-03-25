package server

import (
	"context"
	"reflect"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/google/go-github/v32/github"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-mattermod/server/mocks"
	"github.com/stretchr/testify/assert"
)

func TestPostPRWelcomeMessage(t *testing.T) {
	pr := &model.PullRequest{
		RepoOwner: "owner",
		RepoName:  "repoName",
		Number:    123,
		Username:  "foo",
	}

	ctxInterface := reflect.TypeOf((*context.Context)(nil)).Elem()

	for name, test := range map[string]struct {
		SetupClient      func(*gomock.Controller) *GithubClient
		OrgMembers       []string
		claCommentNeeded bool
	}{
		"No org member": {
			SetupClient: func(ctrl *gomock.Controller) *GithubClient {
				issueMocks := mocks.NewMockIssuesService(ctrl)
				client := &GithubClient{
					Issues: issueMocks,
				}

				comment := &github.IssueComment{Body: github.String("Hi @foo, thanks for the PR!")}

				issueMocks.EXPECT().CreateComment(gomock.AssignableToTypeOf(ctxInterface),
					gomock.Eq(pr.RepoOwner), gomock.Eq(pr.RepoName),
					gomock.Eq(pr.Number), comment).Return(nil, nil, nil)

				return client
			},
			OrgMembers:       []string{"bar"},
			claCommentNeeded: false,
		},
		"No org member, CLA not signed": {
			SetupClient: func(ctrl *gomock.Controller) *GithubClient {
				issueMocks := mocks.NewMockIssuesService(ctrl)
				client := &GithubClient{
					Issues: issueMocks,
				}

				comment := &github.IssueComment{Body: github.String("Hi @foo, thanks for the PR!\n\nYou need to sign the CLA.")}

				issueMocks.EXPECT().CreateComment(gomock.AssignableToTypeOf(ctxInterface),
					gomock.Eq(pr.RepoOwner), gomock.Eq(pr.RepoName),
					gomock.Eq(pr.Number), comment).Return(nil, nil, nil)

				return client
			},
			OrgMembers:       []string{"bar"},
			claCommentNeeded: true,
		},
		"Org member": {
			SetupClient: func(ctrl *gomock.Controller) *GithubClient {
				issueMocks := mocks.NewMockIssuesService(ctrl)
				client := &GithubClient{
					Issues: issueMocks,
				}

				return client
			},
			OrgMembers:       []string{"foo", "bar"},
			claCommentNeeded: false,
		},
	} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			s := &Server{
				Config: &Config{
					PRWelcomeMessage: "Hi {{.Username}}, thanks for the PR!{{if .CLACommentNeeded}}\n\nYou need to sign the CLA.{{end}}",
				},
				OrgMembers:   test.OrgMembers,
				GithubClient: test.SetupClient(ctrl),
			}

			err := s.postPRWelcomeMessage(context.Background(), pr, test.claCommentNeeded)
			assert.NoError(t, err)
		})
	}
}

func TestAssignCommunityLabels(t *testing.T) {
	pr := &model.PullRequest{
		RepoOwner: "owner",
		RepoName:  "repoName",
		Number:    123,
		Username:  "foo",
	}

	t.Run("Org Member", func(t *testing.T) {
		repo := &Repository{
			Owner:          "owner",
			Name:           "repoName",
			GreetingLabels: []string{"hey", "hello"},
		}

		s := &Server{
			Config: &Config{
				Repositories: []*Repository{repo},
			},
			OrgMembers: []string{"foo"},
		}
		assert.NoError(t, s.assignCommunityLabels(context.Background(), pr, repo))
	})

	t.Run("No Labels", func(t *testing.T) {
		repo := &Repository{
			Owner:          "owner",
			Name:           "repoName",
			GreetingLabels: []string{},
		}

		s := &Server{
			Config: &Config{
				Repositories: []*Repository{repo},
			},
			OrgMembers: []string{"foo"},
		}
		assert.NoError(t, s.assignCommunityLabels(context.Background(), pr, repo))
	})

	t.Run("Happy patth", func(t *testing.T) {
		repo := &Repository{
			Owner:          "owner",
			Name:           "repoName",
			GreetingLabels: []string{"hello", "hi"},
		}

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		ctxInterface := reflect.TypeOf((*context.Context)(nil)).Elem()

		issueMocks := mocks.NewMockIssuesService(ctrl)

		client := &GithubClient{
			Issues: issueMocks,
		}

		s := &Server{
			Config: &Config{
				Repositories: []*Repository{repo},
			},
			GithubClient: client,
			OrgMembers:   []string{"foo"},
		}
		assert.NoError(t, s.assignCommunityLabels(context.Background(), pr, repo))

		issueMocks.EXPECT().AddLabelsToIssue(gomock.AssignableToTypeOf(ctxInterface),
			gomock.Eq(pr.RepoOwner), gomock.Eq(pr.RepoName),
			gomock.Eq(pr.Number), repo.GreetingLabels).Return(nil, nil, nil)
	})
}
