package get_key

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/Servflow/servflow/pkg/engine/kv"
)

type GetKey struct {
	key         string
	failIfEmpty bool
}

type Config struct {
	Key         string `json:"key"`
	FailIfEmpty bool   `json:"failIfEmpty"`
}

func NewExecutable(cfg Config) *GetKey {
	return &GetKey{
		key:         cfg.Key,
		failIfEmpty: cfg.FailIfEmpty,
	}
}

func (g *GetKey) Type() string {
	return "get_key"
}

func (g *GetKey) Config() string {
	cfg := Config{
		Key:         g.key,
		FailIfEmpty: g.failIfEmpty,
	}
	configBytes, err := json.Marshal(cfg)
	if err != nil {
		return ""
	}
	return string(configBytes)
}

func (g *GetKey) Execute(ctx context.Context, modifiedConfig string) (interface{}, map[string]string, error) {

	var cfg Config
	if err := json.Unmarshal([]byte(modifiedConfig), &cfg); err != nil {
		return nil, nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if cfg.Key == "" {
		return nil, nil, nil
	}

	value, found, err := kv.Get(cfg.Key)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get key: %w", err)
	}

	if !found {
		if cfg.FailIfEmpty {
			return nil, nil, fmt.Errorf("%w: key '%s' not found", actions.ErrFailure, cfg.Key)
		}
		return "", nil, nil
	}

	return value, nil, nil
}

// Definition describes the get_key action to a host that offers it.
func Definition() actions.Definition {
	fields := map[string]actions.FieldInfo{
		"key": {
			Type:        actions.FieldTypeString,
			Label:       "Key",
			Description: "Storage key to retrieve",
			Required:    true,
		},
		"failIfEmpty": {
			Type:        actions.FieldTypeBoolean,
			Label:       "Fail if Empty",
			Description: "Treat missing key as failure",
			Required:    false,
			Default:     false,
		},
	}

	return actions.Definition{
		Type:        "get_key",
		Name:        "Get Key",
		Description: "Retrieves a value from persistent storage by key",
		Fields:      fields,
		Output: actions.OutputInfo{
			Kind:        actions.OutputValue,
			Description: "The stored value, or empty when the key is not set.",
		},
		New: func(config json.RawMessage) (actions.ActionExecutable, error) {
			var cfg Config
			if err := json.Unmarshal(config, &cfg); err != nil {
				return nil, fmt.Errorf("error creating get_key action: %v", err)
			}
			return NewExecutable(cfg), nil
		},
	}
}
