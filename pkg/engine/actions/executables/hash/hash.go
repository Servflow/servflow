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

// HashV2 is the V2 implementation that handles its own template resolution
type HashV2 struct {
	algorithm string
	value     string
}

func (h *HashV2) Type() string {
	return "hash"
}

func (h *HashV2) SupportsReplica() bool {
	return true
}

func NewV2(value, algorithm string) (*HashV2, error) {
	hash := &HashV2{value: value}
	switch algorithm {
	case Bcrypt:
		hash.algorithm = Bcrypt
	default:
		return nil, fmt.Errorf("unsupported hash algorithm: %s", algorithm)
	}

	return hash, nil
}

// Execute resolves the value template and generates the hash
func (h *HashV2) Execute(ctx context.Context) (interface{}, map[string]string, error) {
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

func init() {
	fields := map[string]actions.FieldInfo{
		"value": {
			Type:        actions.FieldTypeString,
			Label:       "Value",
			Placeholder: "Value to hash",
			Required:    true,
		},
		"algorithm": {
			Type:        actions.FieldTypeString,
			Label:       "Algorithm",
			Placeholder: "Hash algorithm (bcrypt)",
			Required:    true,
			Default:     "bcrypt",
		},
	}

	if err := actions.RegisterAction("hash", actions.ActionRegistrationInfo{
		Name:        "Hash Value",
		Description: "Generates cryptographic hashes using various algorithms like bcrypt",
		Fields:      fields,
		UseV2:       true,
		ConstructorV2: func(config json.RawMessage) (actions.ActionExecutableV2, error) {
			var cfg map[string]interface{}
			if err := json.Unmarshal(config, &cfg); err != nil {
				return nil, fmt.Errorf("error creating hash action: %v", err)
			}
			if f, ok := cfg["value"]; ok {
				if a, ok := cfg["algorithm"]; ok {
					field, algo := f.(string), a.(string)
					if field != "" && algo != "" {
						return NewV2(field, algo)
					}
				}
			}
			return nil, errors.New("invalid hash config")
		},
	}); err != nil {
		panic(err)
	}
}
