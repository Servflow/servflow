//go:generate mockgen -source delete.go -destination delete_mock.go -package delete_action
package delete_action

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/Servflow/servflow/pkg/engine/integration"
	"github.com/Servflow/servflow/pkg/engine/integration/integrations/filters"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
)

type Delete struct {
	cfg *Config
}

func (d *Delete) Config() string {
	filtersStr, err := json.Marshal(d.cfg.Filters)
	if err != nil {
		return ""
	}
	return string(filtersStr)
}

func (d *Delete) Type() string {
	return "delete"
}

type Config struct {
	Integration       string            `json:"integration"`
	Filters           []filters.Filter  `json:"filters"`
	Table             string            `json:"table"`
	DatasourceOptions map[string]string `json:"datasourceOptions"`
}

type deleteImplementation interface {
	integration.Integration
	Delete(ctx context.Context, options map[string]string, filters ...filters.Filter) error
}

func New(config Config) (*Delete, error) {
	if config.Integration == "" {
		return nil, errors.New("datasource is required")
	}
	if config.Table == "" {
		return nil, errors.New("table is required")
	}
	return &Delete{cfg: &config}, nil
}

func (d *Delete) Execute(ctx context.Context, modifiedConfig string) (interface{}, map[string]string, error) {
	var filters []filters.Filter
	if err := json.Unmarshal([]byte(modifiedConfig), &filters); err != nil {
		return "", nil, err
	}

	i, err := requestctx.GetIntegration(ctx, d.cfg.Integration)
	if err != nil {
		return "", nil, err
	}
	impl, ok := i.(deleteImplementation)
	if !ok {
		return "", nil, errors.New("integration is not of type deleteImplementation")
	}

	var ret interface{}
	err = impl.Delete(ctx, map[string]string{"collection": d.cfg.Table}, filters...)
	if err != nil {
		return "", nil, fmt.Errorf("delete with filters: %v", err)
	}
	return ret, nil, nil
}

// Definition describes the delete action to a host that offers it.
func Definition() actions.Definition {
	fields := map[string]actions.FieldInfo{
		"integration": {
			Type:        actions.FieldTypeIntegration,
			Label:       "Database Integration",
			Description: "The SQL or MongoDB integration to delete from",
			Required:    true,
		},
		"filters": {
			Type:        actions.FieldTypeMap,
			Label:       "Filters",
			Description: "Query filters to identify records to delete",
			Required:    true,
			Metadata: map[string]string{
				"type": "filter",
			},
		},
		"table": {
			Type:        actions.FieldTypeString,
			Label:       "Table",
			Description: "Database table name",
			Required:    true,
		},
		"datasourceOptions": {
			Type:        actions.FieldTypeMap,
			Label:       "Datasource Options",
			Description: "Additional datasource options",
			Required:    false,
		},
	}

	return actions.Definition{
		Type:        "delete",
		Name:        "Delete Data",
		Description: "Deletes records from database tables based on specified filters",
		Fields:      fields,
		Output: actions.OutputInfo{
			Kind:        actions.OutputNone,
			Description: "Deleting reports success by continuing; it publishes nothing.",
		},
		New: func(config json.RawMessage) (actions.ActionExecutable, error) {
			var cfg Config
			if err := json.Unmarshal(config, &cfg); err != nil {
				return nil, fmt.Errorf("error creating delete action: %v", err)
			}
			return New(cfg)
		},
	}
}
