package server

import (
	"errors"
	"net/http"

	"github.com/Servflow/servflow/pkg/definitions"
	"github.com/Servflow/servflow/pkg/engine/server/middleware"
	"github.com/Servflow/servflow/pkg/logging"
	"go.uber.org/zap"

	"github.com/justinas/alice"
)

func (h *APIHandler) CreateChain(config *apiconfig.APIConfig) http.Handler {
	chain := alice.New(
		h.middlewareAdaptor(&middleware.Cors{AllowedOrigins: config.HttpConfig.CORSAllowedOrigins}),
	).Then(h)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chain.ServeHTTP(w, r)
	})
}

func (h *APIHandler) middlewareAdaptor(m middleware.Middleware) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			logger := logging.GetRequestLogger(req.Context())
			err := m.Handle(w, req)
			if err != nil {
				if errors.Is(err, middleware.ErrMiddlewareFailed) {
					logger.Warn("middleware failed", zap.Error(err))
					return
				} else {
					logger.Error("middleware failed", zap.Error(err))
					return
				}
			}
			if req.Method == http.MethodOptions {
				return
			}
			next.ServeHTTP(w, req)
		})
	}
}
