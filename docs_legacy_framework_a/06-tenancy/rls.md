> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Row-Level Security"
id: ten-002
status: accepted
category: SPEC
stability: FROZEN
audience: [framework-authors, operators, contributors]
since: "1.0"
normative-level: normative
related:
  - "[Tenant Model](tenant-model.md)"
  - "[System Entities](../05-persistence/system-entities.md)"
  - "[Architecture Laws](../02-architecture/laws.md)"
  - "[Architecture Invariants](../02-architecture/invariants.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Row-Level Security

**TEN-002 | Status: Accepted | Stability: Frozen**

This document specifies PostgreSQL Row-Level Security as used in Awo: the required policy pattern, the `current_tenant_id()` function, global tables, and the application role requirements.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. The RLS Isolation Model

PostgreSQL Row-Level Security (RLS) is the authoritative tenant isolation enforcement mechanism. Application-layer tenant filtering is supplementary. See [LAW-015](../02-architecture/laws.md#law-015-database-enforces-tenant-isolation) and [INV-001](../02-architecture/invariants.md#inv-001-all-tenant-scoped-queries-execute-under-rls).

When a query executes against a tenant-scoped table, PostgreSQL applies the `USING` policy clause to each row before returning it. Rows that do not satisfy the policy are excluded at the database engine level — they are not returned even if the SQL query does not include a WHERE clause.

This means: even if application code issues `SELECT * FROM finance_invoice` without a tenant filter, it receives only the current tenant's records. The database enforces the boundary.

---

## 2. Mandatory RLS Block

Every tenant-scoped table MUST include this exact three-statement block:

```sql
ALTER TABLE {table_name} ENABLE ROW LEVEL SECURITY;
ALTER TABLE {table_name} FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON {table_name}
    USING (tenant_id = current_tenant_id());
```

### Why FORCE ROW LEVEL SECURITY

Without `FORCE`, table owners (the role used to create the table) bypass RLS policies. In PostgreSQL, the application role may be the table owner or may have elevated privileges.

`FORCE ROW LEVEL SECURITY` applies the policy to the table owner as well. This eliminates the bypass: no role, not even the owner, can retrieve rows from other tenants' data.

### The USING Clause

`USING (tenant_id = current_tenant_id())` is the predicate applied to every row.

`current_tenant_id()` is a PostgreSQL function that reads the transaction-local variable set by `set_tenant_context()`:

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

The `TRUE` parameter to `current_setting` suppresses the error if the variable is not set, returning NULL instead. When `current_tenant_id()` returns NULL, `tenant_id = NULL` is false for every row (SQL NULL semantics), so the policy excludes all rows.

This produces a safe fail-closed behavior: if `set_tenant_context()` was not called, no rows are returned.

---

## 3. Application Role Requirements

The PostgreSQL role used by the application MUST NOT be a superuser. Superusers bypass RLS unconditionally in PostgreSQL.

Recommended role setup:

```sql
-- Create application role (not superuser, not createrole)
CREATE ROLE awo_app LOGIN PASSWORD '...' NOSUPERUSER NOCREATEDB NOCREATEROLE;

-- Grant permissions on tenant-scoped tables
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO awo_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO awo_app;

-- Grant execute on stored procedures
GRANT EXECUTE ON FUNCTION set_tenant_context(uuid) TO awo_app;
GRANT EXECUTE ON FUNCTION current_tenant_id() TO awo_app;

-- Do NOT grant: BYPASSRLS, SUPERUSER
```

A migration runner role may be a separate role with broader privileges (to create tables, add constraints, create indexes). The migration runner MUST NOT be used for application queries.

---

## 4. Global Tables

Global tables are accessible to all tenants and contain platform-wide reference data. They MUST NOT have RLS policies.

```
tenants          — tenant registry; read-only for application
iam_audit_log        — cross-tenant audit records; append-only for application
timezones        — timezone reference data; read-only
currencies       — currency reference data; read-only
countries        — country reference data; read-only
paye_bands       — payroll tax bands; read-only
platform_admins  — platform admin registry; no application access
```

Global tables are readable by the `awo_app` role but not writable during normal request processing. The `tenants` table is writable only by tenant provisioning operations (platform admin scope).

---

## 5. Cross-Tenant Queries (Platform Admin)

Platform admin operations that span tenants (e.g., a platform admin listing all tenants with their record counts) must execute as a different role or with an explicit bypass:

```sql
-- Platform admin context (bypasses RLS via role, not via BYPASSRLS attribute)
SET ROLE awo_platform;  -- a role with higher privileges

SELECT tenant_id, COUNT(*) FROM finance_invoice GROUP BY tenant_id;

RESET ROLE;
```

The `awo_platform` role MUST NOT be available to normal request handlers. Platform admin operations MUST go through a separate code path that sets the role explicitly and resets it after the operation.

Platform admin operations MUST be recorded in the audit log with the platform admin identity and the `_platform_` domain.

---

## 6. Verifying RLS Enforcement

Integration tests MUST verify RLS enforcement:

```go
func TestRLS_TenantIsolation(t *testing.T) {
    tenantA := createTestTenant(t)
    tenantB := createTestTenant(t)

    // Create an invoice in tenant A's context
    ctxA := withTenantContext(ctx, tenantA.ID)
    invoiceA, _ := invoiceRepo.Create(ctxA, createInvoiceInput())

    // Query from tenant B's context — should return zero records
    ctxB := withTenantContext(ctx, tenantB.ID)
    records, _, _ := invoiceRepo.Query(ctxB, filter.None())
    assert.Empty(t, records, "tenant B should not see tenant A's invoices")

    // Query from tenant A's context — should return the record
    records, _, _ = invoiceRepo.Query(ctxA, filter.None())
    assert.Len(t, records, 1)
    assert.Equal(t, invoiceA.ID, records[0].ID)
}
```

This test MUST be part of the integration test suite for every entity type (or a shared test helper that runs against a sample entity).

---

## 7. Audit Log RLS

The audit log table is a global table (no RLS). All tenants' audit entries are in a single table. The application role has INSERT permission (to write audit entries) but SELECT permission is scoped:

- The application role queries audit entries with explicit `WHERE tenant_id = $1` (not RLS — this is intentional, as the audit log must be accessible to platform admins spanning tenants)
- Tenant-scoped queries to the audit log use an application-layer filter, not RLS

This exception to the RLS-first rule is documented here to prevent confusion. The audit log's access control is correct: tenant users see their own audit entries via application-layer filtering; platform admins see all entries.

---

## Related Documents

- [Tenant Model](tenant-model.md) — set_tenant_context() that enables this system
- [System Entities](../05-persistence/system-entities.md) — migration pattern for new RLS tables
- [Architecture Laws](../02-architecture/laws.md) — LAW-015
- [Architecture Invariants](../02-architecture/invariants.md) — INV-001
- [Glossary](../GLOSSARY.md) — RLS, set_tenant_context(), current_tenant_id(), Global Table
