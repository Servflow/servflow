package server

import (
	"errors"
	"net/http"

	apiconfig "github.com/Servflow/servflow/pkg/apiconfig"
	"github.com/Servflow/servflow/pkg/engine/server/middleware"
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
