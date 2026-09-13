> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

# Migration Failure Runbook

**Classification:** Runbook — Tier 2
**Owner:** `19-operations/RUNBOOK_MIGRATION_FAILURE.md`
**Trigger:** Migration job fails during deploy. App pods may be in `Init:Error` or started on old schema.

---

## 1. Identify the Failure

```bash
# Find the migration job
kubectl get jobs -n production | grep migrate

# View migration job logs
kubectl logs job/awo-erp-migrate -n production

# If job has multiple pods (retries)
kubectl logs -l job-name=awo-erp-migrate -n production
```

---

## 2. Classify the Error

### Syntax Error in SQL

```
ERROR: syntax error at or near "CONCURRENTLY" (SQLSTATE 42601)
```

The migration file has a SQL error. Must fix source and redeploy.

**Immediate action**: roll back the deploy, fix SQL, redeploy.

```bash
kubectl rollout undo deployment/awo-erp-server -n production
```

### Migration Already Applied (Dirty State)

```
error: Dirty database version {N}. Fix and force version.
```

A previous migration run failed partway through. `migrate` library marks the DB as "dirty".

```bash
# Check state
psql "$DATABASE_URL" -c "SELECT * FROM schema_migrations ORDER BY version DESC LIMIT 5;"

# Force-mark the version as clean (only if you've verified what partially applied)
migrate -database "$DATABASE_URL" -path db/migration force {N}

# Then apply remaining migrations
migrate -database "$DATABASE_URL" -path db/migration up
```

**Warning**: `force` skips re-applying the failed migration. Manually verify the schema is in the expected state before continuing.

### Lock Timeout (concurrent migration)

```
ERROR: deadlock detected
ERROR: canceling statement due to lock timeout
```

Another process holds a lock on the table being migrated (autovacuum, long-running query).

```sql
-- Find blocking processes
SELECT pid, query, state, wait_event_type, wait_event
FROM pg_stat_activity
WHERE state != 'idle'
ORDER BY query_start;

-- Cancel the blocking process if safe
SELECT pg_cancel_backend({pid});
```

Then re-run the migration job:

```bash
kubectl delete job awo-erp-migrate -n production
kubectl apply -f k8s/jobs/migrate.yaml -n production
```

### Constraint Violation

```
ERROR: insert or update on table "X" violates foreign key constraint
ERROR: column "X" of relation "Y" contains null values
```

Existing data violates the new constraint. The migration needs a data fix step before adding the constraint.

**Do not force this** — roll back deploy, fix migration with a backfill step, redeploy.

---

## 3. Check What Applied

```sql
-- View applied migrations in order
SELECT version, dirty FROM schema_migrations ORDER BY version;

-- Undelivered vs files in db/migration/
-- Any version in files but NOT in table = not yet applied
-- Any version in table but NOT in files = orphaned migration
```

---

## 4. Manual Recovery

If automated migration is stuck and app needs to be running (emergency):

```bash
# Run migration manually from a pod
kubectl run migrate-manual \
  --image=awo-erp:latest \
  --restart=Never \
  --env DATABASE_URL="$DATABASE_URL" \
  -n production \
  -- /app/awoerp migrate up
```

---

## 5. Zero-Downtime Migration Principles

All production migrations must follow expand-contract:

1. **Expand**: add new column as nullable (no lock required)
2. **Deploy app**: dual-writes to both old and new columns
3. **Backfill**: `UPDATE ... WHERE new_col IS NULL` (in batches, separate migration)
4. **Contract**: add NOT NULL constraint, drop old column

If a migration violated this pattern and caused an outage, document it as a post-mortem item and add it to the [`MIGRATION_CHECKLIST.md`](../15-migrations/MIGRATION_CHECKLIST.md).

---

## 6. Post-Incident

After recovery:
1. Verify row counts match expectations
2. Run a smoke test against critical endpoints
3. Check migration table — all versions clean (not dirty):
   ```sql
   SELECT COUNT(*) FROM schema_migrations WHERE dirty = true;
   -- Should return 0
   ```
4. Write post-mortem if downtime > 5 minutes

---

## References

- [`15-migrations/MIGRATION_GUIDE.md`](../15-migrations/MIGRATION_GUIDE.md) — Migration patterns
- [`15-migrations/MIGRATION_CHECKLIST.md`](../15-migrations/MIGRATION_CHECKLIST.md) — Prevention checklist
