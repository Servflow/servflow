package actions

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

type Registry struct {
	actions               sync.Map
	availableConstructors map[string]ActionRegistrationInfo
}

type ActionRegistrationInfo struct {
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Fields      map[string]FieldInfo `json:"fields"`
	Constructor factoryFunc          `json:"-"`
}

type FieldType string

const (
	FieldTypeString      FieldType = "string"
	FieldTypeIntegration FieldType = "integration"
	FieldTypeMap         FieldType = "map"
	FieldTypeBoolean     FieldType = "boolean"
)

type FieldInfo struct {
	Type        FieldType `json:"type"`
	Label       string    `json:"label"`
	Placeholder string    `json:"placeholder"`
	Required    bool      `json:"required"`
	Default     any       `json:"default"`
	Values      []string  `json:"values"`
}

func NewRegistry() *Registry {
	return &Registry{
		actions:               sync.Map{},
		availableConstructors: make(map[string]ActionRegistrationInfo),
	}
}

var actionManager = &Registry{
	availableConstructors: make(map[string]ActionRegistrationInfo),
	actions:               sync.Map{},
}

type factoryFunc func(config json.RawMessage) (ActionExecutable, error)

func (r *Registry) RegisterAction(actionType string, registration ActionRegistrationInfo) error {
	_, ok := r.availableConstructors[actionType]
	if ok {
		return fmt.Errorf("action type %s already registered", actionType)
	}
	r.availableConstructors[actionType] = registration
	return nil
}

// GetFieldsForAction kept for backward compatibility
func GetFieldsForAction(actionType string) (map[string]FieldInfo, error) {
	f, ok := actionManager.availableConstructors[actionType]
	if !ok {
		return nil, errors.New("action type " + actionType + " not registered")
	}

	return f.Fields, nil
}

func (r *Registry) ReplaceActionType(actionType string, constructor factoryFunc) {
	existing, ok := r.availableConstructors[actionType]
	if !ok {
		r.availableConstructors[actionType] = ActionRegistrationInfo{
			Constructor: constructor,
		}
		return
	}
	r.availableConstructors[actionType] = ActionRegistrationInfo{
		Constructor: constructor,
		Name:        existing.Name,
		Description: existing.Description,
		Fields:      existing.Fields,
	}
}

func (r *Registry) GetActionExecutable(actionType string, config json.RawMessage) (ActionExecutable, error) {
	constructor, ok := r.availableConstructors[actionType]
	if !ok {
		return nil, fmt.Errorf("action type %s not registered", actionType)
	}

	executable, err := constructor.Constructor(config)
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

func RegisterAction(actionType string, registration ActionRegistrationInfo) error {
	return actionManager.RegisterAction(actionType, registration)
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
