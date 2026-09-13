> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

# Row Level Security Specification

**Classification:** Specification — Tier 0
**Owner:** `04-multitenancy/RLS_SPEC.md`
**Status:** Frozen at v1.0

---

## Purpose

This document specifies the Row Level Security (RLS) enforcement model for the Awo Framework. RLS is the primary mechanism for tenant data isolation. No application-layer filtering supplements or replaces it.

## Scope

- `set_tenant_context()` stored procedure — the single RLS enforcement point
- PostgreSQL RLS configuration required on every tenant-scoped table
- PgBouncer configuration requirements
- `current_tenant_id()` function semantics
- Global tables exempt from RLS

## Dependencies

- [`04-multitenancy/TENANT_LIFECYCLE.md`](TENANT_LIFECYCLE.md) — Tenant status machine (ACTIVE check)

---

## 1. Design Philosophy

Awo uses a single PostgreSQL schema shared by all tenants. Tenant isolation is enforced at the database level using PostgreSQL Row Level Security, not at the application level using `WHERE tenant_id = ?` clauses.

This design means:
- A query that omits `tenant_id` filtering still returns only the current tenant's data
- A bug in application-layer filtering cannot expose cross-tenant data
- The isolation guarantee is enforced by the database engine, not by developer discipline

**This is not optional.** Application-layer tenant filtering is permanently prohibited.

---

## 2. The set_tenant_context() Stored Procedure

This is the **single RLS enforcement point** in the framework. It MUST be called before any tenant-scoped query.

```sql
CREATE OR REPLACE FUNCTION set_tenant_context(p_tenant_id uuid)
RETURNS void AS $$
DECLARE
    v_status text;
BEGIN
    -- Validate tenant exists and is ACTIVE
    SELECT status INTO v_status
    FROM tenants
    WHERE id = p_tenant_id;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'tenant_not_found: %', p_tenant_id
            USING ERRCODE = 'P0001';
    END IF;

    IF v_status != 'ACTIVE' THEN
        RAISE EXCEPTION 'tenant_not_active: % (status=%)', p_tenant_id, v_status
            USING ERRCODE = 'P0002';
    END IF;

    -- Set transaction-local GUC variable
    PERFORM set_config('app.current_tenant_id', p_tenant_id::text, TRUE);
END;
$$ LANGUAGE plpgsql;
```

The `TRUE` flag on `set_config` means the setting is **transaction-local** — it is automatically reset when the transaction commits or rolls back. This is why PgBouncer MUST be in transaction mode.

---

## 3. current_tenant_id() Function

```sql
CREATE OR REPLACE FUNCTION current_tenant_id()
RETURNS uuid AS $$
BEGIN
    RETURN current_setting('app.current_tenant_id', TRUE)::uuid;
EXCEPTION
    WHEN OTHERS THEN
        RETURN NULL;
END;
$$ LANGUAGE plpgsql STABLE;
```

Returns `NULL` if `set_tenant_context()` has not been called in the current transaction. When used in an RLS policy, a `NULL` return means the policy predicate evaluates to false, returning zero rows. This is the correct fail-safe behavior.

---

## 4. RLS Policy on Every Tenant-Scoped Table

Every table that stores tenant data MUST have these three SQL statements applied:

```sql
-- Required on every tenant-scoped table
ALTER TABLE {table_name} ENABLE ROW LEVEL SECURITY;
ALTER TABLE {table_name} FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON {table_name}
    USING (tenant_id = current_tenant_id());
```

`FORCE ROW LEVEL SECURITY` applies RLS even to the table owner. This prevents accidental bypass by the application role even if it is the table owner.

The policy predicate `tenant_id = current_tenant_id()` automatically filters all SELECT, UPDATE, and DELETE operations.

---

## 5. Migration Template

Every new tenant-scoped table migration MUST include the RLS setup. See [`15-migrations/RLS_TABLE_TEMPLATE.md`](../15-migrations/RLS_TABLE_TEMPLATE.md) for the complete template.

```sql
-- Example: add RLS to a new table
CREATE TABLE finance_invoice (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   uuid NOT NULL REFERENCES tenants(id),
    -- ... other columns
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

-- Required RLS setup:
ALTER TABLE finance_invoice ENABLE ROW LEVEL SECURITY;
ALTER TABLE finance_invoice FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON finance_invoice
    USING (tenant_id = current_tenant_id());

-- Required indexes:
CREATE INDEX finance_invoice_tenant_id_idx ON finance_invoice (tenant_id);
```

---

## 6. Application Code Integration

The Go application calls `set_tenant_context()` via a stored procedure invocation at the start of every request:

```go
// awo/runtime/tenant.go

func SetTenantContextFromCtx(ctx context.Context, db *pgxpool.Pool, tenantID uuid.UUID) error {
    _, err := db.Exec(ctx, "SELECT set_tenant_context($1)", tenantID)
    if err != nil {
        return fmt.Errorf("set_tenant_context: %w", err)
    }
    return nil
}
```

This is called by the middleware pipeline at step 6 (after ViewerContext is embedded), before any database operation.

**Prohibition:** Never call `SET LOCAL app.current_tenant_id = $1` directly. The stored procedure validates tenant existence and status. Bypassing the procedure bypasses these checks.

---

## 7. PgBouncer Configuration

PgBouncer MUST be configured in **transaction mode** (`pool_mode = transaction`).

```ini
# pgbouncer.ini
[databases]
awo = host=postgres dbname=awo

[pgbouncer]
pool_mode = transaction   # REQUIRED
```

**Why transaction mode:** The `set_tenant_context()` procedure sets a transaction-local GUC variable (`TRUE` flag). In transaction mode, connections are returned to the pool at transaction end, and the GUC resets with the transaction. In session mode, the connection persists across requests, and the GUC from request N could leak into request N+1 — a cross-tenant data exposure.

**Session mode is permanently prohibited** with this RLS design.

---

## 8. Verification Queries

To verify RLS is correctly configured on a table:

```sql
-- Check RLS is enabled and forced
SELECT relname, relrowsecurity, relforcerowsecurity
FROM pg_class
WHERE relname = 'finance_invoice';
-- Expected: relrowsecurity = true, relforcerowsecurity = true

-- Check policy exists
SELECT policyname, cmd, qual
FROM pg_policies
WHERE tablename = 'finance_invoice';
-- Expected: policyname = 'tenant_isolation', qual contains 'current_tenant_id()'

-- Test RLS in a psql session
SELECT set_tenant_context('tenant-uuid-here');
SELECT count(*) FROM finance_invoice;  -- should return only this tenant's rows
SELECT set_tenant_context('other-tenant-uuid');
SELECT count(*) FROM finance_invoice;  -- should return that tenant's rows only
```

---

## 9. Global Tables (Exempt from RLS)

See [`04-multitenancy/GLOBAL_TABLES.md`](GLOBAL_TABLES.md) for the complete list of tables that are legitimately exempt from RLS.

Global tables include: `tenants`, `audit_log`, `timezones`, `currencies`, `countries`, `paye_bands`, `platform_admins`.

These tables MUST be read-only for the application role (the role used by Awo). Write access is reserved for the migration role.

---

## 10. Normative Requirements

- Every tenant-scoped table MUST have `ENABLE ROW LEVEL SECURITY`.
- Every tenant-scoped table MUST have `FORCE ROW LEVEL SECURITY`.
- Every tenant-scoped table MUST have a `tenant_isolation` policy using `current_tenant_id()`.
- `set_tenant_context()` MUST be called before any tenant-scoped query.
- The stored procedure form MUST be used — not raw `SET LOCAL`.
- PgBouncer MUST operate in transaction mode.
- Application code MUST NOT use `WHERE tenant_id = ?` clauses.

---

## References

- `db/migration/YYYYMMDDHHMMSS_platform_rls.up.sql` — Platform RLS setup migration
- [`04-multitenancy/TENANT_LIFECYCLE.md`](TENANT_LIFECYCLE.md) — Tenant status validation
- [`04-multitenancy/GLOBAL_TABLES.md`](GLOBAL_TABLES.md) — Tables exempt from RLS
- [`15-migrations/RLS_TABLE_TEMPLATE.md`](../15-migrations/RLS_TABLE_TEMPLATE.md) — Migration template
