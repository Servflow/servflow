package tracing

import (
	"context"
	"errors"
	"fmt"
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

	AttrAgentTurn          = "sf.agent.turn"           // 1-based iteration of the agent's tool-calling loop
	AttrAgentTurnTools     = "sf.agent.turn_tools"     // tool names the model asked for on this turn
	AttrAgentToolsWithheld = "sf.agent.tools_withheld" // tools were withheld to force a final answer

	AttrRequestID = requestctx.AttrRequestID // stamped on every span via the rc
)

// createSpan creates a span with a low-cardinality name and always attaches the
// friendly display label as the AttrName attribute. All typed constructors
// below funnel through here so no span can be created without a label — and,
// when the context carries a RequestContext, without secret scrubbing.
func createSpan(ctx context.Context, spanName, display string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	ctx, span := tracer.Start(ctx, spanName)
	if rc, ok := requestctx.FromContext(ctx); ok {
		span = scrubSpan{Span: span, s: rc}
		// Re-store the wrapped span so trace.SpanFromContext(ctx) callers also
		// get the scrubbing wrapper, not the raw span.
		ctx = trace.ContextWithSpan(ctx, span)
		// The request's attributes go on every span it creates, not on the root
		// alone: the trace backend matches per span, so identity carried only by
		// the root cannot narrow a search for one agent's model calls.
		attrs = append(attrs, rc.SpanAttributes()...)
	}
	span.SetAttributes(append(attrs, attribute.String(AttrName, display))...)
	return ctx, span
}

// bindRootLifecycle hands a root entry span to the request lifecycle: the
// RequestContext defers its End until the main flow AND all child flows
// complete, and stamps the request token totals right before ending. Without
// an rc in ctx (tests, tracing-disabled callers) the caller keeps manual End
// responsibility.
//
// It decorates nothing. The request's attributes reach this span through
// createSpan, the same way they reach every other span of the request.
func bindRootLifecycle(ctx context.Context, span trace.Span) {
	if rc, ok := requestctx.FromContext(ctx); ok {
		rc.BindRootSpan(span, func(s trace.Span) { stampRequestTokens(rc, s) })
	}
}

// Span names for the two agent-shaped spans. spanAgentCall is a whole run of
// an agent's workflow and the stem every entry-qualified variant is built
// from; spanAgentCallNode is one agent node inside such a run.
const (
	spanAgentCall     = "Agent Call"
	spanAgentCallNode = "AgentCall"
)

// entrySpanName qualifies the agent-call span with the entry that started the
// run, so a trace distinguishes a webhook delivery from a sub-workflow call
// without opening the span. The entry set is fixed by the host, so this stays
// low-cardinality.
func entrySpanName(entry string) string {
	if entry == "" {
		return spanAgentCall
	}
	return spanAgentCall + ": " + entry
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
	ctx, span := createSpan(ctx, "HTTP Entry", rootDisplay(name, id),
		attribute.String(AttrStepType, "request"),
		attribute.String(AttrWorkflow, id))
	bindRootLifecycle(ctx, span)
	return ctx, span
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
	return createSpan(ctx, "Action", name,
		attribute.String(AttrStepType, "action"),
		attribute.String(AttrActionType, actionType),
		attribute.String(AttrID, id))
}

// StartCondition spans a plan condition step.
func StartCondition(ctx context.Context, id, name string) (context.Context, trace.Span) {
	return createSpan(ctx, "Condition", name,
		attribute.String(AttrStepType, "condition"),
		attribute.String(AttrID, id))
}

// StartResponse spans a plan response step.
func StartResponse(ctx context.Context, id, name string) (context.Context, trace.Span) {
	return createSpan(ctx, "Response", name,
		attribute.String(AttrStepType, "response"),
		attribute.String(AttrID, id))
}

// StartWorkflowExecute spans a workflow invoked through a trigger (e.g. callworkflow).
// name is the workflow's friendly display name and id its stable config id.
//
// entry names the door the run came through, which the host decides and the
// engine cannot know, so the span says which kind of call this was rather than
// naming the machinery that ran it. An empty entry leaves the bare span name.
func StartWorkflowExecute(ctx context.Context, name, id, entry string) (context.Context, trace.Span) {
	ctx, span := createSpan(ctx, entrySpanName(entry), rootDisplay(name, id),
		attribute.String(AttrStepType, "trigger"),
		attribute.String(AttrWorkflow, id))
	bindRootLifecycle(ctx, span)
	return ctx, span
}

// StartScheduledExecution spans a scheduled (cron) workflow run.
// name is the workflow's friendly display name and id its stable config id.
func StartScheduledExecution(ctx context.Context, name, id string) (context.Context, trace.Span) {
	ctx, span := createSpan(ctx, "Scheduled Execution", rootDisplay(name, id),
		attribute.String(AttrStepType, "scheduled"),
		attribute.String(AttrWorkflow, id))
	bindRootLifecycle(ctx, span)
	return ctx, span
}

// StartDashboardRun spans a manual workflow run triggered from the builder dashboard.
// name is the workflow's friendly display name and id its stable config id.
func StartDashboardRun(ctx context.Context, name, id string) (context.Context, trace.Span) {
	ctx, span := createSpan(ctx, "Dashboard Run", rootDisplay(name, id),
		attribute.String(AttrStepType, "request"),
		attribute.String(AttrWorkflow, id))
	bindRootLifecycle(ctx, span)
	return ctx, span
}

// StartAgentInvoke spans a whole agent-action run (the GenAI invoke_agent
// operation). Its child model-call spans are created at the provider boundary.
//
// The span is named for what it is rather than for the GenAI operation it
// reports. gen_ai.operation.name still carries invoke_agent, so a backend that
// reads the conventions classifies it the same as before.
func StartAgentInvoke(ctx context.Context, name string) (context.Context, trace.Span) {
	display := name
	if display == "" {
		display = spanAgentCallNode
	}
	attrs := []attribute.KeyValue{attribute.String(AttrGenAIOperation, opInvokeAgent)}
	if name != "" {
		attrs = append(attrs, attribute.String(AttrGenAIAgentName, name))
	}
	return createSpan(ctx, spanAgentCallNode, display, attrs...)
}

// StartAgentTurn spans one iteration of the agent's tool-calling loop: a single
// model call plus the tools that call asked for. It groups them, so a trace
// shows which tool runs belonged to which model call instead of leaving both
// flat under invoke_agent.
//
// It carries no gen_ai.operation.name. A turn is wider than an inference and
// narrower than the run, so it matches no GenAI operation, and claiming one
// would misfile it for any backend that reads the conventions.
//
// The turn number labels the span; its child chat span carries the model and
// the tokens, so those are deliberately not repeated here.
func StartAgentTurn(ctx context.Context, turn int) (context.Context, trace.Span) {
	return createSpan(ctx, "Agent Turn", fmt.Sprintf("turn %d", turn),
		attribute.Int(AttrAgentTurn, turn))
}

// StartMCPTool spans the invocation of an MCP tool. Carries both the sf.* tool
// keys and the GenAI execute_tool attributes.
//
// The span is a child, which is what an MCP tool called from inside an agent
// run is. A host whose whole request IS the tool call wants StartMCPEntry: the
// lifecycle holds one root span per request, so binding this one would replace
// the entry span and leave it unended, and therefore unexported.
func StartMCPTool(ctx context.Context, name string) (context.Context, trace.Span) {
	return createSpan(ctx, "MCP Tool", name,
		attribute.String(AttrToolName, name),
		attribute.String(AttrToolType, "mcp"),
		attribute.String(AttrGenAIOperation, opExecuteTool),
		attribute.String(AttrGenAIToolName, name),
		attribute.String(AttrGenAIToolType, "mcp"))
}

// StartMCPEntry spans an MCP tool call that is itself the request, as it is for
// a server whose callers speak MCP and nothing else. The span is the run's
// root, so the request lifecycle owns its End and stamps the token totals on
// it.
func StartMCPEntry(ctx context.Context, name string) (context.Context, trace.Span) {
	ctx, span := StartMCPTool(ctx, name)
	bindRootLifecycle(ctx, span)
	return ctx, span
}

// StartAgentTool spans the invocation of an agent workflow tool. Carries both
// the sf.* tool keys and the GenAI execute_tool attributes.
func StartAgentTool(ctx context.Context, identifier string) (context.Context, trace.Span) {
	return createSpan(ctx, "Tool Call", identifier,
		attribute.String(AttrToolName, identifier),
		attribute.String(AttrToolType, "workflow"),
		attribute.String(AttrGenAIOperation, opExecuteTool),
		attribute.String(AttrGenAIToolName, identifier),
		attribute.String(AttrGenAIToolType, "workflow"))
}
