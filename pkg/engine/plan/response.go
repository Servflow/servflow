package plan

import (
	"context"
	"fmt"

	"github.com/Servflow/servflow/internal/http"
	"github.com/Servflow/servflow/internal/logging"
	"github.com/Servflow/servflow/pkg/definitions"
	responsebuilder2 "github.com/Servflow/servflow/pkg/engine/plan/responsebuilder"
	"go.uber.org/zap"
)

// TODO support response type

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
	builderTypeJSON   = "json"
	builderTypeObject = "object"
)

func newResponse(id string, resp apiconfig.ResponseConfig) (*Response, error) {
	if resp.Code < 100 || resp.Code > 999 {
		return nil, fmt.Errorf("invalid response code: %d", resp.Code)
	}

	var responseBuilder ResponseBuilder
	if resp.BuilderType == "" && (resp.Object.Value != "" || len(resp.Object.Fields) > 0) {
		resp.BuilderType = builderTypeObject
	} else {
		resp.BuilderType = builderTypeJSON
	}
	logging.GetLogger().Debug("creating response", zap.String("builder_type", resp.BuilderType), zap.String("id", id))

	switch resp.BuilderType {
	case builderTypeJSON:
		responseBuilder = responsebuilder2.NewJsonResponseBuilder(resp.Code, resp.Template)
	case builderTypeObject:
		responseBuilder = responsebuilder2.NewObjectBuilder(&resp.Object, resp.Code)
	default:
		return nil, fmt.Errorf("unknown builder type: %s", resp.BuilderType)
	}

	return &Response{
		id:              id,
		responseBuilder: responseBuilder,
	}, nil

}

func (r *Response) Execute(ctx context.Context) (Step, error) {
	return nil, nil
}

func (r *Response) WriteResponse(ctx context.Context) (*http.SfResponse, error) {
	return r.responseBuilder.BuildResponse(ctx)
}
