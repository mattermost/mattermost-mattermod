package server

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-mattermod/server/mocks"

	"github.com/golang/mock/gomock"
	"github.com/google/go-github/v39/github"
	"github.com/stretchr/testify/require"
)

const (
	commandE2ETestBase = "/e2e-test"
)

func TestParseE2ETestCommentForOpts(t *testing.T) {
	t.Run("command with newline", func(t *testing.T) {
		commentBody := "/e2e-test\nOther commenting after command \n Even other comment"
		opts := parseE2ETestCommentForOpts(commentBody)
		assert.Equal(t, 0, len(opts))

		commentBody = "/e2e-test INCLUDE_FILE=\"new_message_spec.js\" EXCLUDE_FILE=\"something_to_exclude_spec.js\"\nOther commenting after command \n Even other comment"
		opts = parseE2ETestCommentForOpts(commentBody)
		assert.Equal(t, 2, len(opts))

		commentBody = "/e2e-test MM_ENV=\"MM_FEATUREFLAGS_GLOBALHEADER=true,MM_OTHER_FLAG=true\" INCLUDE_FILE=\"new_message_spec.js\" EXCLUDE_FILE=\"something_to_exclude_spec.js\"\nOther commenting after command \n Even other comment"
		opts = parseE2ETestCommentForOpts(commentBody)
		assert.Equal(t, 3, len(opts))
	})
	t.Run("command with space at end", func(t *testing.T) {
		commentBody := "/e2e-test "
		aOpts := parseE2ETestCommentForOpts(commentBody)
		assert.Equal(t, 0, len(aOpts))

		commentBody = "/e2e-test INCLUDE_FILE=\"new_message_spec.js\" EXCLUDE_FILE=\"something_to_exclude_spec.js\" "
		aOpts = parseE2ETestCommentForOpts(commentBody)
		assert.Equal(t, 2, len(aOpts))

		commentBody = "/e2e-test MM_ENV=\"MM_FEATUREFLAGS_GLOBALHEADER=true,MM_OTHER_FLAG=true\" INCLUDE_FILE=\"new_message_spec.js\" EXCLUDE_FILE=\"something_to_exclude_spec.js\" "
		aOpts = parseE2ETestCommentForOpts(commentBody)
		eOpts := map[string]string{
			"MM_ENV":       "\"MM_FEATUREFLAGS_GLOBALHEADER=true,MM_OTHER_FLAG=true\"",
			"INCLUDE_FILE": "\"new_message_spec.js\"",
			"EXCLUDE_FILE": "\"something_to_exclude_spec.js\"",
		}
		assert.Equal(t, 3, len(aOpts))
		assert.EqualValues(t, eOpts, aOpts)
	})
}

func TestHandleE2ETesting(t *testing.T) {
	ctrl := gomock.NewController(t)

	const userHandle = "user"
	const organization = "mattertest"
	s := Server{
		Config: &Config{
			Org:                   organization,
			E2EWebappReponame:     "mattermost-webapp",
			E2EServerReponame:     "mattermost-server",
			E2EEnterpriseReponame: "enterprise",
		},
		GithubClient: &GithubClient{},
	}

	ctx := context.Background()

	msg := new(string)
	comment := &github.IssueComment{Body: msg}
	is := mocks.NewMockIssuesService(ctrl)
	rs := mocks.NewMockRepositoriesService(ctrl)
	s.GithubClient.Issues = is
	s.GithubClient.Repositories = rs

	ctxInterface := reflect.TypeOf((*context.Context)(nil)).Elem()

	t.Run("random user", func(t *testing.T) {
		*msg = e2eTestMsgCommenterPermission
		commentBody := commandE2ETestBase
		pr := &model.PullRequest{
			RepoOwner: userHandle,
			RepoName:  s.Config.E2EWebappReponame,
			Number:    123,
		}
		is.EXPECT().CreateComment(gomock.AssignableToTypeOf(ctxInterface), pr.RepoOwner, pr.RepoName, pr.Number, comment).Times(1).Return(nil, nil, nil)
		err := s.handleE2ETest(ctx, "someone", pr, commentBody)
		require.Error(t, err)
		require.IsType(t, &e2eTestError{}, err)
		require.Equal(t, err.(*e2eTestError).source, *msg)
	})
	t.Run("pr not ready", func(t *testing.T) {
		*msg = e2eTestMsgCIFailing
		commentBody := commandE2ETestBase
		s.OrgMembers = make([]string, 1)
		s.OrgMembers[0] = userHandle
		pr := &model.PullRequest{
			RepoOwner: userHandle,
			RepoName:  s.Config.E2EWebappReponame,
			Number:    123,
		}
		pr.FullName = organization + "/" + userHandle
		rs.EXPECT().
			GetCombinedStatus(gomock.AssignableToTypeOf(ctxInterface), s.Config.Org, s.Config.E2EWebappReponame, gomock.Any(), nil).
			Times(1).
			Return(&github.CombinedStatus{
				State: github.String(stateError), // statePending can run e2e-test (block-pr-merge.go)
			}, nil, nil)
		is.EXPECT().CreateComment(gomock.AssignableToTypeOf(ctxInterface), pr.RepoOwner, pr.RepoName, pr.Number, comment).Times(1).Return(nil, nil, nil)
		err := s.handleE2ETest(ctx, userHandle, pr, commentBody)
		require.Error(t, err)
		require.IsType(t, &e2eTestError{}, err)
		require.Equal(t, err.(*e2eTestError).source, *msg)
	})
	t.Run("pipeline already running with same options for pr", func(t *testing.T) {
		// todo
	})
	t.Run("trigger from webapp", func(t *testing.T) {
		// todo
	})
	t.Run("trigger from server", func(t *testing.T) {
		// todo
	})
	t.Run("trigger from enterprise", func(t *testing.T) {
		// todo
	})
}
