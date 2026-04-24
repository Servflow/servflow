//go:generate mockgen -source write.go -destination write_mock.go -package write
package write

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/Servflow/servflow/pkg/engine/integration"
	"github.com/Servflow/servflow/pkg/logging"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type storageIntegrations interface {
	integration.Integration
	Store(ctx context.Context, data map[string]interface{}, options map[string]string) error
}

type Config struct {
	IntegrationID     string                 `json:"integrationID"`
	Table             string                 `json:"table"`
	DatasourceOptions map[string]string      `json:"datasourceOptions"`
	Fields            map[string]interface{} `json:"fields"`
}

type Write struct {
	cfg *Config
	i   storageIntegrations
}

func (s *Write) Type() string {
	return "store"
}

func (s *Write) SupportsReplica() bool {
	return true
}

func New(config Config) (*Write, error) {
	if config.IntegrationID == "" {
		return nil, errors.New("datasource is required")
	}
	if config.Table == "" {
		return nil, errors.New("table is required")
	}
	i, err := integration.GetIntegration(context.Background(), config.IntegrationID)
	if err != nil {
		return nil, err
	}

	u, ok := i.(storageIntegrations)
	if !ok {
		return nil, errors.New("integration does not implement storageIntegrations")
	}

	return &Write{
		cfg: &config,
		i:   u,
	}, nil
}

func (s *Write) Config() string {
	filtersStr, err := json.Marshal(s.cfg.Fields)
	if err != nil {
		return ""
	}
	return string(filtersStr)
}

func (s *Write) Execute(ctx context.Context, modifiedConfig string) (interface{}, map[string]string, error) {
	logger := logging.FromContext(ctx).With(zap.String("execution_type", s.Type()))
	ctx = logging.WithLogger(ctx, logger)

	var item map[string]interface{}
	if err := json.Unmarshal([]byte(modifiedConfig), &item); err != nil {
		return "", nil, nil
	}
	if item == nil {
		item = make(map[string]interface{})
	}

	_, ok := item["id"]
	if !ok {
		item["id"] = uuid.New().String()
	}
	err := s.i.Store(ctx, item, map[string]string{"collection": s.cfg.Table})
	if err != nil {
		return "", nil, fmt.Errorf("error storing: %w", err)
	}
	return item, nil, nil
}

func init() {
	fields := map[string]actions.FieldInfo{
		"integrationID": {
			Type:        actions.FieldTypeIntegration,
			Label:       "Integration ID",
			Placeholder: "Database integration identifier",
			Required:    true,
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
		"fields": {
			Type:        actions.FieldTypeMap,
			Label:       "Fields",
			Placeholder: "Data fields to store",
			Required:    true,
		},
	}

	if err := actions.RegisterAction("store", actions.ActionRegistrationInfo{
		Name:        "Write Data",
		Description: "Stores data records into database tables with field mapping",
		Fields:      fields,
		Constructor: func(config json.RawMessage) (actions.ActionExecutable, error) {
			var cfg Config
			if err := json.Unmarshal(config, &cfg); err != nil {
				return nil, fmt.Errorf("error creating store action: %v", err)
			}
			return New(cfg)
		},
	}); err != nil {
		panic(err)
	}
}
