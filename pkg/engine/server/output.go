package server

import (
	"errors"
	"fmt"
	"net/http"

	sfhttp "github.com/Servflow/servflow/internal/http"
	apiconfig "github.com/Servflow/servflow/pkg/apiconfig"
	"github.com/Servflow/servflow/pkg/engine/outputs"
	"github.com/Servflow/servflow/pkg/engine/responses"
)

// resolveOutput builds the output handler for a config. An explicit output
// handler wins; otherwise the deprecated mcpTool.result expression is mapped
// onto the equivalent template handler, so configs written before output
// handlers existed keep producing exactly the same result.
func resolveOutput(config *apiconfig.APIConfig) (outputs.Extractor, error) {
	if config.Output.Handler != "" {
		return outputs.Resolve(config.Output)
	}
	if config.McpTool.Result != "" {
		return outputs.Template(config.McpTool.Result), nil
	}
	return nil, nil
}

// httpResponseFor renders a run's output as an HTTP response. A response step
// already produced one; an output handler produces a value, which is written as
// a 200 with the text as the body — that is what lets an HTTP workflow return
// its agent's answer without configuring a response step. Anything else (no
// output at all, or a response kind that is not HTTP mounted on an HTTP
// endpoint) is a server error naming the actual cause.
func httpResponseFor(result responses.Result) (*sfhttp.SfResponse, error) {
	switch r := result.(type) {
	case nil:
		return nil, errors.New("error executing api, response missing")
	case *sfhttp.SfResponse:
		if r == nil {
			return nil, errors.New("error executing api, response missing")
		}
		return r, nil
	case outputs.TextResult:
		return &sfhttp.SfResponse{
			Code:    http.StatusOK,
			Body:    []byte(r.Text),
			Headers: http.Header{"Content-Type": []string{"text/plain; charset=utf-8"}},
		}, nil
	default:
		return nil, fmt.Errorf("unexpected result type %T for HTTP endpoint", result)
	}
}
