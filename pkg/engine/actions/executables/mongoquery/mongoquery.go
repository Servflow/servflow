package mongoquery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
)

type Config struct {
	Collection  string `json:"collection" yaml:"collection"`
	FilterQuery string `json:"filterQuery" yaml:"filterQuery"`
	Projection  string `json:"projection" yaml:"projection"`
	Integration string `json:"integration" yaml:"integration"`
	FailIfEmpty bool   `json:"failIfEmpty" yaml:"failIfEmpty"`
}

type mongoDBIntegration interface {
	ExecuteQuery(ctx context.Context, collection string, filterQuery string, projectionQuery string) ([]map[string]interface{}, error)
}

type MGOQuery struct {
	config Config
}

func (m *MGOQuery) Config() string {
	b, err := json.Marshal(m.config)
	if err != nil {
		return ""
	}
	return string(b)
}

func New(config Config) (*MGOQuery, error) {
	if config.Integration == "" {
		return nil, errors.New("integration is required")
	}
	if config.Collection == "" {
		return nil, errors.New("collection is required")
	}

	return &MGOQuery{config: config}, nil
}

func (m *MGOQuery) Execute(ctx context.Context, modifiedConfig string) (interface{}, map[string]string, error) {
	var cfg Config
	if err := json.Unmarshal([]byte(modifiedConfig), &cfg); err != nil {
		return nil, nil, err
	}
	m.config = cfg

	i, err := requestctx.GetIntegration(ctx, cfg.Integration)
	if err != nil {
		return nil, nil, err
	}
	impl, ok := i.(mongoDBIntegration)
	if !ok {
		return nil, nil, errors.New("integration does not implement mongoDBIntegration")
	}

	result, err := impl.ExecuteQuery(ctx, cfg.Collection, cfg.FilterQuery, cfg.Projection)
	if err != nil {
		return nil, nil, fmt.Errorf("error executing integration: %v", err)
	}

	if len(result) == 0 && cfg.FailIfEmpty {
		return nil, nil, fmt.Errorf("%w: no documents found", actions.ErrFailure)
	}

	return result, nil, nil

}

func (m *MGOQuery) Type() string {
	return "mongoquery"
}

// Definition describes the mongoquery action to a host that offers it.
func Definition() actions.Definition {
	fields := map[string]actions.FieldInfo{
		"collection": {
			Type:        actions.FieldTypeString,
			Label:       "Collection",
			Description: "MongoDB collection name",
			Required:    true,
		},
		"filterQuery": {
			Type:        actions.FieldTypeString,
			Label:       "Filter Query",
			Description: "MongoDB filter query",
			Required:    true,
		},
		"projection": {
			Type:        actions.FieldTypeString,
			Label:       "Projection",
			Description: "MongoDB projection query",
			Required:    false,
		},
		"integration": {
			Type:        actions.FieldTypeIntegration,
			Label:       "MongoDB Integration",
			Description: "The MongoDB integration to query",
			Required:    true,
		},
		"failIfEmpty": {
			Type:        actions.FieldTypeBoolean,
			Label:       "Fail if Empty",
			Description: "Treat no results as failure",
			Required:    false,
			Default:     true,
		},
	}

	return actions.Definition{
		Type:        "mongoquery",
		Name:        "MongoDB Query",
		Description: "Executes queries against MongoDB collections with filtering and projection",
		Fields:      fields,
		Output: actions.OutputInfo{
			Kind:        actions.OutputDynamic,
			Description: "A list of the matching documents, with the collection's own field names.",
		},
		New: func(config json.RawMessage) (actions.ActionExecutable, error) {
			var cfg Config
			if err := json.Unmarshal(config, &cfg); err != nil {
				return nil, fmt.Errorf("error creating mongoquery action: %v", err)
			}
			return New(cfg)
		},
	}
}
