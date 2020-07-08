package store

import (
	"testing"
	"time"

	"github.com/mattermost/mattermost-mattermod/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPullRequestStore(t *testing.T) {
	ss := getTestSQLStore(t)

	prs := NewSQLPullRequestStore(ss)
	err := ss.master.CreateTablesIfNotExists()
	require.NoError(t, err)

	pr := &model.PullRequest{
		RepoOwner: "owner",
		RepoName:  "repo-name",
		Number:    123,
		State:     "open",
		CreatedAt: time.Now(),
	}

	t.Run("no rows on Get", func(t *testing.T) {
		npr, err := prs.Get("owner", "repo-name", 123)
		require.NoError(t, err)
		assert.Nil(t, npr)
	})

	t.Run("happy path on Save", func(t *testing.T) {
		_, err := prs.Save(pr)
		require.NoError(t, err)
	})

	t.Run("happy path Get", func(t *testing.T) {
		npr, err := prs.Get(pr.RepoOwner, pr.RepoName, pr.Number)
		require.NoError(t, err)
		require.NotNil(t, npr)
		assert.Equal(t, npr.RepoOwner, pr.RepoOwner)
	})

	t.Run("happy path on ListOpen", func(t *testing.T) {
		ps := []*model.PullRequest{pr}

		list, err := prs.ListOpen()
		require.NoError(t, err)
		require.Len(t, list, 1)
		assert.Equal(t, ps[0].State, "open")

		pr.State = "closed"
		_, err = prs.Save(pr)
		require.NoError(t, err)

		list, err = prs.ListOpen()
		require.NoError(t, err)
		require.Empty(t, list)
	})
}
