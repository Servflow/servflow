package static

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/Servflow/servflow/pkg/logging"
	"go.uber.org/zap"
)

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

func (s *Executable) Config() string {
	return s.Return
}

func (s *Executable) Execute(ctx context.Context, modifiedConfig string) (interface{}, error) {
	logger := logging.FromContext(ctx).With(zap.String("execution_type", s.Type()))
	_ = logging.WithLogger(ctx, logger)

	return modifiedConfig, nil
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
		Constructor: func(config json.RawMessage) (actions.ActionExecutable, error) {
			var cfg Config
			if err := json.Unmarshal(config, &cfg); err != nil {
				return nil, fmt.Errorf("error creating static action: %v", err)
			}
			return NewExecutable(cfg), nil
		},
	}); err != nil {
		panic(err)
	}
}
