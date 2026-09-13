> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Architecture Laws"
id: arch-001
status: accepted
category: LAW
stability: FROZEN
audience: [framework-authors, module-authors, contributors, core-team]
since: "1.0"
normative-level: normative
related:
  - "[Architecture Invariants](invariants.md)"
  - "[Five-Layer Architecture](five-layer.md)"
  - "[Philosophy](../01-introduction/philosophy.md)"
  - "[Compilation Pipeline](../03-kernel/compilation-pipeline.md)"
  - "[EntityDefinition](../03-kernel/entity-def.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Architecture Laws

**ARCH-001 | Status: Accepted | Stability: Frozen**

This document states twenty Architecture Laws. Each law is permanently binding on all contributors to the Awo Framework, all module authors, all driver implementors, and all deployment operators. Laws are immutable: they may not be changed without a formal ADR that supersedes this document.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, SHOULD NOT, RECOMMENDED, MAY, and OPTIONAL in this document are to be interpreted as described in [RFC 2119](https://www.rfc-editor.org/rfc/rfc2119).

A law violation is a defect. It must be corrected before the code that violates it is merged. "It works" is not a justification for violating a law. A law violation that produces no observable misbehavior today is still a defect — it produces misbehavior under conditions not yet encountered.

---

## Table of Contents

- [LAW-001: def/ Has No Upward Imports](#law-001-def-has-no-upward-imports)
- [LAW-002: Runtime Consumes Only CompiledSchema](#law-002-runtime-consumes-only-compiledschema)
- [LAW-003: Registry Is Closed After Compilation](#law-003-registry-is-closed-after-compilation)
- [LAW-004: CompiledSchema Is Immutable](#law-004-compiledschema-is-immutable)
- [LAW-005: No Store Operation Without Tenant Context](#law-005-no-store-operation-without-tenant-context)
- [LAW-006: Outbox Entry and Entity Record Commit Atomically](#law-006-outbox-entry-and-entity-record-commit-atomically)
- [LAW-007: Hook Execution Order Is Deterministic](#law-007-hook-execution-order-is-deterministic)
- [LAW-008: Layer Imports Are Strictly Downward](#law-008-layer-imports-are-strictly-downward)
- [LAW-009: Driver Interfaces Are Stable After v1.0](#law-009-driver-interfaces-are-stable-after-v10)
- [LAW-010: Content Hash Is the Schema Identity](#law-010-content-hash-is-the-schema-identity)
- [LAW-011: Entity Names Are Globally Unique and Immutable](#law-011-entity-names-are-globally-unique-and-immutable)
- [LAW-012: Migrations Are Append-Only](#law-012-migrations-are-append-only)
- [LAW-013: Sensitive Fields Are Never Logged](#law-013-sensitive-fields-are-never-logged)
- [LAW-014: Filter Wire Format Is Versioned](#law-014-filter-wire-format-is-versioned)
- [LAW-015: Database Enforces Tenant Isolation](#law-015-database-enforces-tenant-isolation)
- [LAW-016: Workflow ID Format Is Canonical](#law-016-workflow-id-format-is-canonical)
- [LAW-017: Cross-Module Hook Registration Requires Open Policy](#law-017-cross-module-hook-registration-requires-open-policy)
- [LAW-018: Outbox Table Is Framework-Private](#law-018-outbox-table-is-framework-private)
- [LAW-019: Schema Is Identical Across All Instances](#law-019-schema-is-identical-across-all-instances)
- [LAW-020: All Public APIs Carry Stability Annotations](#law-020-all-public-apis-carry-stability-annotations)

---

## LAW-001: def/ Has No Upward Imports

**Scope:** All code in `awo/def/` and `awo/filter/`

The `def/` and `filter/` packages MUST NOT import any package from the framework runtime, driver implementations, platform modules, business modules, or any external infrastructure library (PostgreSQL drivers, Redis clients, Temporal SDK, HTTP frameworks).

`def/` MUST import only: Go standard library packages and packages within `def/` and `filter/` itself.

### Rationale

`def/` contains the framework's kernel types: [EntityDefinition](../GLOSSARY.md#entitydefinition), [FieldDef](../GLOSSARY.md#fielddef), [EdgeDef](../GLOSSARY.md#edgedef), [HookRegistration](../GLOSSARY.md#hookregistration), and related types. These types are the input to the compilation pipeline.

If `def/` imports the runtime, the runtime and the kernel types become circularly coupled. Module authors who import `def/` to declare entities would transitively import the entire framework runtime — creating binary-size inflation, import cycles, and testing friction.

`def/` with zero external dependencies can be imported by any module without side effects. This is the property that allows module packages to consist of pure declarations with no infrastructure coupling.

### Enforcement

The framework's CI MUST include an import graph check that fails the build if any code in `def/` or `filter/` imports outside the permitted set.

---

## LAW-002: Runtime Consumes Only CompiledSchema

**Scope:** All runtime code (route handlers, middleware, hook dispatcher, RBAC enforcement, SDUI generator)

All runtime subsystems MUST derive their behavior exclusively from the [CompiledSchema](../GLOSSARY.md#compiledschema). No runtime subsystem may access the original [EntityDefinition](../GLOSSARY.md#entitydefinition) structs after compilation is complete.

### Rationale

The CompiledSchema is the validated, resolved, and self-consistent snapshot of the entire framework schema. The original EntityDefinition structs are inputs to compilation; they may contain unresolved references, forward references, or fields that the compiler transforms.

Runtime code that reads EntityDefinition structs directly bypasses compiler validation and creates a second source of truth that can diverge from the compiled representation.

After `Registry.Compile()` completes, EntityDefinition references should be eligible for garbage collection (except insofar as they are embedded in the CompiledSchema).

### Permitted Exception

The compilation pipeline itself reads EntityDefinition structs. This is by definition permitted — the compiler is not runtime code.

---

## LAW-003: Registry Is Closed After Compilation

**Scope:** Entity Registry

The [Entity Registry](../GLOSSARY.md#entity-registry) MUST NOT accept any `def.Register()` call after `Registry.Compile()` has been called.

`def.Register()` calls that arrive after compilation MUST return an error or panic. The specific behavior (error vs. panic) is implementation-defined, but the registration MUST NOT succeed.

### Rationale

Accepting registrations after compilation invalidates the CompiledSchema. Route tables, permission policies, page schema indexes, and workflow trigger registrations are all derived from the state of the registry at compile time. A post-compilation registration would produce a schema state that is inconsistent with the derived artifacts.

More concretely: a registration that arrives after the HTTP server has started would produce an entity that has no route (routes were derived before the registration), no permission policy (policies were compiled before the registration), and no page schema (indexes were built before the registration). The entity would be visible to some subsystems and invisible to others.

### Enforcement

`def.Register()` checks an internal flag set by `Registry.Compile()`. The check is atomic. A test MUST verify that registration after compilation produces the expected failure.

---

## LAW-004: CompiledSchema Is Immutable

**Scope:** All code that receives or stores a CompiledSchema reference

After `Registry.Compile()` returns, the [CompiledSchema](../GLOSSARY.md#compiledschema) returned MUST be treated as immutable. No field, list, map, or nested value within the schema may be modified by any code.

CompiledSchema MUST be implemented as a value type or as a deeply-read-only reference type with no exported mutating methods.

### Rationale

The CompiledSchema is shared across all goroutines serving requests. A mutation to a shared schema value without synchronization is a data race. A mutation with synchronization defeats the purpose of pre-compilation (every request would need a lock on schema state).

Beyond concurrency safety, immutability ensures that the content hash remains valid. A mutable CompiledSchema could be modified after its hash was computed, producing a hash that no longer matches the content.

### Enforcement

The CompiledSchema type MUST expose no mutating methods. All fields MUST be unexported or of immutable types (strings, ints, slices returned by copy). A `go vet` check or a `staticcheck` linter rule MUST detect accidental mutation.

---

## LAW-005: No Store Operation Without Tenant Context

**Scope:** All code that calls EntityRepository methods on tenant-scoped entities

Every call to `EntityRepository.Get()`, `Query()`, `Create()`, `Update()`, `Delete()`, `BulkCreate()`, `BulkUpdate()`, `Exists()`, `Count()`, or `Aggregate()` on a tenant-scoped entity MUST be made with a `context.Context` that carries a resolved [TenantContext](../GLOSSARY.md#tenantcontext).

The EntityRepository implementation MUST reject context values that do not carry a TenantContext. Rejection MUST produce an error that causes the request to fail, not silently proceed with an empty tenant.

### Rationale

An entity store operation without a tenant context has two possible incorrect behaviors: it operates on data from all tenants (violating isolation) or it fails silently with an empty result set (masking a programming error as a successful no-data response).

Both behaviors are wrong. Both are harder to debug than an immediate, explicit error from the EntityRepository.

### Permitted Exception

Platform-scoped operations (e.g., reading from global tables like `tenants`, `currencies`, `countries`) do not require a tenant context. The EntityRepository implementations for global tables accept a platform-scoped [ViewerContext](../GLOSSARY.md#viewercontext) rather than a TenantContext.

---

## LAW-006: Outbox Entry and Entity Record Commit Atomically

**Scope:** All code that triggers a Temporal workflow as a consequence of an entity mutation

The [outbox](../GLOSSARY.md#outbox-pattern-transactional-outbox) entry for a workflow start MUST be written in the same PostgreSQL transaction as the entity record it is associated with. The Temporal `StartWorkflow` call MUST occur after the transaction commits, not inside the transaction.

No code may call `temporal.StartWorkflow()` from within a `WithTx` block.

### Rationale

A transaction that writes an entity record and starts a Temporal workflow has two failure modes:

1. The transaction commits, the process crashes before `StartWorkflow` → the workflow is never started
2. `StartWorkflow` succeeds, the transaction rolls back → the workflow runs without a corresponding entity record

The outbox pattern eliminates both failure modes. The outbox entry commits atomically with the entity record (same transaction). If the process crashes, the outbox relay restarts and dispatches the workflow start. The Temporal call occurs outside the transaction, so its failure does not roll back the entity record.

Atomicity of the entity record + outbox entry is guaranteed by PostgreSQL transactions. The outbox relay guarantees at-least-once dispatch (the workflow function must be idempotent for exactly-once semantics).

### Enforcement

No Temporal SDK import may appear in the Domain Layer. All Temporal calls occur in the Workflow Layer or in the outbox relay, which is framework infrastructure. A linter rule MUST detect Temporal SDK imports in domain or store layer code.

---

## LAW-007: Hook Execution Order Is Deterministic

**Scope:** The hook dispatcher

For a given entity and lifecycle stage, the execution order of registered hooks MUST be deterministic across all invocations and all process restarts.

Within a single module, hooks MUST execute in their declaration order (the order in which they appear in the `[]HookRegistration` slice). Across modules, hooks MUST execute in topological sort order of module dependency declarations — a module that depends on another always executes its hooks after the module it depends on.

### Rationale

Hook order matters for business correctness. A validator hook that runs before a normalizer hook will reject input that the normalizer would have made valid. An audit hook that runs before a numbering hook will record an invoice without a number.

Non-deterministic hook order makes business behavior unpredictable and makes bugs in hook interaction impossible to reproduce reliably. Deterministic order ensures that the interaction between hooks is specifiable, testable, and auditable.

### Enforcement

The registry MUST record hooks in the order they are registered (for within-module ordering) and MUST sort across modules by a deterministic topological key at compilation time. A test MUST verify that registering the same hooks in different orders within the same module produces the same execution sequence.

---

## LAW-008: Layer Imports Are Strictly Downward

**Scope:** All code in the framework and all modules

Code at any layer MUST NOT import code from a higher layer. The permitted import directions are:

```
UI Layer      → (no Go code; JSON schema only)
API Layer     → Domain Layer, Workflow Layer, Store Layer
Domain Layer  → (no imports from API, Workflow, or Store)
Workflow Layer→ Store Layer (via Activity functions only)
Store Layer   → (no imports from Domain, Workflow, or API)
```

The Domain Layer MUST have zero imports from framework runtime packages. It imports only `def/`, `filter/`, and the Go standard library.

### Rationale

Upward imports create circular dependencies and couple subsystems that must remain independent. The Domain Layer's zero-import property is particularly important: it allows domain logic to be tested with mock EntityRepository implementations without starting any infrastructure.

A Domain Layer that imports Fiber cannot be tested without an HTTP server. A Domain Layer that imports pgx cannot be tested without a PostgreSQL database. These testing constraints compound over the lifetime of the project and produce integration-test-only test suites that are slow, brittle, and expensive to run in CI.

### Enforcement

Go's compiler detects circular imports. The framework CI MUST include an import graph analysis that verifies the layer hierarchy at every commit.

---

## LAW-009: Driver Interfaces Are Stable After v1.0

**Scope:** All interfaces in `awo/driver/`

After v1.0 release, no interface in `awo/driver/` may gain a new required method. Required method additions are breaking changes for all existing driver implementations.

New capabilities MUST be expressed as optional interfaces: a separate interface with the new method that the runtime probes for using type assertion. Driver implementations that do not implement the optional interface receive the default (less capable) behavior.

### Rationale

Driver interfaces are the extension points for the framework's infrastructure abstraction. Implementors build drivers for their specific infrastructure (PostgreSQL implementations, test-double implementations, alternative implementations). Every new required method on a driver interface breaks all existing implementations simultaneously.

The optional interface pattern (also known as the probe-for-capability pattern) allows the framework to add capabilities without breaking existing drivers. A driver that implements the optional interface gets the new behavior; a driver that does not gets safe fallback behavior.

### Example

```go
// Core interface — never gains new required methods after v1.0
type EntityStore interface {
    Get(ctx context.Context, id uuid.UUID) (EntityRecord, error)
    Query(ctx context.Context, f Filter, opts ...QueryOption) ([]EntityRecord, PageInfo, error)
    // ...
}

// Optional capability added in v1.1 — does not break v1.0 drivers
type EntityStoreWithBulkRead interface {
    EntityStore
    GetMany(ctx context.Context, ids []uuid.UUID) ([]EntityRecord, error)
}
```

---

## LAW-010: Content Hash Is the Schema Identity

**Scope:** All code that identifies or compares schema versions

The [content hash](../GLOSSARY.md#content-hash-schema) of the [CompiledSchema](../GLOSSARY.md#compiledschema) is the only authoritative identifier for a specific schema version. No other identifier (binary build timestamp, Git commit SHA, deployment label) may be used as a proxy for schema identity.

Schema comparison MUST compare content hashes. Two schemas with identical content hashes are semantically equivalent. Two schemas with different content hashes are not semantically equivalent, regardless of what any other identifier says.

### Rationale

A Git commit SHA does not identify the schema if the binary was built from a dirty tree. A build timestamp does not identify the schema if the schema inputs were not changed but the binary was rebuilt. Only the hash of the actual schema content identifies the schema unambiguously.

Content hash comparison is the foundation of schema divergence detection ([LAW-019](#law-019)). Without a canonical schema identifier, divergence detection requires comparing the full compiled schema, which is expensive and complex.

---

## LAW-011: Entity Names Are Globally Unique and Immutable

**Scope:** All entity name declarations and all code that references entity names as strings

[Entity names](../GLOSSARY.md#entity-name) MUST be globally unique within a deployment. The compiler MUST detect duplicate entity names and fail compilation.

Entity names MUST NOT be renamed after they have been used in any non-DRAFT deployed version. Entity names are embedded in:
- Database migration filenames
- Temporal workflow IDs (stored for months in Temporal's event history)
- Redis cache keys
- Casbin policy tuples
- Audit log records

Renaming an entity name after deployment produces inconsistencies in all of these systems simultaneously. There is no safe migration path for renaming entity names; the name is a permanent identifier.

### Enforcement

The compiler detects duplicate names. A lint rule MUST detect entity name strings that do not match the `{module}_{noun}` format. No automated renaming tool may be provided that renames entity names (such a tool would produce a false sense of safety about an inherently unsafe operation).

---

## LAW-012: Migrations Are Append-Only

**Scope:** All files in any `migrations/` directory

Migration files MUST NOT be modified after they have been applied to any non-development environment. This includes: correcting a typo, adding a missing index, adjusting a column default, or adding a missing constraint.

Corrections to already-applied migrations MUST be expressed as new migrations: add the missing index in a new `.up.sql`, add the constraint in a new `.up.sql`, fix the typo by altering the column in a new `.up.sql`.

Migration files MUST be applied in version-timestamp order. The migration runner MUST reject out-of-order application.

### Rationale

A migration that is modified after being applied produces a divergence between the migration runner's record of what has been applied and the actual database state. The migration runner tracks which migrations have run by filename and checksum. Modifying a migration file changes its checksum, causing the runner to see it as unapplied — leading it to attempt to re-apply a migration that has already been applied, with potentially destructive results.

Beyond technical correctness, append-only migrations provide a complete, chronological audit trail of every schema change. This audit trail is a legal and compliance record for regulated ERP deployments.

---

## LAW-013: Sensitive Fields Are Never Logged

**Scope:** All logging code, error message construction, hook input preparation, and API response serialization

The value of any [Sensitive](../GLOSSARY.md#sensitive-field-constraint) field MUST NOT appear in:
- Structured log entries (slog, any logging framework)
- Error message strings returned to callers
- Hook input records passed to hooks
- API response payloads (unless the endpoint explicitly serves the field and the actor has explicit permission)
- Audit log entries (audit log records the field was changed, not the new value)

Logging code MUST redact sensitive fields before passing records to the logger. The framework's log middleware MUST automatically redact any [EntityRecord](../GLOSSARY.md#entityrecord) or [CreateInput](../GLOSSARY.md#createinput)/[UpdateInput](../GLOSSARY.md#updateinput) that is logged, substituting `[REDACTED]` for sensitive field values.

### Rationale

Sensitive fields include: passwords, session tokens, API keys, payment card data, personally identifiable information (PII), and other data whose exposure in logs would constitute a security or privacy breach.

Log aggregation systems (Elasticsearch, Splunk, CloudWatch Logs) often have broader access than production databases. Sensitive data in logs escapes the database's access controls and ends up in a less-controlled environment with longer retention periods.

---

## LAW-014: Filter Wire Format Is Versioned

**Scope:** All code that serializes or deserializes Filter expressions over network boundaries

Every serialized [Filter](../GLOSSARY.md#filter) expression transmitted over a network boundary MUST include an explicit `version` field. The runtime MUST reject any filter without a version field with HTTP 400.

The version field identifies the Filter DSL specification version used to construct the expression. It allows the runtime to apply the correct deserialization logic when the DSL evolves.

### Rationale

The Filter DSL is the framework's query language. Like all query languages, it evolves: new operators are added, operator semantics are refined, deprecated operators are removed. Without a version field in the wire format, the runtime cannot determine which version of the DSL a received filter uses. It cannot detect when a client is sending a filter using a DSL version that has since changed semantics.

A filter received without a version field is ambiguous. Ambiguous filters in a financial system may produce incorrect query results without producing an error. Rejecting unversioned filters forces clients to be explicit about the DSL version they are using.

---

## LAW-015: Database Enforces Tenant Isolation

**Scope:** All PostgreSQL table definitions for tenant-scoped data

Every table that stores tenant-scoped data MUST have:
1. `ENABLE ROW LEVEL SECURITY`
2. `FORCE ROW LEVEL SECURITY`
3. A `USING` policy that filters rows by `tenant_id = current_tenant_id()`

Application-layer filtering (WHERE clauses in queries, predicate injection in EntityRepository) is supplementary. It is not a substitute for database-level enforcement.

The database MUST reject rows that violate the RLS policy even if the application layer fails to apply the correct tenant filter.

### Rationale

Application-layer tenant filtering depends on every query correctly including the tenant predicate. A single missed predicate in any query path is a tenant isolation breach. In a large codebase, guaranteeing that every query always includes the predicate is not achievable through code review alone.

Database-level RLS is enforced by the PostgreSQL engine regardless of what the application sends. It cannot be bypassed by missing a WHERE clause, by using the wrong connection, or by a code path that was written before tenant isolation was a requirement. It is structurally enforced.

`FORCE ROW LEVEL SECURITY` is required because without it, the table owner role (which application code may use) bypasses the policy. `FORCE` applies the policy to the owner as well, eliminating this bypass.

---

## LAW-016: Workflow ID Format Is Canonical

**Scope:** All code that constructs Temporal workflow IDs

[Workflow IDs](../GLOSSARY.md#workflow-id) MUST follow the format:

```
{tenant-uuid}.{entity-type}.{record-id}.{event}.{workflow-function}
```

Example: `a1b2c3d4-e5f6-7890-abcd-ef1234567890.finance_invoice.inv-2024-00042.on_submit.InvoiceSubmissionWorkflow`

No workflow ID may deviate from this format. Workflow IDs are stored in Temporal's event history (months retention), in the entity record, and in audit logs. A consistent format allows workflow IDs to be parsed to retrieve the entity type and record ID without additional database lookups.

### Rationale

Temporal workflow IDs are the durable correlation key between entity records and their associated long-running processes. Support engineers, auditors, and debugging tools use workflow IDs to trace the history of a record through workflow executions. A consistent, parseable format makes this possible without bespoke tooling per entity type.

---

## LAW-017: Cross-Module Hook Registration Requires Open Policy

**Scope:** All code that registers a HookRegistration for an entity not owned by the registering module

A module MUST NOT register a [HookRegistration](../GLOSSARY.md#hookregistration) on an entity it does not own unless the owning entity's [EntityDefinition](../GLOSSARY.md#entitydefinition) declares `HookPolicy: Open` (or the equivalent field name).

An entity with `HookPolicy: Closed` (the default) accepts hook registrations only from its owning module.

The compiler MUST validate hook registrations against the target entity's HookPolicy and fail compilation if a cross-module registration targets a closed entity.

### Rationale

An entity that allows arbitrary cross-module hooks cannot guarantee its lifecycle semantics. A hook registered by module B on an entity owned by module A can:
- Abort mutations that module A expects to succeed
- Mutate the record in ways module A does not expect
- Introduce performance degradation that module A cannot control
- Become a covert coupling between modules that should be independent

Hook sovereignty — the owning module controls who can attach to its lifecycle — is the mechanism by which module boundaries remain meaningful over time. Without it, modules become entangled through shared hook registrations in ways that are not visible in the module manifests.

---

## LAW-018: Outbox Table Is Framework-Private

**Scope:** All module code, application code, and migration files authored by module authors

No code outside the framework's outbox relay implementation may read from or write to the outbox table. No module migration may add columns to the outbox table or add foreign key constraints from other tables to the outbox table.

The outbox table schema is owned by the framework and may change between framework versions. Module code that depends on the outbox table schema will break silently when the schema changes.

### Rationale

The outbox relay is framework infrastructure. Its internal schema is an implementation detail. Exposing it as a stable API would require versioning the outbox table schema as a public contract, preventing the framework from optimizing or restructuring the outbox relay implementation.

Module authors who need to know whether a workflow has been dispatched should query the entity record's workflow ID field (set before the transaction commits) rather than querying the outbox table.

---

## LAW-019: Schema Is Identical Across All Instances

**Scope:** All deployments of the same binary

All instances of the same Awo binary, started with the same configuration, MUST produce a [CompiledSchema](../GLOSSARY.md#compiledschema) with the same [content hash](../GLOSSARY.md#content-hash-schema).

A cluster state where two or more instances of the same binary produce different content hashes is a [schema divergence](../GLOSSARY.md#schema-divergence) defect. Schema divergence MUST be detected and alerted on. Diverged instances MUST be removed from the load balancer rotation until the divergence is resolved.

### Rationale

A load balancer distributing traffic across instances with different schemas produces requests that are routed to different route tables, different permission policies, and different page schemas depending on which instance handles the request. The resulting behavior is non-deterministic and non-debuggable: the same request produces different results on different invocations.

Schema divergence is not a theoretical concern. It can occur when:
- A module's `init()` function produces non-deterministic registration order (e.g., depending on map iteration order)
- A module reads environment-specific configuration during registration (violating LAW-003's spirit)
- An instance is running a different version of the binary than its peers due to a partial rollout

Detecting divergence through content hash comparison requires only two pieces of information: the hash from this instance and the hash from peers. This is far cheaper than comparing full schemas.

---

## LAW-020: All Public APIs Carry Stability Annotations

**Scope:** All exported symbols in `awo/def/`, `awo/filter/`, `awo/driver/`

Every exported type, function, method, and constant in `def/`, `filter/`, and `driver/` MUST carry a godoc comment that includes a stability annotation in the format:

```
// Stability: FROZEN | STABLE | EVOLVING | EXPERIMENTAL
```

The stability annotation MUST be the last line of the godoc comment, immediately preceding the symbol declaration.

Symbols without a stability annotation MUST be treated as EXPERIMENTAL by callers. The CI MUST warn (and eventually fail) on missing stability annotations for exported symbols in the specified packages.

### Rationale

Module authors and driver implementors make long-term commitments when they use framework APIs. A module author who uses a type that is later removed or restructured must update all code that depends on it. A driver implementor who implements an interface that gains a new required method must update their implementation.

Stability annotations allow callers to make informed decisions. A module author who uses an EXPERIMENTAL type knowingly accepts the risk of breakage. A module author who uses a FROZEN type has a contractual guarantee that the type will not change in incompatible ways within the major version.

Without stability annotations, all APIs are implicitly STABLE, creating implicit stability guarantees for APIs that the framework does not intend to stabilize.

---

## Summary Table

| Law | Domain | Breaking Consequence of Violation |
|---|---|---|
| LAW-001 | Package structure | Circular imports, testing friction, binary bloat |
| LAW-002 | Runtime | Divergence between compiled and runtime schema |
| LAW-003 | Registry | Inconsistent route/policy/schema state |
| LAW-004 | Schema | Data races, hash invalidation |
| LAW-005 | Tenancy | Cross-tenant data access or silent empty results |
| LAW-006 | Workflows | Lost workflow starts on crash |
| LAW-007 | Hooks | Non-deterministic business logic |
| LAW-008 | Layers | Circular deps, untestable domain code |
| LAW-009 | Drivers | Breaking all driver implementations |
| LAW-010 | Schema identity | Incorrect divergence detection |
| LAW-011 | Entity names | Broken migrations, workflow history, audit logs |
| LAW-012 | Migrations | Re-applied migrations, data corruption |
| LAW-013 | Security | Sensitive data in logs, compliance violations |
| LAW-014 | Filter DSL | Ambiguous query semantics, incorrect results |
| LAW-015 | Tenancy | Tenant isolation breach |
| LAW-016 | Workflows | Unparseable workflow IDs, broken tooling |
| LAW-017 | Modules | Covert inter-module coupling |
| LAW-018 | Outbox | Broken module code on framework upgrade |
| LAW-019 | Deployment | Non-deterministic request behavior |
| LAW-020 | API contracts | Undisclosed stability commitments |

---

## Related Documents

- [Architecture Invariants](invariants.md) — runtime properties corresponding to these laws
- [Five-Layer Architecture](five-layer.md) — detailed specification of the layer rules in LAW-008
- [Compilation Pipeline](../03-kernel/compilation-pipeline.md) — implementation of LAW-002, LAW-003, LAW-004, LAW-010, LAW-019
- [Tenancy Model](../06-tenancy/tenant-model.md) — implementation of LAW-005, LAW-015
- [Outbox Pattern](../09-workflow/outbox-pattern.md) — implementation of LAW-006, LAW-018
- [Hook System](../04-domain/hooks.md) — implementation of LAW-007, LAW-017
- [Filter DSL](../05-persistence/filter-dsl.md) — implementation of LAW-014
- [Migrations](../14-operations/migrations.md) — implementation of LAW-012
- [Glossary](../GLOSSARY.md) — canonical definitions for all terms
