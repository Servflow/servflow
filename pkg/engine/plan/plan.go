package plan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	sfhttp "github.com/Servflow/servflow/internal/http"
	"github.com/Servflow/servflow/pkg/apiconfig"
	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/Servflow/servflow/pkg/logging"
	"go.uber.org/zap"
)

type contextKey string

const (
	ContextKey        contextKey = "planContextKey"
	RequestContextKey contextKey = "planRequestContextKey"
)

type Plan struct {
	steps           map[string]stepWrapper
	dispatchTimeout time.Duration
}

var (
	ErrContextCanceled = errors.New("context canceled")
)

type stepWrapper struct {
	id   string
	step Step
}

type EndValueType int

const (
	StringEndValue EndValueType = iota
	FileEndValue
)

type EndValueSpec struct {
	ValType   EndValueType
	StringVal string
	FileVal   apiconfig.FileInput
}

func WithRequest(ctx context.Context, r *http.Request) context.Context {
	return context.WithValue(ctx, RequestContextKey, r)
}

func RequestFromContext(ctx context.Context) (*http.Request, error) {
	r, ok := ctx.Value(RequestContextKey).(*http.Request)
	if !ok {
		return nil, errors.New("request context is missing")
	}
	return r, nil
}

func ExecuteSingleAction(actionType string, config json.RawMessage) (any, map[string]string, error) {
	exec, err := actions.GetActionExecutable(actionType, config)
	if err != nil {
		return nil, nil, err
	}

	return exec.Execute(context.Background(), string(config))
}

func (p *Plan) executeStep(ctx context.Context, step *stepWrapper, endValue *EndValueSpec) (*sfhttp.SfResponse, error) {
	select {
	case <-ctx.Done():
		return nil, ErrContextCanceled
	default:
	}
	logger := logging.FromContext(ctx).With(zap.String("step_id", step.id))
	var (
		next *stepWrapper
		err  error
	)

	switch s := step.step.(type) {
	case *Response:
		return s.WriteResponse(ctx)
	default:
		logger.Debug("starting execution")
		next, err = s.execute(logging.WithLogger(ctx, logger))
		logger.Debug("finished execution")
	}
	if err != nil {
		//if errors.Is(err, errExecutingExecutable) {
		//	return p.generateEndValue(ctx, logger, endValue)
		//}
		return nil, fmt.Errorf("error executing step: %w", err)
	}

	if next != nil {
		return p.executeStep(ctx, next, endValue)
	}
	return p.generateEndValue(ctx, logger, endValue)
}

func (p *Plan) generateEndValue(ctx context.Context, logger *zap.Logger, endValue *EndValueSpec) (*sfhttp.SfResponse, error) {
	if endValue == nil {
		return nil, nil
	}

	resp := &sfhttp.SfResponse{}
	switch endValue.ValType {
	case StringEndValue:
		tmpl, err := requestctx.CreateTextTemplate(ctx, endValue.StringVal, nil)
		if err != nil {
			return nil, err
		}
		val, err := requestctx.ExecuteTemplateFromContext(ctx, tmpl)
		if err != nil {
			return nil, err
		}
		logger.Debug("template generated", zap.String("template", val))
		resp.Body = []byte(val)
	case FileEndValue:
		fileVal, err := requestctx.GetFileFromContext(ctx, endValue.FileVal)
		if err != nil {
			return nil, err
		}
		resp.File = fileVal
	}
	return resp, nil
}

func (p *Plan) Execute(ctx context.Context, id string, endValue *EndValueSpec) (*sfhttp.SfResponse, error) {
	id = strings.TrimLeft(id, "$")
	ctx = context.WithValue(ctx, ContextKey, p)
	step, ok := p.steps[id]
	if !ok {
		return nil, errors.New("step not found")
	}

	return p.executeStep(ctx, &step, endValue)
}

func ExecuteFromContext(ctx context.Context, id string, endValue *EndValueSpec) (*sfhttp.SfResponse, error) {
	plan, ok := ctx.Value(ContextKey).(*Plan)
	if !ok {
		return nil, errors.New("plan not found")
	}

	return plan.Execute(ctx, id, endValue)
}

// BackgroundManager manages background goroutines for dispatched action chains.
// It uses a server-level context that only cancels on server shutdown,
// not when individual HTTP requests complete.
type BackgroundManager struct {
	baseCtx context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

const BackgroundManagerContextKey contextKey = "backgroundManagerContextKey"

// NewBackgroundManager creates a new background manager with the given context.
// Each engine instance should create its own BackgroundManager.
func NewBackgroundManager(ctx context.Context) *BackgroundManager {
	bgCtx, cancel := context.WithCancel(ctx)
	return &BackgroundManager{
		baseCtx: bgCtx,
		cancel:  cancel,
	}
}

// WithBackgroundManager attaches a BackgroundManager to the context.
func WithBackgroundManager(ctx context.Context, bm *BackgroundManager) context.Context {
	return context.WithValue(ctx, BackgroundManagerContextKey, bm)
}

// BackgroundManagerFromContext retrieves the BackgroundManager from the context.
// Returns nil if no BackgroundManager is attached.
func BackgroundManagerFromContext(ctx context.Context) *BackgroundManager {
	bm, _ := ctx.Value(BackgroundManagerContextKey).(*BackgroundManager)
	return bm
}

// Dispatch spawns a background goroutine that executes the given function.
// The function receives the background context which only cancels on server shutdown.
func (bm *BackgroundManager) Dispatch(fn func(ctx context.Context)) {
	bm.wg.Add(1)
	go func() {
		defer bm.wg.Done()
		fn(bm.baseCtx)
	}()
}

// Shutdown cancels the background context and waits for all dispatched goroutines to complete.
func (bm *BackgroundManager) Shutdown() {
	bm.cancel()
	bm.wg.Wait()
}
