package plan

import (
	"context"
	"fmt"

	"github.com/Servflow/servflow/internal/http"
	apiconfig "github.com/Servflow/servflow/pkg/apiconfig"
	"github.com/Servflow/servflow/pkg/engine/plan/responsebuilder"
	"github.com/Servflow/servflow/pkg/logging"
	"github.com/Servflow/servflow/pkg/tracing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.uber.org/zap"
)

// TODO improve response handling

type Response struct {
	id              string
	responseBuilder ResponseBuilder
}

func (r *Response) ID() string {
	return r.id
}

type ResponseBuilder interface {
	BuildResponse(ctx context.Context) (*http.SfResponse, error)
}

const (
	builderTypeTemplate = "template"
	builderTypeObject   = "json_object"
)

func newResponse(id string, resp apiconfig.ResponseConfig) (*Response, error) {
	if resp.Code < 100 || resp.Code > 999 {
		return nil, fmt.Errorf("invalid response code: %d", resp.Code)
	}

	var responseBuilder ResponseBuilder
	if resp.Type == "" {
		if resp.Object.Value != "" || len(resp.Object.Fields) > 0 {
			resp.Type = builderTypeObject
		} else {
			resp.Type = builderTypeTemplate
		}
	}

	switch resp.Type {
	case builderTypeTemplate:
		responseBuilder = responsebuilder.NewTemplateBuilder(resp.Code, resp.Template)
	case builderTypeObject:
		responseBuilder = responsebuilder.NewObjectBuilder(&resp.Object, resp.Code)
	default:
		return nil, fmt.Errorf("unknown builder type: %s", resp.Type)
	}

	return &Response{
		id:              id,
		responseBuilder: responseBuilder,
	}, nil

}

func (r *Response) execute(ctx context.Context) (*stepWrapper, error) {
	return nil, nil
}

func (r *Response) WriteResponse(ctx context.Context) (*http.SfResponse, error) {
	ctx, span := tracing.SpanCtxFromContext(ctx, "response."+r.id)
	defer span.End()

	logger := logging.FromContext(ctx).With(zap.String("response_id", r.id))
	ctx = logging.WithLogger(ctx, logger)

	resp, err := r.responseBuilder.BuildResponse(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	span.SetAttributes(attribute.String("result", string(resp.Body)))

	return resp, nil
}
