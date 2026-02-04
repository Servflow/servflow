package server

import (
	"fmt"
	"net/http"
	"net/http/pprof"
	"strings"
	"time"

	apiconfig "github.com/Servflow/servflow/pkg/apiconfig"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/Servflow/servflow/pkg/logging"
	"github.com/mark3labs/mcp-go/server"
	"go.uber.org/zap"

	"github.com/gorilla/mux"
)

// TODO move stuff and test engine easily
// TODO only expose profile if debug

func (e *Engine) createServer(port string) (*http.Server, error) {
	logger := logging.FromContext(e.ctx)

	logger.Info("starting engine on " + port)
	httpServer := &http.Server{
		Addr:    ":" + port,
		Handler: e.handler,
	}

	return httpServer, nil
}

func (e *Engine) createHandler() http.HandlerFunc {
	handler := e.createMuxHandler(e.directConfigs.APIConfigs)
	return handler.ServeHTTP
}

func (e *Engine) createMuxHandler(configs []*apiconfig.APIConfig) http.Handler {
	logger := logging.FromContext(e.ctx)
	if len(configs) == 0 {
		logger.Info("no api configurations")
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

		h := e.wrapMiddlewareWithReqIDLogger(e.logger, handler)
		logger.Info("registered handler for " + conf.ID)

		r.Handle(listenPath, h).Methods(method, http.MethodOptions)
	}

	if e.mcpServer != nil {
		httpHandler := server.NewStreamableHTTPServer(e.mcpServer)
		r.HandleFunc("/mcp", e.wrapMiddlewareWithReqIDLogger(e.logger, httpHandler).ServeHTTP).Methods(http.MethodGet, http.MethodOptions, http.MethodPost)
	}

	return r
}

func (e *Engine) wrapMiddlewareWithReqIDLogger(logger *zap.Logger, handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		e.resetIdleTimer()
		requestID := fmt.Sprintf("request_%d", time.Now().UnixNano())
		aggCtx := requestctx.NewRequestContext(requestID)
		ctx := requestctx.WithAggregationContext(r.Context(), aggCtx)
		logger := logger.With(zap.String("request_id", requestID), zap.String("method", r.Method), zap.String("path", r.URL.Path))
		ctx = logging.WithLogger(ctx, logger)
		r = r.WithContext(ctx)

		if e.requestHook != nil {
			if !e.requestHook(w, r) {
				return
			}
		}

		handler.ServeHTTP(w, r)
	})
}
