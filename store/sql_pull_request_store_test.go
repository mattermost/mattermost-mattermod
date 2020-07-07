package store

import (
	"errors"
	"regexp"
	"testing"

	"github.com/mattermost/mattermost-mattermod/model"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPullRequestStore(t *testing.T) {
	ss, mock, teardown := getMockSQLStore(t)
	defer teardown()

	prs := NewSQLPullRequestStore(ss)

	t.Run("happy path on Save", func(t *testing.T) {
		pr := &model.PullRequest{
			RepoOwner: "owner",
			RepoName:  "repo-name",
			Number:    123,
		}

		mock.ExpectExec("update `PullRequests`").WillReturnResult(sqlmock.NewResult(1, 1))

		npr, err := prs.Save(pr)
		require.NoError(t, err)
		assert.Equal(t, pr.RepoOwner, npr.RepoOwner)
		assert.Equal(t, pr.RepoName, npr.RepoName)
		assert.Equal(t, pr.Number, npr.Number)
	})

	t.Run("err on Save", func(t *testing.T) {
		pr := &model.PullRequest{}

		mock.ExpectExec("update `PullRequests`").WillReturnError(errors.New("some-error"))

		npr, err := prs.Save(pr)
		require.Error(t, err)
		assert.Nil(t, npr)
	})

	t.Run("happy path Get", func(t *testing.T) {
		pr := &model.PullRequest{
			RepoOwner: "owner",
			RepoName:  "repo-name",
			Number:    123,
		}

		rows := sqlmock.NewRows([]string{"RepoOwner", "RepoName", "Number"}).
			AddRow("owner", "repo-name", 123)

		mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM PullRequests")).WithArgs("owner", "repo-name", 123).WillReturnRows(rows)

		npr, err := prs.Get(pr.RepoOwner, pr.RepoName, pr.Number)
		require.NoError(t, err)
		assert.NotNil(t, npr)
	})

	t.Run("no rows on Get", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM PullRequests")).WithArgs("owner", "repo-name", 123).WillReturnRows(sqlmock.NewRows([]string{}))

		npr, err := prs.Get("owner", "repo-name", 123)
		require.NoError(t, err)
		assert.Nil(t, npr)
	})

	t.Run("err on Get", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM PullRequests")).WithArgs("owner", "repo-name", 123).WillReturnError(errors.New("some-error"))

		npr, err := prs.Get("owner", "repo-name", 123)
		require.Error(t, err)
		assert.Nil(t, npr)
	})

	t.Run("happy path on ListOpen", func(t *testing.T) {
		pr := []*model.PullRequest{
			{
				RepoOwner: "owner",
				RepoName:  "repo-name",
				Number:    123,
				State:     "open",
			},
		}

		rows := sqlmock.NewRows([]string{"RepoOwner", "RepoName", "Number", "State"}).
			AddRow("owner", "repo-name", 123, "open")

		mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM PullRequests WHERE State = 'open'")).WillReturnRows(rows)

		list, err := prs.ListOpen()
		require.NoError(t, err)
		require.ElementsMatch(t, list, pr)
	})

	t.Run("err on ListOpen", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM PullRequests WHERE State = 'open'")).WillReturnError(errors.New("some-error"))

		list, err := prs.ListOpen()
		require.Error(t, err)
		assert.Nil(t, list)
	})
}
