// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"flag"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/mlog"
	"github.com/xeipuuv/gojsonschema"
)

var config *ServerConfig
var err error
var s *Server

func TestMain(m *testing.M) {
	var configFile string
	flag.StringVar(&configFile, "config", "config-mattermod.default.json", "")
	flag.Parse()
	config, err = GetConfig(configFile)
	if err != nil {
		panic(err)
	}
	SetupLogging(config)
	mlog.Info("Loaded config", mlog.String("filename", configFile))

	s, err = New(config)
	if err != nil {
		panic(err)
	}
	s.Start()

	exitVal := m.Run()

	os.Exit(exitVal)
}

func TestPing(t *testing.T) {
	defer s.Stop()
	req, err := http.NewRequest("GET", config.ListenAddress, nil)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	s.ping(w, req)

	body := w.Result().Body
	defer body.Close()
	bytes, err := ioutil.ReadAll(body)
	require.NoError(t, err)
	bytesLoader := gojsonschema.NewBytesLoader(bytes)

	dir, err := filepath.Abs(filepath.Dir(""))
	if err != nil {
		mlog.Error("unable to find project path")
	}
	jsonLoader := gojsonschema.NewReferenceLoader("file://" + dir + "/schema/ping.schema.json")

	result, err := gojsonschema.Validate(jsonLoader, bytesLoader)
	if err != nil {
		mlog.Err(err)
	}
	if result.Valid() {
		mlog.Info("json is valid")
	} else {
		for _, err := range result.Errors() {
			mlog.Error(err.Description())
		}
		t.FailNow()
	}
}
