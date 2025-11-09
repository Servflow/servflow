package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/Servflow/servflow/pkg/engine/integration"
	"github.com/Servflow/servflow/pkg/engine/integration/integrations/filters"
)

type updateIntegration interface {
	integration.Integration
	Update(ctx context.Context, data map[string]interface{}, options map[string]string, filter ...filters.Filter) error
}

type Config struct {
	IntegrationID     string                 `json:"integrationID"`
	Filters           []filters.Filter       `json:"filters"`
	Table             string                 `json:"table"`
	DatasourceOptions map[string]string      `json:"datasourceOptions"`
	Fields            map[string]interface{} `json:"fields"`
}

type Update struct {
	cfg *Config
	i   updateIntegration
}

func (u *Update) Type() string {
	return "update"
}

func New(config Config) (*Update, error) {
	if config.IntegrationID == "" {
		return nil, errors.New("datasource is required")
	}
	if config.Table == "" {
		return nil, errors.New("table is required")
	}
	i, err := integration.GetIntegration(config.IntegrationID)
	if err != nil {
		return nil, err
	}

	u, ok := i.(updateIntegration)
	if !ok {
		return nil, errors.New("integration does not implement updateIntegration")
	}

	return &Update{
		cfg: &config,
		i:   u,
	}, nil
}

func (u *Update) Config() string {
	cfgStr, err := json.Marshal(u.cfg)
	if err != nil {
		return ""
	}
	return string(cfgStr)
}

func (u *Update) Execute(ctx context.Context, modifiedConfig string) (interface{}, error) {
	var cfg Config
	if err := json.Unmarshal([]byte(modifiedConfig), &cfg); err != nil {
		return nil, err
	}

	err := u.i.Update(ctx, cfg.Fields, map[string]string{"collection": u.cfg.Table}, cfg.Filters...)
	if err != nil {
		return nil, fmt.Errorf("update operation failed: %w", err)
	}
	return nil, nil
}

func init() {
	fields := map[string]actions.FieldInfo{
		"integrationID": {
			Type:        "string",
			Label:       "Integration ID",
			Placeholder: "Database integration identifier",
			Required:    true,
		},
		"filters": {
			Type:        "array",
			Label:       "Filters",
			Placeholder: "Query filters to identify records to update",
			Required:    true,
		},
		"table": {
			Type:        "string",
			Label:       "Table",
			Placeholder: "Database table name",
			Required:    true,
		},
		"datasourceOptions": {
			Type:        "object",
			Label:       "Datasource Options",
			Placeholder: "Additional datasource options",
			Required:    false,
		},
		"fields": {
			Type:        "object",
			Label:       "Fields",
			Placeholder: "Data fields to update",
			Required:    true,
		},
	}

	if err := actions.RegisterAction("update", actions.ActionRegistrationInfo{
		Name:        "Update Data",
		Description: "Updates existing records in database tables using filters and field mappings",
		Fields:      fields,
		Constructor: func(config json.RawMessage) (actions.ActionExecutable, error) {
			var cfg Config
			if err := json.Unmarshal(config, &cfg); err != nil {
				return nil, fmt.Errorf("error creating update action: %v", err)
			}
			return New(cfg)
		},
	}); err != nil {
		panic(err)
	}
}
