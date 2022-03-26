// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/v43/github"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v5/mlog"
)

type pullRequestEvent struct {
	PullRequest   *github.PullRequest `json:"pull_request"`
	Issue         *github.Issue       `json:"issue"`
	Label         *github.Label       `json:"label"`
	Repo          *github.Repository  `json:"repository"`
	RepositoryURL string              `json:"repository_url"`
	Action        string              `json:"action"`
	PRNumber      int                 `json:"number"`
}

func (s *Server) pullRequestEventHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout*time.Second)
	defer cancel()

	event, err := pullRequestEventFromJSON(r.Body)
	if err != nil {
		mlog.Error("could not parse pr event", mlog.Err(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	mlog.Info("pr event", mlog.String("repo", event.Repo.GetName()), mlog.Int("pr", event.PRNumber), mlog.String("action", event.Action))

	pr, err := s.GetPullRequestFromGithub(ctx, event.PullRequest, event.Action)
	if err != nil {
		mlog.Error("Unable to get PR from GitHub", mlog.Int("pr", event.PRNumber), mlog.Err(err))
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	switch event.Action {
	case prEventOpened:
		mlog.Info("PR opened", mlog.String("repo", pr.RepoName), mlog.Int("pr", pr.Number))

		var claCommentNeeded bool
		claCommentNeeded, err = s.handleCheckCLA(ctx, pr)
		if err != nil {
			mlog.Error("Unable to check CLA", mlog.Err(err))
		}

		if err = s.triggerCircleCIIfNeeded(ctx, pr); err != nil {
			mlog.Error("Unable to trigger CircleCI", mlog.String("repo", pr.RepoName), mlog.Int("pr", pr.Number), mlog.String("fullname", pr.FullName), mlog.Err(err))
		} else {
			mlog.Debug("Triggered CircleCI", mlog.String("repo", pr.RepoName), mlog.Int("pr", pr.Number), mlog.String("fullname", pr.FullName))
		}

		if err = s.postPRWelcomeMessage(ctx, pr, claCommentNeeded); err != nil {
			mlog.Error("Error while commenting PR welcome message", mlog.Err(err))
		}

		s.addHacktoberfestLabel(ctx, pr)
		s.handleTranslationPR(ctx, pr)

		repo, repoExist := GetRepository(s.Config.Repositories, pr.RepoOwner, pr.RepoName)
		if repoExist {
			if err = s.assignGreeter(ctx, pr, repo); err != nil {
				mlog.Error("Error while assigning a greeter to the community PR", mlog.Err(err))
			}

			if err = s.assignGreetingLabels(ctx, pr, repo); err != nil {
				mlog.Error("Error while assigning labels to the community PR", mlog.Err(err))
			}
		}

		if pr.RepoName == s.Config.EnterpriseTriggerReponame {
			s.createEnterpriseTestsPendingStatus(ctx, pr)
			go s.triggerEETestsForOrgMembers(pr)
		}

		if pr.RepoName == serverRepoName {
			if err = s.reviewMlog(ctx, pr, event.PullRequest.GetNodeID(), event.PullRequest.GetDiffURL()); err != nil {
				mlog.Error("Error while reviewing mlog", mlog.Err(err))
			}
		}

		s.setBlockStatusForPR(ctx, pr)
	case "reopened":
		mlog.Info("PR reopened", mlog.String("repo", pr.RepoName), mlog.Int("pr", pr.Number))

		if _, err = s.handleCheckCLA(ctx, pr); err != nil {
			mlog.Error("Unable to check CLA", mlog.Err(err))
		}

		if err = s.triggerCircleCIIfNeeded(ctx, pr); err != nil {
			mlog.Error("Unable to trigger CircleCI", mlog.String("repo", pr.RepoName), mlog.Int("pr", pr.Number), mlog.String("fullname", pr.FullName), mlog.Err(err))
		} else {
			mlog.Debug("Triggered CircleCI", mlog.String("repo", pr.RepoName), mlog.Int("pr", pr.Number), mlog.String("fullname", pr.FullName))
		}

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
		if *event.Label.Name == s.Config.BuildAppTag {
			mlog.Info("Label to run mobile/desktop app build", mlog.Int("pr", event.PRNumber), mlog.String("repo", pr.RepoName), mlog.String("label", *event.Label.Name))
			repoOwner, repoName := pr.RepoOwner, pr.RepoName
			go s.buildApp(pr)

			s.removeLabel(ctx, repoOwner, repoName, pr.Number, s.Config.BuildAppTag)
		}

		if pr.RepoName == s.Config.EnterpriseTriggerReponame &&
			*event.Label.Name == s.Config.EnterpriseTriggerLabel {
			mlog.Info("Label to run ee tests", mlog.Int("pr", event.PRNumber), mlog.String("repo", pr.RepoName))
			go s.triggerEnterpriseTests(pr)

			s.removeLabel(ctx, pr.RepoOwner, pr.RepoName, pr.Number, s.Config.EnterpriseTriggerLabel)
		}

		if s.isBlockPRMerge(*event.Label.Name) {
			if err = s.unblockPRMerge(ctx, pr); err != nil {
				mlog.Error("Unable to create the github status for for PR", mlog.Int("pr", pr.Number), mlog.Err(err))
			}
		}
		if event.Label.GetName() == s.Config.AutoPRMergeLabel {
			msg := "Will try to auto merge this PR once all tests and checks are passing. This might take up to an hour."
			if err = s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, msg); err != nil {
				mlog.Warn("Error while commenting", mlog.Err(err))
			}
		}
	case "unlabeled":
		if event.Label == nil {
			mlog.Error("Unlabel event received, but label object was empty")
			return
		}

		if s.isBlockPRMerge(*event.Label.Name) {
			if err = s.unblockPRMerge(ctx, pr); err != nil {
				mlog.Error("Unable to create the github status for for PR", mlog.Int("pr", pr.Number), mlog.Err(err))
			}
		}
	case "synchronize":
		mlog.Debug("PR has a new commit", mlog.String("repo", pr.RepoName), mlog.Int("pr", pr.Number))

		if _, err = s.handleCheckCLA(ctx, pr); err != nil {
			mlog.Error("Unable to check CLA", mlog.Err(err))
		}

		if err = s.triggerCircleCIIfNeeded(ctx, pr); err != nil {
			mlog.Error("Unable to trigger CircleCI", mlog.String("repo", pr.RepoName), mlog.Int("pr", pr.Number), mlog.String("fullname", pr.FullName), mlog.Err(err))
		} else {
			mlog.Debug("Triggered CircleCI", mlog.String("repo", pr.RepoName), mlog.Int("pr", pr.Number), mlog.String("fullname", pr.FullName))
		}

		if pr.RepoName == s.Config.EnterpriseTriggerReponame {
			s.createEnterpriseTestsPendingStatus(ctx, pr)
			go s.triggerEETestsForOrgMembers(pr)
		}

		s.setBlockStatusForPR(ctx, pr)
	case "closed":
		mlog.Info("PR was closed", mlog.String("repo", *event.Repo.Name), mlog.Int("pr", event.PRNumber))
		go s.checkIfNeedCherryPick(pr)
		go s.CleanUpLabels(pr)
	}

	changed, err := s.checkPullRequestForChanges(ctx, pr)
	if err != nil {
		mlog.Error("Could not check changes for PR", mlog.Err(err))
	} else if changed {
		mlog.Info("pr has changes", mlog.Int("pr", pr.Number))
	}
}

func pullRequestEventFromJSON(data io.Reader) (*pullRequestEvent, error) {
	decoder := json.NewDecoder(data)
	var event pullRequestEvent
	if err := decoder.Decode(&event); err != nil {
		return nil, err
	}

	return &event, nil
}

func (s *Server) checkPullRequestForChanges(ctx context.Context, pr *model.PullRequest) (bool, error) {
	oldPr, err := s.Store.PullRequest().Get(pr.RepoOwner, pr.RepoName, pr.Number)
	if err != nil {
		return false, err
	}

	if oldPr == nil {
		if _, err := s.Store.PullRequest().Save(pr); err != nil {
			mlog.Error("could not save PR", mlog.Err(err))
		}

		for _, label := range pr.Labels {
			if err := s.handlePRLabeled(ctx, pr, label); err != nil {
				mlog.Error("could not handle labeled event", mlog.Err(err))
			}
		}

		return false, nil
	}

	compare := func(src, dst []string, f func(context.Context, *model.PullRequest, string) error) bool {
		for _, label := range src {
			hadLabel := false

			for _, oldLabel := range dst {
				if label == oldLabel {
					hadLabel = true
					break
				}
			}

			if !hadLabel {
				if err := f(ctx, pr, label); err != nil {
					mlog.Error("Could not handle PR labeled event", mlog.Err(err))
				}
				return true
			}
		}
		return false
	}

	prHasChanges := compare(pr.Labels, oldPr.Labels, s.handlePRLabeled) || compare(oldPr.Labels, pr.Labels, s.handlePRUnlabeled)

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

	if oldPr.GetMaintainerCanModify() != pr.GetMaintainerCanModify() {
		prHasChanges = true
	}

	if oldPr.GetMerged() != pr.GetMerged() {
		prHasChanges = true
	}

	if oldPr.GetMilestoneNumber() != pr.GetMilestoneNumber() {
		prHasChanges = true
	}

	if oldPr.GetMilestoneTitle() != pr.GetMilestoneTitle() {
		prHasChanges = true
	}

	if prHasChanges {
		if _, err := s.Store.PullRequest().Save(pr); err != nil {
			return true, fmt.Errorf("could not save PR: %w", err)
		}
		return true, nil
	}

	return prHasChanges, nil
}

func (s *Server) handlePRLabeled(ctx context.Context, pr *model.PullRequest, addedLabel string) error {
	mlog.Info("New PR label detected", mlog.Int("pr", pr.Number), mlog.String("label", addedLabel))

	// Must be sure the comment is created before we let another request test
	s.commentLock.Lock()
	defer s.commentLock.Unlock()

	comments, err := s.getComments(ctx, pr.RepoOwner, pr.RepoName, pr.Number)
	if err != nil {
		return fmt.Errorf("unable to list comments for PR: %w", err)
	}

	for _, label := range s.Config.PrLabels {
		mlog.Info("looking for label", mlog.String("label", label.Label))
		finalMessage := strings.ReplaceAll(label.Message, "USERNAME", pr.Username)
		if label.Label == addedLabel && !messageByUserContains(comments, s.Config.Username, finalMessage) {
			mlog.Info("Posted message for label on PR: ", mlog.String("label", label.Label), mlog.Int("pr", pr.Number))
			if err = s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, finalMessage); err != nil {
				mlog.Warn("Error while commenting", mlog.Err(err))
			}
		}
	}

	return nil
}

func (s *Server) handlePRUnlabeled(ctx context.Context, pr *model.PullRequest, removedLabel string) error {
	s.commentLock.Lock()
	defer s.commentLock.Unlock()

	return nil
}

func (s *Server) CheckPRActivity() {
	start := time.Now()
	mlog.Info("Checking if need to Stale a Pull request")
	ctx, cancel := context.WithTimeout(context.Background(), defaultCronTaskTimeout*time.Second)
	defer cancel()
	defer func() {
		elapsed := float64(time.Since(start)) / float64(time.Second)
		s.Metrics.ObserveCronTaskDuration("check_pr_activity", elapsed)
	}()
	prs, err := s.Store.PullRequest().ListOpen()
	if err != nil {
		mlog.Error(err.Error())
		s.Metrics.IncreaseCronTaskErrors("check_pr_activity")
		return
	}

	for _, pr := range prs {
		pull, _, errPull := s.GithubClient.PullRequests.Get(ctx, pr.RepoOwner, pr.RepoName, pr.Number)
		if errPull != nil {
			mlog.Error(
				"Error getting Pull Request",
				mlog.String("RepoOwner", pr.RepoOwner),
				mlog.String("RepoName", pr.RepoName),
				mlog.Int("PRNumber", pr.Number),
				mlog.Err(errPull),
			)
			continue
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
				mlog.Error(
					"Error getting the labels in the Pull Request",
					mlog.String("RepoOwner", pr.RepoOwner),
					mlog.String("RepoName", pr.RepoName),
					mlog.Int("PRNumber", pr.Number),
					mlog.Err(err),
				)
				s.Metrics.IncreaseCronTaskErrors("check_pr_activity")
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
					mlog.Error(
						"Error adding the stale label in the Pull Request",
						mlog.String("RepoOwner", pr.RepoOwner),
						mlog.String("RepoName", pr.RepoName),
						mlog.Int("PRNumber", pr.Number),
						mlog.Err(errLabel),
					)
					s.Metrics.IncreaseCronTaskErrors("check_pr_activity")
					continue
				}
				if err = s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, s.Config.StaleComment); err != nil {
					mlog.Warn("Error while commenting", mlog.Err(err))
				}
			}
		}
	}
	mlog.Info("Finished checking if need to Stale a Pull request")
}

func (s *Server) CleanOutdatedPRs() {
	mlog.Info("Cleaning outdated PRs in the mattermod database....")
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), defaultCronTaskTimeout*time.Second)
	defer cancel()
	defer func() {
		elapsed := float64(time.Since(start)) / float64(time.Second)
		s.Metrics.ObserveCronTaskDuration("clean_outdated_prs", elapsed)
	}()
	prs, err := s.Store.PullRequest().ListOpen()
	if err != nil {
		mlog.Error(err.Error())
		s.Metrics.IncreaseCronTaskErrors("clean_outdated_prs")
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
				s.Metrics.IncreaseCronTaskErrors("clean_outdated_prs")
			}
		} else {
			mlog.Info("Nothing to do", mlog.String("RepoOwner", pr.RepoOwner), mlog.String("RepoName", pr.RepoName), mlog.Int("PRNumber", pr.Number))
		}

		time.Sleep(200 * time.Millisecond)
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

func (s *Server) getPRFromIssueCommentEvent(ctx context.Context, event *issueCommentEvent) (*model.PullRequest, error) {
	prGitHub, _, err := s.GithubClient.PullRequests.Get(ctx,
		event.Repository.GetOwner().GetLogin(),
		event.Repository.GetName(),
		event.Issue.GetNumber(),
	)
	if err != nil {
		return nil, fmt.Errorf("could not get the latest PR information from github: %w", err)
	}

	pr, err := s.GetPullRequestFromGithub(ctx, prGitHub, event.Action)
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
