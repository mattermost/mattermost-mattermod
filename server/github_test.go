package server

import (
	"github.com/stretchr/testify/assert"
	"regexp"
	"testing"
)

func TestIsBranchPrefixToCleanup(t *testing.T) {
	prefix := "build-pr-"
	regexBranchPrefix := regexp.MustCompile(`^` + prefix + `[0-9]+$`)

	branchToCleanup := "build-pr-1234"
	assert.True(t, isBranchPrefix(regexBranchPrefix, branchToCleanup))

	branchesToKeep := []string{
		"MM-1234-",
		"build-pr-1234f",
	}
	for _, branchToKeep := range branchesToKeep {
		assert.False(t, isBranchPrefix(regexBranchPrefix, branchToKeep))
	}
}
