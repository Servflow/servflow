//go:generate mockgen -source planner.go -destination planner_mocks.go -package plan .
//go:generate mockgen -source ../actions/actions.go -destination actions_mock.go -package plan
package plan

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Servflow/servflow/internal/logging"
	"github.com/Servflow/servflow/pkg/definitions"
	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"go.uber.org/zap"
)

type ActionProvider interface {
	GetActionExecutable(actionType string, config json.RawMessage) (actions.ActionExecutable, error)
}

// PlannerConfig holds config values for the plan generation
type PlannerConfig struct {
	// ResultTag is the key that holds the value copied from EndLookupKey
	ResultTag string
	// TerminateTag is the tag that stops the plan generation
	// Deprecated: can work without it
	TerminateTag string
	// EndLookupKey is the key that should be looked up to get the value copied to
	// the ResultTag, without the "."
	// Deprecated: use EndValue instead
	EndLookupKey string

	// EndValue is the value to be used as the ending of this plan, it can be
	// raw string or go template
	EndValue string

	CustomRegistry *actions.Registry
	Actions        map[string]apiconfig.Action
	Conditions     map[string]apiconfig.Conditional
	Responses      map[string]apiconfig.ResponseConfig
}

type PlannerV2 struct {
	config     PlannerConfig
	finalSteps map[string]Step
	registry   *actions.Registry
}

func NewPlannerV2(config PlannerConfig) *PlannerV2 {
	return &PlannerV2{config: config, finalSteps: make(map[string]Step), registry: config.CustomRegistry}
}

func (p *PlannerV2) Plan() (*Plan, error) {
	for id := range p.config.Actions {
		id = requestctx.ActionConfigPrefix + id
		err := p.generate(id)
		if err != nil {
			return nil, err
		}
	}
	for id := range p.config.Conditions {
		id = requestctx.ConditionalConfigPrefix + id
		err := p.generate(id)
		if err != nil {
			return nil, err
		}
	}
	for id := range p.config.Responses {
		id = requestctx.ResponsesConfigPrefix + id
		err := p.generate(id)
		if err != nil {
			return nil, err
		}
	}
	return &Plan{
		steps: p.finalSteps,
	}, nil
}

func (p *PlannerV2) generate(id string) error {
	if _, ok := p.finalSteps[id]; ok {
		return nil
	}

	_, err := p.generateStep(id)
	if err != nil {
		return err
	}

	return nil
}

func (p *PlannerV2) generateStep(id string) (Step, error) {
	if id == "" || id == p.config.TerminateTag {
		return nil, nil
	}
	logging.GetLogger().Debug("Generating planner v2 step", zap.String("id", id))

	// backwards compatibility
	id = strings.TrimPrefix(id, "$")

	if _, ok := p.finalSteps[id]; ok {
		return p.finalSteps[id], nil
	}

	var (
		err  error
		step Step
	)
	switch {
	case strings.HasPrefix(id, requestctx.ActionConfigPrefix):
		step, err = p.generateActionStep(strings.TrimPrefix(id, requestctx.ActionConfigPrefix))
	case strings.HasPrefix(id, requestctx.ConditionalConfigPrefix):
		step, err = p.generateConditionalStep(strings.TrimPrefix(id, requestctx.ConditionalConfigPrefix))
	case strings.HasPrefix(id, requestctx.ResponsesConfigPrefix):
		step, err = p.generateResponseStep(strings.TrimPrefix(id, requestctx.ResponsesConfigPrefix))
	default:
		err = fmt.Errorf("unknown step type: %s", id)
	}
	if err != nil {
		return nil, err
	}
	p.finalSteps[id] = step
	return step, nil
}

func (p *PlannerV2) generateActionStep(id string) (*Action, error) {
	a, ok := p.config.Actions[id]
	if !ok {
		return nil, fmt.Errorf("actions not found: %s", id)
	}

	var (
		exec actions.ActionExecutable
		err  error
	)

	configJson, err := json.Marshal(a.Config)
	if err != nil {
		return nil, err
	}

	if p.registry != nil {
		exec, err = p.registry.GetActionExecutable(a.Type, configJson)
	} else {
		exec, err = actions.GetActionExecutable(a.Type, configJson)
	}
	if err != nil {
		return nil, fmt.Errorf("error creating actions %s: %w", id, err)
	}

	nextStep, err := p.generateStep(a.Next)
	if err != nil {
		return nil, err
	}

	var failStep Step
	if a.Fail != "" {
		failStep, err = p.generateStep(a.Fail)
		if err != nil {
			return nil, err
		}

	}

	out := fmt.Sprintf("%s%s", requestctx.VariableActionPrefix, id)

	return &Action{
		configStr: exec.Config(),
		id:        id,
		next:      nextStep,
		fail:      failStep,
		out:       out,
		exec:      exec,
	}, nil
}

// generateConditionalStep creates a ConditionStep based on the given id.
func (p *PlannerV2) generateConditionalStep(id string) (*ConditionStep, error) {
	condition, ok := p.config.Conditions[id]
	if !ok {
		return nil, fmt.Errorf("condition not found: %s", id)
	}

	validStep, err := p.generateStep(condition.OnTrue)
	if err != nil {
		return nil, err
	}

	invalidStep, err := p.generateStep(condition.OnFalse)
	if err != nil {
		return nil, err
	}

	return &ConditionStep{
		id:         id,
		OnValid:    validStep,
		OnInvalid:  invalidStep,
		exprString: condition.Expression,
	}, nil
}

// generateResponseStep creates a Response step based on the given id.
func (p *PlannerV2) generateResponseStep(id string) (*Response, error) {
	response, ok := p.config.Responses[id]
	if !ok {
		return nil, fmt.Errorf("response not found: %s", id)
	}

	return newResponse(id, response)
}
