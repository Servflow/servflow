package plan

import (
	"context"
	"errors"
	"fmt"
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
	next      Step
	fail      Step
	exec      actions.ActionExecutable
	out       string
	id        string
}

var (
	errExecutingAction = errors.New("executing action")
)

func (a *Action) ID() string {
	return a.id
}

// TODO think of having actions manage their own executables

func (a *Action) Execute(ctx context.Context) (Step, error) {
	var span trace.Span
	ctx, span = tracing.SpanCtxFromContext(ctx, "actions.StartAction."+a.id)
	defer span.End()

	logger := logging.GetRequestLogger(ctx).With(zap.String("id", a.id))

	var (
		tmpl *template.Template
		err  error
		cfg  string
	)
	if a.configStr != "" {
		logger.Debug("generated template", zap.String("config", a.configStr))
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
			logger.Error("error executing template for action", zap.String("config", a.configStr), zap.Error(err))
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			if a.fail != nil {
				return a.fail, nil
			}
			return nil, err
		}
		logger.Debug("generated template", zap.String("config", cfg))
	}

	resp, err := a.exec.Execute(ctx, cfg)
	if err != nil {
		logger.Debug("error executing action", zap.String("config", a.configStr), zap.Error(err))
		span.RecordError(err)
		if err2 := requestctx.AddRequestVariables(ctx, map[string]interface{}{requestctx.ErrorTagStripped: err.Error()}, ""); err2 != nil {
			return nil, err2
		}
		if err2 := requestctx.AddRequestVariables(ctx, map[string]interface{}{a.out: fmt.Sprintf("error: %v", err)}, ""); err2 != nil {
			return nil, err2
		}
		if a.fail != nil {
			return a.fail, nil
		}
		return nil, fmt.Errorf("%w: %w", errExecutingAction, err)
	}

	logger.Debug("action executed successfully", zap.Any("resp", resp))
	if err = requestctx.AddRequestVariables(ctx, map[string]interface{}{a.out: resp}, ""); err != nil {
		return nil, err
	}
	return a.next, nil
}
