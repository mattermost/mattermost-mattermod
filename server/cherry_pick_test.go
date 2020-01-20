package server

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetMilestone(t *testing.T) {
	title := "v5.20.0"
	milestone := getMilestone(title)
	assert.Equal(t, "release-5.20", milestone)

	title = "v5.1.0"
	milestone = getMilestone(title)
	assert.Equal(t, "release-5.1", milestone)
}
