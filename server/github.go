// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"strconv"

	"github.com/google/go-github/github"
	"github.com/mattermost/mattermost-mattermod/model"
	"golang.org/x/oauth2"
)

func NewGithubClient() *github.Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: Config.GithubAccessToken})
	tc := oauth2.NewClient(oauth2.NoContext, ts)

	return github.NewClient(tc)
}

func GetPullRequestFromGithub(pullRequest *github.PullRequest) (*model.PullRequest, error) {
	pr := &model.PullRequest{
		RepoOwner: *pullRequest.Base.Repo.Owner.Login,
		RepoName:  *pullRequest.Base.Repo.Name,
		Number:    *pullRequest.Number,
		Username:  *pullRequest.User.Login,
		Ref:       *pullRequest.Head.Ref,
		Sha:       *pullRequest.Head.SHA,
		State:     *pullRequest.State,
	}

	client := NewGithubClient()

	repo, ok := Config.GetRepository(pr.RepoOwner, pr.RepoName)
	if ok && repo.BuildStatusContext != "" {
		if combined, _, err := client.Repositories.GetCombinedStatus(pr.RepoOwner, pr.RepoName, pr.Sha, nil); err != nil {
			return nil, err
		} else {
			for _, status := range combined.Statuses {
				if *status.Context == repo.BuildStatusContext {
					pr.BuildStatus = *status.State
					pr.BuildLink = *status.TargetURL
					break
				}
			}
		}
	}

	if labels, _, err := client.Issues.ListLabelsByIssue(pr.RepoOwner, pr.RepoName, pr.Number, nil); err != nil {
		return nil, err
	} else {
		pr.Labels = LabelsToStringArray(labels)
	}

	return pr, nil
}

func GetIssueFromGithub(repoOwner, repoName string, ghIssue *github.Issue) (*model.Issue, error) {
	issue := &model.Issue{
		RepoOwner: repoOwner,
		RepoName:  repoName,
		Number:    *ghIssue.Number,
		Username:  *ghIssue.User.Login,
		State:     *ghIssue.State,
	}

	if labels, _, err := NewGithubClient().Issues.ListLabelsByIssue(issue.RepoOwner, issue.RepoName, issue.Number, nil); err != nil {
		return nil, err
	} else {
		issue.Labels = LabelsToStringArray(labels)
	}

	return issue, nil
}

func LabelsToStringArray(labels []*github.Label) []string {
	out := make([]string, len(labels))

	for i, label := range labels {
		out[i] = *label.Name
	}

	return out
}

func commentOnIssue(repoOwner, repoName string, number int, comment string) {
	LogInfo("Commenting on issue " + strconv.Itoa(number) + " Comment: " + comment)
	client := NewGithubClient()
	_, _, err := client.Issues.CreateComment(repoOwner, repoName, number, &github.IssueComment{Body: &comment})
	if err != nil {
		LogError("Error: ", err)
	}
	LogInfo("Finished commenting")
}
