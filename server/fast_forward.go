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
	"github.com/mattermost/mattermost-server/v6/shared/mlog"
)

const (
	cloudBranchName        = "heads/cloud"
	defaultMainBranchName  = "heads/master"
	processRepo            = "mattermost"
	minBackupIntervalHours = 5 * 24
)

var (
	backupRegex = regexp.MustCompile(`cloud-(\d{4})-(\d{2})-(\d{2})-backup`)
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
	if !s.IsOrgMember(user) || s.IsInBotBlockList(user) {
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

	for _, r := range s.Config.CloudRepositories {
		repository := r.Name
		mainBranchName := r.MainBranch
		if mainBranchName == "" {
			mainBranchName = defaultMainBranchName
		}

		refs, _, err := s.GithubClient.Git.ListMatchingRefs(ctx, s.Config.Org, repository, &github.ReferenceListOptions{
			Ref: cloudBranchName,
		})
		if err != nil {
			return nil, err
		}

		var latestBackup string

		// we are looking for backup branches where has the backup naming patterns
		// and find the most recent backup branch
		for _, ref := range refs {
			refName := *ref.Ref
			if backupRegex.MatchString(refName) && refName > latestBackup {
				latestBackup = refName
			}
		}

		// we check if backup branch do exist
		// if so, we check the backup date whether if it is older than 5 days or not by parsing the branch name.
		// if it's a recent backup, we assume that the fast forward process has been completed
		// and skip for this repository
		if latestBackup != "" {
			submatch := backupRegex.FindStringSubmatch(latestBackup)
			if len(submatch) != 4 {
				return nil, fmt.Errorf("could not match date from branch (%q) with the regex", latestBackup)
			}

			refDate, err2 := time.Parse("2006-01-02", fmt.Sprintf("%s-%s-%s", submatch[1], submatch[2], submatch[3]))
			if err2 != nil {
				return nil, err2
			}

			if !force && time.Now().Before(refDate.Add(minBackupIntervalHours*time.Hour)) {
				result.Skipped = append(result.Skipped, repository)
				continue
			}
		}

		// we get the cloud branch
		ref, _, err := s.GithubClient.Git.GetRef(ctx, s.Config.Org, repository, cloudBranchName)
		if err != nil {
			mlog.Warn("error getting reference", mlog.Err(err), mlog.Int("issue", issue.Number), mlog.String("Repo", repository), mlog.String("Ref", cloudBranchName))
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
				result.Backup = append(result.Backup, fmt.Sprintf("%s:%s (SHA: `%s`)", repository, *newRef.Ref, (*newRef.Object.SHA)[:7]))
			} else {
				backupRef, resp, err = s.GithubClient.Git.CreateRef(ctx, s.Config.Org, repository, newRef)
				if err != nil {
					if resp.StatusCode != http.StatusUnprocessableEntity {
						return nil, err
					}
					// if force flag provided, we are going to delete the backup and create again.
					if force {
						_, err = s.GithubClient.Git.DeleteRef(ctx, s.Config.Org, repository, *newRef.Ref)
						if err != nil {
							mlog.Warn("error deleting reference", mlog.Err(err), mlog.Int("issue", issue.Number), mlog.String("Repo", repository), mlog.String("Ref", cloudBranchName))
							// We don't return here as cloud branch may not exist anyway.
							// Even if it exists, we are going to fail on creating the new cloud branch.
						}
						backupRef, _, err = s.GithubClient.Git.CreateRef(ctx, s.Config.Org, repository, newRef)
						if err != nil {
							return nil, err
						}
						result.Backup = append(result.Backup, fmt.Sprintf("%s:%s (SHA: `%s`)", repository, *backupRef.Ref, (*backupRef.Object.SHA)[:7]))
					} else {
						// backup exist, continue anyway but comment this.
						result.Skipped = append(result.Skipped, repository)
					}
				} else {
					result.Backup = append(result.Backup, fmt.Sprintf("%s:%s (SHA: `%s`)", repository, *backupRef.Ref, (*backupRef.Object.SHA)[:7]))
				}
			}
		}

		// so far we should've back up the cloud branch, it should be safe to delete
		// if we didn't take a backup, it should be the only case of cloud branch is not existing (yet).
		if !dryRun {
			_, err = s.GithubClient.Git.DeleteRef(ctx, s.Config.Org, repository, cloudBranchName)
			if err != nil {
				mlog.Warn("error deleting reference", mlog.Err(err), mlog.Int("issue", issue.Number), mlog.String("Repo", repository), mlog.String("Ref", cloudBranchName))
				// We don't return here as cloud branch may not exist anyway.
				// Even if it exists, we are going to fail on creating the new cloud branch.
			}
		}

		// get main branch
		refHead, _, err := s.GithubClient.Git.GetRef(ctx, s.Config.Org, repository, mainBranchName)
		if err != nil {
			return nil, err
		}

		// create cloud branch from main branch
		newHeadRef := &github.Reference{
			Ref:    github.String(cloudBranchName),
			Object: refHead.Object,
		}

		if !dryRun {
			_, _, err = s.GithubClient.Git.CreateRef(ctx, s.Config.Org, repository, newHeadRef)
			if err != nil {
				return nil, err
			}
		}

		// so far we completed the fast forward process for this iteration, let's report it proudly.
		result.FastForwarded = append(result.FastForwarded, repository)
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

// 3 months
const cloudBranchCleanupThreshold = 24 * time.Hour * 30 * 3

func (s *Server) CleanOutdatedCloudBranches() {
	mlog.Info("Starting to clean outdated cloud branches")
	ctx, cancel := context.WithTimeout(context.Background(), defaultCronTaskTimeout*time.Second)
	defer cancel()
	branchListOpts := &github.BranchListOptions{
		ListOptions: github.ListOptions{PerPage: 50},
	}

	// calculate threshold time.
	year, month, day := time.Now().UTC().Date()
	trunc := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	// subtract threshold
	target := trunc.Add(-cloudBranchCleanupThreshold)

	// iterate repos
	for _, r := range s.Config.CloudRepositories {
		repository := r.Name
		for {
			// for each repo, get all branches
			branches, resp, err := s.GithubClient.Repositories.ListBranches(ctx, s.Config.Org, repository, branchListOpts)
			if err != nil {
				mlog.Error("Failed to get branches", mlog.Err(err))
				if resp.NextPage == 0 {
					break
				}
				branchListOpts.Page = resp.NextPage
				continue
			}

			branchListOpts.Page = resp.NextPage
			time.Sleep(200 * time.Millisecond)

			// parse cloud branch name, if older than threshold, delete them
			for _, branch := range branches {
				if backupRegex.MatchString(*branch.Name) {
					components := backupRegex.FindStringSubmatch(*branch.Name)
					if len(components) != 4 {
						mlog.Error("Could not match date from branch regex", mlog.String("name", *branch.Name))
						continue
					}

					branchDate, err := time.Parse("2006-01-02", components[1]+"-"+components[2]+"-"+components[3])
					if err != nil {
						mlog.Error("Error in parsing the date from branch name", mlog.Err(err))
						continue
					}

					if branchDate.Before(target) {
						// Delete branch
						mlog.Debug("Deleting branch", mlog.String("name", *branch.Name))
						_, err2 := s.GithubClient.Git.DeleteRef(ctx, s.Config.Org, repository, "heads/"+*branch.Name)
						if err2 != nil {
							mlog.Warn("Error deleting the branch", mlog.Err(err2), mlog.String("name", *branch.Name))
						}
					}
				}
			}
			if resp.NextPage == 0 {
				break
			}
		}
	}
}
