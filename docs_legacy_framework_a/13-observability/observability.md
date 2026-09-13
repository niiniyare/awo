> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Observability"
id: obs-001
status: accepted
category: SPEC
stability: STABLE
audience: [module-authors, operators]
since: "1.0"
normative-level: normative
related:
  - "[Configuration](../12-configuration/configuration.md)"
  - "[Startup Sequence](../03-kernel/startup-sequence.md)"
  - "[API Conventions](../11-api/conventions.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Observability

**OBS-001 | Status: Accepted | Stability: Stable**

This document specifies structured logging standards, Prometheus metrics catalog, health endpoint contracts, and distributed tracing integration.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. Structured Logging

All log output uses `log/slog` in JSON format. No `fmt.Println` or unstructured logging is permitted.

### Required Fields

Every log entry from a request handler MUST include:

| Field | Type | Source |
|---|---|---|
| `request_id` | string | `X-Request-ID` header or generated UUID |
| `tenant_id` | string | Resolved from middleware |
| `user_id` | string | Actor from session |
| `method` | string | HTTP method |
| `path` | string | Request path |
| `status` | int | HTTP response status |
| `duration_ms` | int | Time from request start to response send |

```go
slog.Info("request completed",
    "request_id",  c.Locals("request_id"),
    "tenant_id",   tenantID.String(),
    "user_id",     actor.UserID.String(),
    "method",      c.Method(),
    "path",        c.Path(),
    "status",      c.Response().StatusCode(),
    "duration_ms", time.Since(start).Milliseconds(),
)
```

### Log Levels

| Level | Use |
|---|---|
| `DEBUG` | Detailed tracing for development; never in production by default |
| `INFO` | Normal request completion, startup events, Temporal workflow starts |
| `WARN` | Permission denied, rate limit hit, tenant suspension detected |
| `ERROR` | Unhandled errors (HTTP 500), database connection failures, Temporal dispatch failures |

### Sensitive Field Prohibition

Log entries MUST NOT contain:
- Password fields or hashes
- Session tokens
- Encryption keys
- Fields declared `Sensitive: true` in `EntityDefinition`
- Full database URLs (may contain embedded credentials)

### Workflow and Activity Logging

Temporal workflows use `workflow.GetLogger(ctx)`:

```go
logger := workflow.GetLogger(ctx)
logger.Info("step completed", "step", "NotifyApprovers", "invoice_id", input.InvoiceID)
```

Activities use `activity.GetLogger(ctx)`:

```go
logger := activity.GetLogger(ctx)
logger.Error("activity failed", "error", err, "attempt", activity.GetInfo(ctx).Attempt)
```

---

## 2. Prometheus Metrics

Metrics are exposed at `GET /metrics` (Prometheus text format). The endpoint requires no authentication — filter at the network layer (allow only scraper IPs).

### HTTP Metrics

| Metric | Type | Labels | Description |
|---|---|---|---|
| `http_request_duration_seconds` | Histogram | `method`, `path`, `status` | Request latency |
| `http_requests_total` | Counter | `method`, `path`, `status` | Request count |
| `http_request_size_bytes` | Histogram | `method`, `path` | Request body size |
| `http_response_size_bytes` | Histogram | `method`, `path` | Response body size |

Path labels are normalized — `/api/v1/entities/finance_invoice/018e1b2c/submit` becomes `/api/v1/entities/{entity}/{id}/{action}` — to prevent label cardinality explosion.

### Database Metrics

| Metric | Type | Labels | Description |
|---|---|---|---|
| `db_query_duration_seconds` | Histogram | `entity`, `operation` | Query latency |
| `db_pool_connections_total` | Gauge | `state` | Pool connection states |
| `db_errors_total` | Counter | `entity`, `error_code` | Database error counts |

### Cache Metrics

| Metric | Type | Labels | Description |
|---|---|---|---|
| `cache_hit_total` | Counter | `cache_type` | Redis cache hits |
| `cache_miss_total` | Counter | `cache_type` | Redis cache misses |
| `cache_operation_duration_seconds` | Histogram | `operation` | Redis operation latency |

`cache_type` values: `session`, `feature_flag`, `page_schema`.

### Workflow Metrics

| Metric | Type | Labels | Description |
|---|---|---|---|
| `workflow_started_total` | Counter | `workflow_type`, `tenant_id` | Workflow dispatch count |
| `workflow_failed_total` | Counter | `workflow_type`, `tenant_id` | Workflow dispatch failures |
| `outbox_pending_total` | Gauge | — | Unprocessed outbox entries |
| `outbox_failed_total` | Gauge | — | Permanently failed outbox entries |
| `outbox_dispatch_duration_seconds` | Histogram | — | Relay dispatch latency |

### Entity Operation Metrics

| Metric | Type | Labels | Description |
|---|---|---|---|
| `entity_operation_total` | Counter | `entity`, `operation`, `status` | CRUD + action counts |
| `entity_operation_duration_seconds` | Histogram | `entity`, `operation` | End-to-end operation latency |

### Custom Module Metrics

Modules MAY register custom metrics using the framework's metrics registry:

```go
var invoiceSubmissions = metrics.NewCounter(metrics.CounterOpts{
    Name:   "finance_invoice_submissions_total",
    Help:   "Total invoice submissions by outcome.",
    Labels: []string{"tenant_id", "outcome"},  // outcome: approved, rejected, timeout
})

// In activity or hook:
invoiceSubmissions.With("tenant_id", tenantID.String(), "outcome", "approved").Inc()
```

Custom metric names MUST be prefixed with the module name: `{module}_{metric_name}`.

---

## 3. Health Endpoints

### GET /health/live

Liveness check. Returns `200 OK` if the process is running. Never fails due to dependency unavailability — if the process can respond, it is alive.

```json
{"status": "ok"}
```

Kubernetes liveness probe should use this endpoint. Liveness failure triggers process restart.

### GET /health/ready

Readiness check. Returns `200 OK` only when all dependencies are reachable and the EntityRegistry is compiled.

```json
{
  "status": "ready",
  "checks": {
    "postgres":       "ok",
    "redis":          "ok",
    "entity_registry": "ok",
    "temporal":       "ok"
  },
  "schema_hash": "a3b4c5d6e7f8...",
  "version": "1.3.0",
  "uptime_seconds": 3724
}
```

On failure:

```json
{
  "status": "not_ready",
  "checks": {
    "postgres":       "ok",
    "redis":          "error: connection refused",
    "entity_registry": "ok",
    "temporal":       "degraded: connection timeout"
  }
}
```

HTTP 503 when not ready.

Kubernetes readiness probe uses this endpoint. Readiness failure removes the pod from load balancer rotation without restarting it.

**Temporal** is reported as `"degraded"` (not `"error"`) in the readiness check — Temporal unavailability does not make the pod unready for CRUD traffic. The pod is still removed from load balancer rotation only if PostgreSQL or Redis are unavailable.

---

## 4. Distributed Tracing

Tracing is optional and controlled by `TRACING_ENABLED=true`. When enabled, the framework uses OpenTelemetry (OTEL) with the configured `OTEL_ENDPOINT`.

### Trace Context

Every request starts a root span. The trace ID is correlated with the structured log `request_id` field:

```go
span := trace.SpanFromContext(c.Context())
traceID := span.SpanContext().TraceID().String()
// traceID → stored as request_id in logs for correlation
```

### Auto-Instrumented Spans

The framework automatically creates child spans for:
- Database queries (pgx instrumentation)
- Redis operations
- HTTP outbound calls (via HTTP client middleware)
- Temporal workflow dispatch

### Module Spans

Modules MAY add custom spans for long operations:

```go
ctx, span := otel.Tracer("finance").Start(ctx, "InvoiceValidator.BeforeCreate")
defer span.End()

// ... validation logic ...

span.SetAttributes(
    attribute.String("invoice.status", invoice.Status),
    attribute.String("tenant.id", tenantID.String()),
)
```

Span names MUST follow `{module}.{operation}` convention.

---

## 5. Alert Definitions

Recommended Prometheus alert rules for production deployments:

```yaml
# Error rate > 1% for 5 minutes
- alert: HighErrorRate
  expr: rate(http_requests_total{status=~"5.."}[5m]) / rate(http_requests_total[5m]) > 0.01
  for: 5m

# P99 latency > 2 seconds
- alert: HighLatency
  expr: histogram_quantile(0.99, rate(http_request_duration_seconds_bucket[5m])) > 2
  for: 5m

# Outbox backlog growing
- alert: OutboxBacklogGrowing
  expr: outbox_pending_total > 50
  for: 5m

# Failed outbox entries (manual intervention required)
- alert: OutboxFailedEntries
  expr: outbox_failed_total > 0
  for: 1m

# Redis unavailable (auth will fail for all requests)
- alert: RedisUnavailable
  expr: up{job="redis"} == 0
  for: 1m

# Database pool saturation
- alert: DBPoolSaturation
  expr: db_pool_connections_total{state="idle"} == 0
  for: 2m
```

---

## Related Documents

- [Configuration](../12-configuration/configuration.md) — log level, metrics, tracing env vars
- [Startup Sequence](../03-kernel/startup-sequence.md) — health endpoint behavior during startup
- [Outbox Pattern](../09-workflow/outbox-pattern.md) — outbox metrics context
- [Glossary](../GLOSSARY.md) — Structured Logging, Prometheus, Health Check, OpenTelemetry
