package plan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/Servflow/servflow/pkg/logging"
	"github.com/Servflow/servflow/pkg/tracing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// ActionV2 is a plan step that executes a V2 action.
// Unlike v1, template resolution is handled by the action itself,
// and the action's id is used directly as the output variable name.
type ActionV2 struct {
	next       *stepWrapper
	fail       *stepWrapper
	exec       actions.ActionExecutableV2
	id         string // Also used as output variable name (no separate 'out')
	name       string
	useReplica bool
	dispatch   []string
}

func (a *ActionV2) ID() string {
	return a.id
}

func (a *ActionV2) DisplayName() string {
	if a.name != "" {
		return a.name
	}
	return a.id
}

func (a *ActionV2) execute(ctx context.Context) (*stepWrapper, error) {
	var span trace.Span
	ctx, span = tracing.SpanCtxFromContext(ctx, "action."+a.DisplayName())
	defer span.End()

	logger := logging.FromContext(ctx).With(zap.String("action_id", a.id), zap.String("action_name", a.DisplayName()))
	ctx = logging.WithLogger(ctx, logger)

	logger.Debug("executing v2 action", zap.String("action_id", a.id), zap.Bool("use_replica", a.useReplica), zap.Bool("supports_replica", a.exec.SupportsReplica()))

	var (
		resp   interface{}
		fields map[string]string
		err    error
	)

	// V2 actions handle their own template resolution
	if a.useReplica && a.exec.SupportsReplica() {
		logger.Debug("executing replica action")
		// Note: Replica execution for V2 would need a different mechanism
		// since the action handles its own template resolution.
		// For now, fall back to direct execution.
		logger.Warn("replica execution not yet supported for V2 actions, falling back to direct execution")
		resp, fields, err = a.exec.Execute(ctx)
	} else {
		resp, fields, err = a.exec.Execute(ctx)
	}

	for k, v := range fields {
		span.SetAttributes(attribute.String(k, v))
	}

	if err != nil {
		span.RecordError(err)
		if errors.Is(err, ErrFailure) {
			if err := requestctx.AddRequestVariables(ctx, map[string]interface{}{requestctx.ErrorTagStripped: err.Error()}, ""); err != nil {
				return nil, err
			}
			if err := requestctx.AddRequestVariables(ctx, map[string]interface{}{a.id: fmt.Sprintf("error: %v", err)}, ""); err != nil {
				return nil, err
			}
			return a.fail, nil
		}
		logger.Error("error executing action", zap.Error(err))
		return nil, fmt.Errorf("error executing action: %w", err)
	}

	b, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		logger.Error("error marshalling action response", zap.Error(err))
	}
	span.SetAttributes(attribute.String("result", string(b)))
	logger.Debug("action executed successfully. Response: " + string(b))

	// Check if response is an io.Reader and store as action file
	if f, ok := resp.(io.ReadCloser); ok {
		fileValue := requestctx.NewFileValue(f, a.id)
		reqCtx, err := requestctx.FromContextOrError(ctx)
		if err != nil {
			return nil, err
		}
		reqCtx.AddActionFile(a.id, fileValue)
		logger.Debug("stored action output as file", zap.String("action_output", a.id))
	} else {
		if err = requestctx.AddRequestVariables(ctx, map[string]interface{}{a.id: resp}, ""); err != nil {
			return nil, err
		}
	}

	// Fire off dispatch chains in background
	a.dispatchBackgroundChains(ctx, logger)

	return a.next, nil
}

// dispatchBackgroundChains spawns background goroutines for each dispatch target.
// These chains run independently using the server's background context, so they
// won't be cancelled when the HTTP request completes.
func (a *ActionV2) dispatchBackgroundChains(ctx context.Context, logger *zap.Logger) {
	if len(a.dispatch) == 0 {
		return
	}

	bgMgr := BackgroundManagerFromContext(ctx)
	if bgMgr == nil {
		logger.Warn("background manager not in context, skipping dispatch")
		return
	}

	reqCtx, err := requestctx.FromContextOrError(ctx)
	if err != nil {
		logger.Error("failed to get request context for dispatch", zap.Error(err))
		return
	}

	p, _ := ctx.Value(ContextKey).(*Plan)

	// Capture the span context from the original request to propagate trace ID
	// to background goroutines without establishing a parent-child relationship
	spanCtx := trace.SpanContextFromContext(ctx)

	for _, dispatchID := range a.dispatch {
		dispatchID := dispatchID // capture for goroutine

		logger.Debug("dispatching background chain", zap.String("dispatch_id", dispatchID))

		bgMgr.Dispatch(func(bgCtx context.Context) {
			// Apply dispatch timeout if configured
			if p != nil && p.dispatchTimeout > 0 {
				var cancel context.CancelFunc
				bgCtx, cancel = context.WithTimeout(bgCtx, p.dispatchTimeout)
				defer cancel()
			}

			// Embed the original span context so new spans continue the same trace
			if spanCtx.IsValid() {
				bgCtx = trace.ContextWithRemoteSpanContext(bgCtx, spanCtx)
			}

			bgCtx = requestctx.WithAggregationContext(bgCtx, reqCtx)
			bgCtx = logging.WithLogger(bgCtx, logger.With(zap.String("dispatch_id", dispatchID)))
			bgCtx = context.WithValue(bgCtx, ContextKey, p)

			_, err := ExecuteFromContext(bgCtx, dispatchID, nil)
			if err != nil {
				logging.FromContext(bgCtx).Error("dispatch chain failed", zap.Error(err))
			}
		})
	}
}
