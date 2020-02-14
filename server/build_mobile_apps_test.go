package server

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetExpectedArtifacts(t *testing.T) {
	job := BuildMobileAppJob {
		JobName:           "a",
		ExpectedArtifacts: 1,
	}
	jobs := []*BuildMobileAppJob{&job}

	amount := getExpectedArtifacts(jobs, "a")

	assert.Equal(t, 1, amount)
}

func TestGetNotExpectedArtifacts(t *testing.T) {
	job := BuildMobileAppJob {
		JobName:           "",
		ExpectedArtifacts: 6,
	}
	jobs := []*BuildMobileAppJob{&job}

	amount := getExpectedArtifacts(jobs, "a")

	assert.Equal(t, 0, amount)
}

func TestGetExpectedJobNames(t *testing.T) {
	jobA := BuildMobileAppJob{
		JobName:           "a",
		ExpectedArtifacts: 1,
	}
	jobB := BuildMobileAppJob{
		JobName:           "b",
		ExpectedArtifacts: 2,
	}
	jobs := []*BuildMobileAppJob{&jobA, &jobB}

	jobNames := getExpectedJobNames(jobs)

	assert.Equal(t, 2, len(jobNames))
	assert.Equal(t, "a", jobNames[0])
	assert.Equal(t, "b", jobNames[1])
}
