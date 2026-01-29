package tracing

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var tracer = otel.Tracer("servflow")
var initialized bool

const (
	OrgIDKey = attribute.Key("org.id")
)

func GetTracer() trace.Tracer {
	return tracer
}

func SpanCtxFromContext(ctx context.Context, name string) (context.Context, trace.Span) {
	return tracer.Start(ctx, name)
}

func OTELEnabled() bool {
	return initialized
}

func InitTracer(ctx context.Context, serviceName, orgID, collectorEndpoint string) (func(context.Context) error, error) {
	// Create and warm the gRPC connection
	conn, err := grpc.NewClient(collectorEndpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection: %w", err)
	}
	conn.Connect()

	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithGRPCConn(conn),
		otlptracegrpc.WithTimeout(5*time.Minute),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
			OrgIDKey.String(orgID),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	tracer = otel.Tracer(serviceName)
	initialized = true

	return tp.Shutdown, nil
}
