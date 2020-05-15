package server_test

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/google/go-github/v31/github"

	"github.com/mattermost/mattermost-mattermod/server"
	"github.com/mattermost/mattermost-mattermod/server/mocks"
	"github.com/stretchr/testify/assert"
)

func TestIsOrgMember(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orgMocks := mocks.NewMockOrganizationsService(ctrl)
	mockedClient := &server.GithubClient{
		Organizations: orgMocks,
	}

	opts := &github.ListMembersOptions{
		ListOptions: github.ListOptions{},
	}
	users := []*github.User{{
		Login: github.String("test1"),
	}, {
		Login: github.String("test2"),
	}}
	orgMocks.EXPECT().ListMembers(gomock.Any(), "mattertest", opts).Return(users, nil, nil)

	s := &server.Server{
		Config: &server.ServerConfig{
			Org: "mattertest",
		},
		GithubClient: mockedClient,
		OrgMembers:   nil,
	}
	s.RefreshMembers()
	assert.Equal(t, s.IsOrgMember("test3"), false)
	assert.Equal(t, s.IsOrgMember("test1"), true)
}
