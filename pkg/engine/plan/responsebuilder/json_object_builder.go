package responsebuilder

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Servflow/servflow/internal/http"
	"github.com/Servflow/servflow/internal/logging"
	"github.com/Servflow/servflow/pkg/definitions"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
)

type JSONObjectBuilder struct {
	object *apiconfig.ResponseObject
	code   int
}

func NewObjectBuilder(object *apiconfig.ResponseObject, code int) *JSONObjectBuilder {
	return &JSONObjectBuilder{
		object: object,
		code:   code,
	}
}

func (o *JSONObjectBuilder) BuildResponse(ctx context.Context) (*http.SfResponse, error) {
	logging.WithContext(ctx).Debug("running object builder response builder")

	val, err := generateValue(ctx, o.object)
	if err != nil {
		return nil, err
	}

	jsonResp, err := json.Marshal(val)
	if err != nil {
		return nil, err
	}

	response := &http.SfResponse{
		Body: jsonResp,
		Code: o.code,
	}
	response.SetHeader("Content-Type", "application/json")

	return response, nil

}

func generateValue(ctx context.Context, object *apiconfig.ResponseObject) (any, error) {
	if len(object.Fields) > 0 {
		fields := make(map[string]any, len(object.Fields))
		for i := range object.Fields {
			f := object.Fields[i]
			val, err := generateValue(ctx, &f)
			if err != nil {
				return nil, err
			}
			if val != nil {
				fields[i] = val
			}
		}
		return fields, nil
	} else {
		return extractValue(ctx, object.Value)
	}
}

func extractValue(ctx context.Context, value string) (any, error) {
	if value == "" {
		return nil, nil
	}

	value = requestctx.WrapWithFunction(value, "jsonraw")
	template, err := requestctx.CreateTextTemplate(ctx, value, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating template: %w", err)
	}

	tmp, err := requestctx.ExecuteTemplateFromContext(ctx, template)
	if err != nil {
		return nil, fmt.Errorf("error executing template: %w", err)
	}

	var val interface{}
	err = json.Unmarshal([]byte(tmp), &val)
	if err != nil {
		logging.WithContext(ctx).Warn("error unmarshalling template " + tmp)
		return tmp, nil
	}

	return val, nil
}
