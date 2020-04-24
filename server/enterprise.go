package server

import (
	"context"
	"fmt"
	"github.com/google/go-github/v28/github"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v5/mlog"
)

func (s *Server) createEnterpriseTestsStatus(pr *model.PullRequest, status *github.RepoStatus) {
	client := NewGithubClient(s.Config.GithubAccessToken)
	_, _, err := client.Repositories.CreateStatus(context.Background(), pr.RepoOwner, pr.RepoName, pr.Sha, status)
	if err != nil {
		mlog.Error("Unable to create the github status for for PR", mlog.Int("pr", pr.Number), mlog.Err(err))
		return
	}
}

func (s *Server) createEnterpriseTestsPendingStatus(pr *model.PullRequest) {
	enterpriseStatus := &github.RepoStatus{
		State:       github.String("pending"),
		Context:     github.String(s.Config.EnterpriseGithubStatusContext),
		Description: github.String("TODO as org member: After reviewing please trigger label \"Run enterprise tests\""),
	}
	s.createEnterpriseTestsStatus(pr, enterpriseStatus)
}

func (s *Server) createEnterpriseTestsErrorStatus(pr *model.PullRequest, err error) {
	enterpriseErrorStatus := &github.RepoStatus{
		State:       github.String("error"),
		Context:     github.String(s.Config.EnterpriseGithubStatusContext),
		Description: github.String("Enterprise tests error"),
	}
	s.createEnterpriseTestsStatus(pr, enterpriseErrorStatus)
	s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number,
		"Failed running enterprise tests. @mattermost/core-build-engineers have been notified. Error:  \n```"+err.Error()+"```")
}

func (s *Server) triggerEnterpriseTests(pr *model.PullRequest) {
	externalBranch, err := s.getFakeEnvCircleBranch(pr)
	if err != nil {
		s.createEnterpriseTestsErrorStatus(pr, err)
		return
	}

	mlog.Debug("Triggering ee tests with: ", mlog.String("ref", pr.Ref), mlog.String("sha", pr.Sha))
	err = s.triggerEnterprisePipeline(s.Config.Org, s.Config.EnterpriseReponame, pr.Sha, externalBranch)
	if err != nil {
		s.createEnterpriseTestsErrorStatus(pr, err)
		return
	}

	enterpriseSuccessStatus := &github.RepoStatus{
		State:       github.String("success"),
		Context:     github.String(s.Config.EnterpriseGithubStatusContext),
		Description: github.String("Enterprise tests success"),
	}
	s.createEnterpriseTestsStatus(pr, enterpriseSuccessStatus)
}

// todo: adapt enterprise pipeline code so it already knows that it is a fork. This will make the enterprise pipeline code more readable.
// this is a hack to reproduce circleCI $CIRCLEBRANCH env variable, which is pull/PRNUMBER on a forked PR, but normal branchname on an upstream PR
func (s *Server) getFakeEnvCircleBranch(pr *model.PullRequest) (string, error) {
	clientGitHub := NewGithubClient(s.Config.GithubAccessToken)
	pullRequest, _, err := clientGitHub.PullRequests.Get(context.Background(), s.Config.Org, pr.RepoName, pr.Number)
	if err != nil {
		return "", err
	}

	var externalBranch string
	if pullRequest.GetBase().GetRepo().GetFork() {
		externalBranch = fmt.Sprintf("pull/%d", pr.Number)
	} else {
		externalBranch = pr.Ref
	}
	return externalBranch, nil
}

func (s *Server) succeedOutDatedJenkinsStatuses(pr *model.PullRequest) {
	enterpriseStatus := &github.RepoStatus{
		State:       github.String("success"),
		Context:     github.String("continuous-integration/jenkins/pr-merge"),
		Description: github.String("Outdated check"),
	}
	s.createEnterpriseTestsStatus(pr, enterpriseStatus)
}
