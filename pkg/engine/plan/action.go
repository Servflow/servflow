package plan

import (
	"context"
	"fmt"
	"text/template"

	"github.com/Servflow/servflow/internal/logging"
	"github.com/Servflow/servflow/internal/tracing"
	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"go.opentelemetry.io/otel/codes"
	"go.uber.org/zap"

	"go.opentelemetry.io/otel/trace"
)

// TODO deprecate out
// TODO swap id for logger with id
// TODO pass logger in here

type Action struct {
	configStr string
	next      Step
	fail      Step
	exec      actions.ActionExecutable
	out       string
	id        string
	isGroup   bool
}

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
		span.RecordError(err)
		if a.fail != nil {
			if err := requestctx.AddRequestVariables(ctx, map[string]interface{}{requestctx.ErrorTagStripped: err.Error()}, ""); err != nil {
				return nil, err
			}
			logger.Debug("error executing action", zap.String("config", a.configStr), zap.Error(err))
			return a.fail, nil
		}
		return nil, fmt.Errorf("error executing action: %s type: %s: %w", a.id, a.exec.Type(), err)
	}
	logger.Debug("action executed successfully", zap.Any("resp", resp))

	if !a.isGroup {
		if err = requestctx.AddRequestVariables(ctx, map[string]interface{}{a.out: resp}, ""); err != nil {
			return nil, err
		}
	}

	return a.next, nil
}
