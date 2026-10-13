# Chapter 40 — Logging

> **Volume:** VIII — Quality, Observability & Operations
> **Phase:** 4
> **Audience:** Backend Engineers, Platform Engineers, SRE
> **Prerequisites:** Chapters 25–28, 38

---

## Table of Contents

- [40.1 Logging Philosophy](#401-logging-philosophy)
- [40.2 Structured Log Format](#402-structured-log-format)
- [40.3 Log Levels and Usage](#403-log-levels-and-usage)
- [40.4 Standard Log Fields](#404-standard-log-fields)
- [40.5 Request Logging](#405-request-logging)
- [40.6 Application Logging Patterns](#406-application-logging-patterns)
- [40.7 Log Shipping](#407-log-shipping)
- [40.8 Log Retention and Search](#408-log-retention-and-search)
- [40.9 Audit Logs](#409-audit-logs)

---

## 40.1 Logging Philosophy

Logs are the narrative of what happened inside a request. Unlike metrics (which aggregate) or traces (which show causation), logs carry the full context of a specific event — what data was involved, what decision was made, what error was encountered.

**Three rules for AwoERP logging:**

1. **Every log line is structured JSON.** No human-formatted strings. A log line must be parseable by a machine without regex.
2. **Every log line carries `request_id`.** A log line that cannot be linked to a request is nearly useless during incident response.
3. **Log at the point of decision, not at the point of code.** Log when a business rule fires, when a branch is taken for a significant reason, when an external call is made. Do not log "entered function X" or "about to call Y".

---

## 40.2 Structured Log Format

AwoERP uses Go's `log/slog` package (standard library, Go 1.21+) with a JSON handler:

```go
func newLogger(service string) *slog.Logger {
    return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level:     slogLevel(),  // Configured via LOG_LEVEL env var
        AddSource: false,        // Source added selectively; noisy in production
    })).With(
        "service", service,
        "version", buildVersion,
        "pod",     os.Getenv("POD_NAME"),
    )
}
```

All log output goes to stdout. The container runtime (Kubernetes) captures stdout and routes it to the log aggregation pipeline (Section 40.7).

A typical log line:

```json
{
  "time": "2024-03-15T11:32:00.123Z",
  "level": "INFO",
  "msg": "invoice approved",
  "service": "api",
  "version": "2.1.4",
  "pod": "api-abc123",
  "request_id": "01HT2X3Y4Z",
  "tenant_id": "b4e5f678-...",
  "user_id": "a1b2c3d4-...",
  "invoice_id": "inv-001",
  "duration_ms": 47
}
```

---

## 40.3 Log Levels and Usage

| Level | Usage | Examples |
|-------|-------|---------|
| `ERROR` | A request failed in a way that is not the caller's fault (5xx). Should be rare; every ERROR warrants investigation. | DB query failed, Redis unreachable, unexpected nil pointer |
| `WARN` | A recoverable problem occurred; the request succeeded but something unexpected happened. | Cache miss that should have been a hit, slow query, retried external call |
| `INFO` | A significant business event occurred. Readable summary of what happened in the system. | Invoice approved, tenant provisioned, workflow completed |
| `DEBUG` | Detailed diagnostic information. Disabled in production by default. | Cache key computed, RLS context set, flag evaluated |

**INFO is the workhorse level.** Every successful mutation generates exactly one INFO log line summarizing the outcome. Every incoming request generates one INFO log line at response (see 40.5).

**ERROR must not be noisy.** If ERROR is firing more than once per minute per service, it is either a real outage or the level is being misused. Client errors (4xx) are not ERRORs — they are the normal result of invalid input and are logged at INFO.

---

## 40.4 Standard Log Fields

Every log line must include:

| Field | Type | Source |
|-------|------|--------|
| `time` | RFC 3339 UTC | `slog` built-in |
| `level` | string | `slog` built-in |
| `msg` | string | Call site |
| `service` | string | Process startup |
| `version` | string | Build artifact |
| `pod` | string | `POD_NAME` env |
| `request_id` | string | Request context |

Contextual fields added by middleware and passed via `context.Context`:

| Field | When Present |
|-------|-------------|
| `tenant_id` | After tenant resolution middleware |
| `user_id` | After auth middleware |
| `surface_id` | On SDUI compilation requests |
| `trace_id` | When OpenTelemetry is active |

### 40.4.1 Context-Aware Logger

The request context carries a logger pre-populated with request-scoped fields. All log calls use this context logger:

```go
// Middleware: inject enriched logger into context
func LoggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        requestID := r.Header.Get("X-Request-ID")
        if requestID == "" {
            requestID = ulid.Make().String()
        }

        log := slog.Default().With(
            "request_id", requestID,
            "method", r.Method,
            "path", r.URL.Path,
        )
        ctx := slogctx.NewContext(r.Context(), log)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

// Call site: always use context logger
func (s *InvoiceService) ApproveInvoice(ctx context.Context, id string) error {
    log := slogctx.FromContext(ctx)
    log.InfoContext(ctx, "approving invoice", "invoice_id", id)
    // ...
}
```

---

## 40.5 Request Logging

Every HTTP request and response is logged by the access log middleware. One log line per request, emitted after the response:

```go
type statusRecorder struct {
    http.ResponseWriter
    status int
    size   int
}

func (r *statusRecorder) WriteHeader(status int) {
    r.status = status
    r.ResponseWriter.WriteHeader(status)
}

func AccessLogMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        recorder := &statusRecorder{ResponseWriter: w, status: 200}
        next.ServeHTTP(recorder, r)

        log := slogctx.FromContext(r.Context())
        log.InfoContext(r.Context(), "request",
            "status", recorder.status,
            "duration_ms", time.Since(start).Milliseconds(),
            "response_bytes", recorder.size,
            "user_agent", r.UserAgent(),
            // path and method already in the context logger from LoggingMiddleware
        )
    })
}
```

Example access log line:
```json
{
  "time": "2024-03-15T11:32:00.170Z",
  "level": "INFO",
  "msg": "request",
  "service": "api",
  "request_id": "01HT2X3Y4Z",
  "tenant_id": "b4e5f678-...",
  "user_id": "a1b2c3d4-...",
  "method": "POST",
  "path": "/api/invoices/inv-001/approve",
  "status": 200,
  "duration_ms": 47,
  "response_bytes": 312
}
```

4xx responses are logged at INFO (expected client errors). 5xx responses are logged at ERROR and include the error detail.

---

## 40.6 Application Logging Patterns

### 40.6.1 Mutation Events

Every successful mutation logs one INFO event:

```go
func (s *InvoiceService) ApproveInvoice(ctx context.Context, id string) (*Invoice, error) {
    invoice, err := s.store.ApproveInvoice(ctx, id)
    if err != nil {
        // The DB layer logs the low-level error at ERROR
        // Here we log the business context at WARN
        slogctx.FromContext(ctx).WarnContext(ctx, "invoice approval failed",
            "invoice_id", id,
            "error", err,
        )
        return nil, err
    }

    slogctx.FromContext(ctx).InfoContext(ctx, "invoice approved",
        "invoice_id", id,
        "previous_status", invoice.PreviousStatus,
        "approved_amount", invoice.Amount,
        "currency", invoice.CurrencyCode,
    )
    return invoice, nil
}
```

### 40.6.2 External Calls

External calls (Temporal, OpenFGA, Redis, PostgreSQL) log at DEBUG in the normal case. Slow or failed external calls log at WARN or ERROR:

```go
func (c *FGAClient) Check(ctx context.Context, req CheckRequest) (bool, error) {
    start := time.Now()
    result, err := c.client.Check(ctx, req)
    duration := time.Since(start)

    if err != nil {
        slogctx.FromContext(ctx).ErrorContext(ctx, "fga check failed",
            "relation", req.Relation,
            "object", req.Object,
            "duration_ms", duration.Milliseconds(),
            "error", err,
        )
        return false, err
    }

    if duration > 50*time.Millisecond {
        slogctx.FromContext(ctx).WarnContext(ctx, "slow fga check",
            "relation", req.Relation,
            "duration_ms", duration.Milliseconds(),
        )
    }

    return result.Allowed, nil
}
```

### 40.6.3 What Not to Log

- **No passwords, tokens, or secrets.** Never log the full Authorization header or any credential.
- **No PII.** No names, email addresses, or phone numbers. Log `user_id` (UUID) only.
- **No large payloads.** Do not log request or response bodies. Use the trace for payload inspection when needed.
- **No "entered function" logs.** Function entry/exit belongs in traces, not logs.

---

## 40.7 Log Shipping

### 40.7.1 Collection Architecture

```
Pod stdout (JSON)
    → Fluent Bit (DaemonSet, per node)
        → Loki (log aggregation)
            → Grafana (search and visualization)
```

Fluent Bit runs as a DaemonSet and tails container log files without any modification to the service code. It adds Kubernetes metadata (namespace, pod, container) and forwards to Loki.

### 40.7.2 Fluent Bit Configuration

```ini
[INPUT]
    Name              tail
    Path              /var/log/containers/awoerp-*.log
    Parser            docker
    Tag               awoerp.*
    Refresh_Interval  5

[FILTER]
    Name              kubernetes
    Match             awoerp.*
    Kube_Tag_Prefix   awoerp.
    Labels            On
    Annotations       Off

[OUTPUT]
    Name              loki
    Match             awoerp.*
    Host              loki.monitoring.svc
    Port              3100
    Labels            service=$kubernetes['labels']['app'],pod=$kubernetes['pod_name']
    Remove_Keys       kubernetes
```

### 40.7.3 Log Volume Management

Expected log volumes in production:

| Service | Log Rate |
|---------|---------|
| API (access log) | ~500 lines/s at peak |
| API (application) | ~50 lines/s at peak |
| SDUI compiler | ~20 lines/s at peak |
| Workflow workers | ~10 lines/s at peak |

Total: ~600 lines/s at peak. At ~200 bytes per JSON line, this is ~120 KB/s or ~10 GB/day. Loki compresses by ~10:1, yielding ~1 GB/day stored.

---

## 40.8 Log Retention and Search

### 40.8.1 Retention Policy

| Log Type | Retention |
|----------|-----------|
| Application logs | 30 days (hot, searchable) |
| Application logs | 90 days (cold, compressed, on-demand restore) |
| Audit logs | 7 years (see 40.9) |

### 40.8.2 Searching Logs

All log search goes through Grafana (LogQL):

```logql
# Find all ERROR logs for a specific tenant in the last hour
{service="api"} |= `"level":"ERROR"` | json | tenant_id = "b4e5f678-..."

# Find a specific request by request_id
{service=~"api|sdui-compiler"} | json | request_id = "01HT2X3Y4Z"

# Slow requests (>500ms)
{service="api"} | json | duration_ms > 500

# Surface compilation failures in the last 24h
{service="sdui-compiler"} | json | level = "ERROR" | line_format "{{.msg}} | surface: {{.surface_id}} | error: {{.error}}"
```

---

## 40.9 Audit Logs

Audit logs are separate from application logs. They record every user-initiated mutation to the platform's business data. See Chapter 26 (Security) for the `AuditEvent` struct and `AuditMiddleware`.

**Key differences from application logs:**

| Property | Application Logs | Audit Logs |
|----------|-----------------|-----------|
| Purpose | Debugging and operations | Compliance and accountability |
| Retention | 30–90 days | 7 years |
| Storage | Loki (log aggregation) | Append-only PostgreSQL table |
| Searchable by | Tenant admins? | Yes (their own events) |
| Deletable? | Eventually (retention policy) | Never (immutable) |
| Contains user actions? | Incidentally | Always |

Audit logs are surfaced to tenant admins through the `_audit_log` platform surface. Superadmins can search across all tenants.

The audit log write path is non-blocking (goroutine), so a failure to write an audit log does not fail the request. Failed audit log writes are retried via a dead-letter queue, and alert if the queue depth exceeds 1,000 unwritten events.

---

*End of Chapter 40*

---

*End of Volume VIII — Quality, Observability & Operations*

**Previous:** [Chapter 39 — Telemetry](./39-telemetry.md)
**Next:** [Chapter 41 — Testing Strategy](../vol-09-operations/41-testing-strategy.md)
