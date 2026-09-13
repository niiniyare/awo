# Migration Checklist

**Classification:** Reference — Tier 2
**Owner:** `15-migrations/MIGRATION_CHECKLIST.md`
**Status:** Living document

---

## Purpose

Mandatory checklist for every database migration before merge. Reviewer and author MUST both verify all items.

---

## File Structure Checklist

- [ ] File named with 14-digit Unix timestamp: `YYYYMMDDHHMMSS_description.up.sql`
- [ ] Corresponding `.down.sql` file exists and reverses the up migration completely
- [ ] Timestamp is newer than all existing migration timestamps
- [ ] No spaces in filename; description uses `snake_case`

---

## Schema Checklist (New Tables)

- [ ] `id uuid PRIMARY KEY DEFAULT gen_random_uuid()` — no serial/bigserial PKs
- [ ] `tenant_id uuid NOT NULL REFERENCES tenants(id)` — for tenant-scoped tables
- [ ] `ENABLE ROW LEVEL SECURITY` — without this, no policy is enforced
- [ ] `FORCE ROW LEVEL SECURITY` — without this, table owner bypasses RLS
- [ ] `CREATE POLICY tenant_isolation ... USING (tenant_id = current_tenant_id())` — correct function name
- [ ] `created_at timestamptz NOT NULL DEFAULT now()` — use `timestamptz`, not `timestamp`
- [ ] `updated_at timestamptz NOT NULL DEFAULT now()` — where applicable
- [ ] `custom_fields jsonb NOT NULL DEFAULT '{}'` — for system entities (not custom entities)
- [ ] Index on `(tenant_id)` column

---

## Schema Checklist (Column Additions)

- [ ] New column is `NOT NULL` only if it has a `DEFAULT` (otherwise nullable first)
- [ ] No bare `NOT NULL` without `DEFAULT` on non-empty table — causes table rewrite
- [ ] Currency columns use `numeric(20,4)` — never `float`, `real`, `decimal(10,2)`, or `numeric(20,6)`
- [ ] Text columns have appropriate type: `varchar(n)` for indexed, `text` for unindexed free-form
- [ ] Boolean columns have `NOT NULL DEFAULT false` (or `true` if always set)

---

## Index Checklist

- [ ] All FK columns have indexes
- [ ] `Searchable: true` fields have GIN trigram index: `CREATE INDEX ... USING GIN (col gin_trgm_ops)`
- [ ] `JSON` / `custom_fields` columns have GIN index: `CREATE INDEX ... USING GIN (col)`
- [ ] All new indexes use `CREATE INDEX CONCURRENTLY` if added to existing tables with data
- [ ] Unique constraints: `CREATE UNIQUE INDEX` (not `UNIQUE` in column definition, for partial uniqueness support)

---

## Security Checklist

- [ ] Sensitive data columns are NOT indexed unless required (index reveals cardinality)
- [ ] No plaintext password columns
- [ ] Foreign keys to `users` table do NOT expose cross-tenant user references
- [ ] Platform-level tables (no RLS) are explicitly documented in `04-multitenancy/GLOBAL_TABLES.md`

---

## Zero-Downtime Checklist

- [ ] Adding column: `ADD COLUMN ... NULL` first; constraints/defaults in separate migration
- [ ] Adding index: `CREATE INDEX CONCURRENTLY` — no table lock
- [ ] Renaming column: four-step process (add, dual-write, backfill, drop) — not single-step rename
- [ ] Dropping column: code removed from application first, deployed, then column dropped
- [ ] No `ALTER TABLE ... RENAME COLUMN` without prior dual-write cycle

---

## Down Migration Checklist

- [ ] Down migration reverses the up migration completely
- [ ] `DROP TABLE IF EXISTS` (not `DROP TABLE`) — idempotent
- [ ] `DROP INDEX IF EXISTS` — idempotent
- [ ] `ALTER TABLE ... DROP COLUMN IF EXISTS` — idempotent
- [ ] Down migration does NOT drop data not created by the up migration

---

## Review Sign-Off

Before merging:

1. **Author:** All checklist items verified.
2. **Reviewer:** Migration tested locally against a copy of production schema (or staging).
3. **Reviewer:** Down migration tested (rolled back successfully).
4. **Reviewer:** No `golang-migrate` warnings in CI output.

---

## References

- [`15-migrations/MIGRATION_GUIDE.md`](MIGRATION_GUIDE.md) — Migration governance and patterns
- [`15-migrations/RLS_TABLE_TEMPLATE.md`](RLS_TABLE_TEMPLATE.md) — Template for new tenant-scoped tables
- [`04-multitenancy/RLS_SPEC.md`](../04-multitenancy/RLS_SPEC.md) — RLS specification
