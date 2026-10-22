package mongoquery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/Servflow/servflow/pkg/engine/integration"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
)

// Operations the action runs. Every one of them reads.
const (
	OperationFind            = "find"
	OperationAggregate       = "aggregate"
	OperationListCollections = "listCollections"
)

// refusedKeys are the operators a query or pipeline may not hold, at any depth,
// and why. The first two write; the rest run JavaScript on the server.
var refusedKeys = map[string]string{
	"$out":         "writes data",
	"$merge":       "writes data",
	"$where":       "runs JavaScript",
	"$function":    "runs JavaScript",
	"$accumulator": "runs JavaScript",
}

type Config struct {
	Operation   string `json:"operation" yaml:"operation"`
	Collection  string `json:"collection" yaml:"collection"`
	FilterQuery string `json:"filterQuery" yaml:"filterQuery"`
	Projection  string `json:"projection" yaml:"projection"`
	Pipeline    string `json:"pipeline" yaml:"pipeline"`
	Integration string `json:"integration" yaml:"integration"`
	FailIfEmpty bool   `json:"failIfEmpty" yaml:"failIfEmpty"`
}

type mongoDBIntegration interface {
	integration.Integration
	ExecuteQuery(ctx context.Context, collection string, filterQuery string, projectionQuery string) ([]map[string]interface{}, error)
	Aggregate(ctx context.Context, collection string, pipeline string) ([]map[string]interface{}, error)
	ListCollections(ctx context.Context) ([]string, error)
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
	// A templated operation or collection is only known once it is rendered,
	// so Execute checks it then.
	if !strings.Contains(config.Operation+config.Collection, "{{") {
		if err := validate(config); err != nil {
			return nil, err
		}
	}

	return &MGOQuery{config: config}, nil
}

// validate checks that the operation exists and has what it reads.
func validate(cfg Config) error {
	switch cfg.Operation {
	case "", OperationFind:
		if cfg.Collection == "" {
			return errors.New("collection is required")
		}
	case OperationAggregate:
		if cfg.Collection == "" {
			return errors.New("collection is required")
		}
		if cfg.Pipeline == "" {
			return errors.New("pipeline is required for aggregate")
		}
	case OperationListCollections:
	default:
		return fmt.Errorf("unknown operation %q, expected %q, %q or %q", cfg.Operation, OperationFind, OperationAggregate, OperationListCollections)
	}
	return nil
}

// checkReadOnly refuses a query, projection or pipeline that holds a refused
// operator anywhere in it, including inside the sub-pipelines of $lookup,
// $facet and $unionWith.
func checkReadOnly(name string, query string) error {
	if query == "" {
		return nil
	}
	var v any
	if err := json.Unmarshal([]byte(query), &v); err != nil {
		return fmt.Errorf("%w: %s is not valid JSON: %v", actions.ErrFailure, name, err)
	}
	return walk(v)
}

func walk(v any) error {
	switch t := v.(type) {
	case map[string]any:
		for key, value := range t {
			if reason, ok := refusedKeys[key]; ok {
				return fmt.Errorf("%w: refused: %s %s", actions.ErrFailure, key, reason)
			}
			if err := walk(value); err != nil {
				return err
			}
		}
	case []any:
		for _, value := range t {
			if err := walk(value); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *MGOQuery) Execute(ctx context.Context, modifiedConfig string) (interface{}, map[string]string, error) {
	var cfg Config
	if err := json.Unmarshal([]byte(modifiedConfig), &cfg); err != nil {
		return nil, nil, err
	}
	// cfg is this call's rendered config. m.config keeps the templates, which
	// Config returns so the next call renders them again.

	if err := validate(cfg); err != nil {
		return nil, nil, fmt.Errorf("%w: %v", actions.ErrFailure, err)
	}
	for _, q := range []struct{ name, query string }{
		{"filterQuery", cfg.FilterQuery},
		{"projection", cfg.Projection},
		{"pipeline", cfg.Pipeline},
	} {
		if err := checkReadOnly(q.name, q.query); err != nil {
			return nil, nil, err
		}
	}

	i, err := requestctx.GetIntegration(ctx, cfg.Integration)
	if err != nil {
		return nil, nil, err
	}
	impl, ok := i.(mongoDBIntegration)
	if !ok {
		return nil, nil, errors.New("integration does not implement mongoDBIntegration")
	}

	var (
		result any
		count  int
	)
	switch cfg.Operation {
	case OperationListCollections:
		names, err := impl.ListCollections(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("error executing integration: %v", err)
		}
		result, count = names, len(names)
	case OperationAggregate:
		docs, err := impl.Aggregate(ctx, cfg.Collection, cfg.Pipeline)
		if err != nil {
			return nil, nil, fmt.Errorf("error executing integration: %v", err)
		}
		result, count = docs, len(docs)
	default:
		docs, err := impl.ExecuteQuery(ctx, cfg.Collection, cfg.FilterQuery, cfg.Projection)
		if err != nil {
			return nil, nil, fmt.Errorf("error executing integration: %v", err)
		}
		result, count = docs, len(docs)
	}

	if count == 0 && cfg.FailIfEmpty {
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
		"operation": {
			Type:        actions.FieldTypeString,
			Label:       "Operation",
			Description: "find returns matching documents, aggregate runs a pipeline, listCollections returns the collection names. Defaults to find",
			Required:    false,
			Default:     OperationFind,
			Values:      []string{OperationFind, OperationAggregate, OperationListCollections},
		},
		"collection": {
			Type:        actions.FieldTypeString,
			Label:       "Collection",
			Description: "MongoDB collection name. Required for find and aggregate",
			Required:    false,
		},
		"filterQuery": {
			Type:        actions.FieldTypeString,
			Label:       "Filter Query",
			Description: "MongoDB filter query, as extended JSON, for find",
			Required:    false,
		},
		"projection": {
			Type:        actions.FieldTypeString,
			Label:       "Projection",
			Description: "MongoDB projection query, as extended JSON, for find",
			Required:    false,
		},
		"pipeline": {
			Type:        actions.FieldTypeTextArea,
			Label:       "Pipeline",
			Description: "Aggregation pipeline, as an extended JSON array of stages, for aggregate. $lookup and $unionWith read other collections",
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
		Description: "Reads a MongoDB database: finds documents, runs aggregation pipelines across collections, or lists the collections. Never writes: $out, $merge and server-side JavaScript are refused",
		Fields:      fields,
		Output: actions.OutputInfo{
			Kind:        actions.OutputDynamic,
			Description: "For find and aggregate, a list of the resulting documents with the collection's own field names. For listCollections, a sorted list of collection names.",
		},
		New: func(_ context.Context, config json.RawMessage) (actions.ActionExecutable, error) {
			var cfg Config
			if err := json.Unmarshal(config, &cfg); err != nil {
				return nil, fmt.Errorf("error creating mongoquery action: %v", err)
			}
			return New(cfg)
		},
	}
}
