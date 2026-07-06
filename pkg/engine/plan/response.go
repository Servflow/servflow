package plan

import (
	"context"
	"fmt"

	apiconfig "github.com/Servflow/servflow/pkg/apiconfig"
	"github.com/Servflow/servflow/pkg/engine/responses"
	"github.com/Servflow/servflow/pkg/logging"
	"github.com/Servflow/servflow/pkg/tracing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.uber.org/zap"
)

type Response struct {
	id              string
	name            string
	responseBuilder responses.ResponseBuilder
}

func (r *Response) ID() string {
	return r.id
}

func (r *Response) DisplayName() string {
	if r.name != "" {
		return r.name
	}
	return r.id
}

// newResponse builds a Response step by resolving the configured response kind
// from the responses registry. An empty kind defaults to "http". The kind's
// factory owns body-format selection (template/json_object) and code validation.
func newResponse(id, name string, resp apiconfig.ResponseConfig) (*Response, error) {
	kind := responses.ResolveKind(resp.Kind)
	factory, ok := responses.Get(kind)
	if !ok {
		return nil, fmt.Errorf("unknown response kind: %s", kind)
	}
	responseBuilder, err := factory(resp)
	if err != nil {
		return nil, err
	}

	return &Response{
		id:              id,
		name:            name,
		responseBuilder: responseBuilder,
	}, nil
}

func (r *Response) execute(ctx context.Context) (*stepWrapper, error) {
	return nil, nil
}

func (r *Response) WriteResponse(ctx context.Context) (responses.Result, error) {
	ctx, span := tracing.StartResponse(ctx, r.id, r.DisplayName())
	defer span.End()

	logger := logging.FromContext(ctx).With(zap.String("response_id", r.id), zap.String("response_name", r.DisplayName()))
	ctx = logging.WithLogger(ctx, logger)

	resp, err := r.responseBuilder.BuildResponse(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	span.SetAttributes(attribute.String("sf.response_kind", resp.Kind()))

	return resp, nil
}
