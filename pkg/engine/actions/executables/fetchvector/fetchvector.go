package fetchvector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/Servflow/servflow/pkg/engine/integration"
)

type fetchVectorIntegration interface {
	integration.Integration
	FetchVector(inputVector []float32, options map[string]any) (results []map[string]any, err error)
}

// TODO unify fields accross all integration actions

type Config struct {
	IntegrationID string         `json:"integrationID,omitempty"`
	Vector        string         `json:"vector,omitempty"`
	Options       map[string]any `json:"options,omitempty"`
}

type FetchVector struct {
	cfg              *Config
	fetchIntegration fetchVectorIntegration
}

func (f FetchVector) Type() string {
	return "fetchvector"
}

func (f FetchVector) Config() string {
	dat, err := json.Marshal(f.cfg)
	if err != nil {
		return ""
	}
	return string(dat)
}

func (f FetchVector) Execute(ctx context.Context, modifiedConfig string) (interface{}, error) {
	var newCfg Config
	if err := json.Unmarshal([]byte(modifiedConfig), &newCfg); err != nil {
		return nil, err
	}

	var vectors []float32
	if err := json.Unmarshal([]byte(newCfg.Vector), &vectors); err != nil {
		return nil, fmt.Errorf("invalid value for vectors: %v", err)
	}

	resultFields, err := f.fetchIntegration.FetchVector(vectors, newCfg.Options)
	if err != nil {
		return nil, fmt.Errorf("error fetching vectors: %v", err)
	}
	if len(resultFields) == 0 {
		return nil, nil
	}

	return resultFields, nil
}

func New(config Config) (*FetchVector, error) {
	if config.IntegrationID == "" {
		return nil, fmt.Errorf("no integration ID provided")
	}
	i, err := integration.GetIntegration(config.IntegrationID)
	if err != nil {
		return nil, err
	}

	u, ok := i.(fetchVectorIntegration)
	if !ok {
		return nil, errors.New("integration does not implement vector storage")
	}

	return &FetchVector{
		cfg:              &config,
		fetchIntegration: u,
	}, nil
}

func init() {
	fields := map[string]actions.FieldInfo{
		"integrationID": {
			Type:        actions.FieldTypeIntegration,
			Label:       "Integration ID",
			Placeholder: "Vector database integration identifier",
			Required:    false,
		},
		"vector": {
			Type:        actions.FieldTypeString,
			Label:       "Vector",
			Placeholder: "Vector data or identifier",
			Required:    false,
		},
		"options": {
			Type:        actions.FieldTypeMap,
			Label:       "Options",
			Placeholder: "Additional query options",
			Required:    false,
		},
	}

	if err := actions.RegisterAction("fetchvectors", actions.ActionRegistrationInfo{
		Name:        "Fetch Vectors",
		Description: "Retrieves vector embeddings from vector databases for similarity search",
		Fields:      fields,
		Constructor: func(config json.RawMessage) (actions.ActionExecutable, error) {
			var cfg Config
			if err := json.Unmarshal(config, &cfg); err != nil {
				return nil, fmt.Errorf("error creating fetchvector action: %v", err)
			}
			return New(cfg)
		},
	}); err != nil {
		panic(err)
	}
}
