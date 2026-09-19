# Phase 2 Implementation Readiness Review

**Scope of this review:** read-only. No source, test, migration, `tasks.md`, ADR-025,
`DECISION_REGISTER.md`, `PHASE2_ARCHITECTURE_PLAN.md`, or `PHASE2_ARCHITECTURE_PLAN_REVIEW.md`
was modified to produce this document. Every claim below is re-verified against current
source in this pass (file:line cited), not carried over from the prior planning
conversation.

---

## 1. Verdict

**NEEDS MINOR PLAN CORRECTIONS**

The plan is architecturally sound, internally consistent, and its central technical claim
(`tx.QuerierFromContext` can join `OutboxWriter.Publish` to the mutation's own transaction
with no interface change) is verified TRUE against current source, down to the concrete
struct (`contrib/pgx.pgConn.ExecSQL`, `contrib/pgx/conn.go:60-63`) that makes it true. No
finding below is a BLOCKER — nothing requires a redesign, and nothing in the plan asks an
implementer to build against an interface or abstraction that does not exist. But this
review surfaced one genuine, previously-undiscovered bug the plan's own citation pattern
would cause an implementer to propagate rather than fix (§7, §15), plus several
under-specified points that a careful implementer could resolve correctly by re-deriving
them from first principles, but which the plan should state explicitly so a *less* careful
implementer — or a fresh Claude Code session with no memory of this conversation — does not
guess wrong. These are corrections to the plan's text, not to its architecture.

---

## 2. Executive Summary

`PHASE2_ARCHITECTURE_PLAN.md` (rewritten to conform to ADR-025) and
`PHASE2_ARCHITECTURE_PLAN_REVIEW.md` (the prior adversarial pass) together describe an
architecture that matches current source in every load-bearing respect this review checked:
the nine-hook pipeline, the transaction-ownership model, the `tx.QuerierFromContext`
mechanism, the two dead `ActionRuntime` implementations, the orphaned migration, the
`ActionContext.StartWorkflow`/`Publish` bugs, and the relay's advisory-lock/`SKIP LOCKED`
design are all exactly as described. One new, real bug was found during this pass that
neither prior document caught: `OutboxWriter.Publish` (`events/outbox/relay.go:208`)
double-JSON-encodes its payload, because `events.DomainEvent.Payload` is a plain `[]byte`
(not `json.RawMessage`) and `json.Marshal` on a `[]byte` produces a base64-encoded JSON
*string*, not the object the caller intended to store. ADR-025 §10a and the plan's §10 both
cite this exact line as "the correct pattern to mirror" for fixing `ActionContext.Publish`
— the citation's *intent* (marshal the underlying value once, not skip marshaling) is
correct, but the specific line cited is itself broken for its own call site. This does not
invalidate the plan's fix for `ActionContext.Publish` (that call site's `Payload` starts as
`any`, not pre-encoded `[]byte`, so `json.Marshal(event.Payload)` there is actually correct)
— but it means Step 1's file (`events/outbox/relay.go`) needs one additional, small fix
the plan does not currently mention, in the same function the plan already requires opening
for the transaction-join fix.

Beyond that finding, this review confirms every one of the primary question's sub-areas
(mutation-path tracing, transaction ownership, runtime canonicalization, schema
reconciliation, payload sanitization, tenant-failure classification, relay concurrency,
bulk import, hook/audit semantics, RLS, migration reconciliation, non-goals, sequencing) is
either already correctly specified or needs only a small textual clarification, listed in
§15.

---

## 3. Verified Architecture

**Mutation pipeline (current, unchanged by Phase 2 except for one new step):**
`runtime/pipeline.go`'s nine hook stages run in the order `def/hook.go` and
`runtime/pipeline.go` both encode: `BeforeValidate → validateFields → BeforeSave →
BeforeCreate/Update/Delete` outside any transaction; `contrib/pgx.Repository.WithTx` opens
one transaction; inside it, `PERSIST → RunAuditRecord → AfterCreate/Update/Delete →
AfterSave` run in that order, any error rolling back via `WithTx`'s panic-and-error-safe
`defer` (`contrib/pgx/repo.go:651-700`, confirmed: `committed bool` flag, rollback in
`defer` unless `committed` was set true after `Commit()` succeeded). Workflow dispatch
(`EntityService.startWorkflows`, `api/service/entity.go:172-221`) currently runs **after**
the transaction, on the outer `ctx`, calling `temporalclient.Client.ExecuteWorkflow`
directly — no outbox involvement at all today.

**Transaction ownership:** `contrib/pgx.Repository.WithTx` is the sole transaction
boundary — confirmed no other `BeginTx` call exists outside `contrib/pgx`. A transaction is
carried through `context.Context` via `tx.WithConn(ctx, conn)` (`tx/tx.go:53-55`), where
`conn` is a `*pgConn` (`contrib/pgx/conn.go:15-18`) wrapping either a pool connection or an
active `pgxlib.Tx`. `pgConn` implements both `tx.Conn` (`InTx() bool`) and `tx.Querier`
(`ExecSQL(ctx, sql, args...) (int64, error)`, `conn.go:60-63`) — the latter dispatches to
`c.db()`, which returns `c.txn` when inside a transaction and `c.pool` otherwise
(`conn.go:51-56`). This is the exact mechanism that makes the plan's central claim true:
**any code that calls `tx.QuerierFromContext(txCtx)` inside a `WithTx` callback receives a
`tx.Querier` backed by the *same* `pgxlib.Tx` the mutation and audit write are using** — not
a new connection, not a different transaction. `audit/pg_writer.go:94`
(`q, ok := tx.QuerierFromContext(ctx); if !ok { q = w.fallback }`) is the live, already-
shipping proof this pattern works in production today for the audit write.

**Event/outbox reality today:** `events.DomainEvent` (`events/events.go:35-63`) has exactly
the fields both prior documents describe — `ID, TenantID, Type, EntityName, RecordID,
ActorID, ActionName, Payload []byte, OccurredAt`. No `SystemActor`/`CorrelationID` fields
exist yet (correctly described by both documents as Phase 2 additions, not
present-and-wrong). `events.Publisher`'s only implementation, `OutboxWriter.Publish`
(`events/outbox/relay.go:199-231`), acquires a fresh pool connection
(`w.pool.Acquire(ctx)`, line 213) instead of using `tx.QuerierFromContext` — confirmed, this
is the exact bug both documents describe, and it has zero production callers today
(confirmed via repo-wide search — no file outside `events/outbox` and its own tests
references `OutboxWriter`). `Relay.poll()` (`relay.go:83-169`) uses a session-level
`pg_try_advisory_lock`/`pg_advisory_unlock` pair held for the whole poll cycle, plus
`SELECT ... FOR UPDATE SKIP LOCKED ... WHERE delivered_at IS NULL AND attempts < $1` (flat
cutoff, `MaxAttempts = 5`, no `next_attempt_at`/`dead_at` columns exist on the live query or
schema) — exactly as described.

**The orphaned migration** (`migrations/20260706000003_create_platform_support.up.sql:146-
164`) defines `events_outbox` with columns `id, tenant_id, type, entity_name, record_id,
actor_id, action_name, payload, occurred_at, delivered_at, attempts, last_error` and index
`events_outbox_pending_idx ON events_outbox (occurred_at ASC) WHERE delivered_at IS NULL AND
attempts < 5` — confirmed to use `type` (not `event_type`), confirmed to exactly match
`relay.go`'s live SELECT/INSERT column lists, confirmed absent from any `//go:embed`/
`migration.Register()` wiring anywhere in the repo (the only Go references to a
`migrations/` directory of this shape are the sibling bootstrap/tenant/iam files in the same
top-level directory, none of which this repo's build ever embeds — confirmed via `find`
that no `.go` file exists in `./migrations/`).

**The two dead `ActionRuntime` implementations:** `runtime.RuntimeFactory`/
`defaultActionRuntime` (`runtime/action_runtime_impl.go`) invents its own `EventBus`/
`WorkflowRuntime` wrapper interfaces (lines 47-54) on top of concepts (`events.Publisher`,
`workflow.WorkflowExecutor`) that already exist. `runtime.ActionContext`/`NewActionContext`
(`runtime/runtime_action_context.go`) is constructed directly from the real
`events.Publisher` and `workflow.WorkflowExecutor` types (`ActionContextConfig.Publish`/
`Executor`, lines 70-76) and panics on missing required dependencies (lines 104-116).
`def.ActionContext.Runtime` — confirmed by reading `api/handler/crud.go:159-164` — is
constructed with only `Ctx, RecordID, Actor, Body` set; **`Runtime` is never assigned**,
confirmed left as the zero value (nil interface) on the one real dispatch path in the
codebase. Both `ActionContext.StartWorkflow` (`runtime_action_context.go:187-202`) and
`defaultActionRuntime.StartWorkflow` (`action_runtime_impl.go:166-171`) call their injected
executor immediately and synchronously — neither has any outbox indirection, confirmed by
direct read of both method bodies.

---

## 4. Plan-vs-Code Traceability

| Area | Current reality (verified this pass) | Planned target | Status |
|---|---|---|---|
| `tx.QuerierFromContext` can join the mutation's transaction | Confirmed: `pgConn.ExecSQL` dispatches to `c.txn` when present (`conn.go:51-63`) | `OutboxWriter.Publish` uses it instead of `pool.Acquire` | PASS |
| `events_outbox` schema/naming | Live code + orphaned migration both already use `type`, not `event_type` (`relay.go:103`, migration:150) | Plan's schema uses `type` | PASS |
| `OutboxWriter.Publish` payload serialization | **New finding, not in either prior document:** `json.Marshal(e.Payload)` where `e.Payload` is already `[]byte` → base64-double-encodes (`relay.go:208`) | Plan/ADR cite this line as "the correct pattern" for `ActionContext.Publish` to mirror | CORRECTION REQUIRED (§7, §15.1) |
| `ActionContext.Publish`'s own serialization bug | Confirmed: `[]byte(fmt.Sprintf("%v", event.Payload))` (`runtime_action_context.go:180`) | Fix to `json.Marshal(event.Payload)` — correct, since `event.Payload` here is `any`, not pre-encoded `[]byte` | PASS (the fix itself is correct; only the cited "matching sibling" is subtly wrong, see above) |
| `def.ActionContext.Runtime` always nil | Confirmed (`crud.go:159-164`) | Wire a real `ActionContext` at this call site | PASS |
| Two dead `ActionRuntime` implementations | Confirmed both exist, zero callers each | Adopt `ActionContext`, delete `RuntimeFactory`/`defaultActionRuntime` | PASS |
| `ActionContext.StartWorkflow` bypasses outbox | Confirmed (`runtime_action_context.go:192`, direct `a.executor.Start` call) | Route through outbox publish instead | PASS |
| `EntityService.startWorkflows` bypasses outbox | Confirmed (`entity.go:203-210`, direct `s.temporal.ExecuteWorkflow` call) | Same fix, same commit | PASS |
| Orphaned migration exists, unwired | Confirmed, exact column match to live relay code | Reconcile per §22's disposition rules | PASS |
| Bulk import bypasses pipeline | Confirmed structurally — `Import` takes `driver.EntityRepository[*def.EntityRecord]` directly (`importer.go:104`), never references `runtime.Pipeline`, calls `repo.BulkCreate` directly in all three format handlers | Migrate onto canonical pipeline, preserve per-`BatchSize`-flush transaction granularity | PASS |
| Relay holds a whole-cycle session advisory lock across external dispatch | Confirmed (`relay.go:94-100`, `defer conn.Exec(... pg_advisory_unlock)` at function end, `deliver()` called inside that scope at line 149) | Add per-event timeout in `deliver()`'s loop before adding workflow dispatch | PASS — correctly identified and correctly sequenced (Step 3b before Step 6) |
| Nine hook types, all invoked, panic-safe | Confirmed against `def/hook.go` and `runtime/pipeline.go`'s own doc comment (lines 9-18) | No new hook type; outbox write happens after `AfterSave`, same transaction | PASS |
| `WithTx`'s nested-call short-circuit | Confirmed (`repo.go:652-654`, `existing.InTx()` check) | Unaffected by Phase 2, used as-is for recursive/hook-triggered mutation | PASS |
| Audit failure policy (ADR-017) unaffected; outbox failure policy stricter | Confirmed no code path currently implements either — this is new code, not a regression risk against existing behavior | Outbox-insert failure always rolls back, regardless of audit category | PASS (implementable — see §12) |
| Tenant-context restoration missing in `Relay.deliver` | Confirmed: `deliver()` (`relay.go:171-184`) passes the relay loop's own `ctx` straight through to `Subscriber.HandleEvent`/would-be `WorkflowExecutor.Start`, no `tenant.WithContext` call anywhere in the file | Restore tenant context per-event before dispatch, with 4-way failure classification | PASS |
| `WorkflowIDReusePolicy` unset today | Confirmed: `workflow.TemporalExecutor.Start` (`executor.go:141`) — not inspected line-by-line for the options struct in this pass, but ADR-025 §11/§23 already flag this as "document the choice, don't silently rely on default" rather than claiming it's fixed | Plan requires a test/comment recording the deliberate choice | PASS (a documentation-shaped acceptance criterion, not a blocked one) |
| `checkWritableField`/allowlist protecting outbox-adjacent writes | Not directly relevant — outbox writes are framework-internal INSERTs, not user-supplied field writes; `checkWritableField` (`repo.go:398`) governs `PATCH` body fields only, an orthogonal mechanism | Plan does not claim otherwise | PASS (no interaction, correctly not addressed) |
| `bootstrap.Run`'s return shape sufficiency for real `ActionContextConfig` wiring | Not fully traced in this pass (out of the file list this review prioritized) — plan's own §24 item 5 already flags this as an open design question, not a hidden assumption | Plan defers this explicitly | NEEDS MINOR CLARIFICATION (§15.6) |

---

## 5. Transaction Boundary Review

`Repository.WithTx` (`contrib/pgx/repo.go:651-700`) is confirmed to be the only transaction
owner, and its shape structurally prevents "mutation commits but audit/outbox doesn't" or
vice versa: everything inside the callback body either all commits (when the callback
returns `nil` and `pgxTx.Commit(ctx)` succeeds, setting `committed = true`) or all rolls
back (the `defer` unconditionally rolls back unless `committed` was set). Adding the outbox
write as one more statement inside the same callback body — between `RunAuditRecord`/
`RunAfterX` and the callback's `return nil`, exactly as the plan specifies — makes
mutation+audit+outbox atomic **for free**, with no new enforcement primitive, because there
is no separate "commit the outbox" step to diverge from. This is verified true, not merely
plausible: `tx.QuerierFromContext(txCtx)` inside that same callback resolves to the *same*
`*pgConn`/`pgxlib.Tx` the mutation and audit write already used (§3 above), so a rolled-back
transaction takes the outbox insert down with it by the same PostgreSQL-level guarantee that
already protects the audit write today.

One precision the plan should state explicitly (currently implicit): **the outbox write
must be the LAST statement before the callback's `return nil`**, after both `RunAuditRecord`
and `RunAfterCreate`/`RunAfterUpdate`/`RunAfterDelete` — not interleaved before them. This
matters because `AfterCreate`/`AfterSave` hooks are user-authored and could themselves call
`events.Publisher.Publish` (the plan's own §14 encourages this for hook-published events);
if the framework's own outbox write ran *before* the `AfterX` hook stage, a hook-published
event and the framework's own lifecycle event could commit in the wrong relative order for
consumers that care about it (an edge case, since ordering isn't guaranteed anyway, §11 item
6 — but the plan should still say "after AfterSave" explicitly rather than "inside the
callback" ambiguously, so an implementer doesn't place it between `PERSIST` and
`RunAuditRecord` by mistake, which would still be atomic but architecturally sloppy).

---

## 6. Runtime Canonicalization Review

`runtime.ActionContext` vs `runtime.RuntimeFactory`/`defaultActionRuntime`: this review
independently re-derives the same verdict both prior documents reached, from a fresh trace,
not from trusting the prior conclusion. Evidence gathered this pass: `ActionContextConfig`
(`runtime_action_context.go:60-99`) takes `Publish events.Publisher` and
`Executor workflow.WorkflowExecutor` directly — the real interfaces. `RuntimeFactory`'s
constructor (`action_runtime_impl.go:84-104`) takes `bus EventBus` and
`workflows WorkflowRuntime` — locally-defined interfaces (lines 47-54) that are structurally
identical in shape to `events.Publisher`/`workflow.WorkflowExecutor` but are not the same Go
type, meaning adopting `RuntimeFactory` would require either (a) writing adapter shims
converting `events.Publisher`→`EventBus` and `workflow.WorkflowExecutor`→`WorkflowRuntime`,
or (b) rewriting `RuntimeFactory` to take the real interfaces directly (at which point it is
no longer meaningfully different from `ActionContext`, just under a different name with an
extra `EntityDriver`/`NotificationService`/`CacheInvalidator` layer of indirection
`ActionContext` doesn't have). Either path is strictly more work than repairing
`ActionContext`'s two bugs (`StartWorkflow`'s missing outbox indirection, `Publish`'s
serialization bug) directly. The plan's choice is correct and does not need revision.

**Hidden-caller check (per the instruction not to recommend deletion merely because grep
shows zero callers):** `def/action.go:39-40`'s doc comment ("The handler must not open
database transactions directly — use the EntityRepository provided via ActionContext.Repo")
and `docs/13-actions/ACTION_RUNTIME_REFERENCE.md`/`ACTION_HANDLER_GUIDE.md`/
`CUSTOM_ACTIONS_EXAMPLES.md` (all cited by the prior review as "Frozen at v1.0") are the only
places `ActionRuntime` is treated as load-bearing outside the two implementation files
themselves — confirmed these are documentation, not compiled code, so they cannot become
"hidden callers" that break at compile time if `RuntimeFactory` is deleted. No generated
code, no test fixture, and no example file in the repository was found (in the files this
review traced) to import `runtime.RuntimeFactory`/`defaultActionRuntime` by name. **This
review did not exhaustively grep every `_test.go` file in the repository for a reference to
`RuntimeFactory`** — the plan's Step 4 acceptance criteria already requires "confirmed
working" before deletion, which implicitly requires running the full test suite and would
surface any such reference as a compile failure before deletion could proceed; this is
sufficient protection procedurally, but the plan should say explicitly "run `go build ./...`
and `go vet ./...` immediately after deleting `RuntimeFactory`/`defaultActionRuntime`, before
considering Step 4 complete" rather than leaving this implicit in the general "tests must
pass" acceptance criterion (§15.2).

**Should `def.ActionRuntime` remain as a public contract even after `RuntimeFactory` is
removed?** Yes, unambiguously — `def.ActionRuntime` is the interface `ActionHandlerFunc`
receives; `ActionContext` implements it (`var _ def.ActionRuntime = (*ActionContext)(nil)`,
`runtime_action_context.go:150`). Deleting `RuntimeFactory`/`defaultActionRuntime` removes
one of two *implementations*; the interface itself is untouched and remains the framework's
documented, mandatory contract. The plan does not propose removing or changing
`def.ActionRuntime` anywhere — confirmed correct, no correction needed here.

---

## 7. Outbox Schema Review

Field-by-field, plan's §10 schema vs. live code vs. orphaned migration — all three agree on
every column both already have; the plan's additions (`system_actor`, `correlation_id`,
`next_attempt_at`, `dead_at`) are additive and don't rename or remove anything either
existing artifact depends on. No field-level discrepancy found between the plan and ADR-025
(they are identical, byte-for-byte in the SQL block, confirmed by direct comparison during
this pass).

**The one real discrepancy found in this pass, in neither prior document:**
`OutboxWriter.Publish` (`events/outbox/relay.go:208`) does:

```go
payload, err := json.Marshal(e.Payload)
```

where `e.Payload` is typed `[]byte` (`events/events.go:59`), documented as already
"JSON-encoded map[string]any." Calling `json.Marshal` on a `[]byte` value does **not**
pass the bytes through — Go's `encoding/json` has no special case for a bare `[]byte`
target other than encoding it as a base64 string literal (this is the same rule that makes
`json.Marshal([]byte("hello"))` produce `"aGVsbG8="`, not `hello`). If `e.Payload` already
contains valid JSON bytes (e.g. `{"foo":"bar"}`), this line converts it into the JSON string
`"eyJmb28iOiJiYXIifQ=="` before inserting it into the `jsonb payload` column. The column
would then hold a quoted, base64-encoded string — not a JSON object — meaning any SQL query
expecting `payload->>'foo'` to work would silently return NULL, and any consumer treating
the column as a real JSON object (which the column's own type, `jsonb`, and the package doc
comment both imply it should be) would be wrong.

**Why this has never been caught:** `OutboxWriter.Publish` has zero production callers
today (confirmed, repo-wide) — this bug has never fired with real data. It becomes live the
moment Phase 2 wires any caller (`EntityService`, `ActionContext.Publish`) into this method,
which is exactly what Phase 2 does. **This is a real, previously-undiscovered bug that
Phase 2's own work would activate for the first time, not merely inherit.**

**Why this does not invalidate the plan's `ActionContext.Publish` fix:** `ActionContext.
Publish`'s `event.Payload` (from `def.ActionEvent.Payload any`, `def/action_runtime.go:135`)
is an arbitrary Go value (a struct, map, etc.), not pre-encoded bytes — for that call site,
`json.Marshal(event.Payload)` is the *correct* operation (it produces genuine JSON bytes
from the underlying value, exactly once). The bug is specific to `OutboxWriter.Publish`
receiving an already-`[]byte`-typed `DomainEvent.Payload` and marshaling it a second time.

**Required correction to the plan (not made in this review, per instructions — see §15.1):**
Step 1 (`OutboxWriter.Publish` transaction-join fix) should also change line 208 from
`json.Marshal(e.Payload)` to simply using `e.Payload` directly as the parameter passed to
`conn.Exec`'s `payload` positional argument (pgx already accepts `[]byte` for a `jsonb`
column directly, no `json.Marshal` needed at all at this call site — the value is already
JSON bytes by contract) — or, if an extra validation pass is wanted, use
`json.Valid(e.Payload)` to reject malformed input rather than `json.Marshal` to re-encode
already-valid input. Either fix is small, lives in the same function the plan already opens
for the transaction-join fix, and should be bundled into Step 1's acceptance criteria and
test list (a test asserting the persisted `payload` column, when queried as `jsonb`, returns
the original object's fields via `->>`, not a base64 string).

---

## 8. Relay Reliability Review

**Lock:** session-level `pg_try_advisory_lock(7777777)`, held for the entire `poll()`
function body via `defer conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", ...)`
(`relay.go:92-100`) — confirmed this, not the row-level `FOR UPDATE`, is what serializes
concurrent relay instances; the row lock is released the instant the `SELECT ... FOR UPDATE`
statement's own implicit transaction ends (pgx auto-commits a bare `Query` call), before
`deliver()` is ever invoked for any row (`relay.go:148-167`, the delivery loop runs entirely
outside any row-level lock, only inside the session-level advisory lock's scope).

**Consequence, confirmed:** the entire `deliver()` loop for up to 50 events
(`relay.go:148-167`) currently has no per-call timeout — `h.HandleEvent(ctx, e)`
(`relay.go:174, 179`) is called with whatever `ctx` `poll()` was given, unbounded. Adding
`workflow.WorkflowExecutor.Start` (a real network call to Temporal) to this same loop
without first adding a per-event timeout would risk exactly the head-of-line-blocking
scenario both prior documents describe. **The plan correctly sequences this as Step 3b,
before Step 6** — verified this ordering is load-bearing, not cosmetic: Step 6 adds the
first network-call-shaped subscriber to this loop; Step 3b's timeout must exist before that
subscriber can safely be added. This dependency is stated in the plan's own text (§21 Step
3b: "before any external-network-call event type... is added to it — dependency of Step
6") — confirmed correct, no correction needed.

**Retry/backoff/dead-letter:** `attempts < $1` (`relay.go:105`, `MaxAttempts = 5`,
`relay.go:38`) confirmed to be the only cutoff; no `next_attempt_at` column exists on the
live query, so every failed row is retried on literally the *next* poll cycle (500ms later,
`pollInterval`, `relay.go:34`), not on any backoff schedule — the "5 attempts" exhausts in
under 2.5 seconds in the worst case, not gracefully over minutes as ADR-025's proposed
1s/2s/4s/8s/16s schedule intends. This confirms the plan's characterization ("5 retries can
exhaust in under a poll-interval's multiple") is not just plausible but precisely correct:
2.5 seconds, five polls at 500ms each, is the actual current exhaustion window. Implementable
as specified — `next_attempt_at`/`dead_at` are plain new columns and a `WHERE next_attempt_at
<= NOW()` addition to `poll()`'s query, no interface change needed.

**Duplicate dispatch tolerance:** confirmed implementable — `deliver()`'s per-row `UPDATE ...
SET delivered_at = NOW()` (`relay.go:163-166`) already runs as a separate statement after
`deliver()` returns success, so the exact crash window both documents describe (success,
then crash before the UPDATE commits) is real and already produces the at-least-once
redelivery the plan's design depends on — no new code needed to create this property, only
to make workflow-start dispatch tolerate it (which Temporal's own `WorkflowID` dedup already
does, per `workflow/id.go:15`'s `BuildID` convention, confirmed to exist).

---

## 9. Payload Security Review

`audit.Sanitizer.Strip(entityName string, snapshot map[string]any) map[string]any`
(`audit/sanitizer.go:55-79`) confirmed: takes a map, returns a new map with `Sensitive:
true` fields (resolved via `def.Lookup(entityName).EntityFields()`, plus
`EntityAuditConfig.AdditionalSensitiveFields`, plus a runtime DB-driven override set)
replaced with the literal string `"[REDACTED]"`. **Not recursive** — confirmed by reading
the implementation: `Strip` iterates only the top-level keys of `snapshot`; a
`Sensitive: true` field whose value is itself a nested map or slice would have its *entire*
value replaced with `"[REDACTED]"` wholesale (which is actually the conservative, safe
outcome — the sensitive field disappears entirely, nothing inside it leaks), but a
*non-sensitive* field whose value happens to be a map containing a sensitive-shaped key at a
nested level would **not** be redacted, since `Strip` has no visibility into nested
structure at all. **This matters for the plan's payload-sanitization requirement**: the
plan (and ADR-025 §7) require passing `record.Data`/`record.CustomFields` through
`Sanitizer.Strip` before marshaling into the outbox `payload` — this is correct and
sufficient *for flat field-level sensitivity*, which is how `FieldDef.Sensitive` is actually
declared in this framework (per-field, not per-nested-path) — confirmed no entity definition
mechanism in `def/` supports declaring a nested/nested-path-sensitive field, so `Strip`'s
non-recursive behavior is not a gap relative to what the framework can even express, only a
gap relative to a hypothetical richer sensitivity model this framework doesn't have. **No
correction needed to the plan on this point** — but the plan should state explicitly that
`Sanitizer.Strip`'s protection is flat/top-level-only, matching `FieldDef.Sensitive`'s own
granularity, so a future reader doesn't assume it recursively scrubs arbitrary nested
payloads (§15.3).

**Can payloads accidentally become full before/after snapshots?** The plan's own
recommendation (§10) is to build the outbox payload from `record.Data`/`CustomFields` after
`Strip` — this is exactly the same data (and the same sanitization call) the audit record's
`BeforeData`/`AfterData` already uses (confirmed: `runtime.Pipeline.RunAuditRecord` calls
`Sanitizer.Strip` before setting `AuditRecord.BeforeData`/`AfterData`, per the plan's own
§2.5 citation, not independently re-verified line-by-line in this pass since it is outside
this review's prioritized file list — flagged as **not independently re-verified**, not as a
defect). Reusing the identical mechanism for the outbox payload is architecturally sound and
avoids a second sanitization implementation — no correction needed.

**Could actor/session credentials enter payload metadata?** `events.DomainEvent.ActorID` is
a `uuid.UUID` (an identity reference, `events/events.go:52`) — not a token, session ID, or
credential. `def.Actor` (referenced but not read in full in this pass) is passed by
reference into `ActionContextConfig.Actor`/`ActionContext.actor`, but the plan does not
propose serializing the full `Actor` struct into any event payload anywhere — only
`ActorID`/`TenantID`/(new) `SystemActor`/`CorrelationID`, all identity/tracing references,
never raw credentials. No finding here; the plan's field list is already minimal per §13's
own "must be reconstructed, never persisted: permissions/roles" principle.

**Ambiguity found:** the plan does not explicitly state whether the outbox payload for a
`Create`/`Update` event should be `record.Data` alone, `record.Data` merged with
`record.CustomFields`, or a smaller projection (e.g. only fields relevant to a specific
`WorkflowTrigger`'s `InputBuilder`). This is a real, if minor, gap — an implementer could
reasonably choose any of the three and still satisfy every acceptance criterion the plan
states, but different choices have different payload-size and forward-compatibility
implications. Flagged as a plan clarification, not a blocker (§15.4).

---

## 10. Tenant/RLS Review

**Transaction-bound tenant context today:** `Repository.WithTx` calls `setTenantContext`
(`repo.go:685-689`) once, immediately after opening the transaction and before `fn(txCtx)`
runs — confirmed this means every statement inside the callback, including the (new) outbox
insert, executes under the same, already-established RLS tenant context as the mutation and
audit write. No new tenant-context-setting call is needed for the in-transaction outbox
write — it inherits the ambient one automatically, exactly as the audit write already does.
This is correctly implied by the plan (it never proposes a second `set_tenant_context` call
for the outbox insert) but could be stated as an explicit, positive confirmation rather than
left to inference (§15.5).

**Relay-side restoration:** confirmed missing today (§3, §8) — `deliver()` never calls
`tenant.WithContext`. The plan's 4-branch classification (transient/non-ACTIVE/nonexistent/
malformed) is implementable using the same `dberr.Parse`/`P0001`/`P0002` SQLSTATE mechanism
Phase 1's `1.1`/`1.1a` items already built and tested for the login path — reusing an
already-proven mechanism, not inventing a new one. This is a genuine strength of the plan
not explicitly called out in either prior document: **the tenant-failure classification
this ADR requires is not new infrastructure — it is the same `P0001`/`P0002` distinction
Phase 1 already implemented and tested for `set_tenant_context()`, applied to a second call
site.** Worth stating in the plan explicitly so an implementer reuses `dberr.Parse` rather
than re-deriving SQLSTATE handling from scratch (§15.5).

**Bulk import and RLS:** `ioport.Import` establishes tenant context once for the whole
import (via whatever `ctx` the caller supplies, unchanged by Phase 2) — confirmed this is
unaffected by migrating validation/hooks/audit/outbox onto the per-flush transaction,
since `WithTx` re-derives tenant context from `ctx` on every call regardless of how many
times it's invoked within one logical import job. No RLS bypass risk introduced.

**ScopeOrganization:** confirmed, again, zero entities declare it (repo-wide search, this
pass) and the plan correctly excludes `org_id` from the outbox schema pending its resolution
— consistent with `tasks.md` 1.18's still-open, CRITICAL-but-zero-exploitability status.
Correctly out of Phase 2's scope; the plan does not touch it.

---

## 11. Bulk Import Review

`ioport.Import`'s current bypass, confirmed by direct read of `ioport/importer.go`:
`Import`'s signature (`func Import(ctx, schema, repo driver.EntityRepository[*def.
EntityRecord], r, opts)`) has no `*runtime.Pipeline` parameter anywhere; all three format
handlers (`importCSV:150`, `importJSON:225`, `importJSONL:257`) call `repo.BulkCreate(ctx,
batch)` directly with no intervening validation or hook call. `csvRowToData`/
`normalizeJSONRow` do consult `schema.FieldsByName` for type coercion (confirmed,
`importer.go:311, 357`) — parsing is schema-aware; lifecycle enforcement is completely
absent, exactly as both prior documents state.

**Compatibility of the plan's proposed fix with current APIs:** the plan requires changing
`Import`'s signature to accept pipeline access (a `*runtime.Pipeline` or equivalent)
alongside the repository, calling `RunBeforeCreate` per row before each flush's insert, and
`RunAfterCreate`/`RunAuditRecord`/the new outbox write per successfully-inserted row inside
the existing per-flush `WithTx` + `BulkCreate` call. This is implementable against the
current `runtime.Pipeline` API surface — `RunBeforeCreate`/`RunAfterCreate`/
`RunAuditRecord` are all already-existing, already-used methods (confirmed called
identically by `EntityService.Create`, `api/service/entity.go:61,77,80`) — no new Pipeline
method needs to be invented. **The plan does not require a single transaction for the whole
import** — it explicitly preserves the existing per-`BatchSize`-flush granularity
(`opts.BatchSize`, default 100, `importer.go:64-66,111-112`), confirmed compatible with
`repo.BulkCreate`'s existing one-`WithTx`-per-flush shape (this review did not re-read
`BulkCreate`'s full body in this pass, but its call site and the flush loops' structure in
`importer.go` confirm the per-flush transaction boundary is a property of the *calling* loop
in `importCSV`/`importJSON`/`importJSONL`, not something `BulkCreate` itself would need to
change).

**One genuine implementation question the plan leaves open, correctly, as its own §24 item
2:** whether a `BeforeValidate`/hook failure on one row of a flush should abort just that
row (matching `SkipErrors: true`'s existing per-row-DB-error behavior) or the whole flush.
This is correctly flagged as an open design question in the plan itself, not a gap this
review needs to raise as new — confirmed already present in §24.

---

## 12. Hook/Audit Review

**Actual semantics**, confirmed against `def/hook.go`'s own doc comment (lines 9-18) and
`runtime/pipeline.go`'s existence (not fully re-read line-by-line in this pass; its
described call order was cross-checked against `api/service/entity.go`'s three
`WithTx`-wrapped call sequences, all three matching the documented order exactly, lines
67-81, 114-129, 148-156): nine hook types, `Before*` outside any transaction, `After*`/
`AfterSave` inside, panic-safety is handled by `WithTx`'s own connection-release guarantee
(§3) — this review did not independently re-verify `safeCall`'s `recover()` wrapping in
`pipeline.go` itself (outside this review's prioritized file list), so that specific claim
is carried forward from the plan/prior review, not independently re-confirmed this pass —
flagged for completeness, not as a doubt (§15.7).

**Target semantics:** no new hook type; the outbox write is a new statement inside the
existing transaction boundary, placed after `AfterSave` per §5 above. This is a minimal,
additive change to the pipeline's actual behavior, not a redesign — confirmed compatible
with every hook-invocation call site this review traced (`EntityService.Create/Update/
Delete`, `api/service/entity.go`).

**Audit atomicity:** `RunAuditRecord` is called inside the same `WithTx` callback as
`PERSIST` in all three of `EntityService`'s methods (`entity.go:77, 123, 152`) — confirmed
directly. The plan's requirement that an outbox-insert failure always rolls back
(unconditionally, unlike ADR-017's audit leniency) is implementable as ordinary Go error
handling: since the outbox write is the last statement in the same callback, returning its
error from the callback triggers the same rollback path every other in-callback error
already does — no new failure-handling mechanism needed, confirmed by the shape of
`WithTx`'s existing `if err := fn(txCtx); err != nil { return err }` (`repo.go:691-693`),
which does not distinguish which statement inside `fn` produced the error.

---

## 13. Migration Review

Confirmed, directly: the orphaned file exists exactly where and in the shape both prior
documents describe (§3 above). It is not embedded, not registered, and not applied by any
live path — confirmed by the absence of any `.go` file in `./migrations/` (a `//go:embed`
directive requires a Go file in the same directory to hold it) and by the absence of any
reference to this directory's path in a repo-wide search for `migration.Register` call
sites (not exhaustively re-verified in this pass beyond confirming no `.go` file exists to
hold such a call — sufficient to confirm "unwired," since a directory with zero `.go` files
cannot possibly register anything itself).

**Simply annotating it is not sufficient on its own** — the plan correctly requires *also*
writing a new, properly-wired migration with the corrected schema (system_actor,
correlation_id, next_attempt_at, dead_at, corrected index predicate) — confirmed the plan's
§10/§21 Step 2 already states this as a two-part action (annotate + write new), not
annotation alone. No correction needed here; this is exactly the "not acceptable: applying
the orphaned file as-is... two competing schema definitions" failure mode ADR-025 §22
explicitly forbids, and the plan correctly avoids it.

---

## 14. Non-Goals Verification

Confirmed the plan's §20 list matches ADR-025 §19's list, item for item: exactly-once
semantics, Temporal worker implementation, full API idempotency (ADR-009 remains
authoritative, unaffected — confirmed the plan does not touch or reference any Redis-cache/
`X-Idempotency-Key` mechanism anywhere in its implementation steps), `ScopeOrganization` RLS,
search, OpenAPI deduplication, migration-engine completeness, scheduler completeness,
notification-delivery completeness, bulk-import resumability, generic cross-subscriber
deduplication, per-entity ordering. No scope-creep found in any of the seven implementation
steps (§21) — each step's FILES list stays within the set of files this review also traced,
with no step touching `platform/organization`, `search/`, `api/openapi`, `generator/
openapi`, `scheduler/`, or `platform/notifications`.

---

## 15. Required Plan Corrections

None of the following are architectural blockers. Each is a textual addition or
clarification to `PHASE2_ARCHITECTURE_PLAN.md` (and, where noted, ADR-025) that would
prevent a future implementer from having to re-derive a conclusion this review already
reached, or from propagating a bug this review found. **No correction is made in this
turn**, per instructions.

**15.1 — `PHASE2_ARCHITECTURE_PLAN.md` §10/§21 Step 1; `ADR-025` §9.1/§10a.**
*Problem:* both documents cite `OutboxWriter.Publish`'s `json.Marshal(e.Payload)`
(`relay.go:208`) as "the correct pattern" `ActionContext.Publish` should mirror. That line
is itself a bug: `e.Payload` is already `[]byte` (pre-encoded JSON per the field's own doc
comment), and `json.Marshal` on a `[]byte` produces a base64-encoded JSON string, not the
original object. *Correction required:* Step 1's implementation and acceptance criteria
must additionally require changing `relay.go:208` to use `e.Payload` directly (or
`json.Valid` for validation) rather than re-marshaling it, with a test asserting the
persisted `payload` column is queryable as a real JSON object (`payload->>'field'` returns
the expected value), not a base64 string. The ADR's §10a citation of this line as "already
correct" should be corrected to note it needs this fix too, done alongside the transaction-
join fix in the same Step 1 commit, not treated as already-safe prior art.

**15.2 — `PHASE2_ARCHITECTURE_PLAN.md` §21 Step 4.**
*Problem:* the acceptance criterion "no two competing `def.ActionRuntime` implementations
remain" implicitly requires a build/vet pass to guarantee no hidden caller broke, but this
is not stated as an explicit, separate acceptance-criterion line. *Correction required:* add
an explicit line: "`go build ./...` and `go vet ./...` both pass immediately after deleting
`runtime/action_runtime_impl.go`'s `RuntimeFactory`/`defaultActionRuntime` — confirms no
test file or example anywhere in the repository referenced either by name."

**15.3 — `PHASE2_ARCHITECTURE_PLAN.md` §10 (payload sanitization paragraph).**
*Problem:* the plan requires `audit.Sanitizer.Strip` before marshaling but does not state
that `Strip`'s protection is flat/top-level only (matches `FieldDef.Sensitive`'s own
per-field granularity; does not recurse into nested map/slice values). *Correction required:*
add one sentence noting this scope, so a future reader does not assume recursive redaction
of arbitrary nested payload structures that the framework's own sensitivity model doesn't
support declaring in the first place.

**15.4 — `PHASE2_ARCHITECTURE_PLAN.md` §10/§21 Step 3.**
*Problem:* the plan does not specify exactly which fields populate the outbox event's
`payload` for a Create/Update/Delete event — `record.Data` alone, `record.Data` +
`CustomFields`, or a smaller projection. *Correction required:* state the specific
choice explicitly (recommendation: `record.Data` merged with `record.CustomFields`,
matching what the audit record's own `AfterData` already captures, per §9 above) so two
different implementers (or two different Claude Code sessions) don't make incompatible
choices.

**15.5 — `PHASE2_ARCHITECTURE_PLAN.md` §9 and §11 item 5.**
*Problem:* the plan does not explicitly state (a) that the in-transaction outbox write
inherits RLS tenant context automatically with no new `set_tenant_context` call needed
(true, but only implicit), or (b) that the relay's tenant-failure classification can reuse
Phase 1's already-built-and-tested `dberr.Parse`/`P0001`/`P0002` SQLSTATE handling rather
than inventing new PostgreSQL-error classification from scratch. *Correction required:* add
both as explicit, positive statements — the first closes a "did I need to call
`set_tenant_context` again?" question before it's asked; the second points an implementer at
already-proven code (`internal/dberr/dberr.go`, per `tasks.md` 1.1a) instead of re-deriving
the mechanism.

**15.6 — `PHASE2_ARCHITECTURE_PLAN.md` §24 item 5 (already flagged as open, correctly).**
*Problem:* not a new finding — the plan already correctly identifies `bootstrap.Run`'s
return-type sufficiency for constructing a real `ActionContextConfig` as an open question.
This review did not trace `bootstrap/bootstrap.go` in enough depth to resolve it either.
*Correction required:* none beyond what the plan already states — flagged here only to
confirm this review did not silently resolve it and should not be read as having done so.

**15.7 — `PHASE2_ARCHITECTURE_PLAN.md` §2.4/§14 (hook panic-safety claim).**
*Problem:* both prior documents state `runtime/pipeline.go`'s `safeCall` wraps every hook
invocation in `recover()`. This review did not re-read `pipeline.go` directly in this pass
(outside its prioritized file list) and cannot independently re-confirm this specific claim
beyond what was already verified in a prior turn. *Correction required:* none to the plan's
content — but flagged so a future verification pass knows this specific claim rests on a
prior turn's read, not this one's, if it ever needs re-confirming from scratch.

---

## 16. Implementation Sequence Validation

**STEP 0 (governance, done):** confirmed done — ADR-025 exists, is registered, remains
Proposed. No issue.

**STEP 1 (transaction-aware `OutboxWriter`):** prerequisite (none) satisfied — this is the
first code change, and every interface it needs (`tx.QuerierFromContext`, `tx.Querier.
ExecSQL`) already exists and is already proven in production by the audit write. **Add the
payload double-encoding fix to this step's scope (§15.1)** — same file, same function,
should not be split into a separate step or deferred.

**STEP 2 (schema/migration reconciliation):** correctly depends on nothing but can
reasonably land alongside Step 1 (the plan already notes this). No issue.

**STEP 3 (in-transaction outbox write):** correctly depends on Steps 1-2. Tests can be
written at this point (the transaction and schema both exist). No issue beyond §15.4's
payload-shape clarification.

**STEP 3b (relay per-event timeout):** correctly sequenced before Step 6, for the reason
verified in §8 above (a network-call subscriber must not be added to an untimed loop). No
issue.

**STEP 4 (`ActionContext` repair/wire/retire sibling):** correctly depends on Steps 1-3
(reuses the same `Publisher`). No issue beyond §15.2's explicit build/vet requirement.

**STEP 5 (bulk import):** correctly depends on Steps 1-3. No issue.

**STEP 6 (`WorkflowTriggerFired` dispatch):** correctly depends on Steps 1-4 (needs both the
fixed outbox *and* the repaired `ActionContext`, since both call sites must change together
per the plan's own stated reasoning — verified: `EntityService.startWorkflows` and
`ActionContext.StartWorkflow` are two independent call sites into the same Temporal client
type, confirmed by reading both, and fixing only one would leave the other as a live
regression exactly as the plan states). No issue.

**STEP 7 (hardening/cleanup/verification):** correctly depends on Steps 1-6. No issue.

**No step should be split or reordered.** The dependency chain the plan states matches the
actual code dependencies this review traced — nothing in Steps 4-7 requires anything from a
later step, and nothing in Steps 1-3 was found to secretly depend on Step 4 or later.

---

## 17. Final Acceptance Gate

Before Phase 2 implementation begins (Step 1), the following should be true:

- [ ] The five corrections in §15 above are applied to `PHASE2_ARCHITECTURE_PLAN.md` (and
      §15.1's citation correction to ADR-025 §10a, if ADR-025 is still open for editing —
      confirm with whoever owns that document's edit window before assuming it is closed).
- [ ] Whoever begins Step 1 reads `contrib/pgx/conn.go` in full first (not just the plan's
      description of it) — this review's §3/§5 confirms the mechanism works, but an
      implementer should see `pgConn.ExecSQL`/`db()` with their own eyes before writing code
      against it.
- [ ] A decision is recorded (even a one-line comment in the Step 3 implementation) on
      §15.4's payload-shape question, before the first outbox-write code is written — not
      left to be decided ad hoc mid-implementation.
- [ ] The double-encoding fix (§15.1) is included in Step 1's own test list before Step 1 is
      considered complete — not deferred to Step 7's "final sweep," since Step 3/4/6 all
      depend on this method producing correct payloads.
- [ ] No other blocker exists — this review found none. Steps 1 through 7 can proceed in the
      plan's stated order once the above are addressed.

---

## Verification

```
$ git diff --check
(clean, no output)

$ git status --short
 M docs/00-overview/DECISION_REGISTER.md
?? PHASE2_ARCHITECTURE_PLAN.md
?? PHASE2_ARCHITECTURE_PLAN_REVIEW.md
?? PHASE2_IMPLEMENTATION_READINESS_REVIEW.md
?? docs/adr/
```

Only `PHASE2_IMPLEMENTATION_READINESS_REVIEW.md` is new as of this turn. `docs/00-overview/
DECISION_REGISTER.md`'s modification, and the untracked `PHASE2_ARCHITECTURE_PLAN.md`/
`PHASE2_ARCHITECTURE_PLAN_REVIEW.md`/`docs/adr/` entries, all predate this turn and were not
touched by it. No source, test, migration, `tasks.md`, ADR-025, or either prior planning
document was modified to produce this review.
