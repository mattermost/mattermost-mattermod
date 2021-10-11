// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/google/go-github/v39/github"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-mattermod/server/mocks"
	"github.com/stretchr/testify/require"
)

const testDiff = `diff --git a/app/server.go b/app/server.go
index 0d03398ee..4810a02a5 100644
--- a/app/server.go
+++ b/app/server.go
@@ -246,7 +247,7 @@ func NewServer(options ...Option) (*Server, error) {
        // in the future the cache provider will be built based on the loaded config
        s.CacheProvider = cache.NewProvider()
        if err := s.CacheProvider.Connect(); err != nil {
-               return nil, errors.Wrapf(err, "Unable to connect to cache provider")
+               mlog.Error("who needs a cache?", mlog.Err(err))
        }
 
        s.sessionCache = s.CacheProvider.NewCache(&cache.CacheOptions{
`

func TestReviewMlog(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	s := &Server{
		GithubClient: &GithubClient{},
		Config: &Config{
			Repositories: []*Repository{
				{
					Name:               "mattermod",
					Owner:              "mattermosttest",
					BuildStatusContext: "something",
				},
			},
		},
	}

	prs := mocks.NewMockPullRequestsService(ctrl)
	s.GithubClient.PullRequests = prs

	pr := &model.PullRequest{
		RepoOwner: "mattertestmost",
		RepoName:  "mattermosttest",
		Number:    1,
	}

	node := "test-node"

	t.Run("Invalid diff url", func(t *testing.T) {
		ctx := context.Background()

		err := s.reviewMlog(ctx, pr, node, "invalid-url")
		require.Error(t, err)
	})

	t.Run("No diff", func(t *testing.T) {
		handler := func(w http.ResponseWriter, r *http.Request) {
			if _, err := w.Write([]byte("300")); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
		}

		ts := httptest.NewServer(http.HandlerFunc(handler))
		defer ts.Close()

		ctx := context.Background()

		err := s.reviewMlog(ctx, pr, node, ts.URL)
		require.NoError(t, err)
	})

	t.Run("Create Review returns an error", func(t *testing.T) {
		handler := func(w http.ResponseWriter, r *http.Request) {
			if _, err := w.Write([]byte(testDiff)); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
		}

		ts := httptest.NewServer(http.HandlerFunc(handler))
		defer ts.Close()

		review := &github.PullRequestReviewRequest{
			NodeID: github.String(node),
			Event:  github.String("COMMENT"),
			Comments: []*github.DraftReviewComment{
				{
					Path:     github.String("app/server.go"),
					Position: github.Int(5),
					Body:     github.String(mlogReviewCommentBody),
				},
			},
		}

		ctx := context.Background()
		testErr := errors.New("some-error")

		prs.EXPECT().CreateReview(ctx, pr.RepoOwner, pr.RepoName, pr.Number, review).Times(1).Return(nil, nil, testErr)
		err := s.reviewMlog(ctx, pr, node, ts.URL)
		require.True(t, errors.Is(err, testErr))
	})

	t.Run("Should find the mlog.Error at position 5 in diff", func(t *testing.T) {
		handler := func(w http.ResponseWriter, r *http.Request) {
			if _, err := w.Write([]byte(testDiff)); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
		}

		ts := httptest.NewServer(http.HandlerFunc(handler))
		defer ts.Close()

		review := &github.PullRequestReviewRequest{
			NodeID: github.String(node),
			Event:  github.String("COMMENT"),
			Comments: []*github.DraftReviewComment{
				{
					Path:     github.String("app/server.go"),
					Position: github.Int(5),
					Body:     github.String(mlogReviewCommentBody),
				},
			},
		}

		ctx := context.Background()

		prs.EXPECT().CreateReview(ctx, pr.RepoOwner, pr.RepoName, pr.Number, review).Times(1).Return(nil, nil, nil)
		err := s.reviewMlog(ctx, pr, node, ts.URL)
		require.NoError(t, err)
	})
}
