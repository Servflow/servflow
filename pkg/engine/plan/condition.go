package plan

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Servflow/servflow/pkg/apiconfig"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/Servflow/servflow/pkg/logging"
	"github.com/Servflow/servflow/pkg/tracing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const (
	ConditionalTypeTemplate   = "template"
	ConditionalTypeStructured = "structured"

	FunctionEmail    = "email"
	FunctionEmpty    = "empty"
	FunctionNotempty = "notempty"
	FunctionBcrypt   = "bcrypt"
	FunctionEq       = "eq"
	FunctionNe       = "ne"
	FunctionLt       = "lt"
	FunctionLe       = "le"
	FunctionGt       = "gt"
	FunctionGe       = "ge"

	TemplateFalse  = "{{ false }}"
	TemplatePrefix = "{{"
	TemplateSuffix = "}}"
	TemplateOr     = "or"
	TemplateAnd    = "and"
)

// Condition is a conditional compiled down to the one expression it
// evaluates, together with the identity it reports while doing so.
//
// It carries no routing. Which step follows a verdict is the plan's business,
// so a host that gates something other than a step on a conditional — an
// agent's tools, say — compiles one of these and evaluates it directly,
// rather than reimplementing the type inference, the template rules and the
// span that go with a conditional.
type Condition struct {
	id         string
	name       string
	exprString string
}

// NewCondition compiles a conditional into the expression it evaluates.
//
// The type is inferred when the conditional does not state one: a conditional
// carrying a structure is structured, anything else is a template. OnTrue and
// OnFalse are ignored — a compiled condition answers true or false and routes
// nothing.
//
// id names the condition in its span and in the errors returned here. The
// display name falls back to it, so a condition always has something to call
// itself.
func NewCondition(id string, condition apiconfig.Conditional) (*Condition, error) {
	if condition.Type == "" {
		if len(condition.Structure) > 0 {
			condition.Type = ConditionalTypeStructured
		} else {
			condition.Type = ConditionalTypeTemplate
		}
	}

	var (
		exprString string
		err        error
	)
	switch condition.Type {
	case ConditionalTypeStructured:
		if len(condition.Structure) == 0 {
			return nil, fmt.Errorf("structured condition %s has empty structure", id)
		}
		exprString, err = ConvertStructureToTemplate(condition.Structure)
		if err != nil {
			return nil, fmt.Errorf("failed to convert structure to template for condition %s: %w", id, err)
		}
	case ConditionalTypeTemplate:
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

	return &Condition{
		id:         id,
		name:       name,
		exprString: exprString,
	}, nil
}

func (c *Condition) ID() string {
	return c.id
}

// Name is what the condition calls itself in a span or a message. It falls
// back to the id, so it is never empty.
func (c *Condition) Name() string {
	return c.name
}

// Expression is the template the condition evaluates, whether it was written
// as one or converted from a structure. A host that wants to judge the
// template itself — parsing it at config-write time, say — reads it here
// rather than reproducing the conversion.
func (c *Condition) Expression() string {
	return c.exprString
}

// Evaluate resolves the condition against the request context on ctx and
// reports whether it rendered true.
//
// It opens the condition span itself, so a condition reads the same in a
// trace wherever it is evaluated from. Validation errors raised by the
// template functions (email, empty) are recorded on the request and do not
// fail the evaluation, which is what lets a condition double as a validator.
func (c *Condition) Evaluate(ctx context.Context) (bool, error) {
	// set up tracer
	var span trace.Span
	ctx, span = tracing.StartCondition(ctx, c.id, c.name)
	defer span.End()

	span.SetAttributes(attribute.String("sf.config", c.exprString))

	logger := logging.FromContext(ctx).With(
		zap.String("conditional_id", c.id),
		zap.String("conditional_name", c.name),
	)
	ctx = logging.WithLogger(ctx, logger)
	if c.exprString == "" {
		span.SetAttributes(attribute.Bool("sf.result", true))
		return true, nil
	}

	reqCtx, ok := requestctx.FromContext(ctx)
	if !ok {
		return false, errors.New("invalid request context")
	}

	tmpl, err := requestctx.CreateTextTemplate(ctx, c.exprString, nil)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return false, fmt.Errorf("error creating template for condition %w template: %s", err, c.exprString)
	}

	resp, err := requestctx.ExecuteTemplateFromContext(ctx, tmpl)
	if err != nil {
		logger.Error("error executing template",
			zap.String("condition", c.name), zap.String("expression", c.exprString), zap.Error(err))
		logger.Debug("error executing template", zap.String("expression", c.exprString), zap.Any("resp", reqCtx.Variables()))
		span.RecordError(err)
		return false, err
	}
	// add validation errors they should not cause any failures
	err = requestctx.AddValidationErrors(ctx)
	if err != nil {
		logger.Error("error adding validation error", zap.Error(err))
		return false, err
	}

	logger.Debug("condition evaluated to "+resp, zap.String("condition", c.exprString))
	result := strings.TrimSpace(resp) == "true"
	span.SetAttributes(attribute.Bool("sf.result", result))
	return result, nil
}

// ConditionStep is a compiled condition placed in a plan: it evaluates, then
// hands back the step the verdict selects.
type ConditionStep struct {
	*Condition
	OnValid   *stepWrapper
	OnInvalid *stepWrapper
}

// Execute will execute the conditions and generate error messages for conditions that use
// request variables
func (c *ConditionStep) execute(ctx context.Context) (*stepWrapper, error) {
	valid, err := c.Evaluate(ctx)
	if err != nil {
		return nil, err
	}
	if valid {
		return c.OnValid, nil
	}
	return c.OnInvalid, nil
}

type ConditionalFunctionSpec struct {
	Template           string
	RequiresTitle      bool
	RequiresComparison bool
}

var conditionalFunctionSpecs = map[string]ConditionalFunctionSpec{
	FunctionEmail: {
		Template:           "email (%s) (\"%s\")",
		RequiresTitle:      true,
		RequiresComparison: false,
	},
	FunctionEmpty: {
		Template:           "empty (%s) (\"%s\")",
		RequiresTitle:      true,
		RequiresComparison: false,
	},
	FunctionNotempty: {
		Template:           "notempty (%s) (\"%s\")",
		RequiresTitle:      true,
		RequiresComparison: false,
	},
	FunctionBcrypt: {
		Template:           "bcrypt (%s) (%s) (\"%s\")",
		RequiresTitle:      true,
		RequiresComparison: true,
	},
	FunctionEq: {
		Template:           "eq (%s) (%s)",
		RequiresTitle:      false,
		RequiresComparison: true,
	},
	FunctionNe: {
		Template:           "ne (%s) (%s)",
		RequiresTitle:      false,
		RequiresComparison: true,
	},
	FunctionLt: {
		Template:           "lt (%s) (%s)",
		RequiresTitle:      false,
		RequiresComparison: true,
	},
	FunctionLe: {
		Template:           "le (%s) (%s)",
		RequiresTitle:      false,
		RequiresComparison: true,
	},
	FunctionGt: {
		Template:           "gt (%s) (%s)",
		RequiresTitle:      false,
		RequiresComparison: true,
	},
	FunctionGe: {
		Template:           "ge (%s) (%s)",
		RequiresTitle:      false,
		RequiresComparison: true,
	},
}

func ConvertStructureToTemplate(structure [][]apiconfig.ConditionItem) (string, error) {
	if len(structure) == 0 {
		return TemplateFalse, nil
	}

	var orClauses []string

	for i, andGroup := range structure {
		if len(andGroup) == 0 {
			continue
		}

		var andConditions []string
		for j, item := range andGroup {
			conditionTemplate, err := generateConditionItemTemplate(item)
			if err != nil {
				return "", fmt.Errorf("error generating template for structure[%d][%d]: %w", i, j, err)
			}
			andConditions = append(andConditions, fmt.Sprintf("(%s)", conditionTemplate))
		}

		if len(andConditions) == 1 {
			orClauses = append(orClauses, andConditions[0])
		} else {
			andClause := fmt.Sprintf("(%s %s)", TemplateAnd, strings.Join(andConditions, " "))
			orClauses = append(orClauses, andClause)
		}
	}

	if len(orClauses) == 1 {
		clause := orClauses[0]
		if strings.HasPrefix(clause, "(") && strings.HasSuffix(clause, ")") {
			clause = clause[1 : len(clause)-1]
		}
		return fmt.Sprintf("%s %s %s", TemplatePrefix, clause, TemplateSuffix), nil
	}

	return fmt.Sprintf("%s %s %s %s", TemplatePrefix, TemplateOr, strings.Join(orClauses, " "), TemplateSuffix), nil
}

func generateConditionItemTemplate(item apiconfig.ConditionItem) (string, error) {
	spec, exists := conditionalFunctionSpecs[item.Function]
	if !exists {
		return "", fmt.Errorf("unsupported conditional function: %s", item.Function)
	}

	if spec.RequiresTitle && item.Title == "" {
		item.Title = "field"
	}

	if spec.RequiresComparison && item.Comparison == "" {
		return "", fmt.Errorf("function '%s' requires a comparison field", item.Function)
	}

	switch item.Function {
	case FunctionEmail, FunctionEmpty, FunctionNotempty:
		return fmt.Sprintf(spec.Template, item.Content, item.Title), nil
	case FunctionBcrypt:
		return fmt.Sprintf(spec.Template, item.Content, item.Comparison, item.Title), nil
	case FunctionEq, FunctionNe, FunctionLt, FunctionLe, FunctionGt, FunctionGe:
		return fmt.Sprintf(spec.Template, item.Content, item.Comparison), nil
	default:
		return "", fmt.Errorf("unhandled function: %s", item.Function)
	}
}
