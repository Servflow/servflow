package tracing

import (
	"context"
	"testing"

	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// endedByName returns the single ended span with the given SpanName.
func endedByName(t *testing.T, spans []sdktrace.ReadOnlySpan, name string) sdktrace.ReadOnlySpan {
	t.Helper()
	var found sdktrace.ReadOnlySpan
	for _, s := range spans {
		if s.Name() == name {
			require.Nil(t, found, "more than one %q span exported", name)
			found = s
		}
	}
	require.NotNil(t, found, "no %q span exported", name)
	return found
}

// TestLifecycleBinding_RootEndsAfterFlow proves the rc lifecycle owns the root
// span end: a root bound via StartHTTPEntry stays un-ended after Done while a
// child flow is open, then ends exactly once when the flow drains — covering
// request-wide attr stamping on the root only (not children), token totals
// stamped at end including late tokens, and root ⊇ child in time.
func TestLifecycleBinding_RootEndsAfterFlow(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr)))
	tracer = otel.Tracer("servflow-test")

	// host-supplied request-wide attribute, like pro's sf.agent.
	ctx, rc := requestctx.Start(context.Background(), requestctx.Options{
		ID:             "req-123",
		SpanAttributes: []attribute.KeyValue{attribute.String("sf.agent", "AcctBot")},
	})
	ctx, _ = StartHTTPEntry(ctx, "My Workflow", "wf-id")

	// a child action span within the request.
	_, childSpan := StartAction(ctx, "a1", "greet", "stub")
	childSpan.End()

	// child already ended, root not yet (main flow still running).
	assert.Len(t, sr.Ended(), 1, "only the child span has ended")

	// a background dispatch chain registers a flow.
	endFlow := rc.BeginFlow("dispatch:x")

	// main flow completes (response written).
	rc.Done()
	assert.Len(t, sr.Ended(), 1, "root must not end while the flow is open")

	// background work adds tokens AFTER the response.
	rc.AddTokenUsage(100, 50)

	// flow drains → root ends now.
	endFlow()

	ended := sr.Ended()
	root := endedByName(t, ended, "HTTP Entry")
	child := endedByName(t, ended, "Action")

	rootAttrs := attrMap(root.Attributes())
	// Request id + host attr live on the ROOT span only.
	assert.Equal(t, "req-123", rootAttrs[requestctx.AttrRequestID])
	assert.Equal(t, "AcctBot", rootAttrs["sf.agent"])
	// Child spans do NOT carry the request-wide attributes.
	childAttrs := attrMap(child.Attributes())
	assert.NotContains(t, childAttrs, requestctx.AttrRequestID)
	assert.NotContains(t, childAttrs, "sf.agent")

	// Late tokens landed on the root (stamped by the beforeEnd hook).
	assert.Equal(t, int64(150), rootAttrs[AttrUsageTotal])
	assert.Equal(t, int64(100), rootAttrs[AttrUsageInput])
	assert.Equal(t, int64(50), rootAttrs[AttrUsageOutput])

	// Root temporally covers the child (deferred End = true total).
	assert.GreaterOrEqual(t, root.EndTime().UnixNano(), child.EndTime().UnixNano(),
		"root EndTime >= child EndTime")
}
