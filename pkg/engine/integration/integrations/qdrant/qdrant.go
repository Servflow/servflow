package qdrant

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/Servflow/servflow/internal/util"
	"github.com/Servflow/servflow/pkg/engine/integration"
	uuid "github.com/google/uuid"

	"github.com/qdrant/go-client/qdrant"
)

var (
	optionCollectionName = "collection"
)

type Config struct {
	Host string
	Port int
}

type Wrapper struct {
	client *qdrant.Client
}

func (w *Wrapper) Type() string {
	return "qdrant"
}

func parseHostPort(url string) (string, int) {
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")
	parts := strings.Split(url, ":")
	port, err := strconv.Atoi(parts[1])
	if err != nil {
		return url, -1
	}
	return parts[0], port
}

func init() {
	fields := map[string]integration.FieldInfo{
		"url": {
			Type:        integration.FieldTypeString,
			Label:       "URL",
			Placeholder: "localhost:6334",
			Required:    true,
		},
	}

	if err := integration.RegisterIntegration("qdrant", integration.IntegrationRegistrationInfo{
		Name:        "Qdrant",
		Description: "Qdrant vector database for similarity search and vector storage",
		Fields:      fields,
		Constructor: func(m map[string]any) (integration.Integration, error) {
			host, port := parseHostPort(m["url"].(string))
			return New(&Config{
				Host: host,
				Port: port,
			})
		},
	}); err != nil {
		panic(err)
	}
}

func New(cfg *Config) (*Wrapper, error) {
	client, err := qdrant.NewClient(&qdrant.Config{
		Host: cfg.Host,
		Port: cfg.Port,
	})
	if err != nil {
		return nil, err
	}

	return &Wrapper{client: client}, nil
}

func toUint64Ptr(val interface{}) *uint64 {
	switch v := val.(type) {
	case uint64:
		return &v
	case uint:
		u := uint64(v)
		return &u
	case int:
		if v < 0 {
			return nil
		}
		u := uint64(v)
		return &u
	case int8:
		if v < 0 {
			return nil
		}
		u := uint64(v)
		return &u
	case int16:
		if v < 0 {
			return nil
		}
		u := uint64(v)
		return &u
	case int32:
		if v < 0 {
			return nil
		}
		u := uint64(v)
		return &u
	case int64:
		if v < 0 {
			return nil
		}
		u := uint64(v)
		return &u
	case float32:
		if v < 0 {
			return nil
		}
		u := uint64(v)
		return &u
	case float64:
		if v < 0 {
			return nil
		}
		u := uint64(v)
		return &u
	default:
		return nil
	}
}

func (w *Wrapper) FetchVector(inputVector []float32, options map[string]any) (results []map[string]any, err error) {
	collectionName, ok := options[optionCollectionName]
	if !ok {
		return nil, fmt.Errorf("missing required option %s", optionCollectionName)
	}

	var l *uint64
	limit, ok := options["limit"]
	if ok {
		l = toUint64Ptr(limit)
	}

	searchResult, err := w.client.Query(context.Background(), &qdrant.QueryPoints{
		CollectionName: collectionName.(string),
		Query:          qdrant.NewQuery(inputVector...),
		WithPayload:    qdrant.NewWithPayloadEnable(true),
		Limit:          l,
	})

	// convert from ScoredPoints to Vector result
	resultVectors := util.MapSlice(searchResult, func(t *qdrant.ScoredPoint) map[string]any {
		return util.MapMap(t.Payload, func(t *qdrant.Value) any {
			switch v := t.Kind.(type) {
			case *qdrant.Value_BoolValue:
				return v.BoolValue
			case *qdrant.Value_IntegerValue:
				return v.IntegerValue
			case *qdrant.Value_StringValue:
				return v.StringValue
			case *qdrant.Value_DoubleValue:
				return v.DoubleValue
			case *qdrant.Value_NullValue:
				return nil
			default:
				return nil
			}
		})
	})

	return resultVectors, err
}

func (w *Wrapper) StoreVectors(vectors []float32, metadata map[string]any, options map[string]string) error {
	collectionName, ok := options[optionCollectionName]
	if !ok {
		return fmt.Errorf("missing required option %s", optionCollectionName)
	}

	id := uuid.New().String()

	_, err := w.client.Upsert(context.Background(), &qdrant.UpsertPoints{
		CollectionName: collectionName,
		Points: []*qdrant.PointStruct{
			{
				Id:      qdrant.NewID(id),
				Vectors: qdrant.NewVectors(vectors...),
				Payload: qdrant.NewValueMap(metadata),
			},
		},
	})

	if err != nil {
		return err
	}
	return nil
}
