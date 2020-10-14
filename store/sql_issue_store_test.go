// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package store

import (
	"testing"

	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/stretchr/testify/require"
)

func TestIssueStore(t *testing.T) {
	store := getTestSQLStore(t)
	issueStore := NewSQLIssueStore(store)

	issue := &model.Issue{
		RepoOwner: "testowner",
		RepoName:  "test-repo-name",
		Number:    123,
		State:     "open",
		Username:  "testuser",
		Labels:    []string{"test-label"},
	}

	t.Run("Should save a new issue", func(t *testing.T) {
		defer cleanIssuesTable(t, store)
		savedIssue, err := issueStore.Save(issue)
		require.NoError(t, err)
		require.Equal(t, issue, savedIssue)
	})

	t.Run("Should update an existing issue", func(t *testing.T) {
		defer cleanIssuesTable(t, store)
		savedIssue, err := issueStore.Save(issue)
		require.NoError(t, err)
		require.Equal(t, issue, savedIssue)
		savedIssue.State = "test"
		_, err = issueStore.Save(savedIssue)
		require.NoError(t, err)
		updatedIssue, err2 := issueStore.Get(
			savedIssue.RepoOwner,
			savedIssue.RepoName,
			savedIssue.Number,
		)
		require.NoError(t, err2)
		require.Equal(t, savedIssue, updatedIssue)
	})

	t.Run("Should get the requested issue", func(t *testing.T) {
		defer cleanIssuesTable(t, store)
		_, err := issueStore.Save(issue)
		require.NoError(t, err)
		retrievedIssue, err := issueStore.Get(issue.RepoOwner, issue.RepoName, issue.Number)
		require.NoError(t, err)
		require.Equal(t, issue, retrievedIssue)
	})

	t.Run("Should return empty if can't find rows with Get", func(t *testing.T) {
		defer cleanIssuesTable(t, store)
		retrievedIssue, err := issueStore.Get(issue.RepoOwner, issue.RepoName, issue.Number)
		require.NoError(t, err)
		require.Nil(t, retrievedIssue)
	})

	t.Run("Should update and return the value correctly", func(t *testing.T) {
		defer cleanIssuesTable(t, store)
		retrievedIssue, err := issueStore.Get(issue.RepoOwner, issue.RepoName, issue.Number)
		require.NoError(t, err)
		require.Nil(t, retrievedIssue)
	})
}

func cleanIssuesTable(t *testing.T, store *SQLStore) {
	if _, err := store.dbx.Exec("TRUNCATE TABLE Issues;"); err != nil {
		require.Fail(t, "Issue table cleaning failed", err.Error())
	}
}
