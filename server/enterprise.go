package server

import (
	"context"
	"fmt"
	"github.com/google/go-github/v28/github"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v5/mlog"
	"net/http"
	"time"
)

// TODO: Use this function to check before running ee tests, if te tests are passing.
func (s *Server) arePRTETestsPassing(pr *model.PullRequest) (bool, error) {
	client := NewGithubClient(s.Config.GithubAccessToken)
	prStatuses, resp, err := client.Repositories.ListStatuses(context.Background(), pr.RepoOwner, pr.RepoName, pr.Ref, nil)
	if err != nil || resp.StatusCode != http.StatusOK {
		mlog.Error("Failed getting PRTETestsStatuses")
		return false, err
	}

	for _, status := range prStatuses {
		if *status.Context == s.Config.EnterpriseGithubStatusTETests &&
			*status.State == "success" {
			return true, nil
		}
	}
	return false, err
}

func (s *Server) createEnterpriseTestsPendingStatus(pr *model.PullRequest) {
	enterpriseStatus := &github.RepoStatus{
		State:       github.String("pending"),
		Context:     github.String(s.Config.EnterpriseGithubStatusContext),
		Description: github.String("TODO as org member: After reviewing please trigger label \"" + s.Config.EnterpriseTriggerLabel + "\""),
		TargetURL:   github.String(""),
	}
	s.createRepoStatus(pr, enterpriseStatus)
}

func (s *Server) createEnterpriseTestsBlockedStatus(pr *model.PullRequest, description string) {
	enterpriseStatus := &github.RepoStatus{
		State:       github.String("pending"),
		Context:     github.String(s.Config.EnterpriseGithubStatusContext),
		Description: github.String(description),
		TargetURL:   github.String(""),
	}
	s.createRepoStatus(pr, enterpriseStatus)
}

func (s *Server) createEnterpriseTestsErrorStatus(pr *model.PullRequest, err error) {
	enterpriseErrorStatus := &github.RepoStatus{
		State:       github.String("error"),
		Context:     github.String(s.Config.EnterpriseGithubStatusContext),
		Description: github.String("Enterprise tests error"),
		TargetURL:   github.String(""),
	}
	s.createRepoStatus(pr, enterpriseErrorStatus)
	s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number,
		"Failed running enterprise tests. @mattermost/core-build-engineers have been notified. Error:  \n```"+err.Error()+"```")
}
func (s *Server) triggerEETestsforOrgMembers(pr *model.PullRequest) {
	isOrgMember, err := s.isOrgMember(s.Config.Org, pr.Username)
	if err != nil {
		mlog.Error("Failed fetching org membership status")
		isOrgMember = false
	}
	if isOrgMember {
		s.triggerEnterpriseTests(pr)
	}
}

func (s *Server) triggerEnterpriseTests(pr *model.PullRequest) {
	externalBranch, eeBranch, err := s.getPRInfo(pr)
	if err != nil {
		s.createEnterpriseTestsErrorStatus(pr, err)
		return
	}

	mlog.Debug("Triggering ee tests with: ", mlog.String("eeRef", pr.Ref), mlog.String("triggerRef", pr.Ref), mlog.String("sha", pr.Sha))
	buildLink, err := s.triggerEnterprisePipeline(pr, eeBranch, externalBranch)
	if err != nil {
		s.createEnterpriseTestsErrorStatus(pr, err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	err = s.waitForStatus(ctx, pr, s.Config.EnterpriseGithubStatusContext, "success")
	if err != nil {
		s.createEnterpriseTestsErrorStatus(pr, err)
		return
	}

	s.updateBuildStatus(pr, s.Config.EnterpriseGithubStatusContext, buildLink)
}

// todo: adapt enterprise pipeline code so it already knows that it is a fork. This will make the enterprise pipeline code more readable.
// this is a hack to reproduce circleCI $CIRCLEBRANCH env variable, which is pull/PRNUMBER on a forked PR, but normal branchname on an upstream PR
func (s *Server) getPRInfo(pr *model.PullRequest) (string, string, error) {
	clientGitHub := NewGithubClient(s.Config.GithubAccessToken)
	pullRequest, _, err := clientGitHub.PullRequests.Get(context.Background(), s.Config.Org, pr.RepoName, pr.Number)
	if err != nil {
		return "", "", err
	}

	var externalBranch string
	if pullRequest.GetHead().GetRepo().GetFork() {
		externalBranch = fmt.Sprintf("pull/%d", pr.Number)
	} else {
		externalBranch = pr.Ref
	}
	return externalBranch, *pullRequest.Base.Ref, nil
}

func (s *Server) succeedOutDatedJenkinsStatuses(pr *model.PullRequest) {
	enterpriseStatus := &github.RepoStatus{
		State:       github.String("success"),
		Context:     github.String("continuous-integration/jenkins/pr-merge"),
		Description: github.String("Outdated check"),
		TargetURL:   github.String(""),
	}
	s.createRepoStatus(pr, enterpriseStatus)
}

func (s *Server) updateBuildStatus(pr *model.PullRequest, context string, targetUrl string) {
	status := &github.RepoStatus{
		State:       github.String("pending"),
		Context:     github.String(context),
		Description: github.String("Testing EE. SHA: " + pr.Sha),
		TargetURL:   github.String(targetUrl),
	}
	s.createRepoStatus(pr, status)
}
