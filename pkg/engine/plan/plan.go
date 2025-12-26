package plan

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Servflow/servflow/internal/http"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/Servflow/servflow/pkg/logging"
	"go.uber.org/zap"
)

type contextKey string

const ContextKey contextKey = "planContextKey"

type Plan struct {
	steps map[string]stepWrapper
}

var (
	ErrContextCanceled = errors.New("context canceled")
)

type stepWrapper struct {
	id   string
	step Step
}

func (p *Plan) executeStep(ctx context.Context, step *stepWrapper, endValue string) (*http.SfResponse, error) {
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
		if errors.Is(err, errExecutingExecutable) {
			return p.generateEndValue(ctx, logger, endValue)
		}
		return nil, fmt.Errorf("error executing step: %w", err)
	}

	if next != nil {
		return p.executeStep(ctx, next, endValue)
	}
	return p.generateEndValue(ctx, logger, endValue)
}

func (p *Plan) generateEndValue(ctx context.Context, logger *zap.Logger, endValue string) (*http.SfResponse, error) {
	if endValue == "" {
		return nil, nil
	}
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

func (p *Plan) Execute(ctx context.Context, id, endValue string) (*http.SfResponse, error) {
	id = strings.TrimLeft(id, "$")
	ctx = context.WithValue(ctx, ContextKey, p)
	step, ok := p.steps[id]
	if !ok {
		return nil, errors.New("step not found")
	}

	return p.executeStep(ctx, &step, endValue)
}

func ExecuteFromContext(ctx context.Context, id, endValue string) (*http.SfResponse, error) {
	plan, ok := ctx.Value(ContextKey).(*Plan)
	if !ok {
		return nil, errors.New("plan not found")
	}

	return plan.Execute(ctx, id, endValue)
}
