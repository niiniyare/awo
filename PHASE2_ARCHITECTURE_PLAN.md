# Phase 2 Architecture Plan

**Status:** Planning document, revised to conform to `docs/adr/ADR-025-transactional-events-and-workflow-durability.md`
(registered in `docs/00-overview/DECISION_REGISTER.md`, Status: **Proposed** — not yet
Active/Frozen). Every claim below is sourced from a direct read of current source (file:line
cited throughout). **Where this document and ADR-025 could ever disagree, ADR-025 governs —
this revision exists specifically to eliminate every such disagreement, not to record them.**
Nothing here should be read as promoting ADR-025's own status; it remains Proposed, and this
plan is executable *against* that Proposed contract, not a claim that the contract is Frozen.

**Baseline:** commit `5a7e412` ("security: close phase 1 security boundary"). `gofmt`/`go
vet`/`go build`/`go test ./...` all clean (67 packages, real PostgreSQL + Redis). `-race`
unsupported on this sandbox (android/arm64), enforced in CI instead.

---

## 1. Executive Summary

Awo's core metadata-driven thesis — one `EntityDefinition` compiles once and drives
persistence, API, migration, reports, and docgen — **is real and correctly wired** for
those five subsystems. It **is not** real for OpenAPI (two independent
reimplementations), search (does not exist), or the mutation lifecycle's durability
guarantees (workflow dispatch and event publishing are both non-atomic with the mutation
that triggers them, contradicting their own doc comments).

**Two independent, unwired implementations of `def.ActionRuntime` exist**, and ADR-025 has
already resolved which one is canonical: `runtime.ActionContext`
(`runtime/runtime_action_context.go`, constructed via `NewActionContext`) is the
architecturally correct one — it is built directly on the real `events.Publisher` and
`workflow.WorkflowExecutor` interfaces, not redundant wrapper types. Its sibling,
`runtime.RuntimeFactory`/`runtime.defaultActionRuntime` (`runtime/action_runtime_impl.go`),
invents its own `EventBus`/`WorkflowRuntime` wrapper interfaces on top of the same
underlying concepts and is the one to be deleted once `ActionContext` is wired and
verified (ADR-025 §4, §20, §23). Neither has any production caller today —
`def.ActionContext.Runtime` (the field the real dispatch path,
`api/handler/crud.go`'s `EntityHandler.Action`, actually populates) is always left nil —
and neither implementation's `StartWorkflow` currently defers to the outbox despite both
interfaces' doc comments promising exactly that. This changes nothing about the scope of
Phase 2's work — it was always "finish wiring an existing, mostly-complete design" rather
than "invent one from scratch" — it only changes *which* existing struct gets wired.

The two original P0s remain exactly where Phase 1 left them: bulk import
(`ioport/importer.go`) bypasses the entire lifecycle by construction (it is typed to take
a raw `driver.EntityRepository`, not a `*runtime.Pipeline` — it cannot invoke hooks even if
someone tried to fix it without a signature change), and workflow-trigger dispatch has a
literal `// TODO: write to outbox table for guaranteed retry` (`api/service/entity.go:218`)
next to a log message that already claims the outbox exists (`"will retry via
outbox"`, line 212) when it does not.

A new, unrelated-severity finding surfaced while tracing the event outbox: even where a
transactional-outbox *interface* already exists with the right contract
(`events.Publisher`, `events/events.go:65-69`, whose doc comment requires "the context must
carry a transacted connection"), its **only concrete implementation violates that contract**
— `OutboxWriter.Publish` acquires a fresh connection from the pool instead of joining the
caller's transaction (`events/outbox/relay.go`, confirmed by source read). This means the
outbox pattern, as it exists in code today, could not deliver its core guarantee even if the
missing `events_outbox` table were created tomorrow. **A schema with this exact table name
already exists, unapplied, in `migrations/20260706000003_create_platform_support.up.sql`**
— discovered during ADR-025's own review pass, not by this document's original draft (see
§2.7) — and its disposition is now an explicit, required part of the implementation plan
(§10, Step 2), not something Phase 2 can silently ignore.

Phase 2, as scoped in this document and required by ADR-025, is: **fix the transactional-
outbox write to actually join the mutation's transaction; build one canonical
`events_outbox` table (reconciling, not duplicating, the orphaned migration already in the
tree); add the outbox write as the last in-transaction step of the canonical mutation
pipeline, with mandatory payload sanitization; repair and wire `runtime.ActionContext` as
the canonical action-execution abstraction, deleting its dead sibling; migrate bulk import
onto the same pipeline; and route workflow dispatch through the outbox to the existing
(currently bypassed) `workflow.WorkflowExecutor` abstraction, with explicit, tested
tenant-context failure handling and a genuine dead-letter state.** This closes both P0s.
It explicitly does **not** include the dependency-graph cleanups and wiring one-liners
`tasks.md`'s existing "Phase 2" section describes (§20 below reconciles this) — those are
real but orthogonal, lower-severity items that should be renumbered into a later phase so
"Phase 2" in the roadmap actually means what this document means by it. It also does
**not** claim, anywhere in this document, exactly-once delivery or exactly-once workflow
execution — ADR-025 §11/§19 prohibits that claim outright, in documentation and code
comments alike, and this revision has removed every instance of that phrasing the
document's earlier draft contained.

---

## 2. Current Architecture (verified from source)

### 2.1 Layering / dependency direction

Direct-import layering, verified via source reads and `go list`-equivalent grep across the
whole tree (not from `PACKAGE_DEPENDENCY_MAP.md`, which `tasks.md` 0.4 already flags as
stale):

```
def                    — leaf. Only stdlib + google/uuid + shopspring/decimal.
filter, tx, cache       — leaves, zero internal deps.
driver                  — def, filter.
auth                    — def.
registry                — def.
compiler                — auth, def, registry.        ← violates the documented rule (§2.2)
runtime                 — audit, cache, compiler, def, events, naming, workflow.
contrib/pgx             — compiler, def, driver, filter, internal/dberr, runtime,
                          runtime/tenant, tx, contrib/pgx/sqlbuild.
contrib/redis           — auth, cache, lock.
platform/iam            — audit, auth, cache, def, driver, filter, internal/dberr,
                          runtime, runtime/tenant.
api/router              — api/authz, api/handler, api/meta, api/middleware, api/sdui,
                          api/service, audit, auth, cache, compiler, contrib/pgx,
                          contrib/redis, def, driver, runtime, sdui/engine.
generator               — compiler, def.
bootstrap               — audit, compiler, registry.  (no module/ reference at all)
module                  — zero internal deps; zero callers anywhere else.
```

`runtime` is architecturally a fairly high-level aggregation point (it imports `compiler`,
`workflow`, and `events`, not just `def`) rather than a pure kernel-primitives package —
worth naming explicitly since Phase 2 will add more to this package (§15). `events`
itself remains `events → def` only, and ADR-025 requires this stay true even after adding
the `system_actor`/`correlation_id` fields (§10) — both are plain `string`-typed, not
imports of `audit.SystemActor`.

### 2.2 Compiler pipeline

`compiler.Compile()` (`compiler/schema.go:311`) runs, per its own inline phase comments:
Phase 1 stub building (335) → Phase 2 link-target resolution (342) → Phase 2b edge-target
resolution (354) → Phase 2.5 `CompiledLookup` construction for Link/LinkList fields (364) →
Phase 2.7 dependency-graph build + circular-FK/orphaned-link validation (378) → Phase 3
route emission (390) → Phase 4 `CapabilityGrant` emission from `PermissionSet` (395). The
result, `*compiler.CompiledSchema{Entities, ByName, Routes, CapabilityGrants, ...}`, is what
every downstream consumer reads from — when they do (§4).

### 2.3 The canonical mutation pipeline that exists today

Traced end-to-end from `api/handler/crud.go` through `api/service/entity.go` and
`runtime/pipeline.go`:

**Create** (`api/service/entity.go:53-89`): `pipeline.RunBeforeCreate` (61, outside TX) →
`repo.WithTx(ctx, func(txCtx) {...})` (67) wraps `repo.Create` (69) → `pipeline.RunAuditRecord`
(77) → `pipeline.RunAfterCreate` (80), any error rolling the transaction back via Phase 1's
`WithTx` fix → `s.startWorkflows(...)` (86), **after commit, on the outer `ctx`, not `txCtx`,
non-transactional, best-effort**.

**Update** (`entity.go:95-132`): `repo.Get` → `pipeline.RunBeforeUpdate` (outside TX) →
`WithTx` wrapping `repo.Update` → `RunAuditRecord` → `RunAfterUpdate`, then `startWorkflows`
after commit (same pattern).

**Delete** (`entity.go:138-157`): same shape — `RunBeforeDelete` outside TX,
`WithTx`-wrapped `repo.Delete` → `RunAuditRecord` → `RunAfterDelete`.

`runtime/pipeline.go`'s own top-of-file comment states this exact sequence, including
`→ Workflow start (outside TX)` as a deliberate, acknowledged design point — the framework's
own authors already knew workflow-start isn't atomic with the mutation before this planning
pass ever started; that acknowledgment is the direct textual root of P0-B.

### 2.4 Hook contract (verified against `def/hook.go`, not `docs/02-pipeline/HOOK_CONTRACT.md`, which is missing two of the nine real hook types)

`def.HookSet` (`def/hook.go:21-56`) has **nine** hook types:
`BeforeValidate, BeforeCreate, AfterCreate, BeforeUpdate, AfterUpdate, BeforeDelete,
AfterDelete, BeforeSave, AfterSave`. `docs/02-pipeline/HOOK_CONTRACT.md` documents only
six of them in detail (missing `BeforeValidate`/`BeforeSave`/`AfterSave` as first-class
`HookSet` fields, though its own header sketch does mention the stage names) and shows
`BeforeUpdateHook`/`AfterUpdateHook` with a one-argument signature; the real interfaces
(`def/hook.go:87,96`) take `(ctx, record, prev *EntityRecord)` — two arguments. Treat
`def/hook.go` as authoritative; the doc is drifted, not wrong in spirit.

All nine are invoked by `runtime/pipeline.go`, confirmed by reading every `RunBeforeX`/
`RunAfterX` method: `BeforeValidate → validateFields → BeforeSave → BeforeCreate` (or
`BeforeUpdate`) run before any transaction opens (`pipeline.go:133-162`, `231-257`);
`AfterCreate/AfterUpdate/AfterDelete → AfterSave` run inside the transaction, called by
`EntityService` between `PERSIST` and `COMMIT` (`pipeline.go:179-192`, `272-285`,
`307-321`).

**Every hook call is panic-safe today** — `safeCall` (`pipeline.go:471-478`) wraps every
single invocation in a `recover()`, converting a panic into `*HookPanicError` rather than
crashing the process or leaving the transaction in an undefined state. This is a real,
already-correct answer to "what happens if a hook panics" — no fix needed here.

**No `AfterCommit` hook exists anywhere** in this codebase, in any form — confirmed by a
repo-wide grep for `AfterCommit`, `PostCommit`, `OnCommit` (zero matches). Every "after
mutation" side effect that conceptually wants to run only once the transaction is durably
committed (workflow dispatch, event delivery) is instead a bare, non-transactional call
made after `WithTx` returns, using the outer request context. **ADR-025 §15 settles that no
`AfterCommit` hook type should be added** — see §14 below.

Hooks **can** call back into the repository (recursive mutation): they receive the same
`ctx`/`txCtx` the pipeline was given, so a nested `repo.WithTx` call from inside an
`AfterCreate` hook hits `existing.InTx()` (the Phase 1-verified short-circuit in
`contrib/pgx/repo.go`) and joins the *same* transaction rather than opening a second one —
transactional consistency for recursive mutation is a property of `WithTx`'s existing
design, not special-cased hook logic.

### 2.5 Audit semantics

`runtime.Pipeline.RunAuditRecord` (`pipeline.go:337-371`) is called by `EntityService`
*inside* the same transaction as the mutation, between `PERSIST` and the `after_save` hook
stage. Two independent, compile-time opt-outs exist: `EntitySchema.AllowAudit == false`
(ADR-023, checked first) and `audit.EntityAuditConfig.Enabled == false` (per-entity,
registered via `audit.Register` in `init()`, default `Enabled: true, Category:
CategoryData`, `audit/config.go:44-48`).

**The failure policy is real, deliberate, and already implemented** (`audit/failure.go`,
ADR-017): `audit.Apply` calls `AuditWriter.Write`; if it errors, `PolicyFor(category)`
decides whether the error **propagates** (rolls back the mutation — only for
`CategoryAdmin`/`CategorySecurity`) or is **silently suppressed** (logged at WARN, mutation
proceeds — the default for every other category, including the default `CategoryData`
every unregistered entity gets). This means the invariant "every successful,
externally-visible mutation has a corresponding audit record" is a **hard guarantee only
for Admin/Security-category entities** by explicit design — for ordinary data entities, a
transient `AuditWriter.Write` failure is an accepted, logged, non-fatal condition. This is
not a bug to fix; it is a documented trade-off (ADR-017) Phase 2 should preserve and state
explicitly rather than silently reinterpret as "audit always happens." **ADR-025 §6
deliberately does not extend this leniency to the outbox write** — see §8 below.

The existing, real audit-write transaction-participation mechanism —
`tx.QuerierFromContext(ctx)` (`tx/tx.go`), used by both `audit/pg_writer.go:94` and
`audit/transactional_writer.go:234` to obtain a `tx.Querier` and call `q.ExecSQL(...)`
against the caller's active transaction without importing `pgx` — is the exact pattern
`OutboxWriter.Publish` must be made to follow (§9, §11). This document's earlier draft
incorrectly cited `contrib/pgx`'s internal `setTenantContext` as the analogous pattern;
that function lives inside the driver and receives an already-resolved connection as a
direct parameter, it never extracts anything from `ctx`, so it is not actually analogous
to what a non-driver package like `events/outbox` needs. `tx.ConnFromContext` (the other
function in the same file) returns a bare `tx.Conn` with only an `InTx() bool` method — it
cannot execute SQL at all and must not be confused with `tx.QuerierFromContext`.

### 2.6 Bulk import (`ioport/importer.go`)

`Import()`'s package doc comment (lines 1-8) claims *"All framework invariants (immutable
fields, required fields, hooks) are enforced on import."* **This is false as written.**
`Import` takes `repo driver.EntityRepository[*def.EntityRecord]` directly (line 104) — it
has no reference to `*runtime.Pipeline` or `*api/service.EntityService` anywhere, and is
therefore structurally incapable of invoking hooks or field validation without a signature
change. All three format handlers (`importCSV:150`, `importJSON:225`, `importJSONL:257`)
call `repo.BulkCreate(ctx, batch)` directly. `BulkCreate` (`contrib/pgx/repo.go:451`) wraps
itself in one `WithTx` per flush (default `BatchSize: 100`, `ImportOptions.BatchSize:63`)
and never references `runtime.Pipeline` anywhere in its body — confirmed by reading the
method. No validation, no hooks, no audit record, on any imported row, ever, today. Tenant
context is established once for the whole import, not per row. With `SkipErrors: false`
(default), the first parse/DB error aborts the whole import; with `true`, bad rows are
recorded and import continues — but "bad" here means only what PostgreSQL's own column
constraints happen to catch, never a framework-level required/immutable-field rule.

`ioport/importer.go` **does** consult the compiled schema for input mapping — `csvRowToData`
(line 311) and `normalizeJSONRow` (line 357) use `schema.FieldsByName` for field-name/type
coercion during parsing. So the metadata-driven thesis holds for *parsing*, but not for
*lifecycle enforcement* — a precise distinction worth keeping since a Phase 2 fix should
preserve the parsing behavior and add the enforcement, not rebuild both.

### 2.7 Event outbox / workflow dispatch — the actual state, not the documented one

Two "frozen at v1.0" specs describe **two different, mutually inconsistent** durability
models (ADR-007, ADR-008 in `docs/00-overview/DECISION_REGISTER.md` — both now annotated
there as "Superseded in part by ADR-025 (Proposed)"), and the real code matches neither:

- `docs/09-events/EVENT_OUTBOX_SPEC.md` (ADR-008): domain events are written to a table
  called `event_outbox` **within the same transaction** as the triggering mutation, via a
  described `outbox.EventBroker` interface with Kafka/NATS/Redis-Pub/Sub adapters, under a
  top-level `awo.so/awo/outbox` package. **That package does not exist** (`ls outbox` →
  no such directory) — confirmed.
- `docs/08-workflow/OUTBOX_SPEC.md` (ADR-007): Temporal workflow starts are written to a
  *different* table, `workflow_outbox`, **outside** the entity transaction, in "a separate,
  short-lived transaction" with its own bounded local retry (3 attempts, 100ms apart) —
  a deliberately weaker guarantee than the event-outbox spec chose for domain events, for
  reasons neither doc explains. **Neither this table nor this write-after-commit-with-retry
  logic exists in code.**
- The actual, real, applied code (`events/`, `events/outbox/`) is a **third**, different
  design: `events.DomainEvent` (`events/events.go:35-63`) with
  `ID/TenantID/Type/EntityName/RecordID/ActorID/ActionName/Payload/OccurredAt` — no
  deterministic idempotency key, no correlation ID, no causation ID as struct fields today
  (ADR-025 §7/§8 adds `SystemActor`/`CorrelationID`, both `string`; explicitly does *not*
  add a deterministic idempotency key or a causation ID — see §13). `events.Publisher`'s
  doc comment (`events.go:65-69`) correctly demands "the context must carry a transacted
  connection" — but its only implementation, `events/outbox/relay.go`'s
  `OutboxWriter.Publish`, **violates its own interface's contract**: it calls
  `w.pool.Acquire(ctx)` for a fresh connection instead of joining the caller's transaction.
  The relay (`Relay.poll`, `relay.go:83-169`) is otherwise well-built: a hardcoded
  PostgreSQL advisory lock (`pg_try_advisory_lock(7777777)`) for single-active-relay
  coordination, `SELECT ... FOR UPDATE SKIP LOCKED` with a `LIMIT 50` batch and a flat
  `attempts < 5` cutoff with no backoff scheduling and no terminal dead-letter state
  despite a doc comment claiming one exists.
- **A table literally named `events_outbox`, with a schema closely matching
  `events.DomainEvent`, already exists as unapplied SQL text** in
  `migrations/20260706000003_create_platform_support.up.sql` (discovered during ADR-025's
  own adversarial review — this document's earlier draft incorrectly claimed "no
  `events_outbox` migration exists anywhere," which was false as written). That top-level
  `migrations/` directory has no `//go:embed` directive and no `migration.Register()` call
  anywhere in the repository — confirmed by exhaustive search — so this schema is never
  applied by any live bootstrap or migration path today; it is orphaned, not deployed. Its
  column named `type` (not `event_type`) independently confirms the real relay code's own
  column name is the correct one to standardize on (§10). Its `attempts < 5` partial index
  predicate is the exact flat-cutoff assumption ADR-025 replaces (§10, §11). This file's
  disposition is now an explicit implementation requirement, not something Phase 2 can
  silently ignore or silently duplicate (Step 2, §21).
- `EntityService.startWorkflows` (`api/service/entity.go:172-219`) does not touch any of
  the above. It holds a raw `temporalclient.Client` (imported directly, `entity.go:16`) and
  calls `s.temporal.ExecuteWorkflow(...)` synchronously, after `WithTx` returns, on the
  outer `ctx`. Its own comment (169-171) claims *"In production, the outbox pattern provides
  retry guarantees"* — false; nothing connects this call to `events/outbox` or any outbox
  table. Line 218 has a literal `// TODO: write to outbox table for guaranteed retry`
  immediately below a log line (212) that already says `"workflow start failed — will retry
  via outbox"` as if the outbox integration were live.
- `runtime.ActionContext.StartWorkflow` (`runtime/runtime_action_context.go:187-202`) has
  the **identical bug**: it calls `a.executor.Start(...)` immediately, synchronously,
  regardless of whether it is invoked inside or outside a `Tx` callback, with no outbox
  indirection at all — despite living inside the struct ADR-025 designates canonical (§1).
  Both call sites must be fixed together (§21, Step 4 and Step 6).
- `runtime.ActionContext.Publish` (`runtime_action_context.go:170-183`) has a second,
  independent bug: it serializes its payload as `[]byte(fmt.Sprintf("%v", event.Payload))`
  — not JSON, contradicting `events.DomainEvent.Payload`'s own documented format
  ("JSON-encoded map[string]any"). The fix is to call `json.Marshal(event.Payload)` instead
  (§10, §21 Step 4) — but **not** because `OutboxWriter.Publish`'s own marshaling
  (`relay.go:208`) is a correct pattern to imitate: that line has its own, separate,
  previously-undiscovered bug (it re-marshals an already-`[]byte`, already-JSON-encoded
  value, corrupting it into base64 — §10 discusses this defect and its distinct fix in
  full, tracked as Step 1 work). The two call sites' correct behaviors differ because their
  starting types differ (`ActionContext.Publish` receives an unserialized `any`;
  `OutboxWriter.Publish` receives an already-serialized `[]byte`) — do not treat one as a
  template for the other.
- `workflow.WorkflowExecutor` (`workflow/executor.go:74-88`) is a clean, correct interface
  with a working `NoopExecutor` (degraded mode) and `TemporalExecutor` (real SDK wrapper) —
  but neither `EntityService` nor `ActionContext` uses it correctly (both bypass its
  intended durability role); `router.RegisterOptions.Temporal` is typed as the raw
  `temporalclient.Client`, bypassing the abstraction at the wiring layer too.
- **Even if every gap above were fixed and a workflow start reliably reached Temporal, no
  worker would ever execute it**: `workflow.NewWorker` (which registers workflow/activity
  functions and starts polling a task queue) has zero production callers in either
  `cmd/server/main.go` or `cmd/awo/serve_impl.go` — both binaries only ever construct a
  *client*-side `temporalclient.Client` to start workflows, never a worker process to run
  them. This is a distinct, additional gap from P0-B's "durability of the start call"
  framing — it is "nothing is listening," not just "the start call might get lost" — and
  building a worker remains explicitly out of Phase 2's scope (§12, §19, §20).

### 2.8 The unwired `ActionRuntime` canonical-action abstraction — TWO dead implementations, one designated canonical

`def.ActionRuntime` (`def/action_runtime.go:19-64`) declares `Repo`, `Tx` ("workflow starts
issued inside fn are deferred to after commit so they do not trigger on rollback" — its own
doc comment), `Publish` ("delivery is guaranteed by the outbox pattern"), `StartWorkflow`
("If called inside Tx, the start is deferred to after the transaction commits"), `Notify`,
`InvalidateCache`, `Cache`, `Clock`, `Logger`, `TenantID`, `Actor`. `docs/13-actions/
ACTION_RUNTIME_REFERENCE.md` ("Frozen at v1.0") states this interface is "the execution
environment injected into every `ActionHandlerFunc`," and every code example in
`docs/13-actions/ACTION_HANDLER_GUIDE.md`/`CUSTOM_ACTIONS_EXAMPLES.md` (both frozen v1.0)
calls `action.Runtime.Repo/.Tx/.Publish/.StartWorkflow` — this interface is the framework's
own documented, mandatory API, not an aspirational future one.

**Two complete, independent, real implementations of this interface exist, and both are
dead code:**

1. `runtime.RuntimeFactory`/`runtime.defaultActionRuntime` (`runtime/action_runtime_impl.go`),
   injected with its own `EntityDriver`/`EventBus`/`WorkflowRuntime`/`NotificationService`/
   `CacheInvalidator` interfaces — a well-designed dependency-injection seam, but one that
   invents wrapper types (`EventBus`, `WorkflowRuntime`) on top of concepts
   (`events.Publisher`, `workflow.WorkflowExecutor`) that already exist as real interfaces
   elsewhere in the codebase.
2. `runtime.ActionContext`/`NewActionContext` (`runtime/runtime_action_context.go`),
   injected directly with the real `events.Publisher` and `workflow.WorkflowExecutor`
   interfaces (`ActionContextConfig.Publish`/`Executor` fields) — no redundant wrapper
   types. `NewActionContext` panics on any missing required dependency rather than
   silently degrading.

**ADR-025 designates implementation 2 (`runtime.ActionContext`) canonical** (§1, §4, §20,
§23) precisely because it is built on the real interfaces rather than inventing parallel
ones — adopting it requires no new adapter types beyond what `events`/`workflow` already
provide. Implementation 1 (`RuntimeFactory`/`defaultActionRuntime`) is to be deleted once
implementation 2 is wired and verified (§21, Step 4) — leaving both indefinitely would mean
two competing, unused implementations of the same interface sitting in the tree.

Three things are true simultaneously, all confirmed by direct source read:

1. **Both are dead code.** `NewRuntimeFactory` and `NewActionContext` each have zero
   callers anywhere outside their own files (including tests, for `NewActionContext` —
   its only reference outside its own package is `runtime/action_context_test.go`, which
   tests plumbing, not the deferred-commit semantics). `def.ActionContext` — the struct
   `ActionHandlerFunc` (`def/action.go:64`) actually receives — has a
   `Runtime ActionRuntime` field (`def/record.go:203`) whose doc comment says it is
   "Constructed and injected by the framework before the handler is called. Never nil" —
   but the real dispatch path, `EntityHandler.Action` (`api/handler/crud.go:142-170`),
   constructs `&def.ActionContext{Ctx, RecordID, Actor, Body}` with **no `Runtime` field
   set at all** — it is left as the zero value (nil interface). Any real action handler
   calling `actx.Runtime.Repo(...)` today would panic on a nil interface.
2. **Neither dead implementation delivers the guarantees its interface promises for
   `StartWorkflow`.** `defaultActionRuntime.StartWorkflow` (`action_runtime_impl.go:166-171`)
   and `ActionContext.StartWorkflow` (`runtime_action_context.go:187-202`) both call their
   injected executor immediately and synchronously, with no outbox indirection — the
   "deferred to after commit" behavior both interfaces' doc comments promise **does not
   exist in any Go code in this repository**. `ActionContext.Publish`, by contrast, is
   only one fix away from being correct (§2.7) — it already delegates to the real
   `events.Publisher`; once `OutboxWriter.Publish` itself is fixed (§9, §11) and
   `ActionContext.Publish`'s payload serialization is fixed (§2.7), calling `Publish` from
   inside `Tx`'s callback genuinely does become atomic, because `events.Publisher`'s
   contract will then actually be enforced.
3. **Git history is not useful here.** Both files were introduced in the same single
   bulk-import commit (`6ee6557`, "moved awo framework here," ~3430 lines) with no
   explanatory message distinguishing intent between the two — this is not a case of one
   being an earlier draft superseded by the other in this repository's own history; the
   distinguishing factor is architectural quality (§1), not chronology.

This changes the shape of the Phase 2 decision materially: Phase 2 is not choosing between
"build the canonical pipeline from nothing" and "extend `EntityService` ad hoc" — it is
finishing the wiring of an existing, mostly-complete design (`ActionContext`), repairing
its two concrete bugs (`StartWorkflow`'s missing outbox indirection, `Publish`'s payload
serialization), and retiring the sibling implementation that was never the right one to
build on. §21 Step 4 is the concrete implementation of this decision.

### 2.9 OpenAPI, SDUI, docgen, search — metadata-thesis wiring, per subsystem

| Subsystem | Classification | Evidence |
|---|---|---|
| Persistence (`generator/generator.go`) | **IMPLEMENTED** | imports `compiler, def` only; reads `*compiler.EntitySchema` directly |
| API routing (`api/router/router.go`) | **IMPLEMENTED** | `for _, es := range schema.Entities` |
| Migration | **IMPLEMENTED** | same as persistence |
| Docgen (`docgen/`) | **IMPLEMENTED** | imports `compiler, def` directly |
| Reports (`report/report.go`) | **IMPLEMENTED** | takes `*compiler.CompiledSchema` directly |
| Import/export (`ioport/`) | **PARTIALLY IMPLEMENTED** | schema-driven for field parsing/type coercion; bypasses lifecycle enforcement entirely (§2.6) |
| SDUI | **PARTIALLY IMPLEMENTED** | `sdui/adapt.FromCompiled` (`sdui/adapt/adapt.go:58`) explicitly converts `*compiler.EntitySchema` into `sdui/generator`'s own `EntitySchema`/`FieldDef` types — one canonical source feeding an explicit adapter, not a hidden duplicate, but a real translation step exists and the two `EntitySchema` type names collide across packages (naming hazard, not a defect) |
| OpenAPI | **DUPLICATED** | `api/openapi/openapi.go` and `generator/openapi/openapi.go` both import `compiler, def` directly with zero cross-reference between them — two independently-maintained reimplementations of entity→schema logic, exactly as `tasks.md` 4.4 already flags, confirmed still true |
| Search | **MISSING** | no `search/` package or full-text query builder exists anywhere; `Searchable: true` only drives index *generation* |

### 2.10 Transaction ownership

`contrib/pgx.Repository.WithTx` is the **sole** transaction boundary in the codebase —
confirmed no other `BeginTx`/transaction-management code exists outside `contrib/pgx`.
`EntityService` calls it once per Create/Update/Delete, wrapping exactly `repo.Create/
Update/Delete` + `RunAuditRecord` + `RunAfterX`. Nested calls (from a hook, or from
`ActionEntityRepo` once `ActionContext` is wired) reuse the same transaction via
`existing.InTx()`. Since Phase 1 (`tasks.md` 1.17), a panic inside the callback is
guaranteed to roll back and release the connection via a non-recovering `defer`, and
`pgxpool.Tx.Rollback` unconditionally calls `Release()` even if the SQL-level rollback
itself errors (verified from `pgxpool` library source in the Phase 1 security-closure
pass) — so the transaction-ownership model Phase 2 inherits is already sound at the
connection-lifecycle level; what Phase 2 adds is *more steps inside the same boundary*
(canonical outbox write via `tx.QuerierFromContext`, §9), not a new transaction-ownership
mechanism.

---

## 3. Phase 1 Baseline (what is already closed, not to be redone)

Fixed and regression-tested, commit `5a7e412`: `Repository.Query` ORDER BY SQL injection;
`updateSystem`/`BulkUpdate` SET-clause SQL injection and `tenant_id`/`id` write-path
protection; `Repository.Aggregate` and `report.GenerateSQL` equivalent-shape landmines;
`Repository.WithTx` panic-safety (connection-leak fix); tenant lifecycle enforcement;
session revocation tombstones; `/auth/logout`/`/auth/me`; connection-pool tenant isolation;
platform-admin re-verification (no RLS bypass exists). None of this is re-litigated here.

---

## 4. Current Unresolved P0s (re-confirmed this pass, not assumed)

- **P0-A — Bulk import bypasses validation/audit** (`tasks.md` 1.3): confirmed via fresh
  source read (§2.6) — structurally impossible to fix without changing `Import`'s signature
  to accept something with pipeline access.
- **P0-B — Workflow-trigger durability** (`tasks.md` 1.4): confirmed via fresh source read
  (§2.7) — not merely "missing a table," but four separable gaps: (a) no *applied* outbox
  table exists for workflow starts (though an orphaned, unwired schema does), (b) the one
  outbox mechanism that does exist (`events/outbox`) doesn't honor its own transactional
  contract, (c) *both* dispatch call sites (`EntityService.startWorkflows`,
  `ActionContext.StartWorkflow`) bypass outbox indirection identically, (d) even a
  successfully-dispatched workflow has no worker anywhere to execute it.
- **P1/CRITICAL, dormant — `ScopeOrganization` RLS composition bug** (`tasks.md` 1.18): out
  of scope for this document (explicitly not to be fixed here); re-confirmed still open,
  zero production exploitability (no entity declares this scope), blocks Phase 3.

---

## 5. Metadata-Driven Thesis Assessment

Per §2.9's table: **IMPLEMENTED** for persistence, API routing, migration, docgen, reports.
**PARTIALLY IMPLEMENTED** for import/export and SDUI (real, but with a real gap in each
case — lifecycle enforcement and a translation-layer duplication, respectively).
**DUPLICATED** for OpenAPI. **MISSING** for search. **CONTRADICTED BY CURRENT
ARCHITECTURE**: nothing outright contradicts the thesis (no subsystem maintains an entity
model that actively disagrees with the compiled schema) — the failures are omission
(search), duplication (OpenAPI), and incompleteness (import, SDUI's translation step), not
architectural contradiction.

---

## 6. Canonical Mutation Pipeline Proposal

Derived from what already exists (§2.3, §2.4, §2.8), not assumed from any example
sequence. The pipeline `EntityService` already implements is correct in its ordering; the
gap is entirely in what happens **after commit**, and in the fact that only one caller
(`EntityService`) implements it — bulk import and (once wired) actions do not.

```
Auth / Tenant-Org scope / RBAC        — middleware, outside the pipeline entirely (unchanged)
Input normalization + ApplyDefaults   — Pipeline.applyDefaults, outside TX
BeforeValidate hooks                  — outside TX
Field validation (required/type)      — outside TX
BeforeSave, BeforeCreate/Update/Delete hooks — outside TX
────────────────────────── [TX begins] ──────────────────────────
PERSIST (repo.Create/Update/Delete)
AUDIT RECORD (RunAuditRecord — ADMIN/SECURITY failure rolls back; others suppress)
AfterCreate/Update/Delete, AfterSave hooks — inside TX, error rolls back
Canonical outbox write (NEW — see §10), sanitized payload, via tx.QuerierFromContext — inside TX, same connection
────────────────────────── [TX commits] ─────────────────────────
Outbox relay (separate process/goroutine, NEW — see §11) dispatches, at-least-once,
duplicate dispatch possible and tolerated:
  - domain events → Subscribers
  - workflow-trigger events → workflow.WorkflowExecutor (existing interface, currently bypassed)
```

**Which steps MUST occur inside the transaction:** PERSIST, the audit write, `AfterX`/
`AfterSave` hooks, and the canonical outbox write. All three of the first are already
inside the transaction today; the outbox write is the one addition. This is the single
structural change Phase 2 makes to the pipeline itself: **add an outbox-row insert as the
last step before commit**, using `tx.QuerierFromContext(txCtx)` to obtain the same
transaction-participating `tx.Querier` the audit write already uses (§9), never a fresh
connection from the pool.

**Which steps MUST occur before the transaction:** everything validation/authorization-shaped
(`BeforeValidate`, field validation, `BeforeSave`/`BeforeCreate`/`BeforeUpdate`/
`BeforeDelete`) — unchanged from today, and correct per `docs/02-pipeline/HOOK_CONTRACT.md`'s
own normative rule ("`before_validate` hooks MUST NOT perform database writes").

**Which steps MUST occur only after commit:** actually dispatching to Temporal / actually
delivering to subscribers. This is what the relay is for (§11) — it reads already-committed
outbox rows, so "after commit" is structural, not a timing race.

**Which failures must abort the transaction:** any `BeforeX` hook error (already true,
pre-transaction); any `AfterX`/`AfterSave` hook error (already true, per Phase 1's
`WithTx` panic-safety fix); an `AuditWriter.Write` failure for `CategoryAdmin`/
`CategorySecurity` entities (already true, ADR-017); a failure to insert the new outbox
row (new — **unconditional**, regardless of the entity's audit category, per ADR-025 §6 —
see §8).

**Which failures must retry asynchronously:** everything *after* the outbox row is durably
committed — relay delivery to a `Subscriber`, and `workflow.WorkflowExecutor.Start` calls,
**at-least-once, with duplicate dispatch possible and explicitly tolerated, never
exactly-once** (§11). This is exactly what the relay's existing `attempts`-counting design
(`events/outbox/relay.go`) already does for domain events in outline; §11 replaces its flat
cutoff with scheduled exponential backoff and a genuine terminal dead-letter state, and
extends the same mechanism to workflow-trigger events.

**Where idempotency is required:** at the relay's delivery boundary — a `Subscriber`/
`WorkflowExecutor.Start` call may be retried after a crash between "delivered" and "marked
delivered" (§13) — never inside the transaction itself, where PostgreSQL's own
transactional guarantees already make PERSIST+audit+outbox-insert **atomic** (all three
commit together or none do — this is a claim about within-transaction atomicity, not about
downstream delivery, and this document does not use the phrase "exactly-once" to describe
either).

**Where authorization must occur:** unchanged — `api/authz.RequirePermission`, at the HTTP
middleware layer, before `EntityService` is ever invoked. Out of scope for this document
(already Phase-1-verified, not touched).

**Where tenant context must be established:** unchanged — `TenantResolver` middleware, or
(for background/relay code) explicitly reconstructed from the outbox row's own `tenant_id`
column before any repository call, with an explicit, tested policy for what happens when
that reconstruction itself fails (§9, §11).

This sequence becomes the single foundation for API mutations (already there), bulk import
(§7, migrate onto it), actions (§2.8, wire `ActionContext` to use it), and any future
background job that mutates entities.

---

## 7. Bulk Import Architecture

**Two candidate architectures, compared:**

**(A) Route `ioport.Import` through the same canonical mutation path `EntityService`
uses (or the lower-level `runtime.Pipeline` + `contrib/pgx.Repository.WithTx` combination
directly, if going through the full HTTP-shaped `EntityService` is too heavy for a
CLI/batch context).** Every imported row gets `BeforeValidate`/validation/`BeforeCreate`/
`AfterCreate`/audit/outbox exactly like a `POST` would. Cost: one hook invocation + one
audit write + one outbox write per row, and (if `EntityService.Create` is reused as-is)
one `WithTx` per row instead of one per 100-row batch — a real, measurable throughput
cost avoided by keeping the existing flush granularity (below).

**(B) Keep `BulkCreate` as a distinct, explicitly-documented "trusted, pipeline-exempt"
batch path**, and make `ioport.Import` an explicit, narrow exception that performs its own
validation pass (reusing `Pipeline.validateFields`/`applyDefaults`, not reinventing them)
before handing already-validated rows to `BulkCreate`, plus a synthetic audit write per
imported row (or one summary audit record per import job) written in the same transaction
as the batch insert.

**Recommendation: (A), with per-flush-batch pipeline invocation, not per-row transactions
— unchanged from the earlier draft's conclusion, and consistent with ADR-025 §14's
bulk-import contract.** Option (B) is exactly the "bulk path → separate
validation/audit/hooks" architecture this plan was asked to avoid, because it requires
`ioport` to reimplement (not merely call) `Pipeline.validateFields`/hook invocation, which
will drift the moment a hook gains new semantics `ioport`'s reimplementation doesn't know
about. The concrete mechanism: change `Import`'s signature to accept a `*runtime.Pipeline`
(or an `EntityService`-shaped interface) alongside the repository; for each row, call
`pipeline.RunBeforeCreate` (validation/hooks, outside TX, cheap) and then batch the
resulting already-validated records into the *existing* per-`BatchSize`-flush `WithTx` +
`BulkCreate` call, calling `pipeline.RunAfterCreate`, `pipeline.RunAuditRecord`, **and the
canonical outbox write (sanitized payload, one row per successfully-inserted record — not
one summary event per flush)** for each record **inside** that same transaction,
immediately after the batch insert returns their generated IDs. This keeps `BulkCreate`'s
existing batching-for-performance design (one transaction per 100 rows, not one per row)
while making every row's `BeforeX`/`AfterX`/audit/outbox behavior identical to the
single-record path — one source of truth for what "creating a record" means, at whatever
granularity the batch happens to use for the SQL itself. **ADR-025 §14 confirms this
exact design and explicitly excludes resumability/retry-the-whole-job semantics from
Phase 2** — this remains a self-contained, later-phase feature (§19, §20).

`SkipErrors` semantics need one explicit decision (not yet made in this document): a
`BeforeValidate`/validation failure on row N of a 100-row flush should behave the same way
`SkipErrors` already governs a DB-constraint failure today — record it in `ImportResult.Errors`
and continue (if `true`) or abort the whole import (if `false`) — this requires validating
all rows in a flush *before* committing that flush's batch insert, which the current
per-flush `WithTx` structure already accommodates naturally (validate the whole flush,
then insert only the rows that passed, in one transaction).

---

## 8. Audit Architecture

No architectural change proposed. §2.5's findings stand as the current, correct design:
audit runs inside the mutation's transaction; failure policy is category-based (ADR-017);
the only Phase 2-relevant addition is that the **new outbox row insert** (§10) uses the
*same* transaction-participation mechanism as the audit write (`tx.QuerierFromContext`,
§9) but a **stricter, unconditional** failure policy — per ADR-025 §6, an outbox-insert
failure always rolls back the transaction, for every entity, regardless of
`EntityAuditConfig.Category`. This is a deliberate divergence from ADR-017's audit-failure
leniency, not an oversight: a missing audit record for an ordinary data entity is an
accepted, logged, non-fatal compliance gap (ADR-017's own reasoning); a missing outbox
record for a mutation that was supposed to fire a domain event or dispatch a workflow is
not a compliance gap, it is the exact bug (P0-B) this document exists to close, so it
fails closed unconditionally.

---

## 9. Transaction Model

No new transaction-ownership mechanism needed (§2.10) — `contrib/pgx.Repository.WithTx`
remains the sole owner, `EntityService`/`Pipeline`/hooks/audit all already correctly operate
within its boundary or explicitly outside it. The one addition: whatever component performs
the canonical outbox insert (§10) must receive the same `txCtx` `RunAuditRecord` already
receives, and must call **`tx.QuerierFromContext(txCtx)`** (`tx/tx.go`) to obtain a
`tx.Querier` and then `q.ExecSQL(ctx, insertSQL, args...)` — exactly the pattern
`audit/pg_writer.go:94` and `audit/transactional_writer.go:234` already use, **not**
`tx.ConnFromContext` (which returns a bare `tx.Conn` with no `ExecSQL` method and cannot
execute anything). This is the exact, minimal fix for `events/outbox`'s current contract
violation (§2.7), not a new mechanism. The required data flow, stated as the invariant it
is:

```
mutation transaction (repo.WithTx)
    ↓
audit write        (pipeline.RunAuditRecord, via tx.QuerierFromContext — unchanged)
    ↓
outbox write        (publisher.Publish, via tx.QuerierFromContext — this document's fix)
    ↓
COMMIT
    ↓
relay (separate process/goroutine, separate connection — no relationship to the
       mutation's own transaction beyond reading its already-committed result)
```

`OutboxWriter`'s `pool *pgxpool.Pool` field is no longer needed by `Publish` once this
lands — it remains necessary only for `Relay.poll()` (an unrelated, non-transactional,
background read path with no caller transaction to join).

**The in-transaction outbox insert does not establish a second, independent tenant
context — it inherits the one the mutation already established, by construction, not by
coincidence.** `Repository.WithTx` calls `setTenantContext` exactly once, immediately after
opening the transaction and before `fn(txCtx)` runs (`contrib/pgx/repo.go:685-689`) —
`fn` is the callback containing `PERSIST → RunAuditRecord → AfterX → outbox write`. Because
`tx.QuerierFromContext(txCtx)` (this section's fix) resolves to the *same* `*pgConn`/
`pgxlib.Tx` the mutation and audit write already used, the outbox `INSERT` executes as one
more statement inside a PostgreSQL session whose RLS session variables (`awo.tenant_id` and
friends) were already set once for the whole transaction — no second
`set_tenant_context` call is needed, or should be added, for the outbox write itself. This
is part of the same atomic mutation+audit+outbox boundary §5/§6 above describe, not a
separate property to implement.

**This is categorically different from, and must not be confused with, the relay's own
tenant-context obligation (§11 item 5, §21 Step 3b).** The in-transaction write above runs
*inside* the original request's own transaction and process — there is only one tenant
context in play, already correctly established, and nothing to restore. The relay, by
contrast, runs in a **separate process/goroutine**, on a **separate connection**, reading an
already-committed row potentially seconds, minutes, or (per the backoff schedule) longer
after the original request's transaction and tenant context have both ended — it has no
ambient tenant context at all until it deliberately reconstructs one from the outbox row's
own `tenant_id` column. Conflating these two cases — assuming the relay "already has" tenant
context because the write path does — would be exactly the kind of gap `tasks.md` 1.4
already found and this document's §11 item 5 exists to close.

---

## 10. Outbox Architecture

**One canonical outbox, not two — and it must reconcile with, not ignore or duplicate, the
orphaned schema already in the tree.** §2.7 found the codebase's own docs disagree with
each other (event outbox = same-transaction per ADR-008; workflow outbox = separate
post-commit transaction with bounded local retry per ADR-007) and the real code matches
neither; it also found a table literally named `events_outbox` already exists, unapplied,
in `migrations/20260706000003_create_platform_support.up.sql`. Given `events/outbox`'s
relay machinery (advisory-lock coordination, `SKIP LOCKED` batch claiming, attempt
counting) is real and mostly correct, **the recommendation, matching ADR-025 §3/§7/§10
exactly, is to extend `events.DomainEvent`/`events/outbox` to carry workflow-trigger
dispatch too**, rather than building a second `workflow_outbox` table per ADR-007. A
`WorkflowTriggerFired` `EventType` (alongside the existing
`EventCreated`/`EventUpdated`/`EventDeleted`/`EventActionFired`) with a `Subscriber`
implementation that calls `workflow.WorkflowExecutor.Start` unifies both P0-B and the
domain-event pattern under one durability mechanism, one migration, one relay, one set of
adversarial tests.

**Canonical schema** (this supersedes the earlier draft's schema block entirely — the two
must not be read as alternatives):

```sql
CREATE TABLE events_outbox (
    id              uuid PRIMARY KEY,          -- events.DomainEvent.ID (UUIDv7)
    tenant_id       uuid NOT NULL,             -- uuid.Nil permitted for platform-level events
    type            text NOT NULL,             -- events.DomainEvent.Type — matches the live relay.go column name; do NOT rename to event_type
    entity_name     text NOT NULL,
    record_id       uuid NOT NULL,
    actor_id        uuid,                      -- def.Actor.UserID/ServiceAccountID; NULL for system actors
    system_actor    text,                      -- audit.SystemActor's string form, e.g. "system:outbox-relay"; NULL for human/service-account actors. A plain string field on events.DomainEvent — events does NOT import audit to obtain this type (see §2.1)
    action_name     text,
    correlation_id  text,                      -- opaque identifier, NOT a uuid column; sourced from audit.RequestContext.RequestID (itself a plain, unvalidated string) when present, NULL for background-originated events
    payload         jsonb,                     -- MUST be sanitized before insert — see the payload-security requirement below
    occurred_at     timestamptz NOT NULL,
    delivered_at    timestamptz,               -- NULL = pending
    attempts        int NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    last_error      text,
    dead_at         timestamptz                -- NULL until moved to terminal failure — see §11
);
CREATE INDEX events_outbox_pending ON events_outbox (next_attempt_at)
    WHERE delivered_at IS NULL AND dead_at IS NULL;
```

Field-by-field disposition, reconciled against the orphaned migration, the live relay
code, and ADR-025 §7/§8 (this plan introduces no field ADR-025 does not already specify):

- `id`, `tenant_id`, `type`, `entity_name`, `record_id`, `actor_id`, `payload`,
  `occurred_at` — map directly to `events.DomainEvent`'s already-shipped fields and the
  orphaned migration's existing columns. **`type`, not `event_type`** — this is the one
  correction to this document's own earlier draft schema, made to match code that already
  ships today, not a new design choice.
- `system_actor` (`text`) and `correlation_id` (`text`) — additive fields this document
  requires on both the table and `events.DomainEvent`; both are opaque strings, never
  `uuid`-typed, never sourced by importing `audit` into `events` (§2.1).
- **A causation ID is deliberately NOT added.** No event chain in the current architecture
  is deep enough to need one, and adding it speculatively would be exactly the unjustified
  abstraction this document was asked to avoid. If one is ever needed, it should be `text`
  for the same reason `correlation_id` is.
- **A request ID distinct from `correlation_id` is deliberately NOT added.** This plan
  treats them as the same concept — `audit.RequestContext.RequestID`, when present,
  populates `correlation_id` directly; there is no second, separate identifier to carry.
- `next_attempt_at`/`dead_at` — new columns replacing the orphaned migration's/live relay's
  flat `attempts < 5` cutoff with scheduled exponential backoff and a genuine terminal
  state (§11).
- **No deterministic "idempotency key" field is added, contrary to this document's own
  earlier draft, which proposed one.** ADR-025 §12 explicitly rejects this: `events_outbox.id`
  (already unique by primary key), Temporal's own `WorkflowID`-based deduplication (§13),
  and the row's own `attempts` counter are the only identities the mechanism requires. This
  correction removes that proposal entirely rather than leaving it as an unused,
  contradicted alternative.
- **`org_id` is deliberately NOT added.** `ScopeOrganization`'s RLS boundary is currently
  broken (`tasks.md` 1.18, out of scope here) and zero entities use it; adding `org_id` now
  would be premature infrastructure for a scope that doesn't safely exist yet.

**Payload security — mandatory, not optional.** Per ADR-025 §7/§10a, `payload` MUST NOT
contain fields marked `Sensitive: true` on their `FieldDef`, full unrestricted record
snapshots, secrets, credentials, or access/refresh tokens — this restriction is carried
forward from ADR-008's original design and is *more* load-bearing now than it was under
ADR-008, because this document makes an outbox write part of every qualifying mutation
rather than an opt-in path. **Any payload built from entity field data
(`record.Data`/`record.CustomFields`) MUST be passed through the existing
`audit.Sanitizer.Strip(entityName, snapshot)` (`audit/sanitizer.go:55` — already used on
the audit-record path for exactly this purpose) before being marshaled into `payload`.**
This document does not propose a second, independent sanitization implementation —
reusing `audit.Sanitizer` is required. A real PostgreSQL integration test (§21 Step 3,
§22) must prove a `Sensitive: true` field's value does not appear in the persisted
`events_outbox.payload` column for a mutation on an entity that declares one.

**`Sanitizer.Strip`'s protection is flat, not recursive — the implementation must account
for this precisely, not assume broader coverage than the function actually provides.**
`Strip` (`audit/sanitizer.go:55-79`) iterates only the top-level keys of the map it is
given: a `Sensitive: true` field whose *own* value happens to be a nested map or slice has
that entire value replaced with `"[REDACTED]"` wholesale (safe — the sensitive field
disappears entirely), but `Strip` has no visibility into nested structure sitting *below* a
non-sensitive top-level key, and cannot discover or redact a sensitive-shaped value buried
inside an ordinary field's nested object. This is not treated as a defect in `Strip` to fix
in this document — no entity-definition mechanism in this framework currently declares
sensitivity below the top-level field, so `Strip`'s flat behavior already matches
everything `FieldDef.Sensitive` can express — but it does mean the implementation must not
construct an outbox payload by serializing an arbitrary, unreviewed object graph and
expecting `Strip` to clean it afterward. **The payload must be an intentionally, explicitly
constructed, bounded set of values — never an unrestricted copy of the HTTP request body,
the full entity before/after snapshot, credentials, passwords, access/refresh tokens,
session identifiers or session data, API keys, or any other structure that could carry
arbitrary nested secrets below the level `Strip` actually inspects.** Where a field's own
value is itself structured, the implementation must ensure that structure does not carry
sensitive nested content by construction (e.g. by not including such a field in the payload
at all), rather than by assuming a recursive guarantee `Strip` does not provide. This
document does not modify `audit.Sanitizer` in this planning pass, and does not require
modifying it for Phase 2 — implementation uses its existing, flat contract as-is.

**Audit snapshots and outbox payloads are event contracts for different audiences, and must
not be treated as interchangeable, even though both are sanitized via the same `Strip`
call.** `AuditRecord.BeforeData`/`AfterData` (`audit/record.go`) exist to answer "what was
this record's state, for compliance/forensic review" — an audit-scoped artifact, internal
by design. `events_outbox.payload` exists to answer "what does a downstream `Subscriber` or
workflow need to act on this event" — a contract consumed by code running outside the
mutation's own transaction, and potentially outside the framework entirely once real
`Subscriber` implementations exist. Reusing `Sanitizer.Strip` for both is correct and
required (one sanitization mechanism, per this document's single-source-of-truth
principle) — but the two payloads may legitimately differ in *shape* even though both pass
through the same sanitizer, because they serve different purposes and different audiences.

**Exact outbox payload contents, specified here so two different implementers converge on
the same shape rather than each inventing one:** `events.DomainEvent` already separates
identity/routing metadata — carried on the struct's own fields (`ID`, `TenantID`, `Type`,
`EntityName`, `RecordID`, `ActorID`, `ActionName`, `OccurredAt`, plus this document's
additions `SystemActor`/`CorrelationID`) — from the event's business content, carried
exclusively in `Payload []byte`. Nothing below adds a new struct field beyond what this
section's schema and ADR-025 already specify; this is guidance on what belongs inside the
existing `Payload` field, not a proposal to extend `DomainEvent` further.

- **Belongs in `DomainEvent`'s own fields (metadata) — never duplicated a second time
  inside `Payload`:** entity identity/type (`EntityName`), record identity (`RecordID`),
  event/action type (`Type`/`ActionName`), tenant identity (`TenantID`), actor identity
  (`ActorID`/`SystemActor`), correlation identity (`CorrelationID`). A `Subscriber` reads
  these from the envelope; it must not need to also find a copy of any of them inside
  `Payload`. Organisation identity is deliberately absent from both the envelope and
  `Payload` — this section's exclusion of `org_id` pending the `ScopeOrganization` blocker
  (§16, §21) applies identically here; do not add an `org_id` key inside `Payload` as a
  workaround for the column's absence.
- **Belongs inside `Payload` (the sanitized business content):** for a lifecycle event
  (`EventCreated`/`EventUpdated`/`EventDeleted`), the record's own field data
  (`record.Data` merged with `record.CustomFields` — matching what the audit record's own
  `AfterData` already captures, per §2.5/§8 above) passed through `Sanitizer.Strip` —
  **not** a raw, unsanitized snapshot, and **not** the original HTTP request body verbatim
  (the request body may contain fields the entity schema rejects, fields absent from the
  final persisted record, or pre-defaulting/pre-normalization values — the payload must
  reflect the persisted record, never the request). For a `WorkflowTriggerFired` event
  (§21 Step 6), the JSON-marshaled `def.ActionWorkflowSpec` (`WorkflowFn`, `TaskQueue`,
  `WorkflowID`, `Input` — already an existing type) — the workflow's own declared input,
  not a second copy of the triggering record's full data unless the workflow's own
  `InputBuilder` explicitly chooses to include it.
- **Never belongs inside `Payload`, under any circumstance:** raw request bodies, full
  unsanitized entity snapshots, credentials, passwords, access/refresh tokens, session
  identifiers or session data, API keys, or any nested structure not explicitly
  constructed and sanitized per the rule above.

**Two independent payload-serialization defects exist on the two sides of the outbox write
path, and both must be fixed — they are not the same bug, do not share a root cause, and do
not share a fix.**

**(A) `ActionContext.Publish`'s payload is not JSON at all — a missing encoding step.**
`runtime.ActionContext.Publish` (`runtime/runtime_action_context.go:170-183`) currently
serializes its payload as `[]byte(fmt.Sprintf("%v", event.Payload))`, which does not
produce JSON and violates `events.DomainEvent.Payload`'s own documented format
("JSON-encoded map[string]any"). Here `event.Payload` (`def.ActionEvent.Payload any`) is an
arbitrary Go value supplied by the action handler and has never been serialized before this
call. **Required fix: serialize it exactly once**, with `json.Marshal(event.Payload)` (or an
equivalent canonical encoder), producing genuine JSON bytes for the first time. A concrete
integration test (§21 Step 4, §22) must prove: (1) an event is published through
`ActionContext.Publish`, (2) the resulting `events_outbox` row is persisted, (3) its
`payload` column is valid JSON, (4) that JSON unmarshals successfully, and (5) the
unmarshaled values match what was originally published — end-to-end round-trip proof, not
a unit test of `json.Marshal` in isolation. This fix belongs in Step 4 (§21) alongside the
rest of `ActionContext`'s repair.

**(B) `OutboxWriter.Publish` re-serializes an already-serialized payload, corrupting it —
an extra, incorrect encoding step.** Found during this document's own implementation-
readiness review (`PHASE2_IMPLEMENTATION_READINESS_REVIEW.md` §7), not by either earlier
planning pass: `OutboxWriter.Publish` (`events/outbox/relay.go:208`) calls
`json.Marshal(e.Payload)`, where `e.Payload` (`events.DomainEvent.Payload`) is typed
`[]byte` and, per the field's own documented contract, is *already* JSON-encoded by the
time any caller sets it — exactly the value (A) above is responsible for producing.
Calling `json.Marshal` on an already-`[]byte` value does not pass the bytes through
unchanged: Go's `encoding/json` has no special case for a bare `[]byte` other than encoding
it as a base64 string literal. The practical effect: a payload that was correctly
`{"foo":"bar"}` going in comes out of this line as the JSON string
`"eyJmb28iOiJiYXIifQ=="` — a base64-encoded string, not the original object — before being
written into the `jsonb` `payload` column. This has never manifested as a production
failure because `OutboxWriter.Publish` has zero production callers today (confirmed,
repo-wide); **this document's own Steps 3, 4, and 6 are exactly the work that gives it its
first callers**, so this defect must be closed as part of Step 1, in the same function
already being opened for the transaction-join fix — not deferred to a later step, and not
treated as pre-existing, already-correct behavior to imitate elsewhere.

**Required fix for (B), stated as behavior, not as a prescribed code expression** (the
plan does not mandate a specific technique beyond what the existing interface already
requires): `OutboxWriter.Publish` must preserve `e.Payload`'s existing encoded JSON bytes
unchanged as the value written into the `payload` column — it must not pass `e.Payload`
through `json.Marshal`, or any other re-encoding step, a second time. It must continue to
handle a nil or empty `e.Payload` safely per the existing contract
(`Payload []byte \`json:"payload,omitempty"\`` — an absent payload is an already-supported,
valid state, not an error condition to introduce). A non-nil `e.Payload` that is not valid
JSON should be rejected or otherwise safely handled rather than silently inserted — the
`jsonb` column type will reject malformed input at the database level regardless, but the
implementation should not rely solely on that as its only validation. The verified result,
via a real PostgreSQL test: the `payload` column, read back and queried as JSON (e.g. via
`payload->>'field'` or an equivalent JSON-path read), yields the original object's fields —
not a quoted base64 string.

**Required regression test for (B), using the actual transaction-bound path, not a unit
test of the encoding function in isolation:**
1. Construct a `events.DomainEvent` whose `Payload` is pre-encoded JSON representing an
   object with several fields (e.g. `[]byte(`{"foo":"bar","count":3}`)`).
2. Publish it through `OutboxWriter.Publish`, called from inside a real PostgreSQL
   transaction obtained the same way Step 1's transaction-join fix requires (via
   `tx.QuerierFromContext` inside a `WithTx`-equivalent callback) — not directly against a
   bare pool connection.
3. Read the resulting `events_outbox` row back from the database.
4. Verify the stored `payload` value is structurally equivalent to the original JSON
   object — every field and value present and correctly typed — not merely that some bytes
   were stored.
5. Explicitly assert the stored value is **not** a JSON string containing base64 data (a
   negative assertion — e.g. assert the stored value is a JSON object, not a JSON
   string-typed scalar) — proving the specific defect this test exists to catch cannot
   recur silently.

**These two fixes must not be confused with each other, merged into one description, or
applied to the wrong call site:** (A) is "serialize a Go value into JSON for the first
time" (a missing encoding step, fixed in `ActionContext.Publish`, Step 4); (B) is "stop
re-encoding bytes that are already JSON" (an extra, incorrect encoding step, fixed in
`OutboxWriter.Publish`, Step 1). Applying (A)'s fix to (B)'s call site, or vice versa, would
not close either defect — `ActionContext.Publish` genuinely needs a `json.Marshal` call it
currently lacks, and `OutboxWriter.Publish` needs to stop calling one it currently has.

**The orphaned migration's disposition, explicitly required, not left to implementer
discretion:** `migrations/20260706000003_create_platform_support.up.sql`'s existing
`events_outbox` table must not be silently left in place unreconciled, and a second,
differently-shaped `events_outbox` migration must not be created alongside it — that is
exactly the "two competing schema definitions for one table name" failure this document
exists to prevent. The required action (detailed as Step 2, §21): treat the orphaned file
as superseded historical scaffolding (consistent with this project's general policy of not
deleting superseded artifacts), add a comment at its top noting it is unapplied and
superseded by the migration that actually implements the schema above, and write a new,
properly `//go:embed`-wired migration (following the same pattern as
`migration/bootstrap/migrations.go` or one of the `platform/*/migrations` packages)
containing the complete schema above — including the columns and corrected index the
orphaned file lacks. If a future, separate decision resolves `tasks.md` 5.2's open question
about the top-level `migrations/` directory's intended role by actually wiring it in, the
orphaned file may instead be edited in place as part of that broader decision — but not as
an incidental side effect of this document's implementation.

**PostgreSQL as durable source of truth: yes**, unconditionally, for the reasons this
framework already commits to everywhere else (RLS, tenant lifecycle, session audit trail) —
and because `mutation + audit + outbox` in one transaction is only possible at all if the
outbox row lives in the same database as the mutation. This is not a new architectural
commitment; it is completing one the framework already made.

---

## 11. Event Relay Architecture

`events/outbox/relay.go`'s existing design (advisory lock for single-active-relay
coordination, `SELECT ... FOR UPDATE SKIP LOCKED LIMIT 50`) is architecturally sound and
should be kept, not replaced. Required changes, reconciled with ADR-025 §9/§16:

1. **Fix `OutboxWriter.Publish` to honor its own interface contract** — use
   `tx.QuerierFromContext(ctx)` (§9), never `tx.ConnFromContext` or `pool.Acquire`, to write
   within the caller's transaction. This is the single highest-leverage fix in this entire
   document: it turns an outbox mechanism that *cannot* currently deliver atomicity into
   one that can, without changing its public interface at all.
2. **Row claiming stays as-is**: the session-level advisory lock (held for one `poll()`
   cycle's full duration) plus `SELECT ... FOR UPDATE SKIP LOCKED` correctly serializes
   concurrent relay instances cluster-wide — confirmed by direct trace, not changed here.
   (Note for implementers: the row-level `FOR UPDATE` lock itself is released the instant
   its own implicit auto-commit statement completes, before `deliver()` ever runs — it is
   the session-level advisory lock that actually does the serializing work; `SKIP LOCKED`
   is decorative given the advisory lock, not a bug.)
3. **Per-event dispatch timeout, required.** `deliver()`'s loop must wrap each
   `Subscriber.HandleEvent`/`WorkflowExecutor.Start` call in a bounded
   `context.WithTimeout`. The entire poll cycle — advisory lock, connection, and up to 50
   events' worth of external calls — currently runs serially with no per-call bound; adding
   Temporal dispatch (a network call to an external system with a different failure profile
   than an in-process `Subscriber`) to this same loop without a timeout risks one slow or
   hung Temporal call blocking delivery of every other pending event, including unrelated
   domain events, for an unbounded duration. **External dispatch must never occur while the
   relay's session-level advisory lock is the only thing preventing concurrent processing
   of unrelated events from making progress** — the timeout is what bounds how long that
   lock can be effectively held hostage by one slow call.
4. **Retry/backoff/dead-letter, required.** Replace the flat `attempts < 5` cutoff with
   `next_attempt_at`-scheduled exponential backoff (1s/2s/4s/8s/16s, capped at 5 minutes —
   the schedule either frozen doc originally proposed, adopted here as reasonable and
   uncontested) and a genuine terminal state (`dead_at` set, row excluded from the pending
   index) once backoff is exhausted, closing the gap between the package's own doc comment
   (claims a "dead-letter table" exists) and its actual behavior (a poison row simply stops
   being retried, silently, forever, with no operator-visible signal). A dead-lettered
   event must remain queryable (`SELECT * FROM events_outbox WHERE dead_at IS NOT NULL`),
   not deleted.
5. **Tenant-context restoration, with an explicit failure policy — not "just retry."**
   `deliver()` must call `tenant.WithContext(ctx, tenant.TenantContext{TenantID: e.TenantID})`
   before invoking any `Subscriber`/`WorkflowExecutor.Start`, closing the gap `tasks.md` 1.4
   already identified (the relay currently passes its own loop context through unchanged).
   What happens when that restoration itself fails must be classified, per ADR-025 §16 row
   15, into deterministically different outcomes rather than one generic "retry" branch.
   **This classification reuses Phase 1's already-built-and-tested error-classification
   machinery — it is not a new framework.** `internal/dberr/dberr.go`'s `dberr.Parse`
   (fixed under `tasks.md` 1.1a to correctly match real `pgx/v5` errors) already translates
   `set_tenant_context()`'s `RAISE EXCEPTION` codes into distinguishable Go values —
   `dberr.CodeTenantNotFound` (`P0001`) and `dberr.CodeTenantNotActive` (`P0002`) — and
   `platform/iam/service.go`'s `Login` path already calls `dberr.Parse` on exactly this
   error for exactly this reason (`tasks.md` 1.1). The relay's own tenant-context
   restoration should call `dberr.Parse` on the error `set_tenant_context` (or whatever
   restores tenant context on the relay's connection) returns, and branch on the resulting
   code — reusing this mechanism, not re-deriving SQLSTATE handling from scratch.
   - **Transient infrastructure failure** (a brief connection error, pool exhaustion, or
     any `dberr`-classified error other than `CodeTenantNotFound`/`CodeTenantNotActive`) —
     indistinguishable from any other transient dispatch failure. **Retry with the same
     exponential backoff as item 4.**
   - **The tenant exists but `set_tenant_context()` rejects it as not `ACTIVE`** (Phase 1's
     hardened lifecycle check raises `P0002`, translated by `dberr.Parse` to
     `dberr.CodeTenantNotActive` — the tenant was suspended or archived sometime between the
     mutation committing and this delivery attempt, a real possibility given the retry
     window can span the full backoff schedule). This condition will not spontaneously
     resolve the way a transient error does. **Dead-letter immediately** (set `dead_at`,
     skip the remaining backoff schedule), recording the rejection reason in `last_error` so
     an operator can see why and decide whether to manually re-queue if the tenant is later
     reactivated.
   - **The tenant does not exist at all** (`set_tenant_context()` raises `P0001`,
     translated by `dberr.Parse` to `dberr.CodeTenantNotFound` — should be rare, since the
     mutation that created the event necessarily had a valid tenant at write time, but a
     tenant record could theoretically be hard-deleted afterward). **Dead-letter
     immediately**, same reasoning as the non-ACTIVE case.
   - **A malformed/corrupt `tenant_id` value on the row itself** — **the one branch Phase
     1's existing `dberr` machinery cannot classify, and the one small addition this
     document requires beyond reusing it.** The `events_outbox.tenant_id` column is
     `uuid NOT NULL`, so a structurally invalid (non-UUID) value cannot be stored or scanned
     in the first place — a genuinely malformed value here would surface as a Go-level scan
     error before `set_tenant_context` is ever called, not as a `P0001`/`P0002` SQLSTATE
     from it, and `dberr.Parse` operates only on `*pgconn.PgError` values, not on scan
     errors. The relay must therefore add one small, explicit Go-level check the existing
     `dberr` mechanism does not provide: treat `e.TenantID == uuid.Nil`, or any scan/parse
     error encountered while reading the row's `tenant_id`, as this branch directly —
     **without ever calling `set_tenant_context()` at all for such a row**, since doing so
     would let a structurally-invalid value reach the database layer instead of being
     rejected at the Go layer where it is cheaper and safer to catch. (Should this be
     structurally impossible in practice, since the column is `uuid NOT NULL` and always
     populated from the mutation's own validated `tenant.TenantContext` — but if ever
     observed, it indicates data corruption or an attempted boundary bypass, not an
     operational hiccup.) **Dead-letter immediately, and log at a level that pages an
     operator**, distinct from an ordinary dead-letter.
   - **In every branch, the one invariant that must never be violated:** a dispatch attempt
     must never proceed with a fallback, default, or "best guess" tenant context when the
     real one cannot be established. Every branch above ends in either "retry with the
     correct context re-derived fresh on the next attempt" or "dead-letter without ever
     dispatching" — never "dispatch anyway with degraded context."
6. **Per-entity event ordering is NOT guaranteed in Phase 2, and this document does not
   design ordering infrastructure to provide it.** `poll()`'s `ORDER BY occurred_at ASC`
   combined with `SKIP LOCKED` orders each individual batch's *claiming* by timestamp, but
   does not prevent two events for the same entity from landing in different poll batches,
   nor does it serialize concurrent relay instances against each other beyond each one's
   own internal advisory lock. Nothing in the current entity model requires strict
   per-record event ordering, so this is accepted as a stated limitation. A future
   `Subscriber` that genuinely requires ordered delivery must implement its own sequencing;
   the relay provides no FIFO semantics, globally or per-entity.
7. **At-least-once, duplicate dispatch explicitly possible and tolerated, never
   exactly-once — stated as a hard requirement of this design, not an incidental property.**
   The precise crash window that produces a duplicate: `deliver()` succeeds (the external
   call completes) but the relay process crashes before the subsequent
   `UPDATE ... SET delivered_at = NOW()` commits; on restart, the row is still
   `delivered_at IS NULL` and will be redelivered. For workflow-start dispatch, this is
   safe without any new mechanism because Temporal's `WorkflowID`-based start
   deduplication absorbs the duplicate call (§13). For generic domain events, no
   framework-level deduplication is provided — every `Subscriber` implementation must be
   idempotent on its own terms (§14), matching `HOOK_CONTRACT.md`'s existing normative
   rule for `after_save` hooks, extended here to `Subscriber`s by the same reasoning.

---

## 12. Temporal Integration Boundary

**Phase 2 should build the durable transactional event/outbox foundation and wire dispatch
through the existing `workflow.WorkflowExecutor` interface — it should NOT build a Temporal
worker process or real workflow/activity functions.** The dependency runs one direction
only: durable dispatch (getting a `StartWorkflow` call to reliably happen, durably, across
a crash, at-least-once) is a prerequisite for workflow *execution* to be worth building at
all — there is no value in making workflow starts durable if nothing has ever been
designed to run one for real (`cmd/awo/scaffold.go`'s codegen template is the only evidence
of intended workflow shape; zero real workflow functions exist). Confirmed via source:
`workflow.NewWorker` has zero callers in either binary, and neither `cmd/server/main.go`
nor `cmd/awo/serve_impl.go` ever start a worker — so even a perfectly durable outbox would
dispatch into a void today. That is explicitly **out of scope for Phase 2** and belongs in
a later phase once a real business module needs a real workflow.

What Phase 2 *does* need from the Temporal boundary: `EntityService.startWorkflows` *and*
`runtime.ActionContext.StartWorkflow` (both, per §2.7/§2.8 — fixing only one leaves the
other as a live regression) must produce a `WorkflowTriggerFired` outbox event rather than
calling Temporal directly; the relay's corresponding `Subscriber` must call
`workflow.WorkflowExecutor.Start`, not the raw `temporalclient.Client`. **`tasks.md` 2.4
("Unify workflow dispatch paths") is now stale relative to this finding** — its own text
assumes "currently only the Action path uses the interface [correctly]," but this document's
investigation found `ActionContext.StartWorkflow` bypasses outbox durability identically to
`EntityService.startWorkflows`, just while nominally going through the `WorkflowExecutor`
type. Implementation must reconcile task 2.4 with this actual behavior — its WHY/IMPLEMENTATION
text needs correction to reflect that *both* call sites need fixing and that "unify" means
"both route through the outbox-mediated dispatch path," not merely "both call the same
interface type directly." **`tasks.md` itself is not modified by this document** — this is
recorded here as a required reconciliation for whoever next touches that task.
`router.RegisterOptions.Temporal` should be retyped from `temporalclient.Client` to
`workflow.WorkflowExecutor` to close the bypass at the wiring layer too, in the same change
(§21, Step 6).

---

## 13. Idempotency Model

Awo currently has **no canonical idempotency abstraction** — confirmed no `idempotency`
package or middleware exists (the `docs/02-pipeline/PIPELINE_SEQUENCE.md` middleware
diagram's step 7, "Idempotency — Redis GET idempotency_cache:{key}", is documented but not
found anywhere in `api/middleware/`). This is governed entirely by the separate, Frozen
**ADR-009** (`X-Idempotency-Key` header + Redis 24h cache) — untouched, unreferenced, and
unweakened by anything in this document.

Within this document's own scope, exactly two points need idempotency consideration, both
narrower than "build a general idempotency framework":

1. **The outbox row insert itself, for bulk import specifically (§7)** — if a batch
   flush's transaction commits the outbox rows but the caller's process crashes before
   recording "flush N succeeded," a naive retry-the-whole-import could double-insert both
   the entities and their outbox events. Bulk import does not need its own idempotency
   mechanism if it is retried at the *file/job* granularity rather than the *row*
   granularity — re-running the whole import a second time is a user/operator decision
   already, and a full "resumable import" feature is explicitly out of Phase 2's scope
   (§19, §20), per ADR-025 §14.
2. **Relay delivery** — a `Subscriber.HandleEvent`/`WorkflowExecutor.Start` call can be
   retried after a crash between "delivered" and "marked delivered" (§11 item 7). This is
   already handled for the workflow-start case by Temporal's own `WorkflowID`-based
   deduplication (`workflow/id.go`'s `BuildID` convention,
   `{tenantID}.{entityType}.{recordID}.{event}`) — a second `Start` call with the same
   `WorkflowID` against an already-running or completed execution does not create a
   duplicate execution. For generic domain-event `Subscriber`s, idempotency is the
   subscriber implementation's own responsibility (§11 item 7, §14) — this document does
   not build a generic dedup cache.

**No deterministic, generic "idempotency key" field is added to the outbox schema.** An
earlier draft of this document proposed one (derived from
`{entity_name}:{record_id}:{type}:{occurred_at}`); ADR-025 §12 explicitly rejects this —
`events_outbox.id`'s own primary key, Temporal's `WorkflowID` dedup, and the row's
`attempts` counter are the only identities the mechanism requires. That proposal is
withdrawn, not merely superseded, by this revision.

Full-blown API-level idempotency (`Idempotency-Key` header, cached response replay) remains
governed by ADR-009 and is not touched, extended, or duplicated by this document.

---

## 14. Hook Semantics (formalizing what already exists, per ADR-025 §15)

No new hook types proposed. §2.4 already found the real contract sound: nine hook types,
all invoked, all panic-safe, correct TX-boundary placement, recursive-mutation-safe by
construction. **No `AfterCommit` hook type is introduced** — ADR-025 §15 settles this
explicitly: the two needs that might suggest one (publish an event, start a workflow) are
both served by calling the outbox `Publisher` from inside the existing
`AfterCreate`/`AfterSave` stage (still pre-commit, same transaction, per §6), not from a
hypothetical post-commit stage. A genuine post-commit hook would need its own delivery
guarantee against the exact crash it's supposed to survive — precisely the problem the
outbox already solves; introducing a separate `AfterCommit` mechanism would create a
second, competing "guaranteed after commit" channel alongside the outbox.

The one addition Phase 2's pipeline change requires: **document explicitly** (in
`docs/02-pipeline/HOOK_CONTRACT.md`, correcting its existing six-of-nine coverage gap as
part of this work, not as a separate task) that the canonical outbox write happens *after*
`AfterSave` but still *inside* the same transaction — hook authors need to know an
`AfterCreate`/`AfterSave` hook that itself wants to publish an event should call the same
`events.Publisher.Publish(ctx, ...)` the framework's own outbox-write step uses (once
fixed, §11), not invent their own event-publishing side channel, so a hook-published event
gets the same atomicity guarantee a framework-generated lifecycle event does.

---

## 15. Tenant/Context Propagation

No change to the tenant-context model itself (Phase 1 already hardened this exhaustively).
The Phase-2-relevant extension, detailed fully in §11 item 5: the outbox relay must
reconstruct `tenant.TenantContext` from the outbox row's own `tenant_id` column before
invoking any `Subscriber` or `WorkflowExecutor.Start`, with an explicit, tested policy
distinguishing transient-infrastructure failure (retry) from a permanently invalid tenant
(immediate dead-letter) from a malformed tenant identity (immediate dead-letter plus
operator paging). This is the only background-context propagation Phase 2 needs to solve;
the scheduler (`scheduler/scheduler.go`) and the mail/notification workers remain out of
scope (§20 — zero production callers of either today, confirmed fresh).

---

## 16. Organisation-RLS Blocker — Where It Belongs

**Not fixed here, as instructed.** The problem restated precisely: `tenant_isolation` and
the org-scope policy are both PERMISSIVE `CREATE POLICY` statements; PostgreSQL ORs
multiple permissive policies together, so `tenant_isolation` alone (true for every row in
the tenant) makes the org-scope policy a complete no-op for SELECT, INSERT, and UPDATE
alike. Architectural options, evaluated on constraints rather than aesthetics:

- **Declare the org-scope policy `AS RESTRICTIVE`.** RESTRICTIVE policies AND with the set
  of applicable permissive policies rather than OR-ing. This is the smallest possible
  change to `generator.go`'s existing two-`CREATE POLICY`-statement emission — one keyword
  added to one statement. Constraint: RESTRICTIVE policies apply to *every* command the
  policy covers (default `FOR ALL`), so this needs verification that no legitimate
  operation on an org-scoped table would ever need `tenant_isolation` to pass *without*
  `org_isolation` also passing (true today, since nothing currently declares this scope —
  but the policy semantics need re-verifying against real usage once `OrganizationService`
  is implemented in Phase 3, not just against today's synthetic test table).
- **Combine both conditions into one policy expression** (`USING (tenant_id =
  current_tenant_id() AND org_id = current_org_id())`) — functionally equivalent to the
  RESTRICTIVE option for this specific pair, but collapses two conceptually distinct
  concerns (tenant isolation, org isolation) into one policy statement, making future
  ADR-level changes to either concern independently harder to reason about. Slightly worse
  for maintainability than the RESTRICTIVE option for no compensating benefit.
- **Make `org_id` a generator-managed standard column** (excluded from `FieldsByName` the
  way `tenant_id` already is) — this fixes a *different* gap (the write-path
  `checkWritableField` exposure `tasks.md` 1.18 also names) but does **not**, by itself, fix
  the RLS OR-combination bug — it is a necessary companion fix, not a substitute for one of
  the two options above.
- **Application-scope + RLS** (filter every org-scoped query at the application layer in
  addition to RLS) — explicitly the anti-pattern `RLS_SPEC.md` §1 already prohibits
  ("Application-layer tenant filtering is permanently prohibited... A bug in application-
  layer filtering cannot expose cross-tenant data" — the entire point of RLS-as-boundary is
  that application code cannot be trusted to remember the filter). Rejected on the
  framework's own stated design philosophy, not on aesthetics.

**Recommendation for the roadmap** (not implemented here): declare the org-scope policy
`AS RESTRICTIVE` **and** make `org_id` a generator-managed standard column excluded from
`FieldsByName`, as one combined fix — both are needed regardless of which is done first,
and doing them together avoids a window where one is fixed and the other isn't. This
belongs in its own dedicated item (`tasks.md` 1.18, already tracked) with its own PostgreSQL-
verified adversarial test pass, mirroring the rigor of the Phase 1 security-closure work —
explicitly **not** part of Phase 2 as scoped in this document, and explicitly blocking
Phase 3 (organisation hierarchy), as `tasks.md` already states.

---

## 17. Dependency Direction — Violations Relevant to Phase 2

Both previously-flagged violations are confirmed still present (§2.1): `compiler/schema.go`
imports `auth` for `CapabilityGrant`; `audit/queryer.go` imports `pgxpool` directly.
**Neither blocks Phase 2 as scoped in this document** — Phase 2's canonical-pipeline/outbox
work touches `runtime`, `events`, `events/outbox`, `contrib/pgx`, `ioport`, and `workflow`;
it does not touch `compiler` or `audit/queryer.go`. Documented here per the instruction to
identify blockers, with the explicit finding that they are not one. They remain tracked as
`tasks.md` 2.1, appropriately placed in whatever phase actually addresses dependency-graph
cleanup (§20). Separately, this document's own §10 schema addition (`system_actor`,
`correlation_id`) is confirmed, by design, not to introduce an `events → audit` dependency
— both fields are plain `string`s on `events.DomainEvent`, populated at the call site from
values the caller already has, never by importing `audit` into `events` itself.

---

## 18. Core vs. Adapter Boundaries

| Subsystem | Boundary | Reasoning |
|---|---|---|
| `events.DomainEvent`, `events.Publisher`/`Subscriber`/`Bus` interfaces | **CORE** | pure domain semantics, already correctly `def`-only per package doc; no infrastructure type leaks into the interface; the new `system_actor`/`correlation_id` fields preserve this (§17) |
| `events/outbox` (the PostgreSQL-backed `Relay`/`OutboxWriter`) | **ADAPTER** | already correctly isolated behind the `events.Publisher`/`Bus` interfaces — this is exactly right and should stay a `contrib`-shaped adapter even though it currently lives under `events/outbox` rather than `contrib/pgx` (a naming/location inconsistency worth fixing opportunistically, not a boundary violation) |
| `workflow.WorkflowExecutor` interface | **CORE** | already correctly infrastructure-agnostic (`NoopExecutor` proves it) |
| `workflow.TemporalExecutor` | **ADAPTER** | correctly named/scoped already |
| `runtime.ActionContext`/`ActionContextConfig` (the canonical `ActionRuntime` DI seam, §2.8) | **CORE interface, real adapters not yet fully wired** | the interface is the right shape and already uses the real `events.Publisher`/`workflow.WorkflowExecutor` types directly (no invented wrapper interfaces, unlike the sibling `RuntimeFactory`); wiring real repository/notification/cache adapters at `bootstrap` time is what "finish wiring `ActionContext`" (§21 Step 4) means concretely |
| The canonical mutation pipeline's transaction boundary (`WithTx`) | **stays an ADAPTER concern** (`contrib/pgx`) | correctly already — `runtime.Pipeline` never opens a transaction itself; it is invoked *by* the driver from within one. No change proposed. |

No new abstraction is recommended purely for replaceability's sake — the existing `Publisher`/
`Subscriber`/`WorkflowExecutor` interfaces already provide exactly the seams Phase 2 needs;
the work is fixing/wiring their implementations, not adding more interfaces.

---

## 19. Phase 2 Scope

| Candidate item | Classification | Why |
|---|---|---|
| Canonical mutation pipeline (extend `EntityService`'s existing sequence with the outbox-write step; wire `runtime.ActionContext`) | **PHASE 2** | foundation everything else in this list depends on |
| Fix `OutboxWriter.Publish` to join the caller's transaction via `tx.QuerierFromContext` | **PHASE 2** | smallest, highest-leverage fix; prerequisite for atomic outbox writes at all |
| `events_outbox` migration, reconciling the orphaned schema (§10) | **PHASE 2** | the table the existing relay code already assumes exists; the orphaned file's disposition must be explicit |
| Payload sanitization via `audit.Sanitizer.Strip` before every outbox write | **PHASE 2** | mandatory security requirement now that the outbox write is universal, not opt-in |
| Fix `ActionContext.Publish`'s JSON serialization | **PHASE 2** | same file being wired for the first time; a live data-corruption bug in the canonical path |
| Restore tenant context in `Relay.deliver`, with explicit transient/permanent/malformed classification | **PHASE 2** | same code path as the migration/write fix; `tasks.md` 1.4's finding, refined with the failure-classification ADR-025 requires |
| Migrate bulk import onto the canonical pipeline (§7) | **PHASE 2** | this *is* P0-A; the canonical pipeline is a prerequisite, not a parallel track |
| Add `WorkflowTriggerFired` event type + `Subscriber` calling `workflow.WorkflowExecutor.Start` | **PHASE 2** | this *is* P0-B, once the outbox foundation exists |
| Repair `ActionContext.StartWorkflow` (outbox indirection, not direct Temporal call) alongside `EntityService.startWorkflows` | **PHASE 2** | both call sites share the same bug; fixing one without the other leaves a live regression |
| Retype `EntityService`/`router.RegisterOptions` off the raw Temporal SDK client onto `workflow.WorkflowExecutor` | **PHASE 2** | same call site the outbox fix touches; doing it separately later would mean touching this code twice |
| Delete `runtime.RuntimeFactory`/`defaultActionRuntime` once `ActionContext` is verified | **PHASE 2** | leaving two unwired implementations of the same interface indefinitely is a maintenance hazard |
| Poison-event/dead-letter handling + exponential backoff in the relay | **PHASE 2** | small, and the relay's own doc comment already claims dead-lettering exists — closing a doc/code gap while the file is open for the transaction fix anyway |
| `HOOK_CONTRACT.md` correction (document all 9 hook types, the 2-arg Update signature, the outbox-write ordering) | **PHASE 2** | documentation-first principle; small, directly describes the pipeline this phase changes |
| Full API-level idempotency (`Idempotency-Key` header + Redis replay cache) | **LATER, governed by ADR-009** | real gap, but self-contained, orthogonal, and already owned by a separate, Frozen ADR |
| Temporal worker process / real workflow functions | **LATER** | explicitly sequenced after durable dispatch exists (§12); no business module needs a real workflow yet |
| Exactly-once event delivery or workflow execution | **NOT A GOAL, EVER, FOR THIS MECHANISM** | ADR-025 §11/§19 prohibits claiming this; at-least-once with tolerated duplicate dispatch is the permanent design, not a Phase 2 limitation to later remove |
| `ScopeOrganization` RLS fix | **BLOCKED-ELSEWHERE** | already its own tracked item (`tasks.md` 1.18), explicitly out of this document's scope, blocks Phase 3 not Phase 2 |
| Dependency-graph violations (`compiler`→`auth`, `audit`→`pgxpool`) | **NOT NEEDED FOR PHASE 2** | real, tracked (`tasks.md` 2.1), doesn't intersect any file this phase touches |
| Duplicate `platform_audit_log` migrations, audit-history endpoint wiring, `module/` wire-or-remove decision | **NOT NEEDED FOR PHASE 2** | real, tracked, orthogonal — see §20 for the renumbering recommendation |
| OpenAPI deduplication | **LATER** | real gap (§2.9), but doesn't block or get blocked by the mutation-pipeline/outbox work |
| Search | **LATER** | large, self-contained, explicitly lower priority than closing the two P0s per the original audit's own sequencing |
| Bulk-import resumability | **LATER** | ADR-025 §14/§19 explicitly excludes this |
| Generic cross-`Subscriber` deduplication cache | **NOT NEEDED** | no current `Subscriber` implementation exists to justify one; each owns its own idempotence (§13) |
| Per-entity event ordering guarantees | **NOT A GOAL, EVER, FOR THIS MECHANISM** | §11 item 6; no consumer requires it today, and no ordering infrastructure is designed |

This scope is deliberately bounded to "make the canonical pipeline real, and close both
original P0s using it" — nothing else. It is small enough to implement and test with the
same rigor as the Phase 1 security-closure pass (real PostgreSQL, adversarial tests, red/
green verification), and large enough that once it lands, bulk import, actions, and any
future background mutation source share one lifecycle implementation instead of three
diverging ones.

---

## 20. Explicit Out-of-Scope Items (Phase 2 non-goals — protected, not aspirational)

- **Exactly-once event delivery, exactly-once workflow dispatch, exactly-once workflow
  execution** — never claimed, in this document or in any implementation-adjacent
  documentation/code comment, per ADR-025 §11/§19. This mechanism is at-least-once,
  permanently, by design.
- **Temporal worker implementation / real workflow functions** — §12.
- **Full API idempotency** — governed entirely and separately by ADR-009.
- **`ScopeOrganization` RLS repair** — §16, blocks Phase 3, not this phase.
- **Search** — does not exist; not built here.
- **OpenAPI deduplication** — real gap, orthogonal, later phase.
- **Migration-engine completeness** (schema-diff/rename safety, directory consolidation).
- **Scheduler completeness** (`Scheduler.Cancel()`'s existing bug — zero production
  callers, no urgency).
- **Notification-delivery completeness** (`platform/notification` vs `platform/notifications`
  duplication).
- **Bulk-import resumability** — §7, §13, §19.
- **A generic cross-`Subscriber` deduplication cache** — §11 item 7, §13.
- **Per-entity event ordering** — §11 item 6.
- Dependency-graph violations (`compiler`/`audit`), CLI control-plane work, SDUI naming-
  collision cleanup — real, tracked, orthogonal, not needed for this phase.

---

## 21. Exact Implementation Order

**STEP 0 — ADR/governance formalization. Already completed.** ADR-025 is written and
registered in `docs/00-overview/DECISION_REGISTER.md`. Its status remains **Proposed**, not
Active or Frozen — this registration made the decision discoverable and its relationship to
ADR-007/008/009/013/014 explicit; it did not, by itself, authorize implementation as a
binding decision. No implementation step below has begun.

### Step 1 — Transaction-aware `OutboxWriter`
- **GOAL:** `events.Publisher.Publish` actually honors its own documented contract, and
  the payload it persists is the caller's original JSON, not a re-encoded copy of it.
- **WHY:** Prerequisite for every other outbox-atomicity claim in this document; currently
  false regardless of whether the table exists. Additionally, per §10's finding (B),
  `OutboxWriter.Publish` currently double-encodes its payload — this has no effect today
  only because the method has zero callers, and Steps 3/4/6 of this same plan are what give
  it its first ones, so the defect must close before those callers exist, not after.
- **FILES:** `events/outbox/relay.go` (`OutboxWriter.Publish`, `NewWriter`).
- **DEPENDENCIES:** none.
- **IMPLEMENTATION:** Replace `w.pool.Acquire(ctx)` with `tx.QuerierFromContext(ctx)`
  (`tx/tx.go`); return an error (per `events.Publisher`'s documented contract) if no
  transaction is present in `ctx`; call `q.ExecSQL(ctx, insertSQL, args...)` for the
  INSERT, matching the pattern `audit/pg_writer.go:94` and
  `audit/transactional_writer.go:234` already use. `OutboxWriter`'s `pool` field is no
  longer required by `Publish` itself (only `Relay.poll` still needs pool access).
  **In the same change:** stop calling `json.Marshal(e.Payload)` on the already-`[]byte`,
  already-JSON-encoded payload (§10 finding B) — preserve `e.Payload`'s existing bytes as
  the value written into the `payload` column, and handle a nil/empty payload per the
  field's existing `omitempty` contract rather than introducing a new error case for it.
  This document does not prescribe the exact code shape of this fix beyond "do not
  re-encode bytes that are already JSON" — any implementation satisfying that, and the
  existing `events.Publisher` interface, is acceptable.
- **TESTS:** real PostgreSQL — insert an outbox row inside a transaction that then rolls
  back; assert the row does not exist afterward (proves it joined the transaction, not a
  side connection). Adversarial: call `Publish` with no transaction in `ctx` at all — must
  return an error, not silently succeed on a bare connection. **Payload-preservation test
  (§10 finding B), using the transaction-bound path, not the encoding function in
  isolation:** publish a `DomainEvent` whose `Payload` is pre-encoded JSON for an object
  with several fields; read the resulting row back; assert the stored `payload` is
  structurally equivalent to the original object (fields readable via a JSON-path
  expression such as `payload->>'field'`); explicitly assert the stored value is **not** a
  JSON string containing base64 data.
- **SECURITY:** none new (no tenant/auth surface touched directly). The in-transaction
  insert executes under whatever RLS tenant context `WithTx` already established for the
  mutation via `set_tenant_context` before `fn(txCtx)` ran (`contrib/pgx/repo.go:685-689`)
  — it does not establish, and must not attempt to establish, a second, independent tenant
  context; it inherits the ambient one by virtue of running on the same `pgxlib.Tx` (§9).
  Verify the outbox insert doesn't leak tenant context the way Phase 1's connection-pool
  tests already guard against for ordinary mutations.
- **ACCEPTANCE CRITERIA:** [ ] Rollback test passes (PASS/FAIL). [ ] No-transaction-in-context
  test passes (PASS/FAIL). [ ] Payload-preservation test passes: the persisted `payload`
  column is queryable as the original JSON object, and is provably not a base64-encoded
  JSON string (PASS/FAIL). [ ] Full `contrib/pgx`/`events/outbox` suite green (PASS/FAIL).
- **EVIDENCE:** none yet — to be filled in with real test output at implementation time,
  per the discipline established in Phase 1's completion reports.

### Step 2 — Canonical `events_outbox` schema and migration reconciliation
- **GOAL:** The table the relay's `poll()` query already assumes exists, actually exists,
  with the corrected schema (§10), and the orphaned migration is explicitly reconciled,
  not silently duplicated or ignored.
- **WHY:** `migrations/20260706000003_create_platform_support.up.sql` already contains an
  unapplied `events_outbox` definition; a second, differently-shaped migration created
  alongside it would be exactly the "two competing schema definitions" failure this
  document exists to prevent.
- **FILES:** `migrations/20260706000003_create_platform_support.up.sql` (annotate as
  superseded — comment only, not deleted), a new migration file (location per whichever
  convention `tasks.md` 5.2 settles — until then, the `platform/*/migrations` embedded
  `//go:embed` pattern is the closest existing precedent for a framework-native table).
- **DEPENDENCIES:** Step 1 (sequencing them together avoids a half-fixed intermediate
  state, though technically independent).
- **IMPLEMENTATION:** Write the new migration with the complete schema from §10, including
  `type` (not `event_type`), `system_actor`, `correlation_id` (`text`), `next_attempt_at`,
  `dead_at`, and the corrected partial index (`WHERE delivered_at IS NULL AND dead_at IS NULL`,
  not the orphaned file's `attempts < 5` predicate). Wire it into the live migration
  mechanism (a `//go:embed *.sql` + `migration.Register()` package, matching
  `migration/bootstrap/migrations.go`'s pattern). Add a comment to the orphaned file noting
  it is unapplied and superseded by the new migration.
- **TESTS:** apply to a clean database; confirm the relay's existing `poll()` query and the
  fixed `OutboxWriter.Publish` both run without error against the new table; confirm the
  orphaned file's SQL is never separately applied by any test or bootstrap path.
- **SECURITY:** `events_outbox` is a global table with no RLS (consistent with the orphaned
  file's own comment, "global — no RLS," and with `tenant_id` being read, not enforced, at
  the relay layer) — confirm no policy is accidentally added or omitted relative to this
  intended design.
- **ACCEPTANCE CRITERIA:** [ ] Clean-database migration test passes. [ ] `poll()` and the
  fixed `Publish` both execute successfully against the new schema. [ ] Orphaned file
  carries an explicit superseded annotation and is not separately embedded/applied anywhere.
- **EVIDENCE:** none yet.

### Step 3 — Canonical mutation pipeline: in-transaction outbox write, with mandatory sanitization
- **GOAL:** `EntityService.Create`/`Update`/`Delete` write a sanitized `DomainEvent` (via
  the now-fixed `Publisher`) in the same transaction as PERSIST + audit.
- **DEPENDENCIES:** Steps 1-2.
- **FILES:** `api/service/entity.go`, `runtime/pipeline.go` (if the write belongs at the
  Pipeline layer rather than `EntityService` — decide based on whether custom actions
  should get it "for free" too, which argues for `Pipeline`; see §24 open question 1).
- **IMPLEMENTATION:** After `pipeline.RunAfterCreate`/`RunAfterUpdate`/`RunAfterDelete`
  succeeds, inside the same `WithTx` callback, call `publisher.Publish(txCtx, event)` where
  `event.Payload` has already been passed through `audit.Sanitizer.Strip(entityName,
  record.Data)` (and `CustomFields`, if included) before marshaling.
- **TESTS:** real PostgreSQL — (1) create a record, force an `AfterCreate` hook to error,
  confirm the outbox row is rolled back along with the entity row; (2) create a record on
  an entity with a `Sensitive: true` field, confirm the persisted `events_outbox.payload`
  does not contain that field's value.
- **SECURITY:** `tenant_id` on the outbox row must match the mutation's own tenant context
  — adversarial test: attempt to construct an event with a different `tenant_id` than the
  acting context and confirm it's impossible (the event must be built from `ctx`, never
  from caller-suppliable data). Sanitization test above is itself a security acceptance
  criterion, not merely a correctness one.
- **ACCEPTANCE CRITERIA:** [ ] Rollback test passes. [ ] Every Create/Update/Delete produces
  exactly one outbox row on success, zero on rollback. [ ] Sanitization test passes — no
  `Sensitive: true` field value appears in a persisted payload.
- **EVIDENCE:** none yet.

### Step 3b — Relay reliability
- **GOAL:** The relay tolerates slow/hanging external dispatch without blocking unrelated
  events, replaces its flat retry cutoff with scheduled backoff and a genuine dead-letter
  state, and restores tenant context with an explicit, tested failure classification.
- **DEPENDENCIES:** Steps 1-2 (needs the fixed writer and the new schema's `next_attempt_at`/
  `dead_at` columns). Placed before Step 6 (workflow dispatch) deliberately — an
  external-network-call event type must not be added to the relay's dispatch loop before
  the loop itself is proven safe against a slow/hanging call.
- **FILES:** `events/outbox/relay.go` (`deliver`, `poll`).
- **IMPLEMENTATION:** Wrap each `Subscriber.HandleEvent` call in `deliver()` with
  `context.WithTimeout`. Replace the flat `attempts < 5` check with `next_attempt_at`
  scheduling (exponential backoff, capped at 5 minutes) and a `dead_at` terminal
  transition once the schedule is exhausted. Add `tenant.WithContext` restoration before
  each dispatch call, with the four-way classification from §11 item 5 (transient →
  retry; non-ACTIVE/nonexistent tenant → immediate dead-letter; malformed tenant ID →
  immediate dead-letter + operator-paging log level).
- **TESTS:** one slow/hanging fake `Subscriber` alongside one normal `Subscriber` in the
  same poll batch — confirm the normal one is still processed within the expected window.
  Force an event past its backoff schedule — confirm `dead_at` is set and the row is
  excluded from the pending index while remaining queryable. Simulate dispatch against a
  SUSPENDED/ARCHIVED tenant's event — confirm immediate dead-letter with no backoff
  consumed. Simulate dispatch against a nonexistent tenant — same. Simulate a transient
  connection error during tenant-context restoration — confirm ordinary backoff retry, not
  dead-letter (proving the two branches are actually distinguished in code).
- **SECURITY:** a dispatched event's handler must observe only that event's own `tenant_id`
  — never the relay loop's ambient or a previous iteration's leftover context. Test this
  explicitly with two consecutive events for different tenants in one poll batch.
- **ACCEPTANCE CRITERIA:** [ ] Timeout test passes. [ ] Backoff/dead-letter test passes.
  [ ] All four tenant-context-failure classification tests pass, each with distinct,
  correct behavior. [ ] Cross-tenant-context-leak test passes.
- **EVIDENCE:** none yet.

### Step 4 — Canonical `ActionContext`: repair and wire, retire the sibling
- **GOAL:** `runtime.ActionContext` becomes the real, working implementation of
  `def.ActionRuntime` injected into every action invocation; `runtime.RuntimeFactory`/
  `defaultActionRuntime` is removed.
- **DEPENDENCIES:** Steps 1-3 (reuses the same fixed `Publisher` and sanitization
  requirement).
- **FILES:** `runtime/runtime_action_context.go` (`ActionContext.StartWorkflow`,
  `ActionContext.Publish`), `api/handler/crud.go` (`EntityHandler.Action` — construct and
  inject a real `ActionContext`, setting `def.ActionContext.Runtime`), `bootstrap`/`cmd/*`
  (wire real adapters for `ActionContextConfig`'s `TxFn`/`RepoFn`/`Publish`/`Executor`
  fields instead of leaving them entirely unconstructed — see §24 open question 5),
  `runtime/action_runtime_impl.go` (delete `RuntimeFactory`/`defaultActionRuntime` **only
  after** the post-deletion verification gate below passes).
- **IMPLEMENTATION:** Rewrite `ActionContext.StartWorkflow` to publish a
  `WorkflowTriggerFired` event via `a.publish` (the same outbox mechanism `Publish` already
  correctly delegates to) instead of calling `a.executor.Start` directly — this means
  `ActionContext` no longer needs its own injected `executor workflow.WorkflowExecutor`
  field once this lands, only the relay's `WorkflowTriggerFired` subscriber (Step 6) needs
  one. Rewrite `ActionContext.Publish`'s payload serialization from
  `fmt.Sprintf("%v", ...)` to `json.Marshal(event.Payload)` — this is defect (A) from §10's
  payload-serialization discussion (a missing encoding step for an arbitrary Go value being
  serialized for the first time). **Do not describe this as "matching `OutboxWriter.
  Publish`'s pattern"** — that method has its own, different defect (B), fixed separately in
  Step 1, and the two call sites' correct behaviors are not identical (§10 explains why: one
  marshals a raw value for the first time, the other must NOT re-marshal bytes that are
  already JSON). Apply the same `audit.Sanitizer.Strip` requirement and payload-shape rules
  as Step 3 (§10) to any payload built from entity field data.
- **TESTS:** real PostgreSQL — (1) an action handler that calls `runtime.StartWorkflow`
  inside `runtime.Tx(...)`, then returns an error from the `Tx` callback; confirm no outbox
  row (and hence no eventual dispatch) exists afterward. (2) Publish an event through
  `ActionContext.Publish`; read the persisted `payload` column back; `json.Unmarshal` it
  successfully; confirm the unmarshaled values match what was originally published — a
  full round-trip proof, not a unit test of `json.Marshal` alone.
- **SECURITY:** none new beyond Step 3's sanitization/tenant-matching requirements, applied
  identically here.
- **POST-DELETION VERIFICATION GATE (required, not optional, before `RuntimeFactory`/
  `defaultActionRuntime` are considered removed):** deletion happens only after every
  caller, test, example, interface implementation, and generated-code reference to either
  type has been verified, not merely after a grep shows no obvious hits. Immediately after
  deleting `runtime/action_runtime_impl.go`'s `RuntimeFactory`/`defaultActionRuntime`, run,
  in order, and require all four to pass before proceeding: (1) `gofmt -l .` — reports no
  new unformatted files; (2) `go vet ./...` — passes; (3) `go build ./...` — passes,
  confirming no compile-time reference (test file, example, generated code) anywhere in the
  repository named either deleted type; (4) `go test ./...` — passes in full, confirming no
  runtime behavior depended on the deleted implementations. If any of the four fails, the
  reference that caused the failure must be found and reconciled — updated to use
  `ActionContext`, or removed — before the deletion is considered complete; do not work
  around a failure by leaving one of the two implementations partially in place, and do not
  weaken the decision to make `ActionContext` the sole canonical implementation (§2.8, §6)
  in response to a failure — fix the reference instead.
- **ACCEPTANCE CRITERIA:** [ ] The one real `ActionDef` in the codebase
  (`platform/notification`'s `mark_read`) continues to work unmodified. [ ] Deferred-until-
  commit `StartWorkflow` test passes. [ ] JSON round-trip test passes. [ ] `def.ActionContext.Runtime`
  is non-nil for every real action invocation. [ ] The post-deletion verification gate
  above passes in full (`gofmt`, `go vet`, `go build`, `go test ./...` all green). [ ]
  `runtime.RuntimeFactory`/`defaultActionRuntime` no longer exist in the tree; exactly one
  `def.ActionRuntime` implementation remains.
- **EVIDENCE:** none yet.

### Step 5 — Migrate bulk import onto the canonical pipeline (closes P0-A)
- **GOAL:** every imported row runs `BeforeValidate`/validation/`BeforeCreate`/`AfterCreate`/
  audit/outbox, per §7's architecture, batched at the existing flush granularity.
- **DEPENDENCIES:** Steps 1-3 (reuses the same in-transaction audit+sanitized-outbox
  mechanism per flush).
- **FILES:** `ioport/importer.go` (signature change to accept pipeline access),
  `contrib/pgx/repo.go` (`BulkCreate` — likely unchanged if the pipeline calls happen around
  it rather than inside it), package doc comment fix (the false "all invariants enforced"
  claim, §2.6 — correct it to match reality *before* this step lands, then correct it
  again once it's true).
- **IMPLEMENTATION:** Per §7's recommendation (A): validate each flush's rows via
  `pipeline.RunBeforeCreate` outside the transaction, then inside the existing per-flush
  `WithTx` + `BulkCreate` call, run `pipeline.RunAfterCreate`, `pipeline.RunAuditRecord`,
  and the sanitized outbox write for each successfully-inserted row — one outbox event and
  one audit record per row, not one summary event per flush.
- **TESTS:** real PostgreSQL — import a CSV with one row missing a required field; with
  `SkipErrors: false`, confirm the whole import is rejected with no rows persisted; with
  `SkipErrors: true`, confirm only the valid rows persisted, each with its own audit
  record and outbox event, and the bad row appears in `ImportResult.Errors`. Import a CSV
  attempting to set an `Immutable` field on create where the entity has a `Default` —
  confirm framework validation rejects it (currently silently allowed). Confirm a 100-row
  successful import produces exactly 100 outbox rows, not 1.
- **SECURITY:** tenant context still established once per import (not per row) is
  acceptable — verify hooks/audit/outbox records still get the correct `tenant_id` from
  that single context, not from row data.
- **ACCEPTANCE CRITERIA:** [ ] Required/immutable-field violations rejected, not silently
  persisted. [ ] Every imported record has a corresponding audit record and outbox event
  (one each, not aggregated). [ ] Performance cost of per-row hook invocation (still
  batched at the SQL level) is measured and reported, not silently accepted. [ ] No
  resumability/retry-the-whole-job feature is added (§13, §20 — confirm scope was not
  quietly expanded).
- **EVIDENCE:** none yet.

### Step 6 — `WorkflowTriggerFired` dispatch (closes P0-B)
- **GOAL:** `EntityService.startWorkflows` and `ActionContext.StartWorkflow` (both) write a
  `WorkflowTriggerFired` outbox event instead of calling Temporal directly; a new relay
  `Subscriber` reads it and calls `workflow.WorkflowExecutor.Start`.
- **DEPENDENCIES:** Steps 1-4 (needs the fixed, transactional, backoff-capable outbox and
  the repaired `ActionContext` to exist first — this is the actual P0-B fix, and it cannot
  be correct before its own foundation is).
- **FILES:** `events/events.go` (new `EventType`), `api/service/entity.go`
  (`startWorkflows` → outbox write instead of direct `ExecuteWorkflow`), new
  `WorkflowTriggerSubscriber` (likely `workflow/` or `events/outbox/`), `router.go`
  (`RegisterOptions.Temporal` retyped to `workflow.WorkflowExecutor`).
- **IMPLEMENTATION:** Add `EventWorkflowTriggerFired` to `events.EventType`. Rewrite
  `EntityService.startWorkflows` to publish this event (sanitized payload carrying
  `def.ActionWorkflowSpec`) instead of calling `s.temporal.ExecuteWorkflow` directly.
  Write a `Subscriber` that unmarshals the payload and calls
  `workflow.WorkflowExecutor.Start`. Retype `router.RegisterOptions.Temporal` from
  `temporalclient.Client` to `workflow.WorkflowExecutor`.
- **TESTS:** kill-the-process-between-commit-and-dispatch simulation (the exact test
  `tasks.md` 1.4 already specifies) — commit an entity mutation with a `WorkflowTrigger`,
  simulate a crash before the relay ever runs, restart the relay, confirm the workflow
  start is durably retried until it succeeds; confirm a duplicate `Start` call (simulating
  redelivery per §11 item 7) does not create a duplicate execution, via Temporal's
  `WorkflowID`-based dedup — **do not describe this test's outcome as "exactly once" in
  any test name, comment, or report; the correct framing is "duplicate dispatch is safe,"
  not "duplicate dispatch cannot happen."**
- **SECURITY:** none new beyond Step 3b's tenant-context-restoration requirements, exercised
  here for the workflow-dispatch subscriber specifically.
- **ACCEPTANCE CRITERIA:** [ ] `tasks.md` 1.4's existing acceptance criteria, verified
  against real PostgreSQL and a `NoopExecutor`/fake `WorkflowExecutor` (a real Temporal
  server is explicitly not required for Phase 2, per §12). [ ] No code path outside the
  relay's `WorkflowTriggerFired` subscriber calls `WorkflowExecutor.Start`/the raw Temporal
  client directly. [ ] `tasks.md` 2.4's staleness (§12) is documented as a follow-up, even
  though `tasks.md` itself is not edited by this implementation step.
- **EVIDENCE:** none yet.

### Step 7 — Final hardening, cleanup, and verification
- **GOAL:** close remaining small gaps, remove dead code, correct documentation, and run
  the complete verification suite this project's own established discipline requires.
- **DEPENDENCIES:** Steps 1-6.
- **FILES:** `docs/02-pipeline/HOOK_CONTRACT.md` (§14 correction), any remaining dead code
  from Step 4's `RuntimeFactory` removal, `tasks.md` 1.3/1.4 (flip to `[x]` only once their
  own stated acceptance criteria are met by real tests — performed by whoever executes
  this step, not pre-decided here).
- **IMPLEMENTATION:** Documentation corrections; final sweep for any remaining
  `tx.ConnFromContext`/`event_type`/`fmt.Sprintf("%v"` usages introduced by mistake during
  implementation; confirm no "exactly-once" phrasing was introduced in code comments or
  tests during Steps 1-6.
- **TESTS:** the complete adversarial suite from §22, run together, not just per-step.
- **SECURITY:** re-run the full Phase 1 regression suite (tenant/RLS/session/connection-
  pool/dynamic-SQL tests) — this phase must not reopen anything Phase 1 closed.
- **ACCEPTANCE CRITERIA:** every item in §23 below, verified PASS, not assumed.
- **EVIDENCE:** none yet — this step is where the final, complete evidence trail (test
  output, `git diff --stat`, `gofmt`/`vet`/`build` results) gets assembled, per the exact
  discipline Phase 1's own completion reports used.

---

## 22. Test Strategy — the complete failure matrix, mapped to tests

**Real PostgreSQL, required for:** every row below that touches a transaction, RLS, or the
outbox table. **Redis:** unaffected by this phase (no new Redis dependency introduced).
**Temporal:** integration tests only exercise `workflow.WorkflowExecutor.Start` being
called correctly (a fake/mock executor, using `workflow/testing.go`'s existing, currently-
unused `WorkflowTestEnv` harness — this is the moment to finally use it, closing `tasks.md`
9.2 as a side effect) — not that a real Temporal server executed a real workflow, since no
real workflow function will exist yet (§12).

| # | Scenario | Test location | Expected outcome |
|---|---|---|---|
| 1 | Validation failure | Step 5 (bulk import) | No transaction opened; row rejected or skipped per `SkipErrors`; no audit, no outbox row |
| 2 | Mutation (PERSIST) failure | Step 3 | Transaction rolls back; no audit, no outbox row |
| 3 | Audit failure (Admin/Security category) | Step 3 (regression) | Transaction rolls back (ADR-017, unchanged) |
| 4 | Audit failure (other category) | Step 3 (regression) | Mutation commits without audit record; outbox write unaffected by audit's leniency (§8) |
| 5 | Outbox insert failure | Step 3 | Transaction rolls back unconditionally, regardless of audit category |
| 6 | DB transaction rolls back (general) | Step 1, Step 3 | Nothing persisted; Phase 1's panic-safety `WithTx` fix governs connection release, unaffected |
| 7 | Relay crash before dispatch | Step 3b | Row remains pending; next poll (any instance) picks it up, no data lost |
| 8 | Relay crash after dispatch, before ack | Step 3b, Step 6 | Row redelivered on next poll — the at-least-once window; safe for workflow starts (Temporal dedup), requires `Subscriber` idempotence for domain events |
| 9 | External dispatch timeout | Step 3b | Per-event timeout fires; `attempts` increments; `next_attempt_at` scheduled per backoff |
| 10 | External dispatch returns an error | Step 3b | Same as timeout — recorded, backed off, retried |
| 11 | Retry exhaustion | Step 3b | `dead_at` set; excluded from pending index; operator-queryable |
| 12 | Duplicate dispatch | Step 6 | Explicitly possible and tolerated; Temporal `WorkflowID` dedup absorbs it for workflow starts; `Subscriber`-owned idempotence for domain events — never framed as "cannot happen" |
| 13 | Temporal unavailable | Step 3b, Step 6 | Treated as an ordinary dispatch error (row 10) — retried with backoff, not specially distinguished |
| 14 | Tenant context cannot be reconstructed | Step 3b | Classified: transient → retry; non-ACTIVE/nonexistent tenant → immediate dead-letter; malformed tenant ID → immediate dead-letter + operator-paging log |
| 15 | Malformed/poison event | Step 3b (via retry-exhaustion path) | Retried per rows 9/10 until exhausted, then dead-lettered per row 11 — no separate fail-fast path added in Phase 2 |

Additional adversarial tests, matching the rigor established in the Phase 1
security-closure pass:
- `AfterCreate` hook failure — confirm the outbox row rolls back with the entity row, not
  just the entity row.
- Panic inside an `AfterCreate` hook that has already called `Publish` — confirm
  `safeCall`'s existing recovery plus `WithTx`'s existing panic-safety (Phase 1) together
  still roll back the outbox row correctly — this exercises three separate Phase 1/Phase 2
  safety mechanisms at once and should be tested explicitly.
- Sensitive-field sanitization (Step 3, Step 4) and JSON round-trip (Step 4) — detailed in
  their own steps above, restated here as required, not optional, test coverage.
- Cross-tenant-context-leak test (Step 3b) — a dispatched event's handler must see only its
  own event's tenant, never the relay loop's ambient or a prior iteration's context.

---

## 23. Acceptance Criteria (for Phase 2 as a whole)

Every mandatory ADR-025 invariant, restated as an objective PASS/FAIL check:

- [ ] `OutboxWriter.Publish` uses `tx.QuerierFromContext` and `ExecSQL` — not
      `tx.ConnFromContext`, not `pool.Acquire` — verified by reading the implementation.
- [ ] `events_outbox` exists with the schema in §10, using `type` (not `event_type`),
      `correlation_id text` (not `uuid`), `system_actor text`, `next_attempt_at`, `dead_at`.
- [ ] The orphaned `migrations/20260706000003_create_platform_support.up.sql` schema is
      explicitly annotated as superseded; no second, competing `events_outbox` definition
      exists unreconciled.
- [ ] Every `EntityService.Create`/`Update`/`Delete` writes exactly one sanitized outbox
      row on success, zero on any rollback, including a hook-triggered one.
- [ ] A `Sensitive: true` field's value never appears in a persisted `events_outbox.payload`
      — proven by a real PostgreSQL test, not asserted from reading the sanitizer's code.
- [ ] `ActionContext.Publish` serializes with `json.Marshal`; a round-trip test proves the
      persisted payload is valid, parseable JSON.
- [ ] `runtime.ActionContext.StartWorkflow` produces a `WorkflowTriggerFired` outbox event;
      it does not call `workflow.WorkflowExecutor.Start` (or any Temporal client) directly
      from any transaction-bound code path.
- [ ] `def.ActionContext.Runtime` is non-nil for every real action invocation.
- [ ] `runtime.RuntimeFactory`/`defaultActionRuntime` no longer exist in the tree — exactly
      one `def.ActionRuntime` implementation remains.
- [ ] Bulk import runs `BeforeValidate`/validation/hooks/audit/outbox per row, batched at
      the SQL level per existing `BatchSize` semantics; one outbox event per row, not one
      per flush; required/immutable-field violations rejected, not silently persisted
      (closes P0-A). No resumability feature was added.
- [ ] Workflow-trigger starts survive a simulated crash between commit and dispatch; a
      simulated duplicate dispatch does not produce a duplicate Temporal execution (closes
      P0-B). No test, comment, or report describes this as "exactly once."
- [ ] `router.RegisterOptions.Temporal` is `workflow.WorkflowExecutor`, not a raw SDK client.
- [ ] `deliver()`'s dispatch loop enforces a per-event timeout; a hung `Subscriber`/executor
      call does not block delivery of other pending events in the same batch.
- [ ] A dead-lettered event (`dead_at` set) is operator-queryable, not silently invisible.
- [ ] The tenant-context-reconstruction-failure classification (transient/non-ACTIVE/
      nonexistent/malformed) is implemented and each branch is tested independently.
- [ ] A cross-tenant-context-leak adversarial test passes.
- [ ] No document, test name, or code comment produced during implementation contains the
      phrase "exactly once" or "exactly-once" in reference to this mechanism.
- [ ] A test or code comment records that per-entity event ordering is explicitly not
      guaranteed (§11 item 6) — not silently assumed.
- [ ] Full Phase 1 regression suite (tenant/RLS/session/connection-pool/dynamic-SQL tests)
      remains green throughout — this phase must not reopen anything Phase 1 closed.
- [ ] `gofmt`/`go vet`/`go build`/`go test ./...` clean at every step, exactly as this
      session's own verification discipline required throughout Phase 1.
- [ ] `docs/02-pipeline/HOOK_CONTRACT.md` corrected to list all nine real hook types.
- [ ] `tasks.md` 1.3 and 1.4 are flipped to `[x]` only once their own already-stated
      acceptance criteria are met by real tests — not by this document's existence.
      `tasks.md` 2.4's staleness (§12) is flagged for correction, though not corrected by
      this document itself.

---

## 24. Risks and Unresolved Design Questions

1. **Where does the outbox write belong — `EntityService` or `runtime.Pipeline`?** Placing
   it in `Pipeline` gives every future caller of `RunAfterCreate`/`RunAfterUpdate` (bulk
   import once migrated, actions once wired) the outbox write "for free," consistent with
   this document's single-source-of-truth principle — but `Pipeline` currently has no
   concept of `events.Publisher` at all (`runtime`'s own import list, §2.1, doesn't include
   `events.Publisher` as a `Pipeline` field). Recommend resolving this at implementation
   time by adding `Pipeline.eventPublisher events.Publisher` (mirroring how `auditWriter` is
   already wired), not deciding it definitively in this planning document.
2. **`SkipErrors` + per-row hook validation interaction** (§7) — needs a concrete decision
   during implementation, not left ambiguous: does a `BeforeValidate` failure on one row
   count toward the same `ImportResult.Errors` bucket as a DB-constraint failure, or does it
   need its own category? Recommend the same bucket, for simplicity, unless implementation
   reveals a concrete reason to split them.
3. **Performance cost of per-row hook invocation in bulk import** — genuinely unmeasured.
   `tasks.md` 1.3 already flags this as an open question ("Performance regression (if any)
   measured and accepted or mitigated"); Phase 2 must actually measure it (a real benchmark,
   not a guess) before deciding whether chunked/batched hook invocation (running all of a
   flush's `BeforeValidate` calls, then one batch insert, then all `AfterCreate` calls) is
   necessary versus strictly one-row-at-a-time hook invocation.
4. **ADR-025 is Proposed, not yet Frozen.** This plan is executable against a Proposed
   contract; if ADR-025's status changes (further revision, or promotion to Active/Frozen)
   before or during implementation, this document must be re-checked against whatever
   changed, the same way this revision re-checked it against the original plan.
5. **Does `ActionContextConfig`'s dependency-injection shape (`TxFn`/`RepoFn`/`Publish`/
   `Executor`/`Cache`/`NotifyFn`/`InvalidateFn` as separate injected values) match what
   `bootstrap.Run` currently constructs, or does wiring it for the first time require
   restructuring `bootstrap.Run`'s own return type? Left deliberately unresolved — this
   document does not invent a bootstrap/dependency-injection design; Step 4's own
   implementation must resolve it against the real code, not against a design proposed
   here.** What is confirmed, not resolved: `bootstrap.Run` currently returns
   `Result{Pool, Redis, Schema}` only (`bootstrap/bootstrap.go:72-76`) — it constructs no
   `events.Publisher`, no `workflow.WorkflowExecutor`, and no `ActionContext` of any kind.
   **What must eventually be wired**, stated as requirements Step 4 must satisfy, not as a
   prescribed mechanism: (a) a real `ActionContext` constructed per action invocation, with
   real (not stub) `Publish`/`Executor`/`TxFn`/`RepoFn` adapters, injected into
   `EntityHandler.Action` in place of today's `Runtime`-less construction; (b) the outbox
   `Relay` actually started as a background goroutine against the fixed `OutboxWriter` and a
   registered `WorkflowTriggerFired` subscriber (Step 6) — **confirmed this pass to already
   be in an inconsistent state across the two existing binaries**: `cmd/server/main.go:193-
   197` already constructs `outbox.New(result.Pool)` and starts it in a goroutine
   unconditionally, while `cmd/awo/serve_impl.go:196-200` has the identical wiring
   commented out with an explicit `// FIX: outbox relay disabled until events_outbox
   migration is applied` marker — meaning `cmd/server`'s relay goroutine, as committed
   today, would poll a table (`events_outbox`) that does not yet exist as a live migration,
   and would error on every poll cycle until Step 2 lands. **Entry points needing
   inspection before Step 4/6 land, not redesign:** `bootstrap/bootstrap.go`,
   `cmd/server/main.go` (lines around 193-197), `cmd/awo/serve_impl.go` (lines around 32,
   196-200). **Acceptance test that proves the canonical runtime and relay are actually
   active** in a running binary, not merely unit-tested in isolation: an end-to-end test
   that starts the real server wiring (or as close to it as practical, matching the pattern
   `tasks.md` 2.2 already established for a similar "wired in tests, not wired in
   production" class of bug) and proves a real action's `Publish`/`StartWorkflow` call
   produces a persisted `events_outbox` row that the relay subsequently picks up and
   delivers — not just that `ActionContext`'s unit tests pass in isolation. **Before
   deleting or bypassing `RuntimeFactory`/`defaultActionRuntime`, or before assuming the
   relay is live in production, implementation must verify the actual binary wiring in both
   `cmd/server` and `cmd/awo`** — the discrepancy found this pass between the two binaries'
   relay-startup state means "the relay runs in one binary" is not the same claim as "the
   relay runs everywhere this framework is deployed," and Step 4/6's acceptance criteria
   must be checked against both entry points, not just whichever one is easiest to test.
   This is not a generic dependency-injection redesign — it is a requirement to trace and
   fix what already exists, in the shape it already exists in.

---

## 25. Recommended Next Step

Review this document against ADR-025 one final time if either changes further. If the
scope in §19 and the ordering in §21 are approved, the next session should be a normal
implementation session (not another planning pass) starting at Step 1 — it is the
smallest, most isolated, most immediately testable change in the sequence, and every later
step depends on it being correct first. Do not start at Step 3 or later "to move faster" —
each step's test strategy assumes the previous ones are already green, exactly as Phase 1's
own red/green discipline required throughout this project. ADR-025 remains Proposed
throughout; nothing in this plan should be read as treating it as Frozen.
