// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	metricsNamespace = "mattermod"
	httpNamespace    = "requests"
	cronNamespace    = "cron"
	githubNamespace  = "github"

	defaultPrometheusTimeoutSeconds = 60
)

// PrometheusProvider is the implementation of the Provider interface
// to send metrics to Prometheus.
type PrometheusProvider struct {
	Registry *prometheus.Registry

	httpRequestsDuration *prometheus.HistogramVec
	webhookEvents        *prometheus.CounterVec
	webhookErrors        *prometheus.CounterVec

	cronTasksDuration *prometheus.HistogramVec
	cronTasksErrors   *prometheus.CounterVec

	githubRequests    *prometheus.HistogramVec
	githubCacheHits   *prometheus.CounterVec
	githubCacheMisses *prometheus.CounterVec

	rateLimiterErrors prometheus.Counter
}

// NewPrometheusProvider creates a new prometheus metrics provider
// It'll create the provider and initialize all the needed
// metric objects.
func NewPrometheusProvider() *PrometheusProvider {
	provider := &PrometheusProvider{}
	provider.Registry = prometheus.NewRegistry()
	options := collectors.ProcessCollectorOpts{
		Namespace: metricsNamespace,
	}
	provider.Registry.MustRegister(collectors.NewProcessCollector(options))
	provider.Registry.MustRegister(collectors.NewGoCollector())

	provider.httpRequestsDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: metricsNamespace,
			Subsystem: httpNamespace,
			Name:      "requests",
			Help:      "HTTP requests by different categories.",
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

	provider.webhookErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Subsystem: httpNamespace,
			Name:      "webhook_errors",
			Help:      "Number of webhook errors by type.",
		},
		[]string{"type"},
	)
	provider.Registry.MustRegister(provider.webhookErrors)

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
	provider.Registry.MustRegister(provider.githubCacheHits)

	provider.githubCacheMisses = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Subsystem: githubNamespace,
			Name:      "cache_miss",
			Help:      "Number of cache misses for requested method and handler.",
		},
		[]string{"method", "handler"},
	)
	provider.Registry.MustRegister(provider.githubCacheMisses)

	provider.rateLimiterErrors = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Subsystem: httpNamespace,
			Name:      "rate_limit_errors",
			Help:      "Number of rate limit errors.",
		},
	)
	provider.Registry.MustRegister(provider.rateLimiterErrors)

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
	p.webhookEvents.WithLabelValues(name).Add(1)
}

func (p *PrometheusProvider) IncreaseWebhookErrors(name string) {
	p.webhookErrors.WithLabelValues(name).Add(1)
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

func (p *PrometheusProvider) IncreaseRateLimiterErrors() {
	p.rateLimiterErrors.Add(1)
}

// Handler returns the handler that would be used by the metrics server to expose
// the metrics.
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
