package plan

import (
	"context"
	"errors"
	"fmt"

	"github.com/Servflow/servflow/internal/http"
	"github.com/Servflow/servflow/internal/logging"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"go.uber.org/zap"
)

type contextKey string

const ContextKey contextKey = "planContextKey"

type Plan struct {
	steps map[string]Step
}

func (p *Plan) executeStep(ctx context.Context, step Step, endValue string) (*http.SfResponse, error) {
	logger := logging.GetRequestLogger(ctx).With(zap.String("id", step.ID()))
	var (
		next Step
		err  error
	)

	switch s := step.(type) {
	case *Response:
		return s.WriteResponse(ctx)
	default:
		next, err = s.Execute(ctx)
	}
	if err != nil {
		return nil, fmt.Errorf("error executing step: %w", err)
	}

	if next != nil {
		return p.executeStep(ctx, next, endValue)
	}

	if endValue != "" {
		tmpl, err := requestctx.CreateTextTemplate(ctx, endValue, nil)
		if err != nil {
			return nil, err
		}
		val, err := requestctx.ExecuteTemplateFromContext(ctx, tmpl)
		if err != nil {
			return nil, err
		}

		logger.Debug("template generated", zap.String("template", val))

		return &http.SfResponse{
			Body: []byte(val),
		}, nil
	}

	return nil, nil
}

func (p *Plan) Execute(ctx context.Context, id, endValue string) (*http.SfResponse, error) {
	logging.GetRequestLogger(ctx).Debug("Executing step", zap.String("id", id), zap.String("endValue", endValue))
	ctx = context.WithValue(ctx, ContextKey, p)
	step, ok := p.steps[id]
	if !ok {
		return nil, errors.New("step not found")
	}

	return p.executeStep(ctx, step, endValue)
}

func ExecuteFromContext(ctx context.Context, id, endValue string) (*http.SfResponse, error) {
	plan, ok := ctx.Value(ContextKey).(*Plan)
	if !ok {
		return nil, errors.New("plan not found")
	}

	return plan.Execute(ctx, id, endValue)
}
