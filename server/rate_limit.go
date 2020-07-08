// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"net/http"

	"golang.org/x/time/rate"
)

// RateLimitTransport will provide a layer based on http.RounTripper interface
// that provided rate limiting capability
type RateLimitTransport struct {
	limiter *rate.Limiter
	base    http.RoundTripper
}

func (t *RateLimitTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := t.limiter.Wait(req.Context()); err != nil {
		return nil, err
	}
	return t.base.RoundTrip(req)
}

// NewRateLimitTransport will return a new transport that provides rate limiting capability
// based on the provided limit and burst tokens.
// It also needs the base RountTripper that will be called in case the rate limit is not needed
func NewRateLimitTransport(limit rate.Limit, tokens int, base http.RoundTripper) *RateLimitTransport {
	limiter := rate.NewLimiter(limit, tokens)
	return &RateLimitTransport{limiter, base}
}
