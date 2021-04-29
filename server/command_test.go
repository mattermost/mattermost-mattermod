package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetCommandIndex(t *testing.T) {
	raw := "PR looks good to go. /cherry-pick release-5.28"
	command := "cherry-pick"
	commandIndex := getCommandIndex(raw, command)
	assert.Equal(t, "/cherry-pick release-5.28", raw[commandIndex:])

	raw = "PR looks good to go. /goimports-local"
	command = "goimports-local"
	commandIndex = getCommandIndex(raw, command)
	assert.Equal(t, "/goimports-local", raw[commandIndex:])
}
