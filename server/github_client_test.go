// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/m4ns0ur/httpcache"
	"github.com/stretchr/testify/require"
)

func TestRevalidateTransport(t *testing.T) {
	const etag = `"v1"`
	var requests, conditional int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		if r.Header.Get("If-None-Match") == etag {
			atomic.AddInt32(&conditional, 1)
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Cache-Control", "private, max-age=60")
		w.Header().Set("ETag", etag)
		w.Header().Set("Date", time.Now().UTC().Format(http.TimeFormat))
		_, _ = w.Write([]byte("[]"))
	}))
	defer srv.Close()

	client := &http.Client{Transport: &revalidateTransport{next: httpcache.NewTransport(httpcache.NewMemoryCache())}}
	get := func(ctx context.Context) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, http.NoBody)
		require.NoError(t, err)
		resp, err := client.Do(req)
		require.NoError(t, err)
		_, err = io.Copy(io.Discard, resp.Body)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		require.Equal(t, http.StatusOK, resp.StatusCode)
	}

	get(context.Background())
	get(context.Background())
	require.EqualValues(t, 1, atomic.LoadInt32(&requests), "a read within max-age is served from cache")

	get(withRevalidation(context.Background()))
	require.EqualValues(t, 2, atomic.LoadInt32(&requests), "a revalidating read reaches GitHub")
	require.EqualValues(t, 1, atomic.LoadInt32(&conditional), "and sends the cached ETag")
}
