# Awo Framework — Design Principles

**Classification:** Specification — Tier 0
**Owner:** `00-overview/PRINCIPLES.md`
**Status:** Frozen at v1.0

---

## Purpose

This document records the design principles that govern every architectural decision in the Awo Framework. Principles are not guidelines. They are binding constraints on all framework evolution. When a new decision conflicts with a principle, the principle wins unless a new principle explicitly supersedes the old one via the ADR process.

---

## Principle 1 — Declaration Over Implementation

**Statement:** Module authors declare what they want. The framework decides how to implement it.

**Rationale:** An ERP framework serves dozens of modules and hundreds of entity types. If every module author must implement persistence, routing, authorization, and UI separately, the result is divergence, bugs, and maintenance burden. When the framework controls implementation, all modules automatically inherit framework improvements.

**Manifestation:**
- `EntityDefinition` is a pure declaration. No implementation code.
- `PermissionSet` declares role subjects. The framework enforces them.
- `WorkflowTrigger` declares the event binding. The framework starts the workflow.
- `FieldDef` declares a field constraint. The framework generates the SQL CHECK and the runtime validator.

**Comparison:** Frappe (ERPNext) and Django share this principle. The Awo difference is that declarations are Go structs — not Python class attributes, not YAML — giving compile-time validation.

---

## Principle 2 — Fail Fast, Fail Loud

**Statement:** Errors at startup are preferable to errors at runtime. Errors at runtime are preferable to silent data corruption.

**Rationale:** Silent failures are the most dangerous class of failure in financial software. A ledger entry that was never written is worse than a crash, because the crash is visible and recoverable.

**Manifestation:**
- `EntityRegistry.Build()` panics on invalid definitions — framework authors cannot serve requests with a malformed schema.
- `auth.ViewerFromContext()` panics if ViewerContext is absent — the middleware guarantee cannot be silently bypassed.
- Hook recursion panics — recursive mutations are detected at development time, not in production.
- Missing required env vars cause fatal startup — the process cannot run in an undefined configuration.
- PostgreSQL unreachable at startup causes fatal exit — not a retry loop.

**Boundary:** Runtime panics are caught by the middleware panic recovery and converted to HTTP 500 responses. Only panics that indicate programming errors (violated invariants) are allowed to propagate far enough to be visible; all anticipated error conditions return structured errors.

---

## Principle 3 — Isolation Is Structural, Not Conventional

**Statement:** Tenant isolation MUST be enforced by the database, not by application code conventions.

**Rationale:** Application-layer tenant filtering (`WHERE tenant_id = ?`) depends on developers never forgetting a WHERE clause. In a codebase with many contributors and many queries, this is not a reliable guarantee. One missed filter exposes all tenant data.

**Manifestation:**
- PostgreSQL `FORCE ROW LEVEL SECURITY` on every tenant-scoped table — queries without a tenant context return zero rows or an error.
- `set_tenant_context()` stored procedure — not raw `SET LOCAL`. The procedure validates tenant existence and active status before setting the GUC.
- PgBouncer in transaction mode — ensures the GUC is reset on transaction commit/rollback.
- No application code with `WHERE tenant_id = $1` — the database enforces this.

**Boundary:** Global tables (tenants, audit_log, etc.) are legitimately exempt from RLS because they exist outside the tenant boundary by design.

---

## Principle 4 — The Pipeline Is the Contract

**Statement:** Every entity mutation follows the same ordered pipeline. No shortcut paths exist.

**Rationale:** In a multi-module ERP system, multiple concerns need to act on every mutation: validation, authorization, naming series allocation, audit logging, event publishing, workflow triggering. If every module implements its own mutation flow, concerns are missed inconsistently. The pipeline guarantees that all concerns execute in the correct order for every mutation.

**Manifestation:**
- The lifecycle stages (ASSEMBLE → VALIDATE → AUTHORIZE → ... → AUDIT → after_save → Commit → Workflow) are not configurable per entity.
- Audit is a mandatory pipeline stage — it cannot be disabled by individual hooks.
- The TX boundary is fixed: begins before PERSIST, commits after after_save.
- Workflow start is always outside the TX — preventing phantom workflow starts on rollback.

**Boundary:** The `AuditEnabled: false` field on `EntityDefinition` allows high-volume non-sensitive entities to skip audit. This is the only configurable skip in the pipeline.

---

## Principle 5 — One Concept, One Owner

**Statement:** Every architectural concept has exactly one canonical document that defines it. All other documents reference that document.

**Rationale:** Duplicated documentation is worse than no documentation. Duplication produces contradictions. Contradictions produce incorrect implementations.

**Manifestation:**
- `GLOSSARY.md` defines every term. Other documents use the term without redefining it.
- `ENTITY_DEFINITION_SPEC.md` defines the EntityDefinition contract. Action guides reference it, not repeat it.
- `RLS_SPEC.md` defines the RLS model. The security guide references it, not repeat it.
- The 50-file limit on `awo/docs/` enforces this discipline at the filesystem level.

---

## Principle 6 — Public Interfaces Are Promises

**Statement:** Any interface that module authors implement or call is a promise that cannot be broken after v1.0 without a major version increment.

**Rationale:** The Awo Framework is used by module authors who write code against its interfaces. Breaking those interfaces breaks their code. In an ERP context, module authors may not be the same team as the framework authors.

**Manifestation:**
- `def.EntityDefinition`, `def.HookSet`, hook interfaces, `def.ActionRuntime`, `def.ActionEntityRepo`, `filter.*` constructors, and `auth.ViewerContext` are frozen at v1.0.
- `CompiledSchema` is intentionally private — it is a framework implementation detail, not a module author API.
- ADR-011 renamed `CasbinPolicies` to `CapabilityGrants` before v1.0 precisely because the old name was an implementation detail leaking into the public contract.

---

## Principle 7 — Money Is Never a Float

**Statement:** All monetary values MUST use `shopspring/decimal` in Go and `numeric(20,4)` in PostgreSQL.

**Rationale:** IEEE 754 floating-point arithmetic produces rounding errors that are unacceptable in financial calculations. `0.1 + 0.2 != 0.3` in float64. A tax calculation with a float64 error may result in incorrect amounts filed with KRA eTIMS.

**Manifestation:**
- `FieldTypeCurrency` maps to `numeric(20,4)` in PostgreSQL and `decimal.Decimal` in Go.
- The CLAUDE.md CRITICAL RULES list this as a non-negotiable constraint.
- The framework runtime refuses to store a monetary field as any other type.

**Boundary:** `FieldTypeFloat` (`double precision`) is available for scientific measurements, percentages, and non-financial quantities. It MUST NOT be used for monetary values.

---

## Principle 8 — I/O Belongs in Activities

**Statement:** Temporal workflow functions MUST NOT perform I/O. All I/O goes in Temporal activities.

**Rationale:** Temporal guarantees durability by replaying workflow history. Replay only works if workflow code is deterministic — same inputs produce same decisions. I/O (HTTP calls, database queries, time reads) is not deterministic across replays.

**Manifestation:**
- No `time.Now()` in workflow code — use `workflow.Now(ctx)`.
- No `time.Sleep()` in workflow code — use `workflow.Sleep(ctx, d)`.
- No database access in workflow code — database operations go in activities.
- No external API calls in workflow code — API calls go in activities.
- The CLAUDE.md CRITICAL RULES list this explicitly.

---

## Principle 9 — Security Is Structural, Not Additive

**Statement:** Security controls MUST be baked into the framework architecture, not bolted on by individual module authors.

**Rationale:** Security controls that rely on module authors remembering to add them will eventually be omitted. A missed authorization check in one entity is a privilege escalation vulnerability for all tenants.

**Manifestation:**
- Authorization is enforced by the pipeline AUTHORIZE stage — not by individual route handlers.
- RLS enforces tenant isolation at the database — not by application code.
- Sensitive fields are excluded from logs structurally — not by developer discipline.
- Session validation is middleware — not a per-handler concern.
- Stack traces are never sent to clients — the error envelope enforces this.

---

## Principle 10 — No Premature Abstraction

**Statement:** Add abstraction only when three or more concrete cases exist and a pattern is clear.

**Rationale:** Premature abstraction produces interfaces that do not fit the actual use cases and frameworks that are harder to understand than the problems they solve.

**Manifestation:**
- Three similar lines of code are preferable to one premature helper.
- No error handling for impossible scenarios — trust internal framework guarantees.
- No feature flags for hypothetical future requirements.
- No backwards-compatibility shims for features that have never shipped.

**Boundary:** Public interfaces that module authors implement (hooks, PolicyEvaluator, EventBroker) are designed with extensibility in mind — but only the extensibility actually needed.

---

## References

- [`00-overview/DECISION_REGISTER.md`](DECISION_REGISTER.md) — Specific architectural decisions
- [`00-overview/ARCH_OVERVIEW.md`](ARCH_OVERVIEW.md) — How principles manifest in structure
- [`CLAUDE.md`](../../CLAUDE.md) — Operational rules derived from these principles
