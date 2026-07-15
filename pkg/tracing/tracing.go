package tracing

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("servflow")
var initialized bool

const (
	OrgIDKey = attribute.Key("org.id")
)

// Config controls how InitTracer wires the OTLP exporters and span resource.
//
// Ingest is OTLP/HTTP over CollectorEndpoint. Headers are attached to every
// export request (e.g. to carry a credential), and SpanAttributes stamps
// runtime-derived identity onto every span. Tenant identity may be carried
// either as a static "org.id" resource attribute (OrgID) or via SpanAttributes.
type Config struct {
	ServiceName       string
	OrgID             string
	CollectorEndpoint string
	Headers           map[string]string
	SpanAttributes    func() map[string]string
}

func GetTracer() trace.Tracer {
	return tracer
}

func SpanCtxFromContext(ctx context.Context, name string) (context.Context, trace.Span) {
	ctx, span := tracer.Start(ctx, name)
	if rc, ok := requestctx.FromContext(ctx); ok {
		span = scrubSpan{Span: span, s: rc}
		ctx = trace.ContextWithSpan(ctx, span)
	}
	return ctx, span
}

func OTELEnabled() bool {
	return initialized
}

// attributeStampProcessor stamps dynamic attributes onto every span as it
// starts, reading them from attrs on each OnStart. Spans created before a value
// is known simply carry nothing for it; every span after carries it. The api_v3
// trace query matches span attributes as well as resource attributes, so a
// span-level attribute is searchable exactly like a resource-level one.
type attributeStampProcessor struct {
	attrs func() map[string]string
}

func (p *attributeStampProcessor) OnStart(_ context.Context, s sdktrace.ReadWriteSpan) {
	if p.attrs == nil {
		return
	}
	for k, v := range p.attrs() {
		if v != "" {
			s.SetAttributes(attribute.String(k, v))
		}
	}
}

func (p *attributeStampProcessor) OnEnd(sdktrace.ReadOnlySpan)      {}
func (p *attributeStampProcessor) Shutdown(context.Context) error   { return nil }
func (p *attributeStampProcessor) ForceFlush(context.Context) error { return nil }

// InitTracer configures the global trace and metric providers from cfg and
// returns a shutdown func that flushes and closes both. Traces and metrics share
// the same transport and endpoint.
func InitTracer(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	traceExporter, metricExporter, err := buildExporters(ctx, cfg)
	if err != nil {
		return nil, err
	}

	attrs := []attribute.KeyValue{semconv.ServiceNameKey.String(cfg.ServiceName)}
	// Keep the legacy org.id resource attribute only when supplied. Deployments
	// that scope by team leave OrgID empty and rely on TeamIDFunc instead.
	if cfg.OrgID != "" {
		attrs = append(attrs, OrgIDKey.String(cfg.OrgID))
	}
	res, err := resource.New(ctx, resource.WithAttributes(attrs...))
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	tpOpts := []sdktrace.TracerProviderOption{
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	}
	if cfg.SpanAttributes != nil {
		tpOpts = append(tpOpts, sdktrace.WithSpanProcessor(&attributeStampProcessor{attrs: cfg.SpanAttributes}))
	}
	tp := sdktrace.NewTracerProvider(tpOpts...)

	// Metrics back the GenAI "floor" metrics (gen_ai.client.token.usage /
	// operation.duration).
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
	)

	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	tracer = otel.Tracer(cfg.ServiceName)
	initGenAIInstruments(mp)
	initialized = true

	return func(ctx context.Context) error {
		return errors.Join(tp.Shutdown(ctx), mp.Shutdown(ctx))
	}, nil
}

// buildExporters constructs the OTLP/HTTP trace and metric exporters. The
// /v1/traces and /v1/metrics signal paths are derived from CollectorEndpoint.
func buildExporters(ctx context.Context, cfg Config) (*otlptrace.Exporter, sdkmetric.Exporter, error) {
	traceOpts := []otlptracehttp.Option{
		otlptracehttp.WithEndpointURL(signalURL(cfg.CollectorEndpoint, "/v1/traces")),
	}
	metricOpts := []otlpmetrichttp.Option{
		otlpmetrichttp.WithEndpointURL(signalURL(cfg.CollectorEndpoint, "/v1/metrics")),
	}
	if len(cfg.Headers) > 0 {
		traceOpts = append(traceOpts, otlptracehttp.WithHeaders(cfg.Headers))
		metricOpts = append(metricOpts, otlpmetrichttp.WithHeaders(cfg.Headers))
	}

	traceExporter, err := otlptracehttp.New(ctx, traceOpts...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create http trace exporter: %w", err)
	}
	metricExporter, err := otlpmetrichttp.New(ctx, metricOpts...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create http metric exporter: %w", err)
	}
	return traceExporter, metricExporter, nil
}

// signalURL joins an OTLP base endpoint with a signal path. When the base
// already carries an explicit path (anything past the host), it is used verbatim
// so an operator can point a signal at an exact URL.
func signalURL(base, signalPath string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		return signalPath
	}
	trimmed := strings.TrimRight(base, "/")
	if i := strings.Index(trimmed, "://"); i >= 0 {
		if strings.Contains(trimmed[i+3:], "/") {
			return trimmed
		}
	}
	return trimmed + signalPath
}
