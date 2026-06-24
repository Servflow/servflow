package apiconfig

import (
	_ "embed"
	"errors"
	"fmt"
	"sort"
	"strings"

	"git.servflow.io/servflow/definitions/proto"
	"github.com/Servflow/servflow/pkg/engine/actions"
)

//go:embed apiconfig_schema.json
var apiConfigSchema string

type RequestType string

const (
	FileInputTypeRequest = "request"
	FileInputTypeAction  = "action"
	FileInputTypeStorage = "storage"
)

type FileInput struct {
	Type       string `json:"type" yaml:"type"`
	Identifier string `json:"identifier" yaml:"identifier"`
}

const (
	JSON     RequestType = "json"
	FormData RequestType = "form-data"
)

type APIConfig struct {
	Name         string                       `json:"name" yaml:"name"`
	ID           string                       `json:"id" yaml:"id"`
	Actions      map[string]Action            `json:"actions,omitempty" yaml:"actions,omitempty"`
	Conditionals map[string]Conditional       `json:"conditionals,omitempty" yaml:"conditionals,omitempty"`
	Responses    map[string]ResponseConfig    `json:"responses,omitempty" yaml:"responses,omitempty"`
	HttpConfig   HttpConfig                   `json:"http" yaml:"http"`
	McpTool      MCPToolConfig                `json:"mcpTool" yaml:"mcpTool"`
	Integrations map[string]IntegrationConfig `json:"integrations,omitempty" yaml:"integrations,omitempty"`
}

func (a *APIConfig) IsMCPConfig() bool {
	return a.McpTool.Enabled || a.McpTool.Name != ""
}

type HttpConfig struct {
	ListenPath         string   `json:"listenPath" yaml:"listenPath"`
	Method             string   `json:"method" yaml:"method"`
	Next               string   `json:"next" yaml:"next"`
	CORSAllowedOrigins []string `json:"corsAllowedOrigins,omitempty" yaml:"corsAllowedOrigins,omitempty"`
}

type McpConfig struct {
	Tools map[string]MCPToolConfig `json:"tools,omitempty" yaml:"tools,omitempty"`
}

type MCPToolConfig struct {
	Enabled     bool               `json:"enabled" yaml:"enabled"`
	Name        string             `json:"name" yaml:"name"`
	Description string             `json:"description" yaml:"description"`
	Args        map[string]ArgType `json:"args,omitempty" yaml:"args,omitempty"`
	// Result is the expression to be used to get the result
	Result string `json:"result" yaml:"result"`
	Start  string `json:"start" yaml:"start"`
}

type ArgType struct {
	Name string `json:"name" yaml:"name"`
	Type string `json:"type" yaml:"type"`
}

type RequestConfig struct {
	Type               RequestType `json:"type" yaml:"type"`
	Schema             string      `json:"schema" yaml:"schema"`
	FormValues         []string    `json:"formValues,omitempty" yaml:"formValues,omitempty"`
	Next               string      `json:"next" yaml:"next"`
	CORSAllowedOrigins []string    `json:"corsAllowedOrigins,omitempty" yaml:"corsAllowedOrigins,omitempty"`
}

type Action struct {
	Name       string                 `json:"name,omitempty" yaml:"name,omitempty"`
	Type       string                 `json:"type" yaml:"type"`
	Config     map[string]interface{} `json:"config,omitempty" yaml:"config,omitempty"`
	Next       string                 `json:"next" yaml:"next"`
	Fail       string                 `json:"fail" yaml:"fail"`
	UseReplica bool                   `json:"useReplica,omitempty" yaml:"useReplica,omitempty"`
	Dispatch   []string               `json:"dispatch,omitempty" yaml:"dispatch,omitempty"`
}

type Conditional struct {
	Name       string            `json:"name,omitempty" yaml:"name,omitempty"`
	OnTrue     string            `json:"onTrue" yaml:"onTrue"`
	OnFalse    string            `json:"onFalse" yaml:"onFalse"`
	Expression string            `json:"expression" yaml:"expression"`
	Type       string            `json:"type,omitempty" yaml:"type,omitempty"`
	Structure  [][]ConditionItem `json:"structure,omitempty" yaml:"structure,omitempty"`
}

type ConditionItem struct {
	Content    string `json:"content" yaml:"content"`
	Comparison string `json:"comparison,omitempty" yaml:"comparison,omitempty"`
	Function   string `json:"function" yaml:"function"`
	Title      string `json:"title" yaml:"title"`
}

type ResponseConfig struct {
	Name     string         `json:"name,omitempty" yaml:"name,omitempty"`
	Code     int            `json:"code" yaml:"code"`
	Template string         `json:"template" yaml:"template"`
	Type     string         `json:"type" yaml:"type"`
	Object   ResponseObject `json:"responseObject" yaml:"responseObject"`
	File     FileInput      `json:"file" yaml:"file"`
}

type ResponseObject struct {
	Value  string                    `json:"value" yaml:"value"`
	Fields map[string]ResponseObject `json:"fields,omitempty" yaml:"fields,omitempty"`
}

func (o *ResponseObject) ToProto() *proto.ResponseObject {
	resp := proto.ResponseObject{
		Value:  o.Value,
		Fields: make(map[string]*proto.ResponseObject),
	}

	for k, v := range o.Fields {
		resp.Fields[k] = v.ToProto()
	}

	return &resp
}

type IntegrationConfig struct {
	ID       string                 `json:"id" yaml:"id"`
	Config   map[string]interface{} `json:"config,omitempty" yaml:"config,omitempty"`
	Type     string                 `json:"type" yaml:"type"`
	LazyLoad bool                   `json:"lazyLoad" yaml:"lazyLoad"`
}

//	func (d *IntegrationConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
//		var tmp struct {
//			Type      string                 `yaml:"type"`
//			NewConfig map[string]interface{} `yaml:"config"`
//			ID        string                 `yaml:"id"`
//		}
//		if err := unmarshal(&tmp); err != nil {
//			return err
//		}
//
//		data, err := json.Marshal(tmp.NewConfig)
//		if err != nil {
//			return err
//		}
//
//		d.Type = tmp.Type
//		d.Config = data
//		d.ID = tmp.ID
//		d.NewConfig = tmp.NewConfig
//		return nil
//	}
func (a *APIConfig) Validate() error {
	return a.ValidateWithEntries()
}

// ValidateWithEntries validates the config, treating each extra root as an
// additional entry point into the step graph (used by the pro layer to feed a
// trigger's start step, which the engine config has no other knowledge of).
func (a *APIConfig) ValidateWithEntries(extraRoots ...string) error {
	var validationErrors ValidationErrors

	a.collectSchemaErrors(&validationErrors)
	a.collectActionErrors(&validationErrors)
	a.collectGraphErrors(&validationErrors, extraRoots)

	if validationErrors.HasErrors() {
		return &validationErrors
	}
	return nil
}

type ActionConfigError struct {
	ActionID string
	// Field is the offending config field, when the error is field-specific.
	Field   string
	Message string
}

func (e *ActionConfigError) Error() string {
	return fmt.Sprintf("action '%s': %s", e.ActionID, e.Message)
}

type SchemaValidationError struct {
	// Path is the instance location of the offending value (JSON-pointer style).
	Path string
	// Keyword is the failing schema keyword (e.g. "required", "enum", "type").
	Keyword string
	// Message is the humanized, domain-aware message.
	Message string
}

func (e *SchemaValidationError) Error() string {
	return e.Message
}

type ValidationErrors struct {
	errors   []error
	warnings []error
}

func (ve *ValidationErrors) Error() string {
	if len(ve.errors) == 0 {
		return ""
	}
	var lines []string
	for _, err := range ve.errors {
		lines = append(lines, err.Error())
	}
	return strings.Join(lines, "\n")
}

func (ve *ValidationErrors) Add(err error) {
	ve.errors = append(ve.errors, err)
}

// AddWarning records a non-fatal finding. Warnings do not count towards
// HasErrors, so a config with only warnings still validates.
func (ve *ValidationErrors) AddWarning(err error) {
	ve.warnings = append(ve.warnings, err)
}

// Warnings returns the non-fatal findings collected during validation.
func (ve *ValidationErrors) Warnings() []error {
	return ve.warnings
}

func (ve *ValidationErrors) HasErrors() bool {
	return len(ve.errors) > 0
}

func (ve *ValidationErrors) GetActionConfigErrors() []*ActionConfigError {
	var actionErrors []*ActionConfigError
	for _, err := range ve.errors {
		var actionErr *ActionConfigError
		if errors.As(err, &actionErr) {
			actionErrors = append(actionErrors, actionErr)
		}
	}
	return actionErrors
}

func (ve *ValidationErrors) GetSchemaValidationErrors() []*SchemaValidationError {
	var schemaErrors []*SchemaValidationError
	for _, err := range ve.errors {
		var schemaErr *SchemaValidationError
		if errors.As(err, &schemaErr) {
			schemaErrors = append(schemaErrors, schemaErr)
		}
	}
	return schemaErrors
}

func (a *APIConfig) collectActionErrors(validationErrors *ValidationErrors) {
	for actionID, action := range a.Actions {
		if !actions.HasRegisteredActionType(action.Type) {
			validationErrors.Add(&ActionConfigError{
				ActionID: actionID,
				Message:  fmt.Sprintf("invalid action type: %s", action.Type),
			})
			continue
		}

		fields, err := actions.GetFieldsForAction(action.Type)
		if err != nil {
			validationErrors.Add(&ActionConfigError{
				ActionID: actionID,
				Message:  err.Error(),
			})
			continue
		}
		for _, field := range missingRequiredFields(fields, action.Config) {
			validationErrors.Add(&ActionConfigError{
				ActionID: actionID,
				Field:    field,
				Message:  fmt.Sprintf("missing required field %q", field),
			})
		}
	}
}

// missingRequiredFields returns the names of required fields that are absent or
// empty in the action config. We only check presence, not type: a field holding
// a Go template (e.g. "{{ body \"x\" }}") is a non-empty string and so counts as
// provided. Field names are sorted for stable, deterministic error output.
func missingRequiredFields(fields map[string]actions.FieldInfo, values map[string]interface{}) []string {
	var missing []string
	for name, info := range fields {
		if !info.Required {
			continue
		}
		if isEmptyFieldValue(values[name]) {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	return missing
}

// isEmptyFieldValue reports whether a config value should count as "not
// provided". A missing key (nil) and an empty string are empty; any other
// present value counts as provided.
func isEmptyFieldValue(val interface{}) bool {
	if val == nil {
		return true
	}
	if s, ok := val.(string); ok {
		return s == ""
	}
	return false
}
