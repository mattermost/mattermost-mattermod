// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/go-github/v39/github"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v5/mlog"
)

const (
	cloudBranchName = "heads/cloud"
	mainBranchName  = "heads/master"
	processRepo     = "mattermost-server"
)

// HasCloudFF is true if body contains "/cloud-ff"
func (e *issueCommentEvent) HasCloudFF() bool {
	return strings.Contains(strings.TrimSpace(e.Comment.GetBody()), "/cloud-ff")
}

func (s *Server) performFastForwardProcess(ctx context.Context, issue *model.Issue, comment, user string) error {
	if issue.State == model.StateClosed || issue.RepoName != processRepo { // we will perform this only on mm-server issues
		return nil
	}

	// Don't start process if the user is not a core committer
	if !s.IsOrgMember(user) {
		return nil
	}

	// Check if the args are correct
	command := getFFCommand(comment)
	args := strings.Split(command, " ")
	mlog.Info("Args", mlog.String("Args", comment))
	if len(args) < 2 {
		_, _, err := s.GithubClient.Issues.CreateComment(ctx, issue.RepoOwner, issue.RepoName, issue.Number, &github.IssueComment{
			Body: github.String("Invalid number of args, it should contain a backup name. It can be like as following:\n```\n/cloud-ff 2022-04-07\n```\n"),
		})
		if err != nil {
			return err
		}
		return nil
	}

	backupBranchName := cloudBranchName + "-" + args[1] + "-backup"

	for _, repo := range s.Config.CloudRepositories {
		ref, _, err := s.GithubClient.Git.GetRef(ctx, s.Config.Org, repo, cloudBranchName)
		if err != nil {
			mlog.Warn("error getting reference", mlog.Err(err), mlog.Int("issue", issue.Number), mlog.String("Repo", repo), mlog.String("Ref", cloudBranchName))
			// We don't return here as cloud branch may not exist anyway
		}

		// So the cloud branch exists, we try to have a backup, just in case
		// we have problems after we delete the current cloud branch
		if ref != nil {
			newRef := &github.Reference{
				Ref:    github.String(backupBranchName),
				Object: ref.Object,
			}

			var resp *github.Response
			_, resp, err = s.GithubClient.Git.CreateRef(ctx, s.Config.Org, repo, newRef)
			if err != nil {
				mlog.Warn("error creating reference", mlog.Err(err), mlog.Int("issue", issue.Number), mlog.String("Repo", repo), mlog.String("Ref", backupBranchName))
				if resp.StatusCode != http.StatusUnprocessableEntity {
					return err
				}
				// backup exist, continue anyway but comment this.
				_, _, err = s.GithubClient.Issues.CreateComment(ctx, issue.RepoOwner, issue.RepoName, issue.Number, &github.IssueComment{
					Body: github.String(fmt.Sprintf("Could not create the backup branch, it may already exist. Skipping backup for %s", repo)),
				})
				if err != nil {
					mlog.Warn("error creating comment", mlog.Err(err), mlog.Int("issue", issue.Number), mlog.String("Repo", issue.RepoName))
				}
			}
		}

		_, err = s.GithubClient.Git.DeleteRef(ctx, s.Config.Org, repo, cloudBranchName)
		if err != nil {
			mlog.Warn("error deleting reference", mlog.Err(err), mlog.Int("issue", issue.Number), mlog.String("Repo", repo), mlog.String("Ref", cloudBranchName))
			// We don't return here as cloud branch may not exist anyway.
			// Even if it exists, we are going to fail on creating the new cloud branch.
		}

		refHead, _, err := s.GithubClient.Git.GetRef(ctx, s.Config.Org, repo, mainBranchName)
		if err != nil {
			return err
		}

		newHeadRef := &github.Reference{
			Ref:    github.String(cloudBranchName),
			Object: refHead.Object,
		}

		_, _, err = s.GithubClient.Git.CreateRef(ctx, s.Config.Org, repo, newHeadRef)
		if err != nil {
			return err
		}

		// so far we completed the fast forward process for this iteration, let's report it proudly.
		_, _, err = s.GithubClient.Issues.CreateComment(ctx, issue.RepoOwner, issue.RepoName, issue.Number, &github.IssueComment{
			Body: github.String(fmt.Sprintf("Successfully fast-forwarded `%s` branch for `%s`.", strings.TrimPrefix(cloudBranchName, "heads/"), repo)),
		})
		if err != nil {
			mlog.Warn("error creating comment", mlog.Err(err), mlog.Int("issue", issue.Number), mlog.String("Repo", issue.RepoName))
		}
	}

	return nil
}

func getFFCommand(command string) string {
	index := strings.Index(command, "/cloud-ff")
	return command[index:]
}
