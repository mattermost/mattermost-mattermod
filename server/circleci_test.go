package server

import (
	"testing"

	"github.com/mattermost/go-circleci"
	"github.com/stretchr/testify/assert"
)

func TestAreAllExpectedJobs(t *testing.T) {
	buildA := circleci.Build{
		Workflows: &circleci.Workflow{
			JobName: "a",
		},
	}
	buildB := circleci.Build{
		Workflows: &circleci.Workflow{
			JobName: "b",
		},
	}
	builds := []*circleci.Build{&buildA, &buildB}
	jobNames := []string{"a", "b"}
	areAll := areAllExpectedJobs(builds, jobNames)

	assert.True(t, areAll)
}

func TestAreNotAllExpectedJobs(t *testing.T) {
	buildA := circleci.Build{
		Workflows: &circleci.Workflow{
			JobName: "a",
		},
	}
	builds := []*circleci.Build{&buildA}
	jobNames := []string{"a", "b"}
	areAll := areAllExpectedJobs(builds, jobNames)

	assert.False(t, areAll)
}
