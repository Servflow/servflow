package server

import (
	"net/http"
	"net/http/pprof"
	"strings"

	apiconfig "github.com/Servflow/servflow/pkg/apiconfig"
	"github.com/Servflow/servflow/pkg/engine/plan"
	"github.com/Servflow/servflow/pkg/logging"
	"github.com/mark3labs/mcp-go/server"
	"go.uber.org/zap"

	"github.com/gorilla/mux"
)

// TODO move stuff and test engine easily
// TODO only expose profile if debug

func (e *Engine) createMuxHandler(configs []*apiconfig.APIConfig) *mux.Router {
	logger := logging.FromContext(e.ctx)
	if len(configs) == 0 {
		logger.Warn("no API configurations - engine will run with no API endpoints")
	}
	r := mux.NewRouter()

	// Add pprof routes
	r.Handle("/health", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	r.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
	r.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
	r.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
	r.Handle("/debug/pprof/trace", http.HandlerFunc(pprof.Trace))
	r.PathPrefix("/debug/pprof/").Handler(http.HandlerFunc(pprof.Index))

	for _, conf := range configs {
		listenPath := "/" + strings.Trim(conf.HttpConfig.ListenPath, "/")
		method := conf.HttpConfig.Method
		if method == "" {
			method = "GET"
		}

		if conf.IsMCPConfig() {
			err := e.createMCPHandler(conf)
			if err != nil {
				logger.Error("error creating mcp handler", zap.Error(err), zap.String("name", conf.McpTool.Name), zap.String("api", conf.ID))
			}
			continue
		}

		handler, err := e.createBasicHandler(conf)
		if err != nil {
			logger.Error("Error creating APIHandler", zap.Error(err), zap.String("api", conf.ID), zap.String("path", listenPath))
			continue
		}

		h := e.wrapMiddleware(handler)
		logger.Info("registered handler", zap.String("config_id", conf.ID))

		r.Handle(listenPath, h).Methods(method, http.MethodOptions)
	}

	if e.mcpServer != nil {
		httpHandler := server.NewStreamableHTTPServer(e.mcpServer)
		r.HandleFunc("/mcp", e.wrapMiddleware(httpHandler).ServeHTTP).Methods(http.MethodGet, http.MethodOptions, http.MethodPost)
	}

	return r
}

// wrapMiddleware installs the process-level concerns shared by every request:
// idle-timer reset, the background manager, and the request hook. The
// request-scoped facilities (request id, RequestContext, logger, span
// attributes) are opened by the terminal handlers via requestctx.Start.
func (e *Engine) wrapMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		e.resetIdleTimer()
		ctx := r.Context()
		if e.backgroundManager != nil {
			ctx = plan.WithBackgroundManager(ctx, e.backgroundManager)
		}
		r = r.WithContext(ctx)

		if e.requestHook != nil {
			if !e.requestHook(w, r) {
				return
			}
		}

		handler.ServeHTTP(w, r)
	})
}
