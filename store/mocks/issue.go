// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/mattermost/mattermost-mattermod/store (interfaces: IssueStore)

// Package mocks is a generated GoMock package.
package mocks

import (
	gomock "github.com/golang/mock/gomock"
	model "github.com/mattermost/mattermost-mattermod/model"
	reflect "reflect"
)

// MockIssueStore is a mock of IssueStore interface
type MockIssueStore struct {
	ctrl     *gomock.Controller
	recorder *MockIssueStoreMockRecorder
}

// MockIssueStoreMockRecorder is the mock recorder for MockIssueStore
type MockIssueStoreMockRecorder struct {
	mock *MockIssueStore
}

// NewMockIssueStore creates a new mock instance
func NewMockIssueStore(ctrl *gomock.Controller) *MockIssueStore {
	mock := &MockIssueStore{ctrl: ctrl}
	mock.recorder = &MockIssueStoreMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockIssueStore) EXPECT() *MockIssueStoreMockRecorder {
	return m.recorder
}

// Get mocks base method
func (m *MockIssueStore) Get(arg0, arg1 string, arg2 int) (*model.Issue, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", arg0, arg1, arg2)
	ret0, _ := ret[0].(*model.Issue)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Get indicates an expected call of Get
func (mr *MockIssueStoreMockRecorder) Get(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockIssueStore)(nil).Get), arg0, arg1, arg2)
}

// Save mocks base method
func (m *MockIssueStore) Save(arg0 *model.Issue) (*model.Issue, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Save", arg0)
	ret0, _ := ret[0].(*model.Issue)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Save indicates an expected call of Save
func (mr *MockIssueStoreMockRecorder) Save(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Save", reflect.TypeOf((*MockIssueStore)(nil).Save), arg0)
}
