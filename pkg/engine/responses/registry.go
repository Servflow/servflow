// Package responses is the registry of workflow response types. A response kind
// registers a Factory that builds a ResponseBuilder from a response config; the
// plan package looks the factory up by kind. The concrete type implementations
// live in subpackages (e.g. responses/http) that register themselves at init,
// exactly like the action registry and its executables.
package responses

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/Servflow/servflow/pkg/apiconfig"
)

// Result is the polymorphic output of a workflow's response step. Each response
// kind returns its own concrete Result (the built-in "http" kind returns
// *sfhttp.SfResponse); consumers type-assert to the concrete type they expect.
// Kind is for logging/tracing only — it is NOT a render fallback.
type Result interface {
	Kind() string
}

// DefaultKind is the response kind used when a response config leaves Kind empty.
const DefaultKind = "http"

// ResolveKind returns the effective response kind for a config value, applying
// the default. Plan-build and validation both call it so they cannot diverge on
// what an empty kind means.
func ResolveKind(kind string) string {
	if kind == "" {
		return DefaultKind
	}
	return kind
}

// ResponseBuilder turns the request context into a concrete response.
type ResponseBuilder interface {
	BuildResponse(ctx context.Context) (Result, error)
}

// Factory constructs a ResponseBuilder for a configured response. It is
// registered once, at init, by the package implementing a response kind.
type Factory func(cfg apiconfig.ResponseConfig) (ResponseBuilder, error)

var (
	mu       sync.RWMutex
	registry = make(map[string]Factory)
)

// RegisterResponseType registers the factory for a response kind. It panics on a
// duplicate kind: registration happens at init, so a clash is a programming
// error (mirrors how the action registry is populated by its executables).
func RegisterResponseType(kind string, f Factory) {
	mu.Lock()
	defer mu.Unlock()
	if _, exists := registry[kind]; exists {
		panic(fmt.Sprintf("responses: response type %q already registered", kind))
	}
	registry[kind] = f
}

// Get returns the factory registered for a kind.
func Get(kind string) (Factory, bool) {
	mu.RLock()
	defer mu.RUnlock()
	f, ok := registry[kind]
	return f, ok
}

// HasRegisteredType reports whether a response kind is registered.
func HasRegisteredType(kind string) bool {
	mu.RLock()
	defer mu.RUnlock()
	_, ok := registry[kind]
	return ok
}

// RegisteredTypes returns the registered kinds, sorted.
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
