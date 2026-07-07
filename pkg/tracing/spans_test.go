package tracing

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
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

			_, span := StartHTTPEntry(context.Background())
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
