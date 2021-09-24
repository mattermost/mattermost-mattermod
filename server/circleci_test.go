package server

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/mattermost/mattermost-mattermod/server/mocks"

	"github.com/golang/mock/gomock"
	"github.com/google/go-github/v33/github"
	"github.com/mattermost/go-circleci"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		repo          string
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
		{
			name: "Several files the blocklist, some block by repo specific list",
			repo: "mattermost-server",
			input: []*github.CommitFile{
				{
					Filename: github.String("foo1"),
				},
				{
					Filename: github.String("bar1"),
				},
				{
					Filename: github.String("another"),
				},
				{
					Filename: github.String("Makefile"),
				},
			},
			expectedFiles: []string{"foo1", "bar1", "Makefile"},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			s := &Server{
				Config: &Config{
					Org: "mattertest",
					BlockListPathsGlobal: []string{
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
					BlockListPathsPerRepo: map[string][]string{
						"mattermost-server": {"foo1", "foo2", "bar1", "bar2"},
					},
				},
			}

			err := s.validateBlockPaths(tc.repo, tc.input)
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

func TestWaitForWorkflowID(t *testing.T) {
	ctrl := gomock.NewController(t)
	ctxInterface := reflect.TypeOf((*context.Context)(nil)).Elem()
	wfList1 := &circleci.WorkflowList{
		Items: []circleci.WorkflowItem{
			{
				ID:          "id",
				CanceledBy:  "someone",
				WorkflowID:  "foo",
				Name:        "CI",
				ProjectSlug: "github/mattermost/mattermod",
			},
		},
		NextPageToken: "token",
	}
	wfList2 := &circleci.WorkflowList{
		Items: []circleci.WorkflowItem{
			{
				ID:          "targetID",
				CanceledBy:  "someone",
				WorkflowID:  "foo",
				Name:        "targetName",
				ProjectSlug: "github/mattermost/mattermod",
			},
		},
		NextPageToken: "token2",
	}
	wfList3 := &circleci.WorkflowList{
		Items: []circleci.WorkflowItem{
			{
				ID:          "pipelineID",
				WorkflowID:  "foo",
				Name:        "targetName",
				ProjectSlug: "github/mattermost/mattermod",
			},
		},
		NextPageToken: "",
	}
	circleCIService := mocks.NewMockCircleCIService(ctrl)
	circleCIService.EXPECT().GetPipelineWorkflowWithContext(gomock.AssignableToTypeOf(ctxInterface), "pipelineID", "").Return(wfList1, nil)
	circleCIService.EXPECT().GetPipelineWorkflowWithContext(gomock.AssignableToTypeOf(ctxInterface), "pipelineID", "token").Return(wfList2, nil)
	circleCIService.EXPECT().GetPipelineWorkflowWithContext(gomock.AssignableToTypeOf(ctxInterface), "pipelineID", "token2").Return(wfList3, nil)

	s := Server{
		CircleCiClientV2: circleCIService,
	}

	res, err := s.waitForWorkflowID(context.Background(), "pipelineID", "targetName")
	require.NoError(t, err)
	assert.Equal(t, "foo", res)
}
