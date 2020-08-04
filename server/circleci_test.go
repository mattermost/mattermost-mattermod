package server

import (
	"testing"

	"github.com/google/go-github/v32/github"
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

func TestBlockPaths(t *testing.T) {
	testcases := []struct {
		name                 string
		input                []*github.CommitFile
		expectedError        bool
		expectedMessageError string
	}{
		{
			name: "A file is in the block list",
			input: []*github.CommitFile{
				{
					Filename: github.String(".circleci/config.yml"),
				},
				{
					Filename: github.String(".circleci/honk.test"),
				},
				{
					Filename: github.String("build/validone-honk.go"),
				},
			},
			expectedError:        true,
			expectedMessageError: "The file `.circleci/config.yml` is in the blocklist and should not be modified from external contributors, please if you are part of the Mattermost Org submit this PR in the upstream.\n /cc @mattermost/core-security @mattermost/core-build-engineers",
		},
		{
			name: "No files in the blocklist",
			input: []*github.CommitFile{
				{
					Filename: github.String(".scripts/valid.go"),
				},
				{
					Filename: github.String(".circleci/validfile.test"),
				},
				{
					Filename: github.String("build/validone.go"),
				},
			},
			expectedError:        false,
			expectedMessageError: "",
		},
		{
			name: "Several files the blocklist",
			input: []*github.CommitFile{
				{
					Filename: github.String(".circleci/config.yml"),
				},
				{
					Filename: github.String(".circleci/anotherconfig.yml"),
				},
				{
					Filename: github.String(".docker/config.json"),
				},
				{
					Filename: github.String(".circleci/validfile.test"),
				},
				{
					Filename: github.String("build/honk.fake"),
				},
				{
					Filename: github.String("build/honk/honk.fake"),
				},
			},
			expectedError:        true,
			expectedMessageError: "The files `.circleci/config.yml, .circleci/anotherconfig.yml, .docker/config.json, build/honk.fake, build/honk/honk.fake` are in the blocklist and should not be modified from external contributors, please if you are part of the Mattermost Org submit this PR in the upstream.\n /cc @mattermost/core-security @mattermost/core-build-engineers",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			s := &Server{
				Config: &Config{
					Org: "mattertest",
					BlocklistPaths: []string{
						".circleci/*.yml",
						"build/Dockerfile",
						"Makefile",
						".docker/config.json",
						".dockercfg",
						".docker",
						"scripts/*.sh",
						"**/*.fake",
						"**/**/*.fake",
					},
				},
			}

			blockMessage, err := s.validateBlockPaths(tc.input)
			if tc.expectedError {
				assert.Equal(t, tc.expectedMessageError, blockMessage)
				assert.Error(t, err)
			} else {
				assert.Empty(t, blockMessage)
				assert.NoError(t, err)
			}
		})
	}
}
