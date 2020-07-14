// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	metricsNamespace = "mattermod"
	httpNamespace    = "requests"

	defaultPrometheusTimeoutSeconds = 60
)

type PrometheusProvider struct {
	Registry *prometheus.Registry

	httpRequests *prometheus.HistogramVec
}

func NewPrometheusProvider() *PrometheusProvider {
	service := &PrometheusProvider{}
	service.Registry = prometheus.NewRegistry()
	options := prometheus.ProcessCollectorOpts{
		Namespace: metricsNamespace,
	}
	service.Registry.MustRegister(prometheus.NewProcessCollector(options))
	service.Registry.MustRegister(prometheus.NewGoCollector())

	service.httpRequests = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: metricsNamespace,
			Subsystem: httpNamespace,
			Name:      "requests",
			Help:      "Received http requests.",
		},
		[]string{"method", "handler", "status_code"},
	)
	service.Registry.MustRegister(service.httpRequests)

	return service
}

func (p *PrometheusProvider) Handler() http.Handler {
	return promhttp.HandlerFor(p.Registry, promhttp.HandlerOpts{
		Timeout:           time.Duration(defaultPrometheusTimeoutSeconds) * time.Second,
		EnableOpenMetrics: true,
	})
}
