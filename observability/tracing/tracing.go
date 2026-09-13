// Package tracing provides OpenTelemetry span helpers for the framework.
//
// Every major framework operation — HTTP request handling, schema compilation,
// workflow triggers, driver queries, and migrations — wraps its work in a
// span. This gives operators a complete picture of latency distribution across
// the system without instrumenting individual modules.
//
// # Setup
//
//	tp, err := tracing.NewProvider(ctx, tracing.Config{
//	    ServiceName:    "awo-server",
//	    ServiceVersion: version.Version,
//	    Exporter:       tracing.ExporterOTLPGRPC, // or OTLPHTTP, Stdout
//	    Endpoint:       "localhost:4317",
//	})
//	if err != nil { ... }
//	defer tp.Shutdown(ctx)
//
// # Span creation
//
//	ctx, span := tracing.Start(ctx, "compiler.Compile")
//	defer span.End()
//	span.SetAttributes(tracing.EntityCount(n))
package tracing

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "awo.so/awo"

// ExporterKind selects the OTLP transport or stdout.
type ExporterKind string

const (
	// ExporterOTLPGRPC exports spans via OTLP/gRPC (default for production).
	ExporterOTLPGRPC ExporterKind = "otlp-grpc"

	// ExporterOTLPHTTP exports spans via OTLP/HTTP.
	ExporterOTLPHTTP ExporterKind = "otlp-http"

	// ExporterStdout writes human-readable span JSON to stdout.
	// Use for local development and debugging.
	ExporterStdout ExporterKind = "stdout"

	// ExporterNoop discards all spans. Use in tests.
	ExporterNoop ExporterKind = "noop"
)

// Config holds the tracing provider configuration.
type Config struct {
	// ServiceName is the logical name of this service in the trace backend.
	ServiceName string

	// ServiceVersion is embedded in every span (defaults to "dev").
	ServiceVersion string

	// Exporter selects the span transport. Defaults to ExporterNoop.
	Exporter ExporterKind

	// Endpoint is the OTLP collector address (e.g. "localhost:4317" for gRPC).
	// Required for ExporterOTLPGRPC and ExporterOTLPHTTP.
	Endpoint string

	// SampleRate controls the fraction of traces to sample (0.0–1.0).
	// 1.0 = always sample. 0.0 = never sample.
	// Default: 1.0.
	SampleRate float64
}

// Provider wraps the OTel SDK TracerProvider and provides a Shutdown hook.
type Provider struct {
	tp *sdktrace.TracerProvider
}

// NewProvider creates and registers a global OTel TracerProvider.
// Callers must defer Provider.Shutdown to flush pending spans.
func NewProvider(ctx context.Context, cfg Config) (*Provider, error) {
	if cfg.ServiceName == "" {
		cfg.ServiceName = "awo"
	}
	if cfg.ServiceVersion == "" {
		cfg.ServiceVersion = "dev"
	}
	if cfg.SampleRate == 0 {
		cfg.SampleRate = 1.0
	}

	exp, err := buildExporter(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("tracing.NewProvider: build exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			attribute.String("service.name", cfg.ServiceName),
			attribute.String("service.version", cfg.ServiceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("tracing.NewProvider: build resource: %w", err)
	}

	sampler := sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SampleRate))
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	otel.SetTracerProvider(tp)
	return &Provider{tp: tp}, nil
}

// Shutdown flushes buffered spans and releases exporter resources.
// Always call this before process exit.
func (p *Provider) Shutdown(ctx context.Context) error {
	return p.tp.Shutdown(ctx)
}

// Start begins a new span as a child of the span in ctx (if any).
// The returned context carries the new span — pass it to downstream calls.
//
//	ctx, span := tracing.Start(ctx, "handler.CreateInvoice")
//	defer span.End()
func Start(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return otel.Tracer(tracerName).Start(ctx, name, opts...)
}

// RecordError marks the span as errored and records err as a span event.
// If err is nil, this is a no-op.
func RecordError(span trace.Span, err error) {
	if err == nil {
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

// End is a convenience that calls span.End(). Use in defers where you need
// error recording first:
//
//	ctx, span := tracing.Start(ctx, "op")
//	defer tracing.End(span, &err)
func End(span trace.Span, errp *error) {
	if errp != nil && *errp != nil {
		RecordError(span, *errp)
	}
	span.End()
}

// --- Standard attribute keys ---

// EntityName returns an attribute for the entity being operated on.
func EntityName(name string) attribute.KeyValue {
	return attribute.String("awo.entity", name)
}

// EntityCount returns an attribute for the number of entities.
func EntityCount(n int) attribute.KeyValue {
	return attribute.Int("awo.entity_count", n)
}

// TenantID returns an attribute for the tenant.
func TenantID(id string) attribute.KeyValue {
	return attribute.String("awo.tenant_id", id)
}

// UserID returns an attribute for the authenticated user.
func UserID(id string) attribute.KeyValue {
	return attribute.String("awo.user_id", id)
}

// WorkflowID returns an attribute for the Temporal workflow.
func WorkflowID(id string) attribute.KeyValue {
	return attribute.String("awo.workflow_id", id)
}

// MigrationVersion returns an attribute for a migration version number.
func MigrationVersion(v uint) attribute.KeyValue {
	return attribute.Int("awo.migration_version", int(v))
}

// Operation returns an attribute for the CRUD/action operation name.
func Operation(op string) attribute.KeyValue {
	return attribute.String("awo.operation", op)
}

// buildExporter creates the span exporter from cfg.
func buildExporter(ctx context.Context, cfg Config) (sdktrace.SpanExporter, error) {
	switch cfg.Exporter {
	case ExporterOTLPGRPC:
		opts := []otlptracegrpc.Option{otlptracegrpc.WithInsecure()}
		if cfg.Endpoint != "" {
			opts = append(opts, otlptracegrpc.WithEndpoint(cfg.Endpoint))
		}
		return otlptracegrpc.New(ctx, opts...)

	case ExporterOTLPHTTP:
		opts := []otlptracehttp.Option{otlptracehttp.WithInsecure()}
		if cfg.Endpoint != "" {
			opts = append(opts, otlptracehttp.WithEndpoint(cfg.Endpoint))
		}
		return otlptracehttp.New(ctx, opts...)

	case ExporterStdout:
		return stdouttrace.New(stdouttrace.WithPrettyPrint())

	case ExporterNoop, "":
		return &noopExporter{}, nil

	default:
		return nil, fmt.Errorf("unknown exporter kind %q", cfg.Exporter)
	}
}

// noopExporter discards all spans. Used in tests and ExporterNoop mode.
type noopExporter struct{}

func (n *noopExporter) ExportSpans(_ context.Context, _ []sdktrace.ReadOnlySpan) error {
	return nil
}
func (n *noopExporter) Shutdown(_ context.Context) error { return nil }
