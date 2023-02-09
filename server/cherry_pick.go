// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v6/shared/mlog"

	"github.com/google/go-github/v39/github"
	"github.com/pkg/errors"
)

const (
	cherryPickScheduledMsg = "Cherry pick is scheduled."
	tooManyCherryPickMsg   = "There are too many cherry picking requests. Please do this manually or try again later."
	milestoneCloud         = "cloud"
)

type cherryPickRequest struct {
	pr        *model.PullRequest
	milestone *int
	version   string
}

func (s *Server) listenCherryPickRequests() {
	defer func() {
		close(s.cherryPickStoppedChan)
	}()

	for job := range s.cherryPickRequests {
		func() {
			ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout*2*time.Second)
			defer cancel()
			pr := job.pr
			cmdOut, err := s.doCherryPick(ctx, strings.TrimSpace(job.version), job.milestone, pr)
			if err != nil {
				msg := fmt.Sprintf("Error trying doing the automated Cherry picking. Please do this manually\n\n```\n%s\n```\n", cmdOut)
				if cErr := s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, msg); cErr != nil {
					mlog.Warn("Error while commenting", mlog.Err(cErr))
				}
				mlog.Error("Error while cherry picking", mlog.Err(err))
			}
		}()
	}
}

func (s *Server) finishCherryPickRequests() {
	close(s.cherryPickStopChan)
	close(s.cherryPickRequests)
	select {
	case <-time.After(5 * time.Second):
		ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout*time.Second)
		defer cancel()
		// While consuming requests here, listenCherryPickRequests routine will continue
		// to listen as well. We're just trying to cancel requests as much as we can.
		msg := "Cherry picking is canceled due to server shutdown."
		for job := range s.cherryPickRequests {
			if err := s.sendGitHubComment(ctx, job.pr.RepoOwner, job.pr.RepoName, job.pr.Number, msg); err != nil {
				mlog.Warn("Error while commenting", mlog.Err(err))
			}
		}
	case <-s.cherryPickStoppedChan:
	}
}

func (s *Server) handleCherryPick(ctx context.Context, commenter, body string, pr *model.PullRequest) error {
	var msg string
	defer func() {
		if msg != "" {
			if err := s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, msg); err != nil {
				mlog.Warn("Error while commenting", mlog.Err(err))
			}
		}
	}()

	if !s.IsOrgMember(commenter) || s.IsInBotBlockList(commenter) {
		msg = msgCommenterPermission
		return nil
	}
	command := getCommand(body)
	args := strings.Split(command, " ")
	mlog.Info("Args", mlog.String("Args", body))
	if !pr.GetMerged() {
		return nil
	}

	if len(args) < 2 {
		return nil
	}

	select {
	case <-s.cherryPickStopChan:
		return errors.New("server is closing")
	default:
	}

	select {
	case s.cherryPickRequests <- &cherryPickRequest{
		pr:      pr,
		version: strings.TrimSpace(args[1]),
	}:
		msg = cherryPickScheduledMsg
	default:
		msg = tooManyCherryPickMsg
		return errors.New("too many requests")
	}

	return nil
}

func getCommand(command string) string {
	index := strings.Index(command, "/cherry-pick")
	return command[index:]
}

func (s *Server) checkIfNeedCherryPick(pr *model.PullRequest) {
	// We create a new context here instead of using the parent one because this is being called from a goroutine.
	// Ideally, the entire request needs to be handled asynchronously.
	// See: https://github.com/mattermost/mattermost-mattermod/pull/166.
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout*time.Second)
	defer cancel()

	if !pr.GetMerged() {
		mlog.Info("PR not merged, not cherry picking", mlog.Int("PR Number", pr.Number), mlog.String("Repo", pr.RepoName))
		return
	}

	if pr.GetMilestoneNumber() == 0 || pr.GetMilestoneTitle() == "" {
		mlog.Info("PR milestone number not available", mlog.Int("PR Number", pr.Number), mlog.String("Repo", pr.RepoName))
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
			milestoneNumber := int(pr.GetMilestoneNumber())
			milestone := getMilestone(pr.GetMilestoneTitle())

			select {
			case <-s.cherryPickStopChan:
				return
			default:
			}

			var msg string
			select {
			case s.cherryPickRequests <- &cherryPickRequest{
				pr:        pr,
				version:   strings.TrimSpace(milestone),
				milestone: &milestoneNumber,
			}:
				msg = cherryPickScheduledMsg
			default:
				msg = tooManyCherryPickMsg
			}
			if err := s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, msg); err != nil {
				mlog.Warn("Error while commenting", mlog.Err(err))
			}
		}
	}
}

func getMilestone(title string) string {
	milestone := strings.TrimSpace(title)
	milestone = strings.Trim(milestone, "v")
	milestone = strings.TrimSuffix(milestone, ".0")
	if title != milestoneCloud {
		milestone = fmt.Sprintf("release-%s", milestone)
	}
	return milestone
}

func (s *Server) doCherryPick(ctx context.Context, version string, milestoneNumber *int, pr *model.PullRequest) (cmdOutput string, err error) {
	if pr.MergeCommitSHA == "" {
		return "", errors.Errorf("can't get merge commit SHA for PR: %d", pr.Number)
	}

	if s.Config.RepoFolder == "" {
		return "", errors.Errorf("path to folder containing local checkout of repositories is not set in the config")
	}
	repoFolder := filepath.Join(s.Config.RepoFolder, pr.RepoName)

	if _, err = os.Stat(repoFolder); os.IsNotExist(err) {
		err = cloneRepo(ctx, s.Config, pr.RepoName)
		if err != nil {
			return "", fmt.Errorf("error while cloning repo: %s, %v", s.Config.Org+"/"+pr.RepoName, err)
		}
	}

	if s.Config.ScriptsFolder == "" {
		return "", errors.Errorf("path to folder containing the cherry-pick.sh script is not set in the config")
	}
	cherryPickScript := filepath.Join(s.Config.ScriptsFolder, "cherry-pick.sh")

	releaseBranch := fmt.Sprintf("upstream/%s", version)
	cmd := exec.Command(cherryPickScript, releaseBranch, strconv.Itoa(pr.Number), pr.MergeCommitSHA)
	cmd.Dir = repoFolder
	cmd.Env = append(
		os.Environ(),
		os.Getenv("PATH"),
		fmt.Sprintf("ORIGINAL_AUTHOR=%s", pr.Username),
		fmt.Sprintf("GITHUB_USER=%s", s.Config.GithubUsername),
		fmt.Sprintf("GITHUB_TOKEN=%s", s.Config.GithubAccessTokenCherryPick),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		mlog.Error("cmd.Run() failed",
			mlog.Err(err),
			mlog.String("cmdOut", string(out)),
			mlog.String("repo", pr.RepoName),
			mlog.Int("PR", pr.Number),
		)
		err2 := returnToMaster(ctx, repoFolder)
		if err2 != nil {
			mlog.Error("Failed to return to master", mlog.Err(err2))
		}
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
	err = returnToMaster(ctx, repoFolder)
	if err != nil {
		mlog.Error("Failed to return to master", mlog.Err(err))
	}
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

		randomReviewer := rand.Intn(len(reviewersFromPR) - 1) // nolint
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

func returnToMaster(ctx context.Context, dir string) error {
	cmd := exec.CommandContext(ctx, "git", "checkout", "master")
	if err := runCommand(cmd, dir); err != nil {
		return err
	}
	return nil
}

func cloneRepo(ctx context.Context, cfg *Config, repoName string) error {
	originSlug := cfg.Org + "/" + repoName
	upstreamSlug := cfg.GithubUsername + "/" + repoName

	// Clone repo
	cmd := exec.CommandContext(ctx, "git", "clone", "git@github.com:"+originSlug+".git")
	if err := runCommand(cmd, cfg.RepoFolder); err != nil {
		return err
	}

	// Set username and email.
	cmd = exec.CommandContext(ctx, "git", "config", "user.name")
	if out, err := runCommandWithOutput(cmd, filepath.Join(cfg.RepoFolder, repoName)); err != nil {
		return err
	} else if out == "" { // this means username is not set
		cmd = exec.CommandContext(ctx, "git", "config", "user.name", cfg.GithubUsername)
		if err = runCommand(cmd, filepath.Join(cfg.RepoFolder, repoName)); err != nil {
			return err
		}
	}

	cmd = exec.CommandContext(ctx, "git", "config", "user.email")
	if out, err := runCommandWithOutput(cmd, filepath.Join(cfg.RepoFolder, repoName)); err != nil {
		return err
	} else if out == "" { // this means email is not set
		cmd = exec.CommandContext(ctx, "git", "config", "user.email", cfg.GithubEmail)
		if err = runCommand(cmd, filepath.Join(cfg.RepoFolder, repoName)); err != nil {
			return err
		}
	}

	// Set upstream
	cmd = exec.CommandContext(ctx, "git", "remote", "add", "upstream", "git@github.com:"+upstreamSlug+".git")
	if err := runCommand(cmd, filepath.Join(cfg.RepoFolder, repoName)); err != nil {
		return err
	}
	return nil
}

func runCommand(cmd *exec.Cmd, dir string) error {
	cmd.Dir = dir
	cmd.Env = append(
		os.Environ(),
		os.Getenv("PATH"),
	)
	return cmd.Run()
}

func runCommandWithOutput(cmd *exec.Cmd, dir string) (string, error) {
	cmd.Dir = dir
	cmd.Env = append(
		os.Environ(),
		os.Getenv("PATH"),
	)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
