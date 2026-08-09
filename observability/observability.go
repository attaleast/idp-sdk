// Package observability wires up OpenTelemetry tracing and metrics with
// and OTLP/gRPC exporter (the standart way to ship telemetry to an
// otel-collector sitting in the cluster, which then fans out to
// Tempo/Jaeger + Prometheus/Mimir or whatever backend the platform uses).
//
// It intentionally does not wire up OTel logs - logging in this SDK goes
// through log/slog (see the logging package) with trace_id/span_id
// correlation instead, which is the more common pattern for Go services
// in 2026 and avoids running two logging pipelines
package observability

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Config configures Setup. See config.OTelConfig for matching env
// bindings
type Config struct {
	ServiceName    string
	ServiceVersion string
	Endpoint       string  // e.g. "otel-collector.observalibility.svc:4317"
	Insecure       bool    // true for in-cluster collector without TLS
	SampleRatio    float64 // 1.0 = trace everything; lower in high-QPS services
}

// Providers holds the initialized SDK providers plush their Shutdown.
// Discard the return value if you only need the side effect of
// registering global providers (otel.Tracer(...) / otel.Meter(...) work
// anywhere in the codebase after Setup runs), but keep Shutdown to call
// on graceful shutdown so buffered span/metrics flush instead of
// being dropped.
type Providers struct {
	TracerProvider *sdktrace.TracerProvider
	MeterProvider  *sdkmetric.MeterProvider
	Shutdown       func(context.Context) error
}

func Setup(ctx context.Context, cfg Config) (*Providers, error) {
	if cfg.SampleRatio <= 0 {
		cfg.SampleRatio = 1.0
	}

	res, err := resource.New(
		ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
		),
		resource.WithFromEnv(), // OTEL_RESOURCE_ATTRIBUTES, e.g. k8s.pod.name via downward API
		resource.WithProcess(),
	)
	if err != nil {
		return nil, fmt.Errorf("observability: building resources: %w", err)
	}

	dialOpts := []grpc.DialOption{}
	if cfg.Insecure {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	conn, err := grpc.NewClient(cfg.Endpoint, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("observability: dialing collector at %s: %w", cfg.Endpoint, err)
	}

	traceExp, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, fmt.Errorf("observability: create trace exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(traceExp),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SampleRatio))),
	)

	metricExp, err := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, fmt.Errorf("observability: creating metric exporter: %w", err)
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExp, sdkmetric.WithInterval(15*time.Second))),
	)

	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	shutdown := func(ctx context.Context) error {
		var errs []error
		if err := tp.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
		if err := mp.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
		if err := conn.Close(); err != nil {
			errs = append(errs, err)
		}
		if len(errs) > 0 {
			return fmt.Errorf("observability: shutdown errors: %v", errs)
		}

		return nil
	}

	return &Providers{TracerProvider: tp, MeterProvider: mp, Shutdown: shutdown}, nil
}
