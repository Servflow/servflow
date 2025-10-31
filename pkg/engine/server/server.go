package server

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/pprof"
	"strings"
	"time"

	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/Servflow/servflow/pkg/logging"
	"github.com/mark3labs/mcp-go/server"
	"go.uber.org/zap"

	"github.com/Servflow/servflow/pkg/definitions"
	"github.com/gorilla/mux"
)

// TODO move stuff and test engine easily
// TODO only expose profile if debug

func (e *Engine) createServer(apiConfigs []*apiconfig.APIConfig, port string) (*http.Server, error) {
	if len(apiConfigs) < 1 {
		return nil, errors.New("no configuration files found")
	}

	logging.GetLogger().Info("starting engine on " + port)
	httpServer := &http.Server{
		Addr:    ":" + port,
		Handler: e.createCustomMuxHandler(apiConfigs),
	}

	return httpServer, nil
}

func (e *Engine) createCustomMuxHandler(configs []*apiconfig.APIConfig) http.Handler {
	logger := logging.GetLogger()
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

		if conf.McpTool.Name != "" {
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

		h := wrapMiddleware(handler)
		logger.Info("registered handler for " + conf.ID)

		r.Handle(listenPath, h).Methods(method, http.MethodOptions)
	}

	if e.mcpServer != nil {
		httpHandler := server.NewStreamableHTTPServer(e.mcpServer)
		r.HandleFunc("/mcp", wrapMiddleware(httpHandler).ServeHTTP).Methods(http.MethodGet, http.MethodOptions, http.MethodPost)
	}

	return r
}

func wrapMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		aggCtx := requestctx.NewRequestContext(fmt.Sprintf("request_%d", time.Now().UnixNano()))
		ctx := requestctx.WithAggregationContext(r.Context(), aggCtx)
		handler.ServeHTTP(w, r.WithContext(ctx))
	})
}
