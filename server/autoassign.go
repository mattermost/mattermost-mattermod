package server

import (
	"context"
	"fmt"

	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v6/shared/mlog"

	"github.com/google/go-github/v39/github"
)

func (s *Server) handleAutoAssign(ctx context.Context, url string, pr *model.PullRequest) error {
	var err error
	defer func() {
		if err != nil {
			s.autoAssignerPostError(ctx, pr.RepoOwner, pr.RepoName, pr.Number, url)
		}
	}()

	var teams []*github.Team
	teams, _, err = s.GithubClient.Repositories.ListTeams(ctx, pr.RepoOwner, pr.RepoName, nil)
	if err != nil {
		return err
	}

	repoConfigured := false
	for _, team := range teams {
		if team.GetID() == s.Config.AutoAssignerTeamID {
			repoConfigured = true
			break
		}
	}

	if !repoConfigured {
		msg := fmt.Sprintf("In response to [this](%s)\n\n The auto assigner is not configured for this repository. Please talk with a Mattermost Github admin. thanks!", url)
		if err = s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, msg); err != nil {
			mlog.Warn("Error while commenting", mlog.Err(err))
		}
		return nil
	}

	reviewReq := github.ReviewersRequest{
		TeamReviewers: []string{s.Config.AutoAssignerTeam},
	}
	_, _, err = s.GithubClient.PullRequests.RequestReviewers(ctx, pr.RepoOwner, pr.RepoName, pr.Number, reviewReq)
	if err != nil {
		return err
	}

	msg := fmt.Sprintf("In response to [this](%s)\n\n I'm requesting the Pull Panda autoassigner to add reviewers to this PR.", url)
	if err = s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, msg); err != nil {
		mlog.Warn("Error while commenting", mlog.Err(err))
	}

	return nil
}

func (s *Server) autoAssignerPostError(ctx context.Context, repoOwner, repoName string, number int, requestCommentURL string) {
	msg := fmt.Sprintf("In response to [this](%s)\n\n I'm not able to request Pull Panda to add reviewers", requestCommentURL)
	if err := s.sendGitHubComment(ctx, repoOwner, repoName, number, msg); err != nil {
		mlog.Warn("Error while commenting", mlog.Err(err))
	}
}
