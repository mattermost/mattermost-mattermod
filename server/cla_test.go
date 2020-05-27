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

const (
	username    = "mattermod"
	bodySuccess = "Please help complete the Mattermost"
	bodyFail    = "Fail"
)

func TestCheckCLACommentExists(t *testing.T) {
	a := &github.IssueComment{
		ID:   NewInt64(1),
		Body: github.String(bodyFail),
		User: &github.User{
			Login: github.String(username),
		},
	}
	b := &github.IssueComment{
		ID:   NewInt64(23),
		Body: github.String(bodySuccess),
		User: &github.User{
			Login: github.String(username),
		},
	}
	comments := []*github.IssueComment{a, b}

	id, exists := findNeedsToSignCLAComment(comments, username)
	assert.True(t, exists)
	assert.Equal(t, id, *NewInt64(23))
}

func TestCheckCLACommentDoesNotExist(t *testing.T) {
	a := &github.IssueComment{
		ID:   NewInt64(1),
		Body: github.String(bodyFail),
		User: &github.User{
			Login: github.String(username),
		},
	}
	b := &github.IssueComment{
		ID:   NewInt64(23),
		Body: github.String(bodyFail),
		User: &github.User{
			Login: github.String(username),
		},
	}
	comments := []*github.IssueComment{a, b}

	id, exists := findNeedsToSignCLAComment(comments, username)
	assert.False(t, exists)
	assert.Equal(t, id, *NewInt64(0))
}

func TestIsAlreadySigned(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orgMocks := mocks.NewMockOrganizationsService(ctrl)
	mockedClient := &server.GithubClient{
	}

	orgMocks.EXPECT().isAlreadySigned(gomock.Any(), gomock.Any(), opts).Return(dummyUsers, ghR, nil)

	s := &server.Server{
		Config: &server.Config{
			Org: "mattertest",
		},
		GithubClient: mockedClient,
		OrgMembers:   nil,
	}
	s.isAlreadySigned()

	assert.Equal(t, expectedUserSize, len(s.OrgMembers))
	assert.Equal(t, false, s.IsOrgMember("test123"))
	assert.Equal(t, true, s.IsOrgMember("test1"))
}
