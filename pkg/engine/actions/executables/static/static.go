package static

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Servflow/servflow/pkg/engine/actions"
)

type Executable struct {
	ReturnConfig string
	Return       string
}

func (s *Executable) Type() string {
	return "static"
}

type Config struct {
	Return       string `json:"return"`
	ReturnConfig string `json:"config"`
}

func NewExecutable(cfg Config) *Executable {
	return &Executable{
		Return:       cfg.Return,
		ReturnConfig: cfg.ReturnConfig,
	}
}

func (s *Executable) Config() string {
	return s.ReturnConfig
}

func (s *Executable) Execute(ctx context.Context, modifiedConfig string) (interface{}, error) {
	return s.Return, nil
}

func init() {
	fields := map[string]actions.FieldInfo{
		"return": {
			Type:        "string",
			Label:       "Return Value",
			Placeholder: "Value to return",
			Required:    true,
		},
		"config": {
			Type:        "string",
			Label:       "Config",
			Placeholder: "Configuration string",
			Required:    false,
		},
	}

	if err := actions.RegisterAction("static", actions.ActionRegistration{
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
