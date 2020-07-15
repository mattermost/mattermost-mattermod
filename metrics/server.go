// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package metrics

import (
	"context"
	"fmt"
	"net/http"
	"net/http/pprof"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost-server/v5/mlog"
)

type Server struct {
	port     string
	handlers []Handler

	server *http.Server
}

type Handler struct {
	Path        string
	Description string
	Handler     http.Handler
}

func NewServer(port string, handler Handler, pprof bool) *Server {
	handlers := []Handler{handler}
	if pprof {
		handlers = append(handlers, pprofHandlers()...)
	}
	return &Server{port: port, handlers: handlers}
}

// StartServer ...
func (m *Server) Start() {
	const (
		defaultHTTPServerReadTimeoutSeconds  = 30
		defaultHTTPServerWriteTimeoutSeconds = 30
	)

	router := mux.NewRouter()
	router.HandleFunc("/", m.handleRoot)
	for _, handler := range m.handlers {
		mlog.Debug("Adding metrics handler", mlog.String("path", handler.Path))
		router.Handle(handler.Path, handler.Handler)
	}

	m.server = &http.Server{
		Addr:         fmt.Sprintf(":%s", m.port),
		Handler:      router,
		ReadTimeout:  time.Duration(defaultHTTPServerReadTimeoutSeconds) * time.Second,
		WriteTimeout: time.Duration(defaultHTTPServerWriteTimeoutSeconds) * time.Second,
	}

	go func() {
		mlog.Info("Metrics and profiling server started", mlog.String("port", "8067"))
		if err := m.server.ListenAndServe(); err != nil {
			mlog.Error("Error trying to start the metrics server", mlog.Err(err))
			return
		}
	}()
}

// StopServer ...
func (m *Server) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := m.server.Shutdown(ctx); err != nil {
		mlog.Error("Error shutting down the metrics and profiling server", mlog.Err(err))
	}
	mlog.Info("Metrics and profiling server stopped")
}

func (m *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	builder := strings.Builder{}
	for _, handler := range m.handlers {
		builder.WriteString(fmt.Sprintf("<div><a href=\"%s\">%s</a></div>\n", handler.Path, handler.Description))
	}

	html := fmt.Sprintf(`
		<html>
			<body>
				%s
			</body>
		</html>
	`, builder.String())

	if _, err := w.Write([]byte(html)); err != nil {
		mlog.Error("Error rendering metrics page", mlog.Err(err))
	}

}

func pprofHandlers() []Handler {
	return []Handler{
		{Path: "/debug/pprof/", Description: "Profiling Root", Handler: http.HandlerFunc(pprof.Index)},
		{Path: "/debug/pprof/cmdline", Description: "Profiling Command Line", Handler: http.HandlerFunc(pprof.Cmdline)},
		{Path: "/debug/pprof/symbol", Description: "Profiling Symbols", Handler: http.HandlerFunc(pprof.Symbol)},
		{Path: "/debug/pprof/goroutine", Description: "Profiling Goroutines", Handler: pprof.Handler("goroutine")},
		{Path: "/debug/pprof/heap", Description: "Profiling Heap", Handler: pprof.Handler("heap")},
		{Path: "/debug/pprof/threadcreate", Description: "Profiling Threads", Handler: pprof.Handler("threadcreate")},
		{Path: "/debug/pprof/block", Description: "Profiling Blockings", Handler: pprof.Handler("block")},
	}
}
