// Package integration holds the types an integration is written against: the
// Integration interface, the fields it is configured with, and the Definition
// each integration package exports. Registering integrations and keeping
// their live instances is the host's job.
package integration

import "context"

// Integration is a configured connection to an outside system that actions
// use, such as a database or a vector store.
type Integration interface {
	Type() string
}

// Shutdownable is an Integration that holds resources the host must release
// when it replaces or drops the instance.
type Shutdownable interface {
	Shutdown(ctx context.Context) error
}

// BaseIntegration gives an integration a no-op Shutdown.
type BaseIntegration struct{}

func (b *BaseIntegration) Shutdown(ctx context.Context) error {
	return nil
}

type FieldType string

const (
	FieldTypeString   FieldType = "string"
	FieldTypeBoolean  FieldType = "boolean"
	FieldTypeNumber   FieldType = "number"
	FieldTypePassword FieldType = "password"
	FieldTypeSelect   FieldType = "select"
)

type FieldInfo struct {
	Type        FieldType `json:"type"`
	Label       string    `json:"label"`
	Description string    `json:"description"`
	Required    bool      `json:"required"`
	Default     any       `json:"default,omitempty"`
	Values      []string  `json:"values,omitempty"`
}

// Definition describes an integration to a host that offers it: what it is
// called, the fields it is configured with, and how to build an instance from
// a resolved config. Each integration package exports a Definition function
// returning one; the host decides which to register.
type Definition struct {
	Type        string                                           `json:"-"`
	Name        string                                           `json:"name"`
	Description string                                           `json:"description"`
	Fields      map[string]FieldInfo                             `json:"fields"`
	ImageURL    string                                           `json:"image_url"`
	New         func(config map[string]any) (Integration, error) `json:"-"`
}
