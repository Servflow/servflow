package apiconfig

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"git.servflow.io/servflow/definitions/proto"
	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed apiconfig_schema.json
var apiConfigSchema string

type RequestType string

const (
	FileInputTypeRequest = "request"
	FileInputTypeAction  = "action"
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
	ID           string                    `json:"id" yaml:"id"`
	Actions      map[string]Action         `json:"actions" yaml:"actions"`
	Conditionals map[string]Conditional    `json:"conditionals" yaml:"conditionals"`
	Responses    map[string]ResponseConfig `json:"responses" yaml:"responses"`
	HttpConfig   HttpConfig                `json:"http" yaml:"http"`
	McpTool      MCPToolConfig             `json:"mcpTool" yaml:"mcpTool"`
}

type HttpConfig struct {
	ListenPath         string   `json:"listenPath" yaml:"listenPath"`
	Method             string   `json:"method" yaml:"method"`
	Next               string   `json:"next" yaml:"next"`
	CORSAllowedOrigins []string `json:"corsAllowedOrigins" yaml:"corsAllowedOrigins"`
}

type McpConfig struct {
	Tools map[string]MCPToolConfig `json:"tools" yaml:"tools"`
}

type MCPToolConfig struct {
	Enabled     bool               `json:"enabled" yaml:"enabled"`
	Name        string             `json:"name" yaml:"name"`
	Description string             `json:"description" yaml:"description"`
	Args        map[string]ArgType `json:"args" yaml:"args"`
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
	FormValues         []string    `json:"formValues" yaml:"formValues"`
	Next               string      `json:"next" yaml:"next"`
	CORSAllowedOrigins []string    `json:"corsAllowedOrigins" yaml:"corsAllowedOrigins"`
}

type Action struct {
	Type   string                 `json:"type" yaml:"type"`
	Config map[string]interface{} `json:"config" yaml:"config"`
	Next   string                 `json:"next" yaml:"next"`
	Fail   string                 `json:"fail" yaml:"fail"`
}

type Conditional struct {
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
	Code     int            `json:"code" yaml:"code"`
	Template string         `json:"template" yaml:"template"`
	Type     string         `json:"type" yaml:"type"`
	Object   ResponseObject `json:"responseObject" yaml:"responseObject"`
}

type ResponseObject struct {
	Value  string                    `json:"value" yaml:"value"`
	Fields map[string]ResponseObject `json:"fields" yaml:"fields"`
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
	ID        string                 `json:"id" yaml:"id"`
	Config    json.RawMessage        `json:"config" yaml:"-"`
	NewConfig map[string]interface{} `yaml:"config"`
	Type      string                 `json:"type" yaml:"type"`
}

func (d *IntegrationConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var tmp struct {
		Type      string                 `yaml:"type"`
		NewConfig map[string]interface{} `yaml:"config"`
		ID        string                 `yaml:"id"`
	}
	if err := unmarshal(&tmp); err != nil {
		return err
	}

	data, err := json.Marshal(tmp.NewConfig)
	if err != nil {
		return err
	}

	d.Type = tmp.Type
	d.Config = data
	d.ID = tmp.ID
	d.NewConfig = tmp.NewConfig
	return nil
}

func (a *APIConfig) Validate() error {
	if err := a.schemaValidation(); err != nil {
		return err
	}
	if err := a.validateActions(); err != nil {
		return err
	}
	return nil
}

func (a *APIConfig) schemaValidation() error {
	compiler := jsonschema.NewCompiler()

	var schemaData interface{}
	if err := json.Unmarshal(json.RawMessage(apiConfigSchema), &schemaData); err != nil {
		return fmt.Errorf("failed to parse embedded schema: %w", err)
	}

	if err := compiler.AddResource("apiconfig.json", schemaData); err != nil {
		return fmt.Errorf("failed to add schema resource: %w", err)
	}

	schema, err := compiler.Compile("apiconfig.json")
	if err != nil {
		return fmt.Errorf("failed to compile schema: %w", err)
	}

	configJSON, err := json.Marshal(a)
	if err != nil {
		return fmt.Errorf("failed to marshal APIConfig to JSON: %w", err)
	}

	var configData interface{}
	if err := json.Unmarshal(configJSON, &configData); err != nil {
		return fmt.Errorf("failed to unmarshal APIConfig JSON: %w", err)
	}

	if err := schema.Validate(configData); err != nil {
		return fmt.Errorf("APIConfig validation failed: %w", err)
	}

	return nil
}

type ValidationErrors struct {
	errors []error
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

func (ve *ValidationErrors) HasErrors() bool {
	return len(ve.errors) > 0
}

func (a *APIConfig) validateActions() error {
	var validationErrors ValidationErrors
	var invalidActions []string

	for i := range a.Actions {
		action := a.Actions[i]
		if !actions.HasRegisteredActionType(action.Type) {
			invalidActions = append(invalidActions, action.Type)
			continue
		}

		fields, err := actions.GetFieldsForAction(action.Type)
		if err != nil {
			validationErrors.Add(err)
		} else {
			if err := validateFields(fields, action.Config); err != nil {
				validationErrors.Add(err)
			}
		}
	}

	if len(invalidActions) > 0 {
		validationErrors.Add(fmt.Errorf("invalid actions: %s", strings.Join(invalidActions, ", ")))
	}

	if validationErrors.HasErrors() {
		return &validationErrors
	}
	return nil
}

func validateFields(fieldsRequiredMap map[string]actions.FieldInfo, fieldsValues map[string]interface{}) error {
	for k, v := range fieldsRequiredMap {
		if !v.Required {
			continue
		}
		if val, ok := fieldsValues[k]; ok {
			valStr, okStr := val.(string)
			if okStr && valStr == "" {
				return fmt.Errorf("field %s is required", k)
			}
		} else {
			return fmt.Errorf("field %s is required", k)
		}
	}
	return nil
}
