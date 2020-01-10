// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"flag"
	"github.com/mattermost/mattermost-server/mlog"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/xeipuuv/gojsonschema"
)

var config *ServerConfig
var err error
var s *Server

func TestMain(m *testing.M) {
	var configFile string
	flag.StringVar(&configFile, "config", "config-mattermod.test-local.json", "")
	flag.Parse()
	config, err = GetConfig(configFile)
	if err != nil {
		mlog.Err(err)
	}
	SetupLogging(config)
	mlog.Info("Loaded config", mlog.String("filename", configFile))

	s = New(config)
	s.Start()

	exitVal := m.Run()

	os.Exit(exitVal)
}

func TestPing(t *testing.T) {
	defer s.Stop()
	req, _ := http.NewRequest("GET", config.ListenAddress, nil)
	w := httptest.NewRecorder()
	s.ping(w, req)

	var body []byte
	var err error
	if body, err = ioutil.ReadAll(w.Result().Body); err != nil {
		mlog.Err(err)
	}
	bytesLoader := gojsonschema.NewBytesLoader(body)

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
