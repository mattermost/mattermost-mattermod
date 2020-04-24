// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/google/go-github/v28/github"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v5/mlog"
)

func (s *Server) handlePullRequestEvent(event *PullRequestEvent) {
	mlog.Info("PR-Event", mlog.String("repo", *event.Repo.Name), mlog.Int("pr", event.PRNumber), mlog.String("action", event.Action))
	pr, err := s.GetPullRequestFromGithub(event.PullRequest)
	if err != nil {
		mlog.Error("Unable to get PR from GitHub", mlog.Int("pr", event.PRNumber), mlog.Err(err))
		return
	}

	switch event.Action {
	case "opened":
		mlog.Info("PR opened", mlog.String("repo", pr.RepoName), mlog.Int("pr", pr.Number))
		s.checkCLA(pr)
		if pr.RepoName == s.Config.EnterpriseTriggerReponame {
			s.createEnterpriseTestsPendingStatus(pr)
		}
		s.triggerCircleCiIfNeeded(pr)
		s.addHacktoberfestLabel(pr)
		if s.isBlockPRMergeInLabels(pr.Labels) {
			s.blockPRMerge(pr)
		} else {
			s.unblockPRMerge(pr)
		}
	case "reopened":
		mlog.Info("PR reopened", mlog.String("repo", pr.RepoName), mlog.Int("pr", pr.Number))
		s.checkCLA(pr)
		if pr.RepoName == s.Config.EnterpriseTriggerReponame {
			s.createEnterpriseTestsPendingStatus(pr)
		}
		s.triggerCircleCiIfNeeded(pr)
		if s.isBlockPRMergeInLabels(pr.Labels) {
			s.blockPRMerge(pr)
		} else {
			s.unblockPRMerge(pr)
		}
	case "labeled":
		if event.Label == nil {
			mlog.Error("Label event received, but label object was empty")
			return
		}
		if *event.Label.Name == s.Config.BuildMobileAppTag {
			mlog.Info("PR received Build mobile app label", mlog.String("repo", *event.Repo.Name), mlog.Int("pr", event.PRNumber), mlog.String("label", *event.Label.Name))
			mobileRepoOwner, mobileRepoName := pr.RepoOwner, pr.RepoName
			go s.buildMobileApp(pr)
			s.removeLabel(mobileRepoOwner, mobileRepoName, pr.Number, s.Config.BuildMobileAppTag)
		}

		if pr.RepoName == s.Config.EnterpriseTriggerReponame {
			mlog.Debug("before label", mlog.String("enterprise label", s.Config.EnterpriseTriggerLabel))
			mlog.Debug("before label", mlog.String("enterprise repo", s.Config.EnterpriseReponame))
			mlog.Debug("before label", mlog.String("trigger repo", pr.RepoName))
			mlog.Debug("before label", mlog.Int("pr", pr.Number))
			mlog.Debug("before label", mlog.String("label", *event.Label.Name))
			if *event.Label.Name == s.Config.EnterpriseTriggerLabel {
				mlog.Info("Label to run ee tests", mlog.String("repo", pr.RepoName), mlog.Int("pr", event.PRNumber), mlog.String("label", *event.Label.Name))
				go s.triggerEnterpriseTests(pr)
				s.removeLabel(pr.RepoOwner, pr.RepoName, pr.Number, s.Config.EnterpriseTriggerLabel)
			}
		}

		// TODO: remove the old test server code
		if event.Label.GetName() == s.Config.SetupSpinmintTag {
			mlog.Info("Label to spin a old test server")
			s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number, s.Config.SetupSpinmintMessage)
			go s.waitForBuildAndSetupSpinmint(pr, false)
		}
		if s.isBlockPRMerge(*event.Label.Name) {
			s.blockPRMerge(pr)
		}
		if s.isAutoMergeLabelInLabels(pr.Labels) {
			msg := "Will try to auto merge this PR once all tests and checks are passing. This might take up to an hour."
			s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number, msg)
		}
	case "unlabeled":
		if event.Label == nil {
			mlog.Error("Unlabel event received, but label object was empty")
			return
		}
		// TODO: remove the old test server code
		if s.isSpinMintLabel(*event.Label.Name) {
			if result := <-s.Store.Spinmint().Get(pr.Number, pr.RepoName); result.Err != nil {
				mlog.Error("Unable to get the test server information.", mlog.String("pr_error", result.Err.Error()))
			} else if result.Data == nil {
				mlog.Info("Nothing to do. There is not test server for this PR", mlog.Int("pr", pr.Number))
			} else {
				spinmint := result.Data.(*model.Spinmint)
				mlog.Info("test server instance", mlog.String("test server", spinmint.InstanceId))
				mlog.Info("Will destroy the test server for a merged/closed PR.")
				s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number, s.Config.DestroyedSpinmintMessage)
				go s.destroySpinmint(pr, spinmint.InstanceId)
			}
		}
		if s.isBlockPRMerge(*event.Label.Name) {
			s.unblockPRMerge(pr)
		}
	case "synchronize":
		mlog.Info("PR has a new commit", mlog.String("repo", pr.RepoName), mlog.Int("pr", pr.Number))
		s.checkCLA(pr)
		if pr.RepoName == s.Config.EnterpriseTriggerReponame {
			s.createEnterpriseTestsPendingStatus(pr)

			// todo: remove after build.mattermost.com is gone
			s.succeedOutDatedJenkinsStatuses(pr)
		}
		s.triggerCircleCiIfNeeded(pr)
		if s.isBlockPRMergeInLabels(pr.Labels) {
			s.blockPRMerge(pr)
		} else {
			s.unblockPRMerge(pr)
		}
	case "closed":
		mlog.Info("PR was closed", mlog.String("repo", *event.Repo.Name), mlog.Int("pr", event.PRNumber))
		go s.checkIfNeedCherryPick(pr)
		if result := <-s.Store.Spinmint().Get(pr.Number, pr.RepoName); result.Err != nil {
			mlog.Error("Unable to get the spinmint information.", mlog.String("pr_error", result.Err.Error()))
		} else if result.Data == nil {
			mlog.Info("Nothing to do. There is no Spinmint for this PR", mlog.Int("pr", pr.Number))
		} else {
			spinmint := result.Data.(*model.Spinmint)
			mlog.Info("Spinmint instance", mlog.String("spinmint", spinmint.InstanceId))
			mlog.Info("Will destroy the spinmint for a merged/closed PR.")

			s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number, s.Config.DestroyedSpinmintMessage)
			if strings.Contains(spinmint.InstanceId, "i-") {
				go s.destroySpinmint(pr, spinmint.InstanceId)
			}
		}
	}

	s.checkPullRequestForChanges(pr)
}

func (s *Server) checkPullRequestForChanges(pr *model.PullRequest) {
	result := <-s.Store.PullRequest().Get(pr.RepoOwner, pr.RepoName, pr.Number)
	if result.Err != nil {
		mlog.Error(result.Err.Error())
		return
	}

	if result.Data == nil {
		if resultSave := <-s.Store.PullRequest().Save(pr); resultSave.Err != nil {
			mlog.Error(resultSave.Err.Error())
		}

		for _, label := range pr.Labels {
			s.handlePRLabeled(pr, label)
		}

		return
	}

	oldPr := result.Data.(*model.PullRequest)
	prHasChanges := false

	for _, label := range pr.Labels {
		hadLabel := false

		for _, oldLabel := range oldPr.Labels {
			if label == oldLabel {
				hadLabel = true
				break
			}
		}

		if !hadLabel {
			s.handlePRLabeled(pr, label)
			prHasChanges = true
		}
	}

	for _, oldLabel := range oldPr.Labels {
		hasLabel := false

		for _, label := range pr.Labels {
			if label == oldLabel {
				hasLabel = true
				break
			}
		}

		if !hasLabel {
			s.handlePRUnlabeled(pr, oldLabel)
			prHasChanges = true
		}
	}

	if oldPr.Ref != pr.Ref {
		prHasChanges = true
	}

	if oldPr.Sha != pr.Sha {
		prHasChanges = true
	}

	if oldPr.State != pr.State {
		prHasChanges = true
	}

	if oldPr.BuildStatus != pr.BuildStatus {
		prHasChanges = true
	}

	if oldPr.BuildConclusion != pr.BuildConclusion {
		prHasChanges = true
	}

	if oldPr.BuildLink != pr.BuildLink {
		prHasChanges = true
	}

	if prHasChanges {
		mlog.Info("pr has changes", mlog.Int("pr", pr.Number))
		if result := <-s.Store.PullRequest().Save(pr); result.Err != nil {
			mlog.Error(result.Err.Error())
			return
		}
	}
}

func (s *Server) handlePRLabeled(pr *model.PullRequest, addedLabel string) {
	mlog.Info("New PR label detected", mlog.Int("pr", pr.Number), mlog.String("label", addedLabel))

	// Must be sure the comment is created before we let another request test
	s.commentLock.Lock()
	defer s.commentLock.Unlock()

	comments, _, err := NewGithubClient(s.Config.GithubAccessToken).Issues.ListComments(context.Background(), pr.RepoOwner, pr.RepoName, pr.Number, nil)
	if err != nil {
		mlog.Error("Unable to list comments for PR", mlog.Int("pr", pr.Number), mlog.Err(err))
		return
	}

	// Old comment created by Mattermod user for test server deletion will be deleted here
	for _, comment := range comments {
		if *comment.User.Login == s.Config.Username &&
			strings.Contains(*comment.Body, s.Config.DestroyedSpinmintMessage) || strings.Contains(*comment.Body, s.Config.DestroyedExpirationSpinmintMessage) {
			mlog.Info("Removing old server deletion comment with ID", mlog.Int64("ID", *comment.ID))
			_, err := NewGithubClient(s.Config.GithubAccessToken).Issues.DeleteComment(context.Background(), pr.RepoOwner, pr.RepoName, *comment.ID)
			if err != nil {
				mlog.Error("Unable to remove old server deletion comment", mlog.Err(err))
			}
		}
	}

	if addedLabel == s.Config.SetupSpinmintUpgradeTag && !messageByUserContains(comments, s.Config.Username, s.Config.SetupSpinmintUpgradeMessage) {
		mlog.Info("Label to spin a test server for upgrade")
		s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number, s.Config.SetupSpinmintUpgradeMessage)
		go s.waitForBuildAndSetupSpinmint(pr, true)
		// } else if addedLabel == s.Config.StartLoadtestTag {
		// 	mlog.Info("Label to spin a load test")
		// 	s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number, s.Config.StartLoadtestMessage)
		// 	go waitForBuildAndSetupLoadtest(pr)
	} else {
		mlog.Info("looking for other labels")

		for _, label := range s.Config.PrLabels {
			mlog.Info("looking for label", mlog.String("label", label.Label))
			finalMessage := strings.Replace(label.Message, "USERNAME", pr.Username, -1)
			if label.Label == addedLabel && !messageByUserContains(comments, s.Config.Username, finalMessage) {
				mlog.Info("Posted message for label on PR: ", mlog.String("label", label.Label), mlog.Int("pr", pr.Number))
				s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number, finalMessage)
			}
		}
	}
}

func checkFileExists(filepath string) bool {
	if _, err := os.Stat(filepath); err == nil {
		return true
	}
	return false
}

func (s *Server) handlePRUnlabeled(pr *model.PullRequest, removedLabel string) {
	s.commentLock.Lock()
	defer s.commentLock.Unlock()

	comments, err := s.getComments(pr.RepoOwner, pr.RepoName, pr.Number)
	if err != nil {
		mlog.Error("pr_error", mlog.Err(err))
		return
	}

	if s.isSpinMintLabel(removedLabel) &&
		(messageByUserContains(comments, s.Config.Username, s.Config.SetupSpinmintMessage) ||
			messageByUserContains(comments, s.Config.Username, s.Config.SetupSpinmintUpgradeMessage)) &&
		!messageByUserContains(comments, s.Config.Username, s.Config.DestroyedSpinmintMessage) {

		// Old comments created by Mattermod user will be deleted here.
		s.removeOldComments(comments, pr)

		if result := <-s.Store.Spinmint().Get(pr.Number, pr.RepoName); result.Err != nil {
			mlog.Error("Unable to get the test server information.", mlog.String("pr_error", result.Err.Error()))
		} else if result.Data == nil {
			mlog.Info("Nothing to do. There is not test server for this PR", mlog.Int("pr", pr.Number))
		} else {
			spinmint := result.Data.(*model.Spinmint)
			mlog.Info("test server instance", mlog.String("test server", spinmint.InstanceId))
			mlog.Info("Will destroy the test server for a merged/closed PR.")

			s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number, s.Config.DestroyedSpinmintMessage)
			go s.destroySpinmint(pr, spinmint.InstanceId)
		}
	}
}

func (s *Server) removeOldComments(comments []*github.IssueComment, pr *model.PullRequest) {
	serverMessages := []string{s.Config.SetupSpinmintMessage,
		s.Config.SetupSpinmintUpgradeMessage,
		s.Config.SetupSpinmintFailedMessage,
		"Spinmint test server created",
		"Spinmint upgrade test server created",
		"Error during the request to upgrade",
		"Error doing the upgrade process",
	}

	mlog.Info("Removing old Mattermod comments")
	for _, comment := range comments {
		if *comment.User.Login == s.Config.Username {
			for _, message := range serverMessages {
				if strings.Contains(*comment.Body, message) {
					mlog.Info("Removing old comment with ID", mlog.Int64("ID", *comment.ID))
					_, err := NewGithubClient(s.Config.GithubAccessToken).Issues.DeleteComment(context.Background(), pr.RepoOwner, pr.RepoName, *comment.ID)
					if err != nil {
						mlog.Error("Unable to remove old Mattermod comment", mlog.Err(err))
					}
					break
				}
			}
		}
	}
}

func (s *Server) CheckPRActivity() {
	mlog.Info("Checking if need to Stale a Pull request")
	var prs []*model.PullRequest
	result := <-s.Store.PullRequest().ListOpen()
	if result.Err != nil {
		mlog.Error(result.Err.Error())
		return
	}
	prs = result.Data.([]*model.PullRequest)

	client := NewGithubClient(s.Config.GithubAccessToken)
	for _, pr := range prs {
		pull, _, errPull := client.PullRequests.Get(context.Background(), pr.RepoOwner, pr.RepoName, pr.Number)
		if errPull != nil {
			mlog.Error("Error getting Pull Request", mlog.String("RepoOwner", pr.RepoOwner), mlog.String("RepoName", pr.RepoName), mlog.Int("PRNumber", pr.Number))
			break
		}

		if *pull.State == model.STATE_CLOSED {
			mlog.Info("PR/Issue is closed will not comment", mlog.String("RepoOwner", pr.RepoOwner), mlog.String("RepoName", pr.RepoName), mlog.Int("PRNumber", pr.Number), mlog.String("State", *pull.State))
			continue
		}

		// Only mark community contributions as stale
		isContributorOrgMember, err := s.isOrgMember(pr.RepoOwner, pr.Username)
		if err != nil {
			mlog.Error("Error getting org membership", mlog.Err(err), mlog.Int("PR", pr.Number), mlog.String("Repo", pr.RepoName))
			break
		}
		if isContributorOrgMember {
			continue
		}

		timeToStale := time.Now().AddDate(0, 0, -s.Config.DaysUntilStale)
		if timeToStale.After(*pull.UpdatedAt) || timeToStale.Equal(*pull.UpdatedAt) {
			var prLabels []string
			canStale := true
			labels, _, err := client.Issues.ListLabelsByIssue(context.Background(), pr.RepoOwner, pr.RepoName, pr.Number, nil)
			if err != nil {
				mlog.Error("Error getting the labels in the Pull Request", mlog.String("RepoOwner", pr.RepoOwner), mlog.String("RepoName", pr.RepoName), mlog.Int("PRNumber", pr.Number))
				continue
			}

			prLabels = labelsToStringArray(labels)
			for _, prLabel := range prLabels {
				for _, exemptStalelabel := range s.Config.ExemptStaleLabels {
					if prLabel == exemptStalelabel {
						canStale = false
						break
					}
				}
				if !canStale {
					break
				}
			}

			if canStale {
				label := []string{s.Config.StaleLabel}
				_, _, errLabel := client.Issues.AddLabelsToIssue(context.Background(), pr.RepoOwner, pr.RepoName, pr.Number, label)
				if errLabel != nil {
					mlog.Error("Error adding the stale labe in the  Pull Request", mlog.String("RepoOwner", pr.RepoOwner), mlog.String("RepoName", pr.RepoName), mlog.Int("PRNumber", pr.Number))
					break
				}
				s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number, s.Config.StaleComment)
			}
		}
	}
	mlog.Info("Finished checking if need to Stale a Pull request")
}

func (s *Server) CleanOutdatedPRs() {
	mlog.Info("Cleaning outdated prs in the mattermod database....")

	var prs []*model.PullRequest
	if result := <-s.Store.PullRequest().ListOpen(); result.Err != nil {
		mlog.Error(result.Err.Error())
		return
	} else {
		prs = result.Data.([]*model.PullRequest)
	}

	mlog.Info("Will process the PRs", mlog.Int("PRs Count", len(prs)))

	client := NewGithubClient(s.Config.GithubAccessToken)
	for _, pr := range prs {
		pull, _, errPull := client.PullRequests.Get(context.Background(), pr.RepoOwner, pr.RepoName, pr.Number)
		if errPull != nil {
			mlog.Error("Error getting Pull Request", mlog.String("RepoOwner", pr.RepoOwner), mlog.String("RepoName", pr.RepoName), mlog.Int("PRNumber", pr.Number), mlog.Err(errPull))
			if _, ok := errPull.(*github.RateLimitError); ok {
				mlog.Error("GitHub rate limit reached")
				s.CheckLimitRateAndSleep()
			}
		}

		if *pull.State == model.STATE_CLOSED {
			mlog.Info("PR is closed, updating the status in the database", mlog.String("RepoOwner", pr.RepoOwner), mlog.String("RepoName", pr.RepoName), mlog.Int("PRNumber", pr.Number))
			pr.State = *pull.State
			if result := <-s.Store.PullRequest().Save(pr); result.Err != nil {
				mlog.Error(result.Err.Error())
			}
		} else {
			mlog.Info("Nothing do to", mlog.String("RepoOwner", pr.RepoOwner), mlog.String("RepoName", pr.RepoName), mlog.Int("PRNumber", pr.Number))
		}

		time.Sleep(5 * time.Second)
	}
	mlog.Info("Finished update the outdated prs in the mattermod database....")
}

func (s *Server) isBlockPRMerge(label string) bool {
	for _, blocklabel := range s.Config.BlockPRMergeLabels {
		if label == blocklabel {
			return true
		}
	}
	return false
}

func (s *Server) isBlockPRMergeInLabels(labels []string) bool {
	for _, label := range labels {
		if s.isBlockPRMerge(label) {
			return true
		}
	}
	return false
}
