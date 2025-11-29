package plan

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Servflow/servflow/internal/tracing"
	apiconfig "github.com/Servflow/servflow/pkg/definitions"
	requestctx2 "github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/Servflow/servflow/pkg/logging"
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

type ConditionStep struct {
	id         string
	OnValid    *stepWrapper
	OnInvalid  *stepWrapper
	exprString string
}

func (c *ConditionStep) ID() string {
	return c.id
}

// Execute will execute the conditions and generate error messages for conditions that use
// request variables
func (c *ConditionStep) execute(ctx context.Context) (*stepWrapper, error) {
	// set up tracer
	var span trace.Span
	ctx, span = tracing.SpanCtxFromContext(ctx, "condition.execute."+c.id)
	defer span.End()

	logger := logging.FromContext(ctx)
	if c.exprString == "" {
		return c.OnValid, nil
	}

	reqCtx, ok := requestctx2.FromContext(ctx)
	if !ok {
		return nil, errors.New("invalid request context")
	}

	tmpl, err := requestctx2.CreateTextTemplate(ctx, c.exprString, reqCtx.ConditionalTemplateFunctions())
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("error creating template for condition %w template: %s", err, c.exprString)
	}

	resp, err := requestctx2.ExecuteTemplateFromContext(ctx, tmpl)
	if err != nil {
		logger.Error("error executing template: "+c.exprString, zap.Error(err))
		span.RecordError(err)
		return nil, err
	}

	err = requestctx2.AddValidationErrors(ctx)
	if err != nil {
		logger.Error("error adding validation error", zap.Error(err))
		return nil, err
	}

	logger.Debug("condition evaluated to "+resp, zap.String("condition", c.exprString))
	if strings.TrimSpace(resp) == "true" {
		span.SetAttributes(attribute.Bool("condition.isValid", true))
		return c.OnValid, nil
	}
	span.SetAttributes(attribute.Bool("condition.isValid", false))
	return c.OnInvalid, nil
}

type ConditionalFunctionSpec struct {
	Template           string
	RequiresTitle      bool
	RequiresComparison bool
}

var conditionalFunctionSpecs = map[string]ConditionalFunctionSpec{
	FunctionEmail: {
		Template:           "email %s \"%s\"",
		RequiresTitle:      true,
		RequiresComparison: false,
	},
	FunctionEmpty: {
		Template:           "empty %s \"%s\"",
		RequiresTitle:      true,
		RequiresComparison: false,
	},
	FunctionNotempty: {
		Template:           "notempty %s \"%s\"",
		RequiresTitle:      true,
		RequiresComparison: false,
	},
	FunctionBcrypt: {
		Template:           "bcrypt %s %s \"%s\"",
		RequiresTitle:      true,
		RequiresComparison: true,
	},
	FunctionEq: {
		Template:           "eq %s %s",
		RequiresTitle:      false,
		RequiresComparison: true,
	},
	FunctionNe: {
		Template:           "ne %s %s",
		RequiresTitle:      false,
		RequiresComparison: true,
	},
	FunctionLt: {
		Template:           "lt %s %s",
		RequiresTitle:      false,
		RequiresComparison: true,
	},
	FunctionLe: {
		Template:           "le %s %s",
		RequiresTitle:      false,
		RequiresComparison: true,
	},
	FunctionGt: {
		Template:           "gt %s %s",
		RequiresTitle:      false,
		RequiresComparison: true,
	},
	FunctionGe: {
		Template:           "ge %s %s",
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
		return "", fmt.Errorf("function '%s' requires a title field", item.Function)
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
