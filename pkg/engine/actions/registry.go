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

type ActionRegistration struct {
	Name        string
	Description string
	Fields      map[string]FieldInfo
	Constructor factoryFunc
}

type ActionInfo struct {
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Fields      map[string]FieldInfo `json:"fields"`
}

type FieldInfo struct {
	Type        string `json:"type"`
	Label       string `json:"label"`
	Placeholder string `json:"placeholder"`
	Required    bool   `json:"required"`
	Default     any    `json:"default"`
}

type actionFactory struct {
	constructor factoryFunc
	info        ActionInfo
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

type factoryFunc func(config json.RawMessage) (ActionExecutable, error)

func (r *Registry) RegisterAction(actionType string, registration ActionRegistration) error {
	_, ok := r.availableConstructors[actionType]
	if ok {
		return fmt.Errorf("action type %s already registered", actionType)
	}
	r.availableConstructors[actionType] = actionFactory{
		constructor: registration.Constructor,
		info: ActionInfo{
			Name:        registration.Name,
			Description: registration.Description,
			Fields:      registration.Fields,
		},
	}
	return nil
}

// GetInfoForAction replaces GetFieldsForAction and returns complete action info
func GetInfoForAction(actionType string) (ActionInfo, error) {
	f, ok := actionManager.availableConstructors[actionType]
	if !ok {
		return ActionInfo{}, errors.New("action type " + actionType + " not registered")
	}

	return f.info, nil
}

// GetFieldsForAction kept for backward compatibility
func GetFieldsForAction(actionType string) (map[string]FieldInfo, error) {
	f, ok := actionManager.availableConstructors[actionType]
	if !ok {
		return nil, errors.New("action type " + actionType + " not registered")
	}

	return f.info.Fields, nil
}

func (r *Registry) ReplaceActionType(actionType string, constructor factoryFunc) {
	existing, ok := r.availableConstructors[actionType]
	if !ok {
		r.availableConstructors[actionType] = actionFactory{
			constructor: constructor,
		}
		return
	}
	r.availableConstructors[actionType] = actionFactory{
		constructor: constructor,
		info:        existing.info,
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

// New public functions using ActionRegistration
func RegisterAction(actionType string, registration ActionRegistration) error {
	return actionManager.RegisterAction(actionType, registration)
}

// Legacy function kept for backward compatibility with old signature
func RegisterActionLegacy(actionType string, constructor factoryFunc, fields map[string]FieldInfo) error {
	return actionManager.RegisterAction(actionType, ActionRegistration{
		Name:        actionType, // Use actionType as default name
		Description: "",         // Empty description for legacy
		Fields:      fields,
		Constructor: constructor,
	})
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
