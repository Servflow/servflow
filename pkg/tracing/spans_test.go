package tracing

import (
	"context"
	"errors"
	"testing"

	"strings"

	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/Servflow/servflow/pkg/engine/secrets"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func TestSetHTTPStatus(t *testing.T) {
	cases := []struct {
		name       string
		code       int
		err        error
		wantStatus codes.Code
	}{
		{"2xx leaves status unset", 200, nil, codes.Unset},
		{"4xx is a client error, stays unset", 404, nil, codes.Unset},
		{"5xx marks the span errored", 500, errors.New("boom"), codes.Error},
		{"5xx without err still errors", 503, nil, codes.Error},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sr := tracetest.NewSpanRecorder()
			otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr)))
			tracer = otel.Tracer("servflow-test")

			_, span := StartHTTPEntry(context.Background(), "Test Workflow", "test-workflow")
			SetHTTPStatus(span, tc.code, tc.err)
			span.End()

			ended := sr.Ended()
			if len(ended) != 1 {
				t.Fatalf("expected 1 span, got %d", len(ended))
			}
			if got := ended[0].Status().Code; got != tc.wantStatus {
				t.Errorf("status = %v, want %v", got, tc.wantStatus)
			}
			if got := attrMap(ended[0].Attributes())["sf.http.status_code"]; got != int64(tc.code) {
				t.Errorf("sf.http.status_code = %v, want %d", got, tc.code)
			}
		})
	}
}

func TestSetHTTPStatusNilSpanSafe(t *testing.T) {
	// Must not panic when tracing is disabled and the span is nil.
	SetHTTPStatus(nil, 500, errors.New("x"))
}

// TestWorkflowRootSpans checks that every workflow entry-point span carries the
// workflow's friendly name as sf.name (rendered by the trace viewer) and its
// stable config id as sf.workflow (used to group/search traces by workflow).
func TestWorkflowRootSpans(t *testing.T) {
	cases := []struct {
		name  string
		start func(context.Context) (context.Context, trace.Span)
	}{
		{"http", func(ctx context.Context) (context.Context, trace.Span) {
			return StartHTTPEntry(ctx, "My Workflow", "my-workflow")
		}},
		{"trigger", func(ctx context.Context) (context.Context, trace.Span) {
			return StartWorkflowExecute(ctx, "My Workflow", "my-workflow", "trigger")
		}},
		{"scheduled", func(ctx context.Context) (context.Context, trace.Span) {
			return StartScheduledExecution(ctx, "My Workflow", "my-workflow")
		}},
		{"dashboard", func(ctx context.Context) (context.Context, trace.Span) {
			return StartDashboardRun(ctx, "My Workflow", "my-workflow")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sr := tracetest.NewSpanRecorder()
			otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr)))
			tracer = otel.Tracer("servflow-test")

			_, span := tc.start(context.Background())
			span.End()

			attrs := attrMap(sr.Ended()[0].Attributes())
			if got := attrs[AttrName]; got != "My Workflow" {
				t.Errorf("%s = %v, want %q", AttrName, got, "My Workflow")
			}
			if got := attrs[AttrWorkflow]; got != "my-workflow" {
				t.Errorf("%s = %v, want %q", AttrWorkflow, got, "my-workflow")
			}
		})
	}
}

// TestWorkflowRootSpanNameFallsBackToID verifies the display label falls back to
// the config id when a workflow has no friendly name set.
func TestWorkflowRootSpanNameFallsBackToID(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr)))
	tracer = otel.Tracer("servflow-test")

	_, span := StartHTTPEntry(context.Background(), "", "my-workflow")
	span.End()

	if got := attrMap(sr.Ended()[0].Attributes())[AttrName]; got != "my-workflow" {
		t.Errorf("%s = %v, want %q", AttrName, got, "my-workflow")
	}
}

// TestSpansScrubTrackedSecrets verifies the scrub-gateway contract for traces:
// when the context carries a RequestContext with tracked secrets, spans
// created through this package mask those values in string attributes,
// recorded errors and status descriptions.
func TestSpansScrubTrackedSecrets(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr)))
	tracer = otel.Tracer("servflow-test")

	secrets.Reset()
	t.Cleanup(secrets.Reset)
	t.Setenv("SPAN_TOKEN", "spansecretvalue")

	rc := requestctx.NewRequestContext("span-test")
	ctx := requestctx.WithAggregationContext(context.Background(), rc)
	if _, err := rc.Resolve(ctx, `{{ secret "SPAN_TOKEN" }}`); err != nil {
		t.Fatal(err)
	}

	ctx, span := StartAction(ctx, "a1", "my action", "http")
	span.SetAttributes(attribute.String("sf.config", "url?token=spansecretvalue"))
	span.RecordError(errors.New("dial failed: spansecretvalue rejected"))
	span.SetStatus(codes.Error, "boom spansecretvalue")

	// Spans fetched back out of the context must be wrapped too.
	fromCtx := trace.SpanFromContext(ctx)
	fromCtx.SetAttributes(attribute.String("extra", "x spansecretvalue y"))

	span.End()

	ended := sr.Ended()
	if len(ended) != 1 {
		t.Fatalf("expected 1 span, got %d", len(ended))
	}
	got := ended[0]

	if v, _ := attrMap(got.Attributes())["sf.config"].(string); strings.Contains(v, "spansecretvalue") {
		t.Errorf("sf.config not scrubbed: %q", v)
	}
	if v, _ := attrMap(got.Attributes())["extra"].(string); strings.Contains(v, "spansecretvalue") {
		t.Errorf("SpanFromContext attribute not scrubbed: %q", v)
	}
	if d := got.Status().Description; strings.Contains(d, "spansecretvalue") {
		t.Errorf("status description not scrubbed: %q", d)
	}
	for _, ev := range got.Events() {
		for _, a := range ev.Attributes {
			if a.Value.Type() == attribute.STRING && strings.Contains(a.Value.AsString(), "spansecretvalue") {
				t.Errorf("recorded error not scrubbed: %q", a.Value.AsString())
			}
		}
	}
}

// TestAgentNodeSpanAttributes covers the two things the agent-node constructor
// decides: the GenAI operation it reports, which a backend reads to classify
// the span, and the label an agent with no name of its own falls back to.
func TestAgentNodeSpanAttributes(t *testing.T) {
	cases := []struct {
		name        string
		agent       string
		wantDisplay string
	}{
		{"named agent labels the span", "Responder", "Responder"},
		{"unnamed agent falls back to the span name", "", "AgentCall"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sr := tracetest.NewSpanRecorder()
			otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr)))
			tracer = otel.Tracer("servflow-test")

			_, span := StartAgentInvoke(context.Background(), tc.agent)
			span.End()

			attrs := attrMap(sr.Ended()[0].Attributes())
			if got := attrs[AttrGenAIOperation]; got != opInvokeAgent {
				t.Errorf("%s = %v, want %q", AttrGenAIOperation, got, opInvokeAgent)
			}
			if got := attrs[AttrName]; got != tc.wantDisplay {
				t.Errorf("%s = %v, want %q", AttrName, got, tc.wantDisplay)
			}
		})
	}
}

// TestRequestTotalsLandOnTheEntrySpan holds the rule that one request has one
// root span: the entry span is the span the lifecycle ends and stamps the token
// totals on, however many tool calls run underneath it. A tool constructor that
// binds itself breaks this by replacing the entry span, which then never ends
// and never exports.
func TestRequestTotalsLandOnTheEntrySpan(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr)))
	tracer = otel.Tracer("servflow-test")

	ctx, rc := requestctx.Start(context.Background(), requestctx.Options{ID: "req-mcp"})
	ctx, _ = StartHTTPEntry(ctx, "My Workflow", "wf-id")

	// A tool call one level down, ended by its own caller.
	_, toolSpan := StartMCPTool(ctx, "search")
	rc.AddTokenUsage(10, 5)
	toolSpan.End()

	// The response is written; the lifecycle ends the span it owns.
	rc.Done()

	root := endedByName(t, sr.Ended(), "HTTP Entry")
	if got := attrMap(root.Attributes())[AttrUsageTotal]; got != int64(15) {
		t.Errorf("%s = %v, want 15 on the entry span", AttrUsageTotal, got)
	}

	tool := endedByName(t, sr.Ended(), "MCP Tool")
	if _, ok := attrMap(tool.Attributes())[AttrUsageTotal]; ok {
		t.Errorf("%s must not land on a tool span", AttrUsageTotal)
	}
}
