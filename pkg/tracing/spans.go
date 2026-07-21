package tracing

import (
	"context"
	"errors"
	"net/http"

	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// scrubber is the subset of *requestctx.RequestContext spans need to mask
// secret values resolved during the owning request.
type scrubber interface {
	HasSecrets() bool
	Scrub(string) string
}

// scrubSpan wraps a span so every string that lands on it — attributes,
// recorded errors, status descriptions — is scrubbed of the request's tracked
// secret values. All spans created through this package's constructors are
// wrapped when a RequestContext is present, which is why span creation must go
// through these constructors and never through a raw tracer.
type scrubSpan struct {
	trace.Span
	s scrubber
}

func (sp scrubSpan) SetAttributes(kv ...attribute.KeyValue) {
	if sp.s.HasSecrets() {
		for i, a := range kv {
			if a.Value.Type() == attribute.STRING {
				kv[i] = attribute.String(string(a.Key), sp.s.Scrub(a.Value.AsString()))
			}
		}
	}
	sp.Span.SetAttributes(kv...)
}

func (sp scrubSpan) RecordError(err error, opts ...trace.EventOption) {
	if err != nil && sp.s.HasSecrets() {
		err = errors.New(sp.s.Scrub(err.Error()))
	}
	sp.Span.RecordError(err, opts...)
}

func (sp scrubSpan) SetStatus(code codes.Code, description string) {
	if sp.s.HasSecrets() {
		description = sp.s.Scrub(description)
	}
	sp.Span.SetStatus(code, description)
}

// Standard servflow span attribute keys. Everything is namespaced under "sf."
// to stay clear of OpenTelemetry semantic conventions. Span names are kept
// low-cardinality (the step class); the per-instance label lives in AttrName.
const (
	AttrName         = "sf.name"        // friendly per-instance label rendered by dashboards
	AttrAgent        = "sf.agent"       // name of the agent that owns the workflow config, stamped by the host (pro)
	AttrWorkflow     = "sf.workflow"    // stable workflow config id, carried on root entry spans for grouping/search
	AttrStepType     = "sf.step.type"   // request | action | condition | response | trigger | scheduled
	AttrActionType   = "sf.action_type" // concrete action type (http, callworkflow, parallel, ...)
	AttrActionConfig = "sf.config"      // resolved action config (V1 wrapper sets it; V2 actions self-report via fields)
	AttrID           = "sf.id"          // bare node id (no prefix)
	AttrToolName     = "sf.tool_name"
	AttrToolType     = "sf.tool_type"   // mcp | workflow
	AttrToolParams   = "sf.tool_params" // JSON of model-supplied tool-call arguments (sensitive keys redacted, size-capped)
)

// start creates a span with a low-cardinality name and always attaches the
// friendly display label as the AttrName attribute. All typed constructors
// below funnel through here so no span can be created without a label — and,
// when the context carries a RequestContext, without secret scrubbing.
func start(ctx context.Context, spanName, display string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	ctx, span := tracer.Start(ctx, spanName)
	if rc, ok := requestctx.FromContext(ctx); ok {
		span = scrubSpan{Span: span, s: rc}
		// Re-store the wrapped span so trace.SpanFromContext(ctx) callers also
		// get the scrubbing wrapper, not the raw span.
		ctx = trace.ContextWithSpan(ctx, span)
	}
	span.SetAttributes(append(attrs, attribute.String(AttrName, display))...)
	return ctx, span
}

// rootDisplay picks the friendly label for a workflow root span: the
// human-readable name when set, otherwise the stable config id.
func rootDisplay(name, id string) string {
	if name != "" {
		return name
	}
	return id
}

// StartHTTPEntry spans the HTTP request entry point. name is the workflow's
// friendly display name and id its stable config id; both identify which
// workflow the trace belongs to.
func StartHTTPEntry(ctx context.Context, name, id string) (context.Context, trace.Span) {
	return start(ctx, "HTTP Entry", rootDisplay(name, id),
		attribute.String(AttrStepType, "request"),
		attribute.String(AttrWorkflow, id))
}

// SetHTTPStatus records the final HTTP status code on the entry span and, per
// the OpenTelemetry HTTP server conventions, marks the span status as Error
// only for 5xx responses — a 4xx is a client error and leaves the status Unset.
// A non-nil err is attached as a span event. Safe to call with a nil span.
func SetHTTPStatus(span trace.Span, code int, err error) {
	if span == nil {
		return
	}
	span.SetAttributes(attribute.Int("sf.http.status_code", code))
	if err != nil {
		span.RecordError(err)
	}
	if code >= 500 {
		msg := http.StatusText(code)
		if err != nil {
			msg = err.Error()
		}
		span.SetStatus(codes.Error, msg)
	}
}

// StartAction spans a plan action step. actionType is the concrete action type.
func StartAction(ctx context.Context, id, name, actionType string) (context.Context, trace.Span) {
	return start(ctx, "Action", name,
		attribute.String(AttrStepType, "action"),
		attribute.String(AttrActionType, actionType),
		attribute.String(AttrID, id))
}

// StartCondition spans a plan condition step.
func StartCondition(ctx context.Context, id, name string) (context.Context, trace.Span) {
	return start(ctx, "Condition", name,
		attribute.String(AttrStepType, "condition"),
		attribute.String(AttrID, id))
}

// StartResponse spans a plan response step.
func StartResponse(ctx context.Context, id, name string) (context.Context, trace.Span) {
	return start(ctx, "Response", name,
		attribute.String(AttrStepType, "response"),
		attribute.String(AttrID, id))
}

// StartWorkflowExecute spans a workflow invoked through a trigger (e.g. callworkflow).
// name is the workflow's friendly display name and id its stable config id.
func StartWorkflowExecute(ctx context.Context, name, id string) (context.Context, trace.Span) {
	return start(ctx, "Workflow Execute", rootDisplay(name, id),
		attribute.String(AttrStepType, "trigger"),
		attribute.String(AttrWorkflow, id))
}

// StartScheduledExecution spans a scheduled (cron) workflow run.
// name is the workflow's friendly display name and id its stable config id.
func StartScheduledExecution(ctx context.Context, name, id string) (context.Context, trace.Span) {
	return start(ctx, "Scheduled Execution", rootDisplay(name, id),
		attribute.String(AttrStepType, "scheduled"),
		attribute.String(AttrWorkflow, id))
}

// StartDashboardRun spans a manual workflow run triggered from the builder dashboard.
// name is the workflow's friendly display name and id its stable config id.
func StartDashboardRun(ctx context.Context, name, id string) (context.Context, trace.Span) {
	return start(ctx, "Dashboard Run", rootDisplay(name, id),
		attribute.String(AttrStepType, "request"),
		attribute.String(AttrWorkflow, id))
}

// StartAgentInvoke spans a whole agent-action run (the GenAI invoke_agent
// operation). Its child chat spans are created at the integration boundary.
func StartAgentInvoke(ctx context.Context, name string) (context.Context, trace.Span) {
	display := name
	if display == "" {
		display = opInvokeAgent
	}
	attrs := []attribute.KeyValue{attribute.String(AttrGenAIOperation, opInvokeAgent)}
	if name != "" {
		attrs = append(attrs, attribute.String(AttrGenAIAgentName, name))
	}
	return start(ctx, "invoke_agent", display, attrs...)
}

// StartMCPTool spans the invocation of an MCP tool. Carries both the sf.* tool
// keys and the GenAI execute_tool attributes.
func StartMCPTool(ctx context.Context, name string) (context.Context, trace.Span) {
	return start(ctx, "MCP Tool", name,
		attribute.String(AttrToolName, name),
		attribute.String(AttrToolType, "mcp"),
		attribute.String(AttrGenAIOperation, opExecuteTool),
		attribute.String(AttrGenAIToolName, name),
		attribute.String(AttrGenAIToolType, "mcp"))
}

// StartAgentTool spans the invocation of an agent workflow tool. Carries both
// the sf.* tool keys and the GenAI execute_tool attributes.
func StartAgentTool(ctx context.Context, identifier string) (context.Context, trace.Span) {
	return start(ctx, "Tool Call", identifier,
		attribute.String(AttrToolName, identifier),
		attribute.String(AttrToolType, "workflow"),
		attribute.String(AttrGenAIOperation, opExecuteTool),
		attribute.String(AttrGenAIToolName, identifier),
		attribute.String(AttrGenAIToolType, "workflow"))
}
