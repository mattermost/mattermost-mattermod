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
	"golang.org/x/time/rate"

	"github.com/mattermost/mattermost-mattermod/server"
	"github.com/mattermost/mattermost-mattermod/server/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testGetRefReturn = []byte(`{
  "ref": "refs/heads/featureA",
  "node_id": "MDM6UmVmcmVmcy9oZWFkcy9mZWF0dXJlQQ==",
  "url": "https://api.github.com/repos/octocat/Hello-World/git/refs/heads/featureA",
  "object": {
    "type": "commit",
    "sha": "aa218f56b14c9653891f9e74264a383fa43fefbd",
    "url": "https://api.github.com/repos/octocat/Hello-World/git/commits/aa218f56b14c9653891f9e74264a383fa43fefbd"
  }
}`)

func TestIsOrgMember(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orgMocks := mocks.NewMockOrganizationsService(ctrl)
	metricsMock := mocks.NewMockMetricsProvider(ctrl)
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
	metricsMock.EXPECT().ObserveCronTaskDuration(gomock.Any(), gomock.Any()).AnyTimes()
	metricsMock.EXPECT().IncreaseCronTaskErrors(gomock.Any()).AnyTimes()

	s := &server.Server{
		Config: &server.Config{
			Org: "mattertest",
		},
		GithubClient: mockedClient,
		OrgMembers:   nil,
		Metrics:      metricsMock,
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
	metricsMock := mocks.NewMockMetricsProvider(ctrl)
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
	metricsMock.EXPECT().ObserveCronTaskDuration(gomock.Any(), gomock.Any()).AnyTimes()
	metricsMock.EXPECT().IncreaseCronTaskErrors(gomock.Any()).AnyTimes()

	s := &server.Server{
		Config: &server.Config{
			Org: "mattertest",
		},
		GithubClient: mockedClient,
		OrgMembers:   originalUsers,
		Metrics:      metricsMock,
	}
	s.RefreshMembers()

	assert.Equal(t, originalUserSize, len(s.OrgMembers))
}

func TestCacheTransport(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	metricsMock := mocks.NewMockMetricsProvider(ctrl)

	t.Run("Should return cached response", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder("GET", "https://api.github.com/repos/ownerTest/repoTest/git/ref/refTest",
			func(req *http.Request) (*http.Response, error) {
				body := &github.Reference{Object: &github.GitObject{}}
				err := json.Unmarshal(testGetRefReturn, &body)
				require.NoError(t, err)
				resp, err := httpmock.NewJsonResponse(http.StatusOK, body)
				// Needed by httpcache cache the response
				resp.Header.Set("Date", time.Now().Format(time.RFC1123))
				resp.Header.Set("Cache-Control", "max-age=60")
				return resp, err
			},
		)

		metricsMock.EXPECT().ObserveGithubRequestDuration(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
		metricsMock.EXPECT().IncreaseGithubCacheMisses(gomock.Any(), gomock.Any()).AnyTimes()
		metricsMock.EXPECT().IncreaseGithubCacheHits(gomock.Any(), gomock.Any()).AnyTimes()

		// First request should return a non-cached request
		ghClient, _ := server.NewGithubClient("testtoken", 10, metricsMock)
		_, resp, err := ghClient.Git.GetRef(context.TODO(), "ownerTest", "repoTest", "refTest")
		require.NoError(t, err)
		require.Equal(t, "", resp.Header.Get("X-From-Cache"))
		require.Equal(t, http.StatusOK, resp.StatusCode)

		// This part should answer the cached response because max-age hasn't expired
		_, resp, err = ghClient.Git.GetRef(context.TODO(), "ownerTest", "repoTest", "refTest")
		require.NoError(t, err)
		require.Equal(t, "1", resp.Header.Get("X-From-Cache"))
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("Shouldn't return cached response if max-age expires", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder("GET", "https://api.github.com/repos/ownerTest/repoTest/git/ref/refTest",
			func(req *http.Request) (*http.Response, error) {
				body := &github.Reference{Object: &github.GitObject{}}
				err := json.Unmarshal(testGetRefReturn, &body)
				require.NoError(t, err)
				resp, err := httpmock.NewJsonResponse(http.StatusOK, body)
				// Needed by httpcache cache the response
				resp.Header.Set("Date", time.Now().Format(time.RFC1123))
				resp.Header.Set("Cache-Control", "max-age=0")
				return resp, err
			},
		)

		// First request should return a non-cached request
		ghClient, _ := server.NewGithubClient("testtoken", 10, metricsMock)
		_, resp, err := ghClient.Git.GetRef(context.TODO(), "ownerTest", "repoTest", "refTest")
		require.NoError(t, err)
		require.Equal(t, "", resp.Header.Get("X-From-Cache"))
		require.Equal(t, http.StatusOK, resp.StatusCode)

		// Here we should return a non-cached request because the max-age value has expired
		_, resp, err = ghClient.Git.GetRef(context.TODO(), "ownerTest", "repoTest", "refTest")
		require.NoError(t, err)
		require.Equal(t, "", resp.Header.Get("X-From-Cache"))
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("Should returned cached response if Expires is defined", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder("GET", "https://api.github.com/repos/ownerTest/repoTest/git/ref/refTest",
			func(req *http.Request) (*http.Response, error) {
				body := &github.Reference{Object: &github.GitObject{}}
				err := json.Unmarshal(testGetRefReturn, &body)
				require.NoError(t, err)
				resp, err := httpmock.NewJsonResponse(http.StatusOK, body)
				expireTime := time.Now().Local().Add(time.Minute * time.Duration(1))
				resp.Header.Set("Date", time.Now().Format(time.RFC1123))
				resp.Header.Set("Expires", expireTime.Format(time.RFC1123))
				return resp, err
			},
		)

		// First request should return a non-cached request
		ghClient, _ := server.NewGithubClient("testtoken", 10, metricsMock)
		_, resp, err := ghClient.Git.GetRef(context.TODO(), "ownerTest", "repoTest", "refTest")
		require.NoError(t, err)
		require.Equal(t, "", resp.Header.Get("X-From-Cache"))
		require.Equal(t, http.StatusOK, resp.StatusCode)

		// Here we should return a cached request
		_, resp, err = ghClient.Git.GetRef(context.TODO(), "ownerTest", "repoTest", "refTest")
		require.NoError(t, err)
		require.Equal(t, "1", resp.Header.Get("X-From-Cache"))
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("Shouldn't return cached response if Expires header expired", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder("GET", "https://api.github.com/repos/ownerTest/repoTest/git/ref/refTest",
			func(req *http.Request) (*http.Response, error) {
				body := &github.Reference{Object: &github.GitObject{}}
				err := json.Unmarshal(testGetRefReturn, &body)
				require.NoError(t, err)
				resp, err := httpmock.NewJsonResponse(http.StatusOK, body)
				expireTime := time.Now().Local().Add(-time.Minute * time.Duration(1))
				resp.Header.Set("Date", time.Now().Format(time.RFC1123))
				resp.Header.Set("Expires", expireTime.Format(time.RFC1123))
				return resp, err
			},
		)

		// First request should return a non-cached request
		ghClient, _ := server.NewGithubClient("testtoken", 10, metricsMock)
		_, resp, err := ghClient.Git.GetRef(context.TODO(), "ownerTest", "repoTest", "refTest")
		require.NoError(t, err)
		require.Equal(t, "", resp.Header.Get("X-From-Cache"))
		require.Equal(t, http.StatusOK, resp.StatusCode)

		// Here we should return a non-cached request because the max-age value has expired
		_, resp, err = ghClient.Git.GetRef(context.TODO(), "ownerTest", "repoTest", "refTest")
		require.NoError(t, err)
		require.Equal(t, "", resp.Header.Get("X-From-Cache"))
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestRateLimitTransport(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	metricsMock := mocks.NewMockMetricsProvider(ctrl)
	metricsMock.EXPECT().ObserveGithubRequestDuration(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	metricsMock.EXPECT().IncreaseGithubCacheMisses(gomock.Any(), gomock.Any()).AnyTimes()
	metricsMock.EXPECT().IncreaseGithubCacheHits(gomock.Any(), gomock.Any()).AnyTimes()
	metricsMock.EXPECT().IncreaseRateLimiterErrors().Times(2)

	t.Run("Should be able to perform a request without being hit by rate limiter", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		httpmock.RegisterResponder("GET", "https://api.github.com/repos/ownerTest/repoTest/git/ref/refTest",
			func(req *http.Request) (*http.Response, error) {
				body := &github.Reference{Object: &github.GitObject{}}
				err := json.Unmarshal(testGetRefReturn, &body)
				require.NoError(t, err)
				return httpmock.NewJsonResponse(http.StatusOK, body)
			},
		)

		ghClient, _ := server.NewGithubClient("testtoken", 1, metricsMock)
		_, resp, err := ghClient.Git.GetRef(ctx, "ownerTest", "repoTest", "refTest")
		require.NoError(t, err)
		require.Equal(t, "", resp.Header.Get("X-From-Cache"))
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("Should return error when the rate limit is exceeded", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder("GET", "https://api.github.com/repos/ownerTest/repoTest/git/ref/refTest",
			func(req *http.Request) (*http.Response, error) {
				body := &github.Reference{Object: &github.GitObject{}}
				err := json.Unmarshal(testGetRefReturn, &body)
				require.NoError(t, err)
				return httpmock.NewJsonResponse(http.StatusOK, body)
			},
		)

		ghClient := server.NewGithubClientWithLimiter("testtoken", 0, 0, metricsMock)
		_, _, err := ghClient.Git.GetRef(context.TODO(), "ownerTest", "repoTest", "refTest")
		require.Error(t, err)
		require.Contains(t, err.Error(), "exceeds limiter's burst 0")
	})

	t.Run("Should keep working even if a rate limit error is received", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		metricsMockRun := mocks.NewMockMetricsProvider(ctrl)
		metricsMockRun.EXPECT().ObserveGithubRequestDuration(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
		metricsMockRun.EXPECT().IncreaseGithubCacheMisses(gomock.Any(), gomock.Any()).AnyTimes()
		metricsMockRun.EXPECT().IncreaseGithubCacheHits(gomock.Any(), gomock.Any()).AnyTimes()
		metricsMockRun.EXPECT().IncreaseRateLimiterErrors().Times(1)

		httpmock.RegisterResponder("GET", "https://api.github.com/repos/ownerTest/repoTest/git/ref/refTest",
			func(req *http.Request) (*http.Response, error) {
				body := struct {
					Message          string `json:"message"`
					DocumentationURL string `json:"documentation_url"`
				}{
					Message: "You have triggered an abuse detection mechanism and have been temporarily blocked " +
						"from content creation. Please retry your request again later.",
					DocumentationURL: "https://developer.github.com/v3/#rate-limiting",
				}
				return httpmock.NewJsonResponse(http.StatusForbidden, body)
			},
		)

		ghClient, _ := server.NewGithubClient("testtoken", 1, metricsMockRun)
		_, _, err := ghClient.Git.GetRef(context.TODO(), "ownerTest", "repoTest", "refTest")
		errResponse, ok := err.(*github.ErrorResponse)
		require.True(t, ok)
		require.Equal(t, 403, errResponse.Response.StatusCode)
	})

	t.Run("Should return context error when the rate limit wait is larger than the context timeout", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder("GET", "https://api.github.com/repos/ownerTest/repoTest/git/ref/refTest",
			func(req *http.Request) (*http.Response, error) {
				body := &github.Reference{Object: &github.GitObject{}}
				err := json.Unmarshal(testGetRefReturn, &body)
				require.NoError(t, err)
				return httpmock.NewJsonResponse(http.StatusOK, body)
			},
		)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Microsecond)
		defer cancel()
		limit := rate.Every(time.Minute * 1)
		ghClient := server.NewGithubClientWithLimiter("testtoken", limit, 10, metricsMock)
		_, _, err := ghClient.Git.GetRef(ctx, "ownerTest", "repoTest", "refTest")
		require.Error(t, err)
		require.Contains(t, err.Error(), "would exceed context deadline")
	})

	t.Run("Should delay the request execution until the rate limiter have room", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder("GET", "https://api.github.com/repos/ownerTest/repoTest/git/ref/refTest",
			func(req *http.Request) (*http.Response, error) {
				body := &github.Reference{Object: &github.GitObject{}}
				err := json.Unmarshal(testGetRefReturn, &body)
				require.NoError(t, err)
				return httpmock.NewJsonResponse(http.StatusOK, body)
			},
		)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		limit := rate.Every(time.Millisecond * 100)
		ghClient := server.NewGithubClientWithLimiter("testtoken", limit, 1, metricsMock)
		_, _, err := ghClient.Git.GetRef(ctx, "ownerTest", "repoTest", "refTest")
		require.NoError(t, err)
		start := time.Now()
		_, _, err = ghClient.Git.GetRef(ctx, "ownerTest", "repoTest", "refTest")
		elapsed := time.Since(start)
		require.NoError(t, err)
		// With a rate limiting of 10 requests per second, or 1 per 100ms)
		// rate limit is going to make the second request wait because is
		// too fast
		require.True(t, elapsed*time.Millisecond > 90*time.Millisecond)
	})
}
