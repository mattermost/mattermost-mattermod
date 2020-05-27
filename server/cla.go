// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/google/go-github/v31/github"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v5/mlog"
)

func (s *Server) handleCheckCLA(ctx context.Context, eventIssueComment IssueComment) {
	prGitHub, _, err := s.GithubClient.PullRequests.Get(context.Background(), *eventIssueComment.Repository.Owner.Login, *eventIssueComment.Repository.Name, *eventIssueComment.Issue.Number)
	if err != nil {
		mlog.Error("Failed to get PR for CLA", mlog.Err(err))
		return
	}

	pr, err := s.GetPullRequestFromGithub(prGitHub)
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
	if pr.State == model.StateClosed {
		return
	}

	if s.isAlreadySigned(ctx, pr) {
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
		mlog.Info(fmt.Sprintf("%s is excluded to sign the CLA", username))
		return
	}

	resp, err := http.Get(s.Config.SignedCLAURL)
	if err != nil {
		mlog.Error("Unable to get CLA list", mlog.Err(err))
		return
	}

	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		mlog.Error("Unable to read response body", mlog.Err(err))
		return
	}

	comments, _, err := s.GithubClient.Issues.ListComments(context.Background(), pr.RepoOwner, pr.RepoName, pr.Number, nil)
	if err != nil {
		mlog.Error("pr_error", mlog.Err(err))
		return
	}

	if !s.isNameInCLAList(strings.Split(string(body), "\n"), username) {
		_, found := s.findNeedsToSignCLAComment(comments, s.Config.Username)
		if !found {
			s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number, strings.Replace(s.Config.NeedsToSignCLAMessage, "USERNAME", "@"+username, 1))
		}
		status := &github.RepoStatus{
			State:       github.String(stateError),
			Description: github.String(fmt.Sprintf("%v needs to sign the CLA", username)),
			TargetURL:   github.String(s.Config.SignedCLAURL),
			Context:     github.String("cla/mattermost"),
		}
		mlog.Debug("will post error on CLA", mlog.String("user", username))
		_, _, errStatus := s.GithubClient.Repositories.CreateStatus(context.Background(), pr.RepoOwner, pr.RepoName, pr.Sha, status)
		if errStatus != nil {
			mlog.Error("Unable to create the github status for for PR", mlog.Int("pr", pr.Number), mlog.Err(errStatus))
			return
		}
		return
	}

	s.cleanupCLAMissingReportComments(pr, comments)
	status := &github.RepoStatus{
		State:       github.String(stateSuccess),
		Description: github.String(fmt.Sprintf("%s authorized", username)),
		TargetURL:   github.String(s.Config.SignedCLAURL),
		Context:     github.String("cla/mattermost"),
	}
	mlog.Debug("will post success on CLA", mlog.String("user", username))
	_, _, err = s.GithubClient.Repositories.CreateStatus(context.Background(), pr.RepoOwner, pr.RepoName, pr.Sha, status)
	if err != nil {
		mlog.Error("Unable to create the github status for for PR", mlog.Int("pr", pr.Number), mlog.Err(err))
		return
	}
}

func (s *Server) isAlreadySigned(ctx context.Context, pr *model.PullRequest) bool {
	status, err := s.getStatus(ctx, pr, "cla/mattermost")
	if err != nil || status == nil {
		return false
	}

	return status.GetState() == stateSuccess
}

func (s *Server) isNameInCLAList(usersWhoSignedCLA []string, author string) bool {
	authorLowerCase := strings.ToLower(author)
	for _, user := range usersWhoSignedCLA {
		user := strings.TrimSpace(user)
		if strings.Compare(user, author) == 0 || strings.Compare(user, authorLowerCase) == 0 || strings.Compare(strings.ToLower(user), authorLowerCase) == 0 {
			return true
		}
	}
	return false
}

func (s *Server) findNeedsToSignCLAComment(comments []*github.IssueComment, username string) (id int64, found bool) {
	for _, comment := range comments {
		if *comment.User.Login == username && strings.Contains(*comment.Body, "Please help complete the Mattermost") {
			return *comment.ID, true
		}
	}
	return 0, false
}

func (s *Server) cleanupCLAMissingReportComments(pr *model.PullRequest, comments []*github.IssueComment) {
	mlog.Info("will clean some comments regarding the CLA")
	commentToRemove, found := s.findNeedsToSignCLAComment(comments, s.Config.Username)
	if found {
		mlog.Info("Removing old comment with ID", mlog.Int64("ID", commentToRemove))
		_, err := s.GithubClient.Issues.DeleteComment(context.Background(), pr.RepoOwner, pr.RepoName, commentToRemove)
		if err != nil {
			mlog.Error("Unable to remove old Mattermod comment", mlog.Err(err))
		}
	}
}

func (s *Server) createCLAPendingStatus(ctx context.Context, pr *model.PullRequest) {
	status := &github.RepoStatus{
		State:       github.String(statePending),
		Description: github.String("Checking if "+pr.Username+" signed CLA"),
		TargetURL:   github.String(s.Config.SignedCLAURL),
		Context:     github.String("cla/mattermost"),
	}
	s.createRepoStatus(ctx, pr, status)
}
