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
	"time"

	"github.com/google/go-github/v32/github"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v5/mlog"
)

func (s *Server) handleCheckCLA(ctx context.Context, pr *model.PullRequest) error {
	if pr.State == model.StateClosed {
		return nil
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
		return s.createRepoStatus(ctx, pr, status)
	}

	body, err := s.getCSV(ctx)
	if err != nil {
		return err
	}

	if !isNameInCLAList(strings.Split(string(body), "\n"), username) {
		comments, err := s.getComments(ctx, pr.RepoOwner, pr.RepoName, pr.Number)
		if err != nil {
			return fmt.Errorf("failed fetching comments: %w", err)
		}
		_, found := findNeedsToSignCLAComment(comments, s.Config.Username)
		if !found {
			go func() {
				ctx2, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout*time.Second)
				defer cancel()
				if err = s.sendGitHubComment(ctx2, pr.RepoOwner, pr.RepoName, pr.Number, strings.Replace(s.Config.NeedsToSignCLAMessage, "USERNAME", "@"+username, 1)); err != nil {
					mlog.Warn("Error while commenting", mlog.Err(err))
				}
			}()
		}
		status := &github.RepoStatus{
			State:       github.String(stateError),
			Description: github.String(fmt.Sprintf("%v needs to sign the CLA", username)),
			TargetURL:   github.String(s.Config.SignedCLAURL),
			Context:     github.String(s.Config.CLAGithubStatusContext),
		}
		mlog.Debug("will post error on CLA", mlog.String("user", username))
		return s.createRepoStatus(ctx, pr, status)
	}

	status := &github.RepoStatus{
		State:       github.String(stateSuccess),
		Description: github.String(fmt.Sprintf("%s authorized", username)),
		TargetURL:   github.String(s.Config.SignedCLAURL),
		Context:     github.String(s.Config.CLAGithubStatusContext),
	}
	mlog.Debug("will post success on CLA", mlog.String("user", username))
	return s.createRepoStatus(ctx, pr, status)
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

func findNeedsToSignCLAComment(comments []*github.IssueComment, username string) (id int64, found bool) {
	for _, comment := range comments {
		if *comment.User.Login == username && strings.Contains(*comment.Body, "Please help complete the Mattermost") {
			return *comment.ID, true
		}
	}
	return 0, false
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
