package tracing

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestInferenceRecordsSpanAndMetrics(t *testing.T) {
	// Bind instruments to an in-memory MeterProvider + wire a span recorder.
	reader := metric.NewManualReader()
	initGenAIInstruments(metric.NewMeterProvider(metric.WithReader(reader)))

	sr := tracetest.NewSpanRecorder()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr)))
	tracer = otel.Tracer("servflow-test")

	ctx := context.Background()
	ctx, inf := StartInference(ctx, "anthropic", "claude-sonnet-4-5")
	inf.SetResponseModel("claude-sonnet-4-5-20250929")
	inf.RecordUsage(ctx, 120, 45)
	inf.End(ctx, nil)

	// --- span assertions ---
	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	attrs := attrMap(spans[0].Attributes())
	if got := attrs[AttrGenAIOperation]; got != opChat {
		t.Errorf("operation = %q, want %q", got, opChat)
	}
	if got := attrs[AttrGenAIProvider]; got != "anthropic" {
		t.Errorf("provider = %q, want anthropic", got)
	}
	if got := attrs[AttrGenAIRequestModel]; got != "claude-sonnet-4-5" {
		t.Errorf("request.model = %q", got)
	}
	if got := attrs[AttrGenAIResponseModel]; got != "claude-sonnet-4-5-20250929" {
		t.Errorf("response.model = %q", got)
	}
	if got := attrs[AttrGenAIUsageInput]; got != int64(120) {
		t.Errorf("usage.input = %v, want 120", got)
	}
	if got := attrs[AttrGenAIUsageOutput]; got != int64(45) {
		t.Errorf("usage.output = %v, want 45", got)
	}

	// --- metric assertions ---
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("collect: %v", err)
	}
	tokenPoints := map[string]int64{} // token.type -> value
	var durationCount uint64
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			switch m.Name {
			case "gen_ai.client.token.usage":
				h := m.Data.(metricdata.Histogram[int64])
				for _, dp := range h.DataPoints {
					tt, _ := dp.Attributes.Value(attribute.Key(AttrGenAITokenType))
					tokenPoints[tt.AsString()] = dp.Sum
				}
			case "gen_ai.client.operation.duration":
				h := m.Data.(metricdata.Histogram[float64])
				for _, dp := range h.DataPoints {
					durationCount += dp.Count
				}
			}
		}
	}
	if tokenPoints["input"] != 120 {
		t.Errorf("token.usage input = %d, want 120", tokenPoints["input"])
	}
	if tokenPoints["output"] != 45 {
		t.Errorf("token.usage output = %d, want 45", tokenPoints["output"])
	}
	if durationCount != 1 {
		t.Errorf("operation.duration count = %d, want 1", durationCount)
	}
}

func TestInferenceEndRecordsError(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr)))
	tracer = otel.Tracer("servflow-test")

	ctx := context.Background()
	_, inf := StartInference(ctx, "openai", "gpt-4.1")
	inf.End(ctx, errors.New("boom"))

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Status().Code.String() != "Error" {
		t.Errorf("status = %v, want Error", spans[0].Status().Code)
	}
}

func attrMap(kvs []attribute.KeyValue) map[string]interface{} {
	out := make(map[string]interface{}, len(kvs))
	for _, kv := range kvs {
		out[string(kv.Key)] = kv.Value.AsInterface()
	}
	return out
}
