> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Metrics Reference"
id: obs-003
status: accepted
category: SPEC
stability: STABLE
audience: [operators]
since: "1.0"
normative-level: normative
related:
  - "[Observability](observability.md)"
  - "[Deployment](../14-operations/deployment.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Metrics Reference

**OBS-003 | Status: Accepted | Stability: Stable**

Complete reference for all Prometheus metrics exposed by Awo at `/metrics`.

---

## 1. HTTP Metrics

### `http_request_duration_seconds`

Type: Histogram
Buckets: 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10

Labels:
- `method` — HTTP method (GET, POST, PATCH, DELETE)
- `path` — URL path template (not the actual path, to avoid high cardinality)
- `status` — HTTP status code

```promql
# P99 latency for POST /api/v1/entities/finance_invoice
histogram_quantile(0.99,
  rate(http_request_duration_seconds_bucket{
    method="POST",
    path="/api/v1/entities/finance_invoice"
  }[5m])
)
```

### `http_requests_total`

Type: Counter
Labels: `method`, `path`, `status`

### `http_active_requests`

Type: Gauge
Labels: `method`, `path`

---

## 2. Database Metrics

### `db_query_duration_seconds`

Type: Histogram
Labels:
- `entity` — entity type name
- `operation` — `get`, `query`, `create`, `update`, `delete`, `count`, `aggregate`

```promql
# Slow query detection: P95 > 200ms
histogram_quantile(0.95,
  rate(db_query_duration_seconds_bucket{operation="query"}[5m])
) > 0.2
```

### `db_pool_connections`

Type: Gauge
Labels: `state` — `active`, `idle`, `waiting`

```promql
# Alert: connections waiting > 10 for >1 minute
db_pool_connections{state="waiting"} > 10
```

### `db_query_errors_total`

Type: Counter
Labels: `entity`, `operation`, `error_code`

---

## 3. Cache Metrics

### `cache_operations_total`

Type: Counter
Labels:
- `key_type` — `session`, `page_schema`, `feature_flag`, `rate_limit`
- `result` — `hit`, `miss`, `error`

```promql
# Cache hit rate for page schemas
rate(cache_operations_total{key_type="page_schema", result="hit"}[5m]) /
rate(cache_operations_total{key_type="page_schema"}[5m])
```

### `cache_operation_duration_seconds`

Type: Histogram
Labels: `key_type`, `operation` (`get`, `set`, `del`)

---

## 4. Workflow Metrics

### `workflow_started_total`

Type: Counter
Labels: `workflow_type`, `task_queue`, `tenant_id` (hashed)

### `workflow_completed_total`

Type: Counter
Labels: `workflow_type`, `status` — `succeeded`, `failed`, `cancelled`

### `workflow_duration_seconds`

Type: Histogram
Labels: `workflow_type`

### `activity_started_total`

Type: Counter
Labels: `activity_type`, `task_queue`

### `activity_duration_seconds`

Type: Histogram
Labels: `activity_type`, `status` — `succeeded`, `failed`, `retried`

### `outbox_pending_total`

Type: Gauge — current count of outbox rows not yet dispatched

```promql
# Alert: outbox backed up
outbox_pending_total > 100
```

### `outbox_dispatch_duration_seconds`

Type: Histogram — time from outbox row creation to Temporal StartWorkflow call

---

## 5. Entity Metrics

### `entity_operations_total`

Type: Counter
Labels: `entity_type`, `operation` — `created`, `updated`, `deleted`, `action`

### `entity_validation_errors_total`

Type: Counter
Labels: `entity_type`, `field`

High count on a specific field indicates a UX issue or API client misconfiguration.

---

## 6. IAM / Session Metrics

### `session_created_total`

Type: Counter
Labels: `auth_method` — `password`, `api_client`, `mfa_totp`, `mfa_email`

### `session_expired_total`

Type: Counter

### `auth_failures_total`

Type: Counter
Labels: `reason` — `invalid_credentials`, `mfa_failed`, `account_locked`, `tenant_suspended`

```promql
# Alert: brute force detection
rate(auth_failures_total{reason="invalid_credentials"}[5m]) > 10
```

### `rate_limit_exceeded_total`

Type: Counter
Labels: `scope` — `tenant`, `user`, `api_client`

---

## 7. Recommended Alert Rules

```yaml
# prometheus/alerts/awo.yaml
groups:
- name: awo
  rules:
  - alert: HighErrorRate
    expr: rate(http_requests_total{status=~"5.."}[5m]) / rate(http_requests_total[5m]) > 0.01
    for: 2m
    labels:
      severity: warning
    annotations:
      summary: "Error rate above 1% for 2 minutes"

  - alert: SlowP99Latency
    expr: histogram_quantile(0.99, rate(http_request_duration_seconds_bucket[5m])) > 2
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "P99 HTTP latency > 2 seconds"

  - alert: DBConnectionsExhausted
    expr: db_pool_connections{state="waiting"} > 5
    for: 1m
    labels:
      severity: critical
    annotations:
      summary: "Database connection pool backing up"

  - alert: OutboxBacklog
    expr: outbox_pending_total > 100
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "Workflow outbox not being dispatched"

  - alert: CacheHitRateDegraded
    expr: rate(cache_operations_total{key_type="session",result="hit"}[5m]) /
          rate(cache_operations_total{key_type="session"}[5m]) < 0.9
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "Session cache hit rate below 90% — Redis may be under pressure"
```

---

## 8. Grafana Dashboard Panels

Recommended dashboard layout (JSON dashboard available in `web/schemas/grafana/awo-overview.json`):

| Row | Panel | Query |
|---|---|---|
| Request Traffic | RPS by status | `rate(http_requests_total[1m])` by `status` |
| Request Traffic | P50/P95/P99 latency | `histogram_quantile` at 0.5, 0.95, 0.99 |
| Database | Query duration P95 | `histogram_quantile(0.95, ...)` by `operation` |
| Database | Pool utilization | `db_pool_connections` by `state` |
| Workflows | Workflow starts/min | `rate(workflow_started_total[1m])` |
| Workflows | Outbox backlog | `outbox_pending_total` |
| Cache | Hit rate by type | Computed rate as above |
| Auth | Auth failures/min | `rate(auth_failures_total[1m])` |

---

## Related Documents

- [Observability](observability.md) — metrics setup, log configuration, health checks
- [Deployment](../14-operations/deployment.md) — `/metrics` endpoint configuration
- [Troubleshooting](../14-operations/troubleshooting.md) — using metrics for diagnosis
