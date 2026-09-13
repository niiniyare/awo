> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

# Logging Specification

**Classification:** Specification — Tier 1
**Owner:** `17-observability/LOGGING_SPEC.md`
**Status:** Frozen at v1.0

---

## Purpose

This document specifies the structured logging standard for the Awo Framework — the mandatory context fields, log levels, sensitive field exclusions, and slog usage patterns.

---

## 1. Logger

The framework uses Go's standard `log/slog` package with JSON output format. Do not use Zerolog, Logrus, Zap, or any other logging library. All log entries are JSON objects on stdout.

```go
// Global logger initialisation (cmd/server/main.go)
slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: config.LogLevel,  // from LOG_LEVEL env var
})))
```

---

## 2. Mandatory Context Fields

Every log entry in the request path MUST include:

| Field | Type | Description |
|-------|------|-------------|
| `request_id` | string | `X-Request-ID` value, or generated UUID if absent |
| `tenant_id` | string | Current tenant UUID (after tenant resolution) |
| `user_id` | string | Authenticated user UUID (after auth middleware) |
| `method` | string | HTTP method |
| `path` | string | Request path |
| `duration_ms` | int64 | Request duration in milliseconds |

Background jobs (outbox worker, Temporal activities) MUST include:

| Field | Type | Description |
|-------|------|-------------|
| `job` | string | Job/worker name |
| `tenant_id` | string | Current tenant UUID |
| `entity_type` | string | Entity being processed |

---

## 3. Request Log Pattern

```go
// Middleware: request completed
slog.Info("request completed",
    "request_id",  c.Locals("request_id"),
    "tenant_id",   tenantID.String(),
    "user_id",     userID.String(),
    "method",      c.Method(),
    "path",        c.Path(),
    "status",      c.Response().StatusCode(),
    "duration_ms", time.Since(start).Milliseconds(),
)
```

---

## 4. Error Log Pattern

```go
// All error log entries MUST include err, request_id, tenant_id, user_id
slog.Error("operation failed",
    "err",        err,
    "request_id", requestID,
    "tenant_id",  tenantID.String(),
    "user_id",    userID.String(),
    "entity",     entityName,
    "operation",  "submit",
)
```

Never log: `err.Error()` as a string — use `"err", err` so slog serialises the full error.

---

## 5. Log Levels

| Level | When |
|-------|------|
| `DEBUG` | Detailed internal state (disabled in production) |
| `INFO` | Normal operations (request completed, job processed) |
| `WARN` | Recoverable anomalies (cache miss on hot path, retry scheduled) |
| `ERROR` | Unexpected failures (DB error, workflow dispatch failed, panic) |

`WARN` MUST NOT be used for validation errors or business rule violations — those are expected client errors, logged at `INFO` with `status: 422` or `status: 409`.

---

## 6. Sensitive Field Exclusion

The following MUST NEVER appear in log entries:

| Field Category | Examples |
|---------------|---------|
| Session tokens | `session_token`, `bearer_token`, `Authorization` header value |
| Passwords | `password`, `password_hash`, `new_password` |
| Entity fields with `Sensitive: true` | Varies by entity (e.g., `national_id`, `bank_account`) |
| JWT secrets | `jwt_secret`, signing keys |
| API keys | `stripe_api_key`, third-party credentials |
| Credit card data | `card_number`, `cvv`, `expiry` |
| Personally identifiable information | KENYAN_ID numbers, passport numbers |

When logging entity records for debugging, ALWAYS use `EntityRecord.ToLogMap()` which applies sensitive field exclusion:

```go
slog.Debug("record state",
    "record", rec.ToLogMap(),  // safe — Sensitive fields excluded
)
// NEVER: slog.Debug("record", "data", rec.Data)  // may include Sensitive fields
```

---

## 7. ActionRuntime Logger

Action handlers MUST use `action.Runtime.Logger()` — not `slog.Default()`. The runtime logger is pre-seeded with:

```json
{
  "tenant_id":   "...",
  "user_id":     "...",
  "entity_name": "finance_invoice",
  "action_name": "submit",
  "request_id":  "..."
}
```

All entries from the handler are automatically correlated to the request.

---

## 8. Workflow/Activity Logging

Temporal activities MUST use the activity's context logger:

```go
func (a *Activities) SendWelcomeEmailActivity(ctx context.Context, input Input) error {
    logger := activity.GetLogger(ctx)
    logger.Info("sending welcome email", "user_id", input.UserID)
    // ...
}
```

Do NOT use `slog.Default()` in activities — it lacks the Temporal workflow/activity correlation fields.

---

## 9. Log Sampling in Production

For high-volume paths (list queries, health checks), apply log sampling to avoid log volume overwhelming storage:

```go
// Sample 1% of successful list requests at DEBUG level
if rand.Float64() < 0.01 {
    slog.Debug("list query", "entity", entityName, "count", count)
}
```

All `ERROR` and `WARN` entries are NEVER sampled — they are always emitted.

---

## References

- [`17-observability/METRICS_SPEC.md`](METRICS_SPEC.md) — Prometheus metrics
- [`17-observability/HEALTH_CHECKS.md`](HEALTH_CHECKS.md) — Health check endpoints
- [`18-security/SENSITIVE_FIELDS.md`](../18-security/SENSITIVE_FIELDS.md) — Sensitive field semantics
