package fetch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/Servflow/servflow/pkg/engine/integration"
	"github.com/Servflow/servflow/pkg/engine/integration/integrations/filters"
	"github.com/Servflow/servflow/pkg/logging"
	"go.uber.org/zap"
)

type Fetch struct {
	cfg               *Config
	fetchIntegrations fetchImplementation
}

func (f *Fetch) Type() string {
	return "fetch"
}

type fetchImplementation interface {
	integration.Integration
	Fetch(ctx context.Context, options map[string]string, filters ...filters.Filter) ([]map[string]interface{}, error)
}

type Config struct {
	IntegrationID     string            `json:"integrationID"`
	Filters           []filters.Filter  `json:"filters"`
	Table             string            `json:"table"`
	DatasourceOptions map[string]string `json:"datasourceOptions"`
	Single            bool              `json:"single"`
	ShouldFail        bool              `json:"shouldFail"`
}

func New(config Config) (*Fetch, error) {
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

	u, ok := i.(fetchImplementation)
	if !ok {
		return nil, errors.New("integration is not of type fetchImplementation")
	}
	return &Fetch{
		cfg:               &config,
		fetchIntegrations: u,
	}, nil
}

func (f *Fetch) Config() string {
	filtersStr, err := json.Marshal(f.cfg.Filters)
	if err != nil {
		return ""
	}
	return string(filtersStr)
}

func (f *Fetch) Execute(ctx context.Context, modifiedConfig string) (interface{}, error) {
	logger := logging.FromContext(ctx).With(zap.String("execution_type", f.Type()))
	ctx = logging.WithLogger(ctx, logger)

	var filters []filters.Filter
	if err := json.Unmarshal([]byte(modifiedConfig), &filters); err != nil {
		return "", err
	}

	var ret interface{}
	resp, err := f.fetchIntegrations.Fetch(ctx, map[string]string{"collection": f.cfg.Table}, filters...)
	if err != nil {
		return "", fmt.Errorf("fetch with filters: %v", err)
	}
	ret = resp
	if len(resp) < 1 && !f.cfg.ShouldFail {
		return map[string]interface{}{}, nil
	} else if len(resp) < 1 && f.cfg.ShouldFail {
		return nil, fmt.Errorf("no data found")
	}
	if f.cfg.Single && len(resp) > 0 {
		ret = resp[0]
	}
	return ret, nil
}

func init() {
	fields := map[string]actions.FieldInfo{
		"integrationID": {
			Type:        actions.FieldTypeIntegration,
			Label:       "Integration ID",
			Placeholder: "Database integration identifier",
			Required:    true,
		},
		"filters": {
			Type:        "array",
			Label:       "Filters",
			Placeholder: "Query filters",
			Required:    false,
		},
		"table": {
			Type:        actions.FieldTypeString,
			Label:       "Table",
			Placeholder: "Database table name",
			Required:    true,
		},
		"datasourceOptions": {
			Type:        actions.FieldTypeMap,
			Label:       "Datasource Options",
			Placeholder: "Additional datasource options",
			Required:    false,
		},
		"single": {
			Type:        actions.FieldTypeBoolean,
			Label:       "Single Result",
			Placeholder: "Return single result instead of array",
			Required:    false,
			Default:     false,
		},
		"shouldFail": {
			Type:        actions.FieldTypeBoolean,
			Label:       "Should Fail",
			Placeholder: "Whether the action should fail on error",
			Required:    false,
			Default:     false,
		},
	}

	if err := actions.RegisterAction("fetch", actions.ActionRegistrationInfo{
		Name:        "Fetch Data",
		Description: "Retrieves data from database tables using filters and conditions",
		Fields:      fields,
		Constructor: func(config json.RawMessage) (actions.ActionExecutable, error) {
			var cfg Config
			if err := json.Unmarshal(config, &cfg); err != nil {
				return nil, fmt.Errorf("error creating fetch action: %v", err)
			}
			return New(cfg)
		},
	}); err != nil {
		panic(err)
	}
}
