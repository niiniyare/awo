> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Structured Logging Guide"
id: obs-002
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors, operators]
since: "1.0"
normative-level: normative
related:
  - "[Observability](observability.md)"
  - "[Security Model](../15-security/security-model.md)"
  - "[Fields](../04-domain/fields.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Structured Logging Guide

**OBS-002 | Status: Accepted | Stability: Stable**

This document specifies logging conventions for module authors: required fields, log levels, sensitive field exclusion, and error logging patterns.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. Logger Setup

```go
// All logging uses log/slog with JSON handler
import "log/slog"

// Initialized at startup from config
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: cfg.LogLevel,  // "debug", "info", "warn", "error"
}))
slog.SetDefault(logger)
```

All log output is JSON to stdout. Log aggregation (e.g., Datadog, CloudWatch, Loki) consumes from stdout.

---

## 2. Required Fields per Request

Every log entry within a request context MUST include:

| Field | Type | Source |
|---|---|---|
| `request_id` | string | `X-Request-ID` header or generated UUID |
| `tenant_id` | string (UUID) | Resolved by tenant middleware |
| `user_id` | string (UUID) | Resolved by session middleware |
| `method` | string | HTTP method |
| `path` | string | Request path (no query string) |

These are injected by the request logging middleware. Module code does not need to add them manually — use the context-aware logger:

```go
// Extract logger from context (carries request fields)
log := slog.With(
    "request_id", c.Locals("request_id"),
    "tenant_id",  tenant.IDFromContext(c.Context()),
    "user_id",    session.ActorFromContext(c.Context()).UserID,
)
```

---

## 3. Log Levels

| Level | When to use |
|---|---|
| `Debug` | Detailed execution traces (disabled in production; enable per-request via header) |
| `Info` | Successful operations, state transitions, notable events |
| `Warn` | Recoverable problems, degraded behavior, near-limit conditions |
| `Error` | Failures requiring investigation; always include `"err"` attribute |

```go
// Info: state transition
slog.InfoContext(ctx, "invoice submitted",
    "invoice_id", invoiceID,
    "amount",     amount.String(),
)

// Warn: recoverable issue
slog.WarnContext(ctx, "email notification failed, continuing",
    "invoice_id", invoiceID,
    "err",        err,
)

// Error: unexpected failure
slog.ErrorContext(ctx, "failed to post ledger entry",
    "invoice_id",  invoiceID,
    "request_id",  requestID,
    "tenant_id",   tenantID,
    "err",         err,
)
```

Never use `fmt.Println` or `log.Println` — use `slog` exclusively.

---

## 4. Sensitive Field Exclusion

Fields declared `Sensitive: true` on a `FieldDef` MUST NEVER be logged:

```go
// WRONG: logging sensitive field value
slog.Info("user created", "password_hash", user.PasswordHash)  // never log

// CORRECT: log only non-sensitive identifiers
slog.Info("user created", "user_id", user.ID, "email", user.Email)
```

Sensitive fields include (but are not limited to): `password_hash`, `session_token`, `reset_token`, `kra_pin`, `tax_pin`, `bank_account_number`, `client_secret`.

If in doubt: if it's a credential, secret, or regulated personal identifier — do not log the value. Log only the field name and whether it was set:

```go
slog.Info("user credentials updated",
    "user_id",        userID,
    "password_changed", true,  // ok: boolean flag
    // NOT: "new_password_hash": hash
)
```

---

## 5. Error Logging Pattern

Always log errors with full context at the point where the error is handled (not where it's returned):

```go
// WRONG: log at every return point
func (s *Service) CreateInvoice(ctx context.Context, input CreateInput) (*Invoice, error) {
    invoice, err := s.repo.Create(ctx, input)
    if err != nil {
        slog.Error("create failed", "err", err)  // too early — no request context
        return nil, err
    }
    return invoice, nil
}

// CORRECT: log at the handler layer where request context is available
func CreateInvoiceHandler(c *fiber.Ctx) error {
    invoice, err := svc.CreateInvoice(c.Context(), input)
    if err != nil {
        slog.ErrorContext(c.Context(), "create invoice failed",
            "request_id", c.Locals("request_id"),
            "tenant_id",  tenant.IDFromContext(c.Context()),
            "err",        err,
        )
        return mapError(c, err)
    }
    return c.JSON(SuccessEnvelope{Data: invoice})
}
```

Never log and re-return the same error — log once at the top of the call stack where you have context.

---

## 6. Audit vs Application Logs

| Log type | System | Purpose |
|---|---|---|
| **Application logs** | `slog` → stdout | Debugging, performance, errors |
| **Audit log** | `iam_audit_log` DB table | Regulatory compliance, tamper-evident record |

Do not confuse them. Application logs may be rotated, sampled, or lost under high load. The audit log is durable, immutable, and retained for compliance purposes.

Module code does not write to the audit log directly — the framework writes an audit entry for every entity mutation automatically.

---

## 7. Debug Logging in Production

Debug logs are disabled in production (`LOG_LEVEL=info`). To enable debug logs for a specific request without changing global log level, send the `X-Debug-Log: true` header:

```
GET /api/v1/entities/finance_invoice HTTP/1.1
X-Debug-Log: true
```

The middleware enables debug logging for that request only (single-request scope). This header is only honored from IPs in the operator allowlist — it has no effect from other sources.

---

## 8. Log Sampling for High-Volume Paths

For endpoints called >1000 times/minute, log only a sample to avoid log volume overwhelming aggregation systems:

```go
// Sample 1% of successful read operations
if rand.Intn(100) == 0 {
    slog.InfoContext(ctx, "entity list query",
        "entity_type", entityType,
        "result_count", len(results),
        "duration_ms", duration.Milliseconds(),
    )
}

// Always log errors (no sampling)
if err != nil {
    slog.ErrorContext(ctx, "entity list query failed", "err", err)
}
```

Errors MUST NEVER be sampled. Every error gets a log entry.

---

## Related Documents

- [Observability](observability.md) — metrics, tracing, health checks
- [Security Model](../15-security/security-model.md) — sensitive data handling
- [Input Validation](../15-security/input-validation.md) — sensitive field definition
- [Glossary](../GLOSSARY.md) — Structured Logging, Audit Log
