# AWO Framework — Phase A Implementation Blueprint
### Principal Engineering Document · Pre-v1.0 Kernel Freeze

---

## PART 1 — Phase A Item Analysis

---

### A1 — Compiler Dependency Graph
**Priority: CRITICAL**

**Architectural Goal**
Compiler must understand the entity graph as a whole before validating any individual entity. A `FieldTypeLink` referencing a non-existent target, or a cycle in FK references, must be a compile-time error — not a migration-time PostgreSQL error.

**Current Deficiency**
Phase 3 (link resolution) checks whether a linked entity name exists in the registry. It does not:
- Build a directed graph of entity dependencies
- Detect cycles (A→B→A)
- Validate that `LabelField`/`ValueField` names exist in the linked entity's fields
- Topologically sort entities for migration emission order

**Implementation Strategy**
Introduce `DependencyGraph` type internal to the compiler. Construction happens after all entity stubs are built (post phase 2), before any cross-entity validation (phase 3).

Three sub-phases within graph construction:

1. **Edge extraction.** Walk every `FieldDef` across all entities. For every `FieldTypeLink`, extract `Options` field as target entity name. Collect as `(source, target, field_name)` tuples.

2. **Cycle detection.** Run Kahn's algorithm (topological sort via in-degree reduction). If any node remains after sorting, a cycle exists. Report the cycle path explicitly.

3. **Cross-entity field validation.** For every `FieldTypeLink` edge, retrieve the target entity schema and verify `LabelField` and `ValueField` names exist in `FieldsByName`.

Topological sort output becomes the canonical entity ordering used by migration generation (Phase B6) and route registration.

**Required Packages**
- `awo/compiler` — owns graph, construction, validation
- `awo/def` — no changes; provides `FieldDef.Options` as link target
- `awo/registry` — provides complete entity list for graph construction

**API Changes**
- `CompiledSchema` gains `DependencyOrder []string`
- `CompileError` gains `CycleChain []string` for cycle reports

**Compiler Changes**
New phases:
```
Phase 2:   Stub generation
Phase 2.5: Dependency graph construction + cycle detection   ← NEW
Phase 3:   Link resolution (now uses graph for ordering)
Phase 3.5: Cross-entity field reference validation           ← NEW
```

**Runtime Changes:** None.

**Migration Impact**
`DependencyOrder` in `CompiledSchema` becomes authoritative emit order for migration generation (Phase B6). Preparatory — migration generation is Phase B6 but data captured here.

**Backward Compatibility**
Adding `DependencyOrder` to `CompiledSchema` is additive. New compiler phases only add error cases — entities that previously compiled with incorrect link targets will now fail. This is a correct breaking change.

**Testing Strategy**
- Unit: A→B link → B appears before A in order
- Unit: A→B→A cycle → compile error with chain `["a","b","a"]`
- Unit: A links to B with `LabelField: "nonexistent"` → compile error naming field
- Integration: Finance 14-entity graph → valid topological order, no cycles

**Implementation Complexity:** Medium

**Architectural Risks**
- Entities declaring FK links to tables not managed by AWO (external tables) → need `ForeignTable: true` escape hatch on `FieldDef`

---

### A2 — Migration Fingerprint
**Priority: HIGH**

**Architectural Goal**
`CompiledSchema` must carry a deterministic fingerprint of the schema it represents. Any consumer that caches schema-derived artifacts (SDUI pages, API route tables) must detect schema staleness by comparing fingerprints.

**Current Deficiency**
SDUI cache key: `page:{entity}:{view}:{roles_sha256_prefix16}:{tenant_id}`. No schema version component. A deployment that changes a field label produces a new binary — cached SDUI pages remain valid from the cache's perspective. Correctness bug, not performance concern.

**Implementation Strategy**
Fingerprint is a deterministic hash of normalized `CompiledSchema` content:
- Entity list sorted by `DependencyOrder` (A1 output)
- Per entity: fields sorted by name, permission identifiers sorted lexicographically
- No runtime-variable content (timestamps, server IDs) included
- Hash: SHA-256 truncated to 16 bytes (32 hex chars)

`CompiledSchema` gains:
- `SchemaFingerprint string`
- `FingerprintedAt time.Time` (diagnostic only, not for equality comparison)

SDUI cache key becomes: `page:{entity}:{view}:{fingerprint_prefix8}:{roles_sha256_prefix16}:{tenant_id}`

On deployment, new binary's fingerprint differs. All SDUI cache keys are cold misses. Old keys orphaned — expire via TTL. No explicit invalidation required.

**Required Packages**
- `awo/compiler` — fingerprint computation, stored in `CompiledSchema`
- `awo/sdui` — consumes fingerprint in cache key construction
- `awo/api/router` — expose fingerprint in health endpoint

**Compiler Changes**
New terminal phase:
```
Phase N (final): Fingerprint computation
  Input:  fully resolved CompiledSchema
  Output: SchemaFingerprint populated
```

**Backward Compatibility**
Adding fields to `CompiledSchema` is additive. Cache key format changes — existing entries become stale on deploy (intended behavior).

**Testing Strategy**
- Unit: same schema compiled twice → identical fingerprints
- Unit: add one field → fingerprint changes
- Unit: change field ordering in source → fingerprint unchanged (sorted normalization)

**Implementation Complexity:** Low

**Architectural Risks**
- Map iteration order is non-deterministic in Go — all maps must be sorted before hashing. Document which fields participate in normalization explicitly.

---

### A3 — Fail-Closed Authorization
**Priority: CRITICAL**

**Architectural Goal**
When `PolicyEvaluator` returns an error (network partition, Redis failure, Casbin panic), the authorization layer must deny the request, not grant it.

**Current Deficiency**
`canPerform` returns `true` on evaluator error. Intent was resilience. Actual effect: a sufficiently timed Redis failure grants any actor any permission. In an ERP context, this is unacceptable.

**Implementation Strategy**
Change `canPerform` to return `false` on evaluator error. Return 503 Service Unavailable (not 403) to distinguish "infrastructure failure" from "access denied" — enables clients and monitoring to distinguish cases.

Introduce `EvaluatorUnavailableError` sentinel type:
```
EvaluatorUnavailableError → 503 + log ERROR + metric authz_evaluator_error_total
policy_denied             → 403 + log DEBUG
```

503 response includes `Retry-After: 5` for transient failures.

Platform admin bypass (`IsPlatformAdmin()`) happens before evaluator call — unaffected.

**Required Packages**
- `awo/auth` — `PolicyEvaluator` interface, `EvaluatorUnavailableError`
- `awo/api/authz` — `RequirePermission` middleware, HTTP response mapping
- `awo/observability/metrics` — `authz_evaluator_error_total`

**API Changes**
`PolicyEvaluator.CanPerform` signature unchanged — `(bool, error)`. New: `type EvaluatorUnavailableError struct { Cause error }`.

**Middleware logic:**
```
err != nil && errors.As(err, &EvaluatorUnavailableError{}) → 503 + metric + log
err != nil (other)                                         → 500 + log
allowed == false && err == nil                             → 403
allowed == true  && err == nil                             → continue
```

**Backward Compatibility**
Behavior change — intentional. Tests expecting 200 on evaluator error will correctly begin failing.

**Testing Strategy**
- Unit: evaluator returns `(false, EvaluatorUnavailableError{})` → 503
- Unit: evaluator returns `(false, fmt.Errorf("panic"))` → 500
- Integration: Redis unavailable → 503, metric incremented

**Implementation Complexity:** Low

**Architectural Risks**
- Casbin evaluator must wrap Casbin-specific errors as `EvaluatorUnavailableError`. This wrapping belongs in the Casbin adapter, not in the middleware — middleware must not import Casbin.

---

### A4 — Validation Framework
**Priority: HIGH**

**Architectural Goal**
The "Validate" stage must return a structured result, not `error`. A validation failure must be distinguishable from a system error at every layer (HTTP handler, Temporal workflow, monitoring).

**Current Deficiency**
`error` is insufficient as a validation result type because:
1. Cannot carry multiple field-level failures simultaneously
2. `ValidationError` and database error are indistinguishable
3. Temporal retries on error — validation errors (user errors) must not be retried
4. HTTP layer cannot distinguish 400 from 500 without string-matching

**Implementation Strategy**
New `awo/validation` package (no imports from `awo/def`, `awo/compiler`, `awo/runtime` — pure value types):

**`ValidationResult`:**
- Zero or more `FieldViolation` entries (field-level errors)
- Zero or more `EntityViolation` entries (cross-field errors)
- Severity per violation: `Error` (blocks persistence) or `Warning` (advisory)

**`ValidationError`:** wraps `ValidationResult`, implements `error`, has `NonRetryable() bool` method.

**`SystemError`:** wraps infrastructure failures, retryable.

Runtime Validate stage signature changes:
```
Validate(ctx, record) error
→
Validate(ctx, record) (*validation.ValidationResult, error)
```

HTTP handler maps:
- `ValidationResult` with errors → 422 Unprocessable Entity with structured field violations
- `SystemError` → 500

Temporal activity options: `NonRetryableErrorTypes: []string{"ValidationError"}`.

**Required Packages**
- `awo/validation` — new package
- `awo/runtime` — Validate stage signature change
- `awo/api/handler` — HTTP response mapping
- `awo/workflow` — Temporal retry policy conventions

**Backward Compatibility**
Breaking — all hook implementations update signature. Pre-v1.0: update in same commit.

**Testing Strategy**
- Unit: missing required field → `ValidationResult` with one `FieldViolation`
- HTTP: 422 response body contains field names and violation messages
- Temporal: `ValidationError` activity failure → no retry

**Implementation Complexity:** Medium

**Architectural Risks**
- Hook authors will return `fmt.Errorf("field X invalid")` instead of `ValidationResult`. If a hook returns a plain `error` from Validate stage, wrap it as a single-violation `ValidationResult` — degrade gracefully.

---

### A5 — Partial Update Semantics
**Priority: CRITICAL**

**Architectural Goal**
`PATCH /entity/:id` must support sparse updates. Only fields present in the request body are written. The current `Update(ctx, id, patch T)` signature cannot distinguish "field set to zero value" from "field not included in patch."

**Current Deficiency**
`EntityRepository[T].Update(ctx, id, patch T)` takes a typed `T`. Remaining fields are zero-valued in the struct. The repository cannot distinguish "user wants to clear this field" from "user did not send this field." Causes silent data loss on PATCH.

**Implementation Strategy**
Replace the `Update` patch parameter with `driver.FieldPatch[T]`:

```go
type FieldPatch[T any] struct {
    Set   map[string]any  // fields to set
    Clear []string        // fields to set to null
}
Update(ctx, id, patch FieldPatch[T]) (T, error)
```

The compiler, for each entity, generates a `FieldPatchValidator` that validates field names in `FieldPatch.Set` and `FieldPatch.Clear` against the entity schema. Unknown field names fail at the handler layer before reaching the repository.

The repository implementation (`contrib/pgx`) translates `FieldPatch` into dynamic `UPDATE SET field1=$1, field2=$2 WHERE id=$3`, including only fields in `Set` and generating `field = NULL` for fields in `Clear`. SQLC's static query model is incompatible with dynamic patches — dynamic query construction is appropriate here but must be **parameterized exclusively — never string-interpolate field names**.

**Required Packages**
- `awo/driver` — `FieldPatch[T]` type, `Update` signature change
- `awo/contrib/pgx` — dynamic patch query construction
- `awo/api/handler` — JSON → `FieldPatch` translation, field name validation
- `awo/compiler` — `FieldPatchValidator` per entity

**Backward Compatibility**
Breaking. All `Update` callers changed. All `EntityRepository` mock implementations changed. Accept pre-v1.0.

**Testing Strategy**
- Unit: patch with one field → only that field updated in DB
- Unit: patch with `Clear: ["description"]` → field set to NULL
- Unit: patch with unknown field name → 400 before reaching repository
- Integration: concurrent patches to different fields → no lost updates

**Implementation Complexity:** High

**Architectural Risks**
- Field name strings in `FieldPatch.Set` are a public API contract. Renaming a field post-v1.0 breaks API clients. Field names are stable identifiers — same stability requirement as entity names.
- Dynamic query builder must be SQL-injection-safe. Every field name passes through `FieldPatchValidator` before reaching `pgx`. Never allow raw field names from HTTP requests to reach the driver without validation.

---

### A6 — System Field Injection
**Priority: HIGH**

**Architectural Goal**
Infrastructure fields (`id`, `tenant_id`, `created_at`, `updated_at`, `created_by`) must not be declared by module authors. The compiler injects them into every entity schema before validation.

**Current Deficiency**
No framework guarantee that every entity has a `tenant_id` field — meaning RLS policy generation cannot safely assume the field exists.

**Implementation Strategy**
Define `SystemFieldSet []FieldDef` in `awo/def`:

| Field | Type | Properties |
|---|---|---|
| `id` | UUID | PK, auto-generated, not patchable |
| `tenant_id` | UUID | FK→platform_tenant, Hidden, not patchable |
| `created_at` | DateTime | ReadOnly, not patchable |
| `updated_at` | DateTime | ReadOnly, not patchable |
| `created_by` | Link→iam_user | ReadOnly, not patchable |

New compiler Phase 1.5 prepends these fields to every entity. Module entity `Fields` arrays must not contain any name from `SystemFieldSet` — compiler rejects with descriptive error.

`SystemFieldSet` is frozen at v1.0. Adding a new system field post-v1.0 requires an ADR and a migration for every existing entity table in every deployment.

**Compiler Changes**
```
Phase 1:   Validate entity names, detect duplicates
Phase 1.5: Inject system fields; reject reserved field names  ← NEW
Phase 2:   Build stubs
```

**Migration Impact**
Every entity table migration must include system field columns. If missing, add additive migration.

**Backward Compatibility**
Entity definitions manually declaring `id`, `tenant_id` etc. → compile error. Remove those declarations.

**Testing Strategy**
- Unit: entity with no system fields → compiled schema has all 5 system fields prepended
- Unit: entity declaring `id` → compile error naming "id" as reserved
- Integration: Finance entity → `FieldsByName["tenant_id"]` populated

**Implementation Complexity:** Low-Medium

**Architectural Risks**
- Conservative, minimal `SystemFieldSet`. `deleted_at` for soft delete is NOT in `SystemFieldSet` — not every entity requires soft delete.

---

### A7 — Scoped Registry
**Priority: MEDIUM**

**Architectural Goal**
The global entity registry must not pollute across test runs. Tests registering test entities affect global state for the entire test binary.

**Current Deficiency**
`def.Register` writes to a package-level map. No mechanism to register entities in an isolated scope for testing.

**Implementation Strategy**
Introduce `Registry` type in `awo/registry`. The global registry (used by production `init()`) is `registry.Global`. Test code constructs `registry.New()` for isolated registration.

`def.Register` delegates to `registry.Global` — all existing `init()` registrations unchanged.

The compiler accepts `*registry.Registry` parameter instead of reading from the global registry implicitly:
```
compiler.Compile(reg *registry.Registry) (*CompiledSchema, error)
```

Test isolation pattern:
```go
reg := registry.New()
registry.RegisterIn(reg, &TestEntityA{})
schema, err := compiler.New().Compile(reg)
```

**Required Packages**
- `awo/registry` — `Registry` type, `New()`, `RegisterIn(reg, entity)`
- `awo/def` — `Register(entity)` delegates to `registry.Global`
- `awo/compiler` — `Compile(reg)` instead of implicit global access

**Backward Compatibility**
Bootstrap code updated to pass `registry.Global`. No behavioral change for production paths.

**Testing Strategy**
- Unit: two isolated registries with conflicting entity names → no cross-contamination
- Unit: `registry.Global` contains all `init()`-registered entities

**Implementation Complexity:** Low

**Architectural Risks**
- Tests using `def.Register` (global function) instead of `registry.RegisterIn` still pollute the global registry. Linter rule or CI check needed.

---

### A8 — SDUI Production Blockers
**Priority: CRITICAL (as a group)**

**B-01: HTML injection in labels/descriptions**
Add `sanitizeText(s string) string` helper in `awo/sdui/amis` calling `html.EscapeString`. Apply in `applyCommon` and all structural renderer label assignments. Must be exhaustive.

**B-02: Expression XSS**
v1.0 fix: `VisibleOn`/`HiddenOn`/`DisabledOn`/`RequiredOn` are trusted as code (entity definitions require a deployment). Add compile-time validation pass that rejects expressions containing obvious dangerous patterns. Document: "expressions are trusted as code; never accept expression values from user input."

**B-03: renderButton skips applyCommon**
Call `applyCommon(n, out)` in `renderButton` after setting `type` and `label`. One-line fix. Do immediately.

**B-04: Schema fingerprint in cache key** — covered by A2.

**B-05: canPerform fail-closed** — covered by A3.

**B-06: buildDetail missing edge tables**
Extract `buildEdgeTables(es *compiler.EntitySchema) []*widget.Node` from `buildForm`. Call from both `buildForm` and `buildDetail`.

**B-07: Toolbar duplication in list view (TD-12)**
`buildList` assigns `toolbarActions` to both `listNode.Actions` and returned `NodePage.Actions` → "New" button appears twice.

Fix: introduce `EntityDefinition.EntityPageActions() []ActionDef` and `EntityDefinition.EntityListActions() []ActionDef`. List actions → list node. Page actions → page node. For v1.0: "New" actions go to list toolbar; export/import go to page toolbar.

**B-08: Race condition test**
Add `TestGenerator_ConcurrentRender` running 100 goroutines calling `Generate` simultaneously. Run with `go test -race`.

**Required Packages**
- `awo/sdui/amis` — B-01, B-02, B-03
- `awo/sdui` — B-06, B-07
- `awo/def` — B-07 (new methods on `EntityDefinition`)

---

## PART 2 — Dependency Analysis

```
A7 (Scoped Registry)
 └── A1 (Dependency Graph)
       ├── A2 (Migration Fingerprint)
       └── A6 (System Field Injection) [parallel with A1, coordinate compiler phases]
             ├── A4 (Validation Framework)
             │     └── A5 (Partial Update Semantics)
             │           └── A8-B06 (buildDetail edge tables)
             └── A8-B07 (Toolbar split — needs PageActions/ListActions in def)

A3 (Fail-closed Auth)      ← independent, execute immediately
A8-B01/B02/B03/B08         ← independent, execute immediately
```

**Dependency justifications:**

**A7 before A1:** Graph integration tests need isolated registries (real entity sets without global pollution). Technically A1 can precede A7 but test suite will be unreliable.

**A1 before A2:** Fingerprint normalization uses `DependencyOrder` (A1 output) to sort entities deterministically for hashing.

**A6 parallel with A1:** System field injection is a compiler phase that runs before graph construction. Both touch the compiler — coordinate to avoid merge conflicts, but implement independently.

**A6 before A4:** Validation rules for system fields (e.g., "created_at must not be in PATCH") require knowing which fields are system fields.

**A4 before A5:** Partial update must classify patch validation failures as `ValidationError`, not system errors. Requires A4's type.

**A3 independent:** Touches different packages from all other items. No reason to delay.

**A8-B01/B02/B03/B08 independent:** Renderer fixes and tests only. No dependencies on A1–A7.

**A8-B07 depends on A6 coordination:** `PageActions`/`ListActions` split requires a change to `EntityDefinition` interface in `awo/def`. A6 also changes `def`. Do not overlap — combine into one `def` interface change commit or strictly sequence.

**Parallel execution plan:**
```
Immediate (no dependencies):
  A3, A8-B01, A8-B02, A8-B03, A8-B08

Sequential track 1:
  A7 → A1 → A2

Sequential track 2 (parallel with track 1):
  A6 → A4 → A5 → A8-B06

Convergence:
  A8-B07 (requires A6 def changes; coordinate with A1 compiler changes)
```

---

## PART 3 — Kernel Freeze Analysis

**Definition:** Frozen package's public API cannot change in a breaking way without a major version bump. Additive exports allowed. Remove/rename prohibited.

| Package | Status | Rationale |
|---|---|---|
| `awo/def` | **FROZEN** | Module author's primary interface |
| `awo/compiler` (public API) | **FROZEN** | Framework consumer contract |
| `awo/registry` | **FROZEN** | Entity registration contract |
| `awo/driver` | **FROZEN** | Storage abstraction |
| `awo/auth` | **FROZEN** | Auth contract for every layer |
| `awo/validation` | **FROZEN** | Hook author's result contract |
| `awo/sdui/widget` | **FROZEN** | Renderer IR contract |
| `awo/sdui` (Generator interface) | **FROZEN** | SDUI entry point |
| `awo/runtime` | Internal, not frozen | Pipeline implementation |
| `awo/sdui/amis` | Internal, not frozen | One renderer's implementation |
| `awo/platform/iam` | Not frozen | Auth implementation |
| `awo/contrib/*` | Not frozen | Driver implementations |

`awo/def`: `FieldDef` may gain new optional fields (zero-valued = backward compatible). Removing or renaming a field is prohibited.

`awo/compiler`: Internal compiler phases are not exported and may evolve. `CompilerPass` plugin API (Phase C) will be additive new exports.

`awo/sdui/widget`: New `NodeKind` constants are additive and allowed.

---

## PART 4 — Compiler Evolution

### Proposed multi-pass pipeline

Each phase produces an **immutable IR** consumed by the next phase. No phase mutates a previous phase's output. Enables parallel execution, incremental compilation (Phase E), and phase-level testing.

```
Phase 0: Input normalization
  Input:  *registry.Registry
  Output: []EntityInput{Name, Fields, Permissions, Hooks, Actions, Workflows, Layout}
  Work:   Normalize field names to snake_case
          Reject duplicate entity names, reserved characters
  IR:     EntityInput[] — raw, unvalidated, unsorted

Phase 1: System field injection
  Input:  []EntityInput
  Output: []EntityInput (system fields prepended to each)
  Work:   Prepend SystemFieldSet to each entity's Fields
          Reject entities declaring reserved field names
  IR:     EntityInput[] with system fields

Phase 2: Stub construction
  Input:  []EntityInput
  Output: []EntityStub{Name, FieldsByName map, PermissionIDs}
  Work:   Build FieldsByName lookup map per entity
          Extract all permission identifiers
  IR:     EntityStub[] — shallow, fast lookup structures

Phase 3: Dependency graph construction
  Input:  []EntityStub
  Output: DependencyGraph{Nodes, Edges, TopologicalOrder}
  Work:   Extract FieldTypeLink edges
          Topological sort (Kahn's algorithm)
          Detect cycles — error with chain
          Detect missing link targets — error with field name
  IR:     DependencyGraph — directed, acyclic if valid

Phase 4: Cross-entity validation
  Input:  []EntityStub, DependencyGraph
  Output: []CompilerDiagnostic (warnings + errors)
  Work:   Validate LabelField/ValueField exist in linked entity
          Validate PermissionSet identifiers follow naming convention
          Validate ActionDef names match workflow IDs if declared
          Run registered CompilerPass extensions (Phase C)
  IR:     Diagnostic list; non-empty errors halt compilation

Phase 5: Schema materialization
  Input:  []EntityStub, DependencyGraph (validated)
  Output: []EntitySchema{Fields, FieldsByName, Layout, Permissions, Capabilities}
  Work:   Build CapabilityGrant per permission identifier
          Build LayoutDef structure with resolved field references
  IR:     EntitySchema[] — primary consumer artifact

Phase 6: Route table generation
  Input:  []EntitySchema, DependencyGraph.TopologicalOrder
  Output: RouteTable{Routes[], ActionRoutes[]}
  Work:   Generate CRUD routes in topological order
          Generate action routes per ActionDef
  IR:     RouteTable

Phase 7: Fingerprint computation
  Input:  []EntitySchema (fully materialized), RouteTable
  Output: SchemaFingerprint string
  Work:   Deterministic serialization in TopologicalOrder
          SHA-256 truncated to 16 bytes
  IR:     SchemaFingerprint string

Phase 8: Diagnostic report
  Input:  All prior IRs
  Output: CompilerReport{EntityCount, DependencyGraph summary, Fingerprint, Warnings}
  Work:   Summarize compilation for bootstrap logging
  IR:     CompilerReport

Final output: CompiledSchema{
    Entities:          []EntitySchema
    RouteTable:        RouteTable
    DependencyOrder:   []string
    SchemaFingerprint: string
    Report:            CompilerReport
}
```

### CompiledSchema as terminal IR

`CompiledSchema` is the terminal IR consumed at runtime. All intermediate IRs (`EntityInput`, `EntityStub`, `DependencyGraph`, `EntitySchema`) are internal to the compiler — not exported. Runtime code imports no intermediate IR types.

### Multi-pass classification

- Pass 1 (phases 0–2): entity-local normalization
- Pass 2 (phase 3): graph construction (cross-entity, read-only)
- Pass 3 (phase 4): cross-entity validation (requires graph)
- Pass 4 (phases 5–8): materialization and output generation

Each pass is monotonically enriching — later passes read earlier passes' output only, never modify. This is the correct constraint for determinism.

### Migration generation as Phase B6 compiler output

Phase B6 fits naturally between phases 5 and 6:
```
Phase 5.5: Migration generation (Phase B6)
  Input:  []EntitySchema, DependencyGraph.TopologicalOrder
  Output: []MigrationStatement (DDL in topological order)
  Condition: only when --generate-migration flag is set
```

---

## PART 5 — Runtime Evolution

### Current pipeline inadequacy

Five stages (`BeforeCreate → Validate → Authorize → Persist → AfterCreate`) insufficient for Finance lifecycle. Finance needs: default injection, computed field evaluation, state machine enforcement, audit trail, domain event emission. Without dedicated stages, all of these land in hooks — creating hook spaghetti.

### Proposed 12-stage lifecycle

```
Stage 1:  Normalize
  Work:   Field name → FieldDef.Name mapping
          Type coercion (string → decimal, string → uuid)
          Trim whitespace on string fields
  Why:    Hooks receive normalized, type-coerced data — not raw user input

Stage 2:  Inject defaults
  Work:   Apply FieldDef.DefaultValue where field is zero-valued
          Apply system field defaults (created_at=now, tenant_id from context)
  Why:    Default injection is framework-owned. Hooks must not implement defaults.

Stage 3:  Compute fields  [no-op stub at v1.0; Phase B2 implements]
  Work:   Evaluate FieldDef.ComputedFrom expressions in CEL
  Why:    Computed before validation so validators see final values

Stage 4:  Validate (structured)
  Work:   Required field checks, FieldDef type constraints
          Hook-provided validators (HookSet.Validate)
  Output: ValidationResult + system error
  Why:    All prior stages produce final field values. Validate final state.

Stage 5:  Authorize
  Work:   PolicyEvaluator.CanPerform (fail-closed per A3)
          PolicyFunc row-level filter (if declared)
  Why:    Authorization is separate from validation — different error semantics

Stage 6:  State transition  [no-op stub at v1.0; Phase B1 implements]
  Work:   Validate status field transition against StateMachineDef
          Check transition permission
          Queue Temporal workflow if transition declares one
  Why:    State machine enforcement before persistence prevents illegal states

Stage 7:  Persist
  Work:   EntityRepository.Create / Update / Delete
          PostgreSQL RLS enforced at DB level
  Why:    Only stage that writes to database

Stage 8:  Audit  [no-op stub at v1.0; Phase B3 implements]
  Work:   Write audit records for FieldDef.Audited fields
          Runs in same transaction as Stage 7
  Why:    Audit must be transactionally consistent with persistence

Stage 9:  Events
  Work:   Write domain event to outbox table (same transaction)
          Types: EntityCreated, EntityUpdated, EntityDeleted
  Why:    Transactional outbox ensures delivery even after crash

Stage 10: Temporal dispatch
  Work:   Execute Temporal workflows from TransitionDef.Workflow
          Or EntityWorkflows triggered by create/update
  Why:    Temporal starts after persistence — entity exists before workflow begins

Stage 11: After hooks
  Work:   HookSet.AfterCreate / AfterUpdate / AfterDelete
          Hook failures do NOT roll back persistence (post-commit)
  Why:    After hooks are for side effects. If atomicity required, use Stage 10.

Stage 12: Metrics + tracing
  Work:   Emit latency per stage, error counters, entity type labels (OpenTelemetry)
  Why:    Stage-level observability is the only way to diagnose pipeline perf
```

### v1.0 pipeline interface

**Define all 12 stage interfaces at v1.0 even if stages 3, 6, 8 are stubs.** The pipeline contract is frozen — implementations arrive in Phase B without interface change.

```go
type PipelineStage interface {
    Execute(ctx context.Context, op *PipelineOperation) error
}

type PipelineOperation struct {
    EntityName string
    Viewer     auth.ViewerContext
    Record     any
    OldRecord  any               // nil for Create
    Patch      *driver.FieldPatch // nil for Create/Delete
}
```

---

## PART 6 — Metadata Evolution

| Primitive | Location | Decision |
|---|---|---|
| `ComputedFrom string` | `FieldDef` | **Add at v1.0** |
| `DefaultValue any` | `FieldDef` | **Add at v1.0** |
| `Indexed bool` | `FieldDef` | **Add at v1.0** |
| `StateMachineDef` | `EntityDefinition` interface | **Add at v1.0** (struct + compiler validation; runtime enforcement Phase B1) |
| `SoftDelete bool` | `EntityDefinition` | Defer Phase B |
| `ValidationDef` | `FieldDef` | Defer Phase B |
| `UniqueConstraint` | `FieldDef` | Defer Phase B |
| `AuditDef` | `FieldDef` | Defer Phase B |
| `ApprovalDef` | `EntityDefinition` | Defer Phase B |
| `Versioning` | `FieldDef` | Defer Phase B |
| `Search metadata` | `FieldDef` | Defer Phase C |

**ComputedFrom — must exist before Finance.** Invoice line totals require `line_total = quantity * unit_price`. Without this, Finance writes trigger functions or application-layer recalculation — both architectural violations.

**StateMachineDef — must exist before Finance.** Journal entries: `draft → submitted → posted → reversed`. Without state machine primitives, Finance writes ad hoc state validation in hooks, creating N inconsistent implementations across 14 entities. At v1.0: declare structure and validate in compiler. Runtime enforcement is a no-op stub — Phase B1 implements it.

**DefaultValue — must exist before Finance.** Invoice `status` defaults to "draft". Payment `currency` defaults to tenant's base currency. Without this, Finance sets defaults in `BeforeCreate` hooks — scattered, untestable.

**SoftDelete — must NOT exist in `SystemFieldSet`.** Not every entity requires soft delete. Opt-in at entity level (Phase B).

---

## PART 7 — Package Ownership Matrix

| Responsibility | Package | Notes |
|---|---|---|
| Entity metadata declaration | `awo/def` | Module author interface |
| System field set definition | `awo/def` | `SystemFieldSet` exported constant |
| State machine definition | `awo/def` | `StateMachineDef`, `TransitionDef` |
| Validation result types | `awo/validation` | Pure value types, no framework imports |
| Entity registry (global) | `awo/registry` | `Global` + `New()` for tests |
| Schema compilation | `awo/compiler` | All phases, produces `CompiledSchema` |
| Dependency graph | `awo/compiler` | Internal IR, not exported |
| Migration fingerprint | `awo/compiler` | Terminal phase output |
| Migration generation | `awo/compiler` | Phase B6 addition |
| Storage abstraction | `awo/driver` | `EntityRepository[T]`, `FieldPatch[T]` |
| PostgreSQL implementation | `awo/contrib/pgx` | Implements driver interface |
| Redis implementation | `awo/contrib/redis` | Cache, session store |
| Entity lifecycle pipeline | `awo/runtime` | Internal, not exported |
| Authorization enforcement | `awo/api/authz` | `RequirePermission` middleware |
| Authorization interface | `awo/auth` | `PolicyEvaluator`, `ViewerContext` |
| SDUI generation | `awo/sdui` | `Generator` interface |
| Widget tree IR | `awo/sdui/widget` | `Node`, `NodeKind` |
| AMIS rendering | `awo/sdui/amis` | Internal renderer |
| Audit trail writing | `awo/platform/audit` | Invoked from pipeline Stage 8 |
| Transactional outbox | `awo/events/outbox` | Pipeline Stage 9 |
| Temporal workflow declarations | `awo/workflow` | Workflow/Activity interfaces |
| Naming series | `awo/naming` | Auto-ID generation |
| Health/diagnostics | `awo/observability` | Metrics, spans, health endpoint |
| Route registration | `awo/api/router` | Reads `CompiledSchema.RouteTable` |
| Tenant context | `awo/platform/tenant` | `WithTenant`, `FromContext` |

**Key ownership decisions:**

- Migration generation lives in `awo/compiler` — not a separate `migration` package. Compiler has schema knowledge; a standalone package would require importing the compiler (dependency inversion).
- Audit trail writing lives in `awo/platform/audit` — not in `awo/runtime`. Runtime delegates; does not own audit format.
- Dependency graph is internal to `awo/compiler` — `DependencyOrder []string` in `CompiledSchema` is the public contract only.

---

## PART 8 — Architectural Debt (post-Phase A)

### Severity 1 — Structural

**Debt-1: No field-level security**
Authorization has no mechanism to restrict access to individual fields. A user with `finance.invoice.read` sees all fields including sensitive ones. Requires: compiler pass (validate `ReadPermission` identifiers), runtime serialization step (redact fields), SDUI generator change (omit fields from forms). Grows with every sensitive field added.

**Debt-2: `NodeSection` semantic overloading**
`NodeSection` serves as both a form section container and a tab pane container within `NodeTabs`. A renderer must infer which role by inspecting the parent — structurally fragile. Fix: introduce `NodeTabPane`. Affects every renderer implementation. **Fix before v1.0** (see Part 12, question 4).

**Debt-3: Dynamic query construction in partial update**
A5 introduces dynamic `UPDATE SET field=$N` in `contrib/pgx`. Bypasses SQLC's compile-time SQL safety. `FieldPatchValidator` is the only injection defense. Permanent trade-off for partial update semantics. Must be exhaustively tested and audited.

### Severity 2 — Design

**Debt-4: `rowActions` as `map[string]any` in generator**
SDUI generator constructs AMIS-specific button schemas directly inside `buildList`. Renderer concern inside generator layer — layer violation. Fix: generator emits `widget.ActionNode`; amis renderer converts to `map[string]any`.

**Debt-5: `setAllReadOnly` recursive post-processing**
Detail page read-only state applied by O(N) recursive walk. Semantically fragile — nodes added after the walk are not read-only. Fix: `buildLayoutNodes(isReadOnly bool)` parameter propagating read-only intent during tree construction, not after.

**Debt-6: CEL expressions as opaque JavaScript strings**
`VisibleOn`, `HiddenOn`, `DisabledOn`, `RequiredOn` are raw JavaScript strings. Framework cannot validate, transform, or safely evaluate them server-side. At scale (mobile renderer, PDF renderer, accessibility tree), each renderer must independently re-evaluate in its own environment. A structured expression IR (not a JS string) would allow translation across renderers. Critical for Phase C (mobile).

**Debt-7: Compiler phase ordering implicit, not enforced**
Compiler phases are sequential functions called in fixed order. No enforcement that phase N only reads phase N-1's output. The proposed multi-pass IR architecture (Part 4) resolves this.

### Severity 3 — Operational

**Debt-8: No structured compiler diagnostics**
Compiler emits errors as `error` values. No `CompilerDiagnostic` type with error codes, entity names, field names, suggested fixes. An IDE plugin or CI validator cannot parse string errors.

**Debt-9: No SDUI caching for PolicyFunc viewers**
Cache key includes roles hash. If a viewer has a `PolicyFunc` (row-level filter) affecting field visibility, the cache key does not account for it — two viewers with the same roles but different `PolicyFunc` results get the same (potentially incorrect) cached page.

**Debt-10: `contrib/pgx` has no connection health recovery**
`EntityRepository[T]` has no `Ping()` or `HealthCheck()` method. Health endpoint cannot accurately report database health.

---

## PART 9 — Ten-Year Stability Review

### What survives unchanged

- `EntityDefinition` interface — survives; minimal, extended by addition not modification
- `FieldDef` struct — survives with additions; new optional fields are zero-valued and backward compatible
- `EntityRepository[T]` interface — signature survives; implementation revised for performance at scale
- `ValidationResult` — survives; pure value type, no framework dependencies
- `widget.Node` and `NodeKind` — survive; new `NodeKind` constants are additive

### What will require redesign

**Compiler requires incremental compilation within 5 years.** At 1,000 entities, full compilation on every startup will take seconds. The compiler must become incremental — cache per-module compiled artifacts, only recompile modules whose transitive dependencies changed. The multi-pass IR architecture (Part 4) enables this without structural change — design internal phases as isolated transformations now.

**Global registry will not survive beyond 300 modules.** `init()` registration requires all modules compiled into one binary. At 300 modules the binary is unwieldy. `registry.New()` (A7) is the first step — design `Registry` to accept modules from any source, even if v1.0 only uses `init()`.

**SDUI generator will not survive renderer proliferation.** Current generator is shaped for AMIS. As mobile, PDF, and accessibility renderers are added, the generator must become renderer-neutral. The `FieldGroupDef` refactor is the prerequisite — implement before Phase C.

**`PolicyEvaluator` interface will outlive Casbin.** The interface is correctly minimal and will survive. Casbin's performance at 1,000 entities × 10,000 roles × thousands of tenants is unknown and likely inadequate. The interface enables replacement without changing entity definitions. Maintain this — ensure no Casbin-specific types leak through the interface.

**CEL expressions as JavaScript strings will not survive a mobile renderer.** React Native does not evaluate arbitrary JavaScript. A PDF renderer does not. By Phase C, the expression model must change. Introduce an expression IR now — even if v1.0 only serializes it as JavaScript strings for AMIS. Minimal: `type Expression struct { Raw string; Lang string }` with `Lang: "amis-js"`.

**Transactional outbox relay will require revision at high event volumes.** Polling model becomes a bottleneck at millions of events/day. CDC via PostgreSQL logical replication is the replacement. The interface (`events/outbox.Write(ctx, event)`) survives — the relay implementation does not.

### Recommendations for ten-year stability

1. Multi-pass compiler with typed IRs now (Part 4). Enables incremental compilation without structural change.
2. Pluggable registry now (A7). Design `Registry` for runtime population even if v1.0 uses only `init()`.
3. Expression IR now. Even a minimal `Expression{Raw, Lang}` creates the extension point for Phase C.
4. Freeze package boundaries hard at v1.0. Ten-year stability depends entirely on no cross-package dependency violations post-freeze.
5. Version the `CompiledSchema` IR. Add `SchemaVersion int = 1` at v1.0. Runtime rejects mismatched versions explicitly rather than crashing silently.

---

## PART 10 — ADRs

---

### ADR-007: Compiler Becomes Graph-Based

**Context**
AWO compiler processes entities independently. No cross-entity dependency model. Link resolution validates target entity name existence but does not build a traversable dependency graph, detect cycles, validate cross-entity field references, or produce a canonical entity ordering. Finance module has 14 entities with complex interdependencies.

**Decision**
Compiler constructs a `DependencyGraph` after stub generation and before any cross-entity validation. Graph contains one node per entity and one directed edge per `FieldTypeLink` reference. Topological sort (Kahn's algorithm) detects cycles and produces a canonical `DependencyOrder`. All cross-entity validation runs after graph construction. `CompiledSchema` exposes `DependencyOrder []string`.

**Consequences**
Positive: Cycles detected at compile time. Invalid link targets caught at compile time. Cross-entity field references validated at compile time. Migration files emit in dependency order. Registry introspection returns entities in topological order.
Negative: Compiler startup time increases O(V+E). At 1,000 entities — sub-millisecond, not a concern.

**Alternatives rejected**
Detecting cycles at migration time: rejected — compile-time errors are categorically better than runtime errors.
Detecting cycles only for explicitly declared FK fields: rejected — `FieldTypeLink` is already the FK declaration.

**Future implications**
Dependency graph is the foundation for incremental compilation (Phase E). Design graph format with serialization in mind — a cached, serialized graph per module enables incremental builds.

---

### ADR-008: System Fields Injected by Compiler

**Context**
Infrastructure fields (`id`, `tenant_id`, `created_at`, `updated_at`, `created_by`) must be present in every entity table for RLS enforcement and audit correctness. No framework guarantee currently. An entity missing `tenant_id` silently breaks RLS.

**Decision**
`SystemFieldSet []FieldDef` declared in `awo/def`. New compiler Phase 1.5 prepends `SystemFieldSet` to every entity's field list. Module authors must not declare any field whose name appears in `SystemFieldSet` — compiler rejects with named error. System fields are not patchable (validated by `FieldPatchValidator`). System fields not in SDUI form output (generator skips `Hidden: true` system fields by default).

**Consequences**
Positive: RLS correctness guaranteed by compiler. Audit fields guaranteed present. Module authors cannot omit `tenant_id`. Migration baseline uniform across all entities.
Negative: Adding a new system field post-v1.0 requires migration for every existing entity table in every deployment. `SystemFieldSet` must be conservative and minimal at v1.0.

**Alternatives rejected**
Embedded Go struct that module authors embed: rejected — compiler cannot inspect embedded structs, only `FieldDef` slices.
Required convention via a well-known function: rejected — "required" conventions are violated; the compiler cannot enforce them.

**Future implications**
`SystemFieldSet` frozen at v1.0 at exactly 5 fields. Adding a 6th field requires an ADR, migration script generator, and deprecation path. High bar prevents opportunistic accumulation.

---

### ADR-009: Authorization Is Fail-Closed

**Context**
`PolicyEvaluator.CanPerform` returns `(bool, error)`. When evaluator returns non-nil error, current middleware grants access. Rationale was resilience. Actual effect: infrastructure failures grant any actor any permission. Unacceptable for financial records.

**Decision**
`canPerform` returns false on any evaluator error. HTTP middleware maps to 503 Service Unavailable (not 403 — distinction matters for monitoring and client retry). New `EvaluatorUnavailableError` sentinel type distinguishes infrastructure failures from policy denials. Platform admin bypass (`IsPlatformAdmin()`) occurs before evaluator call — unaffected.

**Consequences**
Positive: Infrastructure failures never grant unauthorized access. Monitoring can alert on 503 spikes. Temporal workflows retry on 503 (transient) but not 403 (permanent).
Negative: If evaluator backend is unavailable, legitimate users cannot perform operations. This is the correct trade-off.

**Alternatives rejected**
Circuit breaker with cached policy decisions: deferred to Phase B. Caching authorization decisions introduces staleness — revoked role continues to have access until cache expires. Requires cache invalidation mechanism that does not exist at v1.0.
Allow access for specific error classes: rejected — no safe error class exists that should grant access.

**Future implications**
Circuit breaker with short-lived cached decisions (30 seconds) is appropriate for Phase B. The evaluator interface supports this — circuit breaker is an implementation of `PolicyEvaluator` wrapping the real evaluator. No interface change required.

---

### ADR-010: Sparse Patch Model Replaces Entity Overwrite

**Context**
`EntityRepository[T].Update(ctx, id, patch T)` takes a full entity struct. A PATCH request updating one field passes a struct where all other fields are zero-valued. Repository cannot distinguish "field intentionally cleared" from "field not included in patch." Causes silent data loss.

**Decision**
`EntityRepository[T].Update` changes to `Update(ctx, id, patch driver.FieldPatch[T]) (T, error)`. `FieldPatch[T]` carries `Set map[string]any` (fields to update) and `Clear []string` (fields to null). `contrib/pgx` constructs a dynamic `UPDATE SET` query for only the fields in `Set` and `Clear`. A compiler-generated `FieldPatchValidator` per entity validates field names at the handler layer before they reach the repository. `FieldPatchValidator` is the SQL injection boundary — field names passing through it are safe for dynamic query construction.

**Consequences**
Positive: No silent data loss. Optional fields can be explicitly cleared. API clients send only changed fields.
Negative: Dynamic query construction in `contrib/pgx` bypasses SQLC's compile-time SQL safety. `FieldPatchValidator` is the only injection defense. Permanent trade-off.

**Alternatives rejected**
JSON Merge Patch (RFC 7396): rejected — null is ambiguous in Go (null JSON vs absent JSON both deserialize to Go zero value). `FieldPatch` makes the distinction explicit.
Field masks (proto-style): rejected — requires callers to duplicate all field values plus explicit mask — more verbose than necessary for HTTP APIs.

**Future implications**
Field name strings in `FieldPatch.Set` are a public API contract. Renaming a field post-v1.0 breaks API clients. Establish policy: field names in the public API are stable identifiers, governed by the same stability requirements as entity names.

---

### ADR-011: Migration Fingerprint Embedded in CompiledSchema

**Context**
SDUI pages cached in Redis with no schema version component. A deployment changing a field label produces a new binary — cached pages from the previous deployment remain valid from the cache's perspective. Users see stale UI. Correctness bug.

**Decision**
Compiler computes `SchemaFingerprint string` as its final phase. Fingerprint is SHA-256 of normalized `CompiledSchema` content (entity list in `DependencyOrder`, fields sorted by name, permissions sorted lexicographically). SDUI cache key becomes `page:{entity}:{view}:{fingerprint_prefix8}:{roles}:{tenant}`. Cache entries from previous deployment have different fingerprint prefix — cache misses. Old entries expire via TTL. No explicit invalidation required.

**Consequences**
Positive: Cache correctness on deployment. No stale UI after schema change. Fingerprint exposed via health endpoint for deployment verification.
Negative: Cache cold-start on every deployment. All SDUI pages regenerated on first request after deployment. Burst of generator load. Mitigation: cache warming script (Phase B).

**Alternatives rejected**
Explicit cache invalidation on deployment: rejected — requires coordinating invalidation with deployment across distributed cache nodes. Race conditions between invalidation and new requests are unavoidable.
TTL-based invalidation without fingerprint: rejected — allows stale pages to persist for TTL duration. Up to 1 hour of stale UI after a schema-changing deployment.

**Future implications**
Fingerprint normalization must include every aspect of the schema that affects rendered output. Document the normalization algorithm explicitly — it is a correctness invariant. New `FieldDef` properties affecting rendering must be included in normalization.

---

## PART 11 — Implementation Roadmap

### Milestone 0 — Pre-work (1 day, no code)
ADR-007 through ADR-011 reviewed and accepted. Phase A sequencing agreed. No exit criteria beyond alignment.

---

### Milestone 1 — Immediate fixes (parallel, 1–2 days)

**Stream A: A3 — Fail-closed authorization**
- Packages: `awo/auth`, `awo/api/authz`, `awo/observability/metrics`
- Deliverables: `EvaluatorUnavailableError` type; `canPerform` fail-closed; 503 middleware path; `authz_evaluator_error_total` metric; 403/503/200 tests
- Breaking changes: tests expecting 200 on evaluator error → now fail correctly
- Exit criteria: all three HTTP paths covered by tests; metric fires on evaluator error

**Stream B: A8-B01/B02/B03/B08 — SDUI renderer fixes**
- Packages: `awo/sdui/amis`, `awo/sdui`
- Deliverables: `sanitizeText()` helper; expression policy comment + basic compile-time denylist; `renderButton` calls `applyCommon`; `TestGenerator_ConcurrentRender` race test
- Breaking changes: none visible externally
- Exit criteria: `-race` test passes; `renderButton` test with expressions passes

---

### Milestone 2 — Registry isolation (2 days)
**A7 — Scoped registry**
- Packages: `awo/registry`, `awo/def`, `awo/compiler`
- Deliverables: `Registry` type; `New()`, `RegisterIn()`, `All()`; `registry.Global`; `compiler.Compile(reg *Registry)` signature; bootstrap updated to pass `registry.Global`
- Breaking changes: `compiler.Compile()` signature; all callers updated in same commit
- Migration: update `bootstrap/` to pass `registry.Global`
- Exit criteria: two test registries with conflicting entity names coexist without contamination

---

### Milestone 3 — System field injection (2–3 days)
**A6 — System fields**
- Packages: `awo/def`, `awo/compiler`
- Deliverables: `SystemFieldSet []FieldDef`; compiler Phase 1.5; reserved name rejection; audit of existing entity definitions; migration baseline verified
- Breaking changes: entity definitions manually declaring `id`, `tenant_id` etc. → compile error; remove those declarations
- Migration: audit existing migration files; add additive migration if system field columns missing
- Exit criteria: compiler rejects reserved field names; every entity schema has all 5 system fields

---

### Milestone 4 — Dependency graph + Fingerprint (4–5 days)
**A1 + A2**
- Dependencies: Milestone 2 (scoped registry), Milestone 3 (system fields inject first)
- Packages: `awo/compiler`, `awo/sdui`
- Deliverables: `DependencyGraph` internal type; cycle detection with chain reporting; cross-entity field validation; `CompilerDiagnostic` structured type; `CompiledSchema.DependencyOrder`; `CompiledSchema.SchemaFingerprint`; SDUI cache key updated with fingerprint prefix
- Breaking changes: previously compiling entities with invalid link targets now fail to compile; correct
- Migration: fix any entity definitions with invalid link targets (exposed for first time)
- Exit criteria: Finance 14-entity graph topologically sorts correctly; cycle test fails at compile time; identical fingerprints for identical schemas; fingerprint changes when any field label changes

---

### Milestone 5 — Validation framework (3–4 days)
**A4**
- Dependencies: Milestone 3 (system fields known before validation references them)
- Packages: `awo/validation` (new), `awo/runtime`, `awo/api/handler`
- Deliverables: `ValidationResult`, `FieldViolation`, `EntityViolation`, `ValidationError`; runtime Validate stage signature change; HTTP handler 422/500 distinction; Temporal retry policy conventions documented; all hook implementations updated
- Breaking changes: `HookSet.Validate` signature changes; all hooks updated in same commit
- Migration: update all `HookSet.Validate` implementations
- Exit criteria: 422 response body contains `{"errors": [{"field": "amount", "message": "..."}]}`; `ValidationError` is non-retryable in Temporal

---

### Milestone 6 — Partial update semantics (5–6 days)
**A5**
- Dependencies: Milestone 5 (FieldPatchValidator produces ValidationResult for invalid field names)
- Packages: `awo/driver`, `awo/contrib/pgx`, `awo/api/handler`, `awo/compiler`
- Deliverables: `driver.FieldPatch[T]` type; `EntityRepository[T].Update` signature change; dynamic UPDATE query in `contrib/pgx` (parameterized, injection-safe); compiler-generated `FieldPatchValidator` per entity; HTTP handler JSON → FieldPatch translation
- Breaking changes: all `Update` callers changed; all `EntityRepository` mock implementations changed
- Migration: update all callers of `Update` in handlers, tests, workflows
- Exit criteria: PATCH with one field → DB UPDATE touches only that field; SQL injection attempt via field name rejected at validator; one-field patch leaves other fields unchanged (verified by subsequent GET); `go test -race ./awo/contrib/pgx/...` passes

---

### Milestone 7 — SDUI detail and toolbar fixes (3 days)
**A8-B06, B07**
- Dependencies: Milestone 6 complete
- Packages: `awo/sdui`, `awo/def`
- Deliverables: `buildEdgeTables()` extracted and called from both form and detail builders; `EntityDefinition` interface gains `EntityPageActions() []ActionDef` and `EntityListActions() []ActionDef`; `SystemDefinition` and `CustomDefinition` implement new methods; `buildList` fixed
- Breaking changes: `EntityDefinition` interface gains two methods; all implementations must implement them; provide default (empty slice) implementations
- Migration: all `EntityDefinition` implementations add two new methods
- Exit criteria: list view test confirms "New" button appears exactly once; detail page for entity with FK children shows edge tables

---

### Milestone 8 — v1.0 Kernel Freeze Validation (2 days)
- All Phase A items verified by tests
- Frozen package list documented in CLAUDE.md
- All exported symbols in frozen packages have godoc comments
- No import from frozen package to non-frozen package (except `awo/runtime` → `awo/def`)
- `SchemaFingerprint` exposed in health endpoint
- `CompilerReport` logged at bootstrap
- Security audit: `EvaluatorUnavailableError` path verified; dynamic query injection path audited

---

### Milestone 9 — Pre-Finance metadata additions (5–7 days)
**ComputedFrom, StateMachineDef, DefaultValue — required before Finance begins**
- Dependencies: Milestone 8 complete
- Note: Additive — no frozen APIs change
- Packages: `awo/def`, `awo/compiler`, `awo/runtime`
- Deliverables: `FieldDef.ComputedFrom string`; `FieldDef.DefaultValue any`; `FieldDef.Indexed bool`; `StateMachineDef`, `TransitionDef` types; `EntityDefinition.EntityStateMachine() *StateMachineDef`; compiler validates `ComputedFrom` field references; compiler validates `StateMachineDef` state names and transitions; runtime Stage 2 (DefaultValue injection) implemented; runtime Stage 3 and Stage 6 — stub implementations
- Exit criteria: Finance entity definitions declare `ComputedFrom`, `DefaultValue`, `StateMachineDef` without errors; compiler validates them; runtime applies DefaultValue on Create

---

**Total: 10–12 weeks to v1.0 freeze + Finance-ready**

---

## PART 12 — Final Verdict

### 1. Is the current architecture worthy of becoming a long-lived ERP framework?

Yes — conditionally.

The EntityDefinition-as-single-source-of-truth thesis is correct and differentiating. Compiler-first is architecturally superior to the runtime-discovery approach used by Frappe and Odoo. `PolicyEvaluator` as a replaceable interface is more sophisticated than any open-source ERP framework's authorization model.

The architecture becomes unworthy if:
- Compiler dependency graph is not added before v1.0
- `canPerform` remains fail-open
- `ComputedFrom` is not added before Finance begins
- `StateMachineDef` is not added before Finance begins

Fix these four things and the architecture deserves to stand for a decade.

---

### 2. Which Phase A item has the highest architectural leverage?

**A1 — Compiler dependency graph.**

Everything downstream depends on the compiler knowing the entity graph: migration fingerprints require topological order; system field injection must precede graph construction; cross-entity validation requires the graph; registry introspection requires topological sort; incremental compilation (Phase E) requires the graph as a serializable artifact. A1 is the single change that unlocks the most subsequent improvements.

---

### 3. Which proposed change should be rejected despite sounding attractive?

**Hot reload.**

Hot reload seems attractive for developer experience. It is dangerous. AWO's compiler runs at startup, validates entity definitions, and produces a `CompiledSchema` the entire runtime depends on. Hot reload means recompiling during live request handling — which requires the runtime to handle mid-request schema changes, the SDUI generator to handle cache invalidation during concurrent requests, and the router to handle route table changes without dropping in-flight requests.

This is not a developer experience enhancement. It is a distributed systems problem masquerading as a convenience feature. Build it wrong and you get race conditions in production. Defer until Phase C when the compiler is incremental and the runtime is explicitly designed for schema reload.

---

### 4. Which architectural mistake would be almost impossible to fix after v1.0?

**`NodeSection` as dual-purpose (form section + tab pane).**

After v1.0, external renderers will be built against the `widget.Node` IR. Every renderer author encountering `NodeSection` inside `NodeTabs` will implement the "parent detection" heuristic. This heuristic gets embedded into every renderer.

Adding `NodeTabPane` after v1.0 requires all existing renderers to update. But the old `NodeSection`-in-`NodeTabs` pattern still works — renderers must handle both patterns simultaneously, forever.

**Fix before v1.0:** Introduce `NodeTabPane`. The generator emits `NodeTabPane` for tab contents. `NodeSection` reverts to its single purpose: form section. This takes one hour to implement and prevents a decade of renderer confusion.

---

### 5. Which subsystem deserves a complete redesign before Finance begins?

**The runtime pipeline.**

The current five-stage pipeline is insufficient for Finance's lifecycle requirements. Finance needs: default injection, computed field evaluation, state machine enforcement, audit trail, domain event emission. All of these will be implemented in hooks if the framework provides only five stages. Hooks will become complex, ordered, and interdependent — exactly the hook spaghetti that framework design should prevent.

The 12-stage pipeline defined in Part 5 is the correct design. Stages 3, 6, 8 are no-op stubs at v1.0. But **their interfaces must exist at v1.0** so Finance module authors write against the correct abstractions from the first commit.

Redesign the pipeline interface before Finance begins. This is the most important pre-Finance change.

---

### 6. Which architectural decisions made today will be remembered as the turning point?

**First: EntityDefinition as a Go type, not a configuration format.**

Every other ERP framework uses YAML, JSON, Python classes, or XML. AWO uses a Go struct literal compiled into the binary. This means compile-time validation, IDE navigation, type-safe field references, and zero parse overhead at startup. When AI-generated modules become common (Phase D), the Go type is machine-verifiable in ways that YAML is not.

This decision will be remembered as the reason AWO could guarantee framework correctness at scale.

**Second: The compiler produces a deterministic, fingerprinted schema artifact.**

Two identical source trees produce identical `SchemaFingerprint` values. This enables cache correctness, deployment verification, and eventually module signing. When AWO becomes a framework with a public module registry (Phase F), module integrity verification will be based on the fingerprinted `CompiledSchema` artifact.

This decision will be remembered as the moment AWO became a framework you could trust, not just use.

**Third: PolicyEvaluator as a replaceable interface.**

By separating authorization declaration (`PermissionSet` in `EntityDefinition`) from enforcement (`PolicyEvaluator`), AWO enables enterprise customers to plug in their own policy engines — Casbin today, OPA tomorrow, a custom ReBAC engine the year after. No other open-source ERP framework offers this.

When an enterprise customer requires ABAC, a compliance-heavy industry requires a certified policy engine, or a government deployment requires on-premises policy evaluation with no Redis dependency — AWO accommodates all of these without changing a single entity definition.

This decision will be remembered as the one that made AWO enterprise-viable.
