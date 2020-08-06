package server

import (
	"errors"
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
		name          string
		input         []*github.CommitFile
		expectedFiles []string
	}{
		{
			name: "file is in the block list",
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
			expectedFiles: []string{".circleci/config.yml"},
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
			expectedFiles: []string{},
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
			expectedFiles: []string{".circleci/config.yml", ".circleci/anotherconfig.yml", ".docker/config.json", "build/honk.fake", "build/honk/honk.fake"},
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

			err := s.validateBlockPaths(tc.input)
			if len(tc.expectedFiles) > 0 {
				var blockError *BlockPathValidationError
				assert.True(t, errors.As(err, &blockError))
				assert.Len(t, blockError.BlockListFiles(), len(tc.expectedFiles))
				assert.Equal(t, tc.expectedFiles, blockError.BlockListFiles())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
