package apiconfig

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"git.servflow.io/servflow/definitions/proto"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed apiconfig_schema.json
var apiConfigSchema string

type RequestType string

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

type ConditionalExpressions struct {
	Value   string `json:"value" yaml:"value"`
	Type    string `json:"type" yaml:"type"`
	Compare string `json:"compare,omitempty" yaml:"compare,omitempty"`
}

type Conditional struct {
	ValidPath   string `json:"validPath" yaml:"validPath"`
	InvalidPath string `json:"invalidPath" yaml:"invalidPath"`
	Expression  string `json:"expression" yaml:"expression"`
}

type ResponseConfig struct {
	Code     int    `json:"code" yaml:"code"`
	Template string `json:"template" yaml:"template"`
	Type     string `json:"type" yaml:"type"`
	// Not yet implemented
	Object ResponseObject `json:"responseObject" yaml:"responseObject"`
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

type DatasourceConfig struct {
	ID        string                 `json:"id" yaml:"id"`
	Config    json.RawMessage        `json:"config" yaml:"-"`
	NewConfig map[string]interface{} `yaml:"config"`
	Type      string                 `json:"type" yaml:"type"`
}

func (d *DatasourceConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
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
func (a *APIConfig) SchemaValidation() error {
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
