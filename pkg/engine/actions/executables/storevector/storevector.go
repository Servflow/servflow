package storevector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/Servflow/servflow/pkg/engine/integration"
)

type storeVectorIntegration interface {
	integration.Integration
	StoreVectors(vectors []float32, fields map[string]any, options map[string]string) error
}

type Config struct {
	IntegrationID string            `json:"integrationID,omitempty"`
	Fields        map[string]any    `json:"fields"`
	Options       map[string]string `json:"options,omitempty"`
	Vectors       string            `json:"vectors"`
}

type StoreVectors struct {
	cfg                    *Config
	storeVectorIntegration storeVectorIntegration
}

func (s StoreVectors) Type() string {
	return "storevector"
}

func (s StoreVectors) Config() string {
	cfg := *s.cfg
	cfg.Options = nil
	cfg.IntegrationID = ""
	dat, err := json.Marshal(cfg)
	if err != nil {
		return ""
	}
	return string(dat)
}

func (s StoreVectors) Execute(ctx context.Context, modifiedConfig string) (interface{}, error) {
	var newCfg Config
	err := json.Unmarshal([]byte(modifiedConfig), &newCfg)
	if err != nil {
		return nil, err
	}

	var vectors []float32
	err = json.Unmarshal([]byte(newCfg.Vectors), &vectors)
	if err != nil {
		return nil, fmt.Errorf("invalid value for vectors: %w", err)
	}

	err = s.storeVectorIntegration.StoreVectors(vectors, newCfg.Fields, s.cfg.Options)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func New(config Config) (*StoreVectors, error) {
	if config.IntegrationID == "" {
		return nil, fmt.Errorf("no integration ID provided")
	}
	i, err := integration.GetIntegration(config.IntegrationID)
	if err != nil {
		return nil, err
	}

	u, ok := i.(storeVectorIntegration)
	if !ok {
		return nil, errors.New("integration does not implement vector storage")
	}

	return &StoreVectors{
		cfg:                    &config,
		storeVectorIntegration: u,
	}, nil
}

func init() {
	if err := actions.RegisterAction("storevector", func(config json.RawMessage) (actions.ActionExecutable, error) {
		var cfg Config
		if err := json.Unmarshal(config, &cfg); err != nil {
			return nil, fmt.Errorf("error creating storevector action: %v", err)
		}
		return New(cfg)
	}); err != nil {
		panic(err)
	}
}
