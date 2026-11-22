package hash

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/Servflow/servflow/pkg/logging"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

const (
	Bcrypt = "bcrypt"
)

// Hash hashes a value it resolves when it runs.
type Hash struct {
	algorithm string
	value     string
}

func (h *Hash) Type() string {
	return "hash"
}

func New(value, algorithm string) (*Hash, error) {
	hash := &Hash{value: value}
	switch algorithm {
	case Bcrypt:
		hash.algorithm = Bcrypt
	default:
		return nil, fmt.Errorf("unsupported hash algorithm: %s", algorithm)
	}

	return hash, nil
}

// Execute resolves the value template and generates the hash
func (h *Hash) Execute(ctx context.Context) (interface{}, map[string]string, error) {
	logger := logging.FromContext(ctx).With(zap.String("execution_type", h.Type()))
	ctx = logging.WithLogger(ctx, logger)

	// Get request context for template resolution
	rc, err := requestctx.FromContextOrError(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get request context: %w", err)
	}

	// Resolve the value template
	resolved, err := rc.Resolve(ctx, h.value)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resolve value: %w", err)
	}

	logger.Debug("hash action resolving", zap.String("algorithm", h.algorithm))

	res, err := bcrypt.GenerateFromPassword([]byte(resolved), 10)
	if err != nil {
		return "", nil, err
	}
	return string(res), nil, nil
}

// Definition describes the hash action to a host that offers it.
func Definition() actions.Definition {
	fields := map[string]actions.FieldInfo{
		"value": {
			Type:        actions.FieldTypeString,
			Label:       "Value",
			Description: "Value to hash",
			Required:    true,
		},
		"algorithm": {
			Type:        actions.FieldTypeString,
			Label:       "Algorithm",
			Description: "Hash algorithm (bcrypt)",
			Required:    true,
			Default:     "bcrypt",
		},
	}

	return actions.Definition{
		Type:        "hash",
		Name:        "Hash Value",
		Description: "Generates cryptographic hashes using various algorithms like bcrypt",
		Fields:      fields,
		Output: actions.OutputInfo{
			Kind:        actions.OutputValue,
			Description: "The hash of the value.",
		},
		New: func(_ context.Context, config json.RawMessage) (actions.ActionExecutable, error) {
			var cfg map[string]interface{}
			if err := json.Unmarshal(config, &cfg); err != nil {
				return nil, fmt.Errorf("error creating hash action: %v", err)
			}
			if f, ok := cfg["value"]; ok {
				if a, ok := cfg["algorithm"]; ok {
					field, algo := f.(string), a.(string)
					if field != "" && algo != "" {
						return New(field, algo)
					}
				}
			}
			return nil, errors.New("invalid hash config")
		},
	}
}
