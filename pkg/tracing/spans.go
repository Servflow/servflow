package tracing

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Standard servflow span attribute keys. Everything is namespaced under "sf."
// to stay clear of OpenTelemetry semantic conventions. Span names are kept
// low-cardinality (the step class); the per-instance label lives in AttrName.
const (
	AttrName       = "sf.name"        // friendly per-instance label rendered by dashboards
	AttrStepType   = "sf.step.type"   // request | action | condition | response | trigger | scheduled
	AttrActionType = "sf.action_type" // concrete action type (http, callworkflow, parallel, ...)
	AttrID         = "sf.id"          // bare node id (no prefix)
	AttrToolName   = "sf.tool_name"
	AttrToolType   = "sf.tool_type"   // mcp | workflow
	AttrToolParams = "sf.tool_params" // JSON of model-supplied tool-call arguments (sensitive keys redacted, size-capped)
)

// start creates a span with a low-cardinality name and always attaches the
// friendly display label as the AttrName attribute. All typed constructors
// below funnel through here so no span can be created without a label.
func start(ctx context.Context, spanName, display string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	ctx, span := tracer.Start(ctx, spanName)
	span.SetAttributes(append(attrs, attribute.String(AttrName, display))...)
	return ctx, span
}

// StartHTTPEntry spans the HTTP request entry point.
func StartHTTPEntry(ctx context.Context) (context.Context, trace.Span) {
	return start(ctx, "HTTP Entry", "HTTP Entry",
		attribute.String(AttrStepType, "request"))
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
func StartWorkflowExecute(ctx context.Context, name string) (context.Context, trace.Span) {
	return start(ctx, "Workflow Execute", name,
		attribute.String(AttrStepType, "trigger"))
}

// StartScheduledExecution spans a scheduled (cron) workflow run.
func StartScheduledExecution(ctx context.Context, name string) (context.Context, trace.Span) {
	return start(ctx, "Scheduled Execution", name,
		attribute.String(AttrStepType, "scheduled"))
}

// StartDashboardRun spans a manual workflow run triggered from the builder dashboard.
func StartDashboardRun(ctx context.Context, name string) (context.Context, trace.Span) {
	return start(ctx, "Dashboard Run", name,
		attribute.String(AttrStepType, "request"))
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
