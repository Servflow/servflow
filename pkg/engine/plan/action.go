package plan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"text/template"

	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/Servflow/servflow/pkg/logging"
	"github.com/Servflow/servflow/pkg/tracing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// TODO deprecate out
// TODO swap id for logger with id

type Action struct {
	next       *stepWrapper
	fail       *stepWrapper
	exec       actions.ActionExecutable
	out        string
	id         string
	name       string
	useReplica bool
	dispatch   []string
}

var (
	// ErrFailure is a non-fatal action, it should be written to the error message item in the request variable,
	// and should not interrupt the workflow
	ErrFailure = errors.New("action failed")
)

func (a *Action) ID() string {
	return a.id
}

func (a *Action) DisplayName() string {
	if a.name != "" {
		return a.name
	}
	return a.id
}

// TODO think of having actions manage their own executables

func (a *Action) execute(ctx context.Context) (*stepWrapper, error) {
	var span trace.Span
	ctx, span = tracing.SpanCtxFromContext(ctx, "action."+a.DisplayName())
	defer span.End()

	logger := logging.FromContext(ctx).With(zap.String("action_id", a.id), zap.String("action_name", a.DisplayName()))
	ctx = logging.WithLogger(ctx, logger)

	var (
		tmpl *template.Template
		err  error
		cfg  string
	)
	configStr := a.exec.Config()
	if configStr != "" {
		tmpl, err = requestctx.CreateTextTemplate(ctx, configStr, nil)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, err
		}
	}

	if tmpl != nil {
		cfg, err = requestctx.ExecuteTemplateFromContext(ctx, tmpl)
		if err != nil {
			logger.Error("error executing template for action "+a.name, zap.Error(err))
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			if a.fail != nil {
				return a.fail, nil
			}
			return nil, err
		}
		logger.Debug("template evaluated successfully")
	}

	span.SetAttributes(attribute.String("config", cfg))

	var (
		resp   interface{}
		fields map[string]string
	)
	logger.Debug("executing action", zap.String("action_id", a.id), zap.Bool("use_replica", a.useReplica), zap.Bool("supports_replica", a.exec.SupportsReplica()))
	if a.useReplica && a.exec.SupportsReplica() {
		logger.Debug("executing replica action")
		resp, fields, err = GetReplicaManager().ExecuteAction(a.exec.Type(), cfg)
		if err != nil {
			logger.Warn("replica manager failed, falling back to direct execution", zap.Error(err))
			resp, fields, err = a.exec.Execute(ctx, cfg)
		}
	} else {
		resp, fields, err = a.exec.Execute(ctx, cfg)
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
			if err := requestctx.AddRequestVariables(ctx, map[string]interface{}{a.out: fmt.Sprintf("error: %v", err)}, ""); err != nil {
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
		fileValue := requestctx.NewFileValue(f, a.out)
		reqCtx, err := requestctx.FromContextOrError(ctx)
		if err != nil {
			return nil, err
		}
		reqCtx.AddActionFile(a.out, fileValue)
		logger.Debug("stored action output as file", zap.String("action_output", a.out))
	} else {
		if err = requestctx.AddRequestVariables(ctx, map[string]interface{}{a.out: resp}, ""); err != nil {
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
func (a *Action) dispatchBackgroundChains(ctx context.Context, logger *zap.Logger) {
	if len(a.dispatch) == 0 {
		return
	}

	bgMgr := GetBackgroundManager()
	if bgMgr == nil {
		logger.Warn("background manager not initialized, skipping dispatch")
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
			// Apply dispatch timeout if configured
			if p != nil && p.dispatchTimeout > 0 {
				var cancel context.CancelFunc
				bgCtx, cancel = context.WithTimeout(bgCtx, p.dispatchTimeout)
				defer cancel()
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
