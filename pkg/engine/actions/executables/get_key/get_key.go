package get_key

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/Servflow/servflow/pkg/logging"
	"github.com/Servflow/servflow/pkg/storage"
	"go.uber.org/zap"
)

type GetKey struct {
	key string
}

type Config struct {
	Key string `json:"key"`
}

func NewExecutable(cfg Config) *GetKey {
	return &GetKey{
		key: cfg.Key,
	}
}

func (g *GetKey) Type() string {
	return "get_key"
}

func (g *GetKey) SupportsReplica() bool {
	return true
}

func (g *GetKey) Config() string {
	return g.key
}

func (g *GetKey) Execute(ctx context.Context, modifiedConfig string) (interface{}, map[string]string, error) {
	logger := logging.FromContext(ctx).With(zap.String("execution_type", g.Type()))
	_ = logging.WithLogger(ctx, logger)

	if modifiedConfig == "" {
		return nil, nil, errors.New("key cannot be empty")
	}

	value, found, err := storage.Get(modifiedConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get key: %w", err)
	}

	if !found {
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
