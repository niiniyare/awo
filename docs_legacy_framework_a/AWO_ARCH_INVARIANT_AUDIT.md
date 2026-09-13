> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

# Awo Framework — Architectural Invariant and Correctness Audit

**Classification:** Architecture Verification (Post-Implementation)
**Date:** 2026-07-20
**Scope:** Correctness, invariants, evolution, and risk — implementation gaps excluded
**Benchmark:** Kubernetes, PostgreSQL, Temporal, Zanzibar, Frappe, Odoo

---

## Preface

This report assumes the implementation audit corrections are complete. Every recommendation from `AWO_AUTH_IMPLEMENTATION_AUDIT.md` is treated as shipped. The question asked here is different: **given a correct implementation, does the architecture itself remain sound under 20 years of operational stress?**

The honest answer is: mostly yes, with seven structural flaws that are not correctable after v1.0 without breaking changes, and fourteen risks that are manageable with early mitigation.

---

## 1. Architectural Invariants

### Category A — Authorization Invariants

**A1: Every data access is mediated by a ViewerContext**

*Why it exists:* Authorization cannot be enforced if any path bypasses the authorization context. This is the Complete Mediation requirement from security principles.

*What breaks:* A single unmediated path allows privilege escalation. If an activity in a Temporal workflow calls a repository without a ViewerContext, it can read any tenant's data.

*How enforced:* ViewerContext must be a required parameter on every Repository method signature — not optional, not from context. Go's type system should make it impossible to call `repo.Get()` without supplying a ViewerContext.

*Enforcement belongs in:* Repository interface (compile-time) + middleware (runtime).

*Current status:* `ActionRuntime.Repo()` scopes the repository per entity, but ViewerContext is not yet a first-class parameter in the EntityRepository interface. This is a **structural gap** that becomes unretractable once the EntityRepository interface is frozen in v1.0.

---

**A2: Tenant isolation is absolute — tenant A cannot read, write, or infer tenant B's data**

*Why it exists:* The multi-tenancy contract. Violation is a catastrophic security breach and a regulatory failure.

*What breaks:* GDPR, HIPAA, PCI compliance. Customer trust collapses.

*How enforced:* Three independent layers must agree:
1. PostgreSQL RLS via `FORCE ROW LEVEL SECURITY` + `set_tenant_context()`
2. Application-level TenantID on every query context
3. Redis key namespacing by `{tenant_id}:`

All three are AND-ed. If any single layer is bypassed, isolation fails. The architecture currently lacks a mechanism to detect if layer 2 or 3 is accidentally skipped.

*Enforcement belongs in:* Database (primary), middleware (defense in depth), Redis key convention (secondary defense).

---

**A3: Explicit deny unconditionally overrides any grant**

*Why it exists:* Without this, a poorly scoped grant can override a carefully constructed deny. The invariant makes authorization reasoning tractable.

*What breaks:* Security. The evaluation order must be deterministic and documented as part of the PolicyEvaluator interface contract — not left to implementation.

*Enforcement belongs in:* PolicyEvaluator interface contract (documentation + test suite).

---

**A4: RecordFilter composes with RLS via AND — never via OR**

*Why it exists:* RLS establishes the tenant floor. RecordFilter establishes a within-tenant ceiling. OR-ing them would allow cross-tenant access.

*What breaks:* Tenant isolation. If PolicyFunc returns a filter that includes records from another tenant's data, and RLS is OR-ed, those records become visible.

*How enforced:* Repository applies `WHERE (rls_predicate) AND (record_filter_predicate)`. PostgreSQL RLS handles the first clause automatically. The application adds RecordFilter as additional WHERE — never as subquery OR.

*Enforcement belongs in:* Repository implementation (must be audited).

---

**A5: Authorization decisions are monotonically consistent within a request**

*Why it exists:* If permissions change mid-request (cache invalidation), the request could partially succeed under authorization state A and partially succeed under state B, violating business invariants.

*What breaks:* Data integrity. A journal entry half-posted because posting authorization changed mid-request.

*How enforced:* ViewerContext must be immutable and snapshot-isolated. Constructed at request start, never re-queried during the request's lifecycle.

*Enforcement belongs in:* ViewerContext construction (immutable struct, not interface with live queries).

---

**A6: Capability IDs are globally unique within the CapabilityManifest**

*Why it exists:* If two capabilities share an ID, a grant for capability `X` grants access to both capabilities simultaneously — privilege escalation.

*What breaks:* Authorization correctness. A grant for `finance.invoice.read` accidentally grants `hr.payslip.read` if they share an ID.

*How enforced:* Compiler validates capability ID uniqueness across all registered modules during compilation. Duplicate IDs must cause `Compile()` to return an error.

*Enforcement belongs in:* Compiler (compile-time).

---

**A7: Capability alias chains must terminate — no circular aliases**

*Why it exists:* If capability `A` aliases `B` which aliases `A`, resolution never terminates.

*What breaks:* Process hang or stack overflow on grant evaluation.

*How enforced:* Compiler detects alias cycles during compilation (directed graph cycle detection, O(V+E)).

*Enforcement belongs in:* Compiler.

---

### Category B — Compiler Invariants

**B1: CompiledSchema is immutable after Compile() returns**

*Why it exists:* Runtime reads CompiledSchema from multiple goroutines concurrently. Mutation after return causes data races.

*Enforcement belongs in:* Compiler (construction) + code convention (no mutation after return).

---

**B2: Every FieldTypeLink target resolves to an existing entity**

*Why it exists:* A dangling link target means SDUI cannot generate a search URL, repository cannot enforce FK constraints, compiler cannot generate correct routes.

*Enforcement belongs in:* Compiler Phase 2 (already done — must never be relaxed).

---

**B3: QualifiedName is injective — no two entities produce the same QualifiedName**

*What breaks:* Routes overlap, DB table names conflict, Redis namespaces conflict, Casbin policies apply to wrong entity.

*Enforcement belongs in:* Registry `def.Register()` (already done via panic on duplicate).

---

**B4: Route paths are uniquely keyed — no two routes share (Method, Path)**

*What breaks:* Fiber uses the first matched route. A duplicate on a less-protected handler bypasses authorization middleware.

*Enforcement belongs in:* Compiler Phase 3 (`emitRoutes()` must deduplicate and validate).

---

**B5: Compilation is deterministic — same registry input produces identical CompiledSchema**

*Why it exists:* Non-deterministic compilation means two server instances compile different route sets and different Casbin policies. Debugging becomes impossible; multi-instance behavior diverges.

*How enforced:* Compiler must process definitions in deterministic order (sorted by QualifiedName). All map iteration during compilation must be replaced with sorted-slice iteration. Go maps are non-deterministic in iteration — any phase that ranges over `schema.ByName` produces non-deterministic output.

*Enforcement belongs in:* Compiler (sort all map iterations).

---

### Category C — Runtime Pipeline Invariants

**C1: `after_save` hooks run inside the database transaction — always**

*Why it exists:* `after_save` is for side effects that must be atomic with the persist (audit log rows, denormalized counters). Outside the transaction, partial failures leave data inconsistent.

*What breaks:* Data integrity. A journal entry that posts but its ledger entries are not created.

*Enforcement belongs in:* Driver contract + driver integration tests. The framework cannot enforce this structurally — it is a contractual requirement on whoever implements the driver.

---

**C2: Workflow starts occur outside the transaction — always**

*Why it exists:* `StartWorkflow` is a network call. Inside a transaction: slow Temporal responses hold DB locks; TX rollback means the entity doesn't exist but the workflow runs; TX commit + workflow failure means entity exists but workflow never runs.

*Enforcement belongs in:* Pipeline architecture convention. Must be preserved as new pipeline stages are added.

---

**C3: TenantID is set on EntityRecord before any hook stage executes**

*What breaks:* NamingSeries cross-tenant collision. Authorization context corruption. Audit log without tenant attribution.

*Current defect:* `applyNamingSeries` checks `tenantID == uuid.Nil` and **skips silently**. A missing TenantID should be a hard error, not a silent skip.

*Enforcement belongs in:* Pipeline (must return an error on zero TenantID, not silently skip).

---

**C4: Field defaults are applied exactly once, before validation**

*Why it exists:* Defaults after validation rejects required fields with defaults. Defaults applied twice produce different values if the default function has side effects (UUID generator).

*Enforcement belongs in:* Pipeline stage ordering (already correct — document as invariant, add test).

---

**C5: Immutable fields are enforced on Update — never on Create**

*Why it exists:* Fields marked `Immutable` must be settable on Create. Enforcing immutability on Create makes them unwriteable permanently.

*Enforcement belongs in:* Pipeline (already correct — add regression test).

---

### Category D — Repository Invariants

**D1: `set_tenant_context()` is called before every SQL operation on a tenant-scoped table**

*Why it exists:* PostgreSQL RLS evaluates `current_tenant_id()` which reads the transaction-local config set by `set_tenant_context()`. If not called, `current_tenant_id()` returns NULL.

*Enforcement belongs in:* Repository driver (called on every connection checkout in transaction mode).

---

**D2: Writes to system entity tables must go through the pipeline**

*Why it exists:* The pipeline enforces validation, hooks, naming series, and audit. Bypassing it voids all guarantees.

*Enforcement belongs in:* Code convention + static analysis. The framework cannot technically prevent direct pgx calls to system tables.

---

### Category E — SDUI Invariants

**E1: Permission-gated UI elements are absent, not disabled**

*Why it exists:* Disabled elements can be enabled via browser developer tools. Absent elements cannot. The schema must not reveal the existence of capabilities the actor does not hold.

*Enforcement belongs in:* SDUI generator (capability mask at schema construction).

---

**E2: SDUI schema cache is keyed by (entity, version, tenant, capability_fingerprint)**

*Why it exists:* Without capability_fingerprint, a user with limited permissions sees the full schema cached for a user with broader permissions.

*Current defect:* Cache key is `page:{entity}:{view}:{tenant_id}` — does not include actor's capabilities. Two-level caching (base schema + per-request mask) is the correct solution.

*Enforcement belongs in:* SDUI generator + cache key construction.

---

### Category F — Cache Invariants

**F1: Cache is never the source of truth for authorization decisions**

*Why it exists:* Stale cache grants access after revocation.

*How enforced:* Authorization cache (if any) must be invalidated **synchronously** on grant/deny changes. TTL must represent the maximum tolerable inconsistency window for authorization — near-zero with active invalidation.

*Enforcement belongs in:* IAM module (invalidation on grant change) + PolicyEvaluator.

---

**F2: All cache keys are tenant-namespaced**

*Why it exists:* Non-tenant-namespaced key can be hit by a different tenant's request.

*What breaks:* Tenant isolation. SDUI schema for tenant A served to tenant B.

*How enforced:* The framework should provide a `TenantCache` wrapper that automatically prefixes all keys with the tenant ID, making un-namespaced writes impossible.

*Enforcement belongs in:* Cache wrapper (structural enforcement, not convention).

---

### Category G — Identity Invariants

**G1: Session tokens are cryptographically unpredictable**

*Enforcement belongs in:* IAM module (session creation must use `crypto/rand` or UUID v7).

---

**G2: Background jobs carry explicit TenantID — never inherit from process context**

*Why it exists:* A background goroutine has no inherent tenant. Process-context TenantID is stale or zero.

*What breaks:* NamingSeries allocated with zero TenantID (cross-tenant collision). Audit log unscoped.

*Enforcement belongs in:* Convention + Temporal activity input types (always include TenantID explicitly).

---

**G3: Tenant status is checked on every request, not only at authentication**

*Why it exists:* Tenant may be SUSPENDED after authentication. Subsequent requests must respect the new status.

*Enforcement belongs in:* Middleware (re-validate tenant status per-request with short-TTL cache + synchronous invalidation on status change).

---

## 2. Hidden Architectural Assumptions

| # | Assumption | Depends On | Should Become Invariant? |
|---|---|---|---|
| 1 | Clocks synchronized to within 1 second | NamingSeries period key, JWT expiry | Yes — startup assertion + documented constraint |
| 2 | Redis is single-node or naming-series keys hash to same slot | NamingSeries atomicity | Yes — document hash-tag requirement for Cluster; startup assertion |
| 3 | Application DB role has neither superuser nor BYPASSRLS privilege | RLS enforcement | Yes — startup assertion: query `pg_roles`, fail if either is true |
| 4 | CompiledSchema is not tampered with after Compile() | Every runtime security decision | OS-level concern — document as assumption |
| 5 | ViewerContext is constructed once per request and never mutated | Authorization consistency (A5) | Yes — immutable value type, no setters |
| 6 | All modules compiled into same binary | Registry init() ordering, registry.Build() | Yes — state as explicit architectural constraint: "v1.0 is monolithic" |
| 7 | PgBouncer in transaction mode | `set_tenant_context()` reset on COMMIT | Yes — startup verification query |
| 8 | Compiler output is deterministic across restarts | Multi-instance consistency, cache validity | Yes — sort all map iterations; add determinism test |
| 9 | ActionHandlerFunc is stateless | Concurrent invocation correctness | Yes — document as invariant; prohibit mutable closure state |
| 10 | Temporal workflow IDs are deterministic per (tenant, entity, record, event) | Deduplication semantics | Yes — framework rejects empty WorkflowID; format is enforced |

---

## 3. Formal Correctness

| Principle | Score | Verdict |
|---|---|---|
| Separation of Concerns | 7.5/10 | EntityDefinition drives 5 subsystems well. Violated by PermissionSet in business modules, compiler emitting IAM concepts, PolicyFunc closures in domain layer. |
| Single Responsibility | 6.5/10 | EntityDefinition has too many responsibilities (by design). Pipeline and CompiledSchema are clean. PermissionSet conflates operation-level and row-level authorization. |
| Dependency Inversion | 8/10 | ActionRuntime, EntityRepository, PolicyEvaluator are interfaces. Violated by `def` importing `uuid` (external library), workflow code importing Temporal SDK directly. |
| Open/Closed Principle | 6.5/10 | New entities: open. New FieldType: 5-file change (closed). New EventType: closed without framework change. New PageKind: closed. |
| Information Hiding | 7/10 | `CompiledSchema.def` is unexported (good). `EntityRecord.Data` is a shared mutable map visible to all hooks (poor). `Actor.Roles` exposes auth representation to domain code. |
| Layer Isolation | 8/10 | Dependency direction is clean. def is dependency-free. Minor violations: compiler emitting IAM concepts; SDUI performing permission work; PolicyFunc defined in business modules. |
| Domain Isolation | 7/10 | Modules have own packages. `LinkTarget` strings create inter-module string coupling. No enforcement against cross-module data access via `ActionRuntime.Repo()`. |
| Runtime Isolation | 8/10 | RLS enforces tenant isolation at DB level. ViewerContext scopes per-request. Shared Redis means cache key collisions possible if not namespaced. |
| Data Ownership | 8/10 | Each entity owned by exactly one module. CustomFields JSONB has unclear ownership across modules. |
| Least Privilege | 6/10 | `Actor.IsPlatformAdmin` grants total bypass. `ActionRuntime.Repo(entityName)` accepts any entity name — handler for invoice can read IAM users. |
| Complete Mediation | 7.5/10 | All CRUD routes go through pipeline. RLS enforces at DB level independently. Temporal activities have no mediation on ActionRuntime scope. Internal Go calls between hooks bypass all mediation. |
| Fail Closed | 8/10 | Redis down → 503 on auth. Auth failure → deny. Cache miss on DB unavailable: behavior unspecified — should be 503. NamingSeries zero TenantID silently skips rather than failing. |
| Secure by Default | 7.5/10 | Empty PermissionSet → deny all (correct). Nil PolicyFunc → no restriction (open default — should require explicit `AllowAll`). Empty `ActionDef.Permission` falls back to Write (implicit, opaque). |
| Deterministic Behavior | 7/10 | Pipeline and hook ordering are deterministic. Map iteration in compiler is not. `time.Now()` in NamingSeries makes allocation non-deterministic for replay. Empty WorkflowID generates UUID (non-deterministic). |
| Referential Transparency | 4/10 | Intentionally low and acceptable — hooks and action handlers are effectful by design. Workflow code must be deterministic (documented but unenforced). |

---

## 4. Cyclic Dependency Analysis

### Declared Dependency Graph (No Structural Cycles)

```
stdlib/uuid
    ↑
def (pure declarations — no framework reverse dependencies)
    ↑                    ↑
cache                  filter
    ↑
naming
    ↑
registry ← def
    ↑
compiler ← registry, def
    ↑              ↑
runtime ← compiler, def, naming
    ↑
sdui ← compiler, cache, def
    ↑
business modules ← def
```

No structural cycles exist. The `def` package being dependency-free (modulo stdlib) is a significant architectural achievement.

---

### Latent Operational Cycles

**Latent Cycle 1: Business Hook → ActionRuntime.Repo → Pipeline → Business Hook (recursive)**

```
FinanceHook.AfterCreate()
    → ActionRuntime.Repo("finance_journal_entry").Create()
        → pipeline.RunBeforeCreate()
            → FinanceHook.BeforeCreate()   ← re-entrant
```

Risk: Medium. Stack overflow or infinite loop if a hook creates a related record of the same entity type.
Resolution: Pipeline maintains a per-request invocation depth counter. Depth > 5 returns an error.

---

**Latent Cycle 2: IAM Module ↔ PolicyEvaluator ↔ IAM Module (bootstrap)**

```
Request arrives
    → PolicyEvaluator.Evaluate(actor, capability)
        → loads grants from iam_grant entity (via repository)
            → loading iam_grant requires authorization
                → PolicyEvaluator.Evaluate(...)   ← infinite regress
```

Risk: High. Will surface when IAM module is implemented.
Resolution: PolicyEvaluator requires a privileged direct-read path for IAM bootstrap entities that bypasses the full authorization pipeline. This path must be tightly scoped (IAM module only) and audited separately.

---

**Latent Cycle 3: Audit Hook ↔ Audit Entity ↔ Audit Hook (infinite write)**

```
EntityRecord Create → AuditHook.AfterCreate()
    → writes to platform_audit_event via pipeline
        → pipeline.RunAfterCreate() for audit event
            → AuditHook.AfterCreate() for audit event
                → writes to platform_audit_event   ← infinite
```

Risk: High. Must be addressed before audit module implementation.
Resolution: Audit writes bypass the entity pipeline (direct DB insert). OR platform_audit_event carries a `NoAudit: true` flag on its SystemDefinition that prevents audit hooks from firing for audit entities.

---

**Latent Cycle 4: Workflow → ActionRuntime → WorkflowTrigger → Workflow (recursive workflow)**

```
InvoiceApprovalWorkflow runs
    → activity uses ActionRuntime.Repo("invoice").Update(...)
        → Update triggers WorkflowTrigger(EventOnUpdate)
            → StartWorkflow("InvoiceApprovalWorkflow")   ← same type
```

Risk: Medium. Temporal deduplication prevents infinite execution if WorkflowID is deterministic, but creates duplicate workflows if WorkflowID differs.
Resolution: WorkflowTrigger conditions include state guards. Framework enforces deterministic WorkflowIDs.

---

## 5. Boundary Purity

### Compiler Performing IAM Work

Compiler emits `CasbinPolicies` (or `PolicyTuples`) from `PermissionSet` declarations. This crosses from structural compilation into IAM initialization.

Correct design: Compiler emits a `CapabilityManifest` — a flat list of `{ModuleID, EntityName, Operation, CapabilityID}`. IAM initialization reads the manifest and derives policy tuples. Compiler remains ignorant of policy engine (Casbin, OPA, Cedar).

Impact: Replacing Casbin with OPA requires changing the compiler. This contradicts replaceability.

---

### SDUI Performing Security Work

SDUI generator applies capability masks to remove permission-gated elements — it is performing authorization work.

Correct design: Generator produces full schema. A separate `CapabilityFilter` service (owned by IAM/Authorization layer) applies the mask. Generator knows only structural properties (fields, field types, edges) — not who can see what.

---

### Business Modules Performing Authorization Work

`PermissionSet.Policy: PolicyFunc(func(ctx context.Context) def.Filter {...})` is a closure defined in business module code. Inside, module code calls `session.ActorFromContext(ctx)` and builds row-level filters.

Business module code is: extracting the actor (authentication concern), constructing row-level filters based on actor properties (authorization concern). Both concerns belong above the domain layer.

Correct design: `SystemRecordFilter` — a declarative expression (`{field: "assigned_to", operator: "eq", value: "$actor.user_id"}`). Repository interprets the expression; business module declares only the filter shape.

---

### Runtime Performing Cross-Module Naming Convention Work

`applyNamingSeries()` extracts `organization_code` and `org_code` from the record with hardcoded field name heuristics. The runtime is encoding knowledge of organization-specific field naming conventions.

Correct design: `FieldDef` carries an `OrgCodeField string` attribute specifying which field provides the org code for NamingSeries. Runtime reads the attribute; it does not guess.

---

### Audit as Module-Level Opt-In

Audit module is one of seven platform modules, implemented identically to business modules. Audit logging is opt-in — a module author must add an audit hook, or no audit record is ever written.

Correct design: Audit is a cross-cutting concern implemented at the Pipeline level. Every `RunBeforeCreate`, `RunAfterCreate`, `RunBeforeUpdate`, etc., emits an audit event automatically. Module-level audit hooks add supplementary data; they are not the primary mechanism.

---

## 6. Replaceability Analysis

| Component | Replaceable? | Primary Blocker | Estimated Effort |
|---|---|---|---|
| PolicyEvaluator | Yes (if compiler decoupled) | `CasbinPolicy` type in CompiledSchema; role hierarchy semantics | 2–3 weeks |
| Redis (caching) | Yes | `DeletePrefix()` may not be atomic in all replacements | 1–2 days |
| Redis (sessions) | Moderate | Session storage format; SCAN+DEL semantics | 1 week |
| Redis (NamingSeries) | Moderate | Requires atomic INCR guarantee; Cluster hash-slot constraint | 1 week |
| PostgreSQL | Difficult | RLS stored procedures; `pg_trgm`; `jsonb` GIN; pgx driver; `pgconn.PgError` | 3–6 months |
| Temporal | Difficult | `workflow.Now()`, `workflow.Sleep()`, `workflow.SideEffect()` baked into workflow code; determinism model is Temporal-specific | Rewrite all workflow code |
| Authentication provider | Yes | Session token format in Redis; role name format | 1–2 weeks |
| Audit backend | Yes (if moved to pipeline) | `platform_audit_event` is a PostgreSQL entity; query API tied to EntityRepository | 2–3 weeks |
| **amis SDUI renderer** | **No** | **Entire generator produces amis-specific JSON; no intermediate UISchema abstraction; `web/sdk/` pinned bundle; all PageBuilders use amis constructs** | **Rewrite generator + all PageBuilders** |
| Repository implementation | Yes | EntityRepository interface abstracts it; RLS enforcement must move with it | 2–4 weeks |
| Compiler | No (by design) | Compiler IS the framework kernel — replacing it means replacing the framework | N/A |
| Workflow engine | Difficult | Temporal programming model is unique; no abstraction layer over `go.temporal.io/sdk` in workflow functions | Major |
| Identity provider | Yes | Session middleware is integration point | 1–2 weeks |
| Message bus (event outbox) | Yes (if `EventBus` interface is clean) | Implementation not yet specified | Depends on implementation |

**The single highest-risk replaceability gap:** amis has no intermediate representation. Adding a `WidgetTree` IR between the generator and amis JSON is a 2–3 week investment before v1.0 that prevents a 3–6 month rewrite later.

---

## 7. Evolution Stress Test

| Evolution | Status | Notes |
|---|---|---|
| Millions of capabilities | Additive with Redis O(1) lookup | Casbin in-memory model fails; indexed Redis lookup required |
| Millions of tenants | Native | Designed for this from the start; no bottleneck |
| Millions of org units | Requires extension | Materialized path LIKE query becomes slow; closure table or `ltree` extension required |
| Attribute-Based Access Control | Requires extension | `Actor` needs attribute bag; PolicyEvaluator interface needs widening; RecordFilter already handles resource attributes |
| Relationship-Based Access Control (Zanzibar) | Requires redesign | Flat grant model cannot express graph-traversal authorization; v2.0 architectural change |
| Geo restrictions / data residency | Requires redesign | Single-PostgreSQL model assumes one region; per-region deployment needs tenant routing |
| Offline clients | Requires redesign | amis SDUI requires connectivity; no sync model; conflicts with RLS model |
| Multi-region writes | Requires redesign | NamingSeries counters not globally consistent; write routing not implemented |
| Multi-region reads | Additive | Read replicas straightforward; RLS stored procedure must be in all regions |
| Cross-tenant federation | Impossible without extension | RLS prevents cross-tenant access by design; requires `FederationGrant` concept + RLS policy extension |
| Marketplace plugins | Impossible without redesign | `init()` model requires compile-time presence; Go plugin system or gRPC extension protocol needed |
| External policy engines | Additive | PolicyEvaluator interface accommodates OPA, Cedar, etc. |
| AI agents as principals | Additive | Service account principal + capability scoping already in design; add `Actor.AgentID` |
| Event sourcing (hybrid) | Additive | `after_save` hook emits events to event store alongside mutable record |
| Event sourcing (pure) | Requires redesign | Replace EntityRepository with EventRepository; derive state from projections |
| CQRS | Requires extension | Command side is current pipeline; query side needs separate read models maintained via `after_save` events |
| Graph storage | Additive | Custom EntityRepository implementation backed by graph DB |
| Federated identity (OIDC/SAML) | Additive | Session validation middleware is the integration point |
| Stream processing | Additive | EventBus interface routes to stream processor |

---

## 8. Consistency Analysis

### Critical Fragmentation: "Policy" — Four Distinct Meanings

| Name | Location | Represents |
|---|---|---|
| `PermissionSet.Policy` / `PolicyFunc` | `def/permission.go` | Row-level filter closure |
| `CasbinPolicy` | `compiler/schema.go` | Casbin policy tuple (subject, object, action) |
| `PolicyEvaluator` | canonical design | Engine-agnostic authorization interface |
| `CapabilityManifest` | canonical design | Compiler output listing all capabilities |

Resolution: `PolicyFunc` → `RowFilter`; `CasbinPolicy` → `PolicyTuple`; others correctly named.

---

### Critical Fragmentation: "Schema" — Four Distinct Meanings

| Usage | Meaning |
|---|---|
| `CompiledSchema` | Compiler output (entity metadata) |
| amis "schema" | UI page description (JSON) |
| Database schema | PostgreSQL table definitions |
| `EntitySchema` | Per-entity compiled metadata |

Resolution: Qualify consistently — "page descriptor" for amis JSON; "migration" or "table definition" for SQL; keep `CompiledSchema` and `EntitySchema`.

---

### Principal vs Actor — Same Concept, Two Names

`Actor` (codebase) and `Principal` (canonical design) are the same concept. Document explicitly that `Actor` IS the runtime `Principal`. Do not introduce a separate `Principal` type.

---

### Grant — First-Class in Design, Absent in Code

The canonical design specifies `Grant{Principal, Capability, Scope, OrgConstraint, Expiry, Deny}`. No `Grant` Go type exists. The nearest equivalent is `CasbinPolicy` in the compiler and Casbin's internal policy storage at runtime.

Impact: Grant expiry is not in the Casbin model (must be application-managed). OrgConstraint has no storage location. Deny semantics require Casbin's `deny_override` matcher.

Resolution: Define a `Grant` entity in the IAM module with explicit fields. `after_save` hook synchronizes the Grant entity to Casbin's policy store.

---

### PermissionSet / CapabilitySet — Dual Authorization Models (Critical)

Both exist simultaneously. A module using `PermissionSet` gets one authorization behavior; a module using `CapabilitySet` gets another. Mixed modules produce undefined behavior at the PolicyEvaluator level.

**This is the most critical consistency gap.** A v1.0 decision is required: either `PermissionSet` is the v1.0 model (CapabilitySet deferred to v1.3 with migration plan), or `CapabilitySet` is the v1.0 model (PermissionSet explicitly deprecated). Having both as equal options at release is architecturally incorrect.

---

### RecordFilter — Two Names, One Concept

`PolicyFunc` (current implementation) and `SystemRecordFilter` (canonical design replacement) are the same concept. In code comments, explicitly note `PolicyFunc` is deprecated in favor of `SystemRecordFilter`. Give a sunset version.

---

## 9. State Machine Audit

### Session State Machine

```
ABSENT → VALID      (login)
VALID → EXPIRED     (TTL)
VALID → REVOKED     (logout / admin revocation)
EXPIRED → ABSENT    (Redis cleanup)
REVOKED → ABSENT    (Redis cleanup)
```

**Missing state:** `INVALIDATED_ALL` — password change or breach requires simultaneous invalidation of all user sessions. No user→sessions index exists in Redis.

**Missing transition:** `VALID → SUSPENDED` — user account suspension should immediately terminate active sessions, not wait for TTL expiry.

**Race condition:** Between `VALID` check and use — if revocation happens between the GET and the use of the session data, the revoked session completes one request. Tolerable window (< 1ms) for most threat models. Mitigate with Redis `GETDEL` for single-use tokens if needed.

---

### Grant State Machine

```
ABSENT → ACTIVE     (admin grant creation)
ACTIVE → REVOKED    (admin grant deletion)
ACTIVE → EXPIRED    (grant TTL, if supported)
ACTIVE → SUSPENDED  (tenant suspension)
SUSPENDED → ACTIVE  (tenant reactivation)
```

**Missing transition:** What happens to grants when the Principal (user) is deleted? Casbin does not cascade-delete role assignments. Orphaned grants remain indefinitely.

Resolution: IAM module's `AfterDelete` hook for User entities must remove all grants from the policy store.

**Missing state:** `PENDING` — some models require approval before grant becomes active.

---

### Capability State Machine

```
UNREGISTERED → DECLARED   (EntityDefinition field declaration)
DECLARED → COMPILED       (Compile())
COMPILED → ALIASED        (capability rename with alias declared)
COMPILED → DEPRECATED     (deprecation marker)
DEPRECATED → REMOVED      (after alias migration period)
ALIASED → REMOVED         (alias cleaned up)
```

**Impossible transition:** `REMOVED → COMPILED` — once removed, cannot be restored under the same ID. Aliases are the only recovery path.

**Race condition:** During deprecation, grants may be evaluated using the old capability ID before the alias is propagated. During the transition window, some requests succeed while others fail for the same operation.

---

### Tenant State Machine

```
PENDING → ACTIVE      (provisioning complete)
PENDING → ARCHIVED    (abandoned)
ACTIVE → SUSPENDED    (payment failure)
ACTIVE → ARCHIVED     (account deletion)
SUSPENDED → ACTIVE    (payment resolved)
SUSPENDED → ARCHIVED  (grace period expired)
```

**Race condition:** Tenant transitions from `SUSPENDED → ACTIVE` while a request is being processed under `SUSPENDED` context. The middleware caches tenant status (5-minute TTL). A tenant reactivated during that window continues to receive 402 responses until cache expires — the tenant is paying but cannot access the system.

Resolution: Tenant status cache invalidation must be synchronous on status change. IAM module's `AfterUpdate` hook for Tenant entities must call `cache.DeletePrefix(ctx, "tenant:status:{tenant_id}")` immediately.

---

### Workflow State Machine (Framework Interface Layer)

```
NOT_TRIGGERED → START_PENDING  (WorkflowTrigger fires after TX commit)
START_PENDING → RUNNING        (Temporal StartWorkflow succeeds)
START_PENDING → FAILED         (Temporal StartWorkflow fails)
RUNNING → COMPLETED            (workflow completes)
RUNNING → FAILED               (workflow fails)
FAILED → START_PENDING         (retry queue re-attempt)
```

**Missing state:** `ORPHANED` — if retry queue is not implemented, `START_PENDING → FAILED` has no recovery transition. Entity is saved but workflow never starts. **This is the framework's highest operational risk.** The retry queue is mentioned in architecture but not implemented.

---

### Authorization Decision State Machine

```
PENDING → ALLOW   (grant found, no deny)
PENDING → DENY    (no grant, or explicit deny)
PENDING → ERROR   (evaluation system unavailable)
ERROR → DENY      (fail-closed)
```

**Missing state:** `CACHED_ALLOW` / `CACHED_DENY` — if authorization results are cached, the state machine should distinguish "live evaluated" from "cache-served" decisions, and cache invalidation must be part of the state machine contract.

---

### Cache State Machine

```
EMPTY → POPULATING     (first cache miss — atomic reservation via SET NX)
POPULATING → POPULATED (successful computation)
POPULATING → EMPTY     (computation failure)
POPULATED → STALE      (TTL nearing expiration)
POPULATED → INVALID    (explicit invalidation)
STALE/INVALID → EMPTY → POPULATED (refill)
```

**Missing state:** `POPULATING` — without this state (SET NX with short jitter), multiple simultaneous cache misses all recompute independently (stampede). CLAUDE.md mentions stampede prevention; the SDUI cache implementation does not appear to implement it.

---

## 10. Distributed Systems Audit

### Multiple Regions

**`set_tenant_context()` stored procedure** must be identical in every region's database. Different PostgreSQL major versions may have different NULL handling. A CI pipeline must deploy and test stored procedures across all regions before traffic cutover.

**NamingSeries across regions (critical gap):** If Tenant T's write traffic splits between Region A (Redis A) and Region B (Redis B) during failover, both Redis instances serve INCR on the same logical key independently. Counters diverge. Result: duplicate naming-series values — a data integrity violation for regulatory document numbering.

Resolution requires one of:
1. Write affinity: tenant T always writes to the same region
2. Global atomic counter service (CockroachDB sequences, or distributed counter with strong consistency)
3. Accept gaps and duplicates (not acceptable for regulatory contexts)

---

### Partial Failure — Workflow Start After Commit

```
TX begins
    → PERSIST entity record
    → RunAfterCreate hooks (inside TX)
TX commits
    → StartWorkflow(ctx, spec)   ← FAILS HERE
```

Entity persists. Workflow never starts. No mechanism detects or recovers this state. An invoice appears "submitted" but never enters the approval workflow.

Resolution: Persist workflow start requests to a `workflow_retry_queue` table **inside the TX** (before commit). A background process retries `StartWorkflow` until success. This is the outbox pattern for Temporal workflow starts. It must be designed before v1.0.

---

### Idempotency — Missing for Create Operations

HTTP POST is not idempotent. A double-submit creates two records. Particularly dangerous for:
- Financial records (two ledger entries)
- Naming series allocations (two different numbers for the same document)

Resolution: API layer supports `Idempotency-Key` header. Pipeline checks if a record with that key already exists (keyed by tenant + entity + idempotency_key in Redis or PostgreSQL). If so, returns the existing record.

---

### Eventual Consistency — Cache Invalidation Semantics

For SDUI schemas: eventual consistency is acceptable (users see stale UI for up to 5 minutes).

For authorization decisions: NOT acceptable (must be consistent). If authorization cache includes capability-masked entries (per current single-level implementation), a permission change must invalidate ALL actor-specific cache entries. At N tenants × M actors, this is O(N×M) deletions per permission change. This does not scale.

Two-level caching (base schema cached at entity level; capability mask applied per-request) solves this — invalidating the base schema invalidates all derived schemas without per-actor cache operations.

---

### Split Brain — NamingSeries

Redis Cluster can enter split-brain. Two partitions both serve INCR on the same logical key → two independent counters → duplicate sequence numbers.

Resolution: Redis Cluster deployments for NamingSeries counters must use hash tags to force all naming-series keys for a tenant to the same hash slot: `{tenant_uuid}:naming:counter:...`. Document this as a deployment requirement; verify at startup.

---

### Retry Storms

No backoff strategy specified for Redis failures or cache misses. If Redis briefly becomes unavailable, all requests fail simultaneously. On recovery, all retry simultaneously → thundering herd.

Resolution: Define explicit retry policies:
- Session validation failure: fail immediately (503) — no retry (security-correct)
- Cache miss (Redis down): fall back to DB computation immediately — no retry
- NamingSeries allocation failure: exponential backoff (up to 3 attempts), then fail the request

---

### Duplicate Events — Workflow Deduplication

Temporal deduplicates by workflow ID. If the framework generates deterministic workflow IDs from `(tenant, entity, record_id, event)`, retry queue re-attempts will not create duplicate workflows. This is correct.

**If `ActionWorkflowSpec.WorkflowID` is empty and the framework generates a UUID, retry attempts produce different IDs → duplicate workflows execute.**

Resolution: Framework rejects `ActionWorkflowSpec` with empty `WorkflowID`. The convention `{tenant}.{entity}.{record_id}.{event}` is enforced, not optional.

---

## 11. Complexity Audit

### Fundamental Concepts (Necessary — Cannot Be Simplified)

| Concept | Justification |
|---|---|
| EntityDefinition | Central primitive driving all subsystems; complexity IS the value |
| CompiledSchema | Single runtime authority; eliminates class of inconsistency bugs |
| Hook Pipeline | Lifecycle management with clear stage semantics; well-understood pattern |
| RLS + `set_tenant_context()` | Tenant isolation guarantee at the correct layer |
| ViewerContext | Authorization surface for business modules; decoupling is essential |
| ActionRuntime | Service injection for action handlers; prevents infrastructure imports in business code |
| NamingSeriesService | Atomic tenant-isolated sequence allocation; unavoidable complexity |

### Accidental Complexity (Could Be Simplified)

| Concept | Problem | Resolution |
|---|---|---|
| `FieldTypeDynamicLink` | Polymorphic references add significant compiler/repository/SDUI complexity for < 5% of use cases | Defer to v1.3; document as "advanced, not recommended" |
| `ActionQueryOpt` functional options | Functional options pattern for a 3-field config struct; premature for this size | Replace with plain struct literal `ActionQuery{Limit: 100}` |
| `PermissionSet.Actions map[string][]string` | Per-action permission lists alongside `ActionDef.Permission`; two places for the same thing | Consolidate to `ActionDef.Permission` only |
| `PolicyFunc` as closure | Opaque, cannot be serialized, inspected, or tested in isolation | Replace with `SystemRecordFilter` declarative expression |
| `PageBuilderSet` with four separate functions | Four named functions for four page kinds; not extensible | Replace with `map[PageKind]PageBuilder` |

### Abstractions That Should Disappear

| Abstraction | Reason |
|---|---|
| `IsPlatformAdmin bool` on Actor | Permission check masquerading as type property; replace with `platform.admin` grant |
| `CasbinPolicy` named type | Bakes in Casbin dependency; replace with `PolicyTuple{Subject, Object, Action string}` |
| `def.Filter` return from `PolicyFunc` | `def` doesn't own filter logic; forced there for circular dependency avoidance |

### Under-Specified Concepts (Missing Owners)

| Concept | Gap |
|---|---|
| Event outbox | Table structure, retry mechanism, delivery guarantee, consumer interface — none specified |
| Workflow retry queue | Mentioned in CLAUDE.md; no table, no process, no TTL, no retry count |
| Service account principal | No concrete Go type, no IAM entity, no grant mechanism |
| Idempotency key for creates | Not in any interface or documentation |
| Capability alias mechanism | Which type carries aliases? Where stored? How does grant evaluation resolve? |

---

## 12. Long-Term Maintainability

### Year 1

Framework is well-documented and opinionated. Primary maintenance cost: amis SDK compatibility — every significant amis release must be audited against all PageBuilders and the generator.

Risk: Authorization model is in transition (PermissionSet → CapabilitySet). Every authorization-related change touches two systems.

---

### Year 5

**God objects emerging:** `CompiledSchema` accumulates new fields with every framework version (Phase N+1 compiler outputs). `EntitySchema` may carry 50+ fields by year 5. Navigation becomes difficult.

**Coordination bottleneck:** Global `def.globalRegistry` must be fully populated before `registry.Build()` is called. As modules grow, init() ordering becomes important. Go's init() ordering is deterministic (import order) but complex to reason about at 50+ modules.

**Migration nightmare:** Any entity renamed after v1.0 requires: migration file rename, Temporal workflow ID migration, Redis key migration, Casbin policy object migration, documentation updates, client SDK updates. The "never rename entity names" invariant becomes increasingly difficult to honor as the business domain evolves.

**amis SDUI evolution:** By year 5, amis will likely have released 2–3 major versions. Each major version potentially breaks the generated schema format.

---

### Year 10

**Performance bottleneck:** `set_tenant_context()` per transaction. At 10,000 requests/second, that is 10,000 stored procedure calls/second. Connection pooling helps but does not eliminate the overhead.

**Organizational bottleneck:** All modules in one binary means one team's breaking change blocks all other teams. Without versioning between modules, the compilation unit becomes an organizational coordination point.

**Knowledge silo:** amis is a project with primarily Chinese-language documentation. Team members with amis expertise who leave are difficult to replace.

---

### Year 20

**Inevitably superseded:**
- amis SDUI — web frontend ecosystem will have changed fundamentally
- Casbin — authorization model will have evolved beyond flat RBAC
- Single binary — plugins and marketplace modules will be required
- PostgreSQL-only persistence — graph, time-series, vector databases needed for AI/ML features

**What survives:**
The `EntityDefinition → CompiledSchema → {5 subsystems}` pattern is architecturally sound and will persist as the framework's kernel. Hook pipeline, compiler-as-authority, and RLS-enforced tenant isolation are correct ideas with 20-year lifespans. They will be reimplemented in new technology, but the concepts are durable.

---

## 13. Architectural Risk Matrix

Architecture-level risks only. Ranked by probability × impact × fix-difficulty-after-v1.0.

| # | Risk | Probability | Impact | Fix Difficulty After v1.0 | Mitigation |
|---|---|---|---|---|---|
| 1 | amis lock-in is permanent | Certainty | High | Extreme | Add `WidgetTree` IR before v1.0; 2–3 weeks; prevents 3–6 month rewrite later |
| 2 | NamingSeries duplicates in split-brain Redis or multi-region | Low (single-region), Medium (multi-region) | Critical | High | Document hash-tag constraint; startup assertion; or use global atomic counter |
| 3 | Workflow start failure after TX commit is silent data loss | Medium | High | Medium | Design outbox table before v1.0; background retry process |
| 4 | RLS bypassed by superuser DB role | Low | Critical | Easy | Startup assertion: query `pg_roles`; fail if `rolsuper` or `rolbypassrls` |
| 5 | IAM ↔ PolicyEvaluator bootstrap cycle | Certainty | High | Medium | Define `UncheckedRead` path in Repository for IAM bootstrap before IAM module implementation |
| 6 | Compiler map iteration non-determinism | Medium | Medium | Low | Audit all compiler phases; replace map iteration with sorted slices |
| 7 | Audit is opt-in — regulatory compliance gap | Certainty | High | Medium | Move base audit emission to Pipeline; module hooks become additive |
| 8 | ViewerContext not a first-class interface | High | High | High | Define ViewerContext interface before v1.0 freeze; even minimal implementation |
| 9 | No idempotency keys for creates | High | High | Medium | Add `Idempotency-Key` header support to auto-generated create route before v1.0 |
| 10 | RecordFilter as closure is untestable and opaque | High | Medium | High | Provide `SystemRecordFilter` as preferred path before v1.0; deprecate `PolicyFunc` |
| 11 | PermissionSet and CapabilitySet coexistence | Certainty | High | High | Make a v1.0 decision; no hybrid |
| 12 | Hook pipeline has no recursive invocation guard | Medium | Medium | Low | Add depth counter to pipeline context; error at depth > 5 |
| 13 | PgBouncer mode unenforceable | Medium | Critical | Low | Startup verification query: set context, read it back, verify |
| 14 | Cross-module data access via ActionRuntime is unmediated | High | Medium | High | Pre-construct ActionRuntime with allowed entity names; cross-module access requires declaration |

---

## 14. Final Architecture Score

### Scoring (1–10, benchmarked against Kubernetes, PostgreSQL, Temporal, Zanzibar, Frappe, Odoo)

| Dimension | Score | Key Factor |
|---|---|---|
| Conceptual Integrity | 7.5 | EntityDefinition-as-primitive is elegant; damaged by dual auth models, four meanings of "policy," four meanings of "schema" |
| Architectural Elegance | 8.0 | Compiler-as-authority pattern is the right idea; 5-subsystem derivation from one declaration is genuinely elegant; amis coupling reduces it |
| Replaceability | 5.5 | PolicyEvaluator and Redis: replaceable; PostgreSQL: difficult; Temporal: difficult; amis: essentially irreplaceable without generator rewrite; no UISchema abstraction |
| Correctness | 7.5 | Authorization model is mathematically sound; RLS isolation is correct; pipeline ordering is correct; damaged by missing idempotency, split-brain NamingSeries risk, silent workflow failures |
| Extensibility | 7.0 | New entities: trivial; new FieldTypes: 5-file change; ABAC: requires widening; ReBAC: requires redesign; plugins: impossible without redesign |
| Composability | 6.5 | Hooks compose well (slice); RecordFilters compose poorly (one PolicyFunc per entity); actions compose poorly (single Permission, no OR); edges compose well |
| Consistency | 5.5 | "Policy" means 4 things; "Schema" means 4 things; Principal/Actor duality; PermissionSet/CapabilitySet coexistence; Grant has no concrete type; weakest dimension |
| Layer Isolation | 8.0 | Dependency direction is clean; def is dependency-free; minor violations: compiler emitting IAM concepts, SDUI performing permission work |
| Maintainability | 6.5 | Good documentation; amis coupling is a growing maintenance burden; entity rename prohibition creates rigidity; single binary limits independent module maintenance |
| Operational Simplicity | 7.0 | Redis + PostgreSQL + Temporal is well-understood; PgBouncer requirement is a constraint; RLS stored procedure requires coordinated DB migrations |
| Distributed Systems Readiness | 5.0 | Single-region design is fine; multi-region has critical NamingSeries flaw; no idempotency keys; workflow retry queue unimplemented; no Redis failure backoff |
| Long-Term Evolution | 6.0 | Core concepts have 20-year lifespans; amis needs replacement within 10 years; single binary incompatible with marketplace model; RBAC→ABAC→ReBAC path unclear |
| Framework Quality | 8.0 | Clean interfaces; strong startup-time validation; fail-closed auth defaults; good naming conventions; Temporal determinism rules documented |
| Developer Experience | 8.0 | Single EntityDefinition drives all subsystems — minimal boilerplate; auto-generated CRUD; amis eliminates frontend work; real learning curve for amis, Temporal, Casbin |
| **Overall Architecture** | **6.9** | |

---

### Comparative Positioning

| Framework | Overall | Notes |
|---|---|---|
| Kubernetes | 9.0 | Exceptional replaceability (CRI/CNI/CSI); strong invariants; highest operational complexity |
| PostgreSQL | 9.0 | Exceptional correctness; decades of battle-testing; limited replaceability at storage layer |
| Temporal | 8.5 | Exceptional distributed correctness; unique programming model; limited replaceability |
| Zanzibar | 9.0 (narrow scope) | Exceptional authorization scalability; clean consistency model |
| **Awo** | **6.9** | Strong core; significant distributed systems and replaceability gaps |
| Frappe | 5.0 | Poor replaceability; good DX; Python monoculture |
| Odoo | 4.5 | Poor architectural purity; good ecosystem; high accidental complexity |

Awo is demonstrably better than Frappe and Odoo on architectural purity, layer isolation, and correctness. It approaches Temporal quality in its core domain (lifecycle management, pipeline correctness). It falls short of Kubernetes on replaceability — Kubernetes's CRI/CNI/CSI are genuine substitution interfaces; Awo's amis coupling has no equivalent abstraction.

---

### The Three Architectural Decisions That Determine the Framework's Ceiling

**Decision 1: Introduce a `WidgetTree` IR before v1.0**

If the SDUI generator produces a renderer-agnostic widget tree and an amis serializer converts it, the ceiling rises from "amis is permanent" to "renderer is replaceable." This is a 2–3 week investment with a 10-year payoff. Without it, amis is architecturally permanent — not a technical decision but a business risk.

**Decision 2: Resolve the PermissionSet/CapabilitySet duality before v1.0**

The authorization model cannot be half-transitioned at release. Either commit fully to `CapabilitySet` (v1.0 breaks from `PermissionSet`) or commit to `PermissionSet` (defer `CapabilitySet` to v1.3 with a clear migration plan). A framework with two authorization models at release will have authorization bugs that are impossible to diagnose — the failure mode depends on which model a given module happened to use.

**Decision 3: Define the ViewerContext interface before v1.0 freeze**

ViewerContext is the stability boundary between business modules and the authorization system. If it is not a concrete interface at v1.0, every subsequent authorization evolution forces changes in business modules. The interface must be frozen at release even if the underlying implementation is minimal. Once frozen, the authorization system can evolve behind it indefinitely.

These three decisions are the difference between a 6.9/10 architecture and an 8.2/10 architecture. None require redesigning the core. They are boundary clarifications that cost weeks to get right and years to retrofit if skipped.
