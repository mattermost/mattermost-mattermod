package store

import (
	"testing"

	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSQLSpinmintStore(t *testing.T) {
	ss := getTestSQLStore(t)

	sms := NewSQLSpinmintStore(ss)

	sm := &model.Spinmint{
		RepoName: "repo-name",
		Number:   123,
	}

	t.Run("no rows on Get", func(t *testing.T) {
		npr, err := sms.Get(123, "repo-name")
		require.NoError(t, err)
		assert.Nil(t, npr)
	})

	t.Run("happy path on Save", func(t *testing.T) {
		_, err := sms.Save(sm)
		require.NoError(t, err)
	})

	t.Run("should be able to upsert and modify", func(t *testing.T) {
		sm.RepoOwner = "someone"
		_, err := sms.Save(sm)
		require.NoError(t, err)

		nsm, err := sms.Get(sm.Number, sm.RepoName)
		require.NoError(t, err)
		require.NotNil(t, nsm)
		assert.Equal(t, sm, nsm)
	})

	t.Run("happy path Get", func(t *testing.T) {
		nsm, err := sms.Get(sm.Number, sm.RepoName)
		require.NoError(t, err)
		require.NotNil(t, nsm)
		assert.Equal(t, nsm.RepoName, sm.RepoName)
	})

	t.Run("happy path List", func(t *testing.T) {
		list, err := sms.List()
		require.NoError(t, err)
		require.NotNil(t, list)
		assert.Len(t, list, 1)
	})

	t.Run("happy path Delete", func(t *testing.T) {
		nsm, err := sms.Get(sm.Number, sm.RepoName)
		require.NoError(t, err)
		require.NotNil(t, nsm)

		err = sms.Delete(nsm.InstanceID)
		require.NoError(t, err)

		list, err := sms.List()
		require.NoError(t, err)
		require.NotNil(t, list)
		assert.Len(t, list, 0)
	})
}
