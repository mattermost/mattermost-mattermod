// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/google/go-github/github"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/mlog"
)

func handlePullRequestEvent(event *PullRequestEvent) {
	pr, err := GetPullRequestFromGithub(event.PullRequest)
	if err != nil {
		mlog.Error("pr_error", mlog.Err(err))
		return
	}

	if event.Action == "closed" {
		if result := <-Srv.Store.Spinmint().Get(pr.Number); result.Err != nil {
			mlog.Error("Unable to get the spinmint information: Maybe does not exist.", mlog.String("pr_error", result.Err.Error()))
		} else if result.Data == nil {
			mlog.Info("Nothing to do. There is not Spinmint for this PR", mlog.Int("pr", pr.Number))
		} else {
			spinmint := result.Data.(*model.Spinmint)
			mlog.Info("Spinmint instance", mlog.String("spinmint", spinmint.InstanceId))
			mlog.Info("Will destroy the spinmint for a merged/closed PR.")

			commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, Config.DestroyedSpinmintMessage)
			go destroySpinmint(pr, spinmint.InstanceId)
		}
	} else if event.Action == "synchronize" {
		checkCLA(pr)
	}

	checkPullRequestForChanges(pr)
}

func checkPullRequestForChanges(pr *model.PullRequest) {
	var oldPr *model.PullRequest
	if result := <-Srv.Store.PullRequest().Get(pr.RepoOwner, pr.RepoName, pr.Number); result.Err != nil {
		mlog.Error(result.Err.Error())
		return
	} else if result.Data == nil {
		if resultSave := <-Srv.Store.PullRequest().Save(pr); resultSave.Err != nil {
			mlog.Error(resultSave.Err.Error())
		}

		handlePROpened(pr)

		for _, label := range pr.Labels {
			handlePRLabeled(pr, label)
		}
		return
	} else {
		oldPr = result.Data.(*model.PullRequest)
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
			handlePRLabeled(pr, label)
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
			handlePRUnlabeled(pr, oldLabel)
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

	if oldPr.BuildLink != pr.BuildLink {
		prHasChanges = true
	}

	if prHasChanges {
		mlog.Info("pr has changes", mlog.Int("pr", pr.Number))
		if result := <-Srv.Store.PullRequest().Save(pr); result.Err != nil {
			mlog.Error(result.Err.Error())
			return
		}
	}
}

func handlePROpened(pr *model.PullRequest) {
	checkCLA(pr)
}

func handlePRLabeled(pr *model.PullRequest, addedLabel string) {
	mlog.Info("labeled PR with label", mlog.Int("pr", pr.Number), mlog.String("label", addedLabel))

	// Must be sure the comment is created before we let anouther request test
	commentLock.Lock()
	defer commentLock.Unlock()

	comments, _, err := NewGithubClient().Issues.ListComments(pr.RepoOwner, pr.RepoName, pr.Number, nil)
	if err != nil {
		mlog.Error("Unable to list comments for PR", mlog.Int("pr", pr.Number), mlog.Err(err))
		return
	}

	// Old comment created by Mattermod user for test server deletion will be deleted here
	for _, comment := range comments {
		if *comment.User.Login == Config.Username &&
			strings.Contains(*comment.Body, Config.DestroyedSpinmintMessage) || strings.Contains(*comment.Body, Config.DestroyedExpirationSpinmintMessage) {
			mlog.Info("Removing old server deletion comment with ID", mlog.Int("ID", *comment.ID))
			_, err := NewGithubClient().Issues.DeleteComment(pr.RepoOwner, pr.RepoName, *comment.ID)
			if err != nil {
				mlog.Error("Unable to remove old server deletion comment", mlog.Err(err))
			}
		}
	}

	if addedLabel == Config.SetupSpinmintTag && !messageByUserContains(comments, Config.Username, Config.SetupSpinmintMessage) {
		mlog.Info("Label to spin a test server")
		commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, Config.SetupSpinmintMessage)
		go waitForBuildAndSetupSpinmint(pr, false)
	} else if addedLabel == Config.SetupSpinmintUpgradeTag && !messageByUserContains(comments, Config.Username, Config.SetupSpinmintUpgradeMessage) {
		mlog.Info("Label to spin a test server for upgrade")
		commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, Config.SetupSpinmintUpgradeMessage)
		go waitForBuildAndSetupSpinmint(pr, true)
	} else if addedLabel == Config.BuildMobileAppTag && !messageByUserContains(comments, Config.Username, Config.BuildMobileAppInitMessage) {
		mlog.Info("Label to build the mobile app")
		commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, Config.BuildMobileAppInitMessage)
		go waitForMobileAppsBuild(pr)
	} else if addedLabel == Config.StartLoadtestTag {
		mlog.Info("Label to spin a load test")
		commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, Config.StartLoadtestMessage)
		go waitForBuildAndSetupLoadtest(pr)
	} else {
		mlog.Info("looking for other labels")

		for _, label := range Config.PrLabels {
			mlog.Info("looking for label", mlog.String("label", label.Label))
			finalMessage := strings.Replace(label.Message, "USERNAME", pr.Username, -1)
			if label.Label == addedLabel && !messageByUserContains(comments, Config.Username, finalMessage) {
				mlog.Info("Posted message for label on PR: ", mlog.String("label", label.Label), mlog.Int("pr", pr.Number))
				commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, finalMessage)
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

func handlePRUnlabeled(pr *model.PullRequest, removedLabel string) {
	commentLock.Lock()
	defer commentLock.Unlock()

	comments, _, err := NewGithubClient().Issues.ListComments(pr.RepoOwner, pr.RepoName, pr.Number, nil)
	if err != nil {
		mlog.Error("pr_error", mlog.Err(err))
		return
	}

	if (removedLabel == Config.SetupSpinmintTag || removedLabel == Config.SetupSpinmintUpgradeTag) &&
		(messageByUserContains(comments, Config.Username, Config.SetupSpinmintMessage) || messageByUserContains(comments, Config.Username, Config.SetupSpinmintUpgradeMessage)) &&
		!messageByUserContains(comments, Config.Username, Config.DestroyedSpinmintMessage) {

		upgrade := false
		if removedLabel == Config.SetupSpinmintUpgradeTag {
			upgrade = true
		}

		var instanceId string
		for _, comment := range comments {
			if isSpinmintDoneComment(*comment.Body, upgrade) {
				match := INSTANCE_ID_PATTERN.FindStringSubmatch(*comment.Body)
				instanceId = match[1]
				break
			}
		}

		// Old comments created by Mattermod user will be deleted here.
		removeOldComments(comments, pr)

		if instanceId != "" {
			commentOnIssue(pr.RepoOwner, pr.RepoName, pr.Number, Config.DestroyedSpinmintMessage)

			go destroySpinmint(pr, instanceId)
		}
	}
}

func removeOldComments(comments []*github.IssueComment, pr *model.PullRequest) {
	serverMessages := []string{Config.SetupSpinmintMessage,
		Config.SetupSpinmintUpgradeMessage,
		"Spinmint test server created",
		"Spinmint upgrade test server created",
		Config.SetupSpinmintFailedMessage}

	mlog.Info("Removing old Mattermod comments")
	for _, comment := range comments {
		if *comment.User.Login == Config.Username {
			for _, message := range serverMessages {
				if strings.Contains(*comment.Body, message) {
					mlog.Info("Removing old comment with ID", mlog.Int("ID", *comment.ID))
					_, err := NewGithubClient().Issues.DeleteComment(pr.RepoOwner, pr.RepoName, *comment.ID)
					if err != nil {
						mlog.Error("Unable to remove old Mattermod comment", mlog.Err(err))
					}
					break
				}
			}
		}
	}
}

func isSpinmintDoneComment(message string, upgrade bool) bool {
	var spinmintDoneMessage string
	if upgrade {
		spinmintDoneMessage = regexp.QuoteMeta(Config.SetupSpinmintUpgradeDoneMessage)
	} else {
		spinmintDoneMessage = regexp.QuoteMeta(Config.SetupSpinmintDoneMessage)
	}
	spinmintDoneMessage = strings.Replace(spinmintDoneMessage, SPINMINT_LINK, ".*", -1)
	spinmintDoneMessage = strings.Replace(spinmintDoneMessage, INSTANCE_ID, INSTANCE_ID_PATTERN.String(), -1)

	pattern := regexp.MustCompile(spinmintDoneMessage)
	return pattern.MatchString(message)
}
func CleanOutdatedPRs() {
	mlog.Info("Cleaning outdated prs in the mattermod database....")

	var prs []*model.PullRequest
	if result := <-Srv.Store.PullRequest().ListOpen(); result.Err != nil {
		mlog.Error(result.Err.Error())
		return
	} else {
		prs = result.Data.([]*model.PullRequest)
	}

	mlog.Info("Will process the PRs", mlog.Int("PRs Count", len(prs)))

	client := NewGithubClient()
	for _, pr := range prs {
		pull, _, errPull := client.PullRequests.Get(pr.RepoOwner, pr.RepoName, pr.Number)
		if errPull != nil {
			mlog.Error("Error getting Pull Request", mlog.String("RepoOwner", pr.RepoOwner), mlog.String("RepoName", pr.RepoName), mlog.Int("PRNumber", pr.Number), mlog.Err(errPull))
			if _, ok := errPull.(*github.RateLimitError); ok {
				mlog.Error("GitHub rate limit reached")
				CheckLimitRateAndSleep()
			}
		}

		if *pull.State == "closed" {
			mlog.Info("PR is closed, updating the status in the database", mlog.String("RepoOwner", pr.RepoOwner), mlog.String("RepoName", pr.RepoName), mlog.Int("PRNumber", pr.Number))
			pr.State = *pull.State
			if result := <-Srv.Store.PullRequest().Save(pr); result.Err != nil {
				mlog.Error(result.Err.Error())
			}
		} else {
			mlog.Info("Nothing do to", mlog.String("RepoOwner", pr.RepoOwner), mlog.String("RepoName", pr.RepoName), mlog.Int("PRNumber", pr.Number))
		}

		time.Sleep(5 * time.Second)
	}
	mlog.Info("Finished update the outdated prs in the mattermod database....")
}
