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
	"sync"
	"time"

	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v5/mlog"

	"github.com/google/go-github/v32/github"
	"github.com/pkg/errors"
)

func (s *Server) handleCherryPick(ctx context.Context, commenter, body string, pr *model.PullRequest) error {
	var msg string
	defer func() {
		if msg != "" {
			s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, msg)
		}
	}()

	if !s.IsOrgMember(commenter) {
		msg = MsgCommenterPermission
		return nil
	}

	args := strings.Split(body, " ")
	mlog.Info("Args", mlog.String("Args", body))
	if !pr.Merged.Valid || !pr.Merged.Bool {
		return nil
	}
	cmdOut, err := s.doCherryPick(ctx, strings.TrimSpace(args[1]), nil, pr)
	if err != nil {
		msg = fmt.Sprintf("Error trying doing the automated Cherry picking. Please do this manually\n\n```\n%s\n```\n", cmdOut)
		return err
	}

	return nil
}

func (s *Server) checkIfNeedCherryPick(pr *model.PullRequest) {
	// We create a new context here instead of using the parent one because this is being called from a goroutine.
	// Ideally, the entire request needs to be handled asynchronously.
	// See: https://github.com/mattermost/mattermost-mattermod/pull/166.
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout*time.Second)
	defer cancel()

	if !pr.Merged.Valid || !pr.Merged.Bool {
		mlog.Info("PR not merged, not cherry picking", mlog.Int("PR Number", pr.Number), mlog.String("Repo", pr.RepoName))
		return
	}

	prCherryCandidate, _, err := s.GithubClient.PullRequests.Get(ctx, pr.RepoOwner, pr.RepoName, pr.Number)
	if err != nil {
		mlog.Error("Error getting the PR info", mlog.Err(err))
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
	if pr.MergeCommitSHA == "" {
		return "", errors.Errorf("can't get merge commit SHA for PR: %d", pr.Number)
	}
	releaseBranch := fmt.Sprintf("upstream/%s", version)
	repoFolder := fmt.Sprintf("/app/repos/%s", pr.RepoName)
	cmd := exec.Command("/app/scripts/cherry-pick.sh", releaseBranch, strconv.Itoa(pr.Number), pr.MergeCommitSHA)
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
		mlog.Error("cmd.Run() failed",
			mlog.Err(err),
			mlog.String("cmdOut", string(out)),
			mlog.String("repo", pr.RepoName),
			mlog.Int("PR", pr.Number),
		)
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
	var once sync.Once
	once.Do(func() {
		rand.Seed(time.Now().Unix())
	})

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
