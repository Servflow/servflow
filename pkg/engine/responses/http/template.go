package http

import (
	"context"
	"fmt"

	sfhttp "github.com/Servflow/servflow/internal/http"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/Servflow/servflow/pkg/logging"
	"go.uber.org/zap"
)

type TemplateBuilder struct {
	Code     int
	template string
}

func NewTemplateBuilder(code int, template string) *TemplateBuilder {
	return &TemplateBuilder{Code: code, template: template}
}

func (J *TemplateBuilder) BuildResponse(ctx context.Context) (*sfhttp.SfResponse, error) {
	logger := logging.FromContext(ctx).With(zap.String("builder_type", "template"))
	ctx = logging.WithLogger(ctx, logger)

	logger.Debug("running template response builder")
	logger.Debug("build response body", zap.String("template", J.template))
	template, err := requestctx.CreateTextTemplate(ctx, J.template, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating template '%s': %w", J.template, err)
	}

	tmp, err := requestctx.ExecuteTemplateFromContext(ctx, template)
	if err != nil {
		return nil, err
	}
	logger.Debug("built response body", zap.String("template", tmp))

	response := &sfhttp.SfResponse{
		Body: []byte(tmp),
		Code: J.Code,
	}
	response.SetHeader("Content-Type", "application/json")
	return response, nil
}
