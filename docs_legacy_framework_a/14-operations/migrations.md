> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Migrations"
id: ops-001
status: accepted
category: SPEC
stability: STABLE
audience: [operators, module-authors]
since: "1.0"
normative-level: normative
related:
  - "[Deployment](deployment.md)"
  - "[System Entities](../05-persistence/system-entities.md)"
  - "[Architecture Laws](../02-architecture/laws.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Migrations

**OPS-001 | Status: Accepted | Stability: Stable**

This document specifies the migration workflow, file naming convention, zero-downtime patterns, rollback procedures, and the append-only rule.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. Tool and File Format

Migrations use `golang-migrate` with the PostgreSQL driver. Files are plain SQL — no ORM-generated DDL.

### File Naming

```
{timestamp}_{description}.up.sql
{timestamp}_{description}.down.sql
```

- `{timestamp}`: 14-digit Unix timestamp (`YYYYMMDDHHMMSS`), e.g. `20241215143022`
- `{description}`: lowercase, underscored slug describing the change
- Both `.up.sql` and `.down.sql` MUST exist for every migration

```
db/migration/
    20241215143022_create_finance_invoice.up.sql
    20241215143022_create_finance_invoice.down.sql
    20241215151200_add_invoice_due_date.up.sql
    20241215151200_add_invoice_due_date.down.sql
```

---

## 2. Append-Only Rule (LAW-012)

Migration files MUST NOT be modified after they have been applied to any environment. This is enforced by `golang-migrate`'s checksum verification — modified migration files cause migration runner failure.

If a migration has an error:
1. Write a new migration to correct it — do not edit the original
2. The down migration for the original should undo its changes
3. Apply: run down to the failed version, apply the corrective migration

---

## 3. Migration Runner

Migrations run via a separate process (`cmd/migrate/`) — not embedded in the API server:

```bash
# Apply all pending migrations
./awo-migrate up

# Rollback one step
./awo-migrate down 1

# Apply up to a specific version
./awo-migrate goto 20241215143022

# Check current version
./awo-migrate version
```

The migration runner MUST NOT be run concurrently. In Kubernetes, use a pre-deploy Job (not an initContainer) to ensure migrations complete before any new server pods start.

The migration runner connects directly to PostgreSQL — not through PgBouncer — to avoid transaction mode complications during DDL.

---

## 4. RLS Requirements for New Tables

Every new tenant-scoped table MUST include the RLS block in its `.up.sql`:

```sql
-- 20241215143022_create_finance_invoice.up.sql
CREATE TABLE finance_invoice (
    id          uuid            PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   uuid            NOT NULL REFERENCES tenants(id),
    number      varchar(64)     NOT NULL,
    customer    uuid            REFERENCES crm_customer(id),
    status      varchar(32)     NOT NULL CHECK (status IN ('Draft','Submitted','Approved','Paid','Cancelled')),
    total_kes   numeric(20,4)   NOT NULL CHECK (total_kes >= 0),
    notes       text,
    created_at  timestamptz     NOT NULL DEFAULT now(),
    updated_at  timestamptz     NOT NULL DEFAULT now()
);

-- RLS (required for all tenant-scoped tables)
ALTER TABLE finance_invoice ENABLE ROW LEVEL SECURITY;
ALTER TABLE finance_invoice FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON finance_invoice
    USING (tenant_id = current_tenant_id());

-- Indexes
CREATE INDEX finance_invoice_tenant_idx ON finance_invoice (tenant_id);
CREATE INDEX finance_invoice_status_idx ON finance_invoice (tenant_id, status);
CREATE INDEX finance_invoice_customer_idx ON finance_invoice (customer);
```

The corresponding `.down.sql`:

```sql
-- 20241215143022_create_finance_invoice.down.sql
DROP TABLE IF EXISTS finance_invoice;
```

---

## 5. Zero-Downtime Patterns

The API server and migration runner operate independently. Zero-downtime migrations are required for all production schema changes because old and new server versions run simultaneously during a rolling deploy.

### Adding a Column (Safe)

```sql
-- Step 1: Add nullable column with default (instant, no table rewrite)
ALTER TABLE finance_invoice ADD COLUMN discount_kes numeric(20,4) DEFAULT 0;

-- Step 2 (separate migration, after code deploy): add NOT NULL if required
-- Only safe after all rows have non-null values
ALTER TABLE finance_invoice ALTER COLUMN discount_kes SET NOT NULL;
```

Never add a NOT NULL column without a default in a single step on a large table — it rewrites every row.

### Adding an Index (Safe)

```sql
-- CONCURRENTLY: no table lock, safe on live traffic
-- Never use plain CREATE INDEX on a production table with existing data
CREATE INDEX CONCURRENTLY finance_invoice_due_date_idx ON finance_invoice (tenant_id, due_date);
```

`golang-migrate` does not support `CREATE INDEX CONCURRENTLY` inside a transaction. Wrap in a migration that uses a custom DDL:

```sql
-- Disable auto-transaction for this migration (golang-migrate convention)
-- by using a plain SQL file (no BEGIN/COMMIT) and setting DisableTransactions: true
CREATE INDEX CONCURRENTLY finance_invoice_due_date_idx ON finance_invoice (tenant_id, due_date);
```

### Renaming a Column (Multi-Step)

Never rename a column in a single step on a live table — old code will break immediately.

```sql
-- Step 1: Add new column
ALTER TABLE finance_invoice ADD COLUMN reference_number varchar(64);

-- Step 2 (after dual-write code is deployed):
-- Backfill: UPDATE finance_invoice SET reference_number = old_ref_number WHERE reference_number IS NULL;

-- Step 3 (after reads are switched to new column):
ALTER TABLE finance_invoice DROP COLUMN old_ref_number;
```

### Dropping a Column (Multi-Step)

```sql
-- Step 1: Remove all reads/writes from code (deploy first)
-- Step 2: Drop column (after code is deployed everywhere)
ALTER TABLE finance_invoice DROP COLUMN deprecated_field;
```

Do NOT drop a column while any running server version still reads it.

---

## 6. Down Migrations

Every `.down.sql` MUST undo exactly the changes made by the corresponding `.up.sql`. Down migrations are used for rollback in staging — not for production (data loss risk).

Rules for down migrations:
- `DROP TABLE` reverses `CREATE TABLE`
- `DROP INDEX` reverses `CREATE INDEX`
- `ALTER TABLE DROP COLUMN` reverses `ALTER TABLE ADD COLUMN`
- Do NOT include `DROP TABLE CASCADE` — only drop what was created
- Down migrations for data-seeding steps are typically no-ops (data removal is too risky)

---

## 7. Pre-Deploy Checklist

Before applying a migration to production:

- [ ] Migration applied to staging and verified stable for 24 hours
- [ ] `.down.sql` tested in staging
- [ ] Large table migrations use `ADD COLUMN ... DEFAULT NULL` pattern (not rewrite)
- [ ] New indexes use `CREATE INDEX CONCURRENTLY`
- [ ] RLS block present for all new tenant-scoped tables
- [ ] No `ALTER TABLE ... RENAME` on live tables
- [ ] `golang-migrate version` confirmed on current baseline before upgrade

---

## 8. Emergency Rollback

If a migration must be rolled back in production (data corruption, unexpected blocking):

```bash
# Roll back one migration
./awo-migrate down 1

# Check current state
./awo-migrate version
```

Emergency rollback applies only to the most recent migration. Rolling back multiple migrations in production requires manual review of each down migration — do not automate multi-step rollback.

After rollback, redeploy the previous server version (previous container image tag).

---

## Related Documents

- [Deployment](deployment.md) — migration job as part of deployment pipeline
- [System Entities](../05-persistence/system-entities.md) — SQL schema template for system entities
- [RLS](../06-tenancy/rls.md) — RLS setup requirements
- [Architecture Laws](../02-architecture/laws.md) — LAW-012 (append-only), LAW-019 (identical schema across instances)
- [Glossary](../GLOSSARY.md) — Migration, Zero-Downtime Deploy, golang-migrate
