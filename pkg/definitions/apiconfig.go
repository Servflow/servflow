package apiconfig

import (
	"encoding/json"

	"git.servflow.io/servflow/definitions/proto"
)

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

	// Deprecated: use HttpConfig.Listenpath
	ListenPath string `json:"listenPath" yaml:"listenPath"`
	// Deprecated: use HttpConfig.Method
	Method string `json:"method" yaml:"method"`
	// Deprecated: use HttpConfig
	Request RequestConfig `json:"request" yaml:"request"`
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
	// Result represents the tag to get the result from
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
	Type      string                 `json:"type" yaml:"type"`
	Config    json.RawMessage        `json:"config" yaml:"-"`
	NewConfig map[string]interface{} `yaml:"config"`
	Next      string                 `json:"next" yaml:"next"`
	Fail      string                 `json:"fail" yaml:"fail"`
}

func (a *Action) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var tmp struct {
		Type      string                 `yaml:"type"`
		NewConfig map[string]interface{} `yaml:"config"`
		Next      string                 `yaml:"next"`
		Fail      string                 `yaml:"fail"`
	}
	if err := unmarshal(&tmp); err != nil {
		return err
	}
	config, err := json.Marshal(tmp.NewConfig)
	if err != nil {
		return err
	}
	a.Type = tmp.Type
	a.Config = config
	a.NewConfig = tmp.NewConfig
	a.Next = tmp.Next
	a.Fail = tmp.Fail
	return nil
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

type ActionConfig struct {
	ID            string `yaml:"id"`
	Type          string `yaml:"type"`
	Configuration string `yaml:"configuration"`
}

type ResponseConfig struct {
	Code         int            `json:"code" yaml:"code"`
	Template     string         `json:"template" yaml:"template"`
	Type         string         `json:"type" yaml:"type"`
	ShouldStream bool           `json:"shouldStream" yaml:"shouldStream"`
	BuilderType  string         `json:"builderType" yaml:"builderType"`
	Object       ResponseObject `json:"responseObject" yaml:"responseObject"`
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

func (a *APIConfig) Normalize() {
	if a.HttpConfig.Method == "" {
		a.HttpConfig.Method = a.Method
	}
	if a.HttpConfig.Next == "" {
		a.HttpConfig.Next = a.Request.Next
	}
	if a.HttpConfig.ListenPath == "" {
		a.HttpConfig.ListenPath = a.ListenPath
	}
	if len(a.HttpConfig.CORSAllowedOrigins) < 1 {
		a.HttpConfig.CORSAllowedOrigins = a.Request.CORSAllowedOrigins
	}
}
