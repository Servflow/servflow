// Package apiconfig holds the config types an action or integration reads:
// file inputs, integration configs, and the prefix a step uses to reach
// another action's output. The workflow config that links steps together is
// the host's.
package apiconfig

// ActionConfigPrefix prefixes a reference to an action, e.g.
// "action.createUser". An action reads another action's output by it.
const ActionConfigPrefix = "action."

const (
	FileInputTypeRequest = "request"
	FileInputTypeAction  = "action"
	FileInputTypeStorage = "storage"
)

type FileInput struct {
	Type       string `json:"type" yaml:"type"`
	Identifier string `json:"identifier" yaml:"identifier"`
}

type IntegrationConfig struct {
	ID     string                 `json:"id" yaml:"id"`
	Config map[string]ConfigValue `json:"config,omitempty" yaml:"config,omitempty"`
	Type   string                 `json:"type" yaml:"type"`
}

// ConfigValue is one integration config field. It is either a literal (Value)
// or a reference to a stored secret (Secret). Secret wins when both are set;
// an unset Secret means the field is used as written. The engine resolves these
// to a plain map[string]any before handing the config to an integration
// constructor — see pkg/engine/integration.
type ConfigValue struct {
	Value  any    `json:"value,omitempty" yaml:"value,omitempty"`
	Secret string `json:"secret,omitempty" yaml:"secret,omitempty"`
}
