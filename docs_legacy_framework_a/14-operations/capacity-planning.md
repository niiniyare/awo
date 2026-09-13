> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Capacity Planning"
id: ops-009
status: accepted
category: GUIDE
stability: STABLE
audience: [operators]
since: "1.0"
normative-level: informative
related:
  - "[Performance Tuning](performance-tuning.md)"
  - "[Metrics Reference](../13-observability/metrics-reference.md)"
  - "[Alerting](../13-observability/alerting.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Capacity Planning

**OPS-009 | Status: Accepted | Stability: Stable**

Sizing Awo deployments: baseline resources, scaling triggers, database sizing, Redis sizing, and Temporal worker capacity.

---

## 1. Component Sizing Reference

### Awo API Server

| Tenant Count | Daily Requests | API Replicas | CPU per Pod | Memory per Pod |
|---|---|---|---|---|
| 1–10 | < 50K | 2 | 0.5 CPU | 256 MB |
| 10–50 | < 500K | 3 | 1 CPU | 512 MB |
| 50–200 | < 2M | 5 | 2 CPU | 1 GB |
| 200–500 | < 10M | 8+ | 2 CPU | 2 GB |

These are starting points. Monitor actual CPU and memory utilization and scale at 70% saturation.

### PostgreSQL

| Tenant Count | Estimated DB Size/Year | CPU | Memory | Storage |
|---|---|---|---|---|
| 1–10 | < 5 GB | 2 vCPU | 4 GB | 50 GB SSD |
| 10–50 | < 50 GB | 4 vCPU | 16 GB | 200 GB SSD |
| 50–200 | < 500 GB | 8 vCPU | 32 GB | 1 TB SSD |
| 200–500 | < 2 TB | 16 vCPU | 64 GB | 4 TB SSD |

`shared_buffers` = 25% of RAM. `effective_cache_size` = 75% of RAM. Use `pg_stat_user_tables` to identify tables needing vacuuming/indexing.

### Redis

Redis memory usage per 1000 tenants:

| Data Type | Size |
|---|---|
| Session tokens (avg 10 active sessions/tenant) | ~50 MB |
| Feature flag cache (avg 20 flags/tenant) | ~10 MB |
| Page schema cache (avg 15 entity types/tenant) | ~30 MB |
| Rate limiting state | ~5 MB |
| **Total** | **~95 MB per 1000 tenants** |

Starting size: 1 GB Redis handles ~10K tenants comfortably. Enable eviction policy `allkeys-lru` — Redis may evict cache entries under memory pressure (sessions are NOT evicted: they have TTLs).

### Temporal Workers

| Workflow Volume | Workers | CPU per Worker | Memory |
|---|---|---|---|
| < 100 workflows/day | 1 | 0.5 CPU | 256 MB |
| 100–1000/day | 2 | 1 CPU | 512 MB |
| 1000–10K/day | 3–5 | 2 CPU | 1 GB |
| > 10K/day | 5+ | 2 CPU | 2 GB |

Temporal workers are stateless. Scale horizontally. Each worker handles multiple task queues.

---

## 2. PostgreSQL Connection Sizing

PgBouncer manages connections. Formula:

```
max_client_conn = (API replicas × goroutines_per_pod) + 20% headroom
pool_size = min(max_client_conn, postgresql_max_connections × 0.8)
```

Example:
- 5 API pods × 100 goroutines/pod = 500 potential concurrent connections
- PgBouncer `max_client_conn = 600`, `pool_size = 100` (maps 500+ clients to 100 backend connections)
- PostgreSQL `max_connections = 200` (100 for app, 100 for Temporal + migrations + monitoring)

Monitor `pgbouncer_pool_cl_active` and `pgbouncer_pool_sv_active`. If `sv_active ≈ pool_size` consistently, increase `pool_size` (limited by PostgreSQL `max_connections`).

---

## 3. Scaling Triggers

Set up auto-scaling (Kubernetes HPA) based on:

| Metric | Scale Up At | Scale Down At |
|---|---|---|
| CPU utilization | > 70% | < 30% |
| Memory utilization | > 80% | < 40% |
| `http_request_duration_p99` | > 500ms | — |
| `db_query_duration_p99` | > 100ms | — (not scalable by adding pods) |

Database query latency above threshold → not an application scaling problem. Investigate query plans, indexes, or PgBouncer pool exhaustion.

---

## 4. Database Growth Estimation

Key tables and their growth rates:

| Table | Rows/month (per active tenant) | Size/row |
|---|---|---|
| `iam_audit_log` | ~5000 (all entity mutations) | ~500 bytes |
| `finance_invoice` | ~200 | ~2 KB |
| `finance_journal_entry` | ~500 | ~1 KB |
| `hr_attendance` | ~500 (daily per employee) | ~200 bytes |
| `inventory_stock_move` | ~1000 | ~500 bytes |

Estimate for 100 tenants after 1 year:

```
iam_audit_log:            100 × 5000 × 12 × 500B  ≈ 3 GB
finance_invoice:      100 × 200 × 12 × 2KB    ≈ 480 MB
journal_entries:      100 × 500 × 12 × 1KB    ≈ 600 MB
Total (all tables):   ≈ 10–20 GB/year
```

Factor in: JSONB custom_fields (variable), indexes (~30% of table size), WAL overhead.

---

## 5. Audit Log Retention

The audit log grows fastest. Configure retention:

```bash
# AuditRetentionWorkflow runs daily (see scheduled-workflows.md)
# Default: 365 days
# For compliance-heavy tenants: 2555 days (7 years)
```

With default 365-day retention and 100 active tenants:
- Audit log stabilizes at ~3 GB
- Without retention: grows indefinitely at ~250 MB/month per 100 tenants

---

## 6. Redis Memory Monitoring

Monitor `redis_memory_used_bytes` vs `redis_memory_max_bytes`. Alert at 80%:

```yaml
# Prometheus alert
- alert: RedisMemoryHigh
  expr: redis_memory_used_bytes / redis_memory_max_bytes > 0.8
  for: 5m
  annotations:
    summary: "Redis memory at {{ $value | humanizePercentage }}"
    runbook: "Increase Redis maxmemory or reduce cache TTLs"
```

If eviction rate (`redis_evicted_keys_total`) is > 0, keys are being evicted under memory pressure. Increase Redis memory before sessions are affected.

---

## 7. Temporal Capacity

Monitor Temporal metrics in Grafana:

| Metric | Concern |
|---|---|
| `temporal_workflow_task_schedule_to_start_latency_p99 > 5s` | Worker capacity insufficient |
| `temporal_activity_execute_latency_p99 > 30s` | Activity bottleneck |
| `temporal_workflow_failed_total` rising | Investigate workflow errors |
| `temporal_history_size > 50MB` | Workflow has too many events — refactor |

Add Temporal worker replicas when `task_schedule_to_start_latency` exceeds 5 seconds.

---

## 8. Pre-Scaling Checklist

Before adding capacity:

1. [ ] Identify the bottleneck (CPU, memory, DB connections, disk I/O, Temporal queue)
2. [ ] Check `pg_stat_activity` for blocking queries
3. [ ] Check PgBouncer wait queue (`pgbouncer_pool_cl_waiting > 0`)
4. [ ] Review slow query log (`pg_stat_statements` — queries > 100ms)
5. [ ] Verify indexes exist for all filtered columns (`pg_stat_user_indexes`)
6. [ ] Check Redis eviction rate
7. [ ] Only then: scale horizontally

Throwing more pods at a slow query does not help. Fix the query first.

---

## Related Documents

- [Performance Tuning](performance-tuning.md) — query optimization, index strategy
- [Metrics Reference](../13-observability/metrics-reference.md) — all Prometheus metrics used here
- [Alerting](../13-observability/alerting.md) — alert rules for scaling triggers
- [Deployment](deployment.md) — Kubernetes HPA configuration
