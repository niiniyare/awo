> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Distributed Tracing"
id: obs-005
status: accepted
category: GUIDE
stability: STABLE
audience: [framework-authors, operators]
since: "1.0"
normative-level: informative
related:
  - "[Structured Logging](structured-logging.md)"
  - "[Metrics Reference](metrics-reference.md)"
  - "[Performance Tuning](../14-operations/performance-tuning.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Distributed Tracing

**OBS-005 | Status: Accepted | Stability: Stable**

OpenTelemetry distributed tracing for Awo: instrumentation, span attributes, and Jaeger/Tempo integration.

---

## 1. Tracing Stack

Awo uses OpenTelemetry (OTel) for distributed tracing:

- **SDK**: `go.opentelemetry.io/otel`
- **Exporter**: OTLP (gRPC) to Jaeger or Grafana Tempo
- **Propagation**: W3C TraceContext (`traceparent` header)
- **Sampling**: Probability-based, configurable per environment

---

## 2. Automatic Instrumentation

The framework automatically creates spans for:

| Span | Attributes |
|---|---|
| HTTP request | `http.method`, `http.route`, `http.status_code`, `tenant.id`, `user.id`, `request.id` |
| DB query | `db.operation`, `db.entity_type`, `db.statement` (parameterized only) |
| Hook execution | `hook.type`, `hook.name`, `entity.type` |
| Cache operation | `cache.operation`, `cache.key_type`, `cache.result` |
| Temporal activity call | `temporal.activity`, `temporal.namespace`, `tenant.id` |

---

## 3. Adding Custom Spans

In handlers or activities:

```go
import "go.opentelemetry.io/otel"

func (h *InvoiceHandler) handleSubmit(c *fiber.Ctx) error {
    ctx, span := otel.Tracer("finance").Start(c.UserContext(), "invoice.submit")
    defer span.End()

    // Add attributes to the span
    span.SetAttributes(
        attribute.String("invoice.id", c.Params("id")),
        attribute.String("tenant.id", session.TenantIDFromContext(ctx).String()),
    )

    result, err := h.service.Submit(ctx, invoiceID)
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
        return mapError(c, err)
    }

    return c.JSON(result)
}
```

---

## 4. Span Naming Convention

```
{module}.{operation}          → "finance.invoice.submit"
{entity_type}.{crud_op}       → "finance_invoice.create"
hook.{hook_type}.{hook_name}  → "hook.before_save.InvoicePeriodLockGuard"
db.{entity}.{operation}       → "db.finance_invoice.query"
cache.{operation}             → "cache.get"
```

---

## 5. Trace Propagation

Awo propagates traces using the W3C TraceContext standard. Incoming `traceparent` headers are automatically read and used to join existing traces.

For outgoing requests (webhooks, external APIs):

```go
// Inject trace context into outgoing request
req, _ := http.NewRequest("POST", webhookURL, body)
otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))
```

---

## 6. Configuration

```bash
# Enable tracing
OTEL_EXPORTER_OTLP_ENDPOINT=http://jaeger:4317
OTEL_SERVICE_NAME=awo-api
OTEL_TRACES_SAMPLER=parentbased_traceidratio
OTEL_TRACES_SAMPLER_ARG=0.1  # Sample 10% in production

# Development: sample everything
OTEL_TRACES_SAMPLER=always_on
```

---

## 7. Trace-Log Correlation

Every log entry includes the trace and span ID when inside a traced context:

```go
// Middleware injects trace IDs into logger
slog.Info("request completed",
    "trace_id", span.SpanContext().TraceID().String(),
    "span_id",  span.SpanContext().SpanID().String(),
    // ... other standard fields
)
```

In Grafana, use the trace ID from a log entry to jump directly to the corresponding trace.

---

## 8. Sampling Strategy

| Environment | Sampler | Reason |
|---|---|---|
| Development | `always_on` | See every trace |
| Staging | `parentbased_traceidratio=0.5` | 50% sampling |
| Production | `parentbased_traceidratio=0.05` | 5% sampling |
| Production (errors) | `always_on` for error spans | Always capture errors |

Error spans are always sampled regardless of the global rate — configure in the OTel SDK:

```go
// Custom sampler that always samples error spans
type errorAlwaysSampler struct {
    base sdktrace.Sampler
}

func (s errorAlwaysSampler) ShouldSample(p sdktrace.SamplingParameters) sdktrace.SamplingResult {
    if p.Kind == trace.SpanKindServer {
        return sdktrace.AlwaysSample().ShouldSample(p)
    }
    return s.base.ShouldSample(p)
}
```

---

## Related Documents

- [Structured Logging](structured-logging.md) — trace/span ID in log entries
- [Metrics Reference](metrics-reference.md) — RED metrics complementing traces
- [Performance Tuning](../14-operations/performance-tuning.md) — using traces to identify slow queries
