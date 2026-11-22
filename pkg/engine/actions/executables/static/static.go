package static

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/Servflow/servflow/pkg/logging"
	"go.uber.org/zap"
)

// Executable returns a value it resolves when it runs.
type Executable struct {
	Return string
}

func (s *Executable) Type() string {
	return "static"
}

type Config struct {
	Return string `json:"return"`
}

func NewExecutable(cfg Config) *Executable {
	return &Executable{
		Return: cfg.Return,
	}
}

// Execute resolves the return value template and returns the result
func (s *Executable) Execute(ctx context.Context) (interface{}, map[string]string, error) {
	logger := logging.FromContext(ctx).With(zap.String("execution_type", s.Type()))
	ctx = logging.WithLogger(ctx, logger)

	// Get request context for template resolution
	rc, err := requestctx.FromContextOrError(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get request context: %w", err)
	}

	// Resolve the return value template
	resolved, err := rc.Resolve(ctx, s.Return)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resolve return value: %w", err)
	}

	logger.Debug("static action resolved", zap.String("original", s.Return), zap.String("resolved", resolved))

	return resolved, nil, nil
}

// Definition describes the static action to a host that offers it.
func Definition() actions.Definition {
	fields := map[string]actions.FieldInfo{
		"return": {
			Type:        actions.FieldTypeString,
			Label:       "Return Value",
			Description: "Value to return",
			Required:    true,
		},
		"config": {
			Type:        actions.FieldTypeString,
			Label:       "Config",
			Description: "Configuration string",
			Required:    false,
		},
	}

	return actions.Definition{
		Type:        "static",
		Name:        "Static Value",
		Description: "Returns a static value configured at setup time",
		Fields:      fields,
		Output: actions.OutputInfo{
			Kind:        actions.OutputValue,
			Description: "The text of the return template, with its variables filled in.",
		},
		New: func(_ context.Context, config json.RawMessage) (actions.ActionExecutable, error) {
			var cfg Config
			if err := json.Unmarshal(config, &cfg); err != nil {
				return nil, fmt.Errorf("error creating static action: %v", err)
			}
			return NewExecutable(cfg), nil
		},
	}
}
