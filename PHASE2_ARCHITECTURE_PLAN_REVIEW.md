# Phase 2 Architecture Plan — Adversarial Review

**Purpose of this document:** challenge `PHASE2_ARCHITECTURE_PLAN.md`, not defend it. Every
claim below is re-verified against current source (file:line cited) in this review pass —
several things `PHASE2_ARCHITECTURE_PLAN.md` asserted or implied turn out to be incomplete
or wrong once traced further, and this document corrects them rather than restating them.
No source, test, migration, or other doc was modified to produce this review.

---

## 1. Verdict on the Existing Plan

**Directionally correct, materially incomplete.** The plan correctly identified both real
P0s, correctly found the `OutboxWriter.Publish` transaction-join bug, and correctly declined
to invent new abstractions where real ones already exist. It is incomplete in three
specific, consequential ways this review corrects:

1. It evaluated only ONE of the codebase's two competing, unwired canonical-action
   abstractions (`runtime.RuntimeFactory`/`defaultActionRuntime`) and missed a second,
   architecturally superior one (`runtime.ActionContext`, `runtime/runtime_action_context.go`)
   that already wires the *real* `events.Publisher`/`workflow.WorkflowExecutor` interfaces
   instead of inventing redundant wrapper interfaces. **§4 below reverses the plan's
   implicit recommendation.**
2. It asserted "one canonical mutation pipeline" as the target without first proving it was
   achievable — it is not, and forcing it would break at least one structurally necessary
   exception (`platform/audit.Writer`) and misrepresent at least one legitimate
   multi-entity-transaction pattern (`platform/iam`'s login/logout audit-trail writes) as a
   bug to fix rather than a different, equally valid primitive. **§2 below defines exactly
   two mutation primitives instead of one.**
3. It found `runtime.ActionContext`'s (well, `defaultActionRuntime`'s, at the time) doc
   comments overclaim commit-deferred semantics, but did not discover that the *actual*
   dead implementation's `StartWorkflow` method has the **exact same P0-B durability bug**
   the plan was trying to fix — a call straight to the executor, no outbox indirection at
   all. Adopting either dead implementation as-is, unmodified, would silently reintroduce
   P0-B inside the very abstraction meant to help close it. **§4 and §9 correct this.**

---

## 2. What Is Correct

- The two original P0s, restated with fresh evidence (§9 confirms P0-A is worse than
  "missing validation" — it's a signature-level structural impossibility; §8/§9 confirm P0-B
  is three separable gaps, not one).
- `OutboxWriter.Publish` violating `events.Publisher`'s own documented contract
  (`events/events.go:65-69` vs `events/outbox/relay.go:200-231`) — re-verified by direct
  read, exactly as stated.
- The recommendation to fix that one function as the highest-leverage, lowest-risk first
  step — still correct, unchanged by this review.
- The "one outbox, not two" recommendation (§10 of the original plan) — still correct, with
  one refinement (§8 below: shared claiming/storage, but per-event-type dispatch isolation).
- The recommendation not to build a Temporal worker in Phase 2 — still correct, and this
  review found additional evidence reinforcing it (§9).
- The dependency-direction safety analysis for wiring a real action-runtime implementation
  — the original plan asserted no cycle risk without full proof; §16 below supplies the
  actual proof (two independent, reinforcing reasons), and the conclusion holds.

---

## 3. What Is Unsafe or Premature

- Adopting `runtime.RuntimeFactory`/`defaultActionRuntime` specifically (rather than its
  sibling `runtime.ActionContext`) would mean building real adapters for a design that
  invented its own `EventBus`/`WorkflowRuntime` wrapper interfaces on top of the real
  `events.Publisher`/`workflow.WorkflowExecutor` ones — extra indirection with no benefit,
  and a second parallel implementation left permanently unresolved.
- Treating `platform/iam`'s, `platform/metadata`'s, `platform/registry`'s, and
  `platform/settings`'s direct-repository-call patterns as bugs to fix in Phase 2 would be
  scope creep unjustified by either P0 — none of the two original P0s involve these
  packages, and §2 shows at least one of them (`platform/iam`) is not a bug at all but a
  legitimate multi-entity-transaction shape the canonical single-entity pipeline cannot
  express.
- Claiming "at-least-once" is sufficient without stating *which* layer actually provides the
  idempotency backstop for workflow starts (Temporal's `WorkflowID` convention) versus
  generic domain events (nothing — subscriber's own responsibility) would leave a real gap
  in what Phase 2 promises versus delivers. §13 makes this explicit.

---

## 4. ActionRuntime Verdict — REVISED

**Verdict: REPAIR `runtime.ActionContext`, THEN ADOPT it as the canonical action-execution
implementation. REMOVE `runtime.RuntimeFactory`/`defaultActionRuntime` as superseded dead
design once the former is wired — do not leave both.**

Evidence, re-verified this pass:

- **Two, not one, dead implementations of `def.ActionRuntime` exist.**
  `runtime/action_runtime_impl.go` (`RuntimeFactory`/`defaultActionRuntime`, evaluated by
  the original plan) and `runtime/runtime_action_context.go` (`ActionContext`/
  `NewActionContext`, missed by the original plan). Both were introduced in the same bulk
  monorepo-import commit (`6ee6557`, ~3430 lines, no explanatory message — git archaeology
  is a dead end here, confirmed by `git log --follow` on all four relevant files landing on
  that one commit with no prior history to inspect).
- **Documentation authority is decisive, and was not in the original plan.**
  `docs/13-actions/ACTION_RUNTIME_REFERENCE.md` ("Frozen at v1.0") states `ActionRuntime` is
  "the execution environment injected into every `ActionHandlerFunc`" and specifies all 11
  methods. `docs/13-actions/ACTION_HANDLER_GUIDE.md` and `CUSTOM_ACTIONS_EXAMPLES.md` (both
  frozen v1.0) show **every single code example** calling `action.Runtime.Repo/.Tx/
  .Publish/.StartWorkflow`. This settles question (D) from the challenge decisively:
  `ActionRuntime` **is** the framework's own authoritative, intended architecture — not a
  future/aspirational one, and not something safe to delete. `def/action.go:39-40`'s own
  normative comment agrees: "The handler must not open database transactions directly — use
  the EntityRepository provided via ActionContext.Repo for all persistence." The real
  dispatch path violating this (`ActionContext.Runtime` always nil) is a bug against the
  framework's own stated contract, not a design gap.
- **`runtime.ActionContext` is the architecturally closer-to-correct of the two dead
  implementations.** It is constructed from the *real* `events.Publisher` and
  `workflow.WorkflowExecutor` interfaces directly (`runtime_action_context.go:30,34`), not
  redundant wrapper types. Its constructor (`NewActionContext`, lines 104-116) panics on
  missing required dependencies rather than silently degrading — a stricter, safer
  contract than `RuntimeFactory`'s.
- **Neither implementation actually delivers the commit-deferred semantics both promise —
  and this matters differently for `Publish` versus `StartWorkflow`.**
  - `ActionContext.Publish` (lines 170-183) is a bare pass-through to `a.publish.Publish(ctx,
    ...)`. Its own doc comment claims "The event is written in the same transaction as the
    mutation that caused it" — this claim becomes **true, not aspirational**, the moment
    Step 1 of the original plan (fix `OutboxWriter.Publish` to join the transaction in
    `ctx`) lands, **provided callers always invoke `Publish` from inside `Tx`'s callback**.
    There is no enforcement preventing a caller from calling it outside `Tx` — but once
    Step 1 lands, `events.Publisher`'s own contract ("return an error if no active
    transaction is present," `events/events.go:68-69`) makes that failure mode fail
    *loudly*, not silently. This is a genuinely different, better-off finding than the
    original plan's characterization of "the doc comment is simply false" — it is
    conditionally true, and becomes unconditionally enforced-or-erroring once Step 1 lands.
  - `ActionContext.StartWorkflow` (lines 187-202) has **no such saving grace**. It calls
    `a.executor.Start(ctx, ...)` immediately, synchronously, with **no outbox indirection
    whatsoever** — regardless of whether it's invoked inside or outside `Tx`. If a handler
    calls it inside a `Tx` callback that later fails, the workflow has already started
    against Temporal with no way to un-start it. **This is P0-B, reproduced verbatim inside
    the very abstraction the original plan proposed adopting to help close P0-B.** This was
    not caught by the original plan because it evaluated `defaultActionRuntime`'s equally
    broken `StartWorkflow` and concluded "needs repair" in the abstract, without tracing
    that the sibling implementation has the identical concrete bug.
- **Required repair, concretely:** rewrite `ActionContext.StartWorkflow` to publish a
  `WorkflowTriggerFired` event via `a.publish` (the same outbox mechanism `Publish` already
  correctly delegates to) instead of calling `a.executor` directly. This means
  `ActionContext` no longer needs an injected `executor workflow.WorkflowExecutor` field at
  all — only the new outbox-relay `Subscriber` (§8) needs one. This collapses what would
  otherwise be **two** independent code paths that each call Temporal directly
  (`EntityService.startWorkflows` and `ActionContext.StartWorkflow`) into **one** (both
  publish outbox events; only the relay's subscriber ever touches
  `workflow.WorkflowExecutor`) — directly satisfying the plan's own "one source of truth"
  principle, applied one level deeper than the original plan noticed.
- **Circular dependency: none, proven two independent ways** (item J from the challenge).
  First: every method signature on `runtime.EntityDriver`/`EventBus`/`WorkflowRuntime`
  (`action_runtime_impl.go`) and every dependency field on `ActionContextConfig`
  (`runtime_action_context.go`) is expressed purely in terms of `def`-package types plus
  stdlib — Go's structural interface satisfaction means `contrib/pgx`, `events/outbox`, and
  `workflow` never need to import `runtime` to provide a value satisfying these interfaces;
  only the wiring call site needs to reference `runtime` at all. Second, independently:
  `bootstrap/bootstrap.go` currently imports only `audit`, `compiler`, `registry` (confirmed
  by direct read) and has **zero importers anywhere outside `cmd/*`** (confirmed via
  repo-wide grep) — so even if a concrete adapter needed to import `runtime` directly, doing
  that wiring inside `bootstrap` (which can safely import anything below it) creates no
  cycle. `contrib/pgx` already imports `runtime` today (for `runtime.NotFoundError`/
  `BusinessError`), confirming the edge only ever runs one direction.
- **Bulk import using it (H):** not directly — bulk import's per-flush-batch design (§9)
  doesn't need the full `ActionRuntime` surface (`Notify`, `Cache`, arbitrary-entity `Repo`);
  it needs `runtime.Pipeline`'s hook/validation methods plus the same `events.Publisher` for
  its own outbox writes. `ActionContext` and bulk import should both depend on the same
  lower-level pieces (`Pipeline`, the fixed `Publisher`) rather than bulk import depending on
  `ActionContext` itself — sharing the outbox mechanism, not the action-specific interface.
- **Normal CRUD using it (I):** same answer — `EntityService.Create/Update/Delete` should
  gain its own direct call to the fixed `events.Publisher` for its outbox write; it does not
  need to construct a full `ActionContext` internally (it doesn't need `Repo(entityName)`
  for an arbitrary *other* entity, `Notify`, or `Cache`). Forcing `EntityService` through
  `ActionContext` would be exactly the kind of unjustified abstraction the challenge warned
  against — the two call sites converge on sharing `events.Publisher`, not on sharing a
  common Go struct beyond that.
- **Infrastructure replaceability (K):** unaffected — `events.Publisher`/
  `workflow.WorkflowExecutor` were already correctly infrastructure-agnostic interfaces
  before this review; nothing here changes that boundary.

---

## 5. Canonical Mutation Pipeline Verdict — REVISED

**Verdict: two legitimate mutation primitives exist and should both be named explicitly,
not one canonical pipeline for everything.**

The mutation-path matrix (traced fresh this pass, all paths file:line cited) found:

| Path | Validation | Hooks | Transaction | Audit | Events/Outbox | Workflow |
|---|---|---|---|---|---|---|
| HTTP Create/Update/Delete | Pipeline | invoked | `WithTx`, one entity | `RunAuditRecord`, ADR-017 | none (fixed by Phase 2) | direct Temporal call, no outbox |
| Custom Action | none (framework-level) | not invoked | none by default | none | none | none |
| Bulk Import | none | not invoked | one `WithTx` per 100-row flush | none | none | none |
| IAM Login (`auditLogin`) | ad hoc | not invoked | **one `WithTx` spanning TWO entities** (`iam_sessions` + `iam_users`) | separate `writeAuthAudit`, bypasses ADR-017, errors discarded | none | none |
| IAM Logout | ad hoc | not invoked | `WithTx` wrapping `BulkUpdate` | same as Login | none | none |
| `platform/audit.Writer` | n/a | n/a | participates in caller's TX, never owns one | is the audit mechanism | none | none |
| `platform/metadata` | ad hoc | **claimed** ("FieldNameValidator hook" — doc comment) but **not invoked** (no `Pipeline` reference at all) | none | none | none | none |
| `platform/registry` | ad hoc | not invoked | none | none | none | **claimed** ("WorkflowTrigger fires provisioning workflow") but **structurally impossible** from this call path — never calls `EntityService` |
| `platform/settings` | ad hoc | not invoked | none | none | none | none |
| `platform/notifications` | ad hoc | `DispatchHook.AfterCreate` registered but **likely dead** — the traced `Service.Create` calls the bare repository, and `AfterCreate` only fires from `Pipeline.RunAfterCreate`, which this call site never constructs (flagged as *unconfirmed*, not asserted, since the wiring might exist outside the files traced) | none | none | none | none |

**Answer to the central question ("can we genuinely create ONE canonical pipeline without
breaking legitimate low-level/internal use cases?"): no.** The IAM Login row is the proof:
its `WithTx` call legitimately spans two different entities (`iam_sessions.Create` +
`iam_users.Update`) in one transaction — a shape `EntityService.Create`/`Update`'s one-call-
one-entity signature structurally cannot express without inventing a multi-entity variant
nothing currently needs anywhere else. Separately, `platform/audit.Writer` cannot ever be
routed through `runtime.Pipeline` without infinite recursion (Pipeline auditing its own
audit write) — a **permanent, structural exception**, not a temporary gap.

**Minimum legitimate mutation primitives: exactly two.**

1. **The canonical single-entity pipeline** (`runtime.Pipeline` + `contrib/pgx.Repository.
   WithTx`, via `EntityService` or an equivalent thin wrapper) — for anything mutating ONE
   entity under full business/security semantics: HTTP CRUD (already there), custom actions
   (once `ActionContext` is wired, §4), and bulk import (once migrated, §9). **This is
   Phase 2's actual scope**, and it correctly closes both P0s without needing to touch
   anything else in the matrix above.
2. **The raw transactional primitive** (`contrib/pgx.Repository.WithTx` + direct
   `Repository.Create`/`Update`/`BulkUpdate` calls, no `Pipeline` involvement) — for
   framework-internal, multi-entity, or recursion-sensitive operations that have always
   legitimately needed lower-level control: IAM's login/logout audit-trail writes, and
   `platform/audit.Writer` itself (which *must* stay outside the canonical pipeline,
   permanently, by construction).

`platform/metadata`/`registry`/`settings` sit in neither category cleanly today — they use
primitive 2's *shape* (bare repository calls) but without even `WithTx` (no transaction at
all in the traced code), and their doc comments overclaim hook/workflow behavior that
doesn't exist. This is a **real, separate finding**, correctly **out of Phase 2's scope**
(§17) since it doesn't intersect either P0 — but it should not be silently dropped; it
belongs in `tasks.md` as its own item (see §21).

---

## 6. Transaction Ownership Verdict

**Unchanged from the original plan, confirmed correct by this review's deeper trace:**
`contrib/pgx.Repository.WithTx` remains the sole transaction boundary and owner — confirmed
(again, this pass) that no other `BeginTx` call exists anywhere outside `contrib/pgx`, and
every mutation path in the matrix above that uses a transaction at all uses this same
mechanism (including the IAM multi-entity case, which nests two repository calls inside one
`WithTx`, not two separate transactions). The strongest enforcement mechanism for "mutation
+ audit + outbox must share one transaction" is **structural, not procedural**: put the
outbox-publish call in the same function body, between the same two lines
(`RunAuditRecord`/`RunAfterX` and the `WithTx` callback's `return nil`) that already
guarantee audit atomicity today — there is no separate "commit" step to accidentally
diverge from, because `WithTx`'s own deferred-rollback-unless-committed pattern (Phase 1's
panic-safety fix) already makes "everything in this callback is atomic" the only way the
code can be written. No new enforcement primitive (a linter, a wrapper type) is needed; the
existing `WithTx` shape already prevents accidental transaction-splitting by construction.

---

## 7. Audit Architecture Verdict — REVISED

**Unchanged for the canonical pipeline path** (ADR-017's category-based failure policy,
`runtime.Pipeline.RunAuditRecord` inside the transaction) — confirmed still correct and
unaffected by anything in this review.

**New finding this pass, not in the original plan:** the codebase has **at least three**
distinct "audit-shaped" write mechanisms, not one: (1) `awo.so/awo/audit`'s
`Pipeline.RunAuditRecord` + `audit.Apply`/ADR-017 (the canonical one, used only by
`EntityService`); (2) `awo.so/awo/platform/audit.Writer` (a *different* package, despite the
similar name and role — writes `iam_audit_log`-shaped records, expects to run inside a
caller-owned transaction, never owns one itself); (3) `platform/iam/service.go`'s own
`writeAuthAudit`, which discards `AuditWriter.Write` errors unconditionally regardless of
category, bypassing ADR-017's propagate/suppress distinction entirely, with an explicit
comment justifying it ("the session is already live in Redis"). Mechanism (3) is
**defensible as a deliberate, documented choice** (Redis is the authoritative session store;
the PostgreSQL audit trail is explicitly best-effort by design, established well before
Phase 2) — but it means "every successful mutation has a corresponding audit record" is
*already* not a uniform invariant across the codebase, independent of anything Phase 2
touches. Phase 2 should not claim to unify these three; that would be real scope creep. It
should state clearly, in whatever documentation Phase 2 updates, that the ADR-017 guarantee
applies to the canonical pipeline specifically, not to every audit-shaped write in the
system.

---

## 8. Outbox Architecture Verdict — REFINED

**"One outbox, not two" still holds — refined with a concurrency/isolation correction the
original plan missed.** Direct read of `events/outbox/relay.go`'s `poll()` (lines 83-169)
found: the entire poll cycle — advisory-lock acquisition, the `SELECT ... FOR UPDATE SKIP
LOCKED` claim, and the delivery loop calling `deliver()` for up to 50 events — runs on
**one held connection**, serialized cluster-wide by a **session-level** advisory lock
(`pg_try_advisory_lock`, not the `_xact_` transaction-scoped variant) held for the entire
function's duration via `defer conn.Exec(ctx, "SELECT pg_advisory_unlock...")` at the very
end. This means: **while `deliver()` is looping over events, calling potentially-slow
external code, no other relay instance (and no other poll cycle from the same instance) can
make any progress at all** — the advisory lock, not the row-level `FOR UPDATE`, is what
actually serializes access (the row lock itself is released the instant the `SELECT`
statement's own implicit auto-commit transaction ends, before `deliver()` ever runs — this
does not cause a bug today, because the advisory lock already prevents concurrent
`poll()` calls from existing at all, but it means `FOR UPDATE SKIP LOCKED` is decorative
here, not load-bearing).

**Consequence for extending this relay to workflow-trigger dispatch (the original plan's
§10/§11 recommendation):** a call to `workflow.WorkflowExecutor.Start` is a network call to
an external system (Temporal) with a materially different latency/failure profile than an
in-process domain-event `Subscriber.HandleEvent`. Adding it to the *same* serial
`deliver()` loop, under the *same* whole-cycle advisory lock, means a slow or hanging
Temporal call would stall delivery of every *other* pending event (including ordinary
domain events unrelated to workflows) for however long that call takes — `poll(ctx)` has no
per-event timeout today. **Refinement to the original plan's outbox design (§10/§11):**
keep one table, one claiming mechanism (the advisory lock + `SKIP LOCKED` combination is
fine for claiming), but give `deliver()` (or its caller) a bounded per-event timeout
(`context.WithTimeout` wrapping each `Subscriber.HandleEvent`/`WorkflowExecutor.Start`
call) so one slow external system cannot head-of-line-block the whole batch. This does not
reverse "one outbox, not two" — it is a concurrency-isolation correction *within* that one
design, and should be folded into Step 1/Step 6 of the implementation order (§18) rather
than treated as a new step.

---

## 9. Workflow Durability Guarantee — Defined Precisely

Re-tracing `workflow trigger → event representation → workflow lookup → executor →
Temporal client → retry → idempotency → tenant context → authorization context → audit`:

- **Event representation:** `def.WorkflowTrigger` (`def/workflow.go:19-39`) — `On`,
  `WorkflowFn`, `TaskQueue`, `InputBuilder`, `WorkflowIDFunc`. Confirmed live and read by
  `EntityService.startWorkflows` today (unchanged by this review).
- **A third, previously-unexamined workflow-trigger path exists and is 100% non-functional:**
  `def.ActionDef.WorkflowEvent` (`def/action.go:55-58`) — its doc comment claims "the
  runtime executes the trigger after the handler returns a non-error result." Repo-wide
  search found its **only** other reference is `sdui/adapt/adapt.go:470`, which is SDUI
  metadata rendering (almost certainly just exposing whether an action *would* trigger a
  workflow, for UI display), not execution logic. **This is a third confirmed instance,
  found this pass, of the same pattern**: a doc comment describing behavior with no
  implementing code anywhere. Phase 2 does not need to fix this (it's not part of either
  P0), but it must not be assumed to work if any future action handler relies on it, and it
  should be flagged in `tasks.md` (§21) so it isn't silently rediscovered as a surprise
  later.
- **Executor/Temporal client:** `workflow.WorkflowExecutor` (`NoopExecutor`/
  `TemporalExecutor`) is correctly infrastructure-agnostic, confirmed sound, unchanged.
- **Retry/idempotency at the workflow-start level:** `workflow/id.go`'s `BuildID`
  convention (`{tenantID}.{entityType}.{recordID}.{event}`) gives Temporal-level
  start-deduplication for free — a second `Start` call with the same `WorkflowID` against an
  already-started workflow does not create a duplicate execution. This is the concrete
  mechanism that makes "at-least-once dispatch" safe to retry for workflow starts
  specifically (§13 expands this).
- **No Temporal worker is ever started** by either binary (confirmed again this pass, no
  change from the original plan's finding) — `workflow.NewWorker` has zero production
  callers; both `cmd/server/main.go` and `cmd/awo/serve_impl.go` only ever construct a
  client-side `temporalclient.Client`.

**The precise guarantee Phase 2 delivers, stated without overclaiming:**
**(B) durable event + reliable relay, with (D) at-least-once workflow *dispatch* as a
direct consequence** — not (A) "durable intent only" (that undersells it: once Step 1 and
the `WorkflowTriggerFired` event type land, a dispatch attempt really does survive a crash
between commit and delivery, repeatedly, until it succeeds or is exhausted), and
definitely not (C) actual workflow execution or (E) exactly-once execution — neither is
achievable without a running Temporal worker and real, deterministic workflow functions,
neither of which exist and neither of which Phase 2 proposes building. "Exactly-once
*dispatch*" is not claimed either — the relay's at-least-once redelivery window (§14) means
a `Start` call could genuinely be issued twice for the same event; it is *safe* to do so
(Temporal's dedup), not *guaranteed not to happen*.

---

## 10. Relay Guarantee — Defined Precisely

**At-least-once delivery, not exactly-once, confirmed by direct trace of the crash window:**
`poll()` claims rows (`FOR UPDATE SKIP LOCKED`, immediately released once that statement's
own implicit transaction ends — see §8), calls `deliver()`, then issues a *separate*
`UPDATE ... SET delivered_at = NOW()` statement. A crash between `deliver()` succeeding and
that `UPDATE` committing leaves `delivered_at` still `NULL`, so the next poll cycle
redelivers the same event. This is exactly the guarantee the package's own doc comment
claims ("At-least-once delivery is guaranteed," `events/outbox/relay.go:4`) — confirmed
true, not overclaimed, for this specific property.

**What is NOT yet true, confirmed this pass:** the package's doc comment also claims failed
rows are "moved to the dead-letter table" (line 10) — no such table or logic exists; a row
that exhausts `attempts < 5` simply stops appearing in future poll results, silently,
forever, with no operator-visible signal. This is the same poison-event gap the original
plan already identified (§11 there) — re-confirmed, not new, but worth restating precisely
here since the challenge asked for the relay's actual guarantee, not its documented one.

**Ordering:** not guaranteed across concurrent claims for different rows of the *same*
entity (`ORDER BY occurred_at ASC` plus `SKIP LOCKED` does not prevent a later-occurring
event for record X being claimed and delivered before an earlier one, if they land in
different `poll()` batches) — acceptable for Phase 2 (nothing in the current entity model
requires strict per-record event ordering), but should be stated as an accepted limitation,
not silently assumed away.

---

## 11. Bulk Import Architecture — Confirmed, One Addition

The original plan's per-flush-batch design (validate the whole flush before committing its
batch insert, batched at the SQL level per existing `BatchSize`) is confirmed still correct
after re-tracing `ioport/importer.go` and `contrib/pgx/repo.go`'s `BulkCreate` this pass — no
new finding reverses it. **One addition this review makes:** the outbox write (§8) must
also happen once per *record* inside the flush's transaction, not once per *flush* — an
import of 100 rows produces 100 outbox events (one per created record, matching what a
100-call loop of `POST` would have produced), not one summary event, so downstream
`Subscriber`s cannot tell the difference between an entity created via the API and one
created via import. This was implicit in the original plan's "identical to the single-record
path" framing but should be stated explicitly as an acceptance criterion (§20).

Volume consideration for very large imports (10,000-1,000,000 rows), which the original plan
did not size: at 100 rows/flush, a 1,000,000-row import produces 10,000 transactions and
1,000,000 outbox rows. The outbox table's `events_outbox_pending` partial index
(`WHERE delivered_at IS NULL`) keeps the *pending* working set small regardless of total
table size, so this is not a correctness risk — but it is a real operational one (relay
throughput must exceed import throughput, or the pending backlog grows unbounded during a
large import). Out of Phase 2's scope to solve (no current import is anywhere near this
scale, confirmed by the absence of any large-dataset import in this codebase's own test
fixtures or docs), but worth naming as a known scaling limit rather than silently ignoring
it.

---

## 12. Hook Semantics — Confirmed, One Clarification

Unchanged from the original plan (§2.4/§14 there): nine hook types, all invoked, all
panic-safe via `safeCall`, no `AfterCommit` hook type exists. **Clarification this pass:**
`AfterCommit` should **not** become a new hook type. The two concrete needs that might
suggest one — "publish an event after this mutation" and "start a workflow after this
mutation" — are both already correctly served by calling `events.Publisher.Publish` (or,
once §4's repair lands, `ActionContext.Publish`/`StartWorkflow`) from *inside* the existing
`AfterCreate`/`AfterSave` stage (still pre-commit, same transaction) rather than from a
hypothetical post-commit stage. A genuine `AfterCommit` hook would need its own delivery
guarantee (what happens if the hook itself fails, after the transaction has already
committed and cannot be rolled back?) — which is exactly the problem the outbox pattern
already solves. Introducing a distinct `AfterCommit` hook type would create a second,
competing "guaranteed-after-commit work" mechanism alongside the outbox, contradicting the
plan's own single-source-of-truth principle. **Answer to the challenge's direct question:
`AfterCommit`-shaped work should be a durable outbox event, never an in-process
notification** — an in-process callback cannot survive the crash the outbox exists to
survive.

---

## 13. Tenant-Context Model — Minimum Durable Context Defined

Re-deriving the propagation chain `HTTP/API → mutation → DB transaction → outbox record →
relay → workflow executor → workflow`, and the challenge's explicit instruction not to
blindly persist authorization state:

**Must be durably stored on the outbox row** (already true or already planned in §10 of the
original plan): `tenant_id` (required — RLS depends on it being reconstructable, and it's
already a real column, `events/events.go:40`), `entity_name`/`record_id` (already real
columns), `actor_id` (already a real column — who *caused* the event; needed for audit
trails and notification targeting, safe to persist since it's an identity reference, not a
capability), `correlation_id` (recommended addition, original plan §10 — safe, it's an
opaque tracing token, not authorization state).

**Must be reconstructed, never persisted:** permissions/roles. The challenge's own warning
("Do not blindly persist authorization state if it would become stale or unsafe") is
correct and this review confirms the original plan never proposed persisting them — a
relayed event's `Subscriber`/workflow must re-resolve the acting principal's *current*
permissions at delivery time (potentially minutes, hours, or days after the causing
mutation, given the 24-hour terminal-failure window `docs/08-workflow/OUTBOX_SPEC.md`
describes), not reuse a snapshot from event-creation time — a role revoked in the interim
must take effect. This means any `Subscriber` or workflow that performs its own
authorization-sensitive action must call the real authorization system fresh, using only
`actor_id` (identity) from the event, never a cached permission set.

**Not needed for Phase 2, confirmed by absence of a current use case:** service-account ID
as a distinct field (not found as a separate concept from `actor_id` anywhere in
`events.DomainEvent`'s current design or its callers), locale/timezone (no `Subscriber` or
workflow trigger in the current codebase does anything locale-sensitive — `platform/
notifications`' dispatch is the closest candidate and it resolves locale from the recipient
user record, not from the triggering event), causation ID (per the original plan's own
reasoning, still valid — no event chains deep enough to need it today).

**org_id:** deliberately **not** added to the durable outbox schema by this review, despite
`tenant_id` being present — `ScopeOrganization`'s RLS boundary is currently broken
(`tasks.md` 1.18, explicitly out of scope for both the original plan and this review) and
zero entities use it; adding `org_id` to the outbox schema now would be premature
infrastructure for a scope that doesn't safely exist yet. Add it when 1.18 is resolved, not
before.

---

## 14. Idempotency Model — Confirmed, With Named Identities

The original plan's exclusion of full API-level idempotency stands, re-confirmed correct —
still orthogonal to both P0s. Named identities, as the challenge requested:

- **Event identity:** `events.DomainEvent.ID` (UUIDv7, `events/events.go:37`) — already
  exists, already unique (PostgreSQL primary key once the migration lands).
- **Mutation identity:** the entity record's own primary key (`id`) plus its `updated_at`
  timestamp — sufficient to detect "this is the same logical mutation" for audit/debugging
  purposes; not proposed as a new mechanism, already implicit in the existing schema.
- **Workflow identity:** `WorkflowID`, via `workflow/id.go`'s `BuildID` convention — already
  exists, already enforced by Temporal itself (a `StartWorkflowOptions` call with a
  duplicate ID against a running/completed workflow is rejected or returns the existing
  execution, depending on `WorkflowIDReusePolicy` — Awo's current `TemporalExecutor` does
  not set this policy explicitly, which is worth a one-line note in the ADR (§17) but not a
  Phase 2 code change, since the default policy already prevents concurrent duplicates).
- **Relay attempt identity:** implicit in the row's own `id` + `attempts` counter — no new
  identity needed; the existing schema already supports "how many times has this event been
  attempted."
- **Consumer deduplication identity:** **does not exist today for generic `Subscriber`s**,
  and Phase 2 correctly does not propose building one — per `docs/02-pipeline/
  HOOK_CONTRACT.md`'s already-existing normative rule (extended by analogy from hooks to
  subscribers), a `Subscriber.HandleEvent` implementation "MUST be idempotent where
  possible." Phase 2's job is to document this expectation explicitly for `Subscriber`
  authors (already in the original plan's §14), not to build a generic dedup cache nobody
  has asked for.

**Where uniqueness must be enforced by PostgreSQL:** the outbox table's primary key (`id`)
is the only uniqueness constraint Phase 2 needs — it prevents a double-`INSERT` of the exact
same event row (which cannot happen anyway, since `OutboxWriter.Publish` always generates a
fresh `uuid.New()` when `e.ID` is zero, `relay.go:201-203`), and per-tenant/per-entity
uniqueness of *content* (e.g., "only one `entity.created` event for record X") is not a
property anything in the current design needs or requests.

---

## 15. ADR Contradictions — Precise, With a Proposed Superseding Structure

**ADR-007 (`docs/08-workflow/OUTBOX_SPEC.md`) promises:** a dedicated `workflow_outbox`
table, written **outside** the entity transaction in "a separate, short-lived transaction"
with a bounded local retry (3 attempts, 100ms apart) for the write itself; a 5-attempt
exponential backoff (1s/2s/4s/8s/16s, capped 5min) with an explicit `next_attempt_at`
scheduling column; a terminal `status='failed'` after 24 hours with an alert.

**ADR-008 (`docs/09-events/EVENT_OUTBOX_SPEC.md`) promises:** a dedicated `event_outbox`
table, written **inside** the causing transaction (the correct transactional-outbox
pattern); delivery via a described `outbox.EventBroker` interface with Kafka/NATS/
Redis-Pub/Sub adapters under a top-level `awo.so/awo/outbox` package.

**Where they contradict each other:** transaction coupling (ADR-007 explicitly chooses
weaker, post-commit, bounded-retry semantics for workflow starts; ADR-008 chooses the
stronger, same-transaction guarantee for domain events) with no stated reason for treating
the two differently.

**Where both contradict current source:** neither `workflow_outbox` nor `event_outbox` nor
`awo.so/awo/outbox` exists anywhere in the repository (confirmed, `ls outbox` → no such
directory; no migration for either table name found in any migration directory). The real
`events/outbox` package's `Relay`/`OutboxWriter` design (advisory-lock coordination,
`SKIP LOCKED`, flat `attempts < 5` cutoff with **no** exponential backoff or
`next_attempt_at` scheduling at all) matches **neither** document.

**Which invariants are still valid and should be carried forward:** ADR-008's
same-transaction coupling (this is simply correct, and this review's recommendation is to
apply it uniformly, including to workflow triggers, reversing ADR-007's choice rather than
ADR-008's). The 24-hour-terminal-failure concept and alerting (both docs' spirit, neither
implemented) — worth keeping as the dead-letter policy, decoupled from either doc's specific
table.

**Which decisions must be superseded:** ADR-007's separate-transaction-with-bounded-retry
model for workflow starts (replaced by: same mechanism as domain events, same transaction,
no separate retry-the-insert logic needed since it can't fail independently of the
mutation anymore). ADR-007's and ADR-008's schema names/locations (replaced by the single
`events_outbox` table, §10 of the original plan). ADR-008's `outbox.EventBroker`/Kafka/
NATS/Redis-Pub/Sub design (replaced by the real, already-existing `events.Subscriber`
in-process model — no message broker exists or is proposed).

**Whether either should be retained historically:** yes, both, unmodified, as superseded —
per this framework's own `docs/00-overview/DECISION_REGISTER.md` convention (ADRs are
superseded, not deleted, elsewhere in this codebase's own history per `tasks.md` 0.4's
already-planned ADR renumbering work).

**Proposed superseding ADR structure** (not written here — out of this review's scope; a
prerequisite for Phase 2 implementation, per `tasks.md` 0.4's documentation-first
principle). The next available ADR number is genuinely unresolved: `docs/00-overview/
DECISION_REGISTER.md` ends at ADR-024; `tasks.md` 0.4 (never executed) already planned to
insert ADR-025 through ADR-033 for tasks.md's own historical decisions. This review does not
resolve that numbering — implementation must either wait for 0.4 or provisionally label the
new entry (e.g. "ADR-TRANSACTIONAL-OUTBOX (number pending 0.4)") rather than guess a number
that later collides. The new ADR must state, at minimum: one `events_outbox` table
supersedes both `workflow_outbox` (ADR-007) and `event_outbox` (ADR-008); all outbox writes,
including workflow-trigger events, happen in the causing transaction, no exceptions; the
retry policy actually implemented (flat cutoff today, exponential backoff as a Phase 2
addition per §18) with a stated terminal/dead-letter behavior; the relay's at-least-once
(not exactly-once) guarantee stated explicitly, with the Temporal-`WorkflowID`-dedup
backstop named as the reason workflow dispatch specifically tolerates redelivery safely;
the minimum durable tenant-context fields (§13); ownership of workflow *execution*
explicitly deferred to a later phase (§9), with this ADR covering *dispatch* durability
only; the Temporal boundary (`workflow.WorkflowExecutor` as the sole call surface, raw SDK
client access removed from `EntityService`/`ActionContext` alike).

---

## 16. Organisation-RLS Blocker

**Unchanged — correctly out of scope for both documents.** Re-confirmed this pass: still
zero entities declare `ScopeOrganization`, still tracked as `tasks.md` 1.18, still blocking
Phase 3 rather than Phase 2. Nothing in this review's findings interacts with it (the
`org_id` durable-context question in §13 explicitly defers to 1.18's resolution rather than
building around the current broken state).

---

## 17. Revised Phase 2 Scope

Re-run through every item, including ones the original plan didn't explicitly classify:

| Item | Classification | Why |
|---|---|---|
| Fix `OutboxWriter.Publish` transaction join | **REQUIRED** | unchanged, highest leverage |
| `events_outbox` migration | **REQUIRED** | unchanged |
| Restore tenant context in `Relay.deliver` | **REQUIRED** | unchanged |
| Per-event timeout in `deliver()`'s dispatch loop | **REQUIRED (new this review)** | §8 — without it, adding Temporal calls to the shared relay risks head-of-line blocking every other pending event |
| Add outbox write to `EntityService.Create/Update/Delete` | **REQUIRED** | unchanged, closes the transactional half of P0-B |
| Migrate bulk import onto the canonical pipeline | **REQUIRED** | this is P0-A |
| `WorkflowTriggerFired` event type + relay subscriber calling `WorkflowExecutor` | **REQUIRED** | this is P0-B |
| Retype `EntityService`/`router.RegisterOptions` off the raw Temporal client | **REQUIRED** | same call site as the above; doing it separately later means touching it twice |
| Repair + wire `runtime.ActionContext` (not `RuntimeFactory`) | **REQUIRED (revised from the original plan's less specific recommendation)** | §4 — closes the action-path half of P0-B-adjacent risk and fulfills the framework's own documented contract |
| Remove `runtime.RuntimeFactory`/`defaultActionRuntime` | **REQUIRED (new this review)** | leaving two unwired implementations of the same interface after wiring one is a maintenance hazard the plan should close, not create |
| Poison-event/dead-letter handling | **REQUIRED** | unchanged, small, same files already open |
| Exponential backoff + `next_attempt_at` (matching the *retry policy* both frozen ADRs at least agree is desirable) | **DEPENDENCY of poison-event handling** | the flat `attempts < 5` cutoff with no backoff means 5 retries can exhaust in under a poll-interval's multiple — worth doing alongside dead-letter handling, same file |
| `HOOK_CONTRACT.md` correction | **REQUIRED** | unchanged, small, documentation-first |
| New ADR superseding ADR-007/ADR-008 | **REQUIRED, BEFORE CODING** | §15 — this review, like the original plan, cannot write it, but implementation must not skip it |
| `platform/metadata`/`registry`/`settings` missing `WithTx`/validation | **SHOULD BE LATER** (new finding, not in original plan) | real gap, same *shape* as the P0s, but touches neither P0 directly — belongs in `tasks.md` as its own item, not folded into Phase 2 |
| `platform/notifications`' possibly-dead `DispatchHook.AfterCreate` | **SHOULD BE LATER** (new finding) | needs its own confirmation pass outside this review's traced files before any fix is scoped |
| `ActionDef.WorkflowEvent`'s non-functional doc promise | **SHOULD BE LATER** (new finding) | not part of either P0; flag in `tasks.md`, do not fix inside Phase 2 |
| `runtime/naming`'s parallel, differently-specified series implementation | **SHOULD BE LATER** (new finding) | apparent dead code with a conflicting durability claim versus the real `naming` package; deserves its own confirmation pass, not a Phase 2 distraction |
| Temporal worker / real workflow functions | **LATER** | unchanged from original plan, reinforced by §9's finding that no worker exists today either |
| Full API idempotency | **LATER** | unchanged |
| Everything else in the original plan's §20 (OpenAPI, search, dependency-graph violations, CLI, migration engine) | **NOT NEEDED FOR PHASE 2** | unchanged, re-confirmed orthogonal |

---

## 18. Revised Implementation Order

Same overall shape as the original plan's §21, with three corrections: Step 4 now targets
`runtime.ActionContext` specifically (not `RuntimeFactory`) and includes removing the
sibling dead implementation; a new Step 3b adds per-event dispatch timeouts to the relay
before workflow-trigger events are added to it in Step 6; Step 6 now has `ActionContext.
StartWorkflow` in its file list alongside `EntityService.startWorkflows`, since both must be
fixed together to actually close the workflow-dispatch half of P0-B (fixing only one leaves
the other as a live regression risk the moment any real action handler is written).

1. Fix `OutboxWriter.Publish` transaction join (unchanged).
2. `events_outbox` migration (unchanged).
3. Canonical pipeline: add the in-transaction outbox write to `EntityService.Create/Update/
   Delete` (unchanged).
   **3b (new). Add a bounded per-event timeout to `Relay.deliver`'s dispatch loop**, before
   any external-network-call event type (workflow triggers) is added to it — dependency of
   Step 6, cheap to do now while the file is open for Step 1's fix anyway.
4. **Repair and wire `runtime.ActionContext`** (not `RuntimeFactory`): rewrite
   `StartWorkflow` to publish a `WorkflowTriggerFired` event instead of calling the executor
   directly; construct a real `ActionContext` in `EntityHandler.Action`
   (`api/handler/crud.go`) and set `ActionContext.Runtime`; **delete
   `runtime/action_runtime_impl.go`'s `RuntimeFactory`/`defaultActionRuntime`** once
   `ActionContext` is confirmed working, so exactly one implementation remains.
5. Migrate bulk import onto the canonical pipeline (unchanged, closes P0-A) — including the
   per-record (not per-flush) outbox event requirement named in §11.
6. `WorkflowTriggerFired` event type + relay subscriber calling `workflow.WorkflowExecutor.
   Start`; retype `EntityService.startWorkflows` AND `ActionContext.StartWorkflow` (both,
   per the correction above) onto this same mechanism; retype `router.RegisterOptions.
   Temporal` to `workflow.WorkflowExecutor`. Closes P0-B.
7. Poison-event/dead-letter handling + exponential backoff/`next_attempt_at` (folded
   together, same file) + `HOOK_CONTRACT.md` correction (unchanged from original plan,
   backoff addition new this review).

Each step's own GOAL/WHY/DEPENDENCIES/TEST STRATEGY/SECURITY INVARIANTS/ACCEPTANCE CRITERIA
detail from the original plan's §21 still applies except where corrected above; not
restated in full here to avoid duplicating a document this review is deliberately not
modifying.

---

## 19. Required ADR Changes Before Coding

A single new ADR superseding both ADR-007 and ADR-008, per §15's proposed content. This
review, like the original plan, does not write it (out of scope: only this review document
may be created). Implementation must not begin Step 6 (and arguably Step 1, since it
contradicts ADR-007's letter even though it correctly implements ADR-008's) without it —
per `tasks.md` 0.4's own already-stated principle, fixing code before fixing the doc that
will immediately contradict it is backwards, and this project has visibly paid for that
mistake once already (the two contradictory frozen docs this review just traced).

---

## 20. Required Tests

All tests from the original plan's §22 remain required, with two additions this review
found necessary:

- **Multi-entity transaction regression**: confirm `platform/iam`'s login/logout
  `WithTx`-spanning-two-entities pattern (§5) is unaffected by any canonical-pipeline change
  — this is the concrete case proving primitive 2 (§5) must keep working exactly as today,
  unmodified, while primitive 1 changes underneath it.
- **Bulk import produces per-record, not per-flush, outbox events** (§11) — a 100-row
  import must produce 100 outbox rows, each independently deliverable/retryable, not one
  summary event a `Subscriber` can't distinguish from a single API call.
- **`ActionContext.StartWorkflow` inside a rolled-back `Tx`** (§4 of this review) —
  construct a real `ActionContext`, call `Tx(fn)` where `fn` calls `StartWorkflow` then
  returns an error; confirm no outbox row (and hence no Temporal dispatch) survives —
  this is the test that would have caught the bug this review found in the "canonical"
  abstraction itself.
- **Relay dispatch timeout** (§8) — a `Subscriber`/executor that hangs indefinitely must
  not prevent other pending events in the same batch from being attempted.

---

## 21. Explicit Out-of-Scope Items (revised)

Everything in the original plan's §20, plus (new this review, all confirmed as real
findings that must not be silently lost): `platform/metadata`/`registry`/`settings`'s
missing transactional/validation safety net; `platform/notifications`' possibly-dead
`DispatchHook.AfterCreate` (needs its own confirmation, not fixed here); `ActionDef.
WorkflowEvent`'s non-functional doc promise; `runtime/naming`'s parallel, conflicting-claim
implementation versus the real `naming` package. Each of these should become its own
`tasks.md` item — this review recommends they be logged, not that this review or Phase 2
fix them.

---

## 22. Remaining Design Questions

All four from the original plan's §24 remain open and unresolved by this review (where the
outbox write belongs architecturally between `EntityService` and `Pipeline`; the
`SkipErrors`-plus-validation-failure bucketing decision; the unmeasured per-row hook-
invocation performance cost; `bootstrap.Run`'s return-type sufficiency for real adapter
construction). Two new ones, surfaced by this review:

1. **Should `TemporalExecutor`'s `StartWorkflowOptions` set an explicit
   `WorkflowIDReusePolicy`?** (§14) — the current default policy already prevents
   concurrent-duplicate execution for a reused `WorkflowID`, but relying on the SDK default
   rather than an explicit, documented policy choice is worth a one-line decision in the
   new ADR (§15/§19), not a Phase 2 code change on its own.
2. **Should `platform/notifications`' `DispatchHook.AfterCreate` wiring be confirmed before
   or after Phase 2 lands?** (§17) — if it turns out to be genuinely dead (most likely, per
   the traced call site), it's an unrelated cleanup item; if it turns out to be wired
   somewhere this review didn't trace, it changes the mutation-path matrix's notification
   row and should be re-verified against whatever Phase 2 changes `EntityService`/
   `Pipeline` in case it depends on either.
