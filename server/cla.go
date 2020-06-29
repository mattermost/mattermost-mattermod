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

	"github.com/google/go-github/v31/github"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v5/mlog"
)

func (s *Server) handleCheckCLA(ctx context.Context, eventIssueComment IssueComment) {
	prGitHub, _, err := s.GithubClient.PullRequests.Get(ctx,
		*eventIssueComment.Repository.Owner.Login,
		*eventIssueComment.Repository.Name,
		*eventIssueComment.Issue.Number,
	)
	if err != nil {
		mlog.Error("Failed to get PR for CLA", mlog.Err(err))
		return
	}

	pr, err := s.GetPullRequestFromGithub(ctx, prGitHub)
	if err != nil {
		mlog.Error("pr_error", mlog.Err(err))
		return
	}

	if *prGitHub.State == model.StateClosed {
		return
	}

	s.checkCLA(ctx, pr)
}

func (s *Server) checkCLA(ctx context.Context, pr *model.PullRequest) {
	go s.createCLAPendingStatus(ctx, pr)
	if pr.State == model.StateClosed {
		return
	}

	username := pr.Username
	mlog.Info(
		"Will check the CLA for user",
		mlog.String("user", username),
		mlog.String("repo", pr.RepoOwner),
		mlog.String("reponame", pr.RepoName),
		mlog.Int("pr n", pr.Number),
	)

	if contains(s.Config.CLAExclusionsList, username) {
		status := &github.RepoStatus{
			State:       github.String(stateSuccess),
			Description: github.String(fmt.Sprintf("%s excluded", username)),
			TargetURL:   github.String(s.Config.SignedCLAURL),
			Context:     github.String(s.Config.CLAGithubStatusContext),
		}
		mlog.Debug("will succeed CLA status for excluded user", mlog.String("user", username))
		_ = s.createRepoStatus(ctx, pr, status)
		return
	}

	body, errCSV := s.getCSV()
	if errCSV != nil {
		return
	}

	if !isNameInCLAList(strings.Split(string(body), "\n"), username) {
		comments, err := s.getComments(ctx, pr)
		if err != nil {
			mlog.Error("failed fetching comments", mlog.Err(err))
			return
		}
		_, found := findNeedsToSignCLAComment(comments, s.Config.Username)
		if !found {
			go s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, strings.Replace(s.Config.NeedsToSignCLAMessage, "USERNAME", "@"+username, 1))
		}
		status := &github.RepoStatus{
			State:       github.String(stateError),
			Description: github.String(fmt.Sprintf("%v needs to sign the CLA", username)),
			TargetURL:   github.String(s.Config.SignedCLAURL),
			Context:     github.String(s.Config.CLAGithubStatusContext),
		}
		mlog.Debug("will post error on CLA", mlog.String("user", username))
		_ = s.createRepoStatus(ctx, pr, status)
		return
	}

	status := &github.RepoStatus{
		State:       github.String(stateSuccess),
		Description: github.String(fmt.Sprintf("%s authorized", username)),
		TargetURL:   github.String(s.Config.SignedCLAURL),
		Context:     github.String(s.Config.CLAGithubStatusContext),
	}
	mlog.Debug("will post success on CLA", mlog.String("user", username))
	_ = s.createRepoStatus(ctx, pr, status)
}

func (s *Server) getCSV() ([]byte, error) {
	resp, err := http.Get(s.Config.SignedCLAURL)
	if err != nil {
		mlog.Error("Unable to get CLA list", mlog.Err(err))
		s.logToMattermost("unable to get CLA google csv file Error: ```" + err.Error() + "```")
		return nil, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		mlog.Error("Unable to read response body", mlog.Err(err))
		s.logToMattermost("unable to read CLA google csv file Error: ```" + err.Error() + "```")
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
		s.logToMattermost("failed to create status for PR: " + strconv.Itoa(pr.Number) + " Context: " + s.Config.CLAGithubStatusContext + " Error: ```" + err.Error() + "```")
	}
}
