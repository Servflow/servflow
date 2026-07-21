// Package loop provides the loop action: ServFlow's dynamic-iteration primitive.
// It resolves a template to a JSON array and runs a body action chain once per
// element, exposing the current element and index to that chain via the
// loop_item/loop_index template functions. Unlike parallel (a fixed named set),
// loop iterates a runtime-sized list, so it is the way to "do an action per
// element of a list" (e.g. one HTTP call per id).
package loop

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/Servflow/servflow/pkg/engine/plan"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
)

// Config is the loop action's stored configuration.
type Config struct {
	// Items is a template that must resolve to a JSON array string. Each element
	// becomes the current loop_item for one run of the body chain. Config values
	// are rendered to strings, so a structured source must be emitted as JSON,
	// e.g. {{ jsonout .some.list }} or a literal [...].
	Items string `json:"items"`
	// Start is the action id of the body chain, executed once per element.
	Start string `json:"start"`
}

// Exec runs the loop action. It uses the v2 interface because Items is a
// template it must resolve itself (Start is a static action id, not templated).
type Exec struct {
	config Config
}

func New(cfg Config) (*Exec, error) {
	if strings.TrimSpace(cfg.Start) == "" {
		return nil, fmt.Errorf("loop: start is required")
	}
	return &Exec{config: cfg}, nil
}

func (e *Exec) Type() string {
	return "loop"
}

func (e *Exec) SupportsReplica() bool {
	return false
}

// Execute resolves Items to a JSON array and runs the body chain once per
// element. Iteration is sequential; each element is exposed to the body via
// loop_item/loop_index. The action itself produces no value (returns nil): the
// body chain writes whatever it needs into the request context. An empty array
// is a clean no-op (zero iterations).
func (e *Exec) Execute(ctx context.Context) (interface{}, map[string]string, error) {
	rc, err := requestctx.FromContextOrError(ctx)
	if err != nil {
		return nil, nil, err
	}

	rendered, err := rc.Resolve(ctx, e.config.Items)
	if err != nil {
		return nil, nil, fmt.Errorf("loop: resolving items: %w", err)
	}

	items, err := parseItems(rendered)
	if err != nil {
		return nil, nil, err
	}

	fields := map[string]string{"count": fmt.Sprintf("%d", len(items))}

	for i, item := range items {
		rc.PushLoop(item, i)
		_, err := plan.ExecuteFromContext(ctx, e.config.Start)
		rc.PopLoop()
		if err != nil {
			return nil, fields, fmt.Errorf("loop: iteration %d: %w", i, err)
		}
	}

	return nil, fields, nil
}

// parseItems parses the resolved items template into a slice. An empty (or
// whitespace-only) string is treated as an empty list so a template that
// resolved to nothing runs zero iterations rather than erroring. A value that
// is valid JSON but not an array is rejected.
func parseItems(rendered string) ([]interface{}, error) {
	trimmed := strings.TrimSpace(rendered)
	if trimmed == "" {
		return nil, nil
	}
	var items []interface{}
	if err := json.Unmarshal([]byte(trimmed), &items); err != nil {
		return nil, fmt.Errorf("loop: items must resolve to a JSON array: %w", err)
	}
	return items, nil
}

func init() {
	if err := actions.RegisterAction("loop", actions.ActionRegistrationInfo{
		Name:        "Loop",
		Description: "Iterates a JSON array, running a body chain once per element. The current element and index are available via {{ loop_item }} and {{ loop_index }}.",
		Fields: map[string]actions.FieldInfo{
			"items": {
				Type:        actions.FieldTypeString,
				Label:       "Items",
				Placeholder: "Template resolving to a JSON array, e.g. {{ jsonout .fetch.ids }}",
				Required:    true,
			},
			"start": {
				Type:        actions.FieldTypeString,
				Label:       "Start",
				Placeholder: "Action id of the body chain to run per element",
				Required:    true,
			},
		},
		UseV2: true,
		ConstructorV2: func(config json.RawMessage) (actions.ActionExecutableV2, error) {
			var cfg Config
			if err := json.Unmarshal(config, &cfg); err != nil {
				return nil, fmt.Errorf("error creating loop action: %w", err)
			}
			return New(cfg)
		},
	}); err != nil {
		panic(err)
	}
}
