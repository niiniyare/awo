# Chapter 44 — Migration

> **Volume:** IX — Testing, CI/CD & Governance
> **Phase:** 4
> **Audience:** Backend Engineers, Platform Engineers, DBAs
> **Prerequisites:** Chapters 25, 28, 32, 41

---

## Table of Contents

- [44.1 Migration Philosophy](#441-migration-philosophy)
- [44.2 Database Migrations](#442-database-migrations)
- [44.3 Zero-Downtime Migration Patterns](#443-zero-downtime-migration-patterns)
- [44.4 SDUI Schema Migrations](#444-sdui-schema-migrations)
- [44.5 API Version Migration](#445-api-version-migration)
- [44.6 Tenant Data Migration](#446-tenant-data-migration)
- [44.7 Rollback Procedures](#447-rollback-procedures)

---

## 44.1 Migration Philosophy

AwoERP is a multi-tenant SaaS platform. Any migration — database schema, API, or SDUI schema — affects all tenants simultaneously. This drives several non-negotiable rules:

1. **No migrations that lock tables for more than a few milliseconds.** A table lock on a 10-million-row `invoices` table during business hours is an outage for every tenant.
2. **Migrations are always additive first.** Add the new column or field. Keep the old one. Migrate data in the background. Remove the old one last.
3. **Every migration is reversible.** The down migration is written, tested, and reviewed before the up migration ships.
4. **Data migrations are idempotent.** A data migration job that runs twice must produce the same result as running it once.

---

## 44.2 Database Migrations

### 44.2.1 Migration Files

Each migration is a pair of SQL files:

```
migrations/
├── 00001_initial_schema.up.sql
├── 00001_initial_schema.down.sql
├── 00042_add_invoice_custom_fields.up.sql
├── 00042_add_invoice_custom_fields.down.sql
```

### 44.2.2 Migration Review Checklist

Every migration PR requires review against this checklist:

- [ ] Does the up migration acquire any `ACCESS EXCLUSIVE` locks? (Forbidden without justification)
- [ ] Is the migration reversible (down migration exists and is correct)?
- [ ] Does the migration operate on large tables? If yes, uses `NOT VALID` + `VALIDATE CONSTRAINT` pattern
- [ ] Does the migration add `NOT NULL` columns? If yes, uses default value pattern (see 44.3)
- [ ] Has the down migration been tested locally?
- [ ] Is the data migration (if any) idempotent?

### 44.2.3 Migration Execution in Production

Migrations run as part of the deployment process, before new pods start:

```bash
# Kubernetes init container runs migrations before the API container starts
# This ensures the schema is ready before code tries to use it

# In the deployment YAML:
initContainers:
  - name: migrate
    image: awoerp/migrate:${VERSION}
    command: ["./migrate", "-up"]
    env:
      - name: DATABASE_URL
        valueFrom:
          secretKeyRef:
            name: db-credentials
            key: url
```

---

## 44.3 Zero-Downtime Migration Patterns

### 44.3.1 Adding a Non-Nullable Column

**Wrong (locks the table, blocks reads and writes):**
```sql
ALTER TABLE invoices ADD COLUMN tax_rate DECIMAL(5,2) NOT NULL;
```

**Right (three deployments, no locking):**

**Deployment 1:** Add nullable column:
```sql
-- 00043_add_invoice_tax_rate.up.sql
ALTER TABLE invoices ADD COLUMN tax_rate DECIMAL(5,2);
```

**Between Deployment 1 and 2:** Background job fills the new column:
```go
// Run as a Temporal workflow; idempotent
func BackfillInvoiceTaxRate(ctx workflow.Context) error {
    return batchUpdate(ctx,
        "UPDATE invoices SET tax_rate = 0.0 WHERE tax_rate IS NULL",
        batchSize: 1000,
    )
}
```

**Deployment 2:** Add the NOT NULL constraint (validation only, no rewrite):
```sql
-- 00044_invoice_tax_rate_not_null.up.sql
-- First: add constraint without validation (instant, no lock)
ALTER TABLE invoices ADD CONSTRAINT invoices_tax_rate_not_null
    CHECK (tax_rate IS NOT NULL) NOT VALID;

-- Then: validate (acquires ShareUpdateExclusiveLock, not AccessExclusiveLock)
-- Runs concurrently with reads/writes
ALTER TABLE invoices VALIDATE CONSTRAINT invoices_tax_rate_not_null;
```

**Deployment 3:** Set column as NOT NULL (only after all rows have values):
```sql
-- 00045_invoice_tax_rate_column_not_null.up.sql
-- Now safe: all rows have values, constraint already validated
ALTER TABLE invoices ALTER COLUMN tax_rate SET NOT NULL;
ALTER TABLE invoices DROP CONSTRAINT invoices_tax_rate_not_null;
```

### 44.3.2 Adding an Index on a Large Table

```sql
-- Wrong: locks the table for minutes
CREATE INDEX invoices_vendor_idx ON invoices (vendor_id);

-- Right: runs concurrently, does not lock
CREATE INDEX CONCURRENTLY invoices_vendor_idx ON invoices (vendor_id);
```

Note: `CREATE INDEX CONCURRENTLY` cannot run inside a transaction. Each index creation must be its own migration file.

### 44.3.3 Renaming a Column

**Deployment 1:** Add new column, keep old column, update application to write both:
```sql
ALTER TABLE invoices ADD COLUMN supplier_id UUID;
```
Application code writes to both `vendor_id` and `supplier_id`.

**Deployment 2:** Backfill `supplier_id` from `vendor_id`. Application reads from `supplier_id`.

**Deployment 3:** Remove `vendor_id`. Application stops writing to `vendor_id`.
```sql
ALTER TABLE invoices DROP COLUMN vendor_id;
```

### 44.3.4 Dropping a Table

Tables are never dropped immediately. The process:
1. Remove all application code that reads or writes the table
2. Deploy
3. Wait 2 weeks (confirms no unexpected references)
4. Drop the table in a migration

---

## 44.4 SDUI Schema Migrations

### 44.4.1 Additive Schema Changes

New component types and new optional properties are always additive:

```json
// Before
{ "type": "action.button", "label_key": "..." }

// After: new optional property added
{ "type": "action.button", "label_key": "...", "variant": "primary" }
```

Old clients that do not understand `variant` ignore it. No migration needed.

### 44.4.2 Breaking Schema Changes

When a component type is renamed or a required property changes:

1. Add the new type/property alongside the old one
2. Surface compiler outputs both (for compatibility)
3. All clients have deployed support for the new type/property
4. Old type/property is marked deprecated in the schema
5. After the deprecation window, old type/property is removed from the compiler output

The schema's `version` field enables clients to declare which schema features they support.

### 44.4.3 Surface Definition Migration Jobs

When existing surface definitions need updating (e.g., a property was renamed), a migration job updates all stored surface definitions:

```go
// Temporal workflow: migrate surface definitions
func MigrateSurfaceDefinitions(ctx workflow.Context, input MigrationInput) error {
    surfaces, err := listAllSurfaces(ctx)
    if err != nil {
        return err
    }

    for _, surface := range surfaces {
        err := workflow.ExecuteActivity(ctx, migrateSurface, surface.ID).Get(ctx, nil)
        if err != nil {
            // Log but continue; migration is idempotent, can be retried
            workflow.GetLogger(ctx).Error("surface migration failed",
                "surface_id", surface.ID,
                "error", err,
            )
        }
    }
    return nil
}
```

---

## 44.5 API Version Migration

See Chapter 25 for the API versioning policy. From a migration perspective:

- **New endpoint:** Deploy the new endpoint. Both old and new coexist.
- **Deprecating an endpoint:** Add `X-Deprecated` and `Sunset` headers. Wait 180 days.
- **Removing an endpoint:** Only after the Sunset date has passed. Return 410 Gone.

The 180-day window gives all client applications (web, iOS, Android, third-party integrations) time to migrate. The window is logged and tracked in a deprecation registry:

```yaml
# deprecations.yaml
deprecations:
  - endpoint: "GET /api/v1/invoices/{id}/vendor"
    deprecated_in: "2.3.0"
    deprecated_at: "2024-01-15"
    sunset_at: "2024-07-15"
    replacement: "GET /api/v1/invoices/{id}"
    reason: "Vendor details are now included in the invoice response"
```

---

## 44.6 Tenant Data Migration

### 44.6.1 When Tenant Data Migration Is Needed

Tenant data migrations are required when:
- A new data model replaces an old one (e.g., addresses stored in JSONB moved to a normalized table)
- A new required custom field is added to all tenants' entities
- A third-party integration changes its data format

### 44.6.2 Migration Design

Every tenant data migration is:
1. **Opt-in first:** Tenants can be migrated individually during a rollout, not all at once
2. **Monitored:** The migration job emits progress metrics per tenant
3. **Rollbackable:** The old data is preserved (soft migration) until the new data is verified

```go
type TenantDataMigration struct {
    MigrationID string
    Name        string
    // Returns true if this tenant needs migration
    NeedsMigration func(ctx context.Context, tenantID string) (bool, error)
    // Runs the migration for one tenant; must be idempotent
    Migrate        func(ctx context.Context, tenantID string) error
    // Verifies migration was successful
    Verify         func(ctx context.Context, tenantID string) error
}
```

The migration platform tracks which tenants have completed each migration in the `tenant_migrations` table.

---

## 44.7 Rollback Procedures

### 44.7.1 Code Rollback

Code rollbacks deploy the previous container image. Because migrations are always additive (new columns exist, old code ignores them), rolling back code does not require rolling back schema.

If schema was already migrated forward, the old code runs on the new schema. This is only safe because:
- New NOT NULL columns have defaults
- Old code ignores unknown columns (schema evolution by addition)
- New endpoints do not break old API consumers

### 44.7.2 Schema Rollback

Schema rollbacks are a last resort. The down migration must be tested before it is needed:

```bash
# Test the down migration locally before shipping the up migration
make migrate-up   # Apply up
make migrate-down # Apply down
make migrate-up   # Apply up again
# Verify the DB is in the expected state
```

For production schema rollback:
```bash
# Run the down migration
kubectl exec -it migrate-pod -- ./migrate -down -steps 1

# Verify
kubectl exec -it postgres-pod -- psql -c "\d invoices"
```

### 44.7.3 Data Migration Rollback

If a data migration has gone wrong, the rollback procedure depends on what was done:
- **Soft migration (old data preserved):** Swap the read path back to the old column/table
- **Destructive migration (old data modified):** Restore from the backup taken before the migration began (every migration that modifies existing data requires a backup taken within the previous 6 hours)

The production runbook for each data migration documents the rollback procedure before the migration runs.

---

*End of Chapter 44*

**Previous:** [Chapter 43 — IDE Tooling](./43-ide-tooling.md)
**Next:** [Chapter 45 — Governance](./45-governance.md)
