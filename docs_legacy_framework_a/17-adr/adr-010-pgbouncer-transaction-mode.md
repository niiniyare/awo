> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "ADR-010: PgBouncer Transaction Mode Required"
id: adr-010
status: accepted
category: ADR
stability: FROZEN
audience: [operators, framework-authors]
since: "1.0"
normative-level: informative
related:
  - "[Tenant Model](../06-tenancy/tenant-model.md)"
  - "[RLS](../06-tenancy/rls.md)"
  - "[Deployment](../14-operations/deployment.md)"
  - "[Architecture Laws](../02-architecture/laws.md)"
---

# ADR-010: PgBouncer Transaction Mode Required

**Status:** Accepted
**Date:** 2024-02-18
**Authors:** Framework Team

---

## Context

Awo uses PostgreSQL Row-Level Security with a transaction-local variable (`app.current_tenant_id`) set via `set_tenant_context()`. The variable is set with `set_config('app.current_tenant_id', $1, TRUE)` where `TRUE` means transaction-local — the variable resets when the transaction ends (COMMIT or ROLLBACK).

This mechanism requires that database connections are not shared across requests while a transaction is in progress. PgBouncer, the connection pooler used in front of PostgreSQL, offers three pooling modes with different connection-sharing semantics.

---

## Options Considered

### PgBouncer Session Mode

A client holds a server connection for the entire duration of the client's session. The server connection is returned to the pool only when the client disconnects.

Behavior: `set_config(..., TRUE)` sets a transaction-local variable. After COMMIT, the variable resets to NULL.

Risk: **Irrelevant in session mode** — the tenant context correctly resets after each transaction. However, session mode does not allow connection sharing between concurrent requests. A high-concurrency deployment would require as many server connections as concurrent clients.

Cons: Does not scale — each client holds a dedicated PostgreSQL connection.

### PgBouncer Statement Mode

Connection returned to pool after each SQL statement. No multi-statement transactions possible.

Cons: Incompatible with Awo — all entity mutations occur within explicit transactions (multiple statements: set_tenant_context, INSERT, outbox INSERT, COMMIT). Statement mode breaks multi-statement transactions.

### PgBouncer Transaction Mode

Connection returned to pool after each transaction. Multiple concurrent clients share a smaller pool of PostgreSQL connections.

Behavior: `set_config('app.current_tenant_id', $1, TRUE)` — the `TRUE` (transaction-local) flag means the variable resets automatically on COMMIT. The next transaction on the same connection sees `NULL` for `current_tenant_id()`.

This is correct: each transaction sets its tenant context at the start via `set_tenant_context()`, uses the connection, and the variable automatically resets on COMMIT. The next user of the connection cannot see the previous tenant's context.

Pros: High connection multiplexing, correct RLS variable reset, compatible with multi-statement transactions.

---

## Decision

**PgBouncer MUST operate in transaction mode.**

Transaction mode is the only mode that:
1. Correctly resets the tenant context variable after each transaction (session mode also resets it, but does not scale)
2. Supports multi-statement transactions (statement mode does not)
3. Provides connection multiplexing necessary for multi-tenant scale

Session mode would also be technically correct but would not scale. Transaction mode is the correct choice for production.

---

## Consequences

**Positive:**
- Correct tenant isolation across connection reuse
- High connection multiplexing — hundreds of tenants share tens of PostgreSQL connections
- Automatic tenant context reset eliminates a class of tenant leakage bugs

**Negative:**
- `SET` commands that rely on session state (not transaction-local) are broken in transaction mode
- Advisory locks (`pg_advisory_lock`) cannot span multiple transactions in transaction mode
- Prepared statements behave differently in transaction mode

**Verification:** The startup sequence verifies PgBouncer is in transaction mode (see [Configuration](../12-configuration/configuration.md#7-pgbouncer-transaction-mode-verification)).

---

## Revisit Trigger

If PostgreSQL native connection multiplexing matures (via logical replication slots or native pooling), the PgBouncer requirement may be revisited. The tenant context mechanism (`set_config`) must still be used — only the pooler requirement changes.
