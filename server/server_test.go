// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
	defer res.Body.Close()

	var ping pingResponse

	err = json.NewDecoder(res.Body).Decode(&ping)
	require.NoError(t, err)
	require.NotZero(t, ping.Uptime)
}
