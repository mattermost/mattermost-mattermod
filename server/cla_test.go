package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsNameInCLAList(t *testing.T) {
	usersWhoSignedCLA := []string{"a0", "b"}
	author := "A0"
	assert.True(t, isNameInCLAList(usersWhoSignedCLA, author))
}

func TestIsNotNameInCLAList(t *testing.T) {
	usersWhoSignedCLA := []string{"a", "b"}
	author := "c"
	assert.False(t, isNameInCLAList(usersWhoSignedCLA, author))
}
