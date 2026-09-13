> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

# Global Tables

**Classification:** Reference — Tier 1
**Owner:** `04-multitenancy/GLOBAL_TABLES.md`
**Status:** Frozen at v1.0

---

## Purpose

This document lists all PostgreSQL tables that are exempt from Row Level Security and explains why each is exempt.

---

## Definition

A **global table** is a table that:
1. Does NOT have `ENABLE ROW LEVEL SECURITY`
2. Is accessible by the application role without a tenant context
3. Contains data that is legitimately cross-tenant or pre-tenant

Global tables are readable by the application role. Writes are restricted to the migration role or to specific platform functions.

---

## Global Table Inventory

| Table | Reason for Global Status | Write Access |
|-------|------------------------|--------------|
| `tenants` | Tenant identity; must be readable before `set_tenant_context()` to validate tenant existence | Migration role + tenant provisioning function |
| `audit_log` | Immutable; spans all tenants; queried by platform admins only | Audit pipeline (internal) |
| `timezones` | Reference data; identical for all tenants | Migration role only |
| `currencies` | Reference data; ISO 4217 currency codes | Migration role only |
| `countries` | Reference data; ISO 3166 country codes | Migration role only |
| `paye_bands` | Kenya PAYE tax bands; regulatory data | Migration role only |
| `platform_admins` | Platform-level administrator identities; pre-tenant | Migration role only |
| `workflow_outbox` | Cross-tenant outbox worker reads; filtered by status | Runtime pipeline (internal) |
| `event_outbox` | Cross-tenant outbox worker reads; filtered by status | Runtime pipeline (internal) |

---

## Security Model for Global Tables

Global tables MUST be restricted at the database role level:
- The `awo_app` role (used by the running application) has `SELECT` on all global tables
- The `awo_app` role has `INSERT`, `UPDATE`, `DELETE` ONLY on `workflow_outbox` and `event_outbox`
- All other writes to global tables require the `awo_migration` role

This prevents the application from accidentally writing to reference data tables.

---

## Adding a New Global Table

Adding a new global table requires explicit justification and approval because:
1. Global tables bypass the tenant isolation model
2. Every global table requires careful access control at the role level
3. Global tables are shared across all tenants — a bug affects all tenants simultaneously

**Criteria for global status:**
- The data exists before tenant context (like `tenants` itself)
- The data is logically shared across all tenants (reference data)
- The data spans tenant boundaries by design (outbox workers, audit)

Do not make a table global to avoid implementing RLS. If the data is tenant-specific, it MUST have RLS.

---

## References

- [`04-multitenancy/RLS_SPEC.md`](RLS_SPEC.md) — RLS enforcement on tenant-scoped tables
- [`15-migrations/RLS_TABLE_TEMPLATE.md`](../15-migrations/RLS_TABLE_TEMPLATE.md) — How to add RLS to new tables
