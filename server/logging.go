// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"testing"

	"github.com/wiggin77/logr"
)

type field struct {
	name string
	val  interface{}
}

// TestingTarget is a Logr target proxies logs through a stdlib testing interface.
// This allows tests that spin up App instances to avoid spewing logs unless the test fails or -verbose is specified.
type TestingTarget struct {
	logr.Basic
	tb testing.TB
}

// NewTestingTarget creates a target that proxies logs through a stdlib testing interface.
func NewTestingTarget(filter logr.Filter, tb testing.TB, maxQueue int) (*TestingTarget, error) {
	tt := &TestingTarget{
		tb: tb,
	}
	tt.Basic.Start(tt, tt, filter, nil, maxQueue)

	return tt, nil
}

// Write converts the log record to bytes, via the Formatter, and outputs to the socket.
// Called by dedicated target goroutine and will block until success or shutdown.
func (tt *TestingTarget) Write(rec *logr.LogRec) error {
	recFlds := rec.Fields()
	args := make([]interface{}, len(recFlds)+2)
	args = append(args, rec.Level())
	args = append(args, rec.Msg())
	for k, v := range recFlds {
		args = append(args, field{name: k, val: v})
	}

	switch rec.Level().ID {
	case logr.Error.ID, logr.Fatal.ID, logr.Panic.ID:
		tt.tb.Error(args...)
	default:
		tt.tb.Log(args...)
	}
	return nil
}
