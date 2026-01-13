package server

import (
	"errors"
	"net/http"

	"github.com/Servflow/servflow/pkg/apiconfig"
	"github.com/Servflow/servflow/pkg/engine/server/middleware"
	"github.com/justinas/alice"
)

func resolveCORSOrigins(apiCors []string, engineCors *CorsConfig) []string {
	if len(apiCors) > 0 {
		return apiCors
	}
	if engineCors != nil && len(engineCors.AllowedOrigins) > 0 {
		return engineCors.AllowedOrigins
	}
	return nil
}

func (h *APIHandler) CreateChain(config *apiconfig.APIConfig, engineCors *CorsConfig) http.Handler {
	corsOrigins := resolveCORSOrigins(config.HttpConfig.CORSAllowedOrigins, engineCors)
	chain := alice.New(
		h.middlewareAdaptor(&middleware.Cors{AllowedOrigins: corsOrigins}),
	).Then(h)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chain.ServeHTTP(w, r)
	})
}

func (h *APIHandler) middlewareAdaptor(m middleware.Middleware) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			err := m.Handle(w, req)
			if err != nil {
				if errors.Is(err, middleware.ErrMiddlewareFailed) {
					return
				} else {
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
