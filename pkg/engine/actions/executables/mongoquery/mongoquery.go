package mongoquery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/Servflow/servflow/pkg/engine/integration"
)

type Config struct {
	Collection    string `json:"collection"`
	FilterQuery   string `json:"filter"`
	Projection    string `json:"projection"`
	IntegrationID string `json:"integrationID"`
}

type mongoDBIntegration interface {
	ExecuteQuery(ctx context.Context, collection string, filterQuery string, projectionQuery string) ([]map[string]interface{}, error)
}

type MGOQuery struct {
	config Config
	i      mongoDBIntegration
}

func (m *MGOQuery) Config() string {
	b, err := json.Marshal(m.config)
	if err != nil {
		return ""
	}
	return string(b)
}

func New(config Config) (*MGOQuery, error) {
	if config.IntegrationID == "" {
		return nil, errors.New("IntegrationID is required")
	}
	if config.Collection == "" {
		return nil, errors.New("collection is required")
	}

	i, err := integration.GetIntegration(config.IntegrationID)
	if err != nil {
		return nil, err
	}

	u, ok := i.(mongoDBIntegration)
	if !ok {
		return nil, errors.New("integration does not implement mongoDBIntegration")
	}

	return &MGOQuery{
		config: config,
		i:      u,
	}, nil
}

func (m *MGOQuery) Execute(ctx context.Context, modifiedConfig string) (interface{}, error) {
	var cfg Config
	if err := json.Unmarshal([]byte(modifiedConfig), &cfg); err != nil {
		return nil, err
	}
	m.config = cfg

	result, err := m.i.ExecuteQuery(ctx, cfg.Collection, cfg.FilterQuery, cfg.Projection)
	if err != nil {
		return nil, fmt.Errorf("error executing integration: %v", err)
	}

	return result, nil

}

func (m *MGOQuery) Type() string {
	return "mongoquery"
}

func init() {
	fields := map[string]actions.FieldInfo{
		"collection": {
			Type:        "string",
			Label:       "Collection",
			Placeholder: "MongoDB collection name",
			Required:    true,
		},
		"filter": {
			Type:        "string",
			Label:       "Filter Query",
			Placeholder: "MongoDB filter query",
			Required:    true,
		},
		"projection": {
			Type:        "string",
			Label:       "Projection",
			Placeholder: "MongoDB projection query",
			Required:    false,
		},
		"integrationID": {
			Type:        "string",
			Label:       "Integration ID",
			Placeholder: "MongoDB integration identifier",
			Required:    true,
		},
	}

	if err := actions.RegisterAction("mongoquery", func(config json.RawMessage) (actions.ActionExecutable, error) {
		var cfg Config
		if err := json.Unmarshal(config, &cfg); err != nil {
			return nil, fmt.Errorf("error creating mongoquery action: %v", err)
		}
		return New(cfg)
	}, fields); err != nil {
		panic(err)
	}
}
