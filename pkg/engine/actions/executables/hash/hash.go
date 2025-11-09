package hash

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Servflow/servflow/pkg/engine/actions"
	"golang.org/x/crypto/bcrypt"
)

const (
	Bcrypt = "bcrypt"
)

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

func (h *Hash) Config() string {
	return h.value
}

func (h *Hash) Execute(ctx context.Context, modifiedConfig string) (interface{}, error) {
	res, err := bcrypt.GenerateFromPassword([]byte(modifiedConfig), 10)
	if err != nil {
		return "", err
	}
	return string(res), nil
}

func init() {
	fields := map[string]actions.FieldInfo{
		"value": {
			Type:        "string",
			Label:       "Value",
			Placeholder: "Value to hash",
			Required:    true,
		},
		"algorithm": {
			Type:        "string",
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
		Constructor: func(config json.RawMessage) (actions.ActionExecutable, error) {
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
	}); err != nil {
		panic(err)
	}
}
