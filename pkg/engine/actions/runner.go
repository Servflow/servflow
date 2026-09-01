package actions

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Servflow/servflow/pkg/engine/requestctx"
)

// Runner is one registered action, runnable on its own.
//
// It owns what a caller would otherwise repeat by hand: constructing the
// executable from the registry, resolving its config against the request, and
// knowing which of the two action generations it is holding. A plan step is
// the usual way to run an action, but not the only one — an agent tool runs a
// single action per call, with no graph around it — and that caller should not
// have to know that v1 receives a rendered config string while v2 resolves its
// own.
//
// The lifecycle matches the planner's: construct once, resolve per execution.
type Runner struct {
	actionType string
	v1         ActionExecutable
	v2         ActionExecutableV2
}

// NewRunner constructs the executable for the action type's generation.
//
// It fails on an unregistered type, so holding a *Runner means the action
// exists and its config unmarshalled. Building at construction rather than at
// execution is what lets a caller reject a bad config while it is being set
// up, instead of at the first call.
func NewRunner(actionType string, config json.RawMessage) (*Runner, error) {
	if IsV2Action(actionType) {
		exec, err := GetActionExecutableV2(actionType, config)
		if err != nil {
			return nil, fmt.Errorf("action %q: %w", actionType, err)
		}
		return &Runner{actionType: actionType, v2: exec}, nil
	}

	exec, err := GetActionExecutable(actionType, config)
	if err != nil {
		return nil, fmt.Errorf("action %q: %w", actionType, err)
	}
	return &Runner{actionType: actionType, v1: exec}, nil
}

// Type is the action type this runner executes.
func (r *Runner) Type() string {
	return r.actionType
}

// Execute resolves the action's config and runs it, returning what the action
// returned along with its trace fields.
//
// A v2 action resolves its own config, so ctx is handed to it untouched — any
// overlay on ctx (requestctx.WithTemplateFuncs) is in scope for that
// resolution. A v1 action does not resolve anything: its config template is
// rendered here and passed in, the same contract the plan executor honours.
//
// The result is returned as the action produced it. Scrubbing belongs to
// whoever externalizes it, which is where the plan step does it too.
func (r *Runner) Execute(ctx context.Context) (resp interface{}, fields map[string]string, err error) {
	if r.v2 != nil {
		return r.v2.Execute(ctx)
	}

	var resolved string
	if config := r.v1.Config(); config != "" {
		resolved, err = requestctx.ExecuteTemplateString(ctx, config)
		if err != nil {
			return nil, nil, fmt.Errorf("action %q: resolving config: %w", r.actionType, err)
		}
	}
	return r.v1.Execute(ctx, resolved)
}

// Executable returns the constructed action, whichever generation this runner
// holds.
//
// It is here because a host may want to ask the action for behaviour the
// engine has no opinion about — an agent tool asks whether the action can
// render its own output for a model to read — and asking should not mean
// re-deriving which generation the action is. The host asserts its own
// interface on what comes back; the engine defines none.
func (r *Runner) Executable() any {
	if r.v2 != nil {
		return r.v2
	}
	return r.v1
}
