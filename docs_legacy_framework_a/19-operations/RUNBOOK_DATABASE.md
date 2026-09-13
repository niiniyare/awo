> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

# Database Issues Runbook

**Classification:** Runbook — Tier 2
**Owner:** `19-operations/RUNBOOK_DATABASE.md`
**Use when:** Slow queries, high connection count, pool exhaustion, replication lag, migration errors.

---

## Symptom: High Response Latency

### Check Active Queries

```sql
-- Queries running longer than 5 seconds
SELECT pid, now() - pg_stat_activity.query_start AS duration,
       query, state, wait_event_type, wait_event
FROM pg_stat_activity
WHERE (now() - pg_stat_activity.query_start) > interval '5 seconds'
  AND state != 'idle'
ORDER BY duration DESC;
```

### Check for Lock Waits

```sql
SELECT blocked.pid     AS blocked_pid,
       blocked.query   AS blocked_query,
       blocking.pid    AS blocking_pid,
       blocking.query  AS blocking_query
FROM pg_stat_activity blocked
JOIN pg_stat_activity blocking ON blocking.pid = ANY(pg_blocking_pids(blocked.pid))
WHERE cardinality(pg_blocking_pids(blocked.pid)) > 0;
```

If a long-running transaction is blocking others:

```sql
-- Cancel (graceful)
SELECT pg_cancel_backend(<blocking_pid>);

-- Terminate (hard kill — use only if cancel fails after 30s)
SELECT pg_terminate_backend(<blocking_pid>);
```

---

## Symptom: Connection Pool Exhaustion

Application logs: `failed to acquire connection from pool` or pool wait timeout.

### Check Connection Count

```sql
SELECT count(*), state, wait_event_type
FROM pg_stat_activity
GROUP BY state, wait_event_type
ORDER BY count DESC;
```

Healthy: most connections `idle`, few `active`.

Problem signs:
- Many connections in `idle in transaction` — transaction not committed (possible TX leak)
- Count near `max_connections`

### PgBouncer Mode Check

PgBouncer MUST run in transaction mode. Session mode breaks RLS by not resetting the GUC between requests:

```bash
# Check PgBouncer config
psql "$PGBOUNCER_URL" -c "SHOW CONFIG;" | grep pool_mode
# Expected: transaction
```

If `pool_mode = session` → immediate reconfiguration required (data isolation is broken).

### Fix Options

1. Verify PgBouncer is in transaction mode (required for RLS)
2. Reduce `DB_POOL_MAX` per pod
3. Increase `max_connections` in PostgreSQL (requires restart)

---

## Symptom: Replication Lag (Read Replicas)

```sql
-- On primary
SELECT client_addr, state, sent_lsn, replay_lsn,
       (sent_lsn - replay_lsn) AS replication_lag_bytes
FROM pg_stat_replication;
```

Lag > 100 MB: investigate replica I/O.
Lag > 1 GB: pause reads to replica until caught up.

---

## Symptom: Table Bloat

```sql
SELECT schemaname, relname, n_dead_tup, n_live_tup,
       round(n_dead_tup::numeric / nullif(n_live_tup + n_dead_tup, 0) * 100, 1) AS dead_pct,
       last_autovacuum
FROM pg_stat_user_tables
WHERE n_dead_tup > 10000
ORDER BY dead_pct DESC;
```

Force vacuum on bloated table (non-blocking):

```sql
VACUUM (ANALYZE, VERBOSE) finance_invoice;
```

---

## Symptom: Index Not Being Used

```sql
-- Tables with high seq scan ratio
SELECT schemaname, relname, seq_scan, idx_scan
FROM pg_stat_user_tables
WHERE seq_scan > 100 AND n_live_tup > 10000
ORDER BY seq_scan - idx_scan DESC;
```

Analyze query plan:

```sql
EXPLAIN (ANALYZE, BUFFERS) SELECT * FROM finance_invoice
WHERE tenant_id = '...' AND status = 'Draft' LIMIT 20;
```

Add missing index without lock:

```sql
CREATE INDEX CONCURRENTLY finance_invoice_tenant_status
ON finance_invoice (tenant_id, status);
```

---

## Emergency: DB Unreachable

1. Check database server status (RDS console, etc.)
2. Check VPC security groups / firewall rules
3. Verify DB password not rotated without updating k8s secret:
   ```bash
   kubectl get secret -n awo-erp awo-db-credentials \
     -o jsonpath='{.data.database_url}' | base64 -d
   ```
4. Check disk space on DB host — full disk causes connection refusal

---

## RLS Verification

If cross-tenant data leakage is suspected:

```sql
-- Verify RLS is enabled on all tenant-scoped tables
SELECT tablename, rowsecurity, forcerowsecurity
FROM pg_tables
WHERE schemaname = 'public'
  AND tablename NOT IN ('tenants', 'audit_log', 'timezones', 'currencies',
                        'countries', 'paye_bands', 'platform_admins',
                        'workflow_outbox', 'event_outbox');
-- All rows should have rowsecurity=true AND forcerowsecurity=true
```

---

## Metrics to Watch

| Metric | Warning | Critical |
|--------|---------|----------|
| Active connections | > 60% of max | > 80% of max |
| Query duration p99 | > 500ms | > 2s |
| Replication lag | > 50 MB | > 500 MB |
| Dead tuple ratio | > 10% | > 30% |

---

## References

- [`04-multitenancy/RLS_SPEC.md`](../04-multitenancy/RLS_SPEC.md) — RLS specification
- [`15-migrations/MIGRATION_GUIDE.md`](../15-migrations/MIGRATION_GUIDE.md) — Migration patterns
- [`19-operations/RUNBOOK_MIGRATION_FAILURE.md`](RUNBOOK_MIGRATION_FAILURE.md) — Migration failure recovery
