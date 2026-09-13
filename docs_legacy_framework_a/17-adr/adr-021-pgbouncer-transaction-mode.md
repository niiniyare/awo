> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "ADR-021: PgBouncer Transaction Mode Required"
id: adr-021
status: accepted
category: ADR
stability: STABLE
audience: [framework-authors, operators]
since: "1.0"
normative-level: normative
related:
  - "[RLS Deep Dive](../05-persistence/rls-deep-dive.md)"
  - "[Tenant Model](../06-tenancy/tenant-model.md)"
  - "[Performance Tuning](../14-operations/performance-tuning.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# ADR-021: PgBouncer Transaction Mode Required

**Status: Accepted**

---

## Context

Awo uses `set_tenant_context()` — a PostgreSQL stored procedure that calls `SET LOCAL app.current_tenant_id = $1` — to establish tenant context for Row-Level Security. The `LOCAL` modifier makes this setting transaction-scoped: it resets automatically when the transaction commits or rolls back.

PgBouncer is used as a connection pool. PgBouncer supports two modes relevant to this decision:

1. **Session mode**: A client holds a backend connection for its entire session. `SET LOCAL` persists until the client disconnects.
2. **Transaction mode**: A client holds a backend connection only for the duration of one transaction. After COMMIT/ROLLBACK, the connection is returned to the pool.

---

## Decision

PgBouncer MUST be configured in **transaction mode** (`pool_mode = transaction`).

---

## Consequences

### Positive

**Tenant context isolation**: In transaction mode, `SET LOCAL` resets on COMMIT. When the connection is returned to the pool and reused by the next request, `app.current_tenant_id` is unset. The next request must call `set_tenant_context()` — which it will, via the `BeforeAcquire` pgx hook.

**Connection efficiency**: In session mode, a backend connection is held for the entire HTTP request processing time (including time spent in Go code, hook execution, etc.). In transaction mode, the connection is held only for the actual database transaction. At 100 concurrent users, transaction mode may need only 20-30 backend connections where session mode would need 100.

**No connection per tenant**: All tenants share the same connection pool. Tenant isolation is via RLS, not via separate pools. PgBouncer transaction mode makes this safe and efficient.

### Negative

**Session-level features unavailable**: PostgreSQL features that require a persistent session (advisory locks held across transactions, `SET` without LOCAL, prepared statements managed by the client) are unavailable with PgBouncer transaction mode. These are not used by Awo.

**pgx configuration required**: pgx must not use session-level features. pgx is configured with `simple` protocol (not extended protocol with server-side prepared statements) for PgBouncer compatibility.

---

## Failure Mode if Session Mode Is Used

If PgBouncer is accidentally configured in session mode:

1. Request A calls `set_tenant_context('tenant-A')` on connection C1.
2. Request A's transaction commits. Connection C1 remains allocated to Request A's session (session mode).
3. Request B from Tenant B reuses connection C1 (session mode means the same backend is reused).
4. If `set_tenant_context` is called for Tenant B before any query — correct isolation is maintained.
5. **But if** a query is executed before `set_tenant_context` (e.g., bug in middleware ordering), the query runs with Tenant A's `app.current_tenant_id` still set → Tenant B's query returns Tenant A's data.

This is a silent security failure. PgBouncer transaction mode eliminates this risk structurally — `SET LOCAL` is reset on COMMIT regardless of bug in calling code.

---

## Verification

```bash
# On PgBouncer admin console
psql -h pgbouncer -p 6432 -U pgbouncer pgbouncer -c "SHOW POOLS;"
# pool_mode column must show: transaction

# Or check pgbouncer.ini
grep pool_mode /etc/pgbouncer/pgbouncer.ini
# Must show: pool_mode = transaction
```

---

## Alternatives Considered

### Session mode with explicit RESET

Call `RESET app.current_tenant_id` after each request to clear the tenant context. Rejected:
- Relies on application code executing the reset — a bug or exception could skip it
- Adds a round-trip per request
- Does not solve the connection affinity problem

### Per-tenant connection pools

Each tenant gets its own pgx pool with credentials that restrict access to that tenant's rows. Rejected:
- Does not scale (thousands of tenants = thousands of pools)
- Does not leverage RLS (defeats the structural isolation benefit)
- Requires per-tenant PostgreSQL roles (complex provisioning)

---

## Related Documents

- [RLS Deep Dive](../05-persistence/rls-deep-dive.md) — why `SET LOCAL` must reset per-transaction
- [Performance Tuning](../14-operations/performance-tuning.md) — PgBouncer pool sizing
- [Deployment](../14-operations/deployment.md) — PgBouncer configuration in Kubernetes
