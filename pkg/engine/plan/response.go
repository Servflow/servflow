package plan

import (
	"context"
	"fmt"

	"github.com/Servflow/servflow/internal/http"
	"github.com/Servflow/servflow/internal/logging"
	"github.com/Servflow/servflow/pkg/definitions"
	"github.com/Servflow/servflow/pkg/engine/plan/responsebuilder"
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
	if resp.Type == "" && (resp.Object.Value != "" || len(resp.Object.Fields) > 0) {
		resp.Type = builderTypeObject
	} else {
		resp.Type = builderTypeTemplate
	}
	logging.GetLogger().Debug("creating response", zap.String("builder_type", resp.Type), zap.String("id", id))

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

func (r *Response) Execute(ctx context.Context) (Step, error) {
	return nil, nil
}

func (r *Response) WriteResponse(ctx context.Context) (*http.SfResponse, error) {
	return r.responseBuilder.BuildResponse(ctx)
}
