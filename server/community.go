// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"text/template"
	"time"

	"github.com/google/go-github/v32/github"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v5/mlog"
	"github.com/pkg/errors"
)

func (s *Server) addHacktoberfestLabel(ctx context.Context, pr *model.PullRequest) {
	if pr.State == model.StateClosed {
		return
	}

	// Ignore PRs not created in october
	if pr.CreatedAt.Month() != time.October {
		return
	}

	// Don't apply label if the contributors is a core committer
	if s.IsOrgMember(pr.Username) {
		return
	}

	_, _, err := s.GithubClient.Issues.AddLabelsToIssue(ctx, pr.RepoOwner, pr.RepoName, pr.Number, []string{"Hacktoberfest", "hacktoberfest-accepted"})
	if err != nil {
		mlog.Error("error applying Hacktoberfest labels", mlog.Err(err), mlog.Int("PR", pr.Number), mlog.String("Repo", pr.RepoName))
		return
	}
}

func (s *Server) postPRWelcomeMessage(ctx context.Context, pr *model.PullRequest, claCommentNeeded bool) error {
	// Only post welcome Message for community member
	if s.IsOrgMember(pr.Username) {
		return nil
	}

	t, err := template.New("welcomeMessage").Parse(s.Config.PRWelcomeMessage)
	if err != nil {
		return errors.Wrap(err, "failed to render welcome message template")
	}

	var output bytes.Buffer
	data := map[string]interface{}{
		"CLACommentNeeded": claCommentNeeded,
		"Username":         "@" + pr.Username,
	}
	err = t.Execute(&output, data)
	if err != nil {
		return errors.Wrap(err, "could not execute welcome message template")
	}

	err = s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, output.String())
	if err != nil {
		return errors.Wrap(err, "failed to send welcome message")
	}

	return nil
}

func (s *Server) assignGreeter(ctx context.Context, pr *model.PullRequest, repo *Repository) error {
	// Only assign an greeter for non-member PRs
	if s.IsOrgMember(pr.Username) {
		return nil
	}

	// Is it configured to have a greeting team to handle the PR?
	if repo.GreetingTeam == "" {
		return nil
	}

	// get available members
	greeterList, _, err := s.GithubClient.Teams.ListTeamMembersBySlug(ctx, s.Config.Org, repo.GreetingTeam, nil)
	if err != nil {
		return errors.Wrapf(err, "can't retrieve team members from %s", repo.GreetingTeam)
	}

	// Assign one of them as a reviewer
	size := len(greeterList)
	if size == 0 {
		return fmt.Errorf("there were no members on the greeting team %s", repo.GreetingTeam)
	}

	greeter := greeterList[rand.Intn(size)]
	greetingRequest := github.ReviewersRequest{
		Reviewers: []string{*greeter.Login},
	}

	_, _, err = s.GithubClient.PullRequests.RequestReviewers(ctx, pr.RepoOwner, pr.RepoName, pr.Number, greetingRequest)
	if err != nil {
		return errors.Wrapf(err, "couldn't assing a member of the greeting team %s", repo.GreetingTeam)
	}

	return nil
}

func (s *Server) assignGreetingLabels(ctx context.Context, pr *model.PullRequest, repo *Repository) error {
	// Only assign labels for non-member PRs
	if s.IsOrgMember(pr.Username) {
		return nil
	}

	// Is repo configured to setup labels for non-member PRs?
	if len(repo.GreetingLabels) == 0 {
		return nil
	}

	// Assign labels
	_, _, err := s.GithubClient.Issues.AddLabelsToIssue(ctx, pr.RepoOwner, pr.RepoName, pr.Number, repo.GreetingLabels)
	if err != nil {
		return errors.Wrapf(err, "couldn't apply greeting labels to %s #%d", pr.RepoName, pr.Number)
	}
	return nil
}
