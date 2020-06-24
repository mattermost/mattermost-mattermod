package server

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/google/go-github/v31/github"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v5/mlog"
)

func (s *Server) triggerEETestsForOrgMembers(pr *model.PullRequest) {
	if s.IsOrgMember(pr.Username) {
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

	isBaseBranchReleaseBranch, err := regexp.MatchString(`^release-*`, triggerInfo.BaseBranch)
	if err != nil {
		s.createEnterpriseTestsErrorStatus(ctx, pr, err)
		return
	}
	if triggerInfo.BaseBranch != "master" && !isBaseBranchReleaseBranch {
		mlog.Debug("Succeeding ee statuses", mlog.Int("pr", pr.Number), mlog.String("base ref", triggerInfo.BaseBranch))
		s.succeedEEStatuses(ctx, pr, "base branch not master or release")
		return
	}

	err = s.requestEETriggering(ctx, pr, triggerInfo)
	if err != nil {
		s.createEnterpriseTestsErrorStatus(ctx, pr, err)
		return
	}
}

type EETriggerInfo struct {
	BaseBranch   string
	EEBranch     string
	ServerOwner  string
	ServerBranch string
	WebappOwner  string
	WebappBranch string
}

func (s *Server) getPRInfo(ctx context.Context, pr *model.PullRequest) (info *EETriggerInfo, err error) {
	pullRequest, _, err := s.GithubClient.PullRequests.Get(ctx, s.Config.Org, pr.RepoName, pr.Number)
	if err != nil {
		return nil, err
	}
	baseBranch := pullRequest.GetBase().GetRef()
	isFork := pullRequest.GetHead().GetRepo().GetFork()
	var serverOwner string
	if isFork {
		serverOwner = pullRequest.GetHead().GetUser().GetLogin()
	} else {
		serverOwner = s.Config.Org
	}
	if serverOwner == "" {
		return nil, errors.New("owner of server branch not found")
	}

	eeBranch, err := s.getBranchWithSameName(ctx, s.Config.Org, s.Config.EnterpriseReponame, pr.Ref)
	if err != nil {
		return nil, err
	}
	if eeBranch == "" {
		eeBranch = baseBranch
	}

	webappOwner, webappBranch, err := s.getBranchFromForkOrUpstreamRepo(ctx, pr, s.Config.EnterpriseWebappReponame)
	if err != nil {
		return nil, err
	}
	if webappBranch == "" {
		webappOwner = s.Config.Org
		webappBranch = baseBranch
	}

	info = &EETriggerInfo{
		BaseBranch:   baseBranch,
		EEBranch:     eeBranch,
		ServerOwner:  serverOwner,
		ServerBranch: pr.Ref,
		WebappOwner:  webappOwner,
		WebappBranch: webappBranch,
	}
	return info, nil
}

func (s *Server) getBranchWithSameName(ctx context.Context, remote string, repo string, ref string) (branch string, err error) {
	ghBranch, r, err := s.GithubClient.Repositories.GetBranch(ctx, remote, repo, ref)
	if err != nil {
		if r == nil || r.StatusCode != http.StatusNotFound {
			return "", err
		}
		return "", nil // do not err if branch is not found
	}
	if ghBranch == nil {
		return "", errors.New("unexpected failure case")
	}
	return ghBranch.GetName(), nil
}

func (s *Server) getBranchFromForkOrUpstreamRepo(ctx context.Context, serverPR *model.PullRequest, repo string) (owner string, branch string, err error) {
	forkBranch, err := s.getBranchWithSameName(ctx, serverPR.Username, repo, serverPR.Ref)
	if err != nil {
		return "", "", err
	}
	if forkBranch == "" {
		upstreamBranch, err := s.getBranchWithSameName(ctx, s.Config.Org, repo, serverPR.Ref)
		if err != nil {
			return "", "", err
		}
		if upstreamBranch == "" {
			return s.Config.Org, "", nil
		}
		return s.Config.Org, upstreamBranch, nil
	}
	return serverPR.Username, forkBranch, nil
}

func (s *Server) createEnterpriseTestsPendingStatus(ctx context.Context, pr *model.PullRequest) {
	enterpriseStatus := &github.RepoStatus{
		State:       github.String(statePending),
		Context:     github.String(s.Config.EnterpriseGithubStatusContext),
		Description: github.String("TODO as org member: After reviewing please trigger label \"" + s.Config.EnterpriseTriggerLabel + "\""),
		TargetURL:   github.String(""),
	}
	err := s.createRepoStatus(ctx, pr, enterpriseStatus)
	if err != nil {
		s.logToMattermost("failed to create status for PR: " + strconv.Itoa(pr.Number) + " Context: " + s.Config.EnterpriseGithubStatusContext + " Error: ```" + err.Error() + "```")
	}
}

func (s *Server) createEnterpriseTestsErrorStatus(ctx context.Context, pr *model.PullRequest, err error) {
	enterpriseErrorStatus := &github.RepoStatus{
		State:       github.String(stateError),
		Context:     github.String(s.Config.EnterpriseGithubStatusContext),
		Description: github.String("Enterprise tests error"),
		TargetURL:   github.String(""),
	}
	s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number,
		"Failed running enterprise tests. @mattermost/core-build-engineers have been notified. Error:  \n```"+err.Error()+"```")
	_ = s.createRepoStatus(ctx, pr, enterpriseErrorStatus)
}

func (s *Server) succeedEEStatuses(ctx context.Context, pr *model.PullRequest, desc string) {
	eeTriggeredStatus := &github.RepoStatus{
		State:       github.String(stateSuccess),
		Context:     github.String(s.Config.EnterpriseGithubStatusContext),
		Description: github.String(desc),
		TargetURL:   github.String(""),
	}
	_ = s.createRepoStatus(ctx, pr, eeTriggeredStatus)

	eeReportStatus := &github.RepoStatus{
		State:       github.String(stateSuccess),
		Context:     github.String(s.Config.EnterpriseGithubStatusEETests),
		Description: github.String(desc),
		TargetURL:   github.String(""),
	}
	_ = s.createRepoStatus(ctx, pr, eeReportStatus)
}

func (s *Server) updateBuildStatus(ctx context.Context, pr *model.PullRequest, context string, targetURL string) {
	status := &github.RepoStatus{
		State:       github.String(statePending),
		Context:     github.String(context),
		Description: github.String("Testing EE. SHA: " + pr.Sha),
		TargetURL:   github.String(targetURL),
	}
	_ = s.createRepoStatus(ctx, pr, status)
}
