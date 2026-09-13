> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Architecture Decision Records — Section Overview"
id: adr-000-readme
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: informative
related:
  - "[Architecture Laws](../02-architecture/laws.md)"
  - "[Invariants](../02-architecture/invariants.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Architecture Decision Records

**Section 17 | Architecture Decision Records (ADRs)**

Architecture Decision Records capture the reasoning behind significant decisions in Awo's design. Each ADR documents: the context, the decision, the consequences (positive and negative), and alternatives that were considered and rejected.

ADRs are immutable once accepted — they record history. If a decision is reversed, a new ADR is written superseding the original.

---

## Index

| ADR | Title | Status | Supersedes |
|---|---|---|---|
| [ADR-001](adr-001-temporal-for-workflows.md) | Temporal for Workflow Orchestration | Accepted | — |
| [ADR-002](adr-002-postgres-rls-tenancy.md) | PostgreSQL RLS for Tenant Isolation | Accepted | — |
| [ADR-003](adr-003-amis-sdui.md) | amis for Server-Driven UI | Accepted | — |
| [ADR-004](adr-004-filter-dsl.md) | Declarative Filter DSL over Query Builders | Accepted | — |
| [ADR-005](adr-005-server-side-sessions.md) | Server-Side Sessions over JWT | Accepted | — |
| [ADR-006](adr-006-outbox-pattern.md) | Transactional Outbox for Workflow Dispatch | Accepted | — |
| [ADR-007](adr-007-no-lazy-loading.md) | No Lazy Loading for Edges | Accepted | — |
| [ADR-008](adr-008-entity-naming-immutable.md) | Entity Names Are Immutable After First Migration | Accepted | — |
| [ADR-009](adr-009-uuid-v7-primary-keys.md) | UUID v7 for Primary Keys | Accepted | — |
| [ADR-010](adr-010-pgbouncer-transaction-mode.md) | PgBouncer Transaction Mode Required | Accepted | — |
| [ADR-011](adr-011-decimal-for-money.md) | Use decimal.Decimal for All Monetary Amounts | Accepted | — |
| [ADR-012](adr-012-fiber-http-framework.md) | Fiber v2 as the HTTP Framework | Accepted | — |
| [ADR-013](adr-013-pgx-over-orm.md) | pgx Over ORM for Database Access | Accepted | — |
| [ADR-014](adr-014-casbin-rbac.md) | Casbin for RBAC | Accepted | — |
| [ADR-015](adr-015-golang-migrate.md) | golang-migrate for Schema Migrations | Accepted | — |
| [ADR-016](adr-016-server-side-sessions.md) | Server-Side Sessions over JWT | Accepted | — |
| [ADR-017](adr-017-outbox-pattern.md) | Transactional Outbox for Workflow Dispatch | Accepted | — |
| [ADR-018](adr-018-uuid-v7-primary-keys.md) | UUID v7 for Primary Keys | Accepted | — |
| [ADR-019](adr-019-filter-dsl.md) | Declarative Filter DSL over Query Builders | Accepted | — |
| [ADR-020](adr-020-no-lazy-loading.md) | No Lazy Loading for Edges | Accepted | — |
| [ADR-021](adr-021-pgbouncer-transaction-mode.md) | PgBouncer Transaction Mode Required | Accepted | — |
| [ADR-022](adr-022-entity-naming-immutable.md) | Entity Names Are Immutable After First Migration | Accepted | — |
| [ADR-023](adr-023-wire-dependency-injection.md) | Google Wire for Dependency Injection | Accepted | — |
| [ADR-024](adr-024-single-schema-multitenancy.md) | Single-Schema Multi-Tenancy over Schema-per-Tenant | Accepted | — |
| [ADR-025](adr-025-go-slog-structured-logging.md) | Go slog for Structured Logging | Accepted | — |
| [ADR-026](adr-026-otel-for-tracing.md) | OpenTelemetry for Distributed Tracing | Accepted | — |
| [ADR-027](adr-027-prometheus-metrics.md) | Prometheus for Metrics | Accepted | — |

---

## How to Write a New ADR

See the [ADR Template](../00-documentation/adr-template.md) for the required format. ADRs are proposed as pull requests and accepted only after architecture review. An accepted ADR may be superseded but never deleted.
