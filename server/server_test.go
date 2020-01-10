// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/mattermost/mattermost-server/mlog"
	"github.com/pkg/errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/qri-io/jsonschema"
)

var config *ServerConfig
var err error
var s *Server

func TestMain(m *testing.M) {
	var configFile string
	flag.StringVar(&configFile, "config", "config-mattermod.circeci-test.json", "")
	flag.Parse()
	config, err = GetConfig(configFile)
	if err != nil {
		errors.Wrap(err, "unable to load server config")
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
		panic("no body returned. 500. " + err.Error())
	}

	schemaData, _ := ioutil.ReadFile("./schema/ping.schema.json")
	rs := &jsonschema.RootSchema{}
	if err := json.Unmarshal(schemaData, rs); err != nil {
		panic("unmarshal schema: " + err.Error())
	}

	if valErr, _ := rs.ValidateBytes(body); len(valErr) > 0 {
		fmt.Println(valErr[0].Error())
		t.FailNow()
	}
}
