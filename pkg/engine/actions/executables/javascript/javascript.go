package javascript

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/Servflow/servflow/pkg/logging"
	"github.com/dop251/goja"
	"go.uber.org/zap"
)

type Executable struct {
	compiledScript *goja.Program
	compiledDeps   *goja.Program
}

type Config struct {
	Script       string `json:"script"`
	Dependencies string `json:"dependencies"`
}

func NewExecutable(cfg Config) (*Executable, error) {
	if cfg.Script == "" {
		return nil, fmt.Errorf("script is required")
	}

	compiledScript, err := goja.Compile("script", cfg.Script, false)
	if err != nil {
		return nil, fmt.Errorf("failed to compile script: %w", err)
	}

	var compiledDeps *goja.Program
	if cfg.Dependencies != "" {
		compiledDeps, err = goja.Compile("dependencies", cfg.Dependencies, false)
		if err != nil {
			return nil, fmt.Errorf("failed to compile dependencies: %w", err)
		}
	}

	return &Executable{
		compiledScript: compiledScript,
		compiledDeps:   compiledDeps,
	}, nil
}

func (e *Executable) Type() string {
	return "javascript"
}

func (e *Executable) Config() string {
	return ""
}

func (e *Executable) Execute(ctx context.Context, modifiedConfig string) (interface{}, error) {
	logger := logging.FromContext(ctx).With(zap.String("execution_type", e.Type()))
	ctx = logging.WithLogger(ctx, logger)

	vm := goja.New()

	if e.compiledDeps != nil {
		_, err := vm.RunProgram(e.compiledDeps)
		if err != nil {
			return nil, fmt.Errorf("failed to execute dependencies: %w", err)
		}
	}

	_, err := vm.RunProgram(e.compiledScript)
	if err != nil {
		return nil, fmt.Errorf("failed to execute script: %w", err)
	}

	servflowRun := vm.Get("servflowRun")
	if servflowRun == nil || goja.IsUndefined(servflowRun) {
		return nil, fmt.Errorf("servflowRun function not defined in script")
	}

	fn, ok := goja.AssertFunction(servflowRun)
	if !ok {
		return nil, fmt.Errorf("servflowRun is not a function")
	}

	variables, err := requestctx.GetAllRequestVariables(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get request variables: %w", err)
	}

	varsValue := vm.ToValue(variables)

	result, err := fn(goja.Undefined(), varsValue)
	if err != nil {
		return nil, fmt.Errorf("failed to execute servflowRun: %w", err)
	}

	return result.Export(), nil
}

func init() {
	fields := map[string]actions.FieldInfo{
		"script": {
			Type:        actions.FieldTypeTextArea,
			Label:       "Script",
			Placeholder: "JavaScript code containing servflowRun function",
			Required:    true,
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
