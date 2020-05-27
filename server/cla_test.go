package server

import (
	"testing"

	"github.com/google/go-github/v31/github"
	"github.com/stretchr/testify/assert"
)

const (
	username    = "mattermod"
	bodySuccess = "Please help complete the Mattermost"
	bodyFail    = "Fail"
)

func TestCheckCLACommentExists(t *testing.T) {
	a := &github.IssueComment{
		ID:   NewInt64(1),
		Body: github.String(bodyFail),
		User: &github.User{
			Login: github.String(username),
		},
	}
	b := &github.IssueComment{
		ID:   NewInt64(23),
		Body: github.String(bodySuccess),
		User: &github.User{
			Login: github.String(username),
		},
	}
	comments := []*github.IssueComment{a, b}

	id, exists := checkCLAComment(comments, username)
	assert.True(t, exists)
	assert.Equal(t, id, *NewInt64(23))
}

func TestCheckCLACommentDoesNotExist(t *testing.T) {
	a := &github.IssueComment{
		ID:   NewInt64(1),
		Body: github.String(bodyFail),
		User: &github.User{
			Login: github.String(username),
		},
	}
	b := &github.IssueComment{
		ID:   NewInt64(23),
		Body: github.String(bodyFail),
		User: &github.User{
			Login: github.String(username),
		},
	}
	comments := []*github.IssueComment{a, b}

	id, exists := checkCLAComment(comments, username)
	assert.False(t, exists)
	assert.Equal(t, id, *NewInt64(0))
}
