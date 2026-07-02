// Package http implements the built-in "http" response type: a status code plus
// a body rendered either as a Go template or as a structured JSON object. It
// registers itself with the responses registry at init.
package http

import (
	"fmt"

	"github.com/Servflow/servflow/pkg/apiconfig"
	"github.com/Servflow/servflow/pkg/engine/responses"
)

const (
	bodyTemplate = "template"
	bodyObject   = "json_object"
)

func init() {
	responses.RegisterResponseType("http", newBuilder)
}

// newBuilder selects the body builder for an http response from its config,
// preserving the historical behaviour: an empty type defaults to json_object
// when an object is present, otherwise template.
func newBuilder(cfg apiconfig.ResponseConfig) (responses.ResponseBuilder, error) {
	if cfg.Code < 100 || cfg.Code > 999 {
		return nil, fmt.Errorf("invalid response code: %d", cfg.Code)
	}

	bodyType := cfg.Type
	if bodyType == "" {
		if cfg.Object.Value != "" || len(cfg.Object.Fields) > 0 {
			bodyType = bodyObject
		} else {
			bodyType = bodyTemplate
		}
	}

	switch bodyType {
	case bodyTemplate:
		return NewTemplateBuilder(cfg.Code, cfg.Template), nil
	case bodyObject:
		return NewObjectBuilder(&cfg.Object, cfg.Code), nil
	default:
		return nil, fmt.Errorf("unknown response body type: %s", bodyType)
	}
}
