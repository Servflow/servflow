//go:generate mockgen -source save.go -destination save_mock.go -package save
package save

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
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type saveIntegration interface {
	integration.Integration
	Store(ctx context.Context, data map[string]interface{}, options map[string]string) error
	Update(ctx context.Context, data map[string]interface{}, options map[string]string, filter ...filters.Filter) (string, error)
}

type Config struct {
	IntegrationID     string                 `json:"integrationID"`
	Table             string                 `json:"table"`
	DatasourceOptions map[string]string      `json:"datasourceOptions"`
	Fields            map[string]interface{} `json:"fields"`
	Filters           []filters.Filter       `json:"filters"`
}

type Save struct {
	cfg *Config
	i   saveIntegration
}

func (s *Save) Type() string {
	return "save"
}

func (s *Save) SupportsReplica() bool {
	return true
}

func New(config Config) (*Save, error) {
	if config.IntegrationID == "" {
		return nil, errors.New("integrationID is required")
	}
	if config.Table == "" {
		return nil, errors.New("table is required")
	}

	i, err := integration.GetIntegration(context.Background(), config.IntegrationID)
	if err != nil {
		return nil, err
	}

	si, ok := i.(saveIntegration)
	if !ok {
		return nil, errors.New("integration does not support save operations (must implement Store and Update)")
	}

	return &Save{
		cfg: &config,
		i:   si,
	}, nil
}

func (s *Save) Execute(ctx context.Context) (interface{}, map[string]string, error) {
	logger := logging.FromContext(ctx).With(zap.String("execution_type", s.Type()))
	ctx = logging.WithLogger(ctx, logger)

	rc, err := requestctx.FromContextOrError(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get request context: %w", err)
	}

	// Resolve templates in fields
	resolvedFields, err := s.resolveFields(ctx, rc, s.cfg.Fields)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resolve fields: %w", err)
	}

	options := map[string]string{"collection": s.cfg.Table}

	// If no filters, this is an INSERT operation
	if len(s.cfg.Filters) == 0 {
		return s.executeInsert(ctx, rc, resolvedFields, options)
	}

	// With filters, this is an UPDATE operation
	return s.executeUpdate(ctx, rc, resolvedFields, options)
}

func (s *Save) executeInsert(ctx context.Context, rc *requestctx.RequestContext, fields map[string]interface{}, options map[string]string) (interface{}, map[string]string, error) {
	logger := logging.FromContext(ctx)

	// Generate ID if not provided
	id, ok := fields["id"]
	if !ok || id == "" {
		fields["id"] = uuid.New().String()
		id = fields["id"]
	}

	logger.Debug("save action executing insert", zap.Any("id", id))

	err := s.i.Store(ctx, fields, options)
	if err != nil {
		return nil, nil, fmt.Errorf("error storing: %w", err)
	}

	return map[string]interface{}{"id": id}, nil, nil
}

func (s *Save) executeUpdate(ctx context.Context, rc *requestctx.RequestContext, fields map[string]interface{}, options map[string]string) (interface{}, map[string]string, error) {
	logger := logging.FromContext(ctx)

	// Resolve templates in filters
	resolvedFilters, err := s.resolveFilters(ctx, rc, s.cfg.Filters)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resolve filters: %w", err)
	}

	logger.Debug("save action executing update", zap.Int("filter_count", len(resolvedFilters)))

	id, err := s.i.Update(ctx, fields, options, resolvedFilters...)
	if err != nil {
		return nil, nil, fmt.Errorf("error updating: %w", err)
	}

	return map[string]interface{}{"id": id}, nil, nil
}

func (s *Save) resolveFields(ctx context.Context, rc *requestctx.RequestContext, fields map[string]interface{}) (map[string]interface{}, error) {
	resolved := make(map[string]interface{}, len(fields))

	for key, value := range fields {
		switch v := value.(type) {
		case string:
			resolvedValue, err := rc.Resolve(ctx, v)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve field %s: %w", key, err)
			}
			resolved[key] = resolvedValue
		default:
			resolved[key] = value
		}
	}

	return resolved, nil
}

func (s *Save) resolveFilters(ctx context.Context, rc *requestctx.RequestContext, filtersList []filters.Filter) ([]filters.Filter, error) {
	resolved := make([]filters.Filter, len(filtersList))

	for i, f := range filtersList {
		resolved[i] = filters.Filter{
			Field:     f.Field,
			Operation: f.Operation,
		}

		// Resolve comparator if it's a string (could be a template)
		switch v := f.Comparator.(type) {
		case string:
			resolvedValue, err := rc.Resolve(ctx, v)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve filter comparator for field %s: %w", f.Field, err)
			}
			resolved[i].Comparator = resolvedValue
		default:
			resolved[i].Comparator = f.Comparator
		}
	}

	return resolved, nil
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
			Placeholder: "Data fields to save",
			Required:    true,
		},
		"filters": {
			Type:        actions.FieldTypeMap,
			Label:       "Filters",
			Placeholder: "Query filters to identify records to update (leave empty to insert)",
			Required:    false,
			Metadata: map[string]string{
				"type": "filter",
			},
		},
	}

	if err := actions.RegisterAction("save", actions.ActionRegistrationInfo{
		Name:        "Save Data",
		Description: "Inserts new records or updates existing records in database tables. When filters are provided, updates matching records; otherwise inserts a new record.",
		Fields:      fields,
		UseV2:       true,
		ConstructorV2: func(config json.RawMessage) (actions.ActionExecutableV2, error) {
			var cfg Config
			if err := json.Unmarshal(config, &cfg); err != nil {
				return nil, fmt.Errorf("error creating save action: %v", err)
			}
			return New(cfg)
		},
	}); err != nil {
		panic(err)
	}
}
