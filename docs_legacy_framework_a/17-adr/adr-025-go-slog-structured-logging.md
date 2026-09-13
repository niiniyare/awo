> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "ADR-025: Go slog for Structured Logging"
id: adr-025
status: accepted
category: ADR
stability: STABLE
audience: [framework-authors, module-authors]
since: "1.0"
normative-level: informative
related:
  - "[Structured Logging Guide](../13-observability/structured-logging.md)"
  - "[Observability](../13-observability/observability.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# ADR-025: Go slog for Structured Logging

**Status**: Accepted
**Date**: 2024-02-01
**Deciders**: Awo Framework Team

---

## Context

Awo needs structured logging (JSON output) for production observability. Multiple Go logging libraries exist:

1. **`log/slog`** (standard library, Go 1.21+) — structured, leveled, zero-alloc
2. **`uber-go/zap`** — high-performance, structured, widely used
3. **`rs/zerolog`** — zero-alloc, chainable API
4. **`sirupsen/logrus`** — popular but slow (reflection-based)

The logging library is used throughout the framework and in every module. The choice affects:
- Import surface (standard library vs third-party)
- API ergonomics for module authors
- Performance (request-level logging)
- Compatibility with the Go ecosystem

---

## Decision

Use **`log/slog`** (Go standard library) for all structured logging.

---

## Consequences

### Positive

- **No dependency**: standard library — no `go.mod` entry, no version management
- **Future-proof**: maintained by the Go team; not subject to abandonment
- **Familiar API**: any Go developer knows `slog.Info("msg", "key", value)` immediately
- **Handler swappable**: JSON handler in production, text handler in development, custom handler for tests
- **Context propagation**: `slog.InfoContext(ctx, ...)` passes trace IDs naturally
- **Structured output**: JSON handler produces machine-parseable output for Loki/Elasticsearch

### Negative

- **Go 1.21 minimum**: slog was added in Go 1.21. This is our minimum Go version requirement.
- **Slightly less ergonomic than zerolog**: zerolog's chained API `log.Info().Str("k","v").Msg("")` is more concise but non-standard
- **Performance**: slog with JSON handler is slightly slower than zerolog (still >1M logs/sec — irrelevant in practice)

### Neutral

- zap and zerolog remain valid choices for performance-critical libraries that are not in Awo's critical path

---

## Alternatives Rejected

### `uber-go/zap`

Rejected because:
- Third-party dependency for functionality available in standard library
- `zap.Field` constructors (`zap.String("k", v)`) are more verbose than `slog` key-value pairs
- slog provides similar performance for our use case (ERP API, not logging-rate-limited)

### `rs/zerolog`

Rejected because:
- Third-party dependency
- Chain API is elegant but unfamiliar to Go newcomers
- slog's key-value API maps naturally to Loki/Elasticsearch label conventions

### `sirupsen/logrus`

Rejected because:
- Slow (reflection-based `WithFields`)
- In maintenance mode — no new features
- slog is strictly superior for new projects

---

## Usage Conventions

All log entries carry standard fields (injected by middleware):

```go
slog.InfoContext(ctx, "request completed",
    "request_id",  requestID,
    "tenant_id",   tenantID.String(),
    "user_id",     userID.String(),
    "method",      method,
    "path",        path,
    "status",      statusCode,
    "duration_ms", duration.Milliseconds(),
)
```

Error logging always includes `"err"` key:

```go
slog.ErrorContext(ctx, "database query failed",
    "err",        err,
    "entity",     "finance_invoice",
    "operation",  "Query",
    "request_id", requestID,
)
```

Sensitive fields (passwords, tokens) are never logged. `Sensitive: true` fields are excluded at the repository layer before any log entry is written.

---

## Handler Configuration

```go
// Development: human-readable text
handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})

// Production: JSON for Loki/Elasticsearch
handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})

slog.SetDefault(slog.New(handler))
```

Log level is configured via `LOG_LEVEL` environment variable (see [Configuration](../12-configuration/environment-variables.md)).

---

## Related Documents

- [Structured Logging Guide](../13-observability/structured-logging.md) — required fields, levels, error logging
- [Distributed Tracing](../13-observability/tracing.md) — trace/span ID injection into log entries
