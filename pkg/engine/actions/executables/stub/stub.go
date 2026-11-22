package stub

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
)

type Executable struct {
	Fields map[string]interface{}
}

func (s *Executable) Type() string {
	return "stub"
}

func NewExecutable(cfg map[string]interface{}) *Executable {
	return &Executable{
		Fields: cfg,
	}
}

// Execute returns the configured fields with every string in them resolved.
func (s *Executable) Execute(ctx context.Context) (interface{}, map[string]string, error) {
	fields := make(map[string]any, len(s.Fields))
	for k, v := range s.Fields {
		fields[k] = v
	}
	resolved, err := requestctx.ResolveValues(ctx, fields)
	if err != nil {
		return nil, nil, err
	}
	return resolved, nil, nil
}

// Definition describes the stub action to a host that offers it.
func Definition() actions.Definition {
	fields := map[string]actions.FieldInfo{}

	return actions.Definition{
		Type:        "stub",
		Name:        "Stub Action",
		Description: "A placeholder action that accepts any configuration for testing purposes",
		Fields:      fields,
		Output: actions.OutputInfo{
			Kind:        actions.OutputDynamic,
			Description: "The configured response, echoed back as it was written.",
		},
		New: func(_ context.Context, config json.RawMessage) (actions.ActionExecutable, error) {
			var fields map[string]interface{}
			if err := json.Unmarshal(config, &fields); err != nil {
				return nil, fmt.Errorf("error creating stub action: %v", err)
			}
			return NewExecutable(fields), nil
		},
	}
}
