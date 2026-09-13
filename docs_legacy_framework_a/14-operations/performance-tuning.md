> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Performance Tuning"
id: ops-004
status: accepted
category: GUIDE
stability: STABLE
audience: [operators]
since: "1.0"
normative-level: informative
related:
  - "[Deployment](deployment.md)"
  - "[Observability](../13-observability/observability.md)"
  - "[Cursors and Pagination](../05-persistence/cursors-and-pagination.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Performance Tuning

**OPS-004 | Status: Accepted | Stability: Stable**

Tuning reference for PostgreSQL, Redis, PgBouncer, and the Awo process itself.

---

## 1. PostgreSQL

### Connection Pooling (PgBouncer)

PgBouncer MUST run in transaction mode. Pool size formula:

```
pool_size = (num_cpu_cores * 2) + effective_spindle_count
```

For a 4-core server with SSD: `pool_size = 9`. Start here; increase if `cl_waiting > 0` in `SHOW POOLS`.

```ini
# pgbouncer.ini
pool_mode = transaction
max_client_conn = 1000
default_pool_size = 9
reserve_pool_size = 2
```

### Key Indexes

Every tenant-scoped table needs at minimum:

```sql
-- Tenant isolation index (covered by RLS policy — but B-tree still needed for performance)
CREATE INDEX CONCURRENTLY ON finance_invoice (tenant_id, created_at DESC);

-- GIN on JSONB for custom entities
CREATE INDEX CONCURRENTLY ON my_custom_entity USING GIN (data jsonb_path_ops);

-- GIN trigram on searchable text fields
CREATE INDEX CONCURRENTLY ON crm_contact USING GIN (name gin_trgm_ops);
```

Framework auto-generates GIN indexes for `Searchable()` fields and JSONB columns. Verify with:

```sql
SELECT indexname, indexdef FROM pg_indexes WHERE tablename = 'crm_contact';
```

### `work_mem` for Sort-Heavy Queries

If `EXPLAIN ANALYZE` shows `Sort Method: external merge` (disk sort):

```sql
-- Session-level (per query)
SET work_mem = '64MB';
EXPLAIN ANALYZE SELECT ...;
```

If disk sorts are common, raise `work_mem` in `postgresql.conf`. Default (4MB) is too low for large ERP datasets.

### `autovacuum` Tuning

High-write tables (ledger entries, audit log) need aggressive autovacuum:

```sql
ALTER TABLE finance_ledger_entry SET (
    autovacuum_vacuum_scale_factor = 0.01,   -- vacuum after 1% of rows changed
    autovacuum_analyze_scale_factor = 0.005
);
```

---

## 2. Redis

### Connection Pool

Configure the Redis client pool to match peak concurrency:

```go
// config
RedisPoolSize:    runtime.NumCPU() * 4,
RedisMinIdleConns: runtime.NumCPU(),
```

### Key TTL Review

Ensure all Redis keys have TTLs. Keys without TTLs grow unbounded:

```
redis-cli --scan --pattern "*" | xargs -L 1 redis-cli TTL | grep -c "^-1"
# Count of keys without TTL — should be 0
```

Expected TTLs by key type:

| Key pattern | Expected TTL |
|---|---|
| `session:{token}` | Session expiry (e.g., 8h) |
| `page:{entity}:*` | 300s |
| `eval:{sha256}` | 300s |
| `rl:{tenant}:{user}:*` | Rate limit window |

---

## 3. Fiber HTTP Server

```go
app := fiber.New(fiber.Config{
    ReadTimeout:  10 * time.Second,
    WriteTimeout: 30 * time.Second,
    // Disable Prefork — incompatible with Temporal worker in same process
    Prefork: false,
    // Reduce allocations for high-traffic deployments
    ReduceMemoryUsage: true,
})
```

For CPU-bound pages (complex SDUI schema generation), cache aggressively in Redis — the 5-minute page schema TTL is the primary mitigation.

---

## 4. Temporal Worker

```go
workerOptions := worker.Options{
    MaxConcurrentWorkflowTaskPollers:  4,
    MaxConcurrentActivityTaskPollers:  8,
    MaxConcurrentActivityExecutionSize: 100,
}
```

- Workflow pollers: match number of unique workflow types registered
- Activity pollers: match expected concurrent external I/O (email, payment APIs)
- If Temporal Web UI shows `scheduleToStartLatency` > 1s: increase pollers

---

## 5. Kubernetes Resource Limits

Recommended starting point for a medium-load tenant (100 active users):

```yaml
resources:
  requests:
    cpu:    "500m"
    memory: "256Mi"
  limits:
    cpu:    "2000m"
    memory: "512Mi"
```

Memory limit: Awo processes are memory-stable if Redis keys have TTLs and no goroutine leaks exist. If OOMKilled, check for missing TTLs or goroutine leaks first before raising the limit.

---

## Related Documents

- [Deployment](deployment.md) — Kubernetes manifest, health checks
- [Observability](../13-observability/observability.md) — metrics to monitor
- [Troubleshooting](troubleshooting.md) — slow query diagnosis
