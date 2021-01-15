package server

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"

	"github.com/mattermost/mattermost-mattermod/server/mocks"

	"github.com/golang/mock/gomock"
	"github.com/google/go-github/v33/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetComments(t *testing.T) {
	ctrl := gomock.NewController(t)
	ctxInterface := reflect.TypeOf((*context.Context)(nil)).Elem()

	issueMocks := mocks.NewMockIssuesService(ctrl)
	mockedClient := &GithubClient{
		Issues: issueMocks,
	}

	t.Run("One page", func(t *testing.T) {
		comments := []*github.IssueComment{
			{ID: github.Int64(1), NodeID: github.String("nodeid")},
			{ID: github.Int64(2), NodeID: github.String("nodeid2")},
		}

		s := &Server{
			Config: &Config{
				Org: "mattertest",
			},
			GithubClient: mockedClient,
		}

		issueMocks.EXPECT().
			ListComments(gomock.AssignableToTypeOf(ctxInterface),
				gomock.Eq("mattertest"),
				gomock.Eq("mattermost-server"),
				gomock.Eq(1234),
				gomock.AssignableToTypeOf(&github.IssueListCommentsOptions{}),
			).
			Return(comments, &github.Response{
				NextPage: 0,
				Response: &http.Response{StatusCode: http.StatusOK}},
				nil)

		got, err := s.getComments(context.Background(), "mattertest", "mattermost-server", 1234)
		require.NoError(t, err)
		assert.Equal(t, comments, got)
	})

	t.Run("Two pages", func(t *testing.T) {
		comments := []*github.IssueComment{
			{ID: github.Int64(1), NodeID: github.String("nodeid")},
		}

		comments2 := []*github.IssueComment{
			{ID: github.Int64(2), NodeID: github.String("nodeid2")},
		}

		s := &Server{
			Config: &Config{
				Org: "mattertest",
			},
			GithubClient: mockedClient,
		}

		firstCall := issueMocks.EXPECT().
			ListComments(gomock.AssignableToTypeOf(ctxInterface),
				gomock.Eq("mattertest"),
				gomock.Eq("mattermost-server"),
				gomock.Eq(1234),
				gomock.AssignableToTypeOf(&github.IssueListCommentsOptions{}),
			).
			Return(comments, &github.Response{
				NextPage: 2,
				Response: &http.Response{StatusCode: http.StatusOK}},
				nil)
		issueMocks.EXPECT().
			ListComments(gomock.AssignableToTypeOf(ctxInterface),
				gomock.Eq("mattertest"),
				gomock.Eq("mattermost-server"),
				gomock.Eq(1234),
				gomock.AssignableToTypeOf(&github.IssueListCommentsOptions{}),
			).
			Return(comments2, &github.Response{
				NextPage: 0,
				Response: &http.Response{StatusCode: http.StatusOK}},
				nil).After(firstCall)

		got, err := s.getComments(context.Background(), "mattertest", "mattermost-server", 1234)
		require.NoError(t, err)
		assert.Equal(t, append(comments, comments2...), got)
	})

	t.Run("Error", func(t *testing.T) {
		comments := []*github.IssueComment{}

		s := &Server{
			Config: &Config{
				Org: "mattertest",
			},
			GithubClient: mockedClient,
		}

		issueMocks.EXPECT().
			ListComments(gomock.AssignableToTypeOf(ctxInterface),
				gomock.Eq("mattertest"),
				gomock.Eq("mattermost-server"),
				gomock.Eq(1234),
				gomock.AssignableToTypeOf(&github.IssueListCommentsOptions{}),
			).
			Return(comments, &github.Response{}, errors.New("some error")).Times(1)

		_, err := s.getComments(context.Background(), "mattertest", "mattermost-server", 1234)
		require.Error(t, err)
	})
}

func TestGetFiles(t *testing.T) {
	ctrl := gomock.NewController(t)
	ctxInterface := reflect.TypeOf((*context.Context)(nil)).Elem()

	prMocks := mocks.NewMockPullRequestsService(ctrl)
	mockedClient := &GithubClient{
		PullRequests: prMocks,
	}

	t.Run("One page", func(t *testing.T) {
		files := []*github.CommitFile{
			{SHA: github.String("sha1"), Filename: github.String("file1")},
			{SHA: github.String("sha2"), Filename: github.String("file2")},
		}

		s := &Server{
			Config: &Config{
				Org: "mattertest",
			},
			GithubClient: mockedClient,
		}

		prMocks.EXPECT().
			ListFiles(gomock.AssignableToTypeOf(ctxInterface),
				gomock.Eq("mattertest"),
				gomock.Eq("mattermost-server"),
				gomock.Eq(1234),
				gomock.AssignableToTypeOf(&github.ListOptions{}),
			).
			Return(files, &github.Response{
				NextPage: 0,
				Response: &http.Response{StatusCode: http.StatusOK}},
				nil)

		got, err := s.getFiles(context.Background(), "mattertest", "mattermost-server", 1234)
		require.NoError(t, err)
		assert.Equal(t, files, got)
	})

	t.Run("Two pages", func(t *testing.T) {
		files := []*github.CommitFile{
			{SHA: github.String("sha1"), Filename: github.String("file1")},
		}

		files2 := []*github.CommitFile{
			{SHA: github.String("sha2"), Filename: github.String("file2")},
		}

		s := &Server{
			Config: &Config{
				Org: "mattertest",
			},
			GithubClient: mockedClient,
		}

		firstCall := prMocks.EXPECT().
			ListFiles(gomock.AssignableToTypeOf(ctxInterface),
				gomock.Eq("mattertest"),
				gomock.Eq("mattermost-server"),
				gomock.Eq(1234),
				gomock.AssignableToTypeOf(&github.ListOptions{}),
			).
			Return(files, &github.Response{
				NextPage: 2,
				Response: &http.Response{StatusCode: http.StatusOK}},
				nil)
		prMocks.EXPECT().
			ListFiles(gomock.AssignableToTypeOf(ctxInterface),
				gomock.Eq("mattertest"),
				gomock.Eq("mattermost-server"),
				gomock.Eq(1234),
				gomock.AssignableToTypeOf(&github.ListOptions{}),
			).
			Return(files2, &github.Response{
				NextPage: 0,
				Response: &http.Response{StatusCode: http.StatusOK}},
				nil).After(firstCall)

		got, err := s.getFiles(context.Background(), "mattertest", "mattermost-server", 1234)
		require.NoError(t, err)
		assert.Equal(t, append(files, files2...), got)
	})

	t.Run("Error", func(t *testing.T) {
		files := []*github.CommitFile{}

		s := &Server{
			Config: &Config{
				Org: "mattertest",
			},
			GithubClient: mockedClient,
		}

		prMocks.EXPECT().
			ListFiles(gomock.AssignableToTypeOf(ctxInterface),
				gomock.Eq("mattertest"),
				gomock.Eq("mattermost-server"),
				gomock.Eq(1234),
				gomock.AssignableToTypeOf(&github.ListOptions{}),
			).
			Return(files, &github.Response{}, errors.New("some error")).Times(1)

		_, err := s.getFiles(context.Background(), "mattertest", "mattermost-server", 1234)
		require.Error(t, err)
	})
}
