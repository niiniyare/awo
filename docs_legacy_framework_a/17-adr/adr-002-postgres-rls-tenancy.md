> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "ADR-002: PostgreSQL RLS for Tenant Isolation"
id: adr-002
status: accepted
category: ADR
stability: FROZEN
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: informative
related:
  - "[RLS](../06-tenancy/rls.md)"
  - "[Tenant Model](../06-tenancy/tenant-model.md)"
  - "[Architecture Laws](../02-architecture/laws.md)"
---

# ADR-002: PostgreSQL RLS for Tenant Isolation

**Status:** Accepted
**Date:** 2024-01-15
**Authors:** Framework Team

---

## Context

Awo is a multi-tenant ERP. Tenant data isolation is a hard requirement — Tenant A must never see Tenant B's data, even if application code contains a bug that omits a `WHERE tenant_id = ?` predicate.

The question was at which layer to enforce tenant isolation.

---

## Options Considered

### Option A: Application-Layer Enforcement Only

Every query includes `WHERE tenant_id = ?`. A central query builder ensures this.

Cons:
- One missed call = data breach
- Requires every developer to follow the pattern correctly
- Code paths that bypass the query builder (e.g., raw SQL, new features) are insecure by default
- No defense of last resort

### Option B: Separate Database Per Tenant (Database-per-Tenant)

Each tenant has its own PostgreSQL database.

Pros: Hard isolation — impossible to query another tenant's data.

Cons:
- Operational cost: N tenants = N databases to monitor, backup, patch
- Schema migrations must apply to all N databases (complex automation required)
- Connection overhead: cannot pool across tenants easily
- Not viable at scale (100+ tenants)

### Option C: Separate Schema Per Tenant (Schema-per-Tenant)

Each tenant has its own PostgreSQL schema within a shared database.

Pros: Reasonable isolation, migrations can run in a loop.

Cons:
- Schema migrations still need coordination across N schemas
- Connection pooling complications (must set `search_path` per connection)
- PgBouncer session mode required — conflicts with RLS requirements
- No framework-level enforcement that the schema is set correctly

### Option D: PostgreSQL Row-Level Security (FORCE RLS)

Single shared schema. Each table has a policy: `USING (tenant_id = current_tenant_id())`.
`FORCE ROW LEVEL SECURITY` ensures even the table owner cannot bypass it.
`current_tenant_id()` reads a transaction-local variable set by `set_tenant_context()`.

Pros:
- Defense of last resort: database enforces isolation regardless of application code
- Single schema: one migration applies to all tenants simultaneously
- Application code never needs `WHERE tenant_id = ?`
- Auditable: RLS policy is a SQL object, reviewable independently

Cons:
- Requires PgBouncer in **transaction mode** (session mode would retain the tenant variable across connections)
- Slightly higher query planning overhead (RLS adds a predicate to every plan)
- Requires `set_tenant_context()` to be called before every query — one missed call = 403 (correct fail-secure behavior)

---

## Decision

**Use PostgreSQL Row-Level Security with `FORCE ROW LEVEL SECURITY` as the primary isolation mechanism.**

The database-level enforcement provides a security guarantee that no application bug can bypass. The requirement for PgBouncer transaction mode is acceptable — it is a well-understood operational constraint.

---

## Consequences

**Positive:**
- Tenant isolation enforced at the database level — no application bug can leak data
- Single schema — one migration applies to all tenants
- Application code is simpler — never writes `WHERE tenant_id = ?`
- RLS policy is independently auditable

**Negative:**
- PgBouncer must be deployed in transaction mode — session mode silently breaks the tenant isolation
- Every query must call `set_tenant_context()` first — forgetting returns 0 rows (not an error), which can mask bugs in development

**Architecture Laws generated:**
- LAW-005: No store operation without tenant context
- LAW-015: Database enforces tenant isolation (FORCE RLS)
- INV-001: All tenant-scoped queries execute under RLS

---

## Revisit Trigger

If PostgreSQL RLS performance becomes a bottleneck at multi-million-tenant scale, evaluate database-per-tenant for the largest tenants with a shared pool for smaller tenants. The EntityRepository interface abstracts the storage implementation — switching the backing store does not require changing business logic.
