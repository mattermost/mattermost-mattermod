// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package metrics

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	prometheusModels "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
)

func TestMetrics(t *testing.T) {
	provider := NewPrometheusProvider()
	server := NewServer("12345", provider.Handler(), false)
	server.Start()
	time.Sleep(time.Second * 1)
	defer server.Stop()

	t.Run("Should store metrics for requests duration", func(t *testing.T) {
		m := &prometheusModels.Metric{}
		data, err := provider.httpRequestsDuration.GetMetricWith(prometheus.Labels{"handler": "handler", "method": "method", "status_code": "200"})
		require.NoError(t, err)
		require.NoError(t, data.(prometheus.Histogram).Write(m))
		require.Equal(t, uint64(0), m.Histogram.GetSampleCount())
		require.Equal(t, 0.0, m.Histogram.GetSampleSum())
		provider.ObserveHTTPRequestDuration("handler", "method", "200", 1)
		data, err = provider.httpRequestsDuration.GetMetricWith(prometheus.Labels{"handler": "handler", "method": "method", "status_code": "200"})
		require.NoError(t, err)
		require.NoError(t, data.(prometheus.Histogram).Write(m))
		require.Equal(t, uint64(1), m.Histogram.GetSampleCount())
		require.InDelta(t, 1, m.Histogram.GetSampleSum(), 0.001)
	})

	t.Run("Should store metrics for webhook requests", func(t *testing.T) {
		m := &prometheusModels.Metric{}
		data, err := provider.webhookEvents.GetMetricWithLabelValues("test")
		require.NoError(t, err)
		require.NoError(t, data.(prometheus.Counter).Write(m))
		require.Equal(t, float64(0), m.Counter.GetValue())
		provider.IncreaseWebhookRequest("test")
		data, err = provider.webhookEvents.GetMetricWithLabelValues("test")
		require.NoError(t, err)
		require.NoError(t, data.(prometheus.Counter).Write(m))
		require.Equal(t, float64(1), m.Counter.GetValue())
	})

	t.Run("Should store metrics for github requests duration", func(t *testing.T) {
		m := &prometheusModels.Metric{}
		data, err := provider.githubRequests.GetMetricWith(prometheus.Labels{"handler": "handler", "method": "method", "status_code": "200"})
		require.NoError(t, err)
		require.NoError(t, data.(prometheus.Histogram).Write(m))
		require.Equal(t, uint64(0), m.Histogram.GetSampleCount())
		require.Equal(t, 0.0, m.Histogram.GetSampleSum())
		provider.ObserveGithubRequestDuration("handler", "method", "200", 1)
		data, err = provider.githubRequests.GetMetricWith(prometheus.Labels{"handler": "handler", "method": "method", "status_code": "200"})
		require.NoError(t, err)
		require.NoError(t, data.(prometheus.Histogram).Write(m))
		require.Equal(t, uint64(1), m.Histogram.GetSampleCount())
		require.InDelta(t, 1, m.Histogram.GetSampleSum(), 0.001)
	})

	t.Run("Should store metrics for github requests cache hits", func(t *testing.T) {
		m := &prometheusModels.Metric{}
		data, err := provider.githubCacheHits.GetMetricWithLabelValues("GET", "test")
		require.NoError(t, err)
		require.NoError(t, data.(prometheus.Counter).Write(m))
		require.Equal(t, float64(0), m.Counter.GetValue())
		provider.IncreaseGithubCacheHits("GET", "test")
		data, err = provider.githubCacheHits.GetMetricWithLabelValues("GET", "test")
		require.NoError(t, err)
		require.NoError(t, data.(prometheus.Counter).Write(m))
		require.Equal(t, float64(1), m.Counter.GetValue())
	})

	t.Run("Should store metrics for github requests cache misses", func(t *testing.T) {
		m := &prometheusModels.Metric{}
		data, err := provider.githubCacheMisses.GetMetricWithLabelValues("GET", "test")
		require.NoError(t, err)
		require.NoError(t, data.(prometheus.Counter).Write(m))
		require.Equal(t, float64(0), m.Counter.GetValue())
		provider.IncreaseGithubCacheMisses("GET", "test")
		data, err = provider.githubCacheMisses.GetMetricWithLabelValues("GET", "test")
		require.NoError(t, err)
		require.NoError(t, data.(prometheus.Counter).Write(m))
		require.Equal(t, float64(1), m.Counter.GetValue())
	})

	t.Run("Should store metrics for cron tasks duration", func(t *testing.T) {
		m := &prometheusModels.Metric{}
		data, err := provider.cronTasksDuration.GetMetricWith(prometheus.Labels{"name": "test-task"})
		require.NoError(t, err)
		require.NoError(t, data.(prometheus.Histogram).Write(m))
		require.Equal(t, uint64(0), m.Histogram.GetSampleCount())
		require.Equal(t, 0.0, m.Histogram.GetSampleSum())
		provider.ObserveCronTaskDuration("test-task", 1)
		data, err = provider.cronTasksDuration.GetMetricWith(prometheus.Labels{"name": "test-task"})
		require.NoError(t, err)
		require.NoError(t, data.(prometheus.Histogram).Write(m))
		require.Equal(t, uint64(1), m.Histogram.GetSampleCount())
		require.InDelta(t, 1, m.Histogram.GetSampleSum(), 0.001)
	})

	t.Run("Should store metrics for cron tasks errors", func(t *testing.T) {
		m := &prometheusModels.Metric{}
		data, err := provider.cronTasksErrors.GetMetricWithLabelValues("test-task")
		require.NoError(t, err)
		require.NoError(t, data.(prometheus.Counter).Write(m))
		require.Equal(t, float64(0), m.Counter.GetValue())
		provider.IncreaseCronTaskErrors("test-task")
		data, err = provider.cronTasksErrors.GetMetricWithLabelValues("test-task")
		require.NoError(t, err)
		require.NoError(t, data.(prometheus.Counter).Write(m))
		require.Equal(t, float64(1), m.Counter.GetValue())
	})
}
