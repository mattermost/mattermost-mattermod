package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/google/go-github/v32/github"
	"github.com/jarcoal/httpmock"

	"github.com/mattermost/mattermost-mattermod/server"
	"github.com/mattermost/mattermost-mattermod/server/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testGetRefReturn = `{
  "ref": "refs/heads/featureA",
  "node_id": "MDM6UmVmcmVmcy9oZWFkcy9mZWF0dXJlQQ==",
  "url": "https://api.github.com/repos/octocat/Hello-World/git/refs/heads/featureA",
  "object": {
    "type": "commit",
    "sha": "aa218f56b14c9653891f9e74264a383fa43fefbd",
    "url": "https://api.github.com/repos/octocat/Hello-World/git/commits/aa218f56b14c9653891f9e74264a383fa43fefbd"
  }
}`

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
		Config: &server.Config{
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
		Config: &server.Config{
			Org: "mattertest",
		},
		GithubClient: mockedClient,
		OrgMembers:   originalUsers,
	}
	s.RefreshMembers()

	assert.Equal(t, originalUserSize, len(s.OrgMembers))
}

func TestCacheTransport(t *testing.T) {
	t.Run("Should return cached response", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder("GET", "https://api.github.com/repos/ownerTest/repoTest/git/ref/refTest",
			func(req *http.Request) (*http.Response, error) {
				body := &github.Reference{Object: &github.GitObject{}}
				err := json.Unmarshal([]byte(testGetRefReturn), &body)
				require.NoError(t, err)
				resp, err := httpmock.NewJsonResponse(200, body)
				// Needed by httpcache cache the response
				resp.Header.Set("Date", time.Now().Format(time.RFC1123))
				resp.Header.Set("Cache-Control", "max-age=60")
				return resp, err
			},
		)

		// First request should return a non-cached request
		ghClient, _ := server.NewGithubClient("testtoken", 10)
		_, resp, err := ghClient.Git.GetRef(context.TODO(), "ownerTest", "repoTest", "refTest")
		require.NoError(t, err)
		require.Equal(t, "", resp.Header.Get("X-From-Cache"))
		require.Equal(t, 200, resp.StatusCode)

		// This part should answer the cached response because max-age hasn't expired
		_, resp, err = ghClient.Git.GetRef(context.TODO(), "ownerTest", "repoTest", "refTest")
		require.NoError(t, err)
		require.Equal(t, "1", resp.Header.Get("X-From-Cache"))
		require.Equal(t, 200, resp.StatusCode)
	})

	t.Run("Shouldn't return cached response if max-age expires", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder("GET", "https://api.github.com/repos/ownerTest/repoTest/git/ref/refTest",
			func(req *http.Request) (*http.Response, error) {
				body := &github.Reference{Object: &github.GitObject{}}
				err := json.Unmarshal([]byte(testGetRefReturn), &body)
				require.NoError(t, err)
				resp, err := httpmock.NewJsonResponse(200, body)
				// Needed by httpcache cache the response
				resp.Header.Set("Date", time.Now().Format(time.RFC1123))
				resp.Header.Set("Cache-Control", "max-age=0")
				return resp, err
			},
		)

		// First request should return a non-cached request
		ghClient, _ := server.NewGithubClient("testtoken", 10)
		_, resp, err := ghClient.Git.GetRef(context.TODO(), "ownerTest", "repoTest", "refTest")
		require.NoError(t, err)
		require.Equal(t, "", resp.Header.Get("X-From-Cache"))
		require.Equal(t, 200, resp.StatusCode)

		// Here we should return a non-cached request because the max-age value has expired
		_, resp, err = ghClient.Git.GetRef(context.TODO(), "ownerTest", "repoTest", "refTest")
		require.NoError(t, err)
		require.Equal(t, "", resp.Header.Get("X-From-Cache"))
		require.Equal(t, 200, resp.StatusCode)
	})

	t.Run("Should returned cached response if Expires is defined", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder("GET", "https://api.github.com/repos/ownerTest/repoTest/git/ref/refTest",
			func(req *http.Request) (*http.Response, error) {
				body := &github.Reference{Object: &github.GitObject{}}
				err := json.Unmarshal([]byte(testGetRefReturn), &body)
				require.NoError(t, err)
				resp, err := httpmock.NewJsonResponse(200, body)
				expireTime := time.Now().Local().Add(time.Minute * time.Duration(1))
				resp.Header.Set("Date", time.Now().Format(time.RFC1123))
				resp.Header.Set("Expires", expireTime.Format(time.RFC1123))
				return resp, err
			},
		)

		// First request should return a non-cached request
		ghClient, _ := server.NewGithubClient("testtoken", 10)
		_, resp, err := ghClient.Git.GetRef(context.TODO(), "ownerTest", "repoTest", "refTest")
		require.NoError(t, err)
		require.Equal(t, "", resp.Header.Get("X-From-Cache"))
		require.Equal(t, 200, resp.StatusCode)

		// Here we should return a non-cached request because the max-age value has expired
		_, resp, err = ghClient.Git.GetRef(context.TODO(), "ownerTest", "repoTest", "refTest")
		require.NoError(t, err)
		require.Equal(t, "1", resp.Header.Get("X-From-Cache"))
		require.Equal(t, 200, resp.StatusCode)
	})

	t.Run("Shouldn't return cached response if Expires header expired", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder("GET", "https://api.github.com/repos/ownerTest/repoTest/git/ref/refTest",
			func(req *http.Request) (*http.Response, error) {
				body := &github.Reference{Object: &github.GitObject{}}
				err := json.Unmarshal([]byte(testGetRefReturn), &body)
				require.NoError(t, err)
				resp, err := httpmock.NewJsonResponse(200, body)
				expireTime := time.Now().Local().Add(-time.Minute * time.Duration(1))
				resp.Header.Set("Date", time.Now().Format(time.RFC1123))
				resp.Header.Set("Expires", expireTime.Format(time.RFC1123))
				return resp, err
			},
		)

		// First request should return a non-cached request
		ghClient, _ := server.NewGithubClient("testtoken", 10)
		_, resp, err := ghClient.Git.GetRef(context.TODO(), "ownerTest", "repoTest", "refTest")
		require.NoError(t, err)
		require.Equal(t, "", resp.Header.Get("X-From-Cache"))
		require.Equal(t, 200, resp.StatusCode)

		// Here we should return a non-cached request because the max-age value has expired
		_, resp, err = ghClient.Git.GetRef(context.TODO(), "ownerTest", "repoTest", "refTest")
		require.NoError(t, err)
		require.Equal(t, "", resp.Header.Get("X-From-Cache"))
		require.Equal(t, 200, resp.StatusCode)
	})
}
