package stub

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Servflow/servflow/pkg/engine/actions"
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

func (s *Executable) Config() string {
	d, err := json.Marshal(s.Fields)
	if err != nil {
		panic(err)
	}
	return string(d)
}

func (s *Executable) Execute(ctx context.Context, modifiedConfig string) (interface{}, error) {
	var newFields map[string]interface{}
	if err := json.Unmarshal([]byte(modifiedConfig), &newFields); err != nil {
		return nil, err
	}
	return newFields, nil
}

func init() {
	if err := actions.RegisterAction("stub", func(config json.RawMessage) (actions.ActionExecutable, error) {
		var fields map[string]interface{}
		if err := json.Unmarshal(config, &fields); err != nil {
			return nil, fmt.Errorf("error creating stub action: %v", err)
		}
		return NewExecutable(fields), nil
	}); err != nil {
		panic(err)
	}
}
