package fetchvector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/Servflow/servflow/pkg/engine/integration"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
)

type fetchVectorIntegration interface {
	integration.Integration
	FetchVector(inputVector []float32, options map[string]any) (results []map[string]any, err error)
}

// TODO unify fields accross all integration actions

type Config struct {
	Integration string         `json:"integration,omitempty"`
	Vector      string         `json:"vector,omitempty"`
	Options     map[string]any `json:"options,omitempty"`
}

type FetchVector struct {
	cfg *Config
}

func (f FetchVector) Type() string {
	return "fetchvector"
}

// Execute resolves the vector and options and returns the nearest stored
// vectors.
func (f FetchVector) Execute(ctx context.Context) (interface{}, map[string]string, error) {
	resolved, err := requestctx.ResolveValues(ctx, map[string]any{"vector": f.cfg.Vector, "options": f.cfg.Options})
	if err != nil {
		return nil, nil, err
	}
	fields := resolved.(map[string]any)
	vector, _ := fields["vector"].(string)
	options, _ := fields["options"].(map[string]any)

	var vectors []float32
	if err := json.Unmarshal([]byte(vector), &vectors); err != nil {
		return nil, nil, fmt.Errorf("invalid value for vectors: %v", err)
	}

	i, err := requestctx.GetIntegration(ctx, f.cfg.Integration)
	if err != nil {
		return nil, nil, err
	}
	impl, ok := i.(fetchVectorIntegration)
	if !ok {
		return nil, nil, errors.New("integration does not implement vector storage")
	}

	resultFields, err := impl.FetchVector(vectors, options)
	if err != nil {
		return nil, nil, fmt.Errorf("error fetching vectors: %v", err)
	}
	if len(resultFields) == 0 {
		return nil, nil, nil
	}

	return resultFields, nil, nil
}

func New(config Config) (*FetchVector, error) {
	if config.Integration == "" {
		return nil, fmt.Errorf("no integration ID provided")
	}
	return &FetchVector{cfg: &config}, nil
}

// Definition describes the fetchvectors action to a host that offers it.
func Definition() actions.Definition {
	fields := map[string]actions.FieldInfo{
		"integration": {
			Type:        actions.FieldTypeIntegration,
			Label:       "Vector Database",
			Description: "The vector store to search",
			Required:    true,
		},
		"vector": {
			Type:        actions.FieldTypeString,
			Label:       "Vector",
			Description: "Vector data or identifier",
			Required:    false,
		},
		"options": {
			Type:        actions.FieldTypeMap,
			Label:       "Options",
			Description: "Additional query options",
			Required:    false,
		},
	}

	return actions.Definition{
		Type:        "fetchvectors",
		Name:        "Fetch Vectors",
		Description: "Retrieves vector embeddings from vector databases for similarity search",
		Fields:      fields,
		Output: actions.OutputInfo{
			Kind:        actions.OutputDynamic,
			Description: "A list of the nearest stored vectors, each carrying the fields it was saved with.",
		},
		New: func(_ context.Context, config json.RawMessage) (actions.ActionExecutable, error) {
			var cfg Config
			if err := json.Unmarshal(config, &cfg); err != nil {
				return nil, fmt.Errorf("error creating fetchvector action: %v", err)
			}
			return New(cfg)
		},
	}
}
