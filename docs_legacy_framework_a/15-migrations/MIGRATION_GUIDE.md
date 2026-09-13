> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

# Migration Guide

**Classification:** Guide — Tier 2
**Owner:** `15-migrations/MIGRATION_GUIDE.md`
**Status:** Living document

---

## Purpose

This guide specifies the file naming convention, up/down pair structure, zero-downtime migration patterns, and governance rules for all database migrations in the Awo Framework.

---

## 1. Migration Tool

Tool: [`golang-migrate`](https://github.com/golang-migrate/migrate) with PostgreSQL driver.

Binary: invoked via `cmd/migrate/` — a separate process from the API server.

```
cmd/migrate/main.go   ← migration runner entrypoint
db/migration/         ← migration file directory
```

**Never auto-migrate from the API server startup.** Migrations run as a separate CI step before deployment.

---

## 2. File Naming Convention

```
{14-digit-unix-timestamp}_{description_slug}.up.sql
{14-digit-unix-timestamp}_{description_slug}.down.sql
```

The timestamp is 14 digits: `YYYYMMDDHHMMSS` in UTC.

**Examples:**

```
20241215143022_create_finance_invoice.up.sql
20241215143022_create_finance_invoice.down.sql
20241216091545_add_finance_invoice_submitted_at.up.sql
20241216091545_add_finance_invoice_submitted_at.down.sql
20250103120000_add_invoice_series_counter.up.sql
20250103120000_add_invoice_series_counter.down.sql
```

**Rules:**
- Timestamps MUST be monotonically increasing. Two migrations MUST NOT share the same timestamp.
- Description slugs use `snake_case`. Hyphens are not permitted.
- Every `.up.sql` MUST have a corresponding `.down.sql`.
- Never rename a migration file after it has been applied to any environment.

---

## 3. Up Migration Requirements

Every `.up.sql` file MUST include:

1. `CREATE TABLE` or `ALTER TABLE` statement
2. `PRIMARY KEY` declaration
3. `tenant_id uuid NOT NULL` for every tenant-scoped table
4. `created_at timestamptz NOT NULL DEFAULT now()`
5. `updated_at timestamptz NOT NULL DEFAULT now()` (where applicable)
6. RLS setup (if tenant-scoped): see [`15-migrations/RLS_TABLE_TEMPLATE.md`](RLS_TABLE_TEMPLATE.md)
7. All required indexes

---

## 4. Down Migration Requirements

Every `.down.sql` MUST:
- Reverse the up migration completely.
- `DROP TABLE IF EXISTS` for new tables.
- `ALTER TABLE ... DROP COLUMN` for new columns.
- `DROP INDEX IF EXISTS` for new indexes.
- Be idempotent (safe to run multiple times).

---

## 5. Zero-Downtime Migration Patterns

### Adding a Column (Zero Downtime)

```sql
-- Step 1: Add column as nullable with no default (instant)
ALTER TABLE finance_invoice ADD COLUMN submitted_at timestamptz;

-- Step 2: Backfill in batches (separate migration, after step 1 deployed)
UPDATE finance_invoice
SET submitted_at = updated_at
WHERE submitted_at IS NULL
  AND status IN ('Submitted', 'Approved', 'Paid')
LIMIT 10000;
-- Repeat until no rows affected

-- Step 3: Add constraint (separate migration, after backfill complete)
ALTER TABLE finance_invoice
    ALTER COLUMN submitted_at SET DEFAULT now();
```

**Never:** `ADD COLUMN ... NOT NULL DEFAULT value` on a large table — this rewrites the entire table in PostgreSQL < 11. On PostgreSQL 12+, it is instant for constant defaults, but still requires a careful audit.

### Adding an Index (Zero Downtime)

Always use `CREATE INDEX CONCURRENTLY`:

```sql
CREATE INDEX CONCURRENTLY finance_invoice_customer_id
    ON finance_invoice (customer_id);
```

`CONCURRENTLY` builds the index without locking the table. Note: `CONCURRENTLY` cannot run inside a transaction block — the migration file MUST be wrapped in `BEGIN`/`COMMIT` manually or use a separate migration.

**Never:** `CREATE INDEX` without `CONCURRENTLY` on production tables — causes full table lock.

### Renaming a Column (Zero Downtime)

Never rename a column in a single migration. Use the four-step process:

1. Add new column.
2. Dual-write (application writes to both old and new column) — code change.
3. Backfill new column from old column.
4. Switch reads to new column — code change.
5. Drop old column.

### Dropping a Column

Only drop columns after the code has been updated to stop reading/writing them and the deployment has completed.

```sql
ALTER TABLE finance_invoice DROP COLUMN IF EXISTS legacy_field;
```

---

## 6. Transaction Handling

`golang-migrate` wraps each migration in a transaction by default. This means:
- If any statement in the file fails, the entire migration rolls back.
- DDL in PostgreSQL is transactional (unlike MySQL).
- Exception: `CREATE INDEX CONCURRENTLY` cannot run in a transaction. Use a separate migration file for concurrent index creation, or add `-- migrate: no-transaction` at the top of the file.

---

## 7. Single Shared Schema

All migrations apply to the single shared PostgreSQL schema. Tenant data isolation is via RLS, not per-tenant schemas. This means:

- One migration applies to ALL tenants simultaneously.
- Per-tenant differences go in seed data or settings, not in schema differences.
- A migration that adds a column adds it for every tenant.

---

## 8. Migration Checklist

See [`15-migrations/MIGRATION_CHECKLIST.md`](MIGRATION_CHECKLIST.md) for the mandatory pre-merge checklist.

---

## 9. Governance Rules

- **Never auto-migrate** — no `db.AutoMigrate()`, no migration at API server startup.
- **Never edit a committed migration** — create a new migration to fix mistakes.
- **Never delete a migration** — even if it was applied only to local dev.
- **Always write the down migration** — even if it's `DROP TABLE` — before merging.
- **Review migrations in CI** — migration files are reviewed as carefully as code.

---

## References

- [`15-migrations/RLS_TABLE_TEMPLATE.md`](RLS_TABLE_TEMPLATE.md) — RLS setup template for new tables
- [`15-migrations/MIGRATION_CHECKLIST.md`](MIGRATION_CHECKLIST.md) — Pre-merge checklist
- [`04-multitenancy/RLS_SPEC.md`](../04-multitenancy/RLS_SPEC.md) — RLS policy specification
