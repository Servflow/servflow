package responsebuilder

import (
	"context"
	"fmt"

	"github.com/Servflow/servflow/internal/http"
	dpl2 "github.com/Servflow/servflow/pkg/engine/requestctx"
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

func (J *TemplateBuilder) BuildResponse(ctx context.Context) (*http.SfResponse, error) {
	logging.WithContext(ctx).Debug("running json builder response builder")

	logging.WithContext(ctx).Debug("build response body", zap.String("template", J.template))
	template, err := dpl2.CreateTextTemplate(ctx, J.template, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating template '%s': %w", J.template, err)
	}

	tmp, err := dpl2.ExecuteTemplateFromContext(ctx, template)
	if err != nil {
		return nil, err
	}
	logging.WithContext(ctx).Debug("built response body", zap.String("template", tmp))

	response := &http.SfResponse{
		Body: []byte(tmp),
		Code: J.Code,
	}
	response.SetHeader("Content-Type", "application/json")
	return response, nil
}
