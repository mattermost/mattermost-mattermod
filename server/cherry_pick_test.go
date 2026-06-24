package server

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/google/go-github/v39/github"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-mattermod/server/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleCherryPick(t *testing.T) {
	ctrl := gomock.NewController(t)

	s := Server{
		Config: &Config{
			Org: "some-organization",
		},
		OrgMembers: []string{
			"org-member",
		},
		GithubClient: &GithubClient{},
	}

	pr := &model.PullRequest{
		RepoOwner: "user",
		RepoName:  "repo-name",
		Number:    123,
		Sha:       "some-sha",
		Merged:    NewBool(false),
	}

	ctxInterface := reflect.TypeOf((*context.Context)(nil)).Elem()

	msg := new(string)
	comment := &github.IssueComment{Body: msg}
	is := mocks.NewMockIssuesService(ctrl)
	is.EXPECT().CreateComment(gomock.AssignableToTypeOf(ctxInterface), pr.RepoOwner, pr.RepoName, pr.Number, comment).AnyTimes().Return(nil, nil, nil)
	s.GithubClient.Issues = is

	t.Run("should ignore for non org members", func(t *testing.T) {
		*msg = msgCommenterPermission

		err := s.handleCherryPick(context.Background(), "non-org-member", "/cherry-pick release-5.28", pr)
		require.NoError(t, err)
	})

	t.Run("should ignore not merged PRs", func(t *testing.T) {
		err := s.handleCherryPick(context.Background(), "org-member", "/cherry-pick release-5.28", pr)
		require.NoError(t, err)
	})

	t.Run("should ignore when server is closing", func(t *testing.T) {
		s.cherryPickStopChan = make(chan struct{})
		s.cherryPickRequests = make(chan *cherryPickRequest, 1)
		pr.Merged = NewBool(true)
		close(s.cherryPickStopChan)
		close(s.cherryPickRequests)

		err := s.handleCherryPick(context.Background(), "org-member", "/cherry-pick release-5.28", pr)
		require.EqualError(t, err, "server is closing")
	})

	t.Run("should fail on too many cherry pick tasks", func(t *testing.T) {
		s.cherryPickStopChan = make(chan struct{})
		s.cherryPickRequests = make(chan *cherryPickRequest, 1)
		pr.Merged = NewBool(true)

		*msg = cherryPickScheduledMsg

		err := s.handleCherryPick(context.Background(), "org-member", "/cherry-pick release-5.28", pr)
		require.NoError(t, err)

		*msg = tooManyCherryPickMsg

		err = s.handleCherryPick(context.Background(), "org-member", "/cherry-pick release-5.28", pr)
		require.EqualError(t, err, "too many requests")
	})

	t.Run("should not panic on empty requests", func(t *testing.T) {
		require.NotPanics(t, func() {
			err := s.handleCherryPick(context.Background(), "org-member", "/cherry-pick", pr)
			require.NoError(t, err)
		})
	})
}

func TestDoCherryPickPassesSourceBranch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tempDir := t.TempDir()
	repoName := "repo-name"
	repoFolder := filepath.Join(tempDir, "repos")
	require.NoError(t, os.MkdirAll(filepath.Join(repoFolder, repoName), 0755))
	scriptsFolder := filepath.Join(tempDir, "scripts")
	require.NoError(t, os.MkdirAll(scriptsFolder, 0755))

	argsFile := filepath.Join(tempDir, "args")
	t.Setenv("CHERRY_PICK_ARGS_FILE", argsFile)
	script := `#!/usr/bin/env bash
printf "%s\n" "$@" > "$CHERRY_PICK_ARGS_FILE"
echo "https://github.com/mattermost/repo-name/pull/456"
`
	// #nosec G306 -- the test script must be executable because doCherryPick runs it directly.
	require.NoError(t, os.WriteFile(filepath.Join(scriptsFolder, "cherry-pick.sh"), []byte(script), 0755))

	s := Server{
		Config: &Config{
			RepoFolder:    repoFolder,
			ScriptsFolder: scriptsFolder,
		},
		OrgMembers: []string{
			"org-member",
		},
		GithubClient: &GithubClient{},
	}

	pr := &model.PullRequest{
		RepoOwner:      "mattermost",
		RepoName:       repoName,
		Number:         123,
		Username:       "org-member",
		MergeCommitSHA: "merge-sha",
		Ref:            "feature/shared-branch",
	}

	ctxInterface := reflect.TypeOf((*context.Context)(nil)).Elem()
	is := mocks.NewMockIssuesService(ctrl)
	is.EXPECT().AddLabelsToIssue(gomock.AssignableToTypeOf(ctxInterface), pr.RepoOwner, pr.RepoName, 456, []string{"AutomatedCherryPick", "Changelog/Not Needed", "Docs/Not Needed"}).Return(nil, nil, nil)
	is.EXPECT().AddLabelsToIssue(gomock.AssignableToTypeOf(ctxInterface), pr.RepoOwner, pr.RepoName, pr.Number, []string{"CherryPick/Done"}).Return(nil, nil, nil)
	is.EXPECT().RemoveLabelForIssue(gomock.AssignableToTypeOf(ctxInterface), pr.RepoOwner, pr.RepoName, pr.Number, "CherryPick/Approved").Return(nil, nil)
	is.EXPECT().AddAssignees(gomock.AssignableToTypeOf(ctxInterface), pr.RepoOwner, pr.RepoName, 456, []string{"org-member"}).Return(nil, nil, nil)
	s.GithubClient.Issues = is

	prs := mocks.NewMockPullRequestsService(ctrl)
	prs.EXPECT().RequestReviewers(gomock.AssignableToTypeOf(ctxInterface), pr.RepoOwner, pr.RepoName, 456, github.ReviewersRequest{Reviewers: []string{"org-member"}}).Return(nil, nil, nil)
	s.GithubClient.PullRequests = prs

	cmdOutput, err := s.doCherryPick(context.Background(), "release-5.28", nil, pr)
	require.NoError(t, err)
	require.Empty(t, cmdOutput)

	args, err := os.ReadFile(argsFile)
	require.NoError(t, err)
	assert.Equal(t, "upstream/release-5.28\n123\nmerge-sha\nfeature/shared-branch\n", string(args))
}

func TestCherryPickScriptNamesBranchFromSourceAndTarget(t *testing.T) {
	tempDir := t.TempDir()
	binDir := filepath.Join(tempDir, "bin")
	require.NoError(t, os.MkdirAll(binDir, 0700))
	repoRoot := filepath.Join(tempDir, "repo")
	require.NoError(t, os.MkdirAll(repoRoot, 0700))

	pushArgsFile := filepath.Join(tempDir, "push-args")
	t.Setenv("FAKE_REPO_ROOT", repoRoot)
	t.Setenv("GIT_PUSH_ARGS_FILE", pushArgsFile)
	t.Setenv("GITHUB_USER", "mattermod")
	t.Setenv("ORIGINAL_AUTHOR", "author")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	fakeGit := `#!/usr/bin/env bash
set -e

case "$1" in
  rev-parse)
    echo "$FAKE_REPO_ROOT"
    ;;
  symbolic-ref)
    echo "master"
    ;;
  remote)
    if [[ "$2" == "get-url" ]]; then
      echo "git@github.com:mattermost/repo-name.git"
    fi
    ;;
  log)
    echo "merge-sha"
    ;;
  push)
    printf "%s\n" "$@" > "$GIT_PUSH_ARGS_FILE"
    ;;
esac
`
	writeExecutableTestFile(t, filepath.Join(binDir, "git"), fakeGit)
	writeExecutableTestFile(t, filepath.Join(binDir, "hub"), "#!/usr/bin/env bash\nexit 0\n")

	cmd := exec.Command(
		"bash",
		filepath.Join("..", "hack", "cherry-pick.sh"),
		"upstream/release-11.7",
		"123",
		"merge-sha",
		"my-branch",
	)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))

	pushArgs, err := os.ReadFile(pushArgsFile)
	require.NoError(t, err)

	args := strings.Split(strings.TrimSpace(string(pushArgs)), "\n")
	require.Len(t, args, 3)
	branchParts := strings.SplitN(args[2], ":", 2)
	require.Len(t, branchParts, 2)
	assert.True(t, strings.HasPrefix(branchParts[0], "my-branch-release-11.7-"))
	assert.Equal(t, "my-branch-release-11.7", branchParts[1])
}

func TestCherryPickScriptRejectsEmptySourceBranch(t *testing.T) {
	tempDir := t.TempDir()
	binDir := filepath.Join(tempDir, "bin")
	require.NoError(t, os.MkdirAll(binDir, 0700))
	repoRoot := filepath.Join(tempDir, "repo")
	require.NoError(t, os.MkdirAll(repoRoot, 0700))

	t.Setenv("FAKE_REPO_ROOT", repoRoot)
	t.Setenv("GITHUB_USER", "mattermod")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	fakeGit := `#!/usr/bin/env bash
set -e

case "$1" in
  rev-parse)
    echo "$FAKE_REPO_ROOT"
    ;;
  symbolic-ref)
    echo "master"
    ;;
esac
`
	writeExecutableTestFile(t, filepath.Join(binDir, "git"), fakeGit)
	writeExecutableTestFile(t, filepath.Join(binDir, "hub"), "#!/usr/bin/env bash\nexit 0\n")

	cmd := exec.Command(
		"bash",
		filepath.Join("..", "hack", "cherry-pick.sh"),
		"upstream/release-11.7",
		"123",
		"merge-sha",
		"",
	)
	out, err := cmd.CombinedOutput()
	require.Error(t, err)
	assert.Contains(t, string(out), "error: source branch (argument 4) must not be empty")
}

func writeExecutableTestFile(t *testing.T, path string, content string) {
	t.Helper()

	require.NoError(t, os.WriteFile(path, []byte(content), 0600))
	// #nosec G302 -- these test command shims must be executable to appear in PATH.
	require.NoError(t, os.Chmod(path, 0700))
}

func TestGetMilestone(t *testing.T) {
	title := "v5.20.0"
	milestone := getMilestone(title)
	assert.Equal(t, "release-5.20", milestone)

	title = "v5.1.0"
	milestone = getMilestone(title)
	assert.Equal(t, "release-5.1", milestone)
}

func TestGetCommand(t *testing.T) {
	raw := "PR looks good to go. /cherry-pick release-5.28"
	command := getCommand(raw)
	assert.Equal(t, "/cherry-pick release-5.28", command)

	command = getCommand(command)
	assert.Equal(t, "/cherry-pick release-5.28", command)
}
