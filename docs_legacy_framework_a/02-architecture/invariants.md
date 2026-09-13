> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Architecture Invariants"
id: arch-002
status: accepted
category: LAW
stability: FROZEN
audience: [framework-authors, contributors, core-team]
since: "1.0"
normative-level: normative
related:
  - "[Architecture Laws](laws.md)"
  - "[Five-Layer Architecture](five-layer.md)"
  - "[Tenancy Model](../06-tenancy/tenant-model.md)"
  - "[Compilation Pipeline](../03-kernel/compilation-pipeline.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Architecture Invariants

**ARCH-002 | Status: Accepted | Stability: Frozen**

This document states twelve Architecture Invariants. An Architectural Invariant is a property of the runtime system that must hold unconditionally in every deployment, every tenant context, every request, and across all versions within the major version.

Architecture Invariants differ from [Architecture Laws](laws.md) in scope: laws govern authoring behavior (what contributors must do when writing code), invariants govern runtime behavior (what the running system must uphold). A law violation is a code defect. An invariant violation is a runtime defect — evidence that the system is in an incorrect state.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, SHOULD NOT, RECOMMENDED, MAY, and OPTIONAL in this document are to be interpreted as described in [RFC 2119](https://www.rfc-editor.org/rfc/rfc2119).

Invariant violations MUST be treated as severity-1 incidents. A system where an invariant has been violated is in an undefined state and must be treated as untrusted until the invariant is restored.

---

## Table of Contents

- [INV-001: All Tenant-Scoped Queries Execute Under RLS](#inv-001-all-tenant-scoped-queries-execute-under-rls)
- [INV-002: The CompiledSchema Is Fixed Before the First Request](#inv-002-the-compiledschema-is-fixed-before-the-first-request)
- [INV-003: Hook Execution Order Is Consistent Within a Process Lifetime](#inv-003-hook-execution-order-is-consistent-within-a-process-lifetime)
- [INV-004: Every Committed Entity Mutation Has an Audit Log Entry](#inv-004-every-committed-entity-mutation-has-an-audit-log-entry)
- [INV-005: Every Committed Entity Mutation Has a Corresponding Outbox Entry](#inv-005-every-committed-entity-mutation-has-a-corresponding-outbox-entry)
- [INV-006: Sensitive Field Values Are Absent From All Log and Error Payloads](#inv-006-sensitive-field-values-are-absent-from-all-log-and-error-payloads)
- [INV-007: Session Validation Always Requires a Live Redis Connection](#inv-007-session-validation-always-requires-a-live-redis-connection)
- [INV-008: All Monetary Arithmetic Uses Exact Decimal Representation](#inv-008-all-monetary-arithmetic-uses-exact-decimal-representation)
- [INV-009: The Content Hash Is Identical Across All Running Instances](#inv-009-the-content-hash-is-identical-across-all-running-instances)
- [INV-010: No Stack Trace Appears in a Client-Facing Error Response](#inv-010-no-stack-trace-appears-in-a-client-facing-error-response)
- [INV-011: The Middleware Pipeline Executes in Full for Every Request](#inv-011-the-middleware-pipeline-executes-in-full-for-every-request)
- [INV-012: Entity Names Are Stable After the First Migration](#inv-012-entity-names-are-stable-after-the-first-migration)

---

## INV-001: All Tenant-Scoped Queries Execute Under RLS

**Statement:** Every SQL query issued against a tenant-scoped table MUST execute within a database session where `app.current_tenant_id` has been set by a successful `set_tenant_context()` call.

**Observable consequence of violation:** A query that executes without `app.current_tenant_id` set returns zero rows (because `current_tenant_id()` returns NULL and the USING clause matches nothing) or — if `FORCE ROW LEVEL SECURITY` was not applied — returns rows from all tenants.

**Detection:** Integration tests MUST verify that queries issued without `set_tenant_context()` return zero rows on tenant-scoped tables. Alerting MUST detect queries to tenant-scoped tables from sessions where `app.current_tenant_id` is unset.

**Governing law:** [LAW-005](laws.md#law-005-no-store-operation-without-tenant-context), [LAW-015](laws.md#law-015-database-enforces-tenant-isolation)

---

## INV-002: The CompiledSchema Is Fixed Before the First Request

**Statement:** The [CompiledSchema](../GLOSSARY.md#compiledschema) MUST be produced and sealed before the Fiber HTTP server begins accepting connections. No request may be handled by a route derived from an incomplete or unsealed schema.

**Observable consequence of violation:** A request arrives before compilation is complete and is routed by a partial route table, producing 404 responses for valid entities or incorrect routing for entities registered after the partial table was built.

**Detection:** The startup sequence enforces this by ordering `EntityRegistry.Compile()` before `fiber.Listen()`. The health readiness endpoint (`/health/ready`) MUST return 503 until compilation is complete and the server is listening. Load balancers MUST check readiness before routing traffic.

**Governing law:** [LAW-003](laws.md#law-003-registry-is-closed-after-compilation), [LAW-004](laws.md#law-004-compiledschema-is-immutable)

---

## INV-003: Hook Execution Order Is Consistent Within a Process Lifetime

**Statement:** For a given entity type and lifecycle stage, the sequence of hooks that executes MUST be identical for every invocation within the same process lifetime. The sequence produced in the first invocation MUST be the sequence produced in the millionth invocation.

**Observable consequence of violation:** Two invocations of the same operation on the same entity type produce different hook sequences, making business logic non-deterministic. This produces audit logs that record different sequences of operations for identical inputs, making debugging and compliance verification impossible.

**Detection:** The hook dispatcher MUST be stateless with respect to hook sequence (the sequence is determined at compilation, not at dispatch time). A test MUST invoke an operation with hooks ten thousand times and verify that the observed sequence is always identical.

**Governing law:** [LAW-007](laws.md#law-007-hook-execution-order-is-deterministic)

---

## INV-004: Every Committed Entity Mutation Has an Audit Log Entry

**Statement:** Every successful `Create`, `Update`, `Delete`, `BulkCreate`, and `BulkUpdate` operation that commits to the database MUST produce one or more [Audit Log](../GLOSSARY.md#audit-log) entries in the same transaction.

A committed entity mutation without a corresponding audit log entry is an invariant violation. A transaction that attempts to commit entity mutations without audit log writes MUST be rejected.

**Observable consequence of violation:** The audit log has gaps. An auditor or regulator who queries the audit log for a record finds mutations that are not reflected in the audit history. In regulated environments, an audit log gap is a compliance failure.

**Detection:** The store layer MUST write audit log entries as part of the entity write, not as a post-commit side effect. An integration test MUST verify that every entity write produces the correct audit log entries by counting audit log rows before and after the operation.

**Governing law:** Derived from [Design Goal G-3](../01-introduction/design-goals.md#g-3-financial-and-inventory-integrity) and [Constraint C-3](../01-introduction/design-goals.md#c-3-audit-trail-is-tamper-evident).

---

## INV-005: Every Committed Entity Mutation Has a Corresponding Outbox Entry

**Statement:** For every `Create`, `Update`, or `Delete` operation that commits to the database and has one or more associated [WorkflowTrigger](../GLOSSARY.md#workflowtrigger) declarations, an outbox entry MUST be written in the same transaction.

A committed entity mutation that should trigger a workflow, but has no outbox entry, will never start the associated workflow. This is a silent data loss event.

**Observable consequence of violation:** A submitted invoice (for example) has no associated workflow start. The invoice is stuck in "Submitted" status indefinitely, with no automation advancing it. The failure is silent — no error is returned to the caller.

**Detection:** After every operation that should produce outbox entries, an integration test MUST verify that the outbox table contains the expected entries. A monitoring alert MUST fire if outbox entries exist for more than a configurable maximum time without being dispatched.

**Governing law:** [LAW-006](laws.md#law-006-outbox-entry-and-entity-record-commit-atomically)

---

## INV-006: Sensitive Field Values Are Absent From All Log and Error Payloads

**Statement:** The string representation of any [Sensitive](../GLOSSARY.md#sensitive-field-constraint) field value MUST NOT appear in any structured log entry, any error message, any HTTP response body (including error responses), or any distributed trace span that is exported outside the process.

This invariant applies to: passwords, session tokens, API keys, HMAC secrets, payment card data, national identification numbers, and any field declared `Sensitive: true` in an EntityDefinition.

**Observable consequence of violation:** Sensitive values appear in log aggregation systems (Elasticsearch, Splunk, CloudWatch), which have broader access than production databases, longer retention periods, and are often accessible to teams who should not see the underlying values.

**Detection:** Log output MUST be scanned in CI for patterns that match known sensitive field names. A test MUST verify that creating an entity with sensitive fields does not produce any log entry containing the field value. The framework's log middleware MUST apply automatic redaction before emitting log entries.

**Governing law:** [LAW-013](laws.md#law-013-sensitive-fields-are-never-logged)

---

## INV-007: Session Validation Always Requires a Live Redis Connection

**Statement:** The session validation step of the [Middleware Pipeline](../GLOSSARY.md#middleware-pipeline) MUST NOT succeed if the Redis connection is unavailable. There MUST be no fallback authentication mechanism that bypasses session validation when Redis is down.

When Redis is unavailable, all authenticated requests MUST fail with HTTP 503 (Service Unavailable). Unauthenticated requests to public endpoints (health checks, documentation endpoints) MAY succeed.

**Observable consequence of violation:** If session validation has a "Redis is down, allow the request" fallback, then a Redis outage grants unauthenticated access to authenticated endpoints. This is a security invariant, not merely an availability concern.

**Detection:** An integration test MUST simulate Redis unavailability and verify that all requests to authenticated endpoints return HTTP 503. Monitoring MUST alert on Redis connectivity failures because they degrade the availability of all authentication.

**Governing law:** Derived from [Design Goal G-1](../01-introduction/design-goals.md#g-1-multi-tenant-erp-operation-at-scale) and [Constraint C-4](../01-introduction/design-goals.md#c-4-security-controls-must-not-be-bypassable-from-application-code).

---

## INV-008: All Monetary Arithmetic Uses Exact Decimal Representation

**Statement:** Every arithmetic operation involving a monetary value MUST use `decimal.Decimal` (the `shopspring/decimal` library or equivalent exact-decimal implementation). The `float32`, `float64`, and any floating-point intermediate representation MUST NOT be used in any computation that produces or consumes a monetary value.

This invariant applies throughout the stack: database storage (`numeric(20,4)`), Go representation (`decimal.Decimal`), serialization (decimal string), deserialization (parse to `decimal.Decimal`), arithmetic (all operations through the decimal library), and audit log recording (exact decimal string).

**Observable consequence of violation:** A monetary calculation that passes through a floating-point representation accumulates rounding errors. For a single invoice, the error may be sub-cent and unnoticeable. For ten thousand invoices processed over three years and summed into financial statements, the accumulated error can be material — producing financial statements that are incorrect by amounts large enough to trigger regulatory scrutiny.

**Detection:** Static analysis MUST flag any code in the domain and store layers that performs arithmetic on `float64` values that are traceable to entity fields of type `Currency`. No `float64` conversion of a `decimal.Decimal` may appear in the domain or store layers.

**Governing law:** Derived from [Design Goal G-3](../01-introduction/design-goals.md#g-3-financial-and-inventory-integrity) and [Constraint C-2](../01-introduction/design-goals.md#c-2-financial-calculations-must-be-exact).

---

## INV-009: The Content Hash Is Identical Across All Running Instances

**Statement:** All instances of the same Awo binary, started with the same configuration, MUST emit the same [content hash](../GLOSSARY.md#content-hash-schema) from `CompiledSchema.ContentHash()`.

If two running instances produce different content hashes, [schema divergence](../GLOSSARY.md#schema-divergence) has occurred. The diverged state MUST be detected and alerted on within the health check interval.

**Observable consequence of violation:** A load balancer distributing requests across diverged instances produces requests where identical inputs produce different behavior depending on which instance handles the request. Debugging this class of bug is extremely difficult because the behavior appears non-deterministic from the client's perspective.

**Detection:** The `/health/ready` endpoint MUST include the content hash in its response. An external health checker MUST compare content hashes across all instances and alert if they differ. Instances with a divergent content hash MUST be removed from rotation.

**Governing law:** [LAW-010](laws.md#law-010-content-hash-is-the-schema-identity), [LAW-019](laws.md#law-019-schema-is-identical-across-all-instances)

---

## INV-010: No Stack Trace Appears in a Client-Facing Error Response

**Statement:** The HTTP response body for any error response MUST NOT include a Go stack trace, internal package paths, internal function names, SQL query text, database error codes not intended for clients, or any other internal implementation detail.

All error responses MUST conform to the [Response Envelope](../GLOSSARY.md#response-envelope) error format: `{"error": {"code": "...", "message": "..."}}`. The `code` field MUST be a stable, documented error code. The `message` field MUST be a user-facing string safe for display in UI.

**Observable consequence of violation:** Stack traces in error responses expose: package paths (revealing internal code structure), SQL queries (revealing schema and potentially data), function names (useful for reverse engineering exploits), and Go runtime internals (revealing implementation details that could be targeted by attackers).

**Detection:** An integration test MUST verify that error responses for all error types (validation error, business error, not-found, permission error, internal error) conform to the envelope format and contain no internal details. A response body scanner in CI MUST search for patterns matching Go stack trace format.

**Governing law:** Derived from [Constraint C-4](../01-introduction/design-goals.md#c-4-security-controls-must-not-be-bypassable-from-application-code) and [Design Goal G-5](../01-introduction/design-goals.md#g-5-idiomatic-go-with-no-hidden-magic).

---

## INV-011: The Middleware Pipeline Executes in Full for Every Request

**Statement:** Every inbound HTTP request MUST pass through all eight stages of the [Middleware Pipeline](../GLOSSARY.md#middleware-pipeline) in order: Request ID → Logging → Panic recovery → CORS → Tenant resolution → `set_tenant_context()` → Session validation → Rate limiting.

No route, no query parameter, no header, and no configuration option may cause a request to skip any stage. The pipeline order is immutable.

**Observable consequence of violation:** A request that skips tenant resolution reaches the route handler without a [TenantContext](../GLOSSARY.md#tenantcontext), causing either a panic (nil context dereference) or a query against an unscoped table. A request that skips session validation reaches the route handler without a verified [Actor](../GLOSSARY.md#actor), bypassing authentication. A request that skips rate limiting is not subject to DoS protection.

**Detection:** An integration test MUST verify that requests with deliberately invalid inputs at each pipeline stage are rejected at that stage with the expected HTTP status code, proving that the stage executed.

**Governing law:** Derived from [Architecture Law LAW-005](laws.md#law-005-no-store-operation-without-tenant-context) and [Constraint C-4](../01-introduction/design-goals.md#c-4-security-controls-must-not-be-bypassable-from-application-code).

---

## INV-012: Entity Names Are Stable After the First Migration

**Statement:** An [entity name](../GLOSSARY.md#entity-name) that has been referenced in a migration file that has been applied to any non-development environment MUST NOT be changed.

This invariant is permanent: it applies for the entire operational lifetime of the data created with that entity name. There is no migration path that makes renaming an entity name safe.

**Observable consequence of violation:** An entity name rename after deployment produces inconsistencies in: migration filenames (historical migrations reference the old name, new migrations reference the new name), Temporal workflow IDs stored in event history (months of history reference the old name), Redis cache keys (invalidation must handle both names), Casbin policy tuples (old policies reference the old name, new policies reference the new name), and audit log records (historical records reference the old name).

There is no automated tool that can atomically rename an entity across all of these systems simultaneously. The result of renaming is a partially-migrated state that is inconsistent by def.

**Detection:** The compiler detects entity name format violations. Migration file analysis tools MUST detect entity names referenced in historical migrations that are not present in the current schema. This situation — an entity name in a historical migration absent from the current schema — is either a deletion (permitted, documented) or a rename (invariant violation).

**Governing law:** [LAW-011](laws.md#law-011-entity-names-are-globally-unique-and-immutable)

---

## Invariant Monitoring

| Invariant | Primary Detection | Alert on Violation |
|---|---|---|
| INV-001 | Integration test: query without context returns 0 rows | Zero-row anomaly on non-empty tables |
| INV-002 | Health readiness check before first traffic | 503 from readiness endpoint during startup |
| INV-003 | Unit test: 10k invocations produce identical hook sequence | N/A (detectable only in testing) |
| INV-004 | Integration test: audit row count before/after operation | Audit log row count not increasing on writes |
| INV-005 | Integration test: outbox count after triggered operation | Outbox entries older than max dispatch time |
| INV-006 | CI log scan for sensitive field patterns | Sensitive pattern detected in log stream |
| INV-007 | Integration test: Redis down → HTTP 503 | Redis connectivity loss alert |
| INV-008 | Static analysis: float64 in monetary code | N/A (detectable only in static analysis) |
| INV-009 | Health check: content hash comparison across instances | Content hash mismatch across instances |
| INV-010 | Integration test: error response body format | Stack trace pattern in HTTP response body |
| INV-011 | Integration test: each stage rejects invalid input correctly | N/A (detectable only in testing) |
| INV-012 | Migration analysis: names in old migrations absent from schema | Entity name in migration not in compiled schema |

---

## Related Documents

- [Architecture Laws](laws.md) — authoring rules that, when followed, preserve these invariants
- [Five-Layer Architecture](five-layer.md) — layer structure that enables INV-003, INV-006, INV-010
- [Compilation Pipeline](../03-kernel/compilation-pipeline.md) — process that establishes INV-002, INV-009
- [Tenancy Model](../06-tenancy/tenant-model.md) — mechanism that enforces INV-001
- [Outbox Pattern](../09-workflow/outbox-pattern.md) — mechanism that enforces INV-005
- [Observability](../13-observability/observability.md) — monitoring setup for all invariant alerts
- [Glossary](../GLOSSARY.md) — canonical definitions for all terms
