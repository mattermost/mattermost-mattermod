package metrics

import (
	"net/http"
	"strconv"
	"time"
)

type Transport struct {
	Base    http.RoundTripper
	metrics Provider
}

func NewTransport(base http.RoundTripper, metrics Provider) *Transport {
	return &Transport{base, metrics}
}

func (t *Transport) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	start := time.Now()
	resp, err = t.Base.RoundTrip(req)
	elapsed := float64(time.Since(start)) / float64(time.Second)
	// rate limit error
	if resp == nil && err != nil {
		return resp, err
	}
	statusCode := strconv.Itoa(resp.StatusCode)
	t.metrics.ObserveGithubRequestDuration(req.Method, req.URL.Path, statusCode, elapsed)

	if resp.Header.Get("X-From-Cache") == "1" {
		t.metrics.IncreaseGithubCacheHits(req.Method, req.URL.Path)
	} else {
		t.metrics.IncreaseGithubCacheMisses(req.Method, req.URL.Path)
	}

	return resp, err
}

func (t *Transport) Client() *http.Client {
	return &http.Client{Transport: t}
}
