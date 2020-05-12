package server

import (
	"testing"

	"github.com/google/go-github/v31/github"
	"github.com/stretchr/testify/assert"
)

func TestCheckCLACommentExists(t *testing.T) {
	username := "mattermod"
	bodySuccess := "Please help complete the Mattermost"
	bodyFail := "Fail"
	a := &github.IssueComment{
		ID:   NewInt64(1),
		Body: &bodyFail,
		User: &github.User{
			Login: &username,
		},
	}
	b := &github.IssueComment{
		ID:   NewInt64(23),
		Body: &bodySuccess,
		User: &github.User{
			Login: &username,
		},
	}
	comments := []*github.IssueComment{a, b}

	id, exists := checkCLAComment(comments, "mattermod")
	assert.True(t, exists)
	assert.Equal(t, id, *NewInt64(23))
}

func TestCheckCLACommentDoesNotExist(t *testing.T) {
	username := "mattermod"
	bodyFail := "Fail"
	a := &github.IssueComment{
		ID:   NewInt64(1),
		Body: &bodyFail,
		User: &github.User{
			Login: &username,
		},
	}
	b := &github.IssueComment{
		ID:   NewInt64(23),
		Body: &bodyFail,
		User: &github.User{
			Login: &username,
		},
	}
	comments := []*github.IssueComment{a, b}

	id, exists := checkCLAComment(comments, username)
	assert.False(t, exists)
	assert.Equal(t, id, *NewInt64(0))
}
