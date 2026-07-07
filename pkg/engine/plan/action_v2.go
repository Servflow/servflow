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
	"go.opentelemetry.io/otel/codes"
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
	ctx, span = tracing.StartAction(ctx, a.id, a.DisplayName(), a.exec.Type())
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
		span.SetStatus(codes.Error, err.Error())
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
	span.SetAttributes(attribute.String("sf.result", string(b)))
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

	for _, dispatchID := range a.dispatch {
		dispatchID := dispatchID // capture for goroutine

		logger.Debug("dispatching background chain", zap.String("dispatch_id", dispatchID))

		bgMgr.Dispatch(func(bgCtx context.Context) {
			// Detach the request context's cancellation so the chain survives the
			// HTTP request completing, while retaining its values — crucially the
			// live, recording OTEL span. Re-injecting only a remote span context
			// (the previous approach) left dispatched spans parented to a
			// non-recording reference that never exported, orphaning them as
			// separate trace roots. WithoutCancel keeps the real parent span so
			// dispatched spans nest under the original trace.
			chainCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
			defer cancel()

			// Still cancel the chain when the server shuts down (bgCtx is the
			// background manager's server-level context).
			defer context.AfterFunc(bgCtx, cancel)()

			// Bound the chain by the dispatch timeout.
			if p != nil && p.dispatchTimeout > 0 {
				var timeoutCancel context.CancelFunc
				chainCtx, timeoutCancel = context.WithTimeout(chainCtx, p.dispatchTimeout)
				defer timeoutCancel()
			}

			chainCtx = requestctx.WithAggregationContext(chainCtx, reqCtx)
			chainCtx = logging.WithLogger(chainCtx, logger.With(zap.String("dispatch_id", dispatchID)))
			chainCtx = context.WithValue(chainCtx, ContextKey, p)

			_, err := ExecuteFromContext(chainCtx, dispatchID)
			if err != nil {
				logging.FromContext(chainCtx).Error("dispatch chain failed", zap.Error(err))
			}
		})
	}
}
