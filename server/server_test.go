// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xeipuuv/gojsonschema"
)

func TestPing(t *testing.T) {
	s := &Server{
		StartTime: time.Now(),
	}

	ts := httptest.NewServer(http.HandlerFunc(s.ping))
	defer ts.Close()

	res, err := http.Get(ts.URL)
	require.NoError(t, err)
	bytes, err := ioutil.ReadAll(res.Body)
	require.NoError(t, err)
	bytesLoader := gojsonschema.NewBytesLoader(bytes)

	dir, err := filepath.Abs(filepath.Dir(""))
	assert.NoError(t, err)

	jsonLoader := gojsonschema.NewReferenceLoader("file://" + dir + "/schema/ping.schema.json")
	result, err := gojsonschema.Validate(jsonLoader, bytesLoader)
	assert.NoError(t, err)
	assert.True(t, result.Valid())
	if !result.Valid() {
		for _, err := range result.Errors() {
			t.Log(err.Description())
		}
	}
}
