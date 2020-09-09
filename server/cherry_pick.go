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
	"github.com/mattermost/mattermost-server/v5/mlog"

	"github.com/google/go-github/v32/github"
	"github.com/pkg/errors"
)

const (
	cherryPickScheduledMsg = "Cherry pick is scheduled."
	tooManyCherryPickMsg   = "There are too many cherry picking requests. Please do this manually or try again later."
)

type cherryPickRequest struct {
	pr        *model.PullRequest
	version   string
	milestone *int
}

func (s *Server) listenCherryPickRequests() {
	defer func() {
		close(s.cherryPickStoppedChan)
	}()

	for job := range s.cherryPickRequests {
		func() {
			ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout*time.Second)
			defer cancel()
			pr := job.pr
			cmdOut, err := s.doCherryPick(ctx, strings.TrimSpace(job.version), job.milestone, pr)
			if err != nil {
				msg := fmt.Sprintf("Error trying doing the automated Cherry picking. Please do this manually\n\n```\n%s\n```\n", cmdOut)
				s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, msg)
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
			s.sendGitHubComment(ctx, job.pr.RepoOwner, job.pr.RepoName, job.pr.Number, msg)
		}
	case <-s.cherryPickStoppedChan:
	}
}

func (s *Server) handleCherryPick(ctx context.Context, commenter, body string, pr *model.PullRequest) error {
	var msg string
	defer func() {
		if msg != "" {
			s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, msg)
		}
	}()

	if !s.IsOrgMember(commenter) {
		msg = msgCommenterPermission
		return nil
	}

	args := strings.Split(body, " ")
	mlog.Info("Args", mlog.String("Args", body))
	if !pr.Merged.Valid || !pr.Merged.Bool {
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
			s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, msg)
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

	if s.Config.RepoFolder == "" {
		return "", errors.Errorf("path to folder containing local checkout of repositories is not set in the config")
	}
	repoFolder := filepath.Join(s.Config.RepoFolder, pr.RepoName)

	if _, err := os.Stat(repoFolder); os.IsNotExist(err) {
		err := cloneRepo(s.Config.RepoFolder, s.Config.Org+"/"+pr.RepoName, s.Config.GithubUsername+"/"+pr.RepoName, pr.RepoName)
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
		fmt.Sprintf("DRY_RUN=%t", false),
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
		err2 := returnToMaster(repoFolder)
		if err2 != nil {
			return string(out), fmt.Errorf("failed to return to master: %w", err2)
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
	err = returnToMaster(repoFolder)
	if err != nil {
		return "", fmt.Errorf("failed to return to master: %w", err)
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

func returnToMaster(dir string) error {
	cmd := exec.Command("git", "checkout", "master")
	if err := runCommand(cmd, dir); err != nil {
		return err
	}
	return nil
}

func cloneRepo(dir, upstreamSlug, originSlug, repoName string) error {
	// Clone repo
	cmd := exec.Command("git", "clone", "--depth=1", "git@github.com:"+originSlug+".git")
	if err := runCommand(cmd, dir); err != nil {
		return err
	}

	// Set upstream
	cmd = exec.Command("git", "remote", "add", "upstream", "git@github.com:"+upstreamSlug+".git")
	if err := runCommand(cmd, filepath.Join(dir, repoName)); err != nil {
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
