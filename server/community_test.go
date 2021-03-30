package server

import (
	"context"
	"reflect"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/google/go-github/v33/github"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-mattermod/server/mocks"
	"github.com/stretchr/testify/assert"
)

const BAR = "bar"

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
			OrgMembers:       []string{BAR},
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
			OrgMembers:       []string{BAR},
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
			OrgMembers:       []string{"foo", BAR},
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

func TestAssignGreetingLabels(t *testing.T) {
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
		assert.NoError(t, s.assignGreetingLabels(context.Background(), pr, repo))
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
			OrgMembers: []string{BAR},
		}
		assert.NoError(t, s.assignGreetingLabels(context.Background(), pr, repo))
	})

	t.Run("Happy path", func(t *testing.T) {
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
			OrgMembers:   []string{BAR},
		}

		issueMocks.EXPECT().AddLabelsToIssue(
			gomock.AssignableToTypeOf(ctxInterface),
			gomock.Eq(pr.RepoOwner), gomock.Eq(pr.RepoName),
			gomock.Eq(pr.Number),
			gomock.Eq(repo.GreetingLabels),
		).Return(nil, nil, nil)

		assert.NoError(t, s.assignGreetingLabels(context.Background(), pr, repo))
	})
}

func TestAssignGreeter(t *testing.T) {
	pr := &model.PullRequest{
		RepoOwner: "owner",
		RepoName:  "repoName",
		Number:    123,
		Username:  "foo",
	}

	t.Run("Org Member", func(t *testing.T) {
		repo := &Repository{
			Owner:        "owner",
			Name:         "repoName",
			GreetingTeam: "greetingTeam",
		}

		s := &Server{
			Config: &Config{
				Repositories: []*Repository{repo},
			},
			OrgMembers: []string{"foo"},
		}
		assert.NoError(t, s.assignGreeter(context.Background(), pr, repo))
	})

	t.Run("No greeting team", func(t *testing.T) {
		repo := &Repository{
			Owner: "owner",
			Name:  "repoName",
		}

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		teamMocks := mocks.NewMockTeamsService(ctrl)

		ctrl.RecordCall(teamMocks, "ListTeamMembersBySlug").Times(0)

		client := &GithubClient{
			Teams: teamMocks,
		}

		s := &Server{
			Config: &Config{
				Repositories: []*Repository{repo},
			},
			GithubClient: client,
			OrgMembers:   []string{BAR},
		}

		assert.NoError(t, s.assignGreeter(context.Background(), pr, repo))
	})

	t.Run("Empty team", func(t *testing.T) {
		repo := &Repository{
			Owner:        "owner",
			Name:         "repoName",
			GreetingTeam: "greetingTeam",
		}
		userLogin := BAR
		members := []*github.User{}

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		ctxInterface := reflect.TypeOf((*context.Context)(nil)).Elem()

		teamMocks := mocks.NewMockTeamsService(ctrl)

		client := &GithubClient{
			Teams: teamMocks,
		}

		s := &Server{
			Config: &Config{
				Repositories: []*Repository{repo},
				Org:          "SomeOrg",
			},
			GithubClient: client,
			OrgMembers:   []string{userLogin},
		}

		teamMocks.EXPECT().ListTeamMembersBySlug(
			gomock.AssignableToTypeOf(ctxInterface),
			gomock.Eq(s.Config.Org),
			gomock.Eq(repo.GreetingTeam),
			gomock.Nil(),
		).Return(members, nil, nil)

		err := s.assignGreeter(context.Background(), pr, repo)
		assert.Error(t, err)
	})

	t.Run("Happy path", func(t *testing.T) {
		repo := &Repository{
			Owner:        "owner",
			Name:         "repoName",
			GreetingTeam: "greetingTeam",
		}
		userLogin := BAR
		greeter := &github.User{Login: &userLogin}
		members := []*github.User{greeter}
		greetingRequest := github.ReviewersRequest{
			Reviewers: []string{*greeter.Login},
		}

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		ctxInterface := reflect.TypeOf((*context.Context)(nil)).Elem()

		teamMocks := mocks.NewMockTeamsService(ctrl)
		pullMocks := mocks.NewMockPullRequestsService(ctrl)

		client := &GithubClient{
			Teams:        teamMocks,
			PullRequests: pullMocks,
		}

		s := &Server{
			Config: &Config{
				Repositories: []*Repository{repo},
				Org:          "SomeOrg",
			},
			GithubClient: client,
			OrgMembers:   []string{userLogin},
		}

		teamMocks.EXPECT().ListTeamMembersBySlug(
			gomock.AssignableToTypeOf(ctxInterface),
			gomock.Eq(s.Config.Org),
			gomock.Eq(repo.GreetingTeam),
			gomock.Nil(),
		).Return(members, nil, nil)

		pullMocks.EXPECT().RequestReviewers(
			gomock.AssignableToTypeOf(ctxInterface),
			gomock.Eq(pr.RepoOwner),
			gomock.Eq(pr.RepoName),
			gomock.Eq(pr.Number),
			gomock.Eq(greetingRequest),
		).Return(nil, nil, nil)

		err := s.assignGreeter(context.Background(), pr, repo)

		assert.NoError(t, err)
	})
}
