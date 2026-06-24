//go:generate mockgen -source planner.go -destination planner_mocks.go -package plan .
//go:generate mockgen -source ../actions/actions.go -destination actions_mock.go -package plan
package plan

import (
	"encoding/json"
	"fmt"
	"time"

	apiconfig "github.com/Servflow/servflow/pkg/apiconfig"
	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/Servflow/servflow/pkg/engine/integration"
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
	// EndLookupKey is the key that should be looked up to get the value copied to
	// the ResultTag, without the "."
	// Deprecated: use EndValue instead
	EndLookupKey string

	// EndValue is the value to be used as the ending of this plan, it can be
	// raw string or go template
	EndValue string

	// DispatchTimeout is the timeout duration for background dispatch chains.
	// If not set or set to 0, background actions will run without a timeout.
	DispatchTimeout time.Duration

	// Workspace is the file capability that workspace-aware actions and template
	// functions use. It is resolved per config (from the owning agent's assigned
	// workspace) and applied to each request's context before the plan runs. A
	// nil value means the config has no workspace; file actions then fail with
	// requestctx.ErrNoWorkspace.
	Workspace requestctx.Workspace

	CustomRegistry *actions.Registry
	Actions        map[string]apiconfig.Action
	Conditions     map[string]apiconfig.Conditional
	Responses      map[string]apiconfig.ResponseConfig
	Integrations   map[string]apiconfig.IntegrationConfig
}

type PlannerV2 struct {
	config     PlannerConfig
	finalSteps map[string]stepWrapper
	registry   *actions.Registry
	logger     *zap.Logger
}

func NewPlannerV2(config PlannerConfig, logger *zap.Logger) *PlannerV2 {
	return &PlannerV2{
		config:     config,
		finalSteps: make(map[string]stepWrapper),
		registry:   config.CustomRegistry,
		logger:     logger,
	}
}

func (p *PlannerV2) Plan() (*Plan, error) {
	for id := range p.config.Integrations {
		integ := p.config.Integrations[id]
		if err := integration.InitializeIntegration(integ.Type, id, integ.Config, integ.LazyLoad); err != nil {
			return nil, err
		}
	}
	for id := range p.config.Actions {
		id = apiconfig.ActionConfigPrefix + id
		err := p.generate(id)
		if err != nil {
			return nil, err
		}
	}
	for id := range p.config.Conditions {
		id = apiconfig.ConditionalConfigPrefix + id
		err := p.generate(id)
		if err != nil {
			return nil, err
		}
	}
	for id := range p.config.Responses {
		id = apiconfig.ResponsesConfigPrefix + id
		err := p.generate(id)
		if err != nil {
			return nil, err
		}
	}

	dispatchTimeout := p.config.DispatchTimeout
	if dispatchTimeout == 0 {
		dispatchTimeout = time.Minute
	}

	// Build action name to ID mapping
	actionNameToID := make(map[string]string)
	for id, action := range p.config.Actions {
		if action.Name != "" && action.Name != id {
			actionNameToID[action.Name] = id
		}
	}

	return &Plan{
		steps:           p.finalSteps,
		actionNameToID:  actionNameToID,
		dispatchTimeout: dispatchTimeout,
		workspace:       p.config.Workspace,
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

func (p *PlannerV2) generateStep(id string) (*stepWrapper, error) {
	// Note: This is called during plan generation, not execution, so no context available
	p.logger.Debug("Generating planner v2 step", zap.String("id", id))

	kind, bareID, terminal, err := apiconfig.ParseStepRef(id)
	if err != nil {
		return nil, err
	}
	if terminal {
		return nil, nil
	}

	// canonical id (with any leading "$" stripped) is the step map key
	canonical := apiconfig.CanonicalStepID(id)
	if st, ok := p.finalSteps[canonical]; ok {
		return &st, nil
	}

	var step Step
	switch kind {
	case apiconfig.StepKindAction:
		step, err = p.generateActionStep(bareID)
	case apiconfig.StepKindConditional:
		step, err = p.generateConditionalStep(bareID)
	case apiconfig.StepKindResponse:
		step, err = p.generateResponseStep(bareID)
	}
	if err != nil {
		return nil, err
	}

	stepWr := stepWrapper{
		id:   canonical,
		step: step,
	}
	p.finalSteps[canonical] = stepWr
	return &stepWr, nil
}

func (p *PlannerV2) generateActionStep(id string) (Step, error) {
	a, ok := p.config.Actions[id]
	if !ok {
		return nil, fmt.Errorf("actions not found: %s", id)
	}

	configJson, err := json.Marshal(a.Config)
	if err != nil {
		return nil, err
	}

	// Check if this is a V2 action
	isV2 := false
	if p.registry != nil {
		isV2 = p.registry.IsV2Action(a.Type)
	} else {
		isV2 = actions.IsV2Action(a.Type)
	}

	if isV2 {
		return p.generateActionStepV2(id, a, configJson)
	}
	return p.generateActionStepV1(id, a, configJson)
}

// generateActionStepV1 creates a V1 action step (template resolution in plan executor)
func (p *PlannerV2) generateActionStepV1(id string, a apiconfig.Action, configJson []byte) (*Action, error) {
	var (
		exec actions.ActionExecutable
		err  error
	)

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

	var failStep *stepWrapper
	if a.Fail != "" {
		failStep, err = p.generateStep(a.Fail)
		if err != nil {
			return nil, err
		}
	}

	out := id

	name := a.Name
	if name == "" {
		name = id
	}

	return &Action{
		id:         id,
		name:       name,
		next:       nextStep,
		fail:       failStep,
		out:        out,
		exec:       exec,
		useReplica: a.UseReplica,
		dispatch:   a.Dispatch,
	}, nil
}

// generateActionStepV2 creates a V2 action step (action handles own template resolution)
func (p *PlannerV2) generateActionStepV2(id string, a apiconfig.Action, configJson []byte) (*ActionV2, error) {
	var (
		exec actions.ActionExecutableV2
		err  error
	)

	if p.registry != nil {
		exec, err = p.registry.GetActionExecutableV2(a.Type, configJson)
	} else {
		exec, err = actions.GetActionExecutableV2(a.Type, configJson)
	}
	if err != nil {
		return nil, fmt.Errorf("error creating v2 action %s: %w", id, err)
	}

	nextStep, err := p.generateStep(a.Next)
	if err != nil {
		return nil, err
	}

	var failStep *stepWrapper
	if a.Fail != "" {
		failStep, err = p.generateStep(a.Fail)
		if err != nil {
			return nil, err
		}
	}

	name := a.Name
	if name == "" {
		name = id
	}

	return &ActionV2{
		id:         id,
		name:       name,
		next:       nextStep,
		fail:       failStep,
		exec:       exec,
		useReplica: a.UseReplica,
		dispatch:   a.Dispatch,
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

	if condition.Type == "" {
		if len(condition.Structure) > 0 {
			condition.Type = ConditionalTypeStructured
		} else {
			condition.Type = ConditionalTypeTemplate
		}
	}

	var exprString string
	switch condition.Type {
	case ConditionalTypeStructured:
		if len(condition.Structure) == 0 {
			return nil, fmt.Errorf("structured condition %s has empty structure", id)
		}
		exprString, err = ConvertStructureToTemplate(condition.Structure)
		if err != nil {
			return nil, fmt.Errorf("failed to convert structure to template for condition %s: %w", id, err)
		}
	case ConditionalTypeTemplate, "":
		if condition.Expression == "" {
			return nil, fmt.Errorf("template condition %s has empty expression", id)
		}
		exprString = condition.Expression
	default:
		return nil, fmt.Errorf("unsupported condition type: %s", condition.Type)
	}

	name := condition.Name
	if name == "" {
		name = id
	}

	return &ConditionStep{
		id:         id,
		name:       name,
		OnValid:    validStep,
		OnInvalid:  invalidStep,
		exprString: exprString,
	}, nil
}

// generateResponseStep creates a Response step based on the given id.
func (p *PlannerV2) generateResponseStep(id string) (*Response, error) {
	response, ok := p.config.Responses[id]
	if !ok {
		return nil, fmt.Errorf("response not found: %s", id)
	}

	name := response.Name
	if name == "" {
		name = id
	}

	return newResponse(id, name, response)
}
