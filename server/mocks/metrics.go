// Code generated by MockGen. DO NOT EDIT.
// Source: server/metrics.go

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
)

// MockMetricsProvider is a mock of MetricsProvider interface.
type MockMetricsProvider struct {
	ctrl     *gomock.Controller
	recorder *MockMetricsProviderMockRecorder
}

// MockMetricsProviderMockRecorder is the mock recorder for MockMetricsProvider.
type MockMetricsProviderMockRecorder struct {
	mock *MockMetricsProvider
}

// NewMockMetricsProvider creates a new mock instance.
func NewMockMetricsProvider(ctrl *gomock.Controller) *MockMetricsProvider {
	mock := &MockMetricsProvider{ctrl: ctrl}
	mock.recorder = &MockMetricsProviderMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockMetricsProvider) EXPECT() *MockMetricsProviderMockRecorder {
	return m.recorder
}

// IncreaseCronTaskErrors mocks base method.
func (m *MockMetricsProvider) IncreaseCronTaskErrors(name string) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "IncreaseCronTaskErrors", name)
}

// IncreaseCronTaskErrors indicates an expected call of IncreaseCronTaskErrors.
func (mr *MockMetricsProviderMockRecorder) IncreaseCronTaskErrors(name interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IncreaseCronTaskErrors", reflect.TypeOf((*MockMetricsProvider)(nil).IncreaseCronTaskErrors), name)
}

// IncreaseGithubCacheHits mocks base method.
func (m *MockMetricsProvider) IncreaseGithubCacheHits(method, handler string) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "IncreaseGithubCacheHits", method, handler)
}

// IncreaseGithubCacheHits indicates an expected call of IncreaseGithubCacheHits.
func (mr *MockMetricsProviderMockRecorder) IncreaseGithubCacheHits(method, handler interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IncreaseGithubCacheHits", reflect.TypeOf((*MockMetricsProvider)(nil).IncreaseGithubCacheHits), method, handler)
}

// IncreaseGithubCacheMisses mocks base method.
func (m *MockMetricsProvider) IncreaseGithubCacheMisses(method, handler string) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "IncreaseGithubCacheMisses", method, handler)
}

// IncreaseGithubCacheMisses indicates an expected call of IncreaseGithubCacheMisses.
func (mr *MockMetricsProviderMockRecorder) IncreaseGithubCacheMisses(method, handler interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IncreaseGithubCacheMisses", reflect.TypeOf((*MockMetricsProvider)(nil).IncreaseGithubCacheMisses), method, handler)
}

// IncreaseRateLimiterErrors mocks base method.
func (m *MockMetricsProvider) IncreaseRateLimiterErrors() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "IncreaseRateLimiterErrors")
}

// IncreaseRateLimiterErrors indicates an expected call of IncreaseRateLimiterErrors.
func (mr *MockMetricsProviderMockRecorder) IncreaseRateLimiterErrors() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IncreaseRateLimiterErrors", reflect.TypeOf((*MockMetricsProvider)(nil).IncreaseRateLimiterErrors))
}

// IncreaseWebhookErrors mocks base method.
func (m *MockMetricsProvider) IncreaseWebhookErrors(name string) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "IncreaseWebhookErrors", name)
}

// IncreaseWebhookErrors indicates an expected call of IncreaseWebhookErrors.
func (mr *MockMetricsProviderMockRecorder) IncreaseWebhookErrors(name interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IncreaseWebhookErrors", reflect.TypeOf((*MockMetricsProvider)(nil).IncreaseWebhookErrors), name)
}

// IncreaseWebhookRequest mocks base method.
func (m *MockMetricsProvider) IncreaseWebhookRequest(name string) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "IncreaseWebhookRequest", name)
}

// IncreaseWebhookRequest indicates an expected call of IncreaseWebhookRequest.
func (mr *MockMetricsProviderMockRecorder) IncreaseWebhookRequest(name interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IncreaseWebhookRequest", reflect.TypeOf((*MockMetricsProvider)(nil).IncreaseWebhookRequest), name)
}

// ObserveCronTaskDuration mocks base method.
func (m *MockMetricsProvider) ObserveCronTaskDuration(name string, elapsed float64) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "ObserveCronTaskDuration", name, elapsed)
}

// ObserveCronTaskDuration indicates an expected call of ObserveCronTaskDuration.
func (mr *MockMetricsProviderMockRecorder) ObserveCronTaskDuration(name, elapsed interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ObserveCronTaskDuration", reflect.TypeOf((*MockMetricsProvider)(nil).ObserveCronTaskDuration), name, elapsed)
}

// ObserveGithubRequestDuration mocks base method.
func (m *MockMetricsProvider) ObserveGithubRequestDuration(method, handler, statusCode string, elapsed float64) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "ObserveGithubRequestDuration", method, handler, statusCode, elapsed)
}

// ObserveGithubRequestDuration indicates an expected call of ObserveGithubRequestDuration.
func (mr *MockMetricsProviderMockRecorder) ObserveGithubRequestDuration(method, handler, statusCode, elapsed interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ObserveGithubRequestDuration", reflect.TypeOf((*MockMetricsProvider)(nil).ObserveGithubRequestDuration), method, handler, statusCode, elapsed)
}

// ObserveHTTPRequestDuration mocks base method.
func (m *MockMetricsProvider) ObserveHTTPRequestDuration(method, handler, statusCode string, elapsed float64) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "ObserveHTTPRequestDuration", method, handler, statusCode, elapsed)
}

// ObserveHTTPRequestDuration indicates an expected call of ObserveHTTPRequestDuration.
func (mr *MockMetricsProviderMockRecorder) ObserveHTTPRequestDuration(method, handler, statusCode, elapsed interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ObserveHTTPRequestDuration", reflect.TypeOf((*MockMetricsProvider)(nil).ObserveHTTPRequestDuration), method, handler, statusCode, elapsed)
}
