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
	Name          string               `json:"name"`
	Description   string               `json:"description"`
	Fields        map[string]FieldInfo `json:"fields"`
	Constructor   factoryFunc          `json:"-"`
	ConstructorV2 factoryFuncV2        `json:"-"` // V2 constructor (used when UseV2 is true)
	UseV2         bool                 `json:"-"` // If true, use V2 interface (action handles own template resolution)
}

type FieldType string

// TODO fix support for passing config over replica

const (
	FieldTypeString      FieldType = "string"
	FieldTypeIntegration FieldType = "integration"
	FieldTypeMap         FieldType = "map"
	FieldTypeBoolean     FieldType = "boolean"
	FieldTypeFile        FieldType = "file"
	FieldTypeTextArea    FieldType = "text_area"
	FieldTypeArray       FieldType = "array"
)

type FieldInfo struct {
	Type        FieldType         `json:"type"`
	Label       string            `json:"label"`
	Placeholder string            `json:"placeholder"`
	Required    bool              `json:"required"`
	Default     any               `json:"default"`
	Values      []string          `json:"values"`
	Metadata    map[string]string `json:"metadata"`
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
type factoryFuncV2 func(config json.RawMessage) (ActionExecutableV2, error)

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

func GetInfoForAction(actionType string) (ActionRegistrationInfo, error) {
	f, ok := actionManager.availableConstructors[actionType]
	if !ok {
		return ActionRegistrationInfo{}, errors.New("action type " + actionType + " not registered")
	}

	return f, nil
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

// GetActionExecutableV2 returns a V2 action executable for the given type.
// Returns an error if the action type is not registered or doesn't use V2.
func (r *Registry) GetActionExecutableV2(actionType string, config json.RawMessage) (ActionExecutableV2, error) {
	registration, ok := r.availableConstructors[actionType]
	if !ok {
		return nil, fmt.Errorf("action type %s not registered", actionType)
	}
	if !registration.UseV2 {
		return nil, fmt.Errorf("action type %s is not a V2 action", actionType)
	}
	if registration.ConstructorV2 == nil {
		return nil, fmt.Errorf("action type %s has UseV2=true but no ConstructorV2", actionType)
	}
	return registration.ConstructorV2(config)
}

// IsV2Action returns true if the action type uses the V2 interface.
func (r *Registry) IsV2Action(actionType string) bool {
	registration, ok := r.availableConstructors[actionType]
	if !ok {
		return false
	}
	return registration.UseV2
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

// IsV2Action returns true if the action type uses the V2 interface.
func IsV2Action(actionType string) bool {
	return actionManager.IsV2Action(actionType)
}

// GetActionExecutableV2 returns a V2 action executable for the given type.
func GetActionExecutableV2(actionType string, config json.RawMessage) (ActionExecutableV2, error) {
	return actionManager.GetActionExecutableV2(actionType, config)
}
