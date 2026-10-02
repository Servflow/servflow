package storevector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/Servflow/servflow/pkg/engine/integration"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
)

type storeVectorIntegration interface {
	integration.Integration
	StoreVectors(vectors []float32, fields map[string]any, options map[string]string) error
}

type Config struct {
	Integration string            `json:"integration,omitempty"`
	Fields      map[string]any    `json:"fields"`
	Options     map[string]string `json:"options,omitempty"`
	Vectors     string            `json:"vectors"`
}

type StoreVectors struct {
	cfg *Config
}

func (s StoreVectors) Type() string {
	return "storevector"
}

func (s StoreVectors) Config() string {
	cfg := *s.cfg
	cfg.Options = nil
	cfg.Integration = ""
	dat, err := json.Marshal(cfg)
	if err != nil {
		return ""
	}
	return string(dat)
}

func (s StoreVectors) Execute(ctx context.Context, modifiedConfig string) (interface{}, map[string]string, error) {
	var newCfg Config
	err := json.Unmarshal([]byte(modifiedConfig), &newCfg)
	if err != nil {
		return nil, nil, err
	}

	var vectors []float32
	err = json.Unmarshal([]byte(newCfg.Vectors), &vectors)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid value for vectors: %w", err)
	}

	i, err := requestctx.GetIntegration(ctx, s.cfg.Integration)
	if err != nil {
		return nil, nil, err
	}
	impl, ok := i.(storeVectorIntegration)
	if !ok {
		return nil, nil, errors.New("integration does not implement vector storage")
	}

	err = impl.StoreVectors(vectors, newCfg.Fields, s.cfg.Options)
	if err != nil {
		return nil, nil, err
	}

	return nil, nil, nil
}

func New(config Config) (*StoreVectors, error) {
	if config.Integration == "" {
		return nil, fmt.Errorf("no integration ID provided")
	}
	return &StoreVectors{cfg: &config}, nil
}

// Definition describes the storevector action to a host that offers it.
func Definition() actions.Definition {
	fields := map[string]actions.FieldInfo{
		"integration": {
			Type:        actions.FieldTypeIntegration,
			Label:       "Vector Database",
			Description: "The vector store to write to",
			Required:    true,
		},
		"fields": {
			Type:        actions.FieldTypeMap,
			Label:       "Fields",
			Description: "Data fields to store",
			Required:    true,
		},
		"options": {
			Type:        actions.FieldTypeMap,
			Label:       "Options",
			Description: "Additional storage options",
			Required:    false,
		},
		"vectors": {
			Type:        actions.FieldTypeString,
			Label:       "Vectors",
			Description: "Vector data to store",
			Required:    true,
		},
	}

	return actions.Definition{
		Type:        "storevector",
		Name:        "Store Vectors",
		Description: "Stores vector embeddings into vector databases for similarity search",
		Fields:      fields,
		Output: actions.OutputInfo{
			Kind:        actions.OutputNone,
			Description: "Storing vectors reports success by continuing; it publishes nothing.",
		},
		New: func(_ context.Context, config json.RawMessage) (actions.ActionExecutable, error) {
			var cfg Config
			if err := json.Unmarshal(config, &cfg); err != nil {
				return nil, fmt.Errorf("error creating storevector action: %v", err)
			}
			return New(cfg)
		},
	}
}
