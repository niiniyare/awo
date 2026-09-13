// Package observability provides metrics, tracing, and structured logging for
// the SDUI framework.
//
// # Integration
//
// The package integrates with the framework observability subsystem via the
// standard OpenTelemetry SDK (go.opentelemetry.io/otel) and Prometheus
// (prometheus/client_golang). It does not introduce a parallel observability
// implementation — it wraps the OTel meter and tracer configured by the host
// application.
//
// # Usage
//
//	obs, err := observability.New(observability.Config{
//	    MeterName:  "awo.sdui",
//	    TracerName: "awo.sdui",
//	})
//	// Pass obs to engine.New(..., obs).
//
// # Metrics emitted
//
// All metrics use the "sdui." prefix.
//
//   - sdui.generation_duration_ms  histogram  Generation latency
//   - sdui.render_duration_ms      histogram  Render latency
//   - sdui.validation_duration_ms  histogram  Validation latency
//   - sdui.layout_duration_ms      histogram  Layout latency
//   - sdui.plugin_duration_ms      histogram  Plugin pipeline latency
//   - sdui.cache_hits_total        counter    L2/L3 cache hits
//   - sdui.cache_misses_total      counter    L2/L3 cache misses
//   - sdui.errors_total            counter    Error events by stage
//
// # Spans emitted
//
// All spans use the "sdui." prefix.
//
//   - sdui.request           Top-level span for the full pipeline
//   - sdui.cache_lookup      Cache lookup (L2 or L3)
//   - sdui.generate          Widget tree generation
//   - sdui.validate          Widget tree validation
//   - sdui.layout            Layout computation
//   - sdui.render            Rendering
//   - sdui.plugin_pipeline   Plugin execution
package observability

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Stage labels used in error metrics.
const (
	StageGenerate = "generate"
	StageValidate = "validate"
	StageLayout   = "layout"
	StageRender   = "render"
	StagePlugin   = "plugin"
	StageCache    = "cache"
)

// CacheLevel labels distinguish L2 (widget tree) from L3 (rendered output).
const (
	CacheLevelL2 = "l2"
	CacheLevelL3 = "l3"
)

// Config carries constructor parameters for Metrics.
type Config struct {
	// MeterName is the OTel meter name (default: "awo.sdui").
	MeterName string

	// TracerName is the OTel tracer name (default: "awo.sdui").
	TracerName string
}

func (c *Config) meterName() string {
	if c.MeterName != "" {
		return c.MeterName
	}
	return "awo.sdui"
}

func (c *Config) tracerName() string {
	if c.TracerName != "" {
		return c.TracerName
	}
	return "awo.sdui"
}

// Metrics holds the OTel instruments for the SDUI framework.
// Construct via New(). All methods are safe for concurrent use.
type Metrics struct {
	tracer trace.Tracer

	genDuration      metric.Float64Histogram
	renderDuration   metric.Float64Histogram
	validateDuration metric.Float64Histogram
	layoutDuration   metric.Float64Histogram
	pluginDuration   metric.Float64Histogram
	cacheHits        metric.Int64Counter
	cacheMisses      metric.Int64Counter
	errorsTotal      metric.Int64Counter
}

// New constructs a Metrics from the global OTel meter and tracer providers.
// Returns an error if any instrument fails to register.
func New(cfg Config) (*Metrics, error) {
	meter := otel.GetMeterProvider().Meter(cfg.meterName())
	tracer := otel.GetTracerProvider().Tracer(cfg.tracerName())

	genDur, err := meter.Float64Histogram("sdui.generation_duration_ms",
		metric.WithDescription("SDUI widget tree generation latency in milliseconds"),
		metric.WithUnit("ms"))
	if err != nil {
		return nil, err
	}

	renderDur, err := meter.Float64Histogram("sdui.render_duration_ms",
		metric.WithDescription("SDUI renderer latency in milliseconds"),
		metric.WithUnit("ms"))
	if err != nil {
		return nil, err
	}

	validateDur, err := meter.Float64Histogram("sdui.validation_duration_ms",
		metric.WithDescription("SDUI widget tree validation latency in milliseconds"),
		metric.WithUnit("ms"))
	if err != nil {
		return nil, err
	}

	layoutDur, err := meter.Float64Histogram("sdui.layout_duration_ms",
		metric.WithDescription("SDUI layout computation latency in milliseconds"),
		metric.WithUnit("ms"))
	if err != nil {
		return nil, err
	}

	pluginDur, err := meter.Float64Histogram("sdui.plugin_duration_ms",
		metric.WithDescription("SDUI plugin pipeline latency in milliseconds"),
		metric.WithUnit("ms"))
	if err != nil {
		return nil, err
	}

	hits, err := meter.Int64Counter("sdui.cache_hits_total",
		metric.WithDescription("SDUI cache hits by level"))
	if err != nil {
		return nil, err
	}

	misses, err := meter.Int64Counter("sdui.cache_misses_total",
		metric.WithDescription("SDUI cache misses by level"))
	if err != nil {
		return nil, err
	}

	errs, err := meter.Int64Counter("sdui.errors_total",
		metric.WithDescription("SDUI errors by pipeline stage"))
	if err != nil {
		return nil, err
	}

	return &Metrics{
		tracer:           tracer,
		genDuration:      genDur,
		renderDuration:   renderDur,
		validateDuration: validateDur,
		layoutDuration:   layoutDur,
		pluginDuration:   pluginDur,
		cacheHits:        hits,
		cacheMisses:      misses,
		errorsTotal:      errs,
	}, nil
}

// Noop returns a Metrics that discards all observations. Use in tests and
// components that do not need observability.
func Noop() *Metrics {
	m := &Metrics{
		tracer: otel.GetTracerProvider().Tracer("noop"),
	}
	// Instrument fields remain nil; all record methods guard with nil checks.
	return m
}

// ── Tracing ───────────────────────────────────────────────────────────────────

// StartSpan starts an OTel span named "sdui.{name}" and returns the child
// context and a finish function. Call the finish function in a defer.
//
//	ctx, finish := obs.StartSpan(ctx, "generate")
//	defer finish()
func (m *Metrics) StartSpan(ctx context.Context, name string) (context.Context, func()) {
	ctx, span := m.tracer.Start(ctx, "sdui."+name)
	return ctx, func() { span.End() }
}

// StartSpanWithAttrs starts a span with the given OTel attributes.
func (m *Metrics) StartSpanWithAttrs(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, func()) {
	ctx, span := m.tracer.Start(ctx, "sdui."+name,
		trace.WithAttributes(attrs...))
	return ctx, func() { span.End() }
}

// ── Timing helpers ────────────────────────────────────────────────────────────

// Timer is a helper returned by RecordStart.
// Call Done() to record the elapsed duration.
type Timer struct {
	start    time.Time
	recordFn func(ms float64)
}

// Done records the elapsed milliseconds since the timer was started.
func (t Timer) Done() {
	if t.recordFn != nil {
		t.recordFn(float64(time.Since(t.start).Microseconds()) / 1000.0)
	}
}

// TrackGeneration returns a Timer that records to genDuration when Done() is called.
func (m *Metrics) TrackGeneration(ctx context.Context, entityName, viewMode string) Timer {
	attrs := attribute.NewSet(
		attribute.String("entity", entityName),
		attribute.String("view_mode", viewMode),
	)
	return Timer{
		start: time.Now(),
		recordFn: func(ms float64) {
			if m.genDuration != nil {
				m.genDuration.Record(ctx, ms, metric.WithAttributeSet(attrs))
			}
		},
	}
}

// TrackRender returns a Timer that records to renderDuration when Done() is called.
func (m *Metrics) TrackRender(ctx context.Context, rendererID string) Timer {
	attrs := attribute.NewSet(attribute.String("renderer", rendererID))
	return Timer{
		start: time.Now(),
		recordFn: func(ms float64) {
			if m.renderDuration != nil {
				m.renderDuration.Record(ctx, ms, metric.WithAttributeSet(attrs))
			}
		},
	}
}

// TrackValidation returns a Timer recording validation latency.
func (m *Metrics) TrackValidation(ctx context.Context) Timer {
	return Timer{
		start: time.Now(),
		recordFn: func(ms float64) {
			if m.validateDuration != nil {
				m.validateDuration.Record(ctx, ms)
			}
		},
	}
}

// TrackLayout returns a Timer recording layout computation latency.
func (m *Metrics) TrackLayout(ctx context.Context) Timer {
	return Timer{
		start: time.Now(),
		recordFn: func(ms float64) {
			if m.layoutDuration != nil {
				m.layoutDuration.Record(ctx, ms)
			}
		},
	}
}

// TrackPlugins returns a Timer recording plugin pipeline latency.
func (m *Metrics) TrackPlugins(ctx context.Context, point string) Timer {
	attrs := attribute.NewSet(attribute.String("point", point))
	return Timer{
		start: time.Now(),
		recordFn: func(ms float64) {
			if m.pluginDuration != nil {
				m.pluginDuration.Record(ctx, ms, metric.WithAttributeSet(attrs))
			}
		},
	}
}

// ── Cache metrics ─────────────────────────────────────────────────────────────

// RecordCacheHit increments the cache hit counter for the given level.
func (m *Metrics) RecordCacheHit(ctx context.Context, level string) {
	if m.cacheHits != nil {
		m.cacheHits.Add(ctx, 1, metric.WithAttributes(attribute.String("level", level)))
	}
}

// RecordCacheMiss increments the cache miss counter for the given level.
func (m *Metrics) RecordCacheMiss(ctx context.Context, level string) {
	if m.cacheMisses != nil {
		m.cacheMisses.Add(ctx, 1, metric.WithAttributes(attribute.String("level", level)))
	}
}

// ── Error metrics ─────────────────────────────────────────────────────────────

// RecordError increments the error counter for the given pipeline stage.
func (m *Metrics) RecordError(ctx context.Context, stage string) {
	if m.errorsTotal != nil {
		m.errorsTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("stage", stage)))
	}
}
