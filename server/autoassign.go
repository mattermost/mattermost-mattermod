package server

import (
	"context"
	"fmt"

	"github.com/mattermost/mattermost-server/v5/mlog"

	"github.com/google/go-github/v31/github"
)

func (s *Server) handleAutoassign(eventIssueComment IssueComment) {
	teams, _, err := s.GithubClient.Repositories.ListTeams(context.Background(), *eventIssueComment.Repository.Owner.Login, *eventIssueComment.Repository.Name, nil)
	if err != nil {
		mlog.Error("Error listing the teams to check", mlog.Err(err), mlog.String("RepoOwner", *eventIssueComment.Repository.Owner.Login), mlog.Int("PR", *eventIssueComment.Issue.Number), mlog.String("Repo", *eventIssueComment.Repository.Name))
		s.autoAssignerPostError(*eventIssueComment.Repository.Owner.Login, *eventIssueComment.Repository.Name, *eventIssueComment.Issue.Number, eventIssueComment.Comment.GetHTMLURL())
		return
	}

	repoConfigured := false
	for _, team := range teams {
		if team.GetID() == s.Config.AutoAssignerTeamID {
			mlog.Info("Team configured for this repo", mlog.String("RepoName", *eventIssueComment.Repository.Name))
			repoConfigured = true
			break
		}
	}

	if repoConfigured {
		reviewReq := github.ReviewersRequest{
			TeamReviewers: []string{s.Config.AutoAssignerTeam},
		}

		_, _, err = s.GithubClient.PullRequests.RequestReviewers(context.Background(), *eventIssueComment.Repository.Owner.Login, *eventIssueComment.Repository.Name, *eventIssueComment.Issue.Number, reviewReq)
		if err != nil {
			mlog.Error("Error setting the reviewers for pull panda", mlog.Err(err), mlog.Int("PR", *eventIssueComment.Issue.Number), mlog.String("Repo", *eventIssueComment.Repository.Name))
			s.autoAssignerPostError(*eventIssueComment.Repository.Owner.Login, *eventIssueComment.Repository.Name, *eventIssueComment.Issue.Number, eventIssueComment.Comment.GetHTMLURL())
			return
		}

		msg := fmt.Sprintf("In response to [this](%s)\n\n I'm requesting the Pull Panda autoassigner to add reviewers to this PR.", eventIssueComment.Comment.GetHTMLURL())
		s.sendGitHubComment(*eventIssueComment.Repository.Owner.Login, *eventIssueComment.Repository.Name, *eventIssueComment.Issue.Number, msg)
	} else {
		msg := fmt.Sprintf("In response to [this](%s)\n\n The auto assigner is not configured for this repository. Please talk with a Mattermost Github admin. thanks!", eventIssueComment.Comment.GetHTMLURL())
		s.sendGitHubComment(*eventIssueComment.Repository.Owner.Login, *eventIssueComment.Repository.Name, *eventIssueComment.Issue.Number, msg)
	}
}

func (s *Server) autoAssignerPostError(repoOwner, repoName string, number int, requestCommentURL string) {
	msg := fmt.Sprintf("In response to [this](%s)\n\n I'm not able to request Pull Panda to add reviewers", requestCommentURL)
	s.sendGitHubComment(repoOwner, repoName, number, msg)
}
