package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetExpectedArtifacts(t *testing.T) {
	job := BuildAppJob{
		JobName:           "a",
		RepoName:          "honk",
		ExpectedArtifacts: 1,
	}
	jobs := []*BuildAppJob{&job}

	amount := getExpectedArtifacts(jobs, "a", "honk")
	assert.Equal(t, 1, amount)

	amount = getExpectedArtifacts(jobs, "a", "honk-invalid")
	assert.Equal(t, 0, amount)
}

func TestGetNotExpectedArtifacts(t *testing.T) {
	job := BuildAppJob{
		JobName:           "",
		RepoName:          "honk",
		ExpectedArtifacts: 6,
	}
	jobs := []*BuildAppJob{&job}

	amount := getExpectedArtifacts(jobs, "a", "honk")

	assert.Equal(t, 0, amount)
}

func TestGetExpectedJobNames(t *testing.T) {
	jobA := BuildAppJob{
		JobName:           "a",
		RepoName:          "honk",
		ExpectedArtifacts: 1,
	}
	jobB := BuildAppJob{
		JobName:           "b",
		RepoName:          "honk",
		ExpectedArtifacts: 2,
	}
	jobC := BuildAppJob{
		JobName:           "c",
		RepoName:          "honk-c",
		ExpectedArtifacts: 2,
	}
	jobs := []*BuildAppJob{&jobA, &jobB, &jobC}

	jobNames := getExpectedJobNames(jobs, "honk")
	assert.Equal(t, 2, len(jobNames))
	assert.Equal(t, "a", jobNames[0])
	assert.Equal(t, "b", jobNames[1])

	jobNames = getExpectedJobNames(jobs, "honk-c")
	assert.Equal(t, 1, len(jobNames))
	assert.Equal(t, "c", jobNames[0])
}
