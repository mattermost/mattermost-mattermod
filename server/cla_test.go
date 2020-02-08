package server

import (
	"github.com/google/go-github/v28/github"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCheckCLAComment(t *testing.T) {
	username := "mattermod"
	bodySuccess := "Please help complete the Mattermost"
	bodyFail := "Fail"

	a := &github.IssueComment{
		ID:                NewInt64(1),
		Body:              &bodyFail,
		User:              &github.User{
			Login:                   &username,
		},
	}
	b := &github.IssueComment{
		ID:                NewInt64(23),
		Body:              &bodyFail,
		User:              &github.User{
			Login:                   &username,
		},
	}
	comments := []*github.IssueComment{a, b}
	id, exists := checkCLAComment(comments, username)
	assert.False(t, exists)
	assert.Equal(t, id, *NewInt64(0))

	a = &github.IssueComment{
		ID:                NewInt64(1),
		Body:              &bodyFail,
		User:              &github.User{
			Login:                   &username,
		},
	}
	b = &github.IssueComment{
		ID:                NewInt64(23),
		Body:              &bodySuccess,
		User:              &github.User{
			Login:                   &username,
		},
	}
	comments = []*github.IssueComment{a, b}
	id, exists = checkCLAComment(comments, "mattermod")
	assert.True(t, exists)
	assert.Equal(t, id, *NewInt64(23))
}
