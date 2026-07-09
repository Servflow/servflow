// Package entryhandlers provides a registry of named HTTP entry handlers.
//
// An entry handler is HTTP middleware that wraps an incoming request after the
// standard request prerequisites have run (request context, request template
// functions, loaded files, raw request in context) and before the workflow plan
// executes. A handler can:
//
//   - reject the request (write a response, e.g. 401, and return without
//     calling next),
//   - short-circuit with a protocol response (write it and return without
//     calling next), or
//   - inject values into the request context and call next to run the plan.
//
// Handlers are referenced by type from apiconfig.HttpConfig.Handler, and their
// configuration (from HttpConfig.HandlerConfig) is passed directly to the
// middleware as the config argument.
package entryhandlers

import (
	"fmt"
	"net/http"
	"sort"
	"sync"
)

// Middleware wraps the workflow-plan handler. config is the entry handler's
// configuration (from HttpConfig.HandlerConfig); values may contain templates
// the handler resolves against the request context. Call next.ServeHTTP to
// proceed to the plan, or write a response and return without calling next to
// reject or short-circuit the request.
type Middleware func(config map[string]interface{}, next http.Handler) http.Handler

var (
	mu       sync.RWMutex
	registry = make(map[string]Middleware)
)

// Register makes a middleware available under handlerType. It panics on a
// duplicate registration, matching the responses registry, so conflicts surface
// at startup.
func Register(handlerType string, mw Middleware) {
	mu.Lock()
	defer mu.Unlock()
	if _, ok := registry[handlerType]; ok {
		panic(fmt.Sprintf("entry handler %q already registered", handlerType))
	}
	registry[handlerType] = mw
}

// Get returns the middleware registered for handlerType.
func Get(handlerType string) (Middleware, bool) {
	mu.RLock()
	defer mu.RUnlock()
	mw, ok := registry[handlerType]
	return mw, ok
}

// Has reports whether a handler is registered for handlerType.
func Has(handlerType string) bool {
	_, ok := Get(handlerType)
	return ok
}

// RegisteredTypes returns the registered handler types, sorted.
func RegisteredTypes() []string {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]string, 0, len(registry))
	for k := range registry {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
