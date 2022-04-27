// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mattermost/mattermost-server/v6/shared/mlog"
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
	if resp == nil && err != nil {
		return resp, err
	}
	statusCode := strconv.Itoa(resp.StatusCode)
	requestPath := req.URL.Path
	if strings.Contains(req.URL.Host, "github") {
		requestPath = t.getGithubRequestPath(requestPath)
		errMetrics := t.processGithubMetrics(req, resp, requestPath, statusCode)
		if errMetrics != nil {
			mlog.Warn("can't process github metrics", mlog.Err(errMetrics))
		}
	}
	t.metrics.ObserveGithubRequestDuration(requestPath, req.Method, statusCode, elapsed)

	return resp, err
}

func (t *MetricsTransport) getGithubRequestPath(requestPath string) string {
	path := requestPath
	splittedPath := strings.Split(path, "/")
	if len(splittedPath) > 5 {
		// This would leave path as "/repos/{user/org}/{repository}/issues"
		path = strings.Join(splittedPath[:5], "/")
	}
	return path
}

func (t *MetricsTransport) processGithubMetrics(req *http.Request, resp *http.Response, path, statusCode string) error {
	if resp.Header.Get("X-From-Cache") == "1" {
		t.metrics.IncreaseGithubCacheHits(req.Method, path)
	} else {
		t.metrics.IncreaseGithubCacheMisses(req.Method, path)
	}

	if resp.Body != nil && statusCode == "403" {
		msg := struct {
			Message          string `json:"message"`
			DocumentationURL string `json:"documentation_url"`
		}{}
		// Read body to check if there is an error message,
		// then close it and re-assigning the body again for the
		// next layer so it could read the body without problem
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		resp.Body.Close()
		defer func() {
			resp.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
		}()

		err = json.Unmarshal(bodyBytes, &msg)
		if err == nil && t.hasExceedRateLimit(msg.Message) {
			t.metrics.IncreaseRateLimiterErrors()
		} else if err != nil {
			return err
		}
	}

	return nil
}

func (t *MetricsTransport) hasExceedRateLimit(msg string) bool {
	return strings.Contains(msg, "temporarily blocked") || strings.Contains(msg, "rate limit exceeded")
}

// Client returns a new http.Client using Transport
// as the default transport
func (t *MetricsTransport) Client() *http.Client {
	return &http.Client{Transport: t}
}
