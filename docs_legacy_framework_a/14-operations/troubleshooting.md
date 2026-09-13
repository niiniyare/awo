> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Troubleshooting Guide"
id: ops-003
status: accepted
category: GUIDE
stability: STABLE
audience: [operators]
since: "1.0"
normative-level: informative
related:
  - "[Deployment](deployment.md)"
  - "[Migrations](migrations.md)"
  - "[Observability](../13-observability/observability.md)"
  - "[Security Model](../15-security/security-model.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Troubleshooting Guide

**OPS-003 | Status: Accepted | Stability: Stable**

This document provides structured diagnostic approaches for common operational issues. Use the symptom-first lookup table to find the relevant section.

---

## Quick Symptom Index

| Symptom | Section |
|---|---|
| All requests return 503 | §1 |
| All auth returns 401 | §2 |
| Tenant requests return 402 or 410 | §3 |
| Migrations fail or hang | §4 |
| Temporal workflows not starting | §5 |
| Page schemas not loading (SDUI blank) | §6 |
| Slow queries / high DB latency | §7 |
| RLS returning wrong data | §8 |
| Redis connection errors | §9 |
| Memory or goroutine leak indicators | §10 |

---

## 1. All Requests Return 503

### Diagnosis

Check the readiness probe first:

```
GET /health/ready
```

If this returns non-200, one or more hard dependencies are down.

### PostgreSQL Down

Symptoms: `{"error": "db_unavailable"}` in response, `"msg": "db ping failed"` in logs.

```
# Check PostgreSQL connection
psql $DATABASE_URL -c "SELECT 1;"

# Check PgBouncer pool status
psql $PGBOUNCER_URL -c "SHOW POOLS;"

# Check if max_connections exhausted
psql $DATABASE_URL -c "SELECT count(*) FROM pg_stat_activity;"
```

Resolution: restore PostgreSQL connectivity, check PgBouncer pool configuration.

### Redis Down

Symptoms: `{"error": "redis_unavailable"}` in logs for session validation operations.

```
# Test Redis
redis-cli -u $REDIS_URL PING
# Expected: PONG
```

Resolution: restore Redis connectivity. See §9 for Redis-specific issues.

### EntityRegistry Failed to Populate

Symptoms: process exits at startup with `FATAL entity registry: validation failed`.

Check startup logs for the specific validation error:

```
entity registry validation failed: entity "finance_invoice" field "customer": LinkTarget "crm_customer" not registered
```

Resolution: ensure all modules are blank-imported in `main.go` in correct dependency order. The linked entity must be registered before the entity that references it.

---

## 2. All Auth Returns 401

### Session Token Expired

Expected behavior: tokens expire per the configured `iam.session_ttl` setting. Client must re-authenticate.

### Redis Session Store Empty

If all sessions appear invalid after a Redis restart:

```
# Check if Redis was restarted with persistence disabled
redis-cli -u $REDIS_URL CONFIG GET save
# If empty, persistence was off — all sessions lost on restart
```

Resolution: for production, enable Redis persistence (AOF or RDB). On restart, all users must re-authenticate. This is correct security behavior.

### Clock Skew

Sessions include `issued_at` and `expires_at` timestamps. If server clock drifts significantly:

```
# Check server time
date -u

# Check time sync status
timedatectl status
```

Ensure NTP is synchronized. Clock skew >5 minutes may cause spurious session expiry.

---

## 3. Tenant Requests Return 402 or 410

| HTTP Status | Tenant Status | Meaning |
|---|---|---|
| 402 | SUSPENDED | Tenant payment overdue |
| 410 | ARCHIVED | Tenant permanently deactivated |
| 503 + Retry-After: 60 | PENDING | Tenant provisioning in progress |

These are correct behavior, not errors. To investigate:

```sql
-- Check tenant status
SELECT id, name, status, suspended_at, archived_at
FROM tenants
WHERE id = $1;
```

For SUSPENDED: contact billing module or restore via tenant lifecycle API.
For ARCHIVED: terminal state — data is retained per data retention policy but the tenant cannot be reactivated.

---

## 4. Migrations Fail or Hang

### Migration Failure

Symptoms: migration runner exits non-zero; last successful migration version in `schema_migrations` table.

```sql
-- Check migration state
SELECT version, dirty FROM schema_migrations ORDER BY version DESC LIMIT 5;
```

If `dirty = true`: the last migration partially applied and failed. Do not re-run — first apply the `.down.sql` for that version, then fix the migration, then re-run.

```
# Apply down migration manually
migrate -path ./db/migration -database $DATABASE_URL down 1
```

Never modify a migration file after it has been applied in any environment. Create a new migration to fix the issue.

### Migration Timeout

Long-running DDL (e.g., `ALTER TABLE ADD COLUMN` on a large table) may timeout.

```sql
-- Check for blocking locks
SELECT pid, query, wait_event_type, wait_event
FROM pg_stat_activity
WHERE wait_event_type = 'Lock';

-- Check for long-running queries
SELECT pid, query, now() - query_start AS duration
FROM pg_stat_activity
WHERE state = 'active'
ORDER BY duration DESC;
```

For production: always use `CREATE INDEX CONCURRENTLY` for new indexes. For column additions, add `DEFAULT NULL` first (instant), then backfill, then add constraint.

### PgBouncer in Session Mode

If `SET LOCAL` commands fail during migration:

```
ERROR: SET LOCAL cannot be used inside a transaction, or when not in a transaction
```

Ensure PgBouncer is in transaction mode, not session mode. Session mode breaks the transaction-local variable reset used by `set_tenant_context()`.

---

## 5. Temporal Workflows Not Starting

### Worker Not Connected

Check Temporal Web UI (`http://temporal:8080`) — are workers showing for the expected task queues?

```
# Check Temporal worker connectivity
temporal operator namespace describe default
```

If the worker is registered but workflows queue up without starting, check:
1. Task queue name in `WorkflowTrigger` matches the registered worker task queue
2. Worker `MaxConcurrentWorkflowTaskPollers` is not 0
3. Temporal server is reachable from the worker process

### Workflow ID Already Exists

```
temporal workflow describe --workflow-id "tenant.entity.id.event"
```

If the workflow exists and is running, the entity event won't start a duplicate (by design — Temporal deduplicates by workflow ID). If the entity needs a new workflow run, the prior run must complete or be terminated first.

### Outbox Relay Not Processing

If the entity was created but the workflow was never started:

```sql
-- Check workflow outbox
SELECT id, entity_type, event, status, attempt_count, last_error
FROM workflow_outbox
WHERE entity_id = $1
ORDER BY created_at DESC;
```

- `status = 'PENDING'`: relay not running or behind
- `status = 'PROCESSING'` stuck for >10 min: relay crashed mid-dispatch; see outbox cleanup cron
- `status = 'FAILED'` after max attempts: investigate `last_error`

---

## 6. Page Schemas Not Loading (SDUI Blank)

### Redis Cache Miss (Schema Not Generated)

```
# Check Redis for page schema key
redis-cli -u $REDIS_URL KEYS "page:*" | head -20
```

If no keys: the schema generation is failing silently. Check application logs for errors from the `PageBuilder` function.

### Permission Check Excluding All Components

If the schema is returned but the page appears empty, the page builder's `psc.IfPermitted()` checks may be excluding all components. Verify the actor's role has the expected permissions:

```sql
-- Check Casbin policies for actor
SELECT p.* FROM casbin_rules p
WHERE p.v0 = 'role:finance.viewer'
  AND p.v1 = $tenant_id;
```

### amis SDK Version Mismatch

If the page renders with broken layout but schema is correct, check `web/sdk/` versions. Never update the pinned SDK without a full audit (see ADR-003).

---

## 7. Slow Queries / High DB Latency

### Missing Index

```sql
-- Find slow queries (requires pg_stat_statements extension)
SELECT query, calls, mean_exec_time, total_exec_time
FROM pg_stat_statements
WHERE mean_exec_time > 100  -- slower than 100ms
ORDER BY mean_exec_time DESC
LIMIT 20;
```

For filter queries hitting a JSONB column without a GIN index:

```sql
-- Check if GIN index exists
SELECT indexname, indexdef
FROM pg_indexes
WHERE tablename = 'my_entity'
  AND indexdef LIKE '%gin%';
```

If missing: add `CREATE INDEX CONCURRENTLY ON my_entity USING GIN (data jsonb_path_ops);`

### N+1 Queries

Symptoms: `db_query_duration_seconds` metric shows many sub-millisecond queries rather than fewer longer ones.

Check if edge loading is using `entity.WithEdge("...")` or falling back to per-record loops. Lazy loading is disabled by design — if N+1 is occurring, the page builder or hook code is calling `repo.Get` inside a loop.

### PgBouncer Pool Exhaustion

```
# Check pool waiting queue
psql $PGBOUNCER_URL -c "SHOW POOLS;"
# cl_waiting > 0 = connections queued
# sv_idle = 0 = all connections busy
```

Resolution: increase `pool_size` in PgBouncer config, or reduce query latency to free connections faster.

---

## 8. RLS Returning Wrong Data (Cross-Tenant Leak)

This should never happen if `set_tenant_context()` is called correctly. If suspected:

```sql
-- Verify RLS is enabled on the suspect table
SELECT relname, relrowsecurity, relforcerowsecurity
FROM pg_class
WHERE relname = 'finance_invoice';
-- relrowsecurity = true AND relforcerowsecurity = true required

-- Test RLS manually
SET app.current_tenant_id = 'tenant-uuid-a';
SELECT count(*) FROM finance_invoice;
-- Should return only tenant A's records

SET app.current_tenant_id = 'tenant-uuid-b';
SELECT count(*) FROM finance_invoice;
-- Should return only tenant B's records
```

If cross-tenant data is visible: the RLS policy is missing or misconfigured on the table. Apply the standard RLS migration:

```sql
ALTER TABLE finance_invoice ENABLE ROW LEVEL SECURITY;
ALTER TABLE finance_invoice FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON finance_invoice
    USING (tenant_id = current_tenant_id());
```

Report as a critical security incident and rotate all session tokens.

---

## 9. Redis Connection Errors

### Connection Refused

```
redis-cli -u $REDIS_URL PING
# Could not connect to Redis at host:port: Connection refused
```

Check if Redis process is running. Check firewall rules between app server and Redis.

### AUTH Failed

If Redis is configured with a password and the app is connecting without credentials:

```
WRONGPASS invalid username-password pair
```

Verify `REDIS_URL` includes credentials: `redis://:password@host:port/db`

### Too Many Connections

```
ERR max number of clients reached
```

Increase Redis `maxclients` config, or enable connection pooling in the Redis client.

### Data Eviction

If Redis is configured with `maxmemory-policy allkeys-lru` and memory pressure is high, session tokens may be evicted prematurely — users get logged out unexpectedly.

Resolution: increase Redis memory, or switch to `volatile-lru` (only evict keys with TTL — session keys have TTL, so this is compatible). Increase Redis memory allocation.

---

## 10. Memory or Goroutine Leaks

### Goroutine Leak Detection

```
# Check goroutine count via pprof
curl http://localhost:6060/debug/pprof/goroutine?debug=2
```

If goroutine count grows monotonically:
- Check for goroutines started in request handlers without cancellation propagation
- Check for `go func()` closures that capture `c.Context()` without using `context.WithoutCancel`

### Memory Leak

```
# Heap profile
curl http://localhost:6060/debug/pprof/heap > heap.prof
go tool pprof heap.prof
```

Common causes:
- `sync.Map` or map accumulation in module-level singletons
- Unclosed response bodies from activity HTTP calls
- Large amis page schemas cached beyond their TTL

### Redis Key Accumulation

```
# Count Redis keys by pattern
redis-cli -u $REDIS_URL DBSIZE
redis-cli -u $REDIS_URL SCAN 0 MATCH "page:*" COUNT 1000
```

If page schema keys accumulate without expiry: verify TTL is being set correctly (`SET key value EX 300`). Missing TTL = key persists forever = Redis memory grows.

---

## Related Documents

- [Deployment](deployment.md) — Kubernetes deployment, health checks, graceful shutdown
- [Migrations](migrations.md) — migration process, dirty state recovery
- [Observability](../13-observability/observability.md) — metrics, logs, traces for diagnosis
- [Outbox Pattern](../09-workflow/outbox-pattern.md) — workflow outbox schema and relay
- [Security Model](../15-security/security-model.md) — RLS, session, RBAC enforcement layers
