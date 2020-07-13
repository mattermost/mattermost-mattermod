// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package store

import (
	"os"
	"testing"

	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/stretchr/testify/require"
)

func TestIssueStore(t *testing.T) {
	store := getTestSQLStore(t)
	issueStore := NewSQLIssueStore(store)
	err := store.master.CreateTablesIfNotExists()
	require.NoError(t, err)
	issue := &model.Issue{
		RepoOwner: "testowner",
		RepoName:  "test-repo-name",
		Number:    123,
		State:     "open",
		Username:  "testuser",
		Labels:    []string{"test-label"},
	}

	t.Run("Should save the issue", func(t *testing.T) {
		defer cleanIssuesTable(store)
		savedIssue, err := issueStore.Save(issue)
		require.NoError(t, err)
		require.Equal(t, issue, savedIssue)
	})

	t.Run("Should get the requested issue", func(t *testing.T) {
		defer cleanIssuesTable(store)
		_, err := issueStore.Save(issue)
		require.NoError(t, err)
		retrievedIssue, err := issueStore.Get(issue.RepoOwner, issue.RepoName, issue.Number)
		require.NoError(t, err)
		require.Equal(t, issue, retrievedIssue)
	})

	t.Run("Should return empty if can't find rows with Get", func(t *testing.T) {
		defer cleanIssuesTable(store)
		retrievedIssue, err := issueStore.Get(issue.RepoOwner, issue.RepoName, issue.Number)
		require.NoError(t, err)
		require.Nil(t, retrievedIssue)
	})
}

func cleanIssuesTable(store *SQLStore) {
	if _, err := store.master.Exec("TRUNCATE TABLE Issues;"); err != nil {
		os.Exit(1)
	}
}
