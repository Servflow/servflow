package store_key

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

type StoreKey struct {
	key   string
	value string
}

type Config struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func NewExecutable(cfg Config) *StoreKey {
	return &StoreKey{
		key:   cfg.Key,
		value: cfg.Value,
	}
}

func (s *StoreKey) Type() string {
	return "store_key"
}

func (s *StoreKey) SupportsReplica() bool {
	return true
}

func (s *StoreKey) Config() string {
	return s.value
}

func (s *StoreKey) Execute(ctx context.Context, modifiedConfig string) (interface{}, map[string]string, error) {
	logger := logging.FromContext(ctx).With(zap.String("execution_type", s.Type()))
	_ = logging.WithLogger(ctx, logger)

	if s.key == "" {
		return nil, nil, errors.New("key cannot be empty")
	}

	if err := storage.Set(s.key, modifiedConfig); err != nil {
		return nil, nil, fmt.Errorf("failed to store key: %w", err)
	}

	return modifiedConfig, nil, nil
}

func init() {
	fields := map[string]actions.FieldInfo{
		"key": {
			Type:        actions.FieldTypeString,
			Label:       "Key",
			Placeholder: "Storage key",
			Required:    true,
		},
		"value": {
			Type:        actions.FieldTypeString,
			Label:       "Value",
			Placeholder: "Value to store",
			Required:    true,
		},
	}

	if err := actions.RegisterAction("store_key", actions.ActionRegistrationInfo{
		Name:        "Store Key",
		Description: "Stores a key-value pair in persistent storage",
		Fields:      fields,
		Constructor: func(config json.RawMessage) (actions.ActionExecutable, error) {
			var cfg Config
			if err := json.Unmarshal(config, &cfg); err != nil {
				return nil, fmt.Errorf("error creating store_key action: %v", err)
			}
			return NewExecutable(cfg), nil
		},
	}); err != nil {
		panic(err)
	}
}
