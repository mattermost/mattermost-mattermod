package server_test

import (
	"net/http"
	"strconv"
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
	expectedUserSize := 66
	dummyUsers := make([]*github.User, expectedUserSize)
	var user *github.User
	for i := 0; i < expectedUserSize; i++ {
		user = &github.User{Login: github.String("test" + strconv.Itoa(i))}
		dummyUsers[i] = user
	}
	r := &http.Response{StatusCode: http.StatusOK}
	ghR := &github.Response{
		Response: r,
		NextPage: 0,
	}
	orgMocks.EXPECT().ListMembers(gomock.Any(), "mattertest", opts).Return(dummyUsers, ghR, nil)

	s := &server.Server{
		Config: &server.ServerConfig{
			Org: "mattertest",
		},
		GithubClient: mockedClient,
		OrgMembers:   nil,
	}
	s.RefreshMembers()

	assert.Equal(t, expectedUserSize, len(s.OrgMembers))
	assert.Equal(t, false, s.IsOrgMember("test123"))
	assert.Equal(t, true, s.IsOrgMember("test1"))
}

func TestCannotGetAllOrgMembersDueToRateLimit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orgMocks := mocks.NewMockOrganizationsService(ctrl)
	mockedClient := &server.GithubClient{
		Organizations: orgMocks,
	}

	opts := &github.ListMembersOptions{
		ListOptions: github.ListOptions{},
	}

	originalUserSize := 66
	originalUsers := make([]string, originalUserSize)
	for i := 0; i < originalUserSize; i++ {
		originalUsers[i] = "test" + strconv.Itoa(i)
	}

	rateLimitedUserSize := 33
	newUsers := make([]*github.User, rateLimitedUserSize)
	var newUser *github.User
	for i := 0; i < rateLimitedUserSize; i++ {
		newUser = &github.User{Login: github.String("test" + strconv.Itoa(i))}
		newUsers[i] = newUser
	}

	r := &http.Response{StatusCode: http.StatusForbidden}
	ghR := &github.Response{
		Response: r,
		NextPage: 0,
	}
	orgMocks.EXPECT().ListMembers(gomock.Any(), "mattertest", opts).Return(newUsers, ghR, nil)

	s := &server.Server{
		Config: &server.ServerConfig{
			Org: "mattertest",
		},
		GithubClient: mockedClient,
		OrgMembers:   originalUsers,
	}
	s.RefreshMembers()

	assert.Equal(t, originalUserSize, len(s.OrgMembers))
}
