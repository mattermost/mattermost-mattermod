// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"net/http"

	"golang.org/x/time/rate"
)

type GithubRateLimitTransport struct {
	limiter *rate.Limiter
	base    http.RoundTripper
}

func (t *GithubRateLimitTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := t.limiter.Wait(req.Context()); err != nil {
		return nil, err
	}
	return t.base.RoundTrip(req)
}

func NewGithubRateLimitTransport(limit rate.Limit, tokens int, base http.RoundTripper) *GithubRateLimitTransport {
	limiter := rate.NewLimiter(limit, tokens)
	return &GithubRateLimitTransport{limiter, base}
}
