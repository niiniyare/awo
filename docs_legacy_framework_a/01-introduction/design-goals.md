> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Design Goals"
id: intro-002
status: accepted
category: SPEC
stability: STABLE
audience: [all]
since: "1.0"
normative-level: normative
related:
  - "[Philosophy](philosophy.md)"
  - "[Architecture Overview](architecture-overview.md)"
  - "[Architecture Laws](../02-architecture/laws.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Design Goals

**INTRO-002 | Status: Accepted | Stability: Stable**

This document states what the Awo Framework explicitly commits to achieving (goals), what it explicitly commits to not achieving (non-goals), the constraints that govern the design, and the criteria by which success can be measured.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, SHOULD NOT, RECOMMENDED, MAY, and OPTIONAL in this document are to be interpreted as described in [RFC 2119](https://www.rfc-editor.org/rfc/rfc2119).

Goals marked **[G-N]** are binding design commitments. Non-goals marked **[NG-N]** are explicit exclusions. Constraints marked **[C-N]** are invariant conditions the design must satisfy regardless of goal prioritization.

---

## Table of Contents

1. [Goals](#1-goals)
2. [Non-Goals](#2-non-goals)
3. [Constraints](#3-constraints)
4. [Success Criteria](#4-success-criteria)
5. [Goal Interaction and Priority](#5-goal-interaction-and-priority)

---

## 1. Goals

### G-1: Multi-Tenant ERP Operation at Scale

The framework MUST support hundreds of isolated tenants served from a single process without cross-tenant data leakage.

Tenant isolation MUST be structural (database-enforced via [RLS](../06-tenancy/rls.md)), not conventional (application-code-enforced via WHERE clauses). A single process failure, deployment error, or developer mistake MUST NOT cause data from one tenant to become visible to another.

The framework MUST remain performant across the full tenant lifecycle — from a tenant with zero records to a tenant with tens of millions — without requiring per-tenant infrastructure changes.

### G-2: Metadata-First Extensibility Without Code Changes

The framework MUST allow new [entities](../GLOSSARY.md#entity) and new [fields](../GLOSSARY.md#field) to be introduced without modifying framework source code.

Module authors MUST be able to introduce fully functional entities — with persistence, API routes, SDUI views, permission policies, and workflow triggers — by writing a single [EntityDefinition](../GLOSSARY.md#entitydefinition) declaration. No bespoke route handler, no bespoke permission check, no bespoke UI code should be required for standard CRUD operations.

Tenant administrators MUST be able to add [custom fields](../GLOSSARY.md#custom-field) to any existing entity at runtime, without migration and without redeployment.

### G-3: Financial and Inventory Integrity

All monetary values MUST be represented as `numeric(20,4)` at the database layer and `decimal.Decimal` in Go. Floating-point types are prohibited for monetary values unconditionally.

The framework MUST support double-entry accounting patterns natively: [LedgerEntry](../GLOSSARY.md#system-entity), [JournalEntry](../GLOSSARY.md#system-entity), and [Payment](../GLOSSARY.md#system-entity) entities are mandatorily [system entities](../GLOSSARY.md#system-entity) with typed SQL columns and database-level constraints.

Inventory accuracy MUST be maintained through typed SQL columns and database constraints on quantity fields, not through JSONB storage.

Every data mutation MUST produce a tamper-evident [Audit Log](../GLOSSARY.md#audit-log) entry in the same database transaction as the mutation.

### G-4: Long-Term Stability and Forward Compatibility

The framework MUST be designed for a minimum twenty-year production lifetime.

All public APIs in `def/`, `filter/`, and `driver/` MUST carry explicit [stability annotations](../GLOSSARY.md#stability-class). Interfaces marked FROZEN MUST NOT gain new required methods after v1.0. New capabilities on FROZEN interfaces MUST be expressed through optional interface extensions, never through breaking changes to the base interface.

Every breaking change MUST be preceded by a formal deprecation period of at least one major version. Breaking changes without deprecation are prohibited.

[Entity names](../GLOSSARY.md#entity-name), [field names](../GLOSSARY.md#field), and [workflow ID formats](../GLOSSARY.md#workflow-id) are embedded in migration filenames, Temporal event histories, and audit logs. These identifiers MUST remain stable once established in any non-DRAFT version.

### G-5: Idiomatic Go with No Hidden Magic

The framework MUST be implemented in idiomatic Go. Generics SHOULD be used where they improve type safety without obscuring control flow. Reflection MUST NOT be used in the hot path.

The `plugin` package MUST NOT be used. All modules are compiled into the binary. This is intentional: dynamic plugin loading introduces version skew, incompatible symbol tables, and security boundary violations that are unacceptable in a regulated ERP context.

Code generation MAY be used for boilerplate, but generated code MUST be readable, annotated as generated, and produce behavior identical to what a skilled developer would write by hand. Generated code is not exempt from code review.

The framework's public APIs MUST be comprehensible to an experienced Go developer reading them without documentation, as a sanity check on API design. Documentation supplements — it does not substitute for — clear API design.

### G-6: Regulatory Compliance for East African Markets

The framework MUST support the data structures required for Kenya Revenue Authority (KRA) eTIMS compliance: [TaxEntry](../GLOSSARY.md#system-entity) as a system entity with typed columns.

The framework MUST support payroll calculations that conform to PAYE band tables for Kenya, Uganda, Tanzania, and Rwanda. These tables are stored in global tables and MUST be updatable without schema changes.

DateTime serialization MUST store values as UTC internally and return them as EAT-offset ISO 8601 when serving Kenyan tenants.

Currency serialization in API responses MUST support Kenya locale formatting (`KES 1,234.5600`) while maintaining `numeric(20,4)` internal representation.

### G-7: Developer Leverage Proportional to Declaration Effort

The ratio of developer-authored code to framework-generated behavior MUST be substantially greater than 1:1 for standard ERP entities.

Concretely: declaring a new entity with ten fields, four permissions, one workflow trigger, and two custom actions MUST require less than two hundred lines of Go code and produce a fully functional entity with: a database schema, six HTTP routes, a list page, a create form, an edit form, a detail view, compiled Casbin policies, and a workflow binding — without any additional developer effort.

---

## 2. Non-Goals

### NG-1: General-Purpose Web Framework

Awo is not a general-purpose Go HTTP framework. It is an ERP framework built on top of Fiber v2. The HTTP layer exists to serve ERP entities. Developers who need a general-purpose HTTP framework should use Fiber, Chi, Echo, or net/http directly.

The framework will not support: arbitrary HTTP route shapes, response formats other than the standard envelope, or middleware patterns outside the fixed pipeline.

### NG-2: Microservices Decomposition

Awo is a monolith-first framework. A single process serves all tenant-scoped data. Microservices decomposition — splitting modules into independent services with independent databases — is explicitly out of scope for v1.0 and v2.0.

This is not a statement that microservices are wrong. It is a statement that the complexity of distributed data ownership, cross-service transactions, and inter-service API contracts should not be paid by teams who do not yet need it. Awo chooses operational simplicity at the cost of horizontal decomposability.

### NG-3: Object-Relational Mapping

The [EntityRepository](../GLOSSARY.md#entityrepository) interface is not an ORM. It does not provide: arbitrary join composition, lazy loading, identity maps, unit-of-work patterns, change tracking, or query builder DSLs.

ORM features that Awo explicitly excludes: lazy-loaded associations, raw query builder chains, schema inference from struct tags, automatic migration, and the active record pattern.

The EntityRepository provides a disciplined, narrow interface for the operations ERP logic actually needs. Developers who want a richer query interface should operate at the [driver layer](../GLOSSARY.md#driver) with explicit SQL, not through Awo's domain APIs.

### NG-4: Multi-Database Portability

PostgreSQL is the only supported data store for entity persistence. The [Driver](../GLOSSARY.md#driver) interface exists to enable testing and to abstract away `pgx` internals — not to enable portability to MySQL, SQLite, or NoSQL stores.

Awo relies on PostgreSQL-specific features that have no equivalents in other databases: `FORCE ROW LEVEL SECURITY`, `USING` clause policies, `CREATE INDEX CONCURRENTLY`, `numeric(20,4)`, transaction-local `set_config()`, GIN indexes on JSONB, and `tsvector` full-text search.

Porting Awo to another database would require reimplementing tenant isolation, which is a design change, not a configuration change.

### NG-5: Real-Time Streaming UI

The [SDUI layer](../GLOSSARY.md#sdui-server-driven-ui) using [amis](../GLOSSARY.md#amis) is appropriate for CRUD-oriented ERP views: lists, forms, dashboards, approval workflows. It is not appropriate for real-time streaming data — financial dashboards that update every second, live inventory counters, or collaborative editing.

Real-time features require custom WebSocket handlers or server-sent event streams. These are not provided by the framework.

### NG-6: Cloud-Provider Abstraction

Temporal, PostgreSQL, and Redis are production dependencies — not pluggable infrastructure choices. Awo does not provide abstractions to swap these for equivalent services from specific cloud providers.

The Driver interface enables testing (mocking the store) and future adaptation. It does not provide a stable multi-cloud abstraction that allows running Awo on DynamoDB, SQS, or managed Valkey without code changes.

### NG-7: Zero-Configuration Operation

Awo is explicitly opinionated. It requires explicit, validated configuration for all infrastructure dependencies. It does not infer configuration from environment conventions, cloud metadata services, or service discovery.

This is intentional: zero-configuration systems that silently fall back to defaults are inappropriate for financial systems where an incorrect database URL or Redis address causes silent data loss rather than an observable startup failure.

---

## 3. Constraints

These constraints are invariant. They are not goals to be balanced against other goals — they are conditions the design must satisfy regardless of other trade-offs.

### C-1: Tenant Isolation Is Non-Negotiable

No API, configuration option, or deployment mode may make it possible for data from one tenant to be visible to another. Tenant isolation is not a feature that can be disabled. It is a structural property of the system.

### C-2: Financial Calculations Must Be Exact

All monetary arithmetic must use exact decimal representation. Floating-point is prohibited not as a style rule but as a correctness constraint. A single floating-point rounding error in a financial calculation is a correctness failure, not a performance trade-off.

### C-3: Audit Trail Is Tamper-Evident

The audit log must record every data mutation and must not be modifiable by application code, tenant administrators, or framework code operating in a request context. Audit log records are inserted in the same transaction as the mutation they record. The audit log is a legal record.

### C-4: Security Controls Must Not Be Bypassable From Application Code

Row-Level Security, session validation, rate limiting, and RBAC enforcement must not be bypassable through any API the framework exposes to module authors or application developers. These are not guardrails — they are invariants.

### C-5: The Process Must Fail Fast Rather Than Start in a Degraded State

If any of the following fail at startup, the process must exit rather than start in a degraded state: PostgreSQL pool init, Redis client init, Entity Registry compilation. Serving requests without these dependencies is not acceptable degraded operation — it is undefined behavior that will produce incorrect results.

---

## 4. Success Criteria

A v1.0 release is successful when:

| Criterion | Measurable Definition |
|---|---|
| Developer leverage | A new entity with 10 fields, 4 permissions, 1 workflow trigger, and 2 custom actions requires fewer than 200 lines of Go and produces a fully functional entity |
| Tenant isolation | Zero cross-tenant data leakage incidents in 90 days of production operation with five or more active tenants |
| Financial accuracy | Zero floating-point rounding errors in any currency calculation in 90 days of production operation |
| Schema stability | Zero unplanned breaking changes to any FROZEN interface between v1.0 release and v1.1 release |
| Startup determinism | The process exits with a clear error message for every invalid configuration before accepting the first request |
| Audit completeness | 100% of data mutations produce an audit log entry, verified by integration test suite |
| Documentation coverage | Every public API in `def/`, `filter/`, `driver/` has a corresponding specification document |
| Migration safety | Zero unintentional data changes from any applied migration in 90 days of production operation |

---

## 5. Goal Interaction and Priority

When goals conflict, the following priority order resolves the conflict:

1. **Constraints** (C-1 through C-5) — absolute; override all goals
2. **G-4: Long-term stability** — breaking changes are never a short-term optimization
3. **G-1: Tenant isolation** — safety over convenience
4. **G-3: Financial integrity** — correctness over performance
5. **G-2: Metadata extensibility** — ergonomics for module authors
6. **G-7: Developer leverage** — productivity within the bounds set above
7. **G-5: Idiomatic Go** — style within the bounds set above
8. **G-6: Regulatory compliance** — specific market requirements within the general framework

Non-goals do not have priority positions — they are excluded, not deprioritized. A design that achieves a non-goal in order to achieve a goal is not making a priority trade-off. It is making a design error.

---

## Related Documents

- [Philosophy](philosophy.md) — the axioms from which these goals are derived
- [Architecture Overview](architecture-overview.md) — how the goals are realized structurally
- [Architecture Laws](../02-architecture/laws.md) — normative rules derived from goals and constraints
- [Architecture Invariants](../02-architecture/invariants.md) — runtime properties corresponding to constraints
- [Glossary](../GLOSSARY.md) — canonical definitions for all terms used in this document
