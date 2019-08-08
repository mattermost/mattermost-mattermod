package server

import (
	"context"
	"fmt"

	"github.com/mattermost/mattermost-server/mlog"

	"github.com/google/go-github/github"
)

func (s *Server) handleAutoassign(eventIssueComment IssueComment) {
	client := NewGithubClient(s.Config.GithubAccessToken)

	reviewReq := github.ReviewersRequest{
		Reviewers: []string{s.Config.AutoAssignerTeam},
	}

	_, _, err := client.PullRequests.RequestReviewers(context.Background(), *eventIssueComment.Repository.Owner.Login, *eventIssueComment.Repository.Name, *eventIssueComment.Issue.Number, reviewReq)
	if err != nil {
		mlog.Error("Error setting the reviewers for pull panda", mlog.Err(err), mlog.Int("PR", *eventIssueComment.Issue.Number), mlog.String("Repo", *eventIssueComment.Repository.Name))
		msg := fmt.Sprintf("In response to [this](%s)\n\n I'm was not able to request Pull Panda to add reviewers", eventIssueComment.Comment.GetHTMLURL())
		s.commentOnIssue(*eventIssueComment.Repository.Owner.Login, *eventIssueComment.Repository.Name, *eventIssueComment.Issue.Number, msg)
		return
	}

	msg := fmt.Sprintf("In response to [this](%s)\n\n I'm requesting the Pull Panda autoassigner to add reviewers to this PR.", eventIssueComment.Comment.GetHTMLURL())
	eventIssueComment.Repository.GetOwner()
	s.commentOnIssue(*eventIssueComment.Repository.Owner.Login, *eventIssueComment.Repository.Name, *eventIssueComment.Issue.Number, msg)

}
