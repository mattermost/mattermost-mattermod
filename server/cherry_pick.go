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
	"github.com/mattermost/mattermost-server/v5/mlog"

	"github.com/google/go-github/v31/github"
)

func (s *Server) handleCherryPick(ctx context.Context, eventIssueComment IssueComment) {
	prGitHub, _, err := s.GithubClient.PullRequests.Get(ctx, *eventIssueComment.Repository.Owner.Login, *eventIssueComment.Repository.Name, *eventIssueComment.Issue.Number)
	if err != nil {
		mlog.Error("Failed to get cherry pick PR", mlog.Err(err))
		return
	}

	pr, err := s.GetPullRequestFromGithub(ctx, prGitHub)
	if err != nil {
		mlog.Error("pr_error", mlog.Err(err))
		return
	}

	if !s.IsOrgMember(eventIssueComment.Comment.User.GetLogin()) {
		mlog.Debug("not org member", mlog.String("user", eventIssueComment.Comment.User.GetLogin()))
		s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, "Looks like you don't have permissions to trigger this command.\n Only available for Org members")
		return
	}

	args := strings.Split(*eventIssueComment.Comment.Body, " ")
	mlog.Info("Args", mlog.String("Args", *eventIssueComment.Comment.Body))
	if !prGitHub.GetMerged() {
		mlog.Info("PR not merged, not cherry picking", mlog.Int("PR Number", prGitHub.GetNumber()), mlog.String("Repo", pr.RepoName))
		return
	}
	cmdOut, err := s.doCherryPick(ctx, strings.TrimSpace(args[1]), nil, pr)
	if err != nil {
		mlog.Error("Error doing the cherry pick", mlog.Err(err))
		errMsg := fmt.Sprintf("Error trying doing the automated Cherry picking. Please do this manually\n\n```\n%s\n```\n", cmdOut)
		s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, errMsg)
		return
	}
}

func (s *Server) checkIfNeedCherryPick(ctx context.Context, pr *model.PullRequest) {
	prCherryCandidate, _, err := s.GithubClient.PullRequests.Get(ctx, pr.RepoOwner, pr.RepoName, pr.Number)
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

	labels, _, err := s.GithubClient.Issues.ListLabelsByIssue(ctx, pr.RepoOwner, pr.RepoName, pr.Number, nil)
	if err != nil {
		mlog.Error("Error listing the labels for PR", mlog.Err(err))
		return
	}
	prLabels := labelsToStringArray(labels)
	for _, prLabel := range prLabels {
		if prLabel == "CherryPick/Approved" {
			milestoneNumber := prMilestone.GetNumber()
			milestone := getMilestone(prMilestone.GetTitle())
			cmdOut, err := s.doCherryPick(ctx, milestone, &milestoneNumber, pr)
			if err != nil {
				mlog.Error("Error doing the cherry pick", mlog.Err(err))
				errMsg := fmt.Sprintf("@%s\nError trying doing the automated Cherry picking. Please do this manually\n\n```\n%s\n```\n", pr.Username, cmdOut)
				s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, errMsg)
				return
			}
		}
	}
}

func getMilestone(title string) string {
	milestone := strings.TrimSpace(title)
	milestone = strings.Trim(milestone, "v")
	milestone = strings.TrimSuffix(milestone, ".0")
	milestone = fmt.Sprintf("release-%s", milestone)
	return milestone
}

func (s *Server) doCherryPick(ctx context.Context, version string, milestoneNumber *int, pr *model.PullRequest) (cmdOutput string, err error) {
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
	newPRNumber, _ := strconv.Atoi(newPR[len(newPR)-1])
	assignee := s.getAssignee(ctx, newPRNumber, pr)

	if milestoneNumber != nil {
		s.addMilestone(ctx, newPRNumber, pr, milestoneNumber)
	}
	s.updateCherryPickLabels(ctx, newPRNumber, pr)
	s.addReviewers(ctx, newPRNumber, pr, []string{assignee})
	s.addAssignee(ctx, newPRNumber, pr, []string{assignee})
	returnToMaster(repoFolder)
	return "", nil
}

func (s *Server) getAssignee(ctx context.Context, newPRNumber int, pr *model.PullRequest) string {
	var assignee string
	if s.IsOrgMember(pr.Username) {
		// He/She can review the PR herself/himself
		assignee = pr.Username
	} else {
		// We have to get a random reviewer from the original PR
		// Get the reviewers from the cherry pick PR
		mlog.Debug("not org member", mlog.String("user", pr.Username))
		reviewersFromPR, _, err := s.GithubClient.PullRequests.ListReviews(ctx, pr.RepoOwner, pr.RepoName, pr.Number, nil)
		if err != nil {
			mlog.Error("Error getting the reviewers from the original PR", mlog.Err(err), mlog.Int("PR", pr.Number), mlog.String("Repo", pr.RepoName))
			return ""
		}

		randomReviewer := rand.Intn(len(reviewersFromPR) - 1)
		assignee = reviewersFromPR[randomReviewer].User.GetLogin()
	}

	return assignee
}

func (s *Server) updateCherryPickLabels(ctx context.Context, newPRNumber int, pr *model.PullRequest) {
	// Add the AutomatedCherryPick/Done in the new pr
	labelsNewPR := []string{"AutomatedCherryPick", "Changelog/Not Needed", "Docs/Not Needed"}
	_, _, err := s.GithubClient.Issues.AddLabelsToIssue(ctx, pr.RepoOwner, pr.RepoName, newPRNumber, labelsNewPR)
	if err != nil {
		mlog.Error("Error applying the automated label in the new pr ", mlog.Err(err), mlog.Int("PR", newPRNumber), mlog.String("Repo", pr.RepoName))
		return
	}

	// remove the CherryPick/Approved and add the CherryPick/Done
	_, _, err = s.GithubClient.Issues.AddLabelsToIssue(ctx, pr.RepoOwner, pr.RepoName, pr.Number, []string{"CherryPick/Done"})
	if err != nil {
		mlog.Error("Error applying the automated label in the cherry pick pr ", mlog.Err(err), mlog.Int("PR", pr.Number), mlog.String("Repo", pr.RepoName))
		return
	}

	_, err = s.GithubClient.Issues.RemoveLabelForIssue(ctx, pr.RepoOwner, pr.RepoName, pr.Number, "CherryPick/Approved")
	if err != nil {
		mlog.Error("Error removing the automated label in the cherry pick pr ", mlog.Err(err), mlog.Int("PR", pr.Number), mlog.String("Repo", pr.RepoName))
		return
	}
}

func (s *Server) addMilestone(ctx context.Context, newPRNumber int, pr *model.PullRequest, milestoneNumber *int) {
	// Add the milestone to the new PR
	request := &github.IssueRequest{
		Milestone: milestoneNumber,
	}
	_, _, err := s.GithubClient.Issues.Edit(ctx, pr.RepoOwner, pr.RepoName, newPRNumber, request)
	if err != nil {
		mlog.Error("Error applying the milestone in the new pr", mlog.Err(err), mlog.Int("PR", newPRNumber), mlog.String("Repo", pr.RepoName))
	}
}

func (s *Server) addReviewers(ctx context.Context, newPRNumber int, pr *model.PullRequest, reviewers []string) {
	reviewReq := github.ReviewersRequest{
		Reviewers: reviewers,
	}
	_, _, err := s.GithubClient.PullRequests.RequestReviewers(ctx, pr.RepoOwner, pr.RepoName, newPRNumber, reviewReq)
	if err != nil {
		mlog.Error("Error setting the reviewers ", mlog.Err(err), mlog.Int("PR", newPRNumber), mlog.String("Repo", pr.RepoName))
		return
	}
}

func (s *Server) addAssignee(ctx context.Context, newPRNumber int, pr *model.PullRequest, assignees []string) {
	_, _, err := s.GithubClient.Issues.AddAssignees(ctx, pr.RepoOwner, pr.RepoName, newPRNumber, assignees)
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
	err := cmd.Run()
	if err != nil {
		mlog.Error("Failed to return to master", mlog.Err(err))
	}
}
