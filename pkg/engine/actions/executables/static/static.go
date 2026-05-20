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

// ExecutableV2 is the V2 implementation that handles its own template resolution
type ExecutableV2 struct {
	Return string
}

func (s *ExecutableV2) Type() string {
	return "static"
}

func (s *ExecutableV2) SupportsReplica() bool {
	return true
}

type Config struct {
	Return string `json:"return"`
}

func NewExecutableV2(cfg Config) *ExecutableV2 {
	return &ExecutableV2{
		Return: cfg.Return,
	}
}

// Execute resolves the return value template and returns the result
func (s *ExecutableV2) Execute(ctx context.Context) (interface{}, map[string]string, error) {
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

func init() {
	fields := map[string]actions.FieldInfo{
		"return": {
			Type:        actions.FieldTypeString,
			Label:       "Return Value",
			Placeholder: "Value to return",
			Required:    true,
		},
		"config": {
			Type:        actions.FieldTypeString,
			Label:       "Config",
			Placeholder: "Configuration string",
			Required:    false,
		},
	}

	if err := actions.RegisterAction("static", actions.ActionRegistrationInfo{
		Name:        "Static Value",
		Description: "Returns a static value configured at setup time",
		Fields:      fields,
		UseV2:       true,
		ConstructorV2: func(config json.RawMessage) (actions.ActionExecutableV2, error) {
			var cfg Config
			if err := json.Unmarshal(config, &cfg); err != nil {
				return nil, fmt.Errorf("error creating static action: %v", err)
			}
			return NewExecutableV2(cfg), nil
		},
	}); err != nil {
		panic(err)
	}
}
