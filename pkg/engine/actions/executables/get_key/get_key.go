package get_key

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/Servflow/servflow/pkg/engine/plan"
	"github.com/Servflow/servflow/pkg/logging"
	"github.com/Servflow/servflow/pkg/storage"
	"go.uber.org/zap"
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

func (g *GetKey) SupportsReplica() bool {
	return true
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
	logger := logging.FromContext(ctx).With(zap.String("execution_type", g.Type()))
	_ = logging.WithLogger(ctx, logger)

	var cfg Config
	if err := json.Unmarshal([]byte(modifiedConfig), &cfg); err != nil {
		return nil, nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if cfg.Key == "" {
		return nil, nil, nil
	}

	value, found, err := storage.Get(cfg.Key)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get key: %w", err)
	}

	if !found {
		if cfg.FailIfEmpty {
			return nil, nil, fmt.Errorf("%w: key '%s' not found", plan.ErrFailure, cfg.Key)
		}
		return "", nil, nil
	}

	return value, nil, nil
}

func init() {
	fields := map[string]actions.FieldInfo{
		"key": {
			Type:        actions.FieldTypeString,
			Label:       "Key",
			Placeholder: "Storage key to retrieve",
			Required:    true,
		},
		"failIfEmpty": {
			Type:        actions.FieldTypeBoolean,
			Label:       "Fail if Empty",
			Placeholder: "Treat missing key as failure",
			Required:    false,
			Default:     false,
		},
	}

	if err := actions.RegisterAction("get_key", actions.ActionRegistrationInfo{
		Name:        "Get Key",
		Description: "Retrieves a value from persistent storage by key",
		Fields:      fields,
		Constructor: func(config json.RawMessage) (actions.ActionExecutable, error) {
			var cfg Config
			if err := json.Unmarshal(config, &cfg); err != nil {
				return nil, fmt.Errorf("error creating get_key action: %v", err)
			}
			return NewExecutable(cfg), nil
		},
	}); err != nil {
		panic(err)
	}
}
