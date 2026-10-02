package actions

import (
	"context"
	"encoding/json"
)

// Definition describes an action to a host that offers it: what the action is
// called, the config fields it takes, what it publishes, and how to build it
// from a config. Each action package exports a Definition function returning
// one; the host decides which to register.
type Definition struct {
	Type        string               `json:"-"`
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Fields      map[string]FieldInfo `json:"fields"`
	// Output describes what the action publishes as a request variable, so the
	// dashboard can offer the paths a later step may read. The zero value means
	// OutputDynamic: an action that has not been described yet keeps working and
	// is offered whole.
	Output OutputInfo `json:"output"`
	// Exactly one of New and NewV2 is set. A V1 action receives its config with
	// templates already resolved; a V2 action resolves its own. ctx is the
	// context the action is built for, so what a constructor looks up is
	// looked up for it.
	New   func(ctx context.Context, config json.RawMessage) (ActionExecutable, error)   `json:"-"`
	NewV2 func(ctx context.Context, config json.RawMessage) (ActionExecutableV2, error) `json:"-"`
}

type FieldType string

const (
	FieldTypeString      FieldType = "string"
	FieldTypeIntegration FieldType = "integration"
	FieldTypeMap         FieldType = "map"
	FieldTypeBoolean     FieldType = "boolean"
	FieldTypeFile        FieldType = "file"
	FieldTypeTextArea    FieldType = "text_area"
	FieldTypeArray       FieldType = "array"
	// FieldTypeLLMProvider is a reference to a stored LLM provider instance,
	// held as its numeric id. Like FieldTypeIntegration it names a stored
	// record rather than a literal, so the dashboard resolves it to a picker
	// instead of asking the user to type an id.
	FieldTypeLLMProvider FieldType = "llm_provider"
)

type FieldInfo struct {
	Type        FieldType         `json:"type"`
	Label       string            `json:"label"`
	Description string            `json:"description"`
	Required    bool              `json:"required"`
	Default     any               `json:"default"`
	Values      []string          `json:"values"`
	Metadata    map[string]string `json:"metadata"`
}
