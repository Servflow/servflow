package plan

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Servflow/servflow/internal/tracing"
	requestctx2 "github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/Servflow/servflow/pkg/logging"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// TODO add logger  and report errors

type ConditionStep struct {
	id         string
	OnValid    Step
	OnInvalid  Step
	exprString string
}

func (c *ConditionStep) ID() string {
	return c.id
}

// Execute will execute the conditions and generate error messages for conditions that use
// request variables
func (c *ConditionStep) Execute(ctx context.Context) (Step, error) {
	// set up tracer
	var span trace.Span
	ctx, span = tracing.SpanCtxFromContext(ctx, "condition.execute."+c.id)
	defer span.End()

	logger := logging.GetRequestLogger(ctx)
	if c.exprString == "" {
		return c.OnValid, nil
	}

	reqCtx, ok := requestctx2.FromContext(ctx)
	if !ok {
		return nil, errors.New("invalid request context")
	}

	tmpl, err := requestctx2.CreateTextTemplate(ctx, c.exprString, reqCtx.ConditionalTemplateFunctions())
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("error creating template for condition %w template: %s", err, c.exprString)
	}

	resp, err := requestctx2.ExecuteTemplateFromContext(ctx, tmpl)
	if err != nil {
		logger.Debug("error executing template", zap.String("template", c.exprString), zap.Error(err))
		span.RecordError(err)
		return nil, err
	}

	err = requestctx2.AddValidationErrors(ctx)
	if err != nil {
		logger.Debug("error adding validation error", zap.String("template", c.exprString), zap.Error(err))
		return nil, err
	}

	logger.Debug("executed condition", zap.String("condition", c.exprString), zap.Any("response", resp))
	if strings.TrimSpace(resp) == "true" {
		span.SetAttributes(attribute.Bool("condition.isValid", true))
		return c.OnValid, nil
	}
	span.SetAttributes(attribute.Bool("condition.isValid", false))
	return c.OnInvalid, nil
}
