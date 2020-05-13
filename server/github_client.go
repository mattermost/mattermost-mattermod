// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"

	"github.com/google/go-github/v31/github"
	"golang.org/x/oauth2"
)

type ChecksService interface {
	ListCheckRunsForRef(ctx context.Context, owner, repo, ref string, opts *github.ListCheckRunsOptions) (*github.ListCheckRunsResults, *github.Response, error)
}

type IssuesService interface {
	AddAssignees(ctx context.Context, owner, repo string, number int, assignees []string) (*github.Issue, *github.Response, error)
	AddLabelsToIssue(ctx context.Context, owner string, repo string, number int, labels []string) ([]*github.Label, *github.Response, error)
	CreateComment(ctx context.Context, owner string, repo string, number int, comment *github.IssueComment) (*github.IssueComment, *github.Response, error)
	DeleteComment(ctx context.Context, owner string, repo string, commentID int64) (*github.Response, error)
	Edit(ctx context.Context, owner string, repo string, number int, issue *github.IssueRequest) (*github.Issue, *github.Response, error)
	Get(ctx context.Context, owner string, repo string, number int) (*github.Issue, *github.Response, error)
	ListByRepo(ctx context.Context, owner string, repo string, opts *github.IssueListByRepoOptions) ([]*github.Issue, *github.Response, error)
	ListComments(ctx context.Context, owner string, repo string, number int, opts *github.IssueListCommentsOptions) ([]*github.IssueComment, *github.Response, error)
	ListLabelsByIssue(ctx context.Context, owner string, repo string, number int, opt *github.ListOptions) ([]*github.Label, *github.Response, error)
	RemoveLabelForIssue(ctx context.Context, owner string, repo string, number int, label string) (*github.Response, error)
}

type GitService interface {
	CreateRef(ctx context.Context, owner string, repo string, ref *github.Reference) (*github.Reference, *github.Response, error)
	DeleteRef(ctx context.Context, owner string, repo string, ref string) (*github.Response, error)
	GetRef(ctx context.Context, owner string, repo string, ref string) (*github.Reference, *github.Response, error)
}

type OrganizationsService interface {
	GetOrgMembership(ctx context.Context, user, org string) (*github.Membership, *github.Response, error)
	IsMember(ctx context.Context, org, user string) (bool, *github.Response, error)
}

type PullRequestsService interface {
	Get(ctx context.Context, owner string, repo string, number int) (*github.PullRequest, *github.Response, error)
	List(ctx context.Context, owner string, repo string, opts *github.PullRequestListOptions) ([]*github.PullRequest, *github.Response, error)
	ListFiles(ctx context.Context, owner string, repo string, number int, opts *github.ListOptions) ([]*github.CommitFile, *github.Response, error)
	ListReviewers(ctx context.Context, owner, repo string, number int, opts *github.ListOptions) (*github.Reviewers, *github.Response, error)
	ListReviews(ctx context.Context, owner, repo string, number int, opts *github.ListOptions) ([]*github.PullRequestReview, *github.Response, error)
	Merge(ctx context.Context, owner string, repo string, number int, commitMessage string, options *github.PullRequestOptions) (*github.PullRequestMergeResult, *github.Response, error)
	RequestReviewers(ctx context.Context, owner, repo string, number int, reviewers github.ReviewersRequest) (*github.PullRequest, *github.Response, error)
	UpdateBranch(ctx context.Context, owner, repo string, number int, opts *github.PullRequestBranchUpdateOptions) (*github.PullRequestBranchUpdateResponse, *github.Response, error)
}

type RepositoriesService interface {
	CreateStatus(ctx context.Context, owner, repo, ref string, status *github.RepoStatus) (*github.RepoStatus, *github.Response, error)
	Get(ctx context.Context, owner, repo string) (*github.Repository, *github.Response, error)
	GetBranch(ctx context.Context, owner, repo, branch string) (*github.Branch, *github.Response, error)
	GetCombinedStatus(ctx context.Context, owner, repo, ref string, opts *github.ListOptions) (*github.CombinedStatus, *github.Response, error)
	ListTeams(ctx context.Context, owner string, repo string, opts *github.ListOptions) ([]*github.Team, *github.Response, error)
	ListStatuses(ctx context.Context, owner, repo, ref string, opts *github.ListOptions) ([]*github.RepoStatus, *github.Response, error)
}

// GithubClient wraps the github.Client with relevant interfaces.
type GithubClient struct {
	client *github.Client

	Checks        ChecksService
	Git           GitService
	Issues        IssuesService
	Organizations OrganizationsService
	PullRequests  PullRequestsService
	Repositories  RepositoriesService
}

func NewGithubClient(accessToken string) *GithubClient {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: accessToken})
	tc := oauth2.NewClient(context.Background(), ts)
	client := github.NewClient(tc)

	return &GithubClient{
		client:        client,
		Checks:        client.Checks,
		Git:           client.Git,
		Issues:        client.Issues,
		Organizations: client.Organizations,
		PullRequests:  client.PullRequests,
		Repositories:  client.Repositories,
	}
}

func (c *GithubClient) RateLimits(ctx context.Context) (*github.RateLimits, *github.Response, error) {
	return c.client.RateLimits(ctx)
}
