// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v5/mlog"
	"github.com/pkg/errors"
)

const (
	goimportsLocalScheduledMsg = "goimports -local is scheduled."
)

func (s *Server) doGoImportsLocal(ctx context.Context, pr *model.PullRequest) (cmdOutput string, err error) {
	if s.Config.RepoFolder == "" {
		return "", errors.Errorf("path to folder containing local checkout of repositories is not set in the config")
	}
	repoFolder := filepath.Join(s.Config.RepoFolder, pr.RepoName)
	if _, err = os.Stat(repoFolder); os.IsNotExist(err) {
		err = cloneRepo(ctx, s.Config, pr.RepoName)
		if err != nil {
			return "", fmt.Errorf("error while cloning repo: %s, %v", s.Config.Org+"/"+pr.RepoName, err)
		}
	}
	cmd := exec.Command("goimports", "-local=github.com/mattermost/mattermost-server/v5")
	cmd.Dir = repoFolder
	cmd.Env = append(
		os.Environ(),
		os.Getenv("PATH"),
		fmt.Sprintf("ORIGINAL_AUTHOR=%s", pr.Username),
		fmt.Sprintf("GITHUB_USER=%s", s.Config.GithubUsername),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		mlog.Error("cmd.Run() failed",
			mlog.Err(err),
			mlog.String("cmdOut", string(out)),
			mlog.String("repo", pr.RepoName),
			mlog.Int("PR", pr.Number),
		)
		err2 := returnToMaster(ctx, repoFolder)
		if err2 != nil {
			mlog.Error("Failed to return to master", mlog.Err(err2))
		}
		return string(out), err
	}

	return string(out), nil
}
