// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// MetricsProvider is the interface that exposes the communication with the metrics system
// this interface should be implemented by the different providers we want to include
type MetricsProvider interface {
	// ObserverHTTPRequestDuration stores the elapsed time for an HTTP request
	ObserveHTTPRequestDuration(method, handler, statusCode string, elapsed float64)
	// IncreaseWebhookRequest increases the counter for the webhook requests
	// identified by name
	IncreaseWebhookRequest(name string)
	// IncreaseWebhookErrors stores the number of errors identified by name
	IncreaseWebhookErrors(name string)

	// ObserveGithubRequestDuration stores the elapsed time for github requests
	ObserveGithubRequestDuration(method, handler, statusCode string, elapsed float64)
	// IncreaseGithubCacheHits stores the number of cache hits when a github request
	// is done. The information is stored using the HTTP method and the request handler
	IncreaseGithubCacheHits(method, handler string)
	// IncreaseGithubCacheMisses stores the number of cache misses when a github request
	// is done. The information is stored using the HTTP method and the request handler
	IncreaseGithubCacheMisses(method, handler string)

	// IncreaseRateLimiterErrors stores the number of errors received when trying to
	// rate limit the requests
	IncreaseRateLimiterErrors()

	// ObserverCronTaskDuration stores the elapsed time for a cron task
	ObserveCronTaskDuration(name string, elapsed float64)
	// IncreaseCronTaskErrors stores the number of errors for a cron task
	IncreaseCronTaskErrors(name string)
}

// Transport is an HTTP transport that would check
// the requests and increase some metrics, cache
// errors, etc based on the requests and responses
type MetricsTransport struct {
	Base    http.RoundTripper
	metrics MetricsProvider
}

// NewTransport returns a transport using a provided http.RoundTripper as
// the base and a metrics provider
func NewMetricsTransport(base http.RoundTripper, metrics MetricsProvider) *MetricsTransport {
	return &MetricsTransport{base, metrics}
}

func (t *MetricsTransport) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	start := time.Now()
	resp, err = t.Base.RoundTrip(req)
	elapsed := float64(time.Since(start)) / float64(time.Second)
	// rate limit error
	if resp == nil && err != nil {
		return resp, err
	}
	splittedPath := strings.Split(req.URL.Path, "/")
	path := req.URL.Path
	if len(splittedPath) > 5 {
		// This would leave path as "/repos/{user/org}/{repository}/issues"
		path = strings.Join(splittedPath[:5], "/")
	}
	statusCode := strconv.Itoa(resp.StatusCode)
	t.metrics.ObserveGithubRequestDuration(path, req.Method, statusCode, elapsed)

	if resp.Header.Get("X-From-Cache") == "1" {
		t.metrics.IncreaseGithubCacheHits(req.Method, path)
	} else {
		t.metrics.IncreaseGithubCacheMisses(req.Method, path)
	}

	return resp, err
}

// Client returns a new http.Client using Transport
// as the default transport
func (t *MetricsTransport) Client() *http.Client {
	return &http.Client{Transport: t}
}
