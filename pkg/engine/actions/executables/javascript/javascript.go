package javascript

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/Servflow/servflow/pkg/engine/plan"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/Servflow/servflow/pkg/logging"
	"github.com/dop251/goja"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

type Executable struct {
	compiledDeps *goja.Program
	script       string
}

type Config struct {
	Script       string `json:"script"`
	Dependencies string `json:"dependencies"`
}

func NewExecutable(cfg Config) (*Executable, error) {
	if cfg.Script == "" {
		return nil, fmt.Errorf("script is required")
	}

	var (
		err          error
		compiledDeps *goja.Program
	)
	if cfg.Dependencies != "" {
		compiledDeps, err = goja.Compile("dependencies", cfg.Dependencies, false)
		if err != nil {
			return nil, fmt.Errorf("failed to compile dependencies: %w", err)
		}
	}

	return &Executable{
		script:       cfg.Script,
		compiledDeps: compiledDeps,
	}, nil
}

func (e *Executable) Type() string {
	return "javascript"
}

func (e *Executable) SupportsReplica() bool {
	return true
}

// TODO save config in action not executable

func (e *Executable) Config() string {
	return e.script
}

func (e *Executable) Execute(ctx context.Context, modifiedConfig string) (interface{}, map[string]string, error) {
	logger := logging.FromContext(ctx).With(zap.String("execution_type", e.Type()))
	ctx = logging.WithLogger(ctx, logger)

	vm := goja.New()

	if e.compiledDeps != nil {
		_, err := vm.RunProgram(e.compiledDeps)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to execute dependencies: %w", err)
		}
	}

	compiledScript, err := goja.Compile("script", modifiedConfig, false)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to compile script: %w", err)
	}

	_, err = vm.RunProgram(compiledScript)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to execute script: %w", err)
	}

	servflowRun := vm.Get("servflowRun")
	if servflowRun == nil || goja.IsUndefined(servflowRun) {
		return nil, nil, fmt.Errorf("servflowRun function not defined in script")
	}

	fn, ok := goja.AssertFunction(servflowRun)
	if !ok {
		return nil, nil, fmt.Errorf("servflowRun is not a function")
	}

	variables, err := requestctx.GetAllRequestVariables(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get request variables: %w", err)
	}

	requestBody, params := getRequestBodyAndParams(ctx)

	varsValue := vm.ToValue(variables)
	requestBodyValue := vm.ToValue(requestBody)
	paramsValue := vm.ToValue(params)

	result, err := fn(goja.Undefined(), varsValue, requestBodyValue, paramsValue)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to execute servflowRun: %w", err)
	}

	return result.Export(), nil, nil
}

func getRequestBodyAndParams(ctx context.Context) (string, map[string]string) {
	params := make(map[string]string)
	requestBody := ""

	req, err := plan.RequestFromContext(ctx)
	if err != nil {
		return requestBody, params
	}

	requestBody = requestctx.ReadAndRestoreBody(req)
	params = getRequestParams(req)

	return requestBody, params
}

func getRequestParams(req *http.Request) map[string]string {
	params := make(map[string]string)

	// Get URL path params from mux
	for key, value := range mux.Vars(req) {
		params[key] = value
	}

	// Get query params (URL params take precedence if same key exists)
	for key, values := range req.URL.Query() {
		if _, exists := params[key]; !exists && len(values) > 0 {
			params[key] = values[0]
		}
	}

	return params
}

func init() {
	fields := map[string]actions.FieldInfo{
		"script": {
			Type:        actions.FieldTypeTextArea,
			Label:       "Script",
			Placeholder: "JavaScript code containing servflowRun function",
			Required:    true,
			Metadata: map[string]string{
				"language": "javascript",
			},
		},
		"dependencies": {
			Type:        actions.FieldTypeTextArea,
			Label:       "Dependencies",
			Placeholder: "Bundled JavaScript dependencies (optional)",
			Required:    false,
		},
	}

	if err := actions.RegisterAction("javascript", actions.ActionRegistrationInfo{
		Name:        "JavaScript",
		Description: "Executes JavaScript code using a servflowRun function with access to request variables",
		Fields:      fields,
		Constructor: func(config json.RawMessage) (actions.ActionExecutable, error) {
			var cfg Config
			if err := json.Unmarshal(config, &cfg); err != nil {
				return nil, fmt.Errorf("error creating javascript action: %w", err)
			}
			return NewExecutable(cfg)
		},
	}); err != nil {
		panic(err)
	}
}
