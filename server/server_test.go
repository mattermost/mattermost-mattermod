// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/mattermost/mattermost-mattermod/server/mocks"
	"github.com/stretchr/testify/require"
)

func TestPing(t *testing.T) {
	s := &Server{
		StartTime: time.Now(),
	}

	ts := httptest.NewServer(http.HandlerFunc(s.ping))
	defer ts.Close()

	res, err := http.Get(ts.URL)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, res.StatusCode)
	defer res.Body.Close()

	var ping pingResponse

	err = json.NewDecoder(res.Body).Decode(&ping)
	require.NoError(t, err)
	require.NotZero(t, ping.Uptime)
	_, err = time.ParseDuration(ping.Uptime)
	require.NoError(t, err)
}

type panicHandler struct {
}

func (ph panicHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	panic("bad handler")
}

func TestWithRecovery(t *testing.T) {
	s := Server{}
	defer func() {
		if x := recover(); x != nil {
			require.Fail(t, "got panic")
		}
	}()

	ph := panicHandler{}
	handler := s.withRecovery(ph)

	req := httptest.NewRequest("GET", "http://random", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.Body != nil {
		defer resp.Body.Close()
		_, err := io.Copy(ioutil.Discard, resp.Body)
		require.NoError(t, err)
	}
}

func TestGithubEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	metricsMock := mocks.NewMockMetricsProvider(ctrl)
	s := &Server{
		StartTime: time.Now(),
		Metrics:   metricsMock,
	}

	h := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}

	handler := s.withRequestDuration(http.HandlerFunc(h))
	ts := httptest.NewServer(handler)
	defer ts.Close()

	t.Run("Should observe http request", func(t *testing.T) {
		metricsMock.EXPECT().ObserveHTTPRequestDuration(
			gomock.Eq("POST"),
			gomock.Eq("/"),
			gomock.Eq("201"),
			gomock.Any(),
		).Times(1)

		req, err := http.NewRequest("POST", ts.URL, nil)
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		defer resp.Body.Close()
	})
}
