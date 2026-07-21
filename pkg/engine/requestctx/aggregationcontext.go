package requestctx

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"text/template"

	"go.opentelemetry.io/otel/attribute"
)

var ErrNoContext = errors.New("no context provided in request")

type contextKey string

var aggContextKey = contextKey("aggregationContext")

type RequestContext struct {
	requestID string
	sync.Mutex
	requestVariables map[string]interface{}
	requestFuncs     template.FuncMap
	validationErrors []error
	availableFiles   map[string]*FileValue
	workspace        Workspace

	// tokenInput/tokenOutput accumulate LLM token usage across every model call
	// in this request. Observability-only — not exposed to workflow templates.
	// Atomic so parallel model calls can add without the RequestContext mutex.
	tokenInput  atomic.Int64
	tokenOutput atomic.Int64

	// secrets is the placeholder→value table for this request's call tree.
	// Shared by pointer with child workflow contexts via ShareSecretsWith.
	// Own mutex; never nil (see NewRequestContext).
	secrets *secretTable

	// spanAttrs are the request-wide attributes stamped on this request's root
	// span by pkg/tracing. Set once in Start before the ctx is shared;
	// read-only afterwards.
	spanAttrs []attribute.KeyValue
	// lc is the request completion latch (see lifecycle.go).
	lc lifecycle
}

// AddTokenUsage adds LLM token usage to this request's running total. Safe for
// concurrent callers (e.g. parallel model calls).
func (rc *RequestContext) AddTokenUsage(input, output int64) {
	rc.tokenInput.Add(input)
	rc.tokenOutput.Add(output)
}

// TokenUsage returns the accumulated request-level token totals.
func (rc *RequestContext) TokenUsage() (input, output int64) {
	return rc.tokenInput.Load(), rc.tokenOutput.Load()
}

// TODO move this and dpl together
const errTag = "error"

// AddValidationErrors gets the validation errors added by the various conditional template functions,
// then adds the errors under the errTag in the request variable for parsing
func AddValidationErrors(ctx context.Context) error {
	reqCtx, err := FromContextOrError(ctx)
	if err != nil {
		return err
	}

	errMessages := make([]string, len(reqCtx.validationErrors))
	for i, err := range reqCtx.validationErrors {
		errMessages[i] = err.Error()
	}
	reqCtx.addRequestVariables(map[string]interface{}{errTag: errMessages}, "")
	return nil
}

func (rc *RequestContext) AddRequestTemplateFunctions(templateFuncs template.FuncMap, overwrite bool) {
	rc.Lock()
	defer rc.Unlock()
	for k, v := range templateFuncs {
		if _, exists := rc.requestFuncs[k]; exists && !overwrite {
			continue
		}
		rc.requestFuncs[k] = v
	}
}

func (rc *RequestContext) TemplateFunctions() template.FuncMap {
	return rc.requestFuncs
}

func (rc *RequestContext) Variables() map[string]interface{} {
	return rc.requestVariables
}

// SetWorkspace sets the file capability for this request. It is supplied by the
// host (resolved from the agent's configured workspace) before the plan runs.
func (rc *RequestContext) SetWorkspace(ws Workspace) {
	rc.Lock()
	defer rc.Unlock()
	rc.workspace = ws
}

// GetWorkspace returns the workspace capability for this request, or nil if the
// request has none. Callers that require a workspace should use
// WorkspaceFromContext, which converts the nil case into ErrNoWorkspace.
func (rc *RequestContext) GetWorkspace() Workspace {
	rc.Lock()
	defer rc.Unlock()
	return rc.workspace
}

// NewRequestContext builds a bare RequestContext.
//
// Production entry points should NOT call this directly: open a request via
// Start, which consolidates id generation, span attributes, the request
// logger, template funcs and parent linking. NewRequestContext remains the
// low-level constructor used by Start, tests and NewTestContext.
func NewRequestContext(id string) *RequestContext {
	return &RequestContext{
		requestID:        id,
		requestVariables: make(map[string]interface{}),
		requestFuncs:     make(template.FuncMap),
		validationErrors: make([]error, 0),
		availableFiles:   make(map[string]*FileValue),
		secrets:          newSecretTable(),
	}
}

func (rc *RequestContext) ID() string {
	return rc.requestID
}

// WithAggregationContext adds the aggregate context to the request context
func WithAggregationContext(ctx context.Context, aggCtx *RequestContext) context.Context {
	return context.WithValue(ctx, aggContextKey, aggCtx)
}

func FromContext(ctx context.Context) (*RequestContext, bool) {
	aggCtx, ok := ctx.Value(aggContextKey).(*RequestContext)
	return aggCtx, ok
}

func FromContextOrError(ctx context.Context) (*RequestContext, error) {
	aggCtx, ok := ctx.Value(aggContextKey).(*RequestContext)
	if !ok {
		return nil, ErrNoContext
	}
	return aggCtx, nil
}

func NewTestContext() context.Context {
	aggContext := NewRequestContext("test")
	return context.WithValue(context.Background(), aggContextKey, aggContext)
}

func GetRequestVariable(ctx context.Context, key string) (interface{}, error) {
	agg, err := FromContextOrError(ctx)
	if err != nil {
		return nil, err
	}
	return agg.requestVariables[key], nil
}

// AddRequestVariables add all the variables to the request context, it adds the prefix
// to the variable keys as well
func AddRequestVariables(ctx context.Context, variables map[string]interface{}, prefix string) error {
	agg, err := FromContextOrError(ctx)
	if err != nil {
		return err
	}
	agg.addRequestVariables(variables, prefix)
	return nil
}

// TODO remove prefix
func (rc *RequestContext) addRequestVariables(variables map[string]interface{}, prefix string) {
	rc.Lock()
	defer rc.Unlock()
	for k, v := range variables {
		key := fmt.Sprintf("%s%s", prefix, k)
		rc.requestVariables[key] = v
	}
}

func GetAllRequestVariables(ctx context.Context) (map[string]interface{}, error) {
	agg, err := FromContextOrError(ctx)
	if err != nil {
		return nil, err
	}
	return agg.requestVariables, nil
}
