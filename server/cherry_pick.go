// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/mlog"

	"github.com/google/go-github/github"
)

func (s *Server) handleCherryPick(eventIssueComment IssueComment) {
	client := NewGithubClient(s.Config.GithubAccessToken)
	prGitHub, _, err := client.PullRequests.Get(context.Background(), *eventIssueComment.Repository.Owner.Login, *eventIssueComment.Repository.Name, *eventIssueComment.Issue.Number)
	pr, err := s.GetPullRequestFromGithub(prGitHub)
	if err != nil {
		mlog.Error("pr_error", mlog.Err(err))
		return
	}

	userComment := *eventIssueComment.Comment.User
	if !s.checkUserPermission(userComment.GetLogin(), pr.RepoOwner) {
		s.commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, "Looks like you dont have permissions to trigger this command.\n Only available for Org members")
		return
	}

	args := strings.Split(*eventIssueComment.Comment.Body, " ")
	mlog.Info("Args", mlog.String("Args", *eventIssueComment.Comment.Body))
	if !prGitHub.GetMerged() {
		mlog.Info("PR not merged, not cherry picking", mlog.Int("PR Number", prGitHub.GetNumber()), mlog.String("Repo", pr.RepoName))
		return
	}

	cmdOut, err := s.doCherryPick(args[1], pr)
	if err != nil {
		mlog.Error("Error doing the cherry pick", mlog.Err(err))
		errMsg := fmt.Sprintf("Error trying doing the automated Cherry picking. Please do this manually\n\n```\n%s\n```\n", cmdOut)
		s.commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, errMsg)
		return
	}
}

func (s *Server) checkIfNeedCherryPick(pr *model.PullRequest) {
	client := NewGithubClient(s.Config.GithubAccessToken)

	prCherryCandidate, _, err := client.PullRequests.Get(context.Background(), pr.RepoOwner, pr.RepoName, pr.Number)
	if err != nil {
		mlog.Error("Error getting the PR info", mlog.Err(err))
		return
	}

	if !prCherryCandidate.GetMerged() {
		mlog.Info("PR not merged, not cherry picking", mlog.Int("PR Number", prCherryCandidate.GetNumber()), mlog.String("Repo", pr.RepoName))
		return
	}

	prMilestone := prCherryCandidate.GetMilestone()
	if prMilestone == nil {
		mlog.Info("PR does not have milestone, not cherry picking", mlog.Int("PR Number", prCherryCandidate.GetNumber()), mlog.String("Repo", pr.RepoName))
		return
	}

	milestone := strings.TrimSpace(prMilestone.GetTitle())
	milestone = strings.Trim(milestone, "v")
	milestone = fmt.Sprintf("release-%s", strings.Trim(milestone, ".0"))

	labels, _, err := client.Issues.ListLabelsByIssue(context.Background(), pr.RepoOwner, pr.RepoName, pr.Number, nil)
	if err != nil {
		mlog.Error("Error listing the labels for PR", mlog.Err(err))
		return
	}
	prLabels := LabelsToStringArray(labels)
	for _, prLabel := range prLabels {
		if prLabel == "CherryPick/Approved" {
			cmdOut, err := s.doCherryPick(milestone, pr)
			if err != nil {
				mlog.Error("Error doing the cherry pick", mlog.Err(err))
				errMsg := fmt.Sprintf("@%s\nError trying doing the automated Cherry picking. Please do this manually\n\n```\n%s\n```\n", pr.Username, cmdOut)
				s.commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, errMsg)
				return
			}
		}
	}
}

func (s *Server) doCherryPick(version string, pr *model.PullRequest) (cmdOutput string, err error) {
	releaseBranch := fmt.Sprintf("upstream/%s", version)
	repoFolder := fmt.Sprintf("/home/ubuntu/git/mattermost/%s", pr.RepoName)
	cmd := exec.Command("/home/ubuntu/git/devops/cherry-pick.sh", releaseBranch, strconv.Itoa(pr.Number))
	cmd.Dir = repoFolder
	cmd.Env = append(
		os.Environ(),
		os.Getenv("PATH"),
		fmt.Sprintf("ORIGINAL_AUTHOR=%s", pr.Username),
		fmt.Sprintf("GITHUB_USER=%s", s.Config.GithubUsername),
		fmt.Sprintf("GITHUB_TOKEN=%s", s.Config.GithubAccessTokenCherryPick),
	)
	out, err := cmd.Output()
	if err != nil {
		mlog.Error("cmd.Run() failed", mlog.Err(err), mlog.String("cmdOut", string(out)))
		returnToMaster(repoFolder)
		return string(out), err
	}
	gitHubPR := regexp.MustCompile(`https://github.com/mattermost/.*\.*[0-9]+`)
	newPRURL := gitHubPR.FindString(string(out))
	newPR := strings.Split(newPRURL, "/")
	newPRNumber := newPR[len(newPR)-1]
	s.updateCherryPickLabels(newPRNumber, pr)
	s.addReviewers(newPRNumber, pr)
	s.addAssignee(newPRNumber, pr)
	returnToMaster(repoFolder)
	return "", nil
}

func (s *Server) updateCherryPickLabels(newPR string, pr *model.PullRequest) {
	client := NewGithubClient(s.Config.GithubAccessToken)

	// Add the AutomatedCherryPick/Done in the new pr
	newPRNumner, _ := strconv.Atoi(newPR)
	_, _, err := client.Issues.AddLabelsToIssue(context.Background(), pr.RepoOwner, pr.RepoName, newPRNumner, []string{"AutomatedCherryPick/Done"})
	if err != nil {
		mlog.Error("Error applying the automated label in the new pr ", mlog.Err(err), mlog.Int("PR", newPRNumner), mlog.String("Repo", pr.RepoName))
		return
	}

	// remove the CherryPick/Approved and add the AutomatedCherryPick/Done
	_, _, err = client.Issues.AddLabelsToIssue(context.Background(), pr.RepoOwner, pr.RepoName, pr.Number, []string{"AutomatedCherryPick/Done"})
	if err != nil {
		mlog.Error("Error applying the automated label in the cherry pick pr ", mlog.Err(err), mlog.Int("PR", pr.Number), mlog.String("Repo", pr.RepoName))
		return
	}

	_, err = client.Issues.RemoveLabelForIssue(context.Background(), pr.RepoOwner, pr.RepoName, pr.Number, "CherryPick/Approved")
	if err != nil {
		mlog.Error("Error removing the automated label in the cherry pick pr ", mlog.Err(err), mlog.Int("PR", pr.Number), mlog.String("Repo", pr.RepoName))
		return
	}
}

func (s *Server) addReviewers(newPR string, pr *model.PullRequest) {
	client := NewGithubClient(s.Config.GithubAccessToken)
	newPRNumber, _ := strconv.Atoi(newPR)
	// Get the reviwers from the cherry pick PR
	reviewersFromPR, _, err := client.PullRequests.ListReviews(context.Background(), pr.RepoOwner, pr.RepoName, pr.Number, nil)
	if err != nil {
		mlog.Error("Error getting the reviewers from the original PR", mlog.Err(err), mlog.Int("PR", pr.Number), mlog.String("Repo", pr.RepoName))
		return
	}

	var requestReviewers []string
	randonReviewer := rand.Intn(len(reviewersFromPR) - 1)
	requestReviewers = append(requestReviewers, reviewersFromPR[randonReviewer].User.GetLogin())

	reviewReq := github.ReviewersRequest{
		Reviewers: requestReviewers,
	}
	_, _, err = client.PullRequests.RequestReviewers(context.Background(), pr.RepoOwner, pr.RepoName, newPRNumber, reviewReq)
	if err != nil {
		mlog.Error("Error setting the reviewers ", mlog.Err(err), mlog.Int("PR", newPRNumber), mlog.String("Repo", pr.RepoName))
		return
	}
}

func (s *Server) addAssignee(newPR string, pr *model.PullRequest) {
	client := NewGithubClient(s.Config.GithubAccessToken)
	newPRNumber, _ := strconv.Atoi(newPR)

	assignee := []string{pr.Username}
	_, _, err := client.Issues.AddAssignees(context.Background(), pr.RepoOwner, pr.RepoName, newPRNumber, assignee)
	if err != nil {
		mlog.Error("Error setting the reviewers ", mlog.Err(err), mlog.Int("PR", newPRNumber), mlog.String("Repo", pr.RepoName))
		return
	}
}

func returnToMaster(dir string) {
	cmd := exec.Command("git", "checkout", "master")
	cmd.Dir = dir
	cmd.Env = append(
		os.Environ(),
		os.Getenv("PATH"),
	)
	cmd.Run()
}
