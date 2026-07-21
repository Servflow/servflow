package tracing

import (
	"context"

	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Request-aggregate token attribute keys. These sum token usage across every
// model call in one request/execution and are deliberately sf.* namespaced:
// the gen_ai.usage.* keys are scoped by the spec to a single operation, so
// reusing them for an aggregate would misrepresent the convention.
//
// The running total lives on the per-request RequestContext (one canonical
// request-scoped store), not a separate context value. Dispatched background
// chains share the same RequestContext, so their usage folds into the same
// request total rather than being reset.
const (
	AttrUsageInput  = "sf.usage.input_tokens"
	AttrUsageOutput = "sf.usage.output_tokens"
	AttrUsageTotal  = "sf.usage.total_tokens"
)

// addTokens adds usage to the request-level token counter in ctx, if a request
// context is present. Called from Inference.RecordUsage.
func addTokens(ctx context.Context, input, output int64) {
	if rc, ok := requestctx.FromContext(ctx); ok {
		rc.AddTokenUsage(input, output)
	}
}

// stampRequestTokens writes the request-level token totals onto span.
func stampRequestTokens(rc *requestctx.RequestContext, span trace.Span) {
	in, out := rc.TokenUsage()
	span.SetAttributes(
		attribute.Int64(AttrUsageInput, in),
		attribute.Int64(AttrUsageOutput, out),
		attribute.Int64(AttrUsageTotal, in+out),
	)
}

// Deprecated: root spans stamp token totals via the RequestContext lifecycle
// (BindRootSpan); no new call sites.
func SetRequestTokens(ctx context.Context, span trace.Span) {
	if span == nil {
		return
	}
	rc, ok := requestctx.FromContext(ctx)
	if !ok {
		return
	}
	stampRequestTokens(rc, span)
}
