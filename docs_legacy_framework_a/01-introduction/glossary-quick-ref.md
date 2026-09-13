> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Glossary Quick Reference"
id: intro-004
status: accepted
category: GUIDE
stability: STABLE
audience: [all]
since: "1.0"
normative-level: informative
related:
  - "[Awo Glossary](../GLOSSARY.md)"
  - "[Architecture Overview](architecture-overview.md)"
  - "[Philosophy](philosophy.md)"
---

# Glossary Quick Reference

**INTRO-004 | Status: Accepted | Stability: Stable**

A concise reference card for the most frequently used Awo terms. For complete definitions, see the [full Glossary](../GLOSSARY.md).

---

## Framework Core

| Term | One-line definition |
|---|---|
| **EntityDefinition** | The central declaration struct that drives persistence, API, SDUI, RBAC, and workflows simultaneously |
| **CompiledSchema** | The internal representation of EntityDefinition after the Compilation Phase — read-only at runtime |
| **Entity Registry** | The sealed collection of all registered EntityDefinitions, populated at startup |
| **EntityRecord** | A single instance of an entity — a map of field names to typed values |
| **SystemDefinition** | EntityDefinition backed by a SQL table with typed columns |
| **CustomDefinition** | EntityDefinition backed by a JSONB column in a generic custom entity table |

---

## Persistence

| Term | One-line definition |
|---|---|
| **EntityRepository[T]** | The interface all persistence operations go through — module code never calls SQL directly |
| **Filter** | A declarative predicate tree describing a row selection condition — composed with `filter.And`, `filter.Eq`, etc. |
| **PolicyFunc** | A function that returns a Filter injected into every query for a given actor — row-level security at the application layer |
| **QueryOption** | Modifiers on a Query call: `OrderBy`, `Page`, `PageSize`, `WithCursor`, `WithEdge` |
| **PageInfo** | Struct returned with every list query containing pagination state and total count |
| **NamingSeries** | Field type that generates sequential human-readable identifiers (e.g., `INV-2024-00001`) |

---

## Multi-Tenancy

| Term | One-line definition |
|---|---|
| **Tenant** | An isolated organizational unit; all data is partitioned by tenant |
| **TenantContext** | The Go `context.Context` carrying the active `tenant_id` — required for all repository operations |
| **RLS** | PostgreSQL Row Level Security — DB-level enforcement of tenant isolation |
| **set_tenant_context()** | PostgreSQL stored procedure that sets `app.current_tenant_id` transaction-locally |
| **PgBouncer** | Connection pooler; MUST run in transaction mode for RLS to work correctly |
| **Global Table** | A table without tenant_id column; readable by all tenants; managed by platform admins |

---

## IAM

| Term | One-line definition |
|---|---|
| **Actor** | The authenticated user (or API client) making a request |
| **Session Token** | A 256-bit random token stored in Redis that represents an authenticated session |
| **RBAC** | Role-Based Access Control — implemented via Casbin; governs which actions an actor may perform |
| **PolicyFunc** | (IAM context) Row-level filter function; distinct from Casbin RBAC — both are required |
| **Casbin** | Authorization library enforcing `(subject, domain, object, action)` policies |
| **requires_password_change** | Flag in session data forcing password change before any other API access |

---

## Workflows

| Term | One-line definition |
|---|---|
| **Workflow** | A Temporal workflow function — durable, deterministic, long-running process |
| **Activity** | A Temporal activity function — where I/O happens; retryable; idempotent |
| **Signal** | An asynchronous message sent to a running workflow to advance its state |
| **Query** | A synchronous read of a running workflow's in-memory state |
| **Saga Pattern** | Compensation-based distributed transaction: each step registers a compensating action run in reverse on failure |
| **Outbox** | The `workflow_outbox` table — ensures at-least-once workflow dispatch atomically with entity mutations |
| **Task Queue** | A Temporal routing key matching workflow/activity types to specific worker processes |

---

## SDUI

| Term | One-line definition |
|---|---|
| **SDUI** | Server-Driven UI — server generates complete JSON schemas; browser renders without custom JS |
| **amis** | The Baidu open-source React renderer that interprets Awo's page schemas |
| **Page Schema** | A JSON document describing a complete page (list, form, detail view, dashboard) |
| **PageBuilderSet** | The EntityDefinition field declaring custom page builder functions |
| **PageSchemaContext** | A helper struct in page builders providing `IfPermitted` and `FeatureFlagEnabled` checks |

---

## Operations

| Term | One-line definition |
|---|---|
| **Migration** | A `.up.sql` + `.down.sql` pair managed by `golang-migrate` — the only way to change the DB schema |
| **Zero-Downtime Deploy** | A deploy where availability is maintained throughout — achieved via `maxUnavailable: 0` + additive-only migrations |
| **Readiness Probe** | `GET /health/ready` — returns 200 only when PostgreSQL, Redis, and Entity Registry are all healthy |
| **Liveness Probe** | `GET /health/live` — returns 200 if the process is running; no dependency check |

---

## Related Documents

- [Full Glossary](../GLOSSARY.md) — complete definitions for all terms
- [Architecture Overview](architecture-overview.md) — how these concepts fit together
- [Architecture Laws](../02-architecture/laws.md) — the 20 binding rules
