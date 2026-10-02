package fetch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/Servflow/servflow/pkg/engine/integration"
	"github.com/Servflow/servflow/pkg/engine/integration/integrations/filters"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/Servflow/servflow/pkg/logging"
	"go.uber.org/zap"
)

type Fetch struct {
	cfg *Config
}

func (f *Fetch) Type() string {
	return "fetch"
}

type fetchImplementation interface {
	integration.Integration
	Fetch(ctx context.Context, options map[string]string, filters ...filters.Filter) ([]map[string]interface{}, error)
}

type Config struct {
	Integration string           `json:"integration" yaml:"integration"`
	Filters     []filters.Filter `json:"filters" yaml:"filters"`
	Table       string           `json:"table" yaml:"table"`
	Single      bool             `json:"single" yaml:"single"`
	FailIfEmpty bool             `json:"failIfEmpty" yaml:"failIfEmpty"`
}

func New(config Config) (*Fetch, error) {
	if config.Integration == "" {
		return nil, errors.New("datasource is required")
	}
	if config.Table == "" {
		return nil, errors.New("table is required")
	}
	return &Fetch{cfg: &config}, nil
}

func (f *Fetch) Config() string {
	filtersStr, err := json.Marshal(f.cfg.Filters)
	if err != nil {
		return ""
	}
	return string(filtersStr)
}

func (f *Fetch) Execute(ctx context.Context, modifiedConfig string) (interface{}, map[string]string, error) {
	logger := logging.FromContext(ctx).With(zap.String("execution_type", f.Type()))
	ctx = logging.WithLogger(ctx, logger)

	var filters []filters.Filter
	if err := json.Unmarshal([]byte(modifiedConfig), &filters); err != nil {
		return "", nil, err
	}

	i, err := requestctx.GetIntegration(ctx, f.cfg.Integration)
	if err != nil {
		return "", nil, err
	}
	impl, ok := i.(fetchImplementation)
	if !ok {
		return "", nil, errors.New("integration is not of type fetchImplementation")
	}

	var ret interface{}
	resp, err := impl.Fetch(ctx, map[string]string{"collection": f.cfg.Table}, filters...)
	if err != nil {
		return "", nil, fmt.Errorf("fetch with filters: %v", err)
	}
	ret = resp
	if len(resp) < 1 {
		if f.cfg.FailIfEmpty {
			return nil, nil, fmt.Errorf("%w: no data found", actions.ErrFailure)
		}
		return map[string]interface{}{}, nil, nil
	}
	if f.cfg.Single && len(resp) > 0 {
		ret = resp[0]
	}
	return ret, nil, nil
}

// Definition describes the fetch action to a host that offers it.
func Definition() actions.Definition {
	fields := map[string]actions.FieldInfo{
		"integration": {
			Type:        actions.FieldTypeIntegration,
			Label:       "Database Integration",
			Description: "The SQL or MongoDB integration to read from",
			Required:    true,
		},
		"filters": {
			Type:        actions.FieldTypeMap,
			Label:       "Filters",
			Description: "Query filters",
			Required:    false,
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
		"single": {
			Type:        actions.FieldTypeBoolean,
			Label:       "Single Result",
			Description: "Return single result instead of array",
			Required:    false,
			Default:     false,
		},
		"shouldFail": {
			Type:        actions.FieldTypeBoolean,
			Label:       "Should Fail",
			Description: "Whether the action should fail on error",
			Required:    false,
			Default:     false,
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
		Type:        "fetch",
		Name:        "Fetch Data",
		Description: "Retrieves data from database tables using filters and conditions",
		Fields:      fields,
		Output: actions.OutputInfo{
			Kind:        actions.OutputDynamic,
			Description: "The rows the query matched, with the table's own column names. One row when single is set, otherwise a list.",
		},
		New: func(_ context.Context, config json.RawMessage) (actions.ActionExecutable, error) {
			var cfg Config
			if err := json.Unmarshal(config, &cfg); err != nil {
				return nil, fmt.Errorf("error creating fetch action: %v", err)
			}
			return New(cfg)
		},
	}
}
