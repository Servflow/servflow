// Package outputs is the registry of output handlers — the way OUT of a run,
// mirroring pkg/engine/entryhandlers on the way IN.
//
// An output handler extracts a run's output from the request context once the
// workflow plan has finished, so a workflow does not need an explicitly
// configured terminus (a Response step, a result template) to produce output.
// Handlers are referenced by kind from apiconfig.OutputConfig.Handler and
// configured by apiconfig.OutputConfig.Config.
//
// Extraction is deliberately protocol-free: three of the four run entry points
// (trigger, MCP tool, workflow tool) have no http.ResponseWriter, so an
// Extractor produces a value — a responses.Result — and each caller renders it
// in its own protocol. That is the split against the responses registry: an
// output handler decides WHAT the run's output is; the caller (and, for a
// configured Response step, a response kind) decides HOW it is rendered.
//
// Unlike the responses registry, the built-in kinds register themselves in this
// package's own init rather than in blank-imported subpackages. They depend only
// on requestctx, so there is nothing to isolate, and an output handler is on the
// default path for a run — a binary that forgot a blank import would fail at
// request time rather than never.
package outputs

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/Servflow/servflow/pkg/apiconfig"
	"github.com/Servflow/servflow/pkg/engine/responses"
)

// Extractor produces a run's output from the request context after the plan has
// finished. Implementations are built once, at config-load time, by a Factory.
type Extractor interface {
	Extract(ctx context.Context) (responses.Result, error)
}

// ExtractorFunc adapts a function to the Extractor interface.
type ExtractorFunc func(ctx context.Context) (responses.Result, error)

func (f ExtractorFunc) Extract(ctx context.Context) (responses.Result, error) {
	return f(ctx)
}

// Factory builds an Extractor from an output handler's configuration. It is
// registered once, at init, by the package implementing a handler kind, and is
// called at config-load time so a bad configuration fails before any request.
type Factory func(cfg map[string]interface{}) (Extractor, error)

var (
	mu       sync.RWMutex
	registry = make(map[string]Factory)
)

// Register makes a factory available under kind. It panics on a duplicate
// registration, matching the responses and entryhandlers registries, so
// conflicts surface at startup.
func Register(kind string, f Factory) {
	mu.Lock()
	defer mu.Unlock()
	if _, ok := registry[kind]; ok {
		panic(fmt.Sprintf("outputs: output handler %q already registered", kind))
	}
	registry[kind] = f
}

// Get returns the factory registered for kind.
func Get(kind string) (Factory, bool) {
	mu.RLock()
	defer mu.RUnlock()
	f, ok := registry[kind]
	return f, ok
}

// Has reports whether an output handler is registered for kind.
func Has(kind string) bool {
	_, ok := Get(kind)
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

// Resolve builds the Extractor for an output config. A config with no handler
// yields a nil Extractor and no error: not configuring output is valid — the
// run either terminates in a Response step or produces nothing.
func Resolve(cfg apiconfig.OutputConfig) (Extractor, error) {
	if cfg.Handler == "" {
		return nil, nil
	}
	f, ok := Get(cfg.Handler)
	if !ok {
		return nil, fmt.Errorf("unknown output handler %q (registered: %v)", cfg.Handler, RegisteredTypes())
	}
	ex, err := f(cfg.Config)
	if err != nil {
		return nil, fmt.Errorf("output handler %q: %w", cfg.Handler, err)
	}
	return ex, nil
}

// Finalize produces a run's output and is the single rule shared by every run
// entry point: a Result that the plan already produced — meaning the workflow
// terminated in a Response step, an explicit terminus that may be conditional or
// mid-graph — wins; only when the chain finished without one does the configured
// output handler extract from the request context.
//
// It is called at the run boundary rather than inside plan.Execute because
// Execute also runs sub-chains (dispatched chains, action chains, and the
// workflow tool's own sub-plan on the same *Plan); output is a property of the
// run, not of every chain within it.
func Finalize(ctx context.Context, result responses.Result, ex Extractor) (responses.Result, error) {
	if result != nil {
		return result, nil
	}
	if ex == nil {
		return nil, nil
	}
	return ex.Extract(ctx)
}
