// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/go-github/v43/github"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v5/mlog"
)

// handleCheckCLA checks if the author of a pull request has signed the CLA and sets a status accordingly.
// Returns true, if the user hasn't signed yet.
func (s *Server) handleCheckCLA(ctx context.Context, pr *model.PullRequest) (bool, error) {
	if pr.State == model.StateClosed {
		return false, nil
	}

	s.createCLAPendingStatus(ctx, pr)

	username := pr.Username
	mlog.Info(
		"Will check the CLA for user",
		mlog.String("user", username),
		mlog.String("repo", pr.RepoOwner),
		mlog.String("reponame", pr.RepoName),
		mlog.Int("pr number", pr.Number),
	)

	if s.IsBotUserFromCLAExclusionsList(username) {
		status := &github.RepoStatus{
			State:       github.String(stateSuccess),
			Description: github.String(fmt.Sprintf("%s excluded", username)),
			TargetURL:   github.String(s.Config.SignedCLAURL),
			Context:     github.String(s.Config.CLAGithubStatusContext),
		}
		mlog.Debug("will succeed CLA status for excluded user", mlog.String("user", username))
		return false, s.createRepoStatus(ctx, pr, status)
	}

	body, err := s.getCSV(ctx)
	if err != nil {
		return false, nil
	}

	if !isNameInCLAList(strings.Split(string(body), "\n"), username) {
		status := &github.RepoStatus{
			State:       github.String(stateError),
			Description: github.String(fmt.Sprintf("%v needs to sign the CLA", username)),
			TargetURL:   github.String(s.Config.SignedCLAURL),
			Context:     github.String(s.Config.CLAGithubStatusContext),
		}
		mlog.Debug("will post error on CLA", mlog.String("user", username))
		return true, s.createRepoStatus(ctx, pr, status)
	}

	status := &github.RepoStatus{
		State:       github.String(stateSuccess),
		Description: github.String(fmt.Sprintf("%s authorized", username)),
		TargetURL:   github.String(s.Config.SignedCLAURL),
		Context:     github.String(s.Config.CLAGithubStatusContext),
	}
	mlog.Debug("will post success on CLA", mlog.String("user", username))
	return false, s.createRepoStatus(ctx, pr, status)
}

func (s *Server) getCSV(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.Config.SignedCLAURL, http.NoBody)
	if err != nil {
		return nil, err
	}
	r, err := http.DefaultClient.Do(req) //nolint
	if err != nil {
		s.logToMattermost(ctx, "unable to get CLA google csv file Error: ```"+err.Error()+"```")
		return nil, err
	}
	defer closeBody(r)

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		s.logToMattermost(ctx, "unable to read CLA google csv file Error: ```"+err.Error()+"```")
		return nil, err
	}
	return body, nil
}

func isNameInCLAList(usersWhoSignedCLA []string, authorToTrim string) bool {
	for _, userToTrim := range usersWhoSignedCLA {
		user := strings.ToLower(strings.TrimSpace(userToTrim))
		author := strings.ToLower(authorToTrim)
		if user == author {
			return true
		}
	}
	return false
}

func (s *Server) createCLAPendingStatus(ctx context.Context, pr *model.PullRequest) {
	status := &github.RepoStatus{
		State:       github.String(statePending),
		Description: github.String("Checking if " + pr.Username + " signed CLA"),
		TargetURL:   github.String(s.Config.SignedCLAURL),
		Context:     github.String(s.Config.CLAGithubStatusContext),
	}
	err := s.createRepoStatus(ctx, pr, status)
	if err != nil {
		s.logToMattermost(ctx, "failed to create status for PR: "+strconv.Itoa(pr.Number)+" Context: "+s.Config.CLAGithubStatusContext+" Error: ```"+err.Error()+"```")
	}
}
