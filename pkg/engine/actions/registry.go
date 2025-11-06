package actions

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

type Registry struct {
	actions               sync.Map
	availableConstructors map[string]actionFactory
}

type actionFactory struct {
	constructor factoryFunc
	// fieldsMap is used to get the list of fields, the values represent if they are required
	fieldsMap map[string]FieldInfo
}

func NewRegistry() *Registry {
	return &Registry{
		actions:               sync.Map{},
		availableConstructors: make(map[string]actionFactory),
	}
}

var actionManager = &Registry{
	availableConstructors: make(map[string]actionFactory),
	actions:               sync.Map{},
}

type FieldInfo struct {
	Type        string `json:"type"`
	Label       string `json:"label"`
	Placeholder string `json:"placeholder"`
	Required    bool   `json:"required"`
	Default     any    `json:"default"`
}

type factoryFunc func(config json.RawMessage) (ActionExecutable, error)

func (r *Registry) RegisterAction(actionType string, constructor factoryFunc, fields map[string]FieldInfo) error {
	_, ok := r.availableConstructors[actionType]
	if ok {
		return fmt.Errorf("action type %s already registered", actionType)
	}
	r.availableConstructors[actionType] = actionFactory{
		constructor: constructor,
		fieldsMap:   fields,
	}
	return nil
}

func GetFieldsForAction(actionType string) (map[string]FieldInfo, error) {
	f, ok := actionManager.availableConstructors[actionType]
	if !ok {
		return nil, errors.New("action type " + actionType + " not registered")
	}

	return f.fieldsMap, nil
}

func (r *Registry) ReplaceActionType(actionType string, constructor factoryFunc) {
	existing, ok := r.availableConstructors[actionType]
	if !ok {
		return
	}
	r.availableConstructors[actionType] = actionFactory{
		constructor: constructor,
		fieldsMap:   existing.fieldsMap,
	}
}

func (r *Registry) GetActionExecutable(actionType string, config json.RawMessage) (ActionExecutable, error) {
	constructor, ok := r.availableConstructors[actionType]
	if !ok {
		return nil, fmt.Errorf("action type %s not registered", actionType)
	}

	executable, err := constructor.constructor(config)
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

func RegisterAction(actionType string, constructor factoryFunc, fields map[string]FieldInfo) error {
	return actionManager.RegisterAction(actionType, constructor, fields)
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
