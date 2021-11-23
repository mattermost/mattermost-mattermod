package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v5/mlog"
)

const (
	e2eTestMsgCommenterPermission = "You don't have permissions to trigger this command.\n It's only available for organization members."
	e2eTestMsgCIFailing           = "The command /e2e-test requires all PR checks to pass."
	e2eTestMsgUnknownPRState      = "Failed to check whether PR checks passed. E2E testing isn't triggered. Please retry later."
	e2eTestMsgPRInfo              = "Failed to get the PR information required to trigger testing. Please retry later."
	e2eTestMsgUnknownTriggerRepo  = "The ability to trigger E2E testing pipelines for this repository isn't set up. \n Please contact a maintainer."
	e2eTestMsgTrigger             = "Failed to trigger E2E testing pipeline."
	e2eTestMsgCompanionBranch     = "Failed to locate companion branch."
	e2eTestMsgSameEnvs            = "A pipeline with the same environment variables is already running. \n Please cancel it first with /e2e-cancel, or specify different environment variables."

	e2eTestMsgOpts    = "Triggering e2e testing with options:"
	e2eTestFmtOpts    = "%v\n```%v```"
	e2eTestMsgSuccess = "Successfully triggered e2e testing!"
	e2eTestFmtSuccess = "%v\n%v"
)

func (e *E2ETestError) Error() string {
	switch e.source {
	case e2eTestMsgCommenterPermission:
		return "commenter does not have permissions"
	case e2eTestMsgCIFailing:
		return "PR checks needs to be passing"
	case e2eTestMsgUnknownPRState:
		return "unknown PR state"
	case e2eTestMsgPRInfo:
		return "could not fetch PR info"
	case e2eTestMsgUnknownTriggerRepo:
		return "pipeline triggered from not set up repo"
	case e2eTestMsgTrigger:
		return "could not trigger pipeline"
	case e2eTestMsgCompanionBranch:
		return "failed fetching companion branch"
	case e2eTestMsgSameEnvs:
		return "same pipeline already running"
	default:
		panic("unhandled error type")
	}
}

type E2ETestError struct {
	source string
}

type E2ETestTriggerInfo struct {
	TriggerPR    int
	TriggerRepo  string
	TriggerSHA   string
	RefToTrigger string
	ServerBranch string
	ServerSHA    string
	WebappBranch string
	WebappSHA    string
	EnvVars      map[string]string
	BuildTag     string
}

func (s *Server) handleE2ETest(ctx context.Context, commenter string, pr *model.PullRequest, commentBody string) error {
	var e2eTestErr *E2ETestError
	defer func() {
		if e2eTestErr != nil {
			if err := s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, e2eTestErr.source); err != nil {
				mlog.Warn("Error while commenting", mlog.Err(err))
			}
		}
	}()
	if !s.IsOrgMember(commenter) {
		e2eTestErr = &E2ETestError{source: e2eTestMsgCommenterPermission}
		return e2eTestErr
	}
	prRepoOwner, prRepoName, prNumber := pr.RepoOwner, pr.RepoName, pr.Number

	isCIPassing, err := s.areChecksSuccessfulForPR(ctx, pr)
	if err != nil {
		e2eTestErr = &E2ETestError{source: e2eTestMsgUnknownPRState}
		return e2eTestErr
	}
	if !isCIPassing {
		e2eTestErr = &E2ETestError{source: e2eTestMsgCIFailing}
		return e2eTestErr
	}

	ghStatusContext := ""
	switch prRepoName {
	case s.Config.E2EWebappReponame:
		ghStatusContext = s.Config.E2EWebappStatusContext
	case s.Config.E2EServerReponame:
		ghStatusContext = s.Config.E2EServerStatusContext
	}

	ctx2, cancel := context.WithTimeout(context.Background(), defaultE2ETestTimeout*time.Second)
	defer cancel()
	err = s.waitForStatus(ctx2, pr, ghStatusContext, stateSuccess)
	if err != nil {
		return err
	}

	opts := parseE2ETestCommentForOpts(commentBody)
	optMsg := formatOpts(opts)

	info, err := s.getPRInfoForE2ETest(ctx, pr, opts)
	if err != nil {
		e2eTestErr = &E2ETestError{source: e2eTestMsgPRInfo}
		return e2eTestErr
	}

	initMsg := fmt.Sprintf(e2eTestFmtOpts, e2eTestMsgOpts, optMsg)
	if cErr := s.sendGitHubComment(ctx, prRepoOwner, prRepoName, prNumber, initMsg); cErr != nil {
		mlog.Warn("Error while commenting", mlog.Err(cErr))
	}

	has, err := s.checkForPipelinesWithSameEnvs(ctx, info)
	if err != nil {
		return err
	}
	if has {
		e2eTestErr = &E2ETestError{source: e2eTestMsgSameEnvs}
		return e2eTestErr
	}

	url, err := s.triggerE2EGitLabPipeline(ctx, info)
	if err != nil {
		e2eTestErr = &E2ETestError{source: e2eTestMsgTrigger}
		return e2eTestErr
	}
	endMsg := fmt.Sprintf(e2eTestFmtSuccess, e2eTestMsgSuccess, url)
	if cErr := s.sendGitHubComment(ctx, prRepoOwner, prRepoName, prNumber, endMsg); cErr != nil {
		mlog.Warn("Error while commenting", mlog.Err(cErr))
	}

	return nil
}

// Example commands:
// /e2e-test
// /e2e-test MM_ENV=\"MM_FEATUREFLAGS_GLOBALHEADER=true,MM_OTHER_FLAG=true\" INCLUDE_FILE=\"new_message_spec.js\" EXCLUDE_FILE=\"something_to_exclude_spec.js\"\nOther commenting after command \n Even other comment
func parseE2ETestCommentForOpts(commentBody string) *map[string]string {
	cmd := strings.Split(commentBody, "\n")[0]
	cmd = strings.TrimSuffix(cmd, " ")

	if !strings.Contains(cmd, " ") && !strings.Contains(cmd, "=") {
		mlog.Debug("E2E comment does not contain options")
		return nil
	}

	var opts = make(map[string]string)
	optsAfterBaseCmd := strings.Split(cmd, " ")[1:]
	for _, opt := range optsAfterBaseCmd {
		optSplitInAssignment := strings.SplitAfterN(opt, string('='), 2)
		envKey := strings.TrimSuffix(optSplitInAssignment[0], "=")
		if _, exists := opts[envKey]; exists {
			continue
		}
		envValue := optSplitInAssignment[1]
		opts[envKey] = strings.Trim(envValue, "\"")
	}

	return &opts
}

// We ignore forks for now, since the build tag will still be built for forks.
// This means, modified webapp tests and server config settings will not be accurate in E2E testing for forks.
// https://git.internal.mattermost.com/qa/cypress-ui-automation/-/blob/master/scripts/prepare-test-cycle.sh requires webapp to be cloned
// https://git.internal.mattermost.com/qa/cypress-ui-automation/-/blob/master/scripts/prepare-test-server.sh requires server to be cloned
// getPRInfoForE2ETest returns information needed to trigger E2E testing
func (s *Server) getPRInfoForE2ETest(ctx context.Context, pr *model.PullRequest, opts *map[string]string) (*E2ETestTriggerInfo, error) {
	info := &E2ETestTriggerInfo{
		TriggerPR:   pr.Number,
		TriggerRepo: pr.RepoName,
		TriggerSHA:  pr.Sha,
		BuildTag:    s.Config.E2EDockerRepo + pr.Sha[0:7],
	}

	if opts != nil {
		info.EnvVars = *opts
	}

	info.RefToTrigger = ""
	var err error
	switch info.TriggerRepo {
	case s.Config.E2EWebappReponame:
		info.ServerBranch, info.ServerSHA, err = s.getBranchAndSHAWithSameName(ctx, s.Config.Org, s.Config.E2EServerReponame, pr.Ref)
		if err != nil {
			e2eTestErr := &E2ETestError{source: e2eTestMsgCompanionBranch}
			return nil, fmt.Errorf("%s: %w", e2eTestErr, err)
		}
		if info.ServerBranch == "" {
			pullRequest, _, err2 := s.GithubClient.PullRequests.Get(ctx, s.Config.Org, pr.RepoName, pr.Number)
			if err2 != nil {
				return nil, fmt.Errorf("error trying to get pr number %d for repo %s: %w",
					pr.Number, pr.RepoName, err2)
			}
			info.ServerBranch = pullRequest.GetBase().GetRef()
			info.ServerSHA = ""
		}
		info.RefToTrigger = s.Config.E2EWebappRef
		info.WebappBranch = pr.Ref
		info.WebappSHA = pr.Sha
	case s.Config.E2EServerReponame:
		info.WebappBranch, info.WebappSHA, err = s.getBranchAndSHAWithSameName(ctx, s.Config.Org, s.Config.E2EWebappReponame, pr.Ref)
		if err != nil {
			e2eTestErr := &E2ETestError{source: e2eTestMsgCompanionBranch}
			return nil, fmt.Errorf("%s: %w", e2eTestErr, err)
		}
		if info.WebappBranch == "" {
			pullRequest, _, err2 := s.GithubClient.PullRequests.Get(ctx, s.Config.Org, pr.RepoName, pr.Number)
			if err2 != nil {
				return nil, fmt.Errorf("error trying to get pr number %d for repo %s: %w",
					pr.Number, pr.RepoName, err)
			}
			info.WebappBranch = pullRequest.GetBase().GetRef()
			info.WebappSHA = ""
		}
		info.RefToTrigger = s.Config.E2EServerRef
		info.ServerBranch = pr.Ref
		info.ServerSHA = pr.Sha
	}
	if info.RefToTrigger == "" {
		e2eTestErr := &E2ETestError{source: e2eTestMsgUnknownTriggerRepo}
		return nil, fmt.Errorf("%s: %w", e2eTestErr, err)
	}

	return info, nil
}

func (s *Server) getBranchAndSHAWithSameName(ctx context.Context, owner string, repo string, ref string) (branch string, sha string, err error) {
	ghBranch, r, err := s.GithubClient.Repositories.GetBranch(ctx, owner, repo, ref, false)
	if err != nil {
		if r == nil || r.StatusCode != http.StatusNotFound {
			return "", "", fmt.Errorf("error trying to get branch %s for repo %s: %w",
				ref, repo, err)
		}
		return "", "", nil // do not err if branch is not found
	}
	if ghBranch == nil {
		return "", "", errors.New("unexpected failure case")
	}
	return ghBranch.GetName(), ghBranch.GetCommit().GetSHA(), nil
}

func formatOpts(opts *map[string]string) string {
	if nil == opts {
		return ""
	}
	var optMsg string
	for k, v := range *opts {
		optMsg += fmt.Sprintf("%v=%v\n", k, v)
	}
	return optMsg
}
