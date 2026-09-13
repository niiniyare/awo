> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "ADR-024: Single-Schema Multi-Tenancy over Schema-per-Tenant"
id: adr-024
status: accepted
category: ADR
stability: STABLE
audience: [framework-authors, operators]
since: "1.0"
normative-level: informative
related:
  - "[Tenant Model](../06-tenancy/tenant-model.md)"
  - "[RLS Deep Dive](../05-persistence/rls-deep-dive.md)"
  - "[ADR-002](adr-002-postgres-rls-tenancy.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# ADR-024: Single-Schema Multi-Tenancy over Schema-per-Tenant

**Status**: Accepted
**Date**: 2024-01-18
**Deciders**: Awo Framework Team

---

## Context

PostgreSQL supports multiple tenancy architectures:

1. **Single database, single schema, RLS** — all tenants share tables; `tenant_id` column + RLS policies
2. **Single database, schema-per-tenant** — each tenant has their own `tenant_abc` schema
3. **Database-per-tenant** — each tenant has a dedicated PostgreSQL database

Awo targets small-to-medium East African businesses. A deployment may serve 10–500 tenants. Cost efficiency, operational simplicity, and migration consistency are primary concerns.

---

## Decision

Use **single-schema multi-tenancy** with PostgreSQL Row-Level Security (RLS).

Every tenant-scoped table has a `tenant_id uuid NOT NULL` column. RLS policies enforce isolation at the database level. The application sets `app.current_tenant_id` via `set_tenant_context()` before any query.

---

## Consequences

### Positive

- **Operational simplicity**: one schema to back up, migrate, monitor
- **Consistent migrations**: one `ALTER TABLE` applies to all tenants simultaneously
- **Connection pooling**: PgBouncer serves all tenants from one pool — no per-tenant connections
- **Cost efficiency**: one database instance serves all tenants
- **RLS enforcement at DB level**: even a bug in application code cannot leak cross-tenant data if RLS is correct

### Negative

- **Schema changes affect all tenants simultaneously**: a bad migration cannot be rolled back for just one tenant
- **Noisy neighbor risk**: one tenant's heavy query load can affect others — mitigated by query timeouts and rate limiting
- **No tenant-specific schema extensions**: custom columns are via `custom_fields jsonb`, not actual SQL columns
- **RLS bugs are global**: a misconfigured RLS policy affects all tenants

### Neutral

- Tenant data isolation is not weaker than schema-per-tenant for a correctly configured RLS setup
- Regulatory compliance (data residency) would require database-per-tenant — currently out of scope

---

## Alternatives Rejected

### Schema-per-Tenant

Rejected because:
- Migration complexity: `N tenants × M migrations` = `N × M` migration operations
- `golang-migrate` was designed for single-schema — schema-per-tenant requires custom migration orchestration
- Adding a tenant requires DDL; DDL during high load causes table locks
- Connection pools cannot be shared across schemas efficiently
- Operational overhead scales linearly with tenant count

### Database-per-Tenant

Rejected because:
- Cost: separate PostgreSQL instance per tenant
- Operational complexity: monitoring N databases
- PgBouncer pooling does not work across databases

---

## RLS Correctness Requirements

The single-schema approach is only secure with:

1. Every tenant-scoped table has `FORCE ROW LEVEL SECURITY` (not just `ENABLE`)
2. `set_tenant_context()` validates tenant is ACTIVE before `set_config()`
3. PgBouncer in **transaction mode** (not session mode) so `SET LOCAL` resets on transaction end
4. Application role has no `BYPASSRLS` attribute
5. `platform-admin` operations use the migration role, not the application role

See [RLS Deep Dive](../05-persistence/rls-deep-dive.md) for the full security model.

---

## Related Documents

- [ADR-002](adr-002-postgres-rls-tenancy.md) — PostgreSQL RLS choice
- [ADR-010](adr-010-pgbouncer-transaction-mode.md) — PgBouncer transaction mode requirement
- [RLS Deep Dive](../05-persistence/rls-deep-dive.md) — RLS policy design and coverage verification
- [Tenant Model](../06-tenancy/tenant-model.md) — tenant entity and lifecycle
