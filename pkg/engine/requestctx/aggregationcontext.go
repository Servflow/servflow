package requestctx

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"text/template"
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
}

// TODO move this and dpl together
const errTag = "error"

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

func (rc *RequestContext) AddRequestTemplateFunctions(templateFuncs template.FuncMap) {
	for k, v := range templateFuncs {
		rc.requestFuncs[k] = v
	}
}

func (rc *RequestContext) TemplateFunctions() template.FuncMap {
	return rc.requestFuncs
}

func NewRequestContext(id string) *RequestContext {
	return &RequestContext{
		requestID:        id,
		requestVariables: make(map[string]interface{}),
		requestFuncs:     make(template.FuncMap),
		validationErrors: make([]error, 0),
		availableFiles:   make(map[string]*FileValue),
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
