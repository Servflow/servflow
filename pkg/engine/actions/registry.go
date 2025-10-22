package actions

import (
	"encoding/json"
	"fmt"
	"sync"
)

type Registry struct {
	actions               sync.Map
	availableConstructors map[string]factoryFunc
}

func NewRegistry() *Registry {
	return &Registry{
		actions:               sync.Map{},
		availableConstructors: make(map[string]factoryFunc),
	}
}

var actionManager = &Registry{
	availableConstructors: make(map[string]factoryFunc),
	actions:               sync.Map{},
}

type factoryFunc func(config json.RawMessage) (ActionExecutable, error)

func (r *Registry) RegisterAction(actionType string, constructor factoryFunc) error {
	_, ok := r.availableConstructors[actionType]
	if ok {
		return fmt.Errorf("action type %s already registered", actionType)
	}
	r.availableConstructors[actionType] = constructor
	return nil
}

func (r *Registry) ReplaceActionType(actionType string, constructor factoryFunc) {
	r.availableConstructors[actionType] = constructor
}

func (r *Registry) GetActionExecutable(actionType string, config json.RawMessage) (ActionExecutable, error) {
	constructor, ok := r.availableConstructors[actionType]
	if !ok {
		return nil, fmt.Errorf("action type %s not registered", actionType)
	}

	executable, err := constructor(config)
	if err != nil {
		return nil, err
	}

	return executable, nil
}

func (r *Registry) GetRegisteredActionTypes() []string {
	types := make([]string, 0, len(r.availableConstructors))
	for actionType := range r.availableConstructors {
		types = append(types, actionType)
	}
	return types
}

func RegisterAction(actionType string, constructor factoryFunc) error {
	return actionManager.RegisterAction(actionType, constructor)
}

func ReplaceActionType(actionType string, constructor factoryFunc) {
	actionManager.ReplaceActionType(actionType, constructor)
}

func GetActionExecutable(actionType string, config json.RawMessage) (ActionExecutable, error) {
	return actionManager.GetActionExecutable(actionType, config)
}

func GetRegisteredActionTypes() []string {
	return actionManager.GetRegisteredActionTypes()
}

func HasRegisteredActionType(actionType string) bool {
	_, ok := actionManager.availableConstructors[actionType]
	return ok
}
