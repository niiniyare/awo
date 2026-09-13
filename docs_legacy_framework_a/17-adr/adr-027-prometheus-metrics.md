> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "ADR-027: Prometheus for Metrics"
id: adr-027
status: accepted
category: ADR
stability: STABLE
audience: [framework-authors, operators]
since: "1.0"
normative-level: informative
related:
  - "[Metrics Reference](../13-observability/metrics-reference.md)"
  - "[Alerting](../13-observability/alerting.md)"
  - "[ADR-026](adr-026-otel-for-tracing.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# ADR-027: Prometheus for Metrics

**Status**: Accepted
**Date**: 2024-02-20
**Deciders**: Awo Framework Team

---

## Context

Awo needs metrics for:
- SLO monitoring (error rate, latency percentiles)
- Alerting (service down, Redis failure, Temporal worker failure)
- Capacity planning (DB connection pool usage, request rate trends)
- Business metrics (tenant count, workflow success rate)

Options considered:

1. **Prometheus + Grafana** — open-source, pull-based, widely deployed
2. **StatsD + Graphite** — push-based, older ecosystem
3. **OpenTelemetry Metrics** — standard, but OTel metrics SDK is less mature than tracing SDK
4. **Datadog Metrics** — proprietary, SaaS-only cost

---

## Decision

Use **Prometheus** (`github.com/prometheus/client_golang`) for all metrics. Expose at `GET /metrics`. Grafana for dashboards. Alertmanager for alerting rules.

---

## Consequences

### Positive

- **Standard**: Prometheus is the de facto Kubernetes metrics standard
- **Pull model**: Prometheus scrapes the `/metrics` endpoint — simpler than push (no agent required)
- **Rich ecosystem**: `kube-prometheus-stack` Helm chart deploys Prometheus + Grafana + Alertmanager + node-exporter in one command
- **Histograms**: latency percentiles (p50, p95, p99) built into Prometheus — critical for SLO monitoring
- **PromQL**: powerful query language for dashboards and alert rules
- **Cost**: open-source; runs in the cluster alongside the application

### Negative

- **Cardinality risk**: high-cardinality labels (per-tenant metrics) can blow up Prometheus memory — must be careful with label design
- **Short-term retention**: Prometheus default retention is 15 days — long-term storage requires Thanos or Grafana Mimir
- **Pull model complexity**: Prometheus must be able to reach the service — not suitable for off-cluster telemetry without remote-write

### Neutral

- OTel metrics SDK can export to Prometheus — if we adopt OTel metrics in future, no Grafana/Alertmanager changes needed
- Tenant-level metrics use tenant-count buckets (not per-tenant labels) to avoid cardinality explosion

---

## Label Design Constraints

To prevent cardinality explosion:

```go
// WRONG: per-tenant label = N tenants × M metrics = unbounded
httpRequestsTotal.WithLabelValues(tenantID.String(), method, path, status).Inc()

// CORRECT: tenant ID excluded from metric labels
httpRequestsTotal.WithLabelValues(method, path, status).Inc()

// Aggregate tenant counts separately at low cardinality
activeTenantsGauge.Set(float64(activeTenantCount))
```

Per-tenant breakdown is available in logs (structured with tenant_id) and traces. Metrics provide aggregate views only.

---

## Alternatives Rejected

### StatsD + Graphite

Rejected: push model requires a StatsD agent; Graphite lacks PromQL expressiveness; Grafana integrates better with Prometheus.

### OpenTelemetry Metrics Only

Rejected: OTel metrics Go SDK is less stable than the Prometheus client library as of 2024. We use OTel for tracing (ADR-026) but Prometheus client directly for metrics. This may change when OTel metrics stabilizes.

### Datadog

Rejected: proprietary, per-host pricing, SaaS dependency. Prometheus is self-hosted.

---

## Related Documents

- [Metrics Reference](../13-observability/metrics-reference.md) — complete metric catalog
- [Alerting](../13-observability/alerting.md) — Prometheus alert rules and Alertmanager routing
