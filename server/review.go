// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/go-github/v39/github"
	"github.com/sourcegraph/go-diff/diff"

	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v6/shared/mlog"
)

const mlogReviewCommentBody = "Gentle reminder to check our logging [principles](https://developers.mattermost.com/contribute/server/style-guide/#log-levels) before merging this change."

func (s *Server) reviewMlog(ctx context.Context, pr *model.PullRequest, nodeID, diffURL string) error {
	// Do not review this for organization members. This was in use for a while
	// and we can assume that people in the organization should be aware of the gudieline.
	if s.IsOrgMember(pr.Username) {
		return nil
	}

	b, err := getRawDiff(ctx, diffURL)
	if err != nil {
		return fmt.Errorf("could not retrieve diff: %w", err)
	}

	fileDiffs, err := diff.ParseMultiFileDiff(b)
	if err != nil {
		return fmt.Errorf("could not parse file diff: %w", err)
	}

	mlog.Debug("Going to review files", mlog.Int("num_files", len(fileDiffs)))

	// We simply iterate every file in the diff and check if the PR includes an mlog.Error/mlog.Critical call
	for _, fileDiff := range fileDiffs {
		var position int32 // reset position since it's file specific

		for _, hunk := range fileDiff.Hunks {
			for _, line := range strings.Split(string(hunk.Body), "\n") {
				position++

				if line == "" || line[0] != '+' {
					continue // we are not interested if it's not an addition
				}

				if strings.Contains(line, "mlog.Error(") || strings.Contains(line, "mlog.Critical(") {
					mlog.Info("Found mlog.Error/mlog.Critical insertion", mlog.String("file", fileDiff.NewName), mlog.Int("line", int(position)))

					review := &github.PullRequestReviewRequest{
						NodeID: github.String(nodeID),
						Event:  github.String("COMMENT"),
						Comments: []*github.DraftReviewComment{
							{
								Path:     github.String(strings.TrimPrefix(fileDiff.NewName, "b/")), // b/{file_name} is the file on the right side
								Position: github.Int(int(position)),
								Body:     github.String(mlogReviewCommentBody),
							},
						},
					}

					_, _, err := s.GithubClient.PullRequests.CreateReview(ctx, pr.RepoOwner, pr.RepoName, pr.Number, review)
					if err != nil {
						return fmt.Errorf("could not create the review for PR %s/%s#%d: %w", pr.RepoOwner, pr.RepoName, pr.Number, err)
					}

					return nil
				}
			}
		}
	}

	return nil
}

func getRawDiff(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return b, nil
}
