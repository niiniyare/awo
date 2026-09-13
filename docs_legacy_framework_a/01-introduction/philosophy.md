> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Architectural Philosophy"
id: intro-001
status: accepted
category: THEORY
stability: STABLE
audience: [all]
since: "1.0"
normative-level: informative
related:
  - "[Design Goals](design-goals.md)"
  - "[Architecture Overview](architecture-overview.md)"
  - "[Architecture Laws](../02-architecture/laws.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Architectural Philosophy

**INTRO-001 | Status: Accepted | Stability: Stable**

This document states the five convictions on which the Awo Framework is founded. These convictions are not preferences or defaults. They are axioms — presuppositions that are not argued for within the framework, only reasoned from.

Every significant design decision in Awo follows from one or more of these axioms. Readers who find a design choice counterintuitive will, in most cases, find the explanation here.

This document is informative. It does not specify behavior. It explains why the behavior was chosen. Normative rules derived from these axioms appear in [`02-architecture/laws.md`](../02-architecture/laws.md).

---

## Table of Contents

1. [Tenancy Is Structural](#1-tenancy-is-structural)
2. [Metadata Drives Behavior](#2-metadata-drives-behavior)
3. [Compilation Produces an Immutable Contract](#3-compilation-produces-an-immutable-contract)
4. [The Specification Is Authoritative](#4-the-specification-is-authoritative)
5. [Correctness Is Paid Upfront](#5-correctness-is-paid-upfront)
6. [The Axioms Together](#6-the-axioms-together)

---

## 1. Tenancy Is Structural

### Statement

Multi-tenancy is the foundational assumption of the Awo Framework. The framework does not support multi-tenancy as a feature. It was built for multi-tenancy. Single-tenant operation is a degenerate case, not a primary mode.

### What This Means

In most frameworks, multi-tenancy is added after the core is designed. Tenant IDs are appended to queries. Middleware filters are bolted on. Auth checks are wrapped around existing logic. The result is a framework where the isolation boundary is a convention held together by discipline rather than structure.

Awo inverts this. Every abstraction in the framework — the [Entity Repository](../05-persistence/entity-repository.md), the [Session](../GLOSSARY.md#session), the [Policy Function](../GLOSSARY.md#policy-function-policyfunc), the [Middleware Pipeline](../GLOSSARY.md#middleware-pipeline), the [Audit Log](../GLOSSARY.md#audit-log) — assumes that the calling context belongs to exactly one tenant and that data from other tenants is unreachable.

This assumption is enforced structurally at the database layer through PostgreSQL [Row-Level Security](../06-tenancy/rls.md) (`FORCE ROW LEVEL SECURITY`). It is not a convention that application code must remember to apply. It is a property that the database enforces regardless of what application code does.

The consequence is that it is not possible to accidentally cross a tenant boundary from application code. The isolation is not trust-based. It is structural.

### What It Rules Out

This axiom rules out:

- **Shared tables without RLS.** Every tenant-scoped table must have RLS policies. Tables that serve multiple tenants without RLS are not permitted.
- **Optional tenancy.** There is no API for "tenant-optional" operations. Every operation is either tenant-scoped or platform-scoped; neither is optional.
- **Application-layer-only tenant filtering.** The WHERE clause `tenant_id = ?` in application code is supplementary at best and a false security assumption at worst. Database-level enforcement is non-negotiable.
- **Per-tenant database schemas or connections.** Awo uses a single shared PostgreSQL schema for all tenants, with isolation enforced through RLS. Per-tenant schemas do not scale to thousands of tenants.

### Why

ERP systems serve organizations. The data they contain — financial records, payroll, customer information, inventory — is legally owned by the tenant organization and must not be visible to any other organization. This is not a business preference. In many jurisdictions it is a legal requirement.

Treating tenancy as a structural property rather than a convention ensures that isolation cannot be accidentally broken by a contributor who forgets to add a WHERE clause, passes the wrong context, or calls the wrong function. The cost of a tenant isolation breach — regulatory exposure, customer trust destruction, litigation — is too high to accept isolation by convention.

---

## 2. Metadata Drives Behavior

### Statement

In Awo, behavior is declared, not programmed. An [EntityDefinition](../GLOSSARY.md#entitydefinition) is not a configuration file that influences how pre-written code behaves. It is a declaration from which the framework synthesizes behavior that does not exist until the declaration is processed.

A single `EntityDefinition` declaration drives five subsystems simultaneously: persistence routing, HTTP route generation, SDUI page schema generation, permission policy compilation, and workflow trigger binding. The declaration is not a hint to these subsystems. It is their authoritative instruction.

### What This Means

When a developer declares an entity, they do not write:
- A database migration (the framework identifies what schema changes are needed)
- Route handlers (the framework generates CRUD routes automatically)
- Permission checks (the framework compiles the declared `PermissionSet` into Casbin policies)
- UI forms (the framework generates amis JSON schemas automatically)
- Workflow bindings (the framework registers the `WorkflowTriggers` at startup)

They write one struct. Everything else is derived.

This is not magic. The derivation is deterministic and inspectable. The [compilation pipeline](../03-kernel/compilation-pipeline.md) transforms the declared metadata into each subsystem's specific artifacts. The artifacts are predictable, testable, and versioned.

### What It Rules Out

This axiom rules out:

- **Custom route handlers for standard CRUD.** `GET /`, `GET /:id`, `POST /`, `PATCH /:id`, `DELETE /:id` are generated. Writing these by hand is prohibited; it creates divergence between the declaration and the behavior.
- **Ad-hoc permission checks.** `if actor.Role == "admin"` in handler code is not permitted. Permissions are declared on the EntityDefinition and compiled into Casbin policies. Inline checks create holes in the permission model.
- **Custom UI for standard views.** List pages, create forms, edit forms, and detail views are generated from the EntityDefinition. Custom views are permitted via `PageBuilderSet` overrides but only where the generated views are genuinely insufficient.
- **Direct field mutation outside hooks.** Business logic that modifies entity fields belongs in hooks, declared on the EntityDefinition. Logic scattered in handler code is not subject to the framework's consistency guarantees.

### Why

ERP systems require consistency at scale. A typical ERP module has dozens of entities. A typical ERP deployment has hundreds of entities across all modules. Writing and maintaining bespoke route handlers, permission checks, UI forms, and permission policies for each entity does not scale.

More importantly, bespoke code is inconsistent code. Inconsistency accumulates. After three years, one entity's audit trail uses a different format than another's. One entity's list page sorts differently. One entity's permission check uses a slightly different role name. These inconsistencies become bugs in workflows that span entities.

Metadata-driven behavior eliminates inconsistency at the source. The same compilation pipeline produces every entity's behavior. The behavior is uniform because the code that produces it is the same code for every entity.

There is also a compounding benefit: when a behavior needs to change — say, the audit log format must be updated to include a new field — the change is made once, in the compilation pipeline, and applies to every entity simultaneously. In a bespoke system, this would require updating dozens of handlers.

---

## 3. Compilation Produces an Immutable Contract

### Statement

The Awo Framework distinguishes sharply between two phases: [Initialization](../GLOSSARY.md#initialization-phase) (during which entity definitions are registered) and [Runtime](../GLOSSARY.md#runtime-phase) (during which requests are served). These phases do not overlap.

At the end of the Initialization Phase, the [Entity Registry](../GLOSSARY.md#entity-registry) invokes the compiler, which produces a [CompiledSchema](../GLOSSARY.md#compiledschema). The CompiledSchema is immutable for the entire lifetime of the process. No registration, no field addition, no permission change, and no route modification may occur after the CompiledSchema is produced.

### What This Means

By the time the first HTTP request arrives, the system's behavior is fully determined. There are no runtime surprises. The framework can verify, before accepting any traffic, that:

- Every declared edge references an entity that exists
- Every declared `LinkTarget` references an entity that has been registered
- Every declared workflow trigger uses a valid event name
- Every declared role references a known permission scope
- Every declared field uses a registered field type handler

These verifications happen during compilation. A declaration error causes the process to exit before accepting traffic, rather than producing incorrect behavior at runtime.

The CompiledSchema also carries a [content hash](../GLOSSARY.md#content-hash-schema) that uniquely identifies the exact schema version. This hash is identical across all instances of the same binary running with the same configuration, which makes [schema divergence](../GLOSSARY.md#schema-divergence) detectable.

### What It Rules Out

This axiom rules out:

- **Hot-reload of entity definitions.** Adding or modifying an entity after startup is not supported. The process must be restarted.
- **Per-request schema variations.** The schema is the same for every request. Tenant-specific behavior is expressed through [Custom Fields](../GLOSSARY.md#custom-field) and [Feature Flags](../GLOSSARY.md#feature-flag), not through schema variations.
- **Dynamic route registration.** Routes are derived from the CompiledSchema once, at startup. Adding a route at runtime is not supported.
- **Lazy module loading.** All modules are compiled into the binary and registered at startup. The `plugin` package is intentionally not used.

### Why

Runtime schema mutation is a source of subtle, hard-to-reproduce bugs. A hot-reload that adds a field to an entity during a deployment rollout can cause in-flight requests to fail because they were serialized against the old schema. A race condition in route registration can cause some requests to be routed to the wrong handler.

ERP systems operate in regulated environments where consistency and predictability are legal requirements. An ERP system that behaves differently on different instances of the same binary, or that changes behavior mid-deployment, is not auditable.

Immutability after compilation also enables an important safety property: the framework can guarantee that every instance of the same binary serves identical behavior. This guarantee is the foundation of [LAW-019](../02-architecture/laws.md#law-019).

---

## 4. The Specification Is Authoritative

### Statement

The documentation of the Awo Framework is the specification of the Awo Framework. Source code is the implementation of that specification. When the two conflict, the source code is wrong.

This is not a documentation-first development methodology. It is a statement about the ontological status of the documentation: the specification defines what Awo is; the source code is one instantiation of that def.

### What This Means

A behavior that exists in source code but is not documented is not a public behavior. It is an implementation detail. It may be changed or removed without notice. Callers who depend on undocumented behavior have no stability guarantee.

A guarantee stated in the documentation is binding across all versions within the same major version, even if the guarantee is difficult to implement. If the implementation has not yet caught up to the guarantee, the implementation is in a defect state.

When documentation and implementation conflict, the resolution process is:
1. Confirm the documentation states the intended behavior correctly.
2. File an issue against the implementation.
3. Correct the implementation to match the documentation.

The reverse — silently altering the documentation to match a divergent implementation — is a governance violation.

### What It Rules Out

This axiom rules out:

- **"The code is the documentation."** Go source code with godoc comments is a supplement. It is not a substitute for the specification documents in `awo/docs/`.
- **Emergent behavior as API.** If a behavior emerges from the interaction of subsystems but is not specified, it is not API. Downstream code must not depend on it.
- **Specification-free changes.** A change to the framework's public behavior requires a documentation change first. A PR that changes behavior without a corresponding documentation update cannot be merged.
- **Retroactive documentation.** Documentation written after an implementation has shipped to justify the implementation's actual behavior is not specification-driven documentation. It is post-hoc rationalization.

### Why

Frameworks that use source code as their specification become increasingly difficult to evolve. The "specification" changes whenever the source code changes. Contributors cannot make principled decisions about backward compatibility because there is no stable contract to reason about.

ERP systems accumulate decades of data and business logic that depend on framework contracts. A framework that breaks contracts silently — because the contract lived only in source code and was refactored away — causes enormous downstream damage. The cost of maintaining a migration guide for a ten-year-old ERP database is orders of magnitude higher than the cost of maintaining a stable specification.

The specification-first approach also has an authorship benefit: writing a specification forces precision that implementation does not. An implementation that silently accepts inconsistent input and produces apparently reasonable output may mask a design error that the specification would have surfaced.

---

## 5. Correctness Is Paid Upfront

### Statement

The Awo Framework is designed to remain correct for twenty or more years of production operation. This design lifetime is not aspirational — it is a hard constraint that shapes every decision about complexity, backward compatibility, and technical debt.

The consequence is that complexity that would prevent incorrect behavior long-term is paid for upfront, even when it increases short-term implementation cost.

### What This Means

Several design decisions in Awo are more complex than alternatives that would work correctly today but degrade over time:

**The transactional outbox pattern** (rather than calling Temporal directly from within a transaction) is more complex than a direct call. The outbox guarantees that a committed entity record always has its associated workflow eventually started, even if the process crashes at the worst moment. Without the outbox, a deployment outage coinciding with a high-value financial workflow could silently drop the workflow start.

**The migration file model** (rather than ORM auto-migration) requires writing SQL explicitly. It prevents silent column drops. It provides a complete, auditable history of every schema change. It forces review of every migration before it runs.

**The explicit edge loading model** (rather than ORM lazy loading) requires the developer to declare what related data they need. It prevents N+1 queries from appearing silently when data volumes grow. A lazy-loading ORM may perform correctly at 10,000 records and fail at 1,000,000 because nobody noticed the query pattern until it was too late.

**The content hash schema identity** (rather than trusting that instances are consistent) adds overhead. It prevents a deployment that accidentally runs two different versions of the schema — one instance that got the new binary and one that didn't — from silently serving different behavior.

### What It Rules Out

This axiom rules out:

- **"We'll fix it later."** Known correctness problems that compound over time — such as a race condition in registration, a missing index that will degrade at scale, or a missing validation that will allow invalid data to accumulate — must be fixed before v1.0. They are not deferred.
- **Undocumented workarounds.** A workaround that is not documented is not a stable solution. It is invisible technical debt that the next developer will reimplement differently, creating two inconsistent workarounds.
- **Reversibility trade-offs.** Awo does not make the database schema easy to change at the cost of making it easy to get wrong. The migration model is explicit and slightly burdensome precisely to ensure that schema changes are intentional and reviewed.
- **Shortcuts in financial code.** Using a float for money because it is simpler is not permitted. Using a string for a currency amount because it avoids a decimal library dependency is not permitted. The correctness cost of these shortcuts is paid by customers, years later, as accumulated rounding errors.

### Why

ERP systems are not typical web applications. They accumulate financial records, payroll history, audit logs, and inventory movements that cannot be retroactively corrected without legal consequences. An ERP system that silently loses a financial workflow, silently rounds a currency amount, or silently leaks data across tenant boundaries does not merely have a bug. It has a liability.

The twenty-year design horizon is not an arbitrary number. It reflects the reality that organizations adopt ERP systems and then depend on them for decades. The data created in 2025 will be read in 2045. The schema designed today will constrain migrations in 2035. The contracts guaranteed by the framework today will be depended on by code that hasn't been written yet.

Technical debt in a framework compounds. A shortcut taken in 2025 generates maintenance burden in 2027, a bug report in 2029, and a migration crisis in 2032. The upfront cost of correctness is always lower than the deferred cost of incorrectness.

---

## 6. The Axioms Together

These five axioms are not independent. They reinforce each other.

**Tenancy is structural** creates the isolation requirement that **metadata drives behavior** must satisfy consistently across all entities. A metadata-driven system can enforce RLS for every entity uniformly; a bespoke-code system cannot.

**Compilation produces an immutable contract** enables **the specification to be authoritative**: because the compiled schema is frozen, the specification can make precise statements about runtime behavior without hedging about runtime mutability.

**Correctness is paid upfront** is the implementation consequence of all four preceding axioms. Each axiom requires a design choice that is more correct but more complex than its alternative. The philosophy as a whole chooses correctness.

The practical result is a framework with a steeper initial learning curve than a conventional Go web framework, and a dramatically lower long-term maintenance cost. The complexity is front-loaded, not deferred. The design decisions that feel burdensome on day one — explicit migrations, explicit edge loading, explicit tenant context — prevent categories of errors that would otherwise surface years later, in production, with data that cannot be recreated.

This is the trade-off Awo makes. It is not accidental. It is the considered position of the framework's design.

---

## Related Documents

- [Design Goals](design-goals.md) — translates these axioms into concrete, normative commitments
- [Architecture Overview](architecture-overview.md) — shows how these axioms manifest structurally
- [Architecture Laws](../02-architecture/laws.md) — the normative rules derived from these axioms
- [Architecture Invariants](../02-architecture/invariants.md) — the runtime properties guaranteed by these axioms
- [Glossary](../GLOSSARY.md) — canonical definitions for all terms introduced here
