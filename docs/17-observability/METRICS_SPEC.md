# Metrics Specification

**Classification:** Specification — Tier 1
**Owner:** `17-observability/METRICS_SPEC.md`
**Status:** Frozen at v1.0

---

## Purpose

This document specifies the Prometheus metrics exposed by the Awo Framework server at `/metrics`, including metric names, types, label conventions, and alerting thresholds.

---

## 1. Metrics Endpoint

```
GET /metrics
```

Format: Prometheus text format (OpenMetrics compatible).
Authentication: None required (metrics endpoint is internal-only; exposed only to monitoring subnet via network policy).

---

## 2. HTTP Metrics

### `http_request_duration_seconds`

Type: Histogram
Labels: `method`, `path_template`, `status_code`

Measures end-to-end request duration from first byte received to last byte sent.

```
http_request_duration_seconds_bucket{method="POST",path_template="/api/v1/entities/:type",status_code="201",le="0.1"} 1423
http_request_duration_seconds_sum{...} 89.4
http_request_duration_seconds_count{...} 1500
```

**`path_template`** — use parameterised path, not actual path. `/api/v1/entities/finance-invoice/abc123` → `/api/v1/entities/:type/:id`. Never use the actual entity ID in a label — high cardinality.

Buckets: `0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10`

### `http_requests_in_flight`

Type: Gauge
Labels: `method`, `path_template`

Current number of requests being processed.

---

## 3. Database Metrics

### `db_query_duration_seconds`

Type: Histogram
Labels: `entity`, `operation`

Operations: `list`, `get`, `create`, `update`, `delete`, `count`, `exists`

```
db_query_duration_seconds_bucket{entity="finance_invoice",operation="list",le="0.05"} 980
```

### `db_connections_open`

Type: Gauge
Labels: none

Current number of open PostgreSQL connections in the pool.

### `db_connections_idle`

Type: Gauge

Idle connections in the pool.

---

## 4. Cache Metrics

### `cache_hit_total`

Type: Counter
Labels: `cache_key_type`

`cache_key_type` values: `page_schema`, `feature_flag`, `session`, `idempotency`

### `cache_miss_total`

Type: Counter
Labels: `cache_key_type`

---

## 5. Workflow Metrics

### `workflow_started_total`

Type: Counter
Labels: `workflow_fn`, `tenant_id`

**Note:** `tenant_id` as a label is high cardinality on large deployments. Consider using `tenant_tier` (a tenant classification) instead for high-scale deployments.

### `workflow_dispatch_failed_total`

Type: Counter
Labels: `workflow_fn`

Incremented when the outbox worker fails to dispatch a workflow to Temporal.

### `workflow_outbox_pending`

Type: Gauge
Labels: none

Current number of `workflow_outbox` records with `status = 'pending'`.

**Alert threshold:** > 100 pending for > 5 minutes → PagerDuty (Temporal worker may be down).

---

## 6. Event Metrics

### `event_published_total`

Type: Counter
Labels: `topic`

### `event_delivery_failed_total`

Type: Counter
Labels: `topic`

### `event_outbox_pending`

Type: Gauge

Current pending events in `event_outbox`.

**Alert threshold:** > 500 pending for > 10 minutes.

---

## 7. Rate Limiting Metrics

### `rate_limit_exceeded_total`

Type: Counter
Labels: `tenant_id` (sampled — not every tenant)

---

## 8. Alerting Thresholds

| Metric | Threshold | Severity | Action |
|--------|-----------|----------|--------|
| `http_request_duration_seconds` p99 > 2s | 5 min | WARNING | Investigate slow queries |
| `http_request_duration_seconds` p99 > 5s | 2 min | CRITICAL | Page oncall |
| `db_query_duration_seconds` p95 > 500ms | 5 min | WARNING | Check query plans |
| Error rate (5xx) > 1% | 2 min | CRITICAL | Page oncall |
| `workflow_outbox_pending` > 100 | 5 min | WARNING | Check Temporal worker |
| `event_outbox_pending` > 500 | 10 min | WARNING | Check EventBroker |
| `db_connections_open` > 90% pool size | 5 min | WARNING | Connection pool pressure |

---

## 9. Label Cardinality Rules

- **Never** use entity record IDs, user IDs, or session tokens as metric labels — unbounded cardinality.
- **Never** use raw request paths as labels — use `path_template` with parameters replaced.
- `tenant_id` labels: acceptable for counters on low-tenant deployments; use `tenant_tier` on >1000 tenants.
- Limit distinct label value count per metric to < 100 in production.

---

## References

- [`17-observability/HEALTH_CHECKS.md`](HEALTH_CHECKS.md) — Health probe contracts
- [`17-observability/LOGGING_SPEC.md`](LOGGING_SPEC.md) — Structured logging
