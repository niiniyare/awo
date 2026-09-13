> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

# Awo Authorization Implementation Audit
## Pre-v1.0 Runtime, Compiler, and Framework Consistency Review

**Date:** 2026-07-20
**Status:** Final — Lead Architect Sign-off Required Before Freeze
**Precondition:** Canonical architecture in AWO_AUTH_CANONICAL_DESIGN.md is accepted as final.

---

## 1 Executive Summary

The canonical architecture is sound. The governing principle — framework defines capabilities, IAM defines who holds them, business modules know only what exists — is achievable with the current codebase foundation. However, **six critical implementation gaps exist** that would make the architecture impossible to operate correctly in production if not resolved before v1.0 freeze. Additionally, **four high-severity gaps** would degrade correctness or scalability within eighteen months of launch. None require redesigning the canonical architecture. All are implementation decisions that were not made during the design phase.

The most dangerous gap: **capability renames silently orphan all existing grants**. Without a capability alias mechanism, the first time any capability is renamed after v1.0 ships, every tenant who held that capability loses access with no error and no migration path. This is a permanent data integrity hazard.

The second most dangerous gap: **ViewerContext is designed exclusively for HTTP**. Temporal activities, background jobs, CLI commands, and cron tasks have no defined principal identity. These contexts will execute with either zero authorization or an implicit bypass — both are wrong.

After resolving all critical gaps, the framework is ready for v1.0. The architecture will sustain 20-year evolution without structural changes if the `PolicyEvaluator` interface, `AuthzRequest`/`AuthzDecision` contracts, and the five core primitives (Capability, Principal, Grant, Deny, RecordFilter) are treated as frozen public API from v1.0 onward.

---

## 2 Runtime Architecture Assessment

### 2.1 ViewerContext Lifecycle — Transport-Independent Requirement

**Critical gap.** ViewerContext is currently modeled as a per-request HTTP construct. This is wrong. Authorization is a framework concern, not an HTTP concern. The following execution contexts all require a ViewerContext with no HTTP request present:

| Context | Current State | Required |
|---|---|---|
| HTTP request handler | Designed | Correct |
| Temporal activity | Undefined | Service account principal |
| Temporal workflow function | Undefined (must be I/O-free) | Not applicable — workflows don't do I/O |
| Cron job | Undefined | Service account principal |
| Background goroutine spawned by hook | Undefined | Inherited from spawning context |
| CLI command | Undefined | CLI-authenticated principal |
| Batch import | Undefined | Service account principal |
| WebSocket (long-lived connection) | Undefined | Refreshable viewer |

**Canonical answer for Temporal activities:** Each task queue has an associated service account. The activity executes with a ViewerContext constructed from that service account's grants. The original triggering principal's ID is embedded in the workflow input for audit. Authorization decisions within the activity use the service account's capabilities, not the original human principal's capabilities. The service account for a task queue holds exactly the capabilities that queue's activities need — nothing more.

This must be designed explicitly before v1.0. The framework must provide:
- `NewServiceAccountViewerContext(ctx, serviceAccountID)` — constructs from database, no session
- `NewSystemViewerContext(ctx)` — bypass for genuine framework-internal operations (e.g., bootstrapping), fully logged
- `RequireViewer(ctx)` — panics if no ViewerContext is in context (catches bugs where authorization was skipped)

**WebSocket long-lived connections:** A ViewerContext constructed at connection-open time will be stale within 60 seconds (grant cache TTL). The connection must carry a reference to the `PolicyEvaluator` and re-evaluate at configurable intervals (recommended: every 5 minutes). The ViewerContext must expose a `Refresh(ctx)` method that rebuilds the resolved identity from the current cache/database state. If the principal's session is invalidated during the connection, `Refresh()` returns an error and the connection must be terminated.

### 2.2 Middleware Ordering

The current middleware order requires correction for the new authorization model. The ordering must be:

```
1. Request ID                          — before everything; needed for log correlation
2. Structured logging                  — needs request ID
3. Panic recovery                      — before any business logic
4. CORS                               — before any request body processing
5. IP-based rate limiting              — before auth; protects auth endpoints from abuse
6. Tenant resolution                   — before auth; X-Tenant-ID determines which IDP to use
7. PostgreSQL tenant context           — set_tenant_context() here; RLS must be active before any query
8. Authentication                      — session → Principal; REQUIRES tenant context to be set
9. Identity resolution                 — Principal → ResolvedIdentity; may query IAM tables (RLS-protected)
10. ViewerContext construction          — ResolvedIdentity → ViewerContext
11. Operation permission check         — ViewerContext + route capability; FIRST authz decision
12. Principal-based rate limiting      — AFTER auth; per-user limits
```

The critical ordering constraint: **step 7 (set_tenant_context) must precede step 8 (authentication)**. The `iam_user` table is tenant-scoped and protected by RLS. If a session lookup queries `iam_user` before `set_tenant_context()` is called, it sees no rows (RLS denies everything). This is a subtle correctness bug that will appear only when the RLS-protected user table is queried during authentication.

The current design places tenant resolution at step 5 and session validation at step 6, which is consistent with this requirement — but the PostgreSQL tenant context call must happen as part of step 5, not lazily later.

### 2.3 Context Propagation

`ViewerContext` is propagated via `context.Context`. The key used for storage must be defined in one canonical place in the framework (e.g., `runtime.ViewerContextKey`). Every package that needs the viewer extracts it with `runtime.ViewerFromContext(ctx)`. If no viewer exists, `ViewerFromContext` returns a sentinel error (not nil, not a zero-value ViewerContext).

**Hidden coupling risk:** If the IAM module and the business module both define their own context key for the viewer (different key types), they will fail to share the viewer. The viewer context key MUST be owned by the framework runtime package — not by IAM, not by any business module.

### 2.4 Batch Authorization

**Critical gap.** Bulk operations (import 10,000 records, bulk update, report generation) cannot afford O(N) authorization checks. The current design has no batch authorization path.

The canonical answer:
1. **Collection-level capability check:** At the start of a bulk operation, check that the principal holds the collection-level capability (e.g., `finance.invoice.create`). One check, no per-record evaluation.
2. **Filter-level enforcement:** RecordFilters are injected into the underlying SQL, providing row-level visibility for reads even at scale. One query, N records.
3. **Per-record prohibition:** There is no per-record write authorization path at v1.0. A principal who can create invoices can create any invoice in their authorized scope. Fine-grained per-record authorization (ABAC) is v2.

This means the entity repository's `BulkCreate` and `BulkUpdate` operations receive a single capability check at the start, not per-record checks. The `ActionRuntime` must make this semantics explicit: bulk operations are authorized at collection level.

### 2.5 Concurrency Safety

The `PolicyEvaluator` implementation will be called concurrently for every in-flight request. It must be stateless (all state in Redis/DB) or use immutable state (the CapabilityManifest is already immutable after startup). No global mutable state is permitted in the evaluator.

The `RuntimeRegistry` (which wraps the `CompiledSchema`) is already immutable and safe for concurrent reads.

The grant cache in Redis is safe for concurrent reads (Redis is single-threaded within a keyspace). Concurrent writes to the same cache key (two goroutines populating the same expired cache entry) must be handled with Redis `SET NX EX` (set if not exists, with expiry) to prevent stampede.

---

## 3 Compiler Assessment

### 3.1 Capability Compilation — Missing Phase

The compiler currently has four phases: validate, build entity schemas, resolve links, emit routes, emit policy tuples. The capability manifest is **not a named compiler phase**. This must become Phase 5.

Phase 5 responsibilities:
- Generate CRUD capabilities automatically: for each entity, emit `{module}.{entity}.{read,write,create,delete}`
- Collect action capabilities from `ActionDef.Capability` fields
- Validate: no two capabilities share the same ID
- Validate: no `ActionDef.Capability` value conflicts with auto-generated CRUD IDs
- Collect `SystemRecordFilters` from `CapabilitySet.RecordFilters` on each entity
- Emit `CapabilityManifest` as a top-level output of `CompiledSchema`

Phase 5 must also validate **capability ID uniqueness globally**. If two modules both define `{module}.{entity}.read` with the same module name, that is a collision error.

### 3.2 Capability Alias Mechanism — Critical Gap

**Critical.** This is the most important compiler feature missing from the current plan.

When a capability is renamed between versions:
- v1.0: `ActionDef{Capability: "finance.invoice.submit"}`
- v1.1: `ActionDef{Capability: "finance.invoice.post"}` (renamed for domain accuracy)

Every grant in every tenant's database that references `"finance.invoice.submit"` is now orphaned. The evaluator evaluates `"finance.invoice.post"`, finds no grants, returns Deny. Every user silently loses access. There is no error, no migration warning, no recovery path.

The fix requires three components:

**Component 1: `ActionDef.CapabilityAliases []string`**
Module developer declares previous capability IDs:
```go
ActionDef{
    Capability:        "finance.invoice.post",
    CapabilityAliases: []string{"finance.invoice.submit"},  // v1.0 name
    ...
}
```

**Component 2: Compiler includes aliases in `CapabilityManifest`**
```
Capability {
    ID:      "finance.invoice.post"
    Aliases: ["finance.invoice.submit"]
}
```

**Component 3: PolicyEvaluator checks aliases**
When evaluating `"finance.invoice.post"`, the evaluator also checks grants for all aliases. If a grant exists for any alias, it satisfies the check.

**Alias lifecycle:** Aliases are removed in a later version only after the migration tool confirms no grants reference the old ID. The compiler warns when an alias has existed for more than N releases.

**Non-negotiable:** Without this mechanism, any capability rename after v1.0 ships is a silent breaking change. Module developers will be afraid to rename capabilities, accumulating technical debt indefinitely.

### 3.3 Capability Uniqueness — Module Name Collision

The compiler validates local entity name uniqueness within a module but does not validate module name uniqueness across the binary. If two modules both declare `Module: "finance"`, their capability IDs collide silently.

Add to Phase 1 validation: module names must be unique across all registered EntityDefinitions. Failing this check is a fatal compilation error.

### 3.4 Dead Capability Detection

When a capability exists in grants in the database but is absent from the current `CapabilityManifest`, those grants are orphaned. The evaluator should:
- Treat orphaned grants as non-existent (correct security behavior — deny)
- Emit a metric: `orphan_grants_detected{tenant_id, capability_id}` — this surfaces in monitoring

Additionally, the IAM module should run a periodic job that reports orphaned grants. This is not a v1.0 blocker but must be designed as a hook point.

### 3.5 Manifest Schema Version

The `CapabilityManifest` must carry a schema version number. During rolling deployments (old and new pods running simultaneously):
- New pods produce `CapabilityManifest{SchemaVersion: 2, ...}`
- Old pods produce `CapabilityManifest{SchemaVersion: 1, ...}`
- Both share the same Redis grant cache
- Evaluators must be forward-tolerant of manifests with unknown fields

The manifest schema version is a monotonically increasing integer embedded in the struct. Evaluators must handle capabilities missing from their manifest (treat as Deny, log a warning). Evaluators must handle extra fields in grant cache entries (ignore unknown fields).

### 3.6 Cross-Module Capability References

A business module handler may call `viewer.CanDo("iam.user.create")` to check an IAM capability. This is a string literal with no compile-time validation. If IAM renames this capability, Finance silently breaks.

Two-part solution:

**Part 1: Capability ID as exported constant**
Each module exports its capability IDs as typed string constants in a sub-package:
```go
// internal/platform/iam/caps/caps.go
package iamcaps
const UserCreate = "iam.user.create"
```

Business modules import and reference these constants. A rename requires updating the constant definition and all importers — caught by the Go compiler.

**Part 2: Compiler validates capability references**
The compiler receives all registered EntityDefinitions. For every `ActionDef.Capability` string and every string passed to `viewer.CanDo()` in hook/handler code, the compiler validates the string exists in the `CapabilityManifest`. This requires either static analysis tooling or convention enforcement.

At v1.0: the constants convention is enforced by code review. Compiler validation is v1.1.

### 3.7 Manifest as Single Authorization Source of Truth

**Yes, the compiler should be the single source of truth for authorization metadata.** The `CapabilityManifest` becomes the authoritative record of all capabilities that exist in the system. The IAM module reads this manifest at startup to:
- Validate existing grants against known capabilities (detect orphans)
- Apply bootstrap defaults for capabilities with no grants
- Expose the manifest via `/api/v1/platform/capabilities` endpoint

The manifest is embedded in the binary (not fetched from a remote source). For offline inspection, the binary must support `awo manifest export --output capabilities.json`.

### 3.8 Incremental Compilation

Full compilation on startup is acceptable at v1.0. At 100+ modules with 1000+ entities, compilation might add 500ms–2s to startup time. This is acceptable for a restart-to-deploy model.

The fingerprint mechanism (already implemented) enables cache invalidation without recompilation. The fingerprint should be included in the `CapabilityManifest` header so any consumer can detect version changes.

---

## 4 Framework Layer Assessment

### 4.1 `PolicyEvaluator` Interface — Stability Requirement

This is the highest-value abstraction in the framework. Its method signature must be frozen at v1.0 and treated as a public API. Breaking this interface requires a major version bump.

The interface must accept an extensible request struct (not variadic arguments or individual parameters):

```
PolicyEvaluator interface {
    Evaluate(ctx, AuthzRequest) (AuthzDecision, error)
    // EvaluateWithExplanation added in v1.5:
    // EvaluateWithExplanation(ctx, AuthzRequest) (AuthzDecision, Explanation, error)
}
```

Adding `EvaluateWithExplanation` as a separate method (rather than modifying `Evaluate`) allows v1.0 evaluator implementations to not implement it — the framework calls `Evaluate` by default and only calls `EvaluateWithExplanation` if the implementation satisfies the extended interface (via type assertion).

`AuthzRequest` is a struct — new fields can be added without breaking existing implementations that ignore unknown fields. `AuthzDecision` is likewise a struct.

### 4.2 `AuthzRequest` — Extensibility Fields

At v1.0, `AuthzRequest` contains:
- `Principal` — who
- `CapabilityID` — what capability
- `Resource` — optional (nil for collection operations)

Fields reserved for future use (present at v1.0 as optional, zero-valued):
- `SessionCtx *SessionContext` — MFA status, auth method, session age (Zero Trust, v1.3)
- `RequestCtx RequestContext` — IP, timestamp, request ID (always present but not evaluated by v1.0 evaluator)

This future-proofs the struct without requiring a breaking change when Zero Trust is added.

### 4.3 `ViewerContext` Public API Freeze

The methods exposed by `ViewerContext` to business modules must be frozen at v1.0:

```
CanDo(capabilityID string) bool
CanDoOnResource(capabilityID string, r Resource) bool
RecordFilter(entityName string) Filter
VisibleFields(entityName string) []string        // returns all at v1.0
PrincipalID() uuid
TenantID() uuid
RequestID() string
```

These are the stable public methods. Internal state (evaluator, resolved identity, org memberships) is unexported. Business module code that type-asserts to an internal type to access unexported fields is an architectural violation.

### 4.4 The `Resource` Struct

`Resource` is passed to `CanDoOnResource` and embedded in `AuthzRequest`. It must carry enough information for org-scope evaluation and (eventually) ABAC condition evaluation:

```
Resource {
    EntityName  string
    RecordID    *uuid           // nil for collection operations
    OrgUnitID   *uuid           // nil if entity has no org scope
    Attributes  map[string]any  // field values for ABAC (empty at v1.0)
}
```

At v1.0, the evaluator uses only `OrgUnitID` for scope checking. `Attributes` is populated but unused by the evaluator. This is the ABAC extension point — when grant conditions are introduced (v1.2), the evaluator evaluates conditions against `Attributes`.

Populating `Attributes` for v1.0 is optional but the infrastructure must exist. For collection-level checks, `Resource` is nil.

### 4.5 `AuditEmitter` — Non-Negotiable Contract

Every authorization decision — allow or deny — must be emitted as an audit event. This is not optional, not configurable, and not performance-optimizable by disabling. It must be:

- **Asynchronous:** The critical path does not wait for the audit write to complete. The emitter enqueues the event and returns immediately.
- **Durable:** Events go to a write-ahead buffer (outbox table or in-memory queue with crash recovery). If Redis and the outbox both fail, the decision is still logged inline (synchronous fallback) because losing audit records is a compliance violation.
- **Ordered:** Audit events for a single request must be written in causality order (deny before the handler even runs; allow + outcome after the handler completes).

The audit event contains:
- RequestID, TenantID, PrincipalID
- CapabilityID, ResourceEntityName, ResourceID
- Decision (allow/deny)
- Reason string (matched grant ID, or "no matching grant")
- Timestamp, evaluation duration

Missing: **decision tracing** (full explanation of which rules were checked and why). This is the most common operational debugging need. At v1.0, the Reason string is sufficient. At v1.5, add structured Explanation.

---

## 5 IAM Layer Assessment

### 5.1 Bootstrap Configuration — Ownership and Evolution

The IAM module's `bootstrap.go` applies default role-capability grants when a new tenant is provisioned. This is the correct location (not business modules). However, `bootstrap.go` will grow significantly as more modules are added.

**Governance concern:** As Finance, HR, CRM, Inventory, and Payroll modules are added, the IAM bootstrap config becomes the central registry of "which roles get which capabilities by default." This file will be touched by every module addition and every capability change. Without governance, it becomes a coordination bottleneck.

Solution: Each module declares a `BootstrapContributor` interface implementation that registers its default capability grants. The IAM bootstrap process calls all registered contributors in deterministic order. This distributes the configuration back to each module (where it is closer to the capability declarations) without coupling the module to IAM entities.

Wait — this creates an apparent contradiction: modules should not declare grants. But bootstrap contributors are not grants to specific roles. They are grant *templates* — declarations like "the module-local admin role should have capability X." The IAM module translates these templates into actual grants after resolving role IDs.

The key distinction: a `BootstrapContribution` is `{local_role_name: "admin", capability: "finance.invoice.create"}` — it references a local role name from a known vocabulary, not a global role UUID. The IAM module maps local role names to the tenant's actual roles at bootstrap time. This maintains the separation.

### 5.2 Grant Evaluation Algorithm

**High severity.** The naive grant evaluation algorithm — "load all capabilities for all roles, store as a flat set per role in Redis, then check membership" — does not scale.

At 100,000 capabilities per role (unusual but possible with wildcards or large modules):
- Redis SET with 100K members for one role: ~5MB per role entry
- A user with 10 roles: 50MB of Redis memory per active user
- At 1,000 concurrent users: 50GB Redis memory → not viable

The correct algorithm for the default evaluator:

**Option: Per-capability indexed lookup**

When evaluating capability `X` for a principal with roles `[R1, R2, R3]`:
```sql
SELECT role_id, org_constraint, valid_from, valid_until
FROM iam_role_capability
WHERE role_id = ANY(ARRAY[R1, R2, R3])
  AND capability_id = X
  AND (valid_until IS NULL OR valid_until > NOW())
```

Redis cache key: `grant:{tenant_id}:{sorted_role_ids_fingerprint}:{capability_id}` → `{allow, org_constraint, matched_role_id}`

This caches the result of the above query per (role set, capability) pair. Cache miss = one DB query. Cache hit = one Redis GET.

The `sorted_role_ids_fingerprint` is a SHA-256 or FNV hash of the sorted role ID list. This prevents N×M cache entries (one per role per capability) while keeping entries bounded.

**Worst-case complexity:**
- 10 users: O(1) Redis per capability check (warm cache)
- 1,000 users: O(1) Redis per capability check
- 100,000 users: O(1) Redis per check, O(number of concurrent cache misses × DB query latency) for cache warming
- 10 million grants: The grants table has 10M rows but the query indexes on `(role_id, capability_id)` — O(log N) DB lookup on cache miss
- 100 million grants: Same — the index makes per-lookup cost O(log N). Total table size doesn't matter if the index is correct.

**The index is the key performance guarantee.** The `iam_role_capability` table must have a composite index on `(tenant_id, role_id, capability_id)`. Without this index, every capability check scans the full table. This must be in the migration file, not a post-deployment concern.

### 5.3 Org Scope Evaluation

Org scope evaluation for grants must use materialized paths. The `platform_org_unit` table must carry a `path` column: a string like `/00000000-root/abc123-nairobi/def456-westlands/`. This is a closure table approach.

Org scope check for "is resource's org unit in actor's authorized subtree?":
```sql
-- resource is in actor's subtree if the resource's path starts with actor's authorized unit path
resource_path LIKE (actor_authorized_unit_path || '%')
```

This is an O(1) string comparison after fetching the two path strings. The org unit paths are cached in Redis (key: `org:{tenant_id}:{org_unit_id}`) with a 24-hour TTL (org structure changes rarely). A path lookup for the resource is O(1) from cache.

**The actor's authorized org units per grant must be pre-resolved during identity resolution.** When building `ResolvedIdentity`, for each grant that has an org constraint, fetch the org unit path and store it in the resolved identity. This way, at evaluation time, the org check is a string prefix comparison with no DB round trip.

### 5.4 Deny Rule Precedence

Deny rules must be loaded and checked before grant evaluation. The evaluation algorithm:

```
1. Load deny rules for: {principal_id} and {role_id IN actor_roles}
   → Cache key: deny:{tenant_id}:{principal_id}
   → Include role-level denies in this cache entry

2. For the requested capability X:
   a. Filter deny rules by capability_id = X
   b. Check org scope (same as grant scope check)
   c. Check temporal validity
   d. If any deny rule matches → DENY immediately, skip grant evaluation

3. Load grant for (actor_roles, X):
   → Cache key: grant:{tenant_id}:{role_fingerprint}:{capability_id}

4. If no grant matches → DENY
5. Check org scope on matched grant → if scope fails → DENY
6. Check temporal validity on matched grant → if expired → DENY
7. → ALLOW
```

The deny rule cache entry includes all deny rules for a principal (direct and role-based). In practice this is a small set (<10 entries per principal). A single Redis GET returns the full deny list for a principal, then the evaluation filters in-memory.

### 5.5 Role Inheritance Expansion

If a role inherits from a parent role, the parent role's capabilities are implicitly inherited. How is this reflected in grant evaluation?

Option A: Expand inherited roles at identity resolution time. When building `ResolvedIdentity.DirectRoles`, walk the inheritance graph and include all ancestor role IDs. The grant lookup uses all role IDs (direct + inherited) in the IN clause.

Option B: Recursive SQL query at evaluation time. This is expensive and cannot be cached without knowing the full inheritance chain.

Option A is correct. The inheritance graph is walked once at identity resolution and the expanded role list is cached as part of `ResolvedIdentity`. Cache invalidation: when role inheritance is changed, invalidate the affected principals' identity caches.

The inheritance graph is shallow in practice (max 3-4 levels). Walking it at identity resolution is O(depth × breadth) — negligible.

### 5.6 Service Account Grant Scope Enforcement

A service account token should not have more grants than the human who created the service account. This "privilege escalation prevention" is a correctness requirement.

At v1.0: this constraint is enforced by code convention (the service account creation workflow checks the creating principal's grants). No automated enforcement in the framework.

At v1.1: add enforcement in the IAM module's service account creation handler. A service account cannot be granted capability X by principal P if P does not itself hold capability X. This check is `viewer.CanDo(X)` during service account creation — simple.

---

## 6 Repository Assessment

### 6.1 Filter Injection Protocol

The `EntityRepository` interface must clarify exactly how `RecordFilter` predicates from authorization are composed with caller-specified query filters.

The canonical composition:
1. Authorization filter (from `ViewerContext.RecordFilter(entityName)`) — injected by the framework
2. Caller filter (business logic predicates, e.g., `filter.Eq("status", "Draft")`) — passed by the handler
3. RLS (PostgreSQL-level) — enforced by the DB engine

Composition: (1) AND (2), then RLS applied on top automatically.

**The critical constraint:** The authorization filter must be composed in the repository base implementation, not in each concrete repository's `Query` method. If each repository implementation must manually call `ViewerContext.RecordFilter()` and AND it with its own filter, it will be forgotten. It must be automatic.

The repository base class (or generic implementation) must:
```
func (r *BaseRepo) Query(ctx, callerFilter, opts):
    authzFilter = runtime.ViewerFromContext(ctx).RecordFilter(r.entityName)
    effectiveFilter = filter.And(authzFilter, callerFilter)
    return r.driver.Query(ctx, effectiveFilter, opts)
```

This design makes forgetting authorization impossible for repository implementations.

### 6.2 Filter-First, Paginate-Second Invariant

Pagination must be applied after the authorization filter is composed. The repository must never:
1. Fetch records first
2. Then apply authorization filter
3. Then paginate

The SQL must be: `SELECT * FROM entity WHERE [authz filter] AND [caller filter] ORDER BY [sort] LIMIT [page] OFFSET [offset]`.

If the framework generates SQL, this order must be enforced by the query builder. If using a raw query approach, this must be documented as a hard invariant with a test that catches violations.

### 6.3 Cascading Visibility — Unresolved Gap

**High severity.** A parent entity (Invoice) has an authorization RecordFilter. A child entity (InvoiceItem) does not. A query directly on `invoice_items` returns all items across all invoices, including items from invoices the current user cannot see.

Three approaches:

**Approach A: Explicit RecordFilter per entity.** Invoice items declare their own RecordFilter: "only visible if parent invoice is visible." Expressed as a subquery: `WHERE invoice_id IN (SELECT id FROM finance_invoice WHERE [invoice authz filter])`.

Problem: this requires module developers to explicitly declare edge-visibility rules. Easy to forget.

**Approach B: Compiler-derived cascade.** For every `EdgeDef` with a parent entity that has a SystemRecordFilter, the compiler automatically derives a cascading filter for the child entity.

Problem: complex compiler logic; edge cases with ManyToMany and polymorphic edges.

**Approach C: Edge-load-only child access.** Child entities can only be queried via the parent's edge (loaded as part of the parent query), not directly. Direct queries on child entities require an explicit authorization exception.

**Recommendation:** Approach C for v1.0 (simplest, most secure by default), with Approach A available for explicit override. A direct query on `invoice_items` without loading via the `invoice` edge requires the caller to explicitly declare that they intend to bypass cascade visibility. This is detected by linting or convention.

### 6.4 Repository Agnosticism

The current design targets PostgreSQL exclusively. The `EntityRepository` interface deliberately excludes ORM types, raw SQL, and connection management. This is correct.

However, the `Filter` type used by `RecordFilter` must be repository-agnostic. If `Filter` is a PostgreSQL-specific predicate type, swapping to a different database requires changing the `Filter` interface.

The `Filter` is currently defined as an opaque interface (`type Filter any`). The concrete implementation in `awo/filter` may be PostgreSQL-specific. For true repository agnosticism, the Filter DSL must have a translation layer that converts logical predicates (Eq, In, Like, etc.) to database-specific SQL. This translation must be in the repository driver, not in the Filter DSL itself.

At v1.0: the Filter DSL can be PostgreSQL-specific since PostgreSQL is the only supported database. But the interface definition must be generic. The concrete type must be in a driver-specific package, not in the core Filter DSL. This prevents coupling.

### 6.5 Aggregations

When a user queries `COUNT(*)` or `SUM(amount)`, the authorization RecordFilter must apply. The `EntityRepository.Aggregate()` method receives the same filter composed by the base repository. The filter applies to the WHERE clause before aggregation. This is correct by design — the aggregation functions operate on the filtered result set.

Risk: if the framework offers a "bypass aggregation filter" option for reporting, this would bypass row-level visibility. No such bypass may exist. Reports must apply the same authorization filters as regular queries.

The exception: platform admin reports spanning all tenants. These use the `SystemViewerContext` and have their behavior logged in the audit trail.

---

## 7 SDUI Assessment

### 7.1 Schema Caching Cannot Be Per-Viewer

The SDUI schema cache currently has key: `page:{entity}:{view}:{tenant_id}`. After the redesign, schemas must reflect the viewer's capabilities (which buttons are visible, which fields are shown). A single cached schema cannot serve all viewers in a tenant — different users have different capabilities.

**High severity.** The current design implies one schema per tenant per view. The correct design requires either:

Option A: One schema per role combination (not per user). Users with identical role sets see identical schemas. Cache key: `sdui:{entity}:{view}:{schema_fingerprint}:{role_set_fingerprint}`. This is bounded: N schemas where N = distinct role combinations in use.

Option B: Two-level rendering. Cache the base schema (entity structure, no authorization). At serve time, apply a "capability mask" that removes unauthorized elements. The mask is cheap to compute from the ViewerContext. The final schema is not cached.

Option B is correct for v1.0. Base schema is cached (same for all users). The capability mask is applied per-request using `ViewerContext.CanDo()` for each capability referenced by a UI element. This adds 5–20ms per schema serve (N calls to CanDo where N = number of capability-gated elements in the schema). These CanDo calls hit the grant cache (Redis), so the added latency is bounded by Redis round trips.

### 7.2 SDUI Leaks Entity Structure

**Current bug.** The schema endpoint currently returns the full schema without checking that the requesting viewer has any capability for the entity. A user with no access to `finance_invoice` can request the `finance_invoice` create form schema and see the complete field structure.

This is an information leak. Fields marked as `Sensitive` are excluded, but the field names, types, and options of non-sensitive fields are visible to unauthorized users.

Fix: before generating or serving any schema, check `viewer.CanDo("{module}.{entity}.read")`. If the viewer doesn't have at least read access, return 403 (not 404 — the entity exists; the viewer just has no access). Field-level sensitivity exclusion continues to apply.

### 7.3 SDUI and Capability-Gated Actions

SDUI schemas include action buttons (`toolbar` items, inline actions on rows). These buttons must be absent from the schema for viewers who lack the required capability. This requires:
- Each `ActionDef` declares `Capability`
- The SDUI generator, when building the action button for `submit`, calls `viewer.CanDo("finance.invoice.submit")`
- If denied, the button is absent from the schema (not disabled — absent)

This is the "absent, not disabled" principle. An absent button cannot be manipulated by the client to trigger an API call (the API-level check is still authoritative, but absent buttons prevent confusion).

### 7.4 Client Caching of SDUI Schemas

If a client caches an SDUI schema with an "approve" button visible, then the user's approval capability is revoked, the client still shows the button until the schema cache expires. The API will reject the invocation, but the stale UI creates confusion.

Mitigation: SDUI schema responses include `Cache-Control: max-age=300, must-revalidate`. The schema fingerprint is included as an ETag. When the capability grant for a principal changes, the IAM module publishes a server-sent event (SSE) to that principal's browser session, triggering a schema re-fetch. This is a v1.1 feature. At v1.0, the 5-minute TTL is acceptable.

---

## 8 Performance Assessment

### 8.1 Request Latency Breakdown (Warm Path)

Expected latency for a standard CRUD request after authorization integration:

| Stage | P50 | P95 | Notes |
|---|---|---|---|
| Session → Principal | 0.5ms | 2ms | Redis GET; single round trip |
| Identity resolution | 0.3ms | 1ms | Redis GET; cached per session |
| ViewerContext construction | 0.1ms | 0.3ms | In-memory assembly |
| Operation permission check | 0.5ms | 2ms | Redis GET; grant cache |
| RecordFilter loading | 0.3ms | 1ms | Redis GET; rule cache |
| Business handler | varies | varies | Application logic |
| Repository query (with filter) | 2ms | 15ms | PostgreSQL; depends on index use |
| Response serialization + field strip | 0.2ms | 1ms | In-memory |
| Audit emission (async) | ~0ms | ~0ms | Non-blocking enqueue |

**Authorization overhead on warm path: ~2ms P50, ~6ms P95**

This is acceptable. The authorization overhead is less than 10% of a typical response time.

### 8.2 Cold Start Latency

On cache miss (new session, first request after deployment, cache eviction):

| Stage | Cold path latency |
|---|---|
| Session validation (cache miss → DB) | 5–15ms |
| Identity resolution (cache miss → DB) | 10–30ms (2–4 queries) |
| Grant lookup (cache miss → DB) | 5–15ms (1 query, indexed) |
| RecordFilter loading (cache miss → DB) | 5–10ms (1 query) |

**Cold path overhead: ~25–70ms**

Under burst traffic (post-deployment, cache empty), many requests simultaneously hit cold paths. With 100 concurrent requests, each needing 4 DB queries, that's 400 queries hitting the database simultaneously. PgBouncer's connection pool must be sized for this burst.

**Mitigation:**
1. Redis `SET NX` for cache population (only one goroutine populates a given key)
2. Jitter (±10%) on all cache TTLs (prevents simultaneous mass expiry)
3. Pre-warming: on deployment, the framework pre-populates caches for the last N active sessions

### 8.3 P99 Latency Concerns

At P99, authorization adds:
- Grant cache miss: +50ms DB query
- Identity cache miss: +30ms DB query
- Org tree traversal (cache miss): +20ms DB query

**Total P99 authorization overhead: ~100ms on triple cache miss**

This is a tail latency problem. At moderate scale (1,000 concurrent users), a triple cache miss affects ~10 requests per second. At high scale (100,000 concurrent users), it affects ~1,000 requests per second — significant.

The mitigation is layered caching with different TTLs and pre-warming. The org tree cache has a 24-hour TTL (cache misses are extremely rare). The identity cache has a 5-minute TTL (synchronized with session TTL). The grant cache has a 60-second TTL. Simultaneous expiry is prevented by TTL jitter.

### 8.4 Memory Hotspots

**Grant cache in Redis:** If using role-level flat expansion (all capabilities per role as a Redis SET), memory usage is O(capabilities_per_role × roles_in_system). At 1,000 capabilities per role × 100 roles × 50 bytes per entry = 5MB. Manageable.

If roles grow to 10,000 capabilities per role, this is 500MB — not manageable. The per-capability indexed lookup (Section 5.2) avoids this by caching per (role_fingerprint, capability) pair instead of per role.

**ResolvedIdentity in Redis:** A user with 20 roles, each with 50 capabilities — the identity cache stores 20 role IDs (320 bytes) plus org membership data (small). The grant data is NOT stored in the identity cache; it is fetched separately per capability check. This keeps the identity cache small.

**In-memory per-request cache:** Within a request, `CanDo()` results are cached in a map on the ViewerContext. This map grows with the number of distinct capability checks per request. In a typical request (3–10 capability checks), this is negligible.

### 8.5 Database Round Trips Per Request (Worst Case)

Worst case (triple cold start, no Redis):
1. Tenant status check: 1 query
2. Session validation: 1 query
3. Identity resolution: 1 query (roles) + 1 query (org memberships)
4. Deny rule loading: 1 query
5. Grant lookup: 1 query
6. RecordFilter loading: 1 query
7. Main entity query: 1 query (with injected filter)
8. Audit write: 1 query (async)

**Total: 8 DB round trips worst case, 1 round trip (entity query) warm case.**

With PgBouncer in transaction mode, each round trip costs ~1ms connection acquisition. Total worst-case DB overhead: ~40ms.

This is acceptable. The 1 warm-path round trip (entity query only) means authorization is effectively free in steady state.

### 8.6 Authorization Latency Target

**Requirement:** Authorization overhead must be less than 5ms P95 on the warm path.

Based on the analysis above:
- Warm path P95: ~6ms — slightly over target
- The overage is one Redis round trip (2ms) for the grant cache

To achieve 5ms P95: implement the per-request CanDo cache (first CanDo for a capability pays 2ms Redis; subsequent calls within the same request are free). With this optimization, warm path P95 is ~4ms for requests with 1–5 distinct capability checks.

---

## 9 Scalability Assessment

### 9.1 Grant Table Scaling

The `iam_role_capability` table grows as:
- Roles × Capabilities per role × Tenants

At 100 tenants × 50 roles × 500 capabilities = 2,500,000 rows. This is trivially small. Even at 10,000 tenants × 50 roles × 500 capabilities = 250,000,000 rows — manageable with proper indexing.

The critical index: `(tenant_id, role_id, capability_id)` covering index. With this index, the per-capability grant lookup is a B-tree seek (O(log N)) regardless of table size.

### 9.2 Org Hierarchy Scaling

A tenant with 10,000 org units (large enterprise) needs:
- A materialized path of at most ~100 characters per node
- A subtree query using LIKE prefix matching on the materialized path

This scales to 1,000,000 org units before becoming a concern. The ERP use case rarely exceeds 1,000 org units.

### 9.3 Deny Rule Scaling

Deny rules are exceptional — most users have zero active deny rules. The deny rule evaluation adds 1 Redis GET per request. The deny list is small (typically 0–5 entries). This scales trivially.

### 9.4 Record Rule Scaling

Record rules per entity per tenant: typically 1–5. Loading them from cache is 1 Redis GET returning a small JSON array. This scales trivially.

### 9.5 Scaling Bottlenecks at 10 Million Users

At 10 million users:
- Redis session store: 10M sessions × ~500 bytes each = 5GB. Large but manageable with Redis Cluster.
- Redis identity cache: 10M entries × ~2KB each = 20GB. Requires Redis Cluster.
- Redis grant cache: not per-user, but per (role_fingerprint, capability). With 10,000 distinct role combinations × 500 frequently-checked capabilities × ~100 bytes each = 500MB. Manageable.

The grant cache key design (per role-fingerprint, not per user) is what makes this scale. Per-user grant caches would be 10M × 500 capabilities × 100 bytes = 500GB — not viable.

### 9.6 Multi-Tenant Scaling

With 100,000 tenants:
- Each tenant has its own Redis namespace (key prefix includes tenant_id). No interference.
- Each tenant's records are RLS-isolated in PostgreSQL. No interference.
- The capability manifest is shared (global). No per-tenant overhead.

The per-tenant costs are entirely in the tenant's own data volume. Tenants with 5 users contribute negligibly. Tenants with 100,000 users contribute proportionally. This is correct multi-tenant behavior.

---

## 10 Failure Mode Assessment

### 10.1 Redis Unavailable

**Behavior:** Authorization falls through to the database for all cache misses. Performance degrades significantly (8 DB round trips per request instead of 1). The system remains functionally correct — all requests are still authorized correctly, just slowly.

Under sustained Redis outage with high traffic, the PostgreSQL database pool may become exhausted. This triggers a secondary failure: 503 responses from connection pool exhaustion. This is the correct failure mode (reject requests rather than serve unauthorized responses).

**Last-resort in-memory cache:** To survive brief Redis outages (5–30 seconds, e.g., Redis restart or failover), each process node maintains a bounded in-memory LRU cache of recent authorization decisions. TTL: 30 seconds. Max size: 10,000 entries per process. This cache is populated from Redis and serves as a warm fallback. After 30 seconds without Redis, entries expire and the system starts querying the database for every request.

This in-memory cache must be keyed on the same key scheme as Redis. It must be documented as a degradation layer, not a reliability guarantee.

### 10.2 Database Unavailable

**Authentication stage:** Cannot validate sessions that are not cached in Redis. New sessions (not recently validated) fail with 503. Existing Redis-cached sessions continue to be validated by Redis for the session TTL duration.

**Authorization stage:** Cannot load grants for cold cache. The in-memory last-resort cache (30-second TTL) serves recent decisions. After 30 seconds, fail closed (deny all for uncached decisions).

**Critical:** PostgreSQL unavailability must never cause fail-open authorization. If neither the cache nor the database can confirm a grant, the decision must be Deny.

### 10.3 Grant Cache Stale

A user's role is revoked. The grant cache TTL is 60 seconds. For 60 seconds, the user retains access.

**Immediate revocation path:** When a role is revoked by an admin action, the IAM module:
1. Writes the revocation to the database
2. Publishes a cache invalidation message to a Redis pub/sub channel
3. All process nodes subscribe to this channel and immediately evict the affected principal's grant cache entries
4. The principal's next request re-evaluates from the database

This pub/sub invalidation reduces the stale window from 60 seconds to the round-trip latency of the pub/sub message (~1ms). It is not guaranteed (the subscriber might be down), which is why the 60-second TTL also exists as a fallback.

### 10.4 Capability Removed After Deployment

A capability ID is removed from the `CapabilityManifest` during a deployment (e.g., action was deleted).

- Existing grants for the removed capability are orphaned in the database.
- The evaluator encounters a capability ID not in its manifest.
- Correct behavior: treat as Deny. Log a warning: `capability_not_in_manifest{capability_id}`.
- The evaluator must not panic or error on unknown capability IDs.

**Rolling deployment window:** During a rolling deploy (old pods + new pods), old pods know about the removed capability, new pods do not. For requests routed to new pods: the evaluator returns Deny (because the capability is not in the manifest, so no grant can be found). For requests routed to old pods: the evaluator evaluates normally.

To avoid this inconsistency: capabilities must be deprecated before removal (see Section 3.2 for the alias mechanism). Removal of a capability should span two releases: mark as deprecated in release N, remove in release N+1.

### 10.5 Manifest Corrupted at Startup

The binary refuses to start. This is the correct behavior. A process with a corrupted manifest cannot know what capabilities exist or which routes are registered. Starting in a partial state creates unpredictable authorization behavior.

Startup validation of the manifest is mandatory: verify fingerprint, verify all capability IDs are syntactically valid, verify all route capability references exist in the capability list.

### 10.6 Tenant Disabled Mid-Request

The tenant's status changes from ACTIVE to SUSPENDED while a request is in-flight (past Stage 2 — tenant validation). The in-flight request completes. The response is returned to the client.

The tenant status cache (1-minute TTL) is immediately invalidated via pub/sub when the status changes. The next request to any pod for that tenant is rejected at Stage 2 with 402 (Suspended).

This creates a window of up to 1 minute plus one request where a suspended tenant's users can complete in-flight requests. This is operationally acceptable — the alternative (aborting in-flight requests) risks data corruption (mid-transaction abort).

### 10.7 Role Deleted During Request

A role is deleted while a user with that role has an in-flight request. The ViewerContext was constructed at the start of the request with the role's grants. The request completes with the stale ViewerContext. The next request re-evaluates; the role is gone.

This is identical to the grant cache staleness window. Acceptable for ERP use cases.

### 10.8 Policy Evaluator Crashes

If the PolicyEvaluator implementation panics:
- Panic recovery (middleware stage 3) catches the panic
- Returns 500 Internal Server Error
- Logs the panic with full stack trace
- Does NOT fail open — authorization failure returns 500, not a successful response

The evaluator must be tested with adversarial inputs (nil resource, empty capability, unknown capabilities) to ensure it does not panic in production.

---

## 11 Future Evolution Assessment

### 11.1 What Requires No Framework Changes

- Adding ABAC grant conditions (null `Conditions` column promoted to evaluated)
- Adding field-level capabilities (`FieldDef.ReadCapability` added as optional)
- JIT access workflows (time-bounded grants via Temporal)
- Delegation workflows (grants created by workflow on behalf of delegator)
- Break-glass workflows (emergency time-bounded grants with approval)
- External IDP integration (authentication layer only; authorization unchanged)
- Multi-company org hierarchy (org tree is already a tree; multiple roots are additive)
- Policy simulation endpoint (`EvaluateWithExplanation` on the evaluator — type-assertion extension)

### 11.2 What Requires Additive Framework Changes (No Breaking)

**Zero Trust session context:** `AuthzRequest.SessionCtx` field added (zero-valued at v1.0). Evaluator begins checking session context when v1.3 is deployed. Business modules unchanged.

**ReBAC engine replacement:** `PolicyEvaluator` implementation is swapped. The IAM data model (iam_grant, iam_role, iam_deny) is reinterpreted as Zanzibar-style relationship tuples by the new evaluator. No business module changes. The public API contracts (AuthzRequest/AuthzDecision) are preserved.

**Offline authorization:** A manifest export tool is added. An offline evaluator implementation reads from the exported manifest + cached grant snapshot. The `PolicyEvaluator` interface is unchanged.

**Distributed authorization (remote OPA/Cedar):** A new `PolicyEvaluator` implementation makes gRPC calls to an external engine. Serialization of `AuthzRequest` and deserialization of `AuthzDecision`. No interface changes.

### 11.3 What Requires Breaking Changes

**Only one breaking change scenario was identified:** If the `AuthzRequest` struct needs a field that cannot be zero-valued backward-compatibly (e.g., a required interface with no sensible zero value). This is avoidable by designing all `AuthzRequest` fields as optional (pointer types or types with sensible zero values).

**Recommendation:** Every field added to `AuthzRequest` in future versions must have a sensible zero value that preserves backward compatibility. Document this as a framework invariant.

---

## 12 Hidden Coupling Detected

### 12.1 IAM Module ↔ Business Module via Bootstrap String Matching

If the IAM bootstrap config maps local role names to capability IDs using string matching on capability ID prefixes (e.g., "all capabilities starting with `finance.`"), this creates implicit coupling between IAM's bootstrap logic and Finance's module name. If Finance renames its module, the bootstrap config silently stops applying Finance capabilities.

Solution: bootstrap contributors (Section 5.1) eliminate this by having each module declare its own default grant templates, eliminating the need for IAM to guess capability namespaces.

### 12.2 `PolicyFunc` Captures Session Package at Declaration Time

Existing `PolicyFunc` closures capture the `session` package (to call `session.ActorFromContext(ctx)`). This creates a compile-time dependency from EntityDefinition (in `def` package) to the `session` package (in the IAM or middleware layer). If the session package changes, all `PolicyFunc` declarations need updating.

The `SystemRecordFilter` design (declarative metadata, no closure) eliminates this coupling entirely. The deprecation of `PolicyFunc` is therefore also a coupling removal, not just an architectural preference.

### 12.3 `FieldDef` Comment References Casbin

`awo/def/field.go` comments mention "Casbin policy scope." This couples the field documentation to a specific authorization implementation. Update the comment to describe "the entity's permission namespace" without naming Casbin.

### 12.4 `RouteDescriptor.RequiredPermission` as String

`RouteDescriptor.RequiredPermission` is currently a string like `"read"` or `"submit"`. After the redesign, this should be a full Capability ID like `"finance.invoice.read"` or `"finance.invoice.submit"`. Keeping it as a short action string (`"read"`, `"write"`) requires the IAM middleware to reconstruct the full Capability ID from the entity name + action string — this is redundant computation that introduces a coupling between the middleware and the capability ID naming convention.

Fix: change `RouteDescriptor.RequiredPermission` to `RouteDescriptor.RequiredCapability` containing the full Capability ID. The compiler emits this from the entity module/name/operation combination.

### 12.5 `sdui/generator.go` Hardcodes `/api/v1/entities/{tableName}`

The SDUI generator uses `es.TableName` in API URLs (`/api/v1/entities/{tableName}`). This couples the SDUI layer to the current route structure. After redesign, routes follow `/api/v1/{module}/{resource}`. The SDUI should use `es.RoutePrefix` (already available on `EntitySchema`) rather than constructing its own URL from the table name.

This is also a correctness bug: the current URL format doesn't match the actual route prefix format.

### 12.6 `iam_record_rule.IsSystemSeeded` as a Boolean

Distinguishing system-seeded rules from tenant-created rules with a boolean is too coarse. A tenant admin cannot modify system-seeded rules, but they can add to them (AND-composition). If a tenant admin wants to see why a record is not visible, they need to know which rule is responsible — the system-seeded rule or their own custom rule.

A better approach: `iam_record_rule.Source` as an enum: `system_compiled` (from CapabilityManifest), `platform_seeded` (from IAM bootstrap), `tenant_configured` (created by tenant admin). This provides three distinct tiers of record visibility control with clear ownership.

---

## 13 Architectural Violations

### 13.1 Role Names in Business Module Source — Existing Code

The current `internal/core/finance/` module contains `PermissionSet` declarations with `[]string{"role:finance.accounts_payable", ...}`. This is the primary architectural violation. These strings must be removed before v1.0.

Severity: **Critical.** This is the exact violation the canonical architecture was designed to prevent.

### 13.2 `CasbinPolicy` Type Name in Compiler Output

`compiler.CasbinPolicy` names a third-party library in a framework public type. Any consumer of the compiler output (tooling, IAM module, documentation generators) is coupled to the name Casbin even if they never use Casbin.

Severity: **High.** Rename to `PolicyTuple` or remove entirely (the manifest replaces it).

### 13.3 `Actor.IsPlatformAdmin` Bypasses Evaluation Pipeline

The boolean flag makes the platform admin authorization decision before the evaluation pipeline runs. This means platform admin access is not logged in the audit trail at the authorization decision level (it is bypassed, not evaluated). A platform admin who accesses a tenant's sensitive record has no authorization audit event.

Severity: **Critical.** Platform admin must be a grant evaluated by the same pipeline as every other authorization decision. The evaluation path is fast (O(1) grant check) and fully auditable.

### 13.4 SDUI Schema Endpoint Has No Authorization Gate

Any caller can request the SDUI schema for any entity without demonstrating authorization. This reveals entity structure (field names, types, select options) to unauthorized callers.

Severity: **High.** Must be gated before v1.0.

### 13.5 `PolicyFunc` in `PermissionSet.Policy` Not Deprecated

`PolicyFunc` is a Go closure embedded in an EntityDefinition. It is not introspectable, not tenant-configurable, and not auditable. Its continued presence as the primary row-level filtering mechanism is an architectural violation against the principle that all authorization metadata should be compiler-generated and auditable.

Severity: **Medium** (the system works correctly; it's an evolution blocker, not a correctness bug). Must be marked deprecated before v1.0 to signal the migration path.

---

## 14 Recommended Corrections Before v1.0

These are non-negotiable. The system cannot be frozen without them.

### 14.1 [Critical] Remove Role Names from All Business Module Source

**Action:** Replace `PermissionSet` (with string slices) with `CapabilitySet` (with no role references) in all EntityDefinitions in `internal/core/` and `internal/platform/`. Move all default capability-to-role mappings to `internal/platform/iam/bootstrap.go`.

**Validation:** The compiler should refuse to compile any `CapabilitySet` that contains a role-name-shaped string. A linting rule (or the compiler itself) should flag strings matching `role:*` anywhere in EntityDefinition declarations.

### 14.2 [Critical] Implement Capability Alias Mechanism

**Action:** Add `CapabilityAliases []string` to `ActionDef`. Compiler includes aliases in `CapabilityManifest`. PolicyEvaluator checks aliases during grant lookup. Add migration tool command `awo migrate capabilities --from X --to Y` that updates grants in the database.

**Without this:** any capability rename after v1.0 ships is an irreversible silent breaking change for all tenants.

### 14.3 [Critical] Define Execution Identity for Non-HTTP Contexts

**Action:** Define and implement:
- `NewServiceAccountViewerContext(ctx, serviceAccountID)` in framework runtime
- Task queue → service account mapping in Temporal worker configuration
- Convention for Temporal activities: always call `RequireViewer(ctx)` at the start to catch missing identity

**Without this:** Temporal activities, background jobs, and CLI commands have undefined authorization behavior.

### 14.4 [Critical] Replace `Actor.IsPlatformAdmin` with Grant-Based Evaluation

**Action:** Remove the `IsPlatformAdmin bool` field. Create a platform-scoped grant for `platform.admin` capability, held by platform admin principals. The PolicyEvaluator's default implementation recognizes `platform.admin` as a cross-tenant super-capability that satisfies any other capability check. This grant is evaluated through the normal pipeline and therefore logged in the audit trail.

### 14.5 [Critical] Gate the SDUI Schema Endpoint

**Action:** Before generating or returning any entity schema, check that the requesting viewer holds at minimum the `{module}.{entity}.read` capability. Return 403 if not.

### 14.6 [Critical] Rename `CasbinPolicy` to `PolicyTuple`

**Action:** Rename the `compiler.CasbinPolicy` type. This is a pure rename with no semantic change. Update all consumers.

### 14.7 [High] Implement Two-Level SDUI Schema Caching

**Action:** Separate base schema generation (cached, entity-level) from capability filtering (per-request, viewer-level). Base schemas are cached without capability context. Capability filtering is applied at serve time using `ViewerContext.CanDo()`.

### 14.8 [High] Fix SDUI URL Construction

**Action:** `sdui/generator.go` must use `es.RoutePrefix` for API URLs instead of constructing `/api/v1/entities/{tableName}`. The current format doesn't match the actual route structure.

### 14.9 [High] Change `RouteDescriptor.RequiredPermission` to Full Capability ID

**Action:** Rename field to `RequiredCapabilityID`. Change value from `"read"` to `"finance.invoice.read"` (full capability ID emitted by the compiler). Remove the IAM middleware's current need to reconstruct the capability ID from route metadata.

### 14.10 [High] Design the In-Memory Last-Resort Authorization Cache

**Action:** Define and document the bounded in-memory LRU cache for authorization decisions. 30-second TTL, 10,000 entries per process. Populated transparently by the PolicyEvaluator. Served when Redis is unavailable. Properly documented as a degradation layer, not a reliability guarantee.

---

## 15 Non-Blocking Improvements

These are recommended but not required for v1.0 freeze.

### 15.1 Capability ID as Exported Constants

Each module should export capability IDs as typed string constants in a sub-package. This prevents typos in cross-module `CanDo()` calls. Implement as a convention with linting enforcement.

### 15.2 Debug Authorization Endpoint

`GET /api/v1/platform/authz/explain?principal={id}&capability={id}&resource_entity={name}&resource_id={id}` returns a full explanation of the authorization decision. Requires `platform.admin` capability. Logs its use in the audit trail. Essential for tenant configuration debugging.

### 15.3 Telemetry Additions

Add before v1.0 if time permits; mandatory by v1.1:
- `authz_decisions_total{capability, outcome, tenant}` counter
- `authz_decision_duration_seconds{capability, cache_hit}` histogram
- `authz_cache_hits_total{layer}` and `authz_cache_misses_total{layer}` counters
- `orphan_grant_count{tenant}` gauge (grants with no matching capability)

### 15.4 RecordFilter Cascade for Child Entities

Define the convention for cascading visibility: child entities queried via parent edges are automatically visibility-bounded by the parent's RecordFilter. Direct queries on child entities require explicit RecordFilter declarations or a documented exception.

### 15.5 Deny Rule Pub/Sub Invalidation

Implement Redis pub/sub invalidation for grant and deny rule caches. When a role assignment is revoked, invalidate the affected principal's grant cache entries immediately (rather than waiting for TTL expiry). This reduces the revocation window from 60 seconds to ~1ms.

### 15.6 BootstrapContributor Interface

Move default capability grant declarations out of `iam/bootstrap.go` and into per-module `BootstrapContributor` implementations. This distributes the maintenance burden while keeping IAM as the authority.

### 15.7 Grant Table Performance Index

Ensure the migration for `iam_role_capability` includes the composite index `(tenant_id, role_id, capability_id)` as the primary lookup path. This is a correctness precondition for the per-capability batch lookup algorithm. Without it, the grant lookup degrades from O(log N) to O(N) at scale.

---

## 16 Risk Matrix

| # | Issue | Severity | Likelihood | Mitigation | Status |
|---|---|---|---|---|---|
| R1 | Capability rename orphans all grants | Critical | Certain (capability names will evolve) | Capability alias mechanism (§14.2) | Not implemented |
| R2 | Role names in business module source | Critical | Already present | Remove before freeze (§14.1) | In code now |
| R3 | No execution identity for Temporal/cron | Critical | Will manifest on first background job | Service account ViewerContext (§14.3) | Not designed |
| R4 | IsPlatformAdmin bypasses audit | Critical | Exploitable now | Replace with grant (§14.4) | In code now |
| R5 | SDUI schema served without auth check | Critical | Exploitable now | Gate schema endpoint (§14.5) | In code now |
| R6 | CasbinPolicy type in compiler output | High | Coupling exists | Rename (§14.6) | In code now |
| R7 | SDUI cache cannot be per-viewer | High | Will appear on first multi-role tenant | Two-level schema caching (§14.7) | Not designed |
| R8 | SDUI uses wrong URL format | High | Current bug | Use RoutePrefix (§14.8) | In code now |
| R9 | RouteDescriptor.RequiredPermission is partial | High | IAM middleware must reconstruct | Full capability ID (§14.9) | Not designed |
| R10 | No last-resort auth cache for Redis failure | High | Will cause outage on Redis failure | In-memory LRU fallback (§14.10) | Not designed |
| R11 | Grant cache may not scale to 100K caps/role | High | Will manifest if roles accumulate capabilities | Per-capability indexed lookup (§5.2) | Not designed |
| R12 | Cold start DB stampede | Medium | Will occur on every deployment | SET NX, TTL jitter, pre-warm | Not designed |
| R13 | Cascading child entity visibility gap | Medium | Will produce authorization bugs on child queries | Cascade convention (§15.4) | Not designed |
| R14 | No authorization metrics/tracing | Medium | Will appear in operations | Telemetry additions (§15.3) | Not designed |
| R15 | PolicyFunc not formally deprecated | Medium | Evolution blocker | Mark deprecated (§13.5) | Not done |
| R16 | RecordFilter order vs pagination not enforced | Medium | Could produce wrong paginated results | Repository invariant (§6.2) | Not enforced |
| R17 | Cache invalidation too aggressive on role change | Low | Temporary performance degradation | Targeted pub/sub invalidation (§15.5) | Not designed |
| R18 | Bootstrap.go becomes coordination bottleneck | Low | Will appear at 5+ modules | BootstrapContributor interface (§15.6) | Not designed |
| R19 | Manifest schema version absent | Low | Rolling deploy inconsistency | Add schema version (§3.5) | Not designed |
| R20 | Debug endpoint for auth decisions absent | Low | Operational friction | Explain endpoint (§15.2) | Not designed |

---

## 17 Final Readiness Score

Scores reflect the current implementation plan before the corrections in §14 are applied.

| Dimension | Score | Assessment |
|---|---|---|
| **Architecture Correctness** | 6/10 | Canonical design is sound. Five critical violations in the current implementation plan (R1–R5). Score rises to 9/10 after §14 corrections. |
| **Runtime Correctness** | 5/10 | ViewerContext for non-HTTP contexts undefined (R3). Platform admin bypass not auditable (R4). SDUI leak (R5). Score rises to 9/10 after corrections. |
| **Compiler Design** | 6/10 | Capability manifest not a named phase. No alias mechanism (R1). Missing capability uniqueness validation. Score rises to 9/10 after corrections. |
| **Maintainability** | 5/10 | Role names in business module source (R2) are the core maintainability problem. PolicyFunc not deprecated (R15). Score rises to 9/10 after corrections. |
| **Performance** | 7/10 | Warm path acceptable. Cold start design incomplete (R12). Grant cache scale concern (R11) not addressed. Score rises to 8/10 after corrections. |
| **Scalability** | 6/10 | Grant evaluation algorithm not finalized for scale. Per-capability indexed lookup not designed (§5.2, R11). Score rises to 8/10 after corrections. |
| **Developer Experience** | 5/10 | No debug endpoint (R20). No capability constants convention. No formal testing helpers for ViewerContext. Score rises to 7/10 after §15 non-blocking improvements. |
| **Future Evolution** | 8/10 | PolicyEvaluator interface is stable. AuthzRequest is extensible. Additive path clear for all future requirements. One point deducted for missing alias mechanism (R1). Score rises to 9/10 after correction. |
| **Operational Simplicity** | 5/10 | No last-resort cache (R10). No authorization metrics (R14). No debug endpoint (R20). Dependency on Redis without defined failure mode. Score rises to 8/10 after corrections. |
| **Overall Production Readiness** | **5.9/10** | **Not ready for v1.0 freeze in current state.** Six critical corrections (§14.1–14.6) required. After corrections: 8.5/10. The canonical architecture is correct; the implementation plan has gaps. |

---

## Conclusion

The canonical authorization architecture will survive twenty years if the `PolicyEvaluator` interface and `AuthzRequest`/`AuthzDecision` contracts are treated as frozen public API from the moment v1.0 ships. The five core primitives (Capability, Principal, Grant, Deny, RecordFilter) are the right abstraction. The separation of capabilities (framework) from grants (IAM) from business logic (modules) is sound.

What prevents shipping today: the implementation plan has not resolved the six critical gaps. Specifically, the capability alias mechanism (R1) is not optional — the first capability rename after v1.0 will silently break authorization for all tenants with no recovery path other than a hotfix deployment. The absence of execution identity for background contexts (R3) means the first Temporal activity or cron job deployed will operate with undefined authorization behavior.

Both gaps are implementation decisions that were not made during design. Neither requires redesigning the canonical architecture. Both can be resolved in a focused implementation sprint before the freeze.

After §14 corrections are implemented and validated, the framework is ready for v1.0 freeze. The authorization system will be one of the framework's strongest architectural assets — a stable, replaceable, auditable, multi-tenant evaluation pipeline that grows from RBAC at v1.0 to ABAC, ReBAC, and Zero Trust without breaking changes to the public API surface.

---

*End of Implementation Audit*
