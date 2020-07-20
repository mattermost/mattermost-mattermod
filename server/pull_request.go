// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/v32/github"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v5/mlog"
)

func (s *Server) handlePullRequestEvent(ctx context.Context, event *PullRequestEvent) {
	mlog.Info("PR-Event", mlog.String("repo", *event.Repo.Name), mlog.Int("pr", event.PRNumber), mlog.String("action", event.Action))
	pr, err := s.GetPullRequestFromGithub(ctx, event.PullRequest)
	if err != nil {
		mlog.Error("Unable to get PR from GitHub", mlog.Int("pr", event.PRNumber), mlog.Err(err))
		return
	}

	switch event.Action {
	case "opened":
		mlog.Info("PR opened", mlog.String("repo", pr.RepoName), mlog.Int("pr", pr.Number))
		if err := s.handleCheckCLA(ctx, pr); err != nil {
			mlog.Error("Unable to check CLA", mlog.Err(err))
		}
		s.triggerCircleCiIfNeeded(ctx, pr)
		s.addHacktoberfestLabel(ctx, pr)
		s.handleTranslationPR(ctx, pr)

		if pr.RepoName == s.Config.EnterpriseTriggerReponame {
			s.createEnterpriseTestsPendingStatus(ctx, pr)
			go s.triggerEETestsForOrgMembers(pr)
		}

		s.setBlockStatusForPR(ctx, pr)
	case "reopened":
		mlog.Info("PR reopened", mlog.String("repo", pr.RepoName), mlog.Int("pr", pr.Number))
		if err := s.handleCheckCLA(ctx, pr); err != nil {
			mlog.Error("Unable to check CLA", mlog.Err(err))
		}
		s.triggerCircleCiIfNeeded(ctx, pr)
		s.handleTranslationPR(ctx, pr)

		if pr.RepoName == s.Config.EnterpriseTriggerReponame {
			s.createEnterpriseTestsPendingStatus(ctx, pr)
			go s.triggerEETestsForOrgMembers(pr)
		}

		s.setBlockStatusForPR(ctx, pr)
	case "labeled":
		if event.Label == nil {
			mlog.Error("Label event received, but label object was empty")
			return
		}
		if *event.Label.Name == s.Config.BuildMobileAppTag {
			mlog.Info("Label to run mobile build", mlog.Int("pr", event.PRNumber), mlog.String("repo", pr.RepoName), mlog.String("label", *event.Label.Name))
			mobileRepoOwner, mobileRepoName := pr.RepoOwner, pr.RepoName
			go s.buildMobileApp(pr)

			s.removeLabel(ctx, mobileRepoOwner, mobileRepoName, pr.Number, s.Config.BuildMobileAppTag)
		}

		if pr.RepoName == s.Config.EnterpriseTriggerReponame &&
			*event.Label.Name == s.Config.EnterpriseTriggerLabel {
			mlog.Info("Label to run ee tests", mlog.Int("pr", event.PRNumber), mlog.String("repo", pr.RepoName))
			go s.triggerEnterpriseTests(pr)

			s.removeLabel(ctx, pr.RepoOwner, pr.RepoName, pr.Number, s.Config.EnterpriseTriggerLabel)
		}

		// TODO: remove the old test server code
		if event.Label.GetName() == s.Config.SetupSpinmintTag {
			mlog.Info("Label to spin a old test server")
			s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, s.Config.SetupSpinmintMessage)
			go s.waitForBuildAndSetupSpinmint(pr, false)
		}
		if s.isBlockPRMerge(*event.Label.Name) {
			if err := s.unblockPRMerge(ctx, pr); err != nil {
				mlog.Error("Unable to create the github status for for PR", mlog.Int("pr", pr.Number), mlog.Err(err))
			}
		}
		if s.hasAutoMerge(pr.Labels) {
			msg := "Will try to auto merge this PR once all tests and checks are passing. This might take up to an hour."
			s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, msg)
		}
	case "unlabeled":
		if event.Label == nil {
			mlog.Error("Unlabel event received, but label object was empty")
			return
		}

		if s.isBlockPRMerge(*event.Label.Name) {
			if err := s.unblockPRMerge(ctx, pr); err != nil {
				mlog.Error("Unable to create the github status for for PR", mlog.Int("pr", pr.Number), mlog.Err(err))
			}
		}

		// TODO: remove the old test server code
		if s.isSpinMintLabel(*event.Label.Name) {
			spinmint, err := s.Store.Spinmint().Get(pr.Number, pr.RepoName)
			if err != nil {
				mlog.Error("Unable to get the test server information.", mlog.String("pr_error", err.Error()))
				break
			}

			if spinmint == nil {
				mlog.Info("Nothing to do. There is no test server for this PR", mlog.Int("pr", pr.Number))
				break
			}

			mlog.Info("test server instance", mlog.String("test server", spinmint.InstanceID))
			mlog.Info("Will destroy the test server for a merged/closed PR.")
			s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, s.Config.DestroyedSpinmintMessage)
			go s.destroySpinmint(pr, spinmint.InstanceID)
		}
	case "synchronize":
		mlog.Debug("PR has a new commit", mlog.String("repo", pr.RepoName), mlog.Int("pr", pr.Number))
		if err := s.handleCheckCLA(ctx, pr); err != nil {
			mlog.Error("Unable to check CLA", mlog.Err(err))
		}
		s.triggerCircleCiIfNeeded(ctx, pr)

		if pr.RepoName == s.Config.EnterpriseTriggerReponame {
			s.createEnterpriseTestsPendingStatus(ctx, pr)
			go s.triggerEETestsForOrgMembers(pr)
		}

		s.setBlockStatusForPR(ctx, pr)
	case "closed":
		mlog.Info("PR was closed", mlog.String("repo", *event.Repo.Name), mlog.Int("pr", event.PRNumber))
		go s.checkIfNeedCherryPick(pr)
		go s.CleanUpLabels(pr)

		spinmint, err := s.Store.Spinmint().Get(pr.Number, pr.RepoName)
		if err != nil {
			mlog.Error("Unable to get the spinmint information.", mlog.String("pr_error", err.Error()))
			break
		}

		if spinmint == nil {
			mlog.Info("Nothing to do. There is no Spinmint for this PR", mlog.Int("pr", pr.Number))
			break
		}

		mlog.Info("Spinmint instance", mlog.String("spinmint", spinmint.InstanceID))
		mlog.Info("Will destroy the spinmint for a merged/closed PR.")

		s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, s.Config.DestroyedSpinmintMessage)
		if strings.Contains(spinmint.InstanceID, "i-") {
			go s.destroySpinmint(pr, spinmint.InstanceID)
		}
	}

	s.checkPullRequestForChanges(ctx, pr)
}

func (s *Server) checkPullRequestForChanges(ctx context.Context, pr *model.PullRequest) {
	oldPr, err := s.Store.PullRequest().Get(pr.RepoOwner, pr.RepoName, pr.Number)
	if err != nil {
		mlog.Error(err.Error())
		return
	}

	if oldPr == nil {
		if _, err := s.Store.PullRequest().Save(pr); err != nil {
			mlog.Error(err.Error())
		}

		for _, label := range pr.Labels {
			s.handlePRLabeled(ctx, pr, label)
		}

		return
	}

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
			s.handlePRLabeled(ctx, pr, label)
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
			s.handlePRUnlabeled(ctx, pr, oldLabel)
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

	if !oldPr.MaintainerCanModify.Valid || (oldPr.MaintainerCanModify.Bool != pr.MaintainerCanModify.Bool) {
		prHasChanges = true
	}

	if !oldPr.Merged.Valid || (oldPr.Merged.Bool != pr.Merged.Bool) {
		prHasChanges = true
	}

	if prHasChanges {
		mlog.Info("pr has changes", mlog.Int("pr", pr.Number))
		if _, err := s.Store.PullRequest().Save(pr); err != nil {
			mlog.Error(err.Error())
			return
		}
	}
}

func (s *Server) handlePRLabeled(ctx context.Context, pr *model.PullRequest, addedLabel string) {
	mlog.Info("New PR label detected", mlog.Int("pr", pr.Number), mlog.String("label", addedLabel))

	// Must be sure the comment is created before we let another request test
	s.commentLock.Lock()
	defer s.commentLock.Unlock()

	comments, _, err := s.GithubClient.Issues.ListComments(ctx, pr.RepoOwner, pr.RepoName, pr.Number, nil)
	if err != nil {
		mlog.Error("Unable to list comments for PR", mlog.Int("pr", pr.Number), mlog.Err(err))
		return
	}

	// Old comment created by Mattermod user for test server deletion will be deleted here
	for _, comment := range comments {
		if *comment.User.Login == s.Config.Username &&
			strings.Contains(*comment.Body, s.Config.DestroyedSpinmintMessage) || strings.Contains(*comment.Body, s.Config.DestroyedExpirationSpinmintMessage) {
			mlog.Info("Removing old server deletion comment with ID", mlog.Int64("ID", *comment.ID))
			_, err := s.GithubClient.Issues.DeleteComment(ctx, pr.RepoOwner, pr.RepoName, *comment.ID)
			if err != nil {
				mlog.Error("Unable to remove old server deletion comment", mlog.Err(err))
			}
		}
	}

	if addedLabel == s.Config.SetupSpinmintUpgradeTag && !messageByUserContains(comments, s.Config.Username, s.Config.SetupSpinmintUpgradeMessage) {
		mlog.Info("Label to spin a test server for upgrade")
		s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, s.Config.SetupSpinmintUpgradeMessage)
		go s.waitForBuildAndSetupSpinmint(pr, true)
	} else {
		mlog.Info("looking for other labels")

		for _, label := range s.Config.PrLabels {
			mlog.Info("looking for label", mlog.String("label", label.Label))
			finalMessage := strings.Replace(label.Message, "USERNAME", pr.Username, -1)
			if label.Label == addedLabel && !messageByUserContains(comments, s.Config.Username, finalMessage) {
				mlog.Info("Posted message for label on PR: ", mlog.String("label", label.Label), mlog.Int("pr", pr.Number))
				s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, finalMessage)
			}
		}
	}
}

func (s *Server) handlePRUnlabeled(ctx context.Context, pr *model.PullRequest, removedLabel string) {
	s.commentLock.Lock()
	defer s.commentLock.Unlock()

	comments, err := s.getComments(ctx, pr)
	if err != nil {
		mlog.Error("failed fetching comments", mlog.Err(err))
		return
	}

	if s.isSpinMintLabel(removedLabel) &&
		(messageByUserContains(comments, s.Config.Username, s.Config.SetupSpinmintMessage) ||
			messageByUserContains(comments, s.Config.Username, s.Config.SetupSpinmintUpgradeMessage)) &&
		!messageByUserContains(comments, s.Config.Username, s.Config.DestroyedSpinmintMessage) {
		// Old comments created by Mattermod user will be deleted here.
		s.removeOldComments(ctx, comments, pr)

		spinmint, err := s.Store.Spinmint().Get(pr.Number, pr.RepoName)
		if err != nil {
			mlog.Error("Unable to get the test server information.", mlog.String("pr_error", err.Error()))
			return
		}

		if spinmint == nil {
			mlog.Info("Nothing to do. There is not test server for this PR", mlog.Int("pr", pr.Number))
			return
		}

		mlog.Info("test server instance", mlog.String("test server", spinmint.InstanceID))
		mlog.Info("Will destroy the test server for a merged/closed PR.")

		s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, s.Config.DestroyedSpinmintMessage)
		go s.destroySpinmint(pr, spinmint.InstanceID)
	}
}

func (s *Server) removeOldComments(ctx context.Context, comments []*github.IssueComment, pr *model.PullRequest) {
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
					_, err := s.GithubClient.Issues.DeleteComment(ctx, pr.RepoOwner, pr.RepoName, *comment.ID)
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
	ctx, cancel := context.WithTimeout(context.Background(), defaultCronTaskTimeout*time.Second)
	defer cancel()
	prs, err := s.Store.PullRequest().ListOpen()
	if err != nil {
		mlog.Error(err.Error())
		return
	}

	for _, pr := range prs {
		pull, _, errPull := s.GithubClient.PullRequests.Get(ctx, pr.RepoOwner, pr.RepoName, pr.Number)
		if errPull != nil {
			mlog.Error("Error getting Pull Request", mlog.String("RepoOwner", pr.RepoOwner), mlog.String("RepoName", pr.RepoName), mlog.Int("PRNumber", pr.Number))
			break
		}

		if *pull.State == model.StateClosed {
			mlog.Info("PR/Issue is closed will not comment", mlog.String("RepoOwner", pr.RepoOwner), mlog.String("RepoName", pr.RepoName), mlog.Int("PRNumber", pr.Number), mlog.String("State", *pull.State))
			continue
		}

		// Only mark community contributions as stale
		if s.IsOrgMember(pr.Username) {
			continue
		}

		timeToStale := time.Now().AddDate(0, 0, -s.Config.DaysUntilStale)
		if timeToStale.After(*pull.UpdatedAt) || timeToStale.Equal(*pull.UpdatedAt) {
			var prLabels []string
			canStale := true
			labels, _, err := s.GithubClient.Issues.ListLabelsByIssue(ctx, pr.RepoOwner, pr.RepoName, pr.Number, nil)
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
				_, _, errLabel := s.GithubClient.Issues.AddLabelsToIssue(ctx, pr.RepoOwner, pr.RepoName, pr.Number, label)
				if errLabel != nil {
					mlog.Error("Error adding the stale labe in the  Pull Request", mlog.String("RepoOwner", pr.RepoOwner), mlog.String("RepoName", pr.RepoName), mlog.Int("PRNumber", pr.Number))
					break
				}
				s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, s.Config.StaleComment)
			}
		}
	}
	mlog.Info("Finished checking if need to Stale a Pull request")
}

func (s *Server) CleanOutdatedPRs() {
	mlog.Info("Cleaning outdated PRs in the mattermod database....")

	ctx, cancel := context.WithTimeout(context.Background(), defaultCronTaskTimeout*time.Second)
	defer cancel()
	prs, err := s.Store.PullRequest().ListOpen()
	if err != nil {
		mlog.Error(err.Error())
		return
	}

	mlog.Info("Processing PRs", mlog.Int("PRs Count", len(prs)))

	for _, pr := range prs {
		pull, _, err := s.GithubClient.PullRequests.Get(ctx, pr.RepoOwner, pr.RepoName, pr.Number)
		if _, ok := err.(*github.RateLimitError); ok {
			return
		}
		if err != nil {
			mlog.Error("Error getting PR", mlog.String("RepoOwner", pr.RepoOwner), mlog.String("RepoName", pr.RepoName), mlog.Int("PRNumber", pr.Number), mlog.Err(err))
			continue
		}

		if pull.GetState() == model.StateClosed {
			mlog.Info("PR is closed, updating the status in the database", mlog.String("RepoOwner", pr.RepoOwner), mlog.String("RepoName", pr.RepoName), mlog.Int("PRNumber", pr.Number))
			pr.State = pull.GetState()
			if _, err := s.Store.PullRequest().Save(pr); err != nil {
				mlog.Error(err.Error())
			}
		} else {
			mlog.Info("Nothing to do", mlog.String("RepoOwner", pr.RepoOwner), mlog.String("RepoName", pr.RepoName), mlog.Int("PRNumber", pr.Number))
		}

		time.Sleep(5 * time.Second)
	}
	mlog.Info("Finished update the outdated prs in the mattermod database....")
}

func (s *Server) CleanUpLabels(pr *model.PullRequest) {
	if len(s.Config.IssueLabelsToCleanUp) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout*time.Second)
	defer cancel()
	labels, _, err := s.GithubClient.Issues.ListLabelsByIssue(ctx, pr.RepoOwner, pr.RepoName, pr.Number, nil)
	if err != nil {
		mlog.Error("Error listing the labels for closed PR", mlog.Err(err))
		return
	}

	var wg sync.WaitGroup

	for _, l := range labels {
		for _, labelToRemove := range s.Config.IssueLabelsToCleanUp {
			if l.GetName() == labelToRemove {
				wg.Add(1)
				go func(label string) {
					defer wg.Done()
					s.removeLabel(ctx, pr.RepoOwner, pr.RepoName, pr.Number, label)
				}(labelToRemove)
				continue
			}
		}
	}

	wg.Wait()
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

func (s *Server) getPRFromEvent(ctx context.Context, event EventData) (*model.PullRequest, error) {
	if event.Issue == nil || event.Repository == nil {
		return nil, errors.New("either issue or repository field is missing from the event data")
	}

	prGitHub, _, err := s.GithubClient.PullRequests.Get(ctx,
		*event.Repository.Owner.Login,
		*event.Repository.Name,
		*event.Issue.Number,
	)
	if err != nil {
		return nil, fmt.Errorf("could not get the latest PR information from github: %w", err)
	}

	pr, err := s.GetPullRequestFromGithub(ctx, prGitHub)
	if err != nil {
		return nil, fmt.Errorf("error updating the PR in the DB: %w", err)
	}
	return pr, nil
}

func (s *Server) setBlockStatusForPR(ctx context.Context, pr *model.PullRequest) {
	var err error
	if s.isBlockPRMergeInLabels(pr.Labels) {
		err = s.blockPRMerge(ctx, pr)
	} else {
		err = s.unblockPRMerge(ctx, pr)
	}
	if err != nil {
		mlog.Error("Unable to create the github status for for PR", mlog.Int("pr", pr.Number), mlog.Err(err))
	}
}
