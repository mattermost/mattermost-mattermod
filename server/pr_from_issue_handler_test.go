// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/google/go-github/v32/github"
	"github.com/mattermost/mattermost-mattermod/model"
	stmock "github.com/mattermost/mattermost-mattermod/store/mocks"
	"github.com/stretchr/testify/require"
)

func TestPRFromIssueHandler(t *testing.T) {
	ctrl := gomock.NewController(t)

	// ctxInterface := reflect.TypeOf((*context.Context)(nil)).Elem()

	url := "https://github.com/mattermost/mmctl/pull/3"
	number := 3
	login := "mattermost"
	name := "mmctl"

	event := issueEvent{
		Issue: &github.Issue{
			Repository: &github.Repository{
				Owner: &github.User{
					Login: &login,
				},
				Name: &name,
			},
			Number:  &number,
			HTMLURL: &url,
			User: &github.User{
				Login: &login,
			},
			PullRequestLinks: &github.PullRequestLinks{
				HTMLURL: &url,
			},
			Milestone: &github.Milestone{
				Number: github.Int(2),
				Title:  github.String("release-5.28"),
			},
		},
		Repo: &github.Repository{
			Owner: &github.User{
				Login: &login,
			},
			Name: &name,
		},
	}

	ss := stmock.NewMockStore(ctrl)

	prStoreMock := stmock.NewMockPullRequestStore(ctrl)
	prStoreMock.EXPECT().Save(gomock.Eq(&model.PullRequest{
		RepoOwner:           event.Repo.GetOwner().GetLogin(),
		RepoName:            event.Repo.GetName(),
		Number:              event.Issue.GetNumber(),
		Sha:                 "sha",
		Labels:              []string{},
		State:               "closed",
		Merged:              sql.NullBool{Bool: false, Valid: true},
		MaintainerCanModify: sql.NullBool{Bool: false, Valid: true},
		MilestoneNumber:     sql.NullInt64{Int64: int64(event.Issue.Milestone.GetNumber()), Valid: true},
		MilestoneTitle:      sql.NullString{String: event.Issue.Milestone.GetTitle(), Valid: true},
	})).
		Times(1).Return(nil, nil)
	prStoreMock.EXPECT().Get(gomock.Eq(event.Repo.GetOwner().GetLogin()),
		gomock.Eq(event.Repo.GetName()),
		gomock.Eq(event.Issue.GetNumber())).
		Times(1).
		Return(&model.PullRequest{
			RepoOwner:           event.Repo.GetOwner().GetLogin(),
			RepoName:            event.Repo.GetName(),
			Number:              event.Issue.GetNumber(),
			Sha:                 "sha",
			Labels:              []string{},
			State:               "closed",
			Merged:              sql.NullBool{Bool: false, Valid: true},
			MaintainerCanModify: sql.NullBool{Bool: false, Valid: true},
			MilestoneNumber:     sql.NullInt64{Int64: int64(0), Valid: true},
			MilestoneTitle:      sql.NullString{String: "release-5.28", Valid: true},
		}, nil)

	ss.EXPECT().
		PullRequest().
		Return(prStoreMock).
		AnyTimes()

	s := &Server{
		GithubClient: &GithubClient{},
		Config: &Config{
			IssueLabels: []LabelResponse{},
			Username:    "mattermost",
		},
		Store: ss,
	}

	ts := httptest.NewServer(http.HandlerFunc(s.githubEvent))
	defer ts.Close()

	b, err := json.Marshal(&event)
	require.NoError(t, err)

	req, err := http.NewRequest("POST", ts.URL, bytes.NewReader(b))
	req.Header.Set("X-GitHub-Event", "issues")
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}
