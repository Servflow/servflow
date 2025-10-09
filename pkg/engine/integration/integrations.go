package integration

import (
	"fmt"
	"sync"
)

type Manager struct {
	integrations          sync.Map
	availableConstructors map[string]factoryFunc
}

var integrationManager = &Manager{
	availableConstructors: make(map[string]factoryFunc),
	integrations:          sync.Map{},
}

type Integration interface {
	Type() string
}

type factoryFunc func(map[string]any) (Integration, error)

func RegisterIntegration(integrationType string, constructor factoryFunc) error {
	_, ok := integrationManager.availableConstructors[integrationType]
	if ok {
		return fmt.Errorf("integration type %s already registered", integrationType)
	}
	integrationManager.availableConstructors[integrationType] = constructor
	return nil
}

func ReplaceIntegrationType(integrationType string, constructor factoryFunc) {
	integrationManager.availableConstructors[integrationType] = constructor
}

func InitializeIntegration(integrationType, id string, config map[string]any) error {
	constructor, ok := integrationManager.availableConstructors[integrationType]
	if !ok {
		return fmt.Errorf("integration type %s not registered", integrationType)
	}

	integration, err := constructor(config)
	if err != nil {
		return err
	}

	integrationManager.integrations.Store(id, integration)
	return nil
}

func GetIntegration(id string) (Integration, error) {
	integration, ok := integrationManager.integrations.Load(id)
	if !ok {
		return nil, fmt.Errorf("integration %s not found", id)
	}
	return integration.(Integration), nil
}
