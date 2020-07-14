// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	metricsNamespace = "mattermod"
	httpNamespace    = "requests"
	cronNamespace    = "cron"
	githubNamespace  = "github"

	defaultPrometheusTimeoutSeconds = 60
)

type Provider interface {
	ObserveHTTPRequestDuration(handler, method, statusCode string, elapsed float64)
	IncreaseWebhookRequest(name string)

	ObserveGithubRequestDuration(handler, method, statusCode string, elapsed float64)
	IncreaseGithubCacheHits(method, handler string)
	IncreaseGithubCacheMisses(method, handler string)

	ObserveCronTaskDuration(name string, elapsed float64)
	IncreaseCronTaskErrors(name string)
}

type PrometheusProvider struct {
	Registry *prometheus.Registry

	httpRequestsDuration *prometheus.HistogramVec
	webhookEvents        *prometheus.CounterVec

	cronTasksDuration *prometheus.HistogramVec
	cronTasksErrors   *prometheus.CounterVec

	githubRequests    *prometheus.HistogramVec
	githubCacheHits   *prometheus.CounterVec
	githubCacheMisses *prometheus.CounterVec
}

func NewPrometheusProvider() *PrometheusProvider {
	provider := &PrometheusProvider{}
	provider.Registry = prometheus.NewRegistry()
	options := prometheus.ProcessCollectorOpts{
		Namespace: metricsNamespace,
	}
	provider.Registry.MustRegister(prometheus.NewProcessCollector(options))
	provider.Registry.MustRegister(prometheus.NewGoCollector())

	provider.httpRequestsDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: metricsNamespace,
			Subsystem: httpNamespace,
			Name:      "requests",
			Help:      "Received http requests.",
		},
		[]string{"method", "handler", "status_code"},
	)
	provider.Registry.MustRegister(provider.httpRequestsDuration)

	provider.webhookEvents = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Subsystem: httpNamespace,
			Name:      "webhook_requests",
			Help:      "Number webhook requests by type.",
		},
		[]string{"type"},
	)
	provider.Registry.MustRegister(provider.webhookEvents)

	provider.cronTasksDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: metricsNamespace,
			Subsystem: cronNamespace,
			Name:      "tasks",
			Help:      "Duration for the executed cron tasks.",
		},
		[]string{"name"},
	)
	provider.Registry.MustRegister(provider.cronTasksDuration)

	provider.cronTasksErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Subsystem: cronNamespace,
			Name:      "errors",
			Help:      "Number of failed cron tasks.",
		},
		[]string{"name"},
	)
	provider.Registry.MustRegister(provider.cronTasksErrors)

	provider.githubRequests = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: metricsNamespace,
			Subsystem: githubNamespace,
			Name:      "requests",
			Help:      "Duration of the performed github http requests.",
		},
		[]string{"method", "handler", "status_code"},
	)
	provider.Registry.MustRegister(provider.githubRequests)

	provider.githubCacheHits = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Subsystem: githubNamespace,
			Name:      "cache_hits",
			Help:      "Number of cache hits for requested method and handler.",
		},
		[]string{"method", "handler"},
	)

	provider.githubCacheMisses = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Subsystem: githubNamespace,
			Name:      "cache_miss",
			Help:      "Number of cache misses for requested method and handler.",
		},
		[]string{"method", "handler"},
	)

	return provider
}

func (p *PrometheusProvider) ObserveHTTPRequestDuration(handler, method, statusCode string, elapsed float64) {
	p.httpRequestsDuration.With(
		prometheus.Labels{"method": method, "handler": handler, "status_code": statusCode},
	).Observe(elapsed)
}

func (p *PrometheusProvider) ObserveGithubRequestDuration(handler, method, statusCode string, elapsed float64) {
	p.githubRequests.With(
		prometheus.Labels{"method": method, "handler": handler, "status_code": statusCode},
	).Observe(elapsed)
}

func (p *PrometheusProvider) IncreaseWebhookRequest(name string) {
	p.cronTasksErrors.WithLabelValues(name).Add(1)
}

func (p *PrometheusProvider) ObserveCronTaskDuration(name string, elapsed float64) {
	p.cronTasksDuration.With(prometheus.Labels{"name": name}).Observe(elapsed)
}

func (p *PrometheusProvider) IncreaseCronTaskErrors(name string) {
	p.cronTasksErrors.WithLabelValues(name).Add(1)
}

func (p *PrometheusProvider) IncreaseGithubCacheHits(method, handler string) {
	p.githubCacheHits.WithLabelValues(method, handler).Add(1)
}

func (p *PrometheusProvider) IncreaseGithubCacheMisses(method, handler string) {
	p.githubCacheMisses.WithLabelValues(method, handler).Add(1)
}

func (p *PrometheusProvider) Handler() Handler {
	handler := promhttp.HandlerFor(p.Registry, promhttp.HandlerOpts{
		Timeout:           time.Duration(defaultPrometheusTimeoutSeconds) * time.Second,
		EnableOpenMetrics: true,
	})
	return Handler{
		Path:        "/metrics",
		Description: "Prometheus Metrics",
		Handler:     handler,
	}
}
