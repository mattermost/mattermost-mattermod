// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/google/go-github/v39/github"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v5/mlog"
)

const (
	cloudBranchName        = "heads/cloud"
	mainBranchName         = "heads/master"
	processRepo            = "mattermost-server"
	minBackupIntervalHours = 5 * 24
)

var (
	backupRegex = regexp.MustCompile(`cloud-20\d{2}-\d{2}-\d{2}-backup`)
)

type fastForwardResult struct {
	Backup        []string
	Skipped       []string
	FastForwarded []string
	DryRun        bool
}

// HasCloudFF is true if body contains "/cloud-ff"
func (e *issueCommentEvent) HasCloudFF() bool {
	return strings.Contains(strings.TrimSpace(e.Comment.GetBody()), "/cloud-ff")
}

func (s *Server) performFastForwardProcess(ctx context.Context, issue *model.Issue, comment, user string) (*fastForwardResult, error) {
	if issue.State == model.StateClosed || issue.RepoName != processRepo { // we will perform this only on mm-server issues
		return nil, nil
	}

	// Don't start process if the user is not a core committer
	if !s.IsOrgMember(user) {
		return nil, nil
	}

	// We are using a mutex for multi-deployment setups
	mx := s.Store.Mutex()
	if err := mx.Lock(ctx); err != nil {
		return nil, err
	}
	defer func() {
		if err := mx.Unlock(); err != nil {
			mlog.Error("unable to release lock", mlog.Err(err))
		}
	}()

	result := &fastForwardResult{}

	// Check if the args are correct
	command := getFFCommand(comment)
	args := strings.Split(command, " ")
	mlog.Info("Args", mlog.String("Args", comment))
	var force, dryRun bool
	if len(args) >= 2 {
		force = args[1] == "--force"
		dryRun = args[1] == "--dry-run"
		result.DryRun = dryRun
	}

	backupBranchName := cloudBranchName + "-" + time.Now().Format("2006-01-02") + "-backup"

	for _, repo := range s.Config.CloudRepositories {
		refs, _, err := s.GithubClient.Git.ListMatchingRefs(ctx, s.Config.Org, repo, &github.ReferenceListOptions{
			Ref: cloudBranchName,
		})
		if err != nil {
			return nil, err
		}

		var latestBackup string
		var latestBackupSHA string

		// we are looking for backup branches where has the backup naming patterns
		// and find the most recent backup branch
		for _, ref := range refs {
			refName := *ref.Ref
			if backupRegex.MatchString(refName) && refName > latestBackup {
				latestBackup = refName
				latestBackupSHA = *ref.Object.SHA
			}
		}

		// we check if backup branch do exist
		// if so, we check the commit day whether if it is older than 5 days or not.
		// if it's a recent backup, we assume that the fast forward process has been completed
		// and skip for this repository
		if latestBackup != "" && latestBackupSHA != "" {
			commit, _, err2 := s.GithubClient.Git.GetCommit(ctx, s.Config.Org, repo, latestBackupSHA)
			if err2 != nil {
				return nil, err2
			}
			if !force && time.Now().Before(commit.Author.Date.Add(minBackupIntervalHours*time.Hour)) {
				result.Skipped = append(result.Skipped, repo)
				continue
			}
		}

		// we get the cloud branch
		ref, _, err := s.GithubClient.Git.GetRef(ctx, s.Config.Org, repo, cloudBranchName)
		if err != nil {
			mlog.Warn("error getting reference", mlog.Err(err), mlog.Int("issue", issue.Number), mlog.String("Repo", repo), mlog.String("Ref", cloudBranchName))
			// We don't return here as cloud branch may not exist anyway
		}

		if ref != nil {
			// So the cloud branch exists, we try to have a backup, just in case
			// we have problems after we delete the current cloud branch
			newRef := &github.Reference{
				Ref:    github.String(backupBranchName),
				Object: ref.Object,
			}

			var resp *github.Response
			var backupRef *github.Reference

			if dryRun {
				result.Backup = append(result.Backup, fmt.Sprintf("%s:%s (SHA: `%s`)", repo, *newRef.Ref, (*newRef.Object.SHA)[:7]))
			} else {
				backupRef, resp, err = s.GithubClient.Git.CreateRef(ctx, s.Config.Org, repo, newRef)
				if err != nil {
					if resp.StatusCode != http.StatusUnprocessableEntity {
						return nil, err
					}
					// if force flag provided, we are going to delete the backup and create again.
					if force {
						_, err = s.GithubClient.Git.DeleteRef(ctx, s.Config.Org, repo, *newRef.Ref)
						if err != nil {
							mlog.Warn("error deleting reference", mlog.Err(err), mlog.Int("issue", issue.Number), mlog.String("Repo", repo), mlog.String("Ref", cloudBranchName))
							// We don't return here as cloud branch may not exist anyway.
							// Even if it exists, we are going to fail on creating the new cloud branch.
						}
						backupRef, _, err = s.GithubClient.Git.CreateRef(ctx, s.Config.Org, repo, newRef)
						if err != nil {
							return nil, err
						}
						result.Backup = append(result.Backup, fmt.Sprintf("%s:%s (SHA: `%s`)", repo, *backupRef.Ref, (*backupRef.Object.SHA)[:7]))
					} else {
						// backup exist, continue anyway but comment this.
						result.Skipped = append(result.Skipped, repo)
					}
				} else {
					result.Backup = append(result.Backup, fmt.Sprintf("%s:%s (SHA: `%s`)", repo, *backupRef.Ref, (*backupRef.Object.SHA)[:7]))
				}
			}
		}

		// so far we should've back up the cloud branch, it should be safe to delete
		// if we didn't take a backup, it should be the only case of cloud branch is not existing (yet).
		if !dryRun {
			_, err = s.GithubClient.Git.DeleteRef(ctx, s.Config.Org, repo, cloudBranchName)
			if err != nil {
				mlog.Warn("error deleting reference", mlog.Err(err), mlog.Int("issue", issue.Number), mlog.String("Repo", repo), mlog.String("Ref", cloudBranchName))
				// We don't return here as cloud branch may not exist anyway.
				// Even if it exists, we are going to fail on creating the new cloud branch.
			}
		}

		// get main branch
		refHead, _, err := s.GithubClient.Git.GetRef(ctx, s.Config.Org, repo, mainBranchName)
		if err != nil {
			return nil, err
		}

		// create cloud branch from main branch
		newHeadRef := &github.Reference{
			Ref:    github.String(cloudBranchName),
			Object: refHead.Object,
		}

		if !dryRun {
			_, _, err = s.GithubClient.Git.CreateRef(ctx, s.Config.Org, repo, newHeadRef)
			if err != nil {
				return nil, err
			}
		}

		// so far we completed the fast forward process for this iteration, let's report it proudly.
		result.FastForwarded = append(result.FastForwarded, repo)
	}

	return result, nil
}

func getFFCommand(command string) string {
	index := strings.Index(command, "/cloud-ff")
	return command[index:]
}

func executeFFSummary(res *fastForwardResult) (string, error) {
	tmpl, err := template.New("ff").Parse(fftmpl)
	if err != nil {
		return "", err
	}

	buf := bytes.NewBuffer(make([]byte, 0))
	err = tmpl.Execute(buf, res)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

var fftmpl = `
## Summary for the fast forward process{{ if (.DryRun) }} (dry run){{ end }}
{{ if gt (len .Backup) 0 }}### Backup branches{{ end }}
{{range .Backup}}    - {{.}}
{{end}}
{{ if gt (len .Skipped) 0 }}### Skipped repositories
Could not create the backup branch for following (a backup may already exist):{{ end }}
{{range .Skipped}}    - {{.}}
{{end}}
{{ if gt (len .FastForwarded) 0 }}### Fast-Forwarded branches{{ end }}
{{range .FastForwarded}}    - {{.}}
{{end}}
{{ if gt (len .FastForwarded) 0 }} Successfully fast-forwarded {{(len .FastForwarded)}} repositories. {{end}}
`
