package tracing

import (
	"context"
	"os"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// GenAI semantic-convention attribute keys. These follow the OpenTelemetry
// GenAI conventions (experimental as of semconv v1.41) and are used ONLY on the
// agent-action subtree (invoke_agent -> chat -> execute_tool). Structural
// workflow spans keep the sf.* keys defined in spans.go.
const (
	AttrGenAIOperation     = "gen_ai.operation.name" // chat | invoke_agent | execute_tool
	AttrGenAIProvider      = "gen_ai.provider.name"  // anthropic | openai
	AttrGenAIRequestModel  = "gen_ai.request.model"  // model requested
	AttrGenAIResponseModel = "gen_ai.response.model" // model that served the response
	AttrGenAIUsageInput    = "gen_ai.usage.input_tokens"
	AttrGenAIUsageOutput   = "gen_ai.usage.output_tokens"
	AttrGenAITokenType     = "gen_ai.token.type" // input | output (metric dimension)
	AttrGenAIToolName      = "gen_ai.tool.name"
	AttrGenAIToolType      = "gen_ai.tool.type" // mcp | workflow
	AttrGenAIAgentName     = "gen_ai.agent.name"

	// Message-content keys, recorded only when content capture is enabled.
	AttrGenAISystemInstr    = "gen_ai.system_instructions"
	AttrGenAIInputMessages  = "gen_ai.input.messages"
	AttrGenAIOutputMessages = "gen_ai.output.messages"
)

// captureContent gates recording of prompt/response CONTENT onto chat spans.
// Off by default (content may hold PII/secrets and is large); enable per the
// OTel-standard env var. Read once at init; tests may set it directly.
var captureContent = os.Getenv("OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT") == "true"

// maxContentBytes caps each captured content attribute so a large context
// window cannot bloat every span.
const maxContentBytes = 8 * 1024

func truncateContent(s string) string {
	if len(s) <= maxContentBytes {
		return s
	}
	return s[:maxContentBytes] + "…[truncated]"
}

const (
	opChat        = "chat"
	opInvokeAgent = "invoke_agent"
	opExecuteTool = "execute_tool"
)

// GenAI "floor" metrics per the OTel spec. Bound to the concrete MeterProvider
// by initGenAIInstruments during InitTracer; nil (and inert) until then.
var (
	tokenUsageHist metric.Int64Histogram
	opDurationHist metric.Float64Histogram
)

// initGenAIInstruments creates the GenAI metric instruments from the given
// provider. InitTracer calls this with the concrete MeterProvider so the
// instruments bind directly to it, rather than relying on global delegation.
func initGenAIInstruments(mp metric.MeterProvider) {
	m := mp.Meter("servflow")
	tokenUsageHist, _ = m.Int64Histogram(
		"gen_ai.client.token.usage",
		metric.WithUnit("{token}"),
		metric.WithDescription("Number of tokens used per GenAI request, split by token type."),
	)
	opDurationHist, _ = m.Float64Histogram(
		"gen_ai.client.operation.duration",
		metric.WithUnit("s"),
		metric.WithDescription("Duration of a GenAI client operation."),
	)
}

// Inference brackets a single LLM model call: it owns the "chat" span and the
// timing for the operation-duration metric. Create one at the integration
// boundary via StartInference, then SetResponseModel/RecordUsage on success and
// End (always, via defer) to close the span and emit the duration metric.
type Inference struct {
	span     trace.Span
	provider string
	model    string
	start    time.Time
}

// StartInference opens a GenAI "chat" span around one model call and starts the
// duration clock. provider is the semconv gen_ai.provider.name value
// ("anthropic", "openai"); model is the requested model id.
func StartInference(ctx context.Context, provider, model string) (context.Context, *Inference) {
	ctx, span := start(ctx, "chat", provider+" "+model,
		attribute.String(AttrGenAIOperation, opChat),
		attribute.String(AttrGenAIProvider, provider),
		attribute.String(AttrGenAIRequestModel, model),
	)
	return ctx, &Inference{span: span, provider: provider, model: model, start: time.Now()}
}

// SetResponseModel records the model that actually served the response.
func (i *Inference) SetResponseModel(model string) {
	if model != "" {
		i.span.SetAttributes(attribute.String(AttrGenAIResponseModel, model))
	}
}

// SetInput records the system instructions and input messages on the chat span.
// No-op unless content capture is enabled; each field is size-capped.
func (i *Inference) SetInput(system, messages string) {
	if !captureContent {
		return
	}
	attrs := make([]attribute.KeyValue, 0, 2)
	if system != "" {
		attrs = append(attrs, attribute.String(AttrGenAISystemInstr, truncateContent(system)))
	}
	if messages != "" {
		attrs = append(attrs, attribute.String(AttrGenAIInputMessages, truncateContent(messages)))
	}
	if len(attrs) > 0 {
		i.span.SetAttributes(attrs...)
	}
}

// SetCompletion records the model's response content on the chat span. No-op
// unless content capture is enabled; size-capped.
func (i *Inference) SetCompletion(output string) {
	if !captureContent || output == "" {
		return
	}
	i.span.SetAttributes(attribute.String(AttrGenAIOutputMessages, truncateContent(output)))
}

// RecordUsage sets the token-usage span attributes and emits the token-usage
// metric once per token type.
func (i *Inference) RecordUsage(ctx context.Context, input, output int64) {
	i.span.SetAttributes(
		attribute.Int64(AttrGenAIUsageInput, input),
		attribute.Int64(AttrGenAIUsageOutput, output),
	)
	addTokens(ctx, input, output)
	if tokenUsageHist == nil {
		return
	}
	tokenUsageHist.Record(ctx, input, metric.WithAttributes(
		attribute.String(AttrGenAIProvider, i.provider),
		attribute.String(AttrGenAIRequestModel, i.model),
		attribute.String(AttrGenAITokenType, "input"),
	))
	tokenUsageHist.Record(ctx, output, metric.WithAttributes(
		attribute.String(AttrGenAIProvider, i.provider),
		attribute.String(AttrGenAIRequestModel, i.model),
		attribute.String(AttrGenAITokenType, "output"),
	))
}

// End records the operation-duration metric, marks the span's error status when
// err is non-nil, and ends the span. Safe to call via defer.
func (i *Inference) End(ctx context.Context, err error) {
	if err != nil {
		i.span.RecordError(err)
		i.span.SetStatus(codes.Error, err.Error())
	}
	if opDurationHist != nil {
		opDurationHist.Record(ctx, time.Since(i.start).Seconds(), metric.WithAttributes(
			attribute.String(AttrGenAIOperation, opChat),
			attribute.String(AttrGenAIProvider, i.provider),
			attribute.String(AttrGenAIRequestModel, i.model),
		))
	}
	i.span.End()
}
