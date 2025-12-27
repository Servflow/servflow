package plan

import (
	"context"
	"errors"
	"fmt"
	"io"
	"text/template"

	"github.com/Servflow/servflow/internal/tracing"
	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/Servflow/servflow/pkg/logging"
	"go.opentelemetry.io/otel/codes"
	"go.uber.org/zap"

	"go.opentelemetry.io/otel/trace"
)

// TODO deprecate out
// TODO swap id for logger with id

type Action struct {
	configStr string
	next      *stepWrapper
	fail      *stepWrapper
	exec      actions.ActionExecutable
	out       string
	id        string
}

var (
	// ErrFailure is a non-fatal action, it should be written to the error message item in the request variable,
	// and should not interrupt the workflow
	ErrFailure = errors.New("action failed")
)

func (a *Action) ID() string {
	return a.id
}

// TODO think of having actions manage their own executables

func (a *Action) execute(ctx context.Context) (*stepWrapper, error) {
	var span trace.Span
	ctx, span = tracing.SpanCtxFromContext(ctx, "actions.StartAction."+a.id)
	defer span.End()

	logger := logging.FromContext(ctx).With(zap.String("action_id", a.id))
	ctx = logging.WithLogger(ctx, logger)

	var (
		tmpl *template.Template
		err  error
		cfg  string
	)
	if a.configStr != "" {
		tmpl, err = requestctx.CreateTextTemplate(ctx, a.configStr, nil)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, err
		}
	}

	if tmpl != nil {
		cfg, err = requestctx.ExecuteTemplateFromContext(ctx, tmpl)
		if err != nil {
			logger.Error("error executing template for action", zap.Error(err))
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			if a.fail != nil {
				return a.fail, nil
			}
			return nil, err
		}
		logger.Debug("template evaluated successfully")
	}

	resp, err := a.exec.Execute(ctx, cfg)
	if err != nil {
		logger.Error("error executing action", zap.Error(err))
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

		return nil, fmt.Errorf("error executing action: %w", err)
	}

	logger.Debug("action executed successfully", zap.Any("resp", resp))

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

	return a.next, nil
}
