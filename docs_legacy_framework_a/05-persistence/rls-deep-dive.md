> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Row-Level Security Deep Dive"
id: pers-010
status: accepted
category: SPEC
stability: STABLE
audience: [framework-authors, operators]
since: "1.0"
normative-level: normative
related:
  - "[Tenant Isolation](../06-tenancy/README.md)"
  - "[ADR-002: PostgreSQL RLS for Tenant Isolation](../17-adr/adr-002-postgres-rls-tenancy.md)"
  - "[Troubleshooting](../14-operations/troubleshooting.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Row-Level Security Deep Dive

**PERS-010 | Status: Accepted | Stability: Stable**

PostgreSQL Row-Level Security (RLS) is the cornerstone of Awo's tenant isolation. This document covers the implementation in detail: how tenant context is set, how policies evaluate, and how to verify correctness.

---

## 1. How Tenant Context Flows

```
HTTP Request
    ↓
Tenant middleware: identify tenant from X-Tenant-ID header
    ↓
store.SetTenantContextFromCtx(ctx)
    ├── Acquires pgx connection from PgBouncer pool
    ├── Calls set_tenant_context($tenant_id) stored procedure
    │   ├── Validates tenant exists in `tenants` global table
    │   ├── Validates tenant.status = 'ACTIVE'
    │   └── Calls SET LOCAL app.current_tenant_id = $tenant_id
    │       (LOCAL = transaction-local, resets on COMMIT/ROLLBACK)
    └── Returns connection with tenant context set
         ↓
All subsequent queries on this connection filtered by RLS
```

---

## 2. The Stored Procedure

```sql
CREATE OR REPLACE FUNCTION set_tenant_context(p_tenant_id uuid)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
AS $$
BEGIN
    -- Validate tenant exists and is active
    IF NOT EXISTS (
        SELECT 1 FROM tenants
        WHERE id = p_tenant_id AND status = 'ACTIVE'
    ) THEN
        RAISE EXCEPTION 'tenant_not_found_or_inactive: %', p_tenant_id
            USING ERRCODE = 'P0001';
    END IF;

    -- Set transaction-local config variable
    PERFORM set_config('app.current_tenant_id', p_tenant_id::text, TRUE);
END;
$$;

CREATE OR REPLACE FUNCTION current_tenant_id()
RETURNS uuid
LANGUAGE sql
STABLE
AS $$
    SELECT nullif(current_setting('app.current_tenant_id', TRUE), '')::uuid;
$$;
```

`set_config(..., TRUE)` — the `TRUE` flag means LOCAL (transaction-scoped). It resets automatically on `COMMIT` or `ROLLBACK`. PgBouncer transaction mode returns the connection to the pool after COMMIT, ensuring no tenant context leaks to the next request.

---

## 3. RLS Policy Per Table

Every tenant-scoped table has:

```sql
ALTER TABLE finance_invoice ENABLE ROW LEVEL SECURITY;
ALTER TABLE finance_invoice FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON finance_invoice
    AS PERMISSIVE
    FOR ALL
    TO app_role
    USING (tenant_id = current_tenant_id());
```

`FORCE ROW LEVEL SECURITY` ensures the policy applies even to the table owner. Without it, superusers and table owners bypass RLS.

`AS PERMISSIVE` — the default. A record is accessible if any PERMISSIVE policy allows it (or if no policies exist). `AS RESTRICTIVE` would require ALL policies to pass — not used here.

---

## 4. Global Tables (No RLS)

These tables are accessible without tenant context — they contain cross-tenant reference data:

```
tenants           — read-only after bootstrap
iam_audit_log         — read by service layer with explicit tenant filter
timezones         — reference data
currencies        — reference data
countries         — reference data
paye_bands        — KRA tax bands (Kenya statutory)
platform_admins   — platform admin user records
casbin_rule       — RBAC policies (all tenants)
```

The application role has `SELECT` on global tables. `UPDATE`/`DELETE` restricted to the migration role.

---

## 5. Verifying RLS Coverage

Run this SQL to identify tenant-scoped tables missing RLS policies:

```sql
SELECT
    c.relname AS table_name,
    c.relrowsecurity AS rls_enabled,
    c.relforcerowsecurity AS rls_forced,
    (SELECT count(*) FROM pg_policies p WHERE p.tablename = c.relname) AS policy_count
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = 'public'
  AND c.relkind = 'r'
  AND c.relname NOT IN (
    'tenants', 'iam_audit_log', 'timezones', 'currencies', 'countries',
    'paye_bands', 'platform_admins', 'casbin_rule', 'schema_migrations',
    'outbox_events', 'naming_series_counters'
  )
ORDER BY c.relname;
```

Expected result for all tenant-scoped tables:
- `rls_enabled = true`
- `rls_forced = true`
- `policy_count >= 1`

---

## 6. Testing RLS Isolation

Integration test to verify cross-tenant isolation:

```go
func TestRLSIsolation(t *testing.T) {
    tenantA := setupTestTenant(t)
    tenantB := setupTestTenant(t)

    // Create invoice in tenant A
    ctxA := withTenantContext(t, tenantA.ID)
    invoice, _ := invoiceRepo.Create(ctxA, def.CreateInput{
        Fields: map[string]any{"total_kes": "5000.0000", ...},
    })

    // Query from tenant B's context — should return nothing
    ctxB := withTenantContext(t, tenantB.ID)
    results, _, err := invoiceRepo.Query(ctxB, filter.Eq("id", invoice.ID))
    require.NoError(t, err)
    assert.Empty(t, results, "Tenant B must not see Tenant A's invoice")

    // Direct ID fetch from wrong tenant — must return 404
    _, err = invoiceRepo.Get(ctxB, invoice.ID)
    require.ErrorIs(t, err, def.ErrNotFound)
}
```

---

## 7. Common RLS Failure Modes

### Symptom: Cross-tenant data visible

**Cause**: `set_tenant_context()` not called before query. Typical in:
- Temporal activities that forgot to call `store.SetTenantContextFromCtx(ctx)`
- Background goroutines that use a context without tenant information
- Tests that query without establishing tenant context

**Fix**: Call `store.SetTenantContextFromCtx(ctx)` at the start of every database operation that touches tenant-scoped tables.

### Symptom: 403 on valid tenant request

**Cause**: Tenant status is not `ACTIVE` (PENDING, SUSPENDED, or ARCHIVED). The stored procedure raises an exception.

**Fix**: Check `tenants.status` for the affected tenant.

### Symptom: RLS bypassed (all tenants' data returned)

**Cause**: Query executed using a role with `BYPASSRLS` privilege (e.g., the migration role or superuser).

**Fix**: Application role must NOT have `BYPASSRLS`. Verify:
```sql
SELECT rolbypassrls FROM pg_roles WHERE rolname = 'app_role';
-- Must return: f
```

### Symptom: PgBouncer session mode leaking tenant context

**Cause**: PgBouncer configured in session mode. `SET LOCAL` resets on COMMIT, but in session mode the same backend connection is reused across requests from different users. The previous request's `set_config` value may bleed into the next request's query before `set_tenant_context` is called.

**Fix**: PgBouncer MUST be in transaction mode. See [ADR-010](../17-adr/adr-010-pgbouncer-transaction-mode.md).

---

## 8. Performance Impact

RLS policies add a predicate to every query: `AND tenant_id = current_tenant_id()`. This is equivalent to a manually added WHERE clause. With a B-tree index on `tenant_id`, the overhead is a single index lookup — negligible.

Every tenant-scoped table has an index on `tenant_id`:

```sql
CREATE INDEX ON finance_invoice(tenant_id);
-- Or as part of composite index:
CREATE INDEX ON finance_invoice(tenant_id, created_at DESC);
```

Queries that filter by both `tenant_id` and another column (e.g. `status`) benefit from composite indexes:

```sql
CREATE INDEX CONCURRENTLY ON finance_invoice(tenant_id, status)
WHERE status IN ('Draft', 'Submitted');
```

---

## Related Documents

- [Tenant Provisioning](../06-tenancy/tenant-provisioning.md) — when `set_tenant_context` is seeded
- [PgBouncer Transaction Mode](../17-adr/adr-010-pgbouncer-transaction-mode.md) — why session mode breaks RLS
- [Troubleshooting](../14-operations/troubleshooting.md) — RLS cross-tenant leak symptoms
- [Migrations](../14-operations/migrations.md) — adding RLS to new tables
