package apiconfig

import (
	"git.servflow.io/servflow/definitions/proto"
)

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
	// Output names the output handler that extracts this workflow's output from
	// the request context after the plan runs, for workflows that do not
	// terminate in a Response step. Empty means none.
	Output OutputConfig `json:"output,omitempty" yaml:"output,omitempty"`
}

// OutputConfig selects and configures a workflow's output handler (see
// pkg/engine/outputs). It is the counterpart of HttpConfig.Handler /
// HandlerConfig on the way out: where an entry handler decides what happens to a
// request before the plan, an output handler decides what the run returns after
// it.
type OutputConfig struct {
	// Handler is the registered output handler kind, e.g. "conversation" to
	// return the agent's last message or "template" to render an expression.
	Handler string `json:"handler,omitempty" yaml:"handler,omitempty"`
	// Config is the handler's configuration, passed to its factory at
	// config-load time.
	Config map[string]interface{} `json:"config,omitempty" yaml:"config,omitempty"`
}

func (a *APIConfig) IsMCPConfig() bool {
	return a.McpTool.Enabled || a.McpTool.Name != ""
}

type HttpConfig struct {
	ListenPath         string   `json:"listenPath" yaml:"listenPath"`
	Method             string   `json:"method" yaml:"method"`
	Next               string   `json:"next" yaml:"next"`
	CORSAllowedOrigins []string `json:"corsAllowedOrigins,omitempty" yaml:"corsAllowedOrigins,omitempty"`
	// Handler names a registered HTTP entry handler (see pkg/engine/entryhandlers).
	// When set, its middleware wraps the request after the standard request
	// prerequisites and before the workflow plan runs. Empty means no handler.
	Handler string `json:"handler,omitempty" yaml:"handler,omitempty"`
	// HandlerConfig carries the entry handler's configuration. Values may contain
	// templates (e.g. {"secret": "{{ secret \"github\" }}"}) which the handler
	// resolves at request time.
	HandlerConfig map[string]interface{} `json:"handlerConfig,omitempty" yaml:"handlerConfig,omitempty"`
}

type McpConfig struct {
	Tools map[string]MCPToolConfig `json:"tools,omitempty" yaml:"tools,omitempty"`
}

type MCPToolConfig struct {
	Enabled     bool               `json:"enabled" yaml:"enabled"`
	Name        string             `json:"name" yaml:"name"`
	Description string             `json:"description" yaml:"description"`
	Args        map[string]ArgType `json:"args,omitempty" yaml:"args,omitempty"`
	// Result is the expression to be used to get the result.
	//
	// Deprecated: use APIConfig.Output with the "template" handler. Result is
	// still honoured — it is applied as a template output handler when the
	// config sets no Output — but new configs should carry an Output instead, so
	// every run shape describes its output the same way.
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
	Name       string                 `json:"name,omitempty" yaml:"name,omitempty" jsonschema:"required"`
	Type       string                 `json:"type" yaml:"type"`
	Config     map[string]interface{} `json:"config,omitempty" yaml:"config,omitempty"`
	Next       string                 `json:"next" yaml:"next"`
	Fail       string                 `json:"fail" yaml:"fail"`
	UseReplica bool                   `json:"useReplica,omitempty" yaml:"useReplica,omitempty"`
	Dispatch   []string               `json:"dispatch,omitempty" yaml:"dispatch,omitempty"`
}

type Conditional struct {
	Name       string            `json:"name,omitempty" yaml:"name,omitempty" jsonschema:"required"`
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
	// Name and Kind are shared by every response kind. Kind selects the response
	// type from the responses registry; an empty Kind defaults to "http". The
	// fields below it are specific to the built-in "http" kind.
	Name string `json:"name,omitempty" yaml:"name,omitempty" jsonschema:"required"`
	Kind string `json:"kind,omitempty" yaml:"kind,omitempty"`

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
