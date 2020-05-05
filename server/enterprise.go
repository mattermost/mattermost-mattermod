package server

import (
	"context"
	"fmt"
	"github.com/google/go-github/v28/github"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v5/mlog"
	"net/http"
	"regexp"
	"time"
)

func (s *Server) triggerEETestsForOrgMembers(pr *model.PullRequest) {
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	triggerInfo, err := s.getPRInfo(ctx, pr)
	if err != nil {
		s.createEnterpriseTestsErrorStatus(ctx, pr, err)
		return
	}

	isBaseBranchReleaseBranch, err := regexp.MatchString(`$release-*`, triggerInfo.BaseBranch)
	if err != nil {
		s.createEnterpriseTestsErrorStatus(ctx, pr, err)
		return
	}
	if triggerInfo.BaseBranch != "master" || !isBaseBranchReleaseBranch {
		s.succeedEEStatuses(ctx, pr, "base branch not master or release")
		return
	}

	mlog.Debug("Triggering ee tests", mlog.Int("pr", pr.Number), mlog.String("sha", pr.Sha))
	err = s.requestEETriggering(ctx, pr, triggerInfo)
	if err != nil {
		s.createEnterpriseTestsErrorStatus(ctx, pr, err)
		return
	}
}

type EETriggerInfo struct {
	BaseBranch   string
	ServerOwner  string
	ServerBranch string
	WebappOwner  string
	WebappBranch string
}

func (s *Server) getPRInfo(ctx context.Context, pr *model.PullRequest) (*EETriggerInfo, error) {
	clientGitHub := NewGithubClient(s.Config.GithubAccessToken)
	pullRequest, _, err := clientGitHub.PullRequests.Get(ctx, s.Config.Org, pr.RepoName, pr.Number)
	if err != nil {
		return nil, err
	}

	isFork := pullRequest.GetHead().GetRepo().GetFork()
	var serverOwner string
	if isFork {
		serverOwner = pullRequest.GetHead().GetUser().GetLogin()
	} else {
		serverOwner = s.Config.Org
	}
	if serverOwner == "" {
		return nil, fmt.Errorf("owner of server branch not found")
	}

	webappOwner, webappBranch, err := s.findWebappBranch(clientGitHub, ctx, pullRequest)
	if err != nil {
		return nil, err
	}
	info := &EETriggerInfo{
		BaseBranch:   *pullRequest.Base.Ref,
		ServerOwner:  serverOwner,
		ServerBranch: pr.Ref,
		WebappOwner:  webappOwner,
		WebappBranch: webappBranch,
	}
	return info, nil
}

func (s *Server) findWebappBranch(client *github.Client, ctx context.Context, serverPR *github.PullRequest) (string, string, error) {
	serverBranch := serverPR.GetHead().GetRef()
	destinationServerBranch := serverPR.GetBase().GetRef()

	owner, webappBranch, err := s.getWebappBranchWithSameName(client, ctx, serverPR)
	if err != nil {
		return "", "", err
	}

	if webappBranch == "" {
		mlog.Debug("Setting base ref of server as webapp branch", mlog.Int("pr", serverPR.GetNumber()), mlog.String("ref", serverBranch))
		return s.Config.Org, destinationServerBranch, nil
	}

	return owner, webappBranch, nil
}

func (s *Server) getWebappBranchWithSameName(client *github.Client, ctx context.Context, serverPR *github.PullRequest) (owner string, branch string, err error) {
	prAuthor := serverPR.GetUser().GetLogin()
	ref := serverPR.GetHead().GetRef()
	forkBranch, resp, err := client.Repositories.GetBranch(ctx, prAuthor, s.Config.EnterpriseWebappReponame, ref)
	if err != nil || (resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusOK) {
		return "", "", err
	}

	if resp.StatusCode == http.StatusNotFound {
		upstreamBranch, resp, err := client.Repositories.GetBranch(ctx, s.Config.Org, s.Config.EnterpriseWebappReponame, ref)
		if err != nil || (resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusOK) {
			return "", "", err
		}

		if resp.StatusCode == http.StatusNotFound {
			return s.Config.Org, serverPR.GetBase().GetRef(), nil
		}

		owner := s.Config.Org
		mlog.Debug("Found upstream webapp branch", mlog.String("owner", owner), mlog.String("branch", upstreamBranch.GetName()))
		return owner, upstreamBranch.GetName(), nil
	}

	mlog.Debug("Found webapp branch", mlog.String("owner", prAuthor), mlog.String("branch", forkBranch.GetName()))
	return prAuthor, forkBranch.GetName(), nil
}

func (s *Server) createEnterpriseTestsPendingStatus(ctx context.Context, pr *model.PullRequest) {
	enterpriseStatus := &github.RepoStatus{
		State:       github.String("pending"),
		Context:     github.String(s.Config.EnterpriseGithubStatusContext),
		Description: github.String("TODO as org member: After reviewing please trigger label \"" + s.Config.EnterpriseTriggerLabel + "\""),
		TargetURL:   github.String(""),
	}
	s.createRepoStatus(ctx, pr, enterpriseStatus)
}

func (s *Server) createEnterpriseTestsBlockedStatus(ctx context.Context, pr *model.PullRequest, description string) {
	enterpriseStatus := &github.RepoStatus{
		State:       github.String("pending"),
		Context:     github.String(s.Config.EnterpriseGithubStatusContext),
		Description: github.String(description),
		TargetURL:   github.String(""),
	}
	s.createRepoStatus(ctx, pr, enterpriseStatus)
}

func (s *Server) createEnterpriseTestsErrorStatus(ctx context.Context, pr *model.PullRequest, err error) {
	enterpriseErrorStatus := &github.RepoStatus{
		State:       github.String("error"),
		Context:     github.String(s.Config.EnterpriseGithubStatusContext),
		Description: github.String("Enterprise tests error"),
		TargetURL:   github.String(""),
	}
	s.createRepoStatus(ctx, pr, enterpriseErrorStatus)
	s.sendGitHubComment(pr.RepoOwner, pr.RepoName, pr.Number,
		"Failed running enterprise tests. @mattermost/core-build-engineers have been notified. Error:  \n```"+err.Error()+"```")
}

func (s *Server) succeedOutDatedJenkinsStatuses(ctx context.Context, pr *model.PullRequest) {
	enterpriseStatus := &github.RepoStatus{
		State:       github.String("success"),
		Context:     github.String("continuous-integration/jenkins/pr-merge"),
		Description: github.String("Outdated check"),
		TargetURL:   github.String(""),
	}
	s.createRepoStatus(ctx, pr, enterpriseStatus)
}

func (s *Server) succeedEEStatuses(ctx context.Context, pr *model.PullRequest, desc string) {
	eeTriggeredStatus := &github.RepoStatus{
		State:       github.String("success"),
		Context:     github.String(s.Config.EnterpriseGithubStatusContext),
		Description: github.String(desc),
		TargetURL:   github.String(""),
	}
	s.createRepoStatus(ctx, pr, eeTriggeredStatus)

	eeReportStatus := &github.RepoStatus{
		State:       github.String("success"),
		Context:     github.String(s.Config.EnterpriseGithubStatusEETests),
		Description: github.String(desc),
		TargetURL:   github.String(""),
	}
	s.createRepoStatus(ctx, pr, eeReportStatus)
}

func (s *Server) updateBuildStatus(ctx context.Context, pr *model.PullRequest, context string, targetUrl string) {
	status := &github.RepoStatus{
		State:       github.String("pending"),
		Context:     github.String(context),
		Description: github.String("Testing EE. SHA: " + pr.Sha),
		TargetURL:   github.String(targetUrl),
	}
	s.createRepoStatus(ctx, pr, status)
}
