# Chapter 38 — Observability

> **Volume:** VIII — Quality, Observability & Operations
> **Phase:** 4
> **Audience:** Platform Engineers, SRE, Backend Engineers
> **Prerequisites:** Chapters 25–28, 32

---

## Table of Contents

- [38.1 Observability Strategy](#381-observability-strategy)
- [38.2 Metrics](#382-metrics)
- [38.3 Distributed Tracing](#383-distributed-tracing)
- [38.4 SLOs and Error Budgets](#384-slos-and-error-budgets)
- [38.5 Dashboards](#385-dashboards)
- [38.6 Alerting](#386-alerting)
- [38.7 Incident Response](#387-incident-response)

---

## 38.1 Observability Strategy

AwoERP's observability is built on three pillars: metrics, traces, and logs. Each pillar answers a different question:

- **Metrics** — Is the system healthy right now? (Prometheus + Grafana)
- **Traces** — Why is this specific request slow or failing? (OpenTelemetry + Jaeger)
- **Logs** — What happened, in what order, for this specific request? (Chapter 40)

The three pillars are linked by the `request_id` (X-Request-ID header), which appears in metrics labels, trace IDs, and log fields, allowing an alert to lead directly to the trace and then to the structured logs.

**SRE philosophy:** The platform operates with explicit SLOs and error budgets. SLO breaches trigger error budget burn alerts, which drive the on-call rotation. The system is designed to be operable without deep knowledge of the codebase — dashboards and runbooks are the first line of response.

---

## 38.2 Metrics

### 38.2.1 Instrumentation (Go)

All services use the Prometheus Go client library. Metrics are registered in a central metrics package:

```go
package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    HTTPRequestDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "awoerp_http_request_duration_seconds",
            Help:    "HTTP request duration in seconds",
            Buckets: prometheus.DefBuckets,
        },
        []string{"method", "path", "status_code", "tenant_id"},
    )

    SurfaceCompilationDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "awoerp_surface_compilation_duration_seconds",
            Help:    "Surface definition compilation duration in seconds",
            Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1},
        },
        []string{"surface_id", "cache_result"},  // cache_result: hit_l1, hit_l2, compiled
    )

    ActiveSSEConnections = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "awoerp_sse_connections_active",
            Help: "Number of active SSE connections",
        },
        []string{"tenant_id"},
    )

    OfflineQueueDepth = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "awoerp_offline_queue_depth",
            Help: "Number of queued offline actions awaiting replay",
        },
        []string{"tenant_id"},
    )

    DBQueryDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "awoerp_db_query_duration_seconds",
            Help:    "PostgreSQL query duration in seconds",
            Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
        },
        []string{"query_name", "pool"},  // pool: primary, replica
    )

    CacheHits = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "awoerp_cache_hits_total",
            Help: "Number of cache hits",
        },
        []string{"cache", "result"},  // cache: l1, l2, auth, session; result: hit, miss
    )
)
```

### 38.2.2 Standard Metric Labels

Every metric carries a consistent set of labels to enable filtering and aggregation:

| Label | Values | Notes |
|-------|--------|-------|
| `service` | `api`, `sdui-compiler`, `workflow-worker` | Set at process startup |
| `pod` | K8s pod name | Set at process startup |
| `tenant_id` | UUID | Omitted on platform-wide metrics |
| `surface_id` | Surface identifier | On SDUI metrics only |
| `status_code` | HTTP status | On HTTP metrics only |

### 38.2.3 Key Platform Metrics

| Metric | Type | SLO-relevant |
|--------|------|-------------|
| `awoerp_http_request_duration_seconds` | Histogram | ✓ |
| `awoerp_http_requests_total` | Counter | ✓ |
| `awoerp_http_errors_total` | Counter | ✓ |
| `awoerp_surface_compilation_duration_seconds` | Histogram | ✓ |
| `awoerp_surface_cache_hit_rate` | Gauge | ✓ |
| `awoerp_db_query_duration_seconds` | Histogram | ✓ |
| `awoerp_db_pool_active_connections` | Gauge | |
| `awoerp_redis_operation_duration_seconds` | Histogram | |
| `awoerp_sse_connections_active` | Gauge | |
| `awoerp_temporal_workflow_duration_seconds` | Histogram | |
| `awoerp_offline_queue_depth` | Gauge | |
| `awoerp_fga_check_duration_seconds` | Histogram | |

---

## 38.3 Distributed Tracing

### 38.3.1 OpenTelemetry Setup

All services are instrumented with OpenTelemetry. Traces propagate through HTTP headers (W3C Trace Context) and gRPC metadata:

```go
func setupTracing(serviceName string) func() {
    exporter, err := otlptracehttp.New(context.Background(),
        otlptracehttp.WithEndpoint(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")),
        otlptracehttp.WithInsecure(),
    )
    if err != nil {
        log.Fatal(err)
    }

    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exporter),
        sdktrace.WithResource(resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceNameKey.String(serviceName),
            semconv.ServiceVersionKey.String(buildVersion),
        )),
    )

    otel.SetTracerProvider(tp)
    otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
        propagation.TraceContext{},
        propagation.Baggage{},
    ))

    return func() { tp.Shutdown(context.Background()) }
}
```

### 38.3.2 Span Conventions

Custom spans follow OpenTelemetry semantic conventions:

```go
func (c *SurfaceCompiler) Compile(ctx context.Context, req CompileRequest) (*SurfaceDef, error) {
    ctx, span := otel.Tracer("sdui").Start(ctx, "surface.compile",
        trace.WithAttributes(
            attribute.String("surface.id", req.SurfaceID),
            attribute.String("tenant.id", req.TenantID),
            attribute.String("locale", req.Locale),
        ),
    )
    defer span.End()

    // Child spans for each compilation stage
    ctx, cacheSpan := otel.Tracer("sdui").Start(ctx, "surface.cache_lookup")
    cached, err := c.cache.Get(ctx, req.cacheKey())
    cacheSpan.SetAttributes(attribute.Bool("cache.hit", cached != nil))
    cacheSpan.End()

    if cached != nil {
        span.SetAttributes(attribute.String("cache.result", "hit"))
        return cached, nil
    }

    // ... remaining compile stages each in their own span
}
```

### 38.3.3 Trace Sampling

The sampling strategy:
- **Always sample:** All 5xx errors; all requests with `X-Force-Trace: 1` header; all requests from superadmin sessions
- **Tail-based sampling at 10%:** Normal requests
- **Always sample for new tenants:** First 24 hours after tenant provisioning

```go
type AwoERPSampler struct{}

func (s AwoERPSampler) ShouldSample(params sdktrace.SamplingParameters) sdktrace.SamplingResult {
    // Check if parent was sampled
    if trace.SpanFromContext(params.ParentContext).SpanContext().IsSampled() {
        return sdktrace.AlwaysSample().ShouldSample(params)
    }

    // Always sample errors and forced traces
    for _, attr := range params.Attributes {
        if attr.Key == "http.status_code" && attr.Value.AsInt64() >= 500 {
            return sdktrace.AlwaysSample().ShouldSample(params)
        }
        if attr.Key == "awoerp.force_trace" && attr.Value.AsBool() {
            return sdktrace.AlwaysSample().ShouldSample(params)
        }
    }

    // 10% tail sampling
    return sdktrace.TraceIDRatioBased(0.1).ShouldSample(params)
}
```

---

## 38.4 SLOs and Error Budgets

### 38.4.1 SLO Definitions

| SLO | Target | Window |
|-----|--------|--------|
| API availability | 99.9% | 30-day rolling |
| Surface definition latency p99 | < 300ms | 7-day rolling |
| Surface definition latency p50 | < 30ms | 7-day rolling |
| Data mutation success rate | 99.5% | 7-day rolling |
| Workflow completion rate | 99.0% | 7-day rolling |
| SSE connection uptime | 99.5% | 7-day rolling |

### 38.4.2 Error Budget Calculation

For a 99.9% availability SLO over a 30-day window:
- Total minutes: 30 × 24 × 60 = 43,200
- Allowed downtime: 43,200 × 0.001 = 43.2 minutes per month
- Error budget remaining: displayed as a percentage in the SRE dashboard

```promql
# Error budget burn rate (1-hour window vs 30-day budget)
(
  1 - (
    sum(rate(awoerp_http_requests_total{status_code!~"5.."}[1h]))
    /
    sum(rate(awoerp_http_requests_total[1h]))
  )
) / (1 - 0.999)
```

A burn rate >14.4 means the error budget will be exhausted in less than 2 days — this triggers a page.

### 38.4.3 Multi-Window Multi-Burn-Rate Alerting

Following the Google SRE approach, alerts fire on error budget consumption rate, not on raw error rate:

| Alert | Burn Rate | Short Window | Long Window | Severity |
|-------|-----------|-------------|------------|---------|
| Critical | 14.4× | 1h | 5h | Page |
| High | 6× | 6h | 1d | Page |
| Medium | 3× | 1d | 3d | Ticket |
| Low | 1× | 3d | 7d | Weekly review |

---

## 38.5 Dashboards

### 38.5.1 Platform Overview Dashboard

The top-level dashboard visible to all engineers shows:

- **Request rate** (req/s, 5m avg) — by service
- **Error rate** (% 5xx) — with SLO line at 0.1%
- **Latency percentiles** (p50/p95/p99) — for API and SDUI compilation
- **Active tenants** (gauge)
- **SSE connections** (gauge)
- **Error budget remaining** (single stat, green/yellow/red)

### 38.5.2 Tenant Health Dashboard

Drillable per-tenant dashboard:

- Request volume for this tenant
- Error rate for this tenant
- Surface compilation latency for this tenant
- Active SSE connections for this tenant
- Offline queue depth (if mobile sessions active)
- Recent audit events (linked to log viewer)

### 38.5.3 Cache Health Dashboard

- L1/L2 hit rates over time
- Redis memory usage
- Cache invalidation latency
- 304 response rate (surface definitions)

### 38.5.4 Database Dashboard

- Query duration by query name (p50/p99)
- Slow query log (>100ms threshold)
- Connection pool utilization (primary + replica)
- Active locks
- Replication lag (primary → replica)
- Table bloat

---

## 38.6 Alerting

### 38.6.1 Alert Routing

Alerts route through PagerDuty with escalation policies:

```yaml
# alertmanager.yml (excerpt)
route:
  receiver: 'platform-low'
  routes:
    - match:
        severity: critical
      receiver: 'platform-critical'
      continue: false
    - match:
        severity: high
      receiver: 'platform-high'
      continue: false
    - match:
        tenant_id: '.+'
      receiver: 'tenant-alerts'  # Non-paging; logged to tenant health queue
      continue: false

receivers:
  - name: 'platform-critical'
    pagerduty_configs:
      - routing_key: '${PAGERDUTY_KEY}'
        severity: 'critical'
  - name: 'platform-high'
    pagerduty_configs:
      - routing_key: '${PAGERDUTY_KEY}'
        severity: 'error'
```

### 38.6.2 Core Alert Rules

```yaml
# alerts.yml
groups:
  - name: availability
    rules:
      - alert: APIHighErrorRate
        expr: |
          (
            sum(rate(awoerp_http_requests_total{status_code=~"5.."}[5m]))
            /
            sum(rate(awoerp_http_requests_total[5m]))
          ) > 0.01
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "API error rate > 1% for 5 minutes"
          runbook: "https://runbooks.internal/api-high-error-rate"

      - alert: SurfaceCompilationLatencyHigh
        expr: |
          histogram_quantile(0.99,
            rate(awoerp_surface_compilation_duration_seconds_bucket[5m])
          ) > 0.3
        for: 5m
        labels:
          severity: high
        annotations:
          summary: "Surface compilation p99 > 300ms"
          runbook: "https://runbooks.internal/surface-compilation-slow"

      - alert: DBConnectionPoolExhausted
        expr: |
          awoerp_db_pool_active_connections / awoerp_db_pool_max_connections > 0.9
        for: 2m
        labels:
          severity: high
        annotations:
          summary: "DB connection pool > 90% utilized"
```

---

## 38.7 Incident Response

### 38.7.1 Incident Severity Levels

| Level | Definition | Response Time |
|-------|-----------|--------------|
| P1 | Complete platform outage; all tenants affected | Immediate page; 15-min acknowledge |
| P2 | Degraded service; multiple tenants affected | Page; 30-min acknowledge |
| P3 | Single-tenant impact; workaround exists | Ticket; next business day |
| P4 | Cosmetic / minor; no user impact | Ticket; sprint backlog |

### 38.7.2 Incident Runbook Structure

Every alert links to a runbook. Runbooks follow a standard structure:

```markdown
# Runbook: Surface Compilation Slow

## Symptoms
- SurfaceCompilationLatencyHigh alert firing
- Users reporting slow page loads

## Immediate Checks
1. Check surface cache hit rate (dashboard link)
2. Check Redis availability and memory
3. Check DB replication lag
4. Check for recent surface definition publications

## Mitigation Steps
1. If Redis is down: failover to DB-only compilation (disable L2 cache)
2. If specific surface: roll back surface definition to previous version
3. If DB slow: switch SDUI compiler to use replica pool

## Escalation
If not resolved in 30 minutes: escalate to platform-lead
```

### 38.7.3 Post-Incident Review

All P1 and P2 incidents require a post-incident review within 48 hours. The review document covers:
- Timeline of events
- Root cause
- Impact (number of tenants, duration, affected features)
- Contributing factors
- Action items with owners and deadlines

Action items feed directly into the engineering backlog.

---

*End of Chapter 38*

**Previous:** [Chapter 37 — Accessibility](./37-accessibility.md)
**Next:** [Chapter 39 — Telemetry](./39-telemetry.md)
