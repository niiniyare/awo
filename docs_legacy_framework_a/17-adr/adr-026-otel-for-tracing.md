> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "ADR-026: OpenTelemetry for Distributed Tracing"
id: adr-026
status: accepted
category: ADR
stability: STABLE
audience: [framework-authors, operators]
since: "1.0"
normative-level: informative
related:
  - "[Distributed Tracing](../13-observability/tracing.md)"
  - "[Observability](../13-observability/observability.md)"
  - "[ADR-025](adr-025-go-slog-structured-logging.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# ADR-026: OpenTelemetry for Distributed Tracing

**Status**: Accepted
**Date**: 2024-02-15
**Deciders**: Awo Framework Team

---

## Context

Awo's request path spans multiple components: Fiber HTTP handler → Domain hooks → PostgreSQL → Redis → Temporal activity. Debugging latency issues or errors in production requires tracing a request across all these hops.

Without distributed tracing:
- "Invoice submission is slow" requires log correlation across multiple services manually
- Identifying which DB query is the bottleneck is guesswork
- Cross-service debugging requires reproducing in staging

Options considered:

1. **No tracing** — rely on logs and metrics only
2. **OpenTelemetry (OTel)** — vendor-neutral, standard-library-first
3. **Jaeger SDK directly** — vendor-specific
4. **Datadog APM SDK** — proprietary, cloud-only pricing

---

## Decision

Use **OpenTelemetry (OTel)** SDK for all distributed tracing. Export via OTLP (gRPC) to either Jaeger or Grafana Tempo (operator's choice).

---

## Consequences

### Positive

- **Vendor-neutral**: switch backends without code changes (Jaeger ↔ Tempo ↔ Honeycomb)
- **Auto-instrumentation**: OTel contrib libraries instrument Fiber, pgx, Redis automatically
- **Standard**: W3C TraceContext propagation works with any downstream that supports it
- **Trace-log correlation**: inject trace_id/span_id into slog entries — Grafana links logs to traces
- **Sampling control**: probability-based sampling configurable per environment via env vars (no code change)

### Negative

- **SDK footprint**: `go.opentelemetry.io/otel` adds ~3MB to binary size
- **Cold start overhead**: tracer provider initialization at startup (~10ms — acceptable)
- **Context threading**: every function that creates spans must receive `context.Context` — already required by our conventions

### Neutral

- Jaeger and Grafana Tempo are both supported backends; the framework does not mandate one
- OTel is CNCF graduated — long-term maintenance guaranteed

---

## Sampling Strategy

| Environment | Sampler | Rate |
|---|---|---|
| Development | `always_on` | 100% |
| Staging | `parentbased_traceidratio` | 50% |
| Production | `parentbased_traceidratio` | 5% |
| Production errors | Custom `errorAlwaysSampler` | 100% |

Error spans are always sampled (custom sampler wraps the base sampler). This ensures every error has a trace in Jaeger regardless of the global sampling rate.

---

## Alternatives Rejected

### No Tracing

Rejected: latency debugging in production without traces requires log correlation at scale — too slow and imprecise. The 5% production sampling rate makes OTel overhead negligible.

### Jaeger SDK Directly

Rejected: vendor lock-in. Migrating to Tempo or Honeycomb would require SDK changes across all instrumented code.

### Datadog APM

Rejected: proprietary SDK, cloud-only pricing, requires Datadog agent sidecar. OTel can export to Datadog if needed without SDK changes.

---

## Related Documents

- [Distributed Tracing](../13-observability/tracing.md) — implementation guide
- [Structured Logging](../13-observability/structured-logging.md) — trace/span ID in log entries
- [ADR-025](adr-025-go-slog-structured-logging.md) — slog for log correlation
