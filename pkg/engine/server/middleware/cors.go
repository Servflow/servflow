package middleware

import (
	"fmt"
	"net/http"
)

type Cors struct {
	AllowedOrigins []string
}

func (c *Cors) Name() string {
	return "CORS check"
}

func (c *Cors) Handle(w http.ResponseWriter, r *http.Request) error {
	origin := r.Header.Get("Origin")
	if c.isAllowedOrigin(origin) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
	} else {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return fmt.Errorf("%w: origin not allowed", ErrMiddlewareFailed)
	}

	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization")

	// Handle preflight request
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return nil
	}

	return nil
}

func (c *Cors) isAllowedOrigin(origin string) bool {
	if len(c.AllowedOrigins) == 0 {
		return true
	}
	for _, allowedOrigin := range c.AllowedOrigins {
		if origin == allowedOrigin {
			return true
		}
	}
	return false
}
