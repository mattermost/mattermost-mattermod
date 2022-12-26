// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"text/template"
	"time"

	"github.com/google/go-github/v39/github"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v6/shared/mlog"
)

const (
	cloudTagName = "tags/cloud"
	dryRunFlag   = "--dry-run"
	forceFlag    = "--force"
)

type cloudTagResult struct {
	Tagged  map[string]string
	Skipped []string
	DryRun  bool
}

// HasCloudTag is true if body contains "/cloud-tag"
func (e *issueCommentEvent) HasCloudTag() bool {
	return strings.Contains(strings.TrimSpace(e.Comment.GetBody()), "/cloud-tag")
}

func (s *Server) createCloudTag(ctx context.Context, issue *model.Issue, comment, user string) (*cloudTagResult, error) {
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

	result := &cloudTagResult{
		Tagged: map[string]string{},
	}

	// Check if the args are correct
	command := getCloudTagCommand(comment)
	args := strings.Split(command, " ")
	mlog.Info("Args", mlog.String("Args", comment))
	var force, dryRun bool
	if len(args) >= 2 {
		force = args[1] == forceFlag
		dryRun = args[1] == dryRunFlag
		result.DryRun = dryRun
	}

	tagName := cloudTagName + "-" + time.Now().Format("2006-01-02") + "-1"

	for _, r := range s.Config.CloudRepositories {
		repository := r.Name

		ref, _, err := s.GithubClient.Git.GetRef(ctx, s.Config.Org, repository, cloudBranchName)
		if err != nil {
			result.Skipped = append(result.Skipped, repository)
			mlog.Warn("Error while getting the ref", mlog.Err(err))
			continue
		}

		if !dryRun {
			if force {
				resp, err2 := s.GithubClient.Git.DeleteRef(ctx, s.Config.Org, repository, tagName)
				if err2 != nil && (resp.StatusCode >= 300 && resp.StatusCode != http.StatusUnprocessableEntity) {
					result.Skipped = append(result.Skipped, repository)
					mlog.Warn("Error while deleting the ref", mlog.Err(err2))
					continue
				}
			}

			tag, _, tErr := s.GithubClient.Git.CreateRef(ctx, s.Config.Org, repository, &github.Reference{
				Ref: NewString(tagName),
				Object: &github.GitObject{
					Type: NewString("tag"),
					SHA:  ref.Object.SHA,
				},
			})
			if tErr != nil {
				result.Skipped = append(result.Skipped, repository)
				mlog.Warn("Error while creating tag", mlog.Err(tErr))
				continue
			}
			result.Tagged[repository] = *tag.Object.SHA
		} else {
			// while running in dry run mode, we will just return the SHA of the cloud branch
			result.Tagged[repository] = *ref.Object.SHA
		}
	}

	return result, nil
}

func getCloudTagCommand(command string) string {
	index := strings.Index(command, "/cloud-tag")
	return command[index:]
}

func executeCloudTagSummary(res *cloudTagResult) (string, error) {
	tmpl, err := template.New("cloudTag").Parse(cloudTagTmpl)
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

var cloudTagTmpl = `
## Summary for the cloud tag process{{ if (.DryRun) }} (dry run){{ end }}
{{ if gt (len .Tagged) 0 }}### Tagged repositories{{ end }}
{{range $index, $element := .Tagged}}    - mattermost/{{$index}} ({{$element}}  -> cloud-2022-10-11-1)
{{end}}
{{ if gt (len .Skipped) 0 }}### Skipped repositories
Could not create the tag for following (a tag may already exist or cloud branch could not be found):{{ end }}
{{range .Skipped}}    - {{.}}
{{end}}
{{ if gt (len .Tagged) 0 }}Successfully tagged {{(len .Tagged)}} repositories. {{end}}
`
