// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/mlog"

	"github.com/google/go-github/github"
)

func handleCherryPick(eventIssueComment IssueComment) {
	client := NewGithubClient()
	prGitHub, _, err := client.PullRequests.Get(context.Background(), *eventIssueComment.Repository.Owner.Login, *eventIssueComment.Repository.Name, *eventIssueComment.Issue.Number)
	pr, err := GetPullRequestFromGithub(prGitHub)
	if err != nil {
		mlog.Error("pr_error", mlog.Err(err))
		return
	}

	args := strings.Split(*eventIssueComment.Comment.Body, " ")
	mlog.Info("Args", mlog.String("Args", *eventIssueComment.Comment.Body))
	if !prGitHub.GetMerged() {
		mlog.Info("PR not merged, not cherry picking", mlog.Int("PR Number", prGitHub.GetNumber()), mlog.String("Repo", pr.RepoName))
		return
	}

	doCherryPick(args[1], pr)
}

func checkIfNeedCherryPick(pr *model.PullRequest) {
	client := NewGithubClient()

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
	milestone := strings.Trim(prMilestone.GetTitle(), "v")
	milestone = fmt.Sprintf("release-%s", strings.Trim(milestone, ".0"))

	labels, _, err := client.Issues.ListLabelsByIssue(context.Background(), pr.RepoOwner, pr.RepoName, pr.Number, nil)
	if err != nil {
		mlog.Error("Error listing the labels for PR", mlog.Err(err))
		return
	}
	prLabels := LabelsToStringArray(labels)
	for _, prLabel := range prLabels {
		if prLabel == "CherryPick/Approved" {
			err := doCherryPick(milestone, pr)
			if err != nil {
				mlog.Error("Error doing the cherry pick", mlog.Err(err))
				return
			}
		}
	}
}

func doCherryPick(version string, pr *model.PullRequest) error {
	releaseBranch := fmt.Sprintf("upstream/%s", version)
	repoFolder := fmt.Sprintf("/home/ubuntu/git/mattermost/%s", pr.RepoName)
	cmd := exec.Command("/home/ubuntu/git/devops/cherry-pick.sh", releaseBranch, strconv.Itoa(pr.Number))
	cmd.Dir = repoFolder
	cmd.Env = append(
		os.Environ(),
		os.Getenv("PATH"),
		fmt.Sprintf("ORIGINAL_AUTHOR=%s", pr.Username),
		fmt.Sprintf("GITHUB_USER=%s", Config.GithubUsername),
		fmt.Sprintf("GITHUB_TOKEN=%s", Config.GithubAccessTokenCherryPick),
	)
	out, err := cmd.Output()
	if err != nil {
		mlog.Error("cmd.Run() failed", mlog.Err(err), mlog.String("cmdOut", string(out)))
		returnToMaster(repoFolder)
		webhookMessage := fmt.Sprintf("Error doing the Cherry pick, see the logs\n%s\n", string(out))
		webhookRequest := &WebhookRequest{Username: "Mattermost-Build", Text: webhookMessage}
		if errWebhook := sendToWebhook(webhookRequest, Config.MattermostWebhookURL); err != nil {
			mlog.Error("Unable to post to Mattermost webhook", mlog.Err(errWebhook))
		}
		return err
	}
	fmt.Println(string(out))
	gitHubPR := regexp.MustCompile(`https://github.com/mattermost/.*\.*[0-9]+`)
	newPRURL := gitHubPR.FindString(string(out))
	newPR := strings.Split(newPRURL, "/")
	updateCherryPickLabels(newPR[len(newPR)-1], pr)
	addReviewers(newPR[len(newPR)-1], pr)
	returnToMaster(repoFolder)
	return nil
}

func updateCherryPickLabels(newPR string, pr *model.PullRequest) {
	client := NewGithubClient()

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

func addReviewers(newPR string, pr *model.PullRequest) {
	client := NewGithubClient()
	newPRNumner, _ := strconv.Atoi(newPR)
	// Get the reviwers from the cherry pick PR

	reviewersFromPR, _, err := client.PullRequests.ListReviewers(context.Background(), pr.RepoOwner, pr.RepoName, pr.Number, nil)
	if err != nil {
		mlog.Error("Error getting the reviewers from the original PR", mlog.Err(err), mlog.Int("PR", pr.Number), mlog.String("Repo", pr.RepoName))
		return
	}

	var requestReviewers []string
	for _, reviewer := range reviewersFromPR.Users {
		requestReviewers = append(requestReviewers, reviewer.GetLogin())
	}

	reviewReq := github.ReviewersRequest{
		Reviewers: requestReviewers,
	}
	_, _, err = client.PullRequests.RequestReviewers(context.Background(), pr.RepoOwner, pr.RepoName, newPRNumner, reviewReq)
	if err != nil {
		mlog.Error("Error setting the reviewers ", mlog.Err(err), mlog.Int("PR", newPRNumner), mlog.String("Repo", pr.RepoName))
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
