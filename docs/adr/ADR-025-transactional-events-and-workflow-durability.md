# ADR-025: Transactional Events and Workflow Durability

**Status:** Proposed
**Supersedes (in part):** ADR-007 (Workflow Outbox), ADR-008 (Event Outbox Schema Is the
Public Contract) — both `docs/00-overview/DECISION_REGISTER.md`. Neither is deleted or
rewritten; both remain as historical record, marked superseded where this ADR
contradicts them (§21).
**Extends, does not contradict:** ADR-005 (Audit as Mandatory Pipeline Stage), ADR-013
(Transaction Ownership), ADR-014 (Audit Integration Point), ADR-015 (Unified Actor Model),
ADR-017 (Audit Failure Policy).
**Does not touch:** ADR-009 (Idempotency Keys — API-level, explicitly out of scope, §19),
ADR-010 (Hook Recursion Policy — referenced, not modified, §15), ADR-012 (Organization
Hierarchy — the `ScopeOrganization` RLS blocker remains separately tracked, §21).

---

## A note on this ADR's number

The requesting instruction for this document specified `ADR-009`. `docs/00-overview/
DECISION_REGISTER.md`'s ADR index already assigns ADR-009 to "Idempotency: `X-Idempotency-
Key` header + Redis 24h cache" (status: **Frozen**) — a real, different, already-decided
ADR this document does not touch and must not collide with. Per `DECISION_REGISTER.md`'s
own governance rule ("No architectural decision may be made that contradicts these ADRs
without an ARB review and a new numbered ADR appended to this document"), the correct way
to supersede or extend a Frozen ADR is to append a new number, never reuse one. This
document therefore uses **ADR-025** — the next number after the register's current highest
entry (ADR-024) — instead. One further caveat: `tasks.md` 0.4 (not yet executed) already
tentatively earmarks ADR-025 through ADR-033 for renumbering nine unrelated, pre-existing
`tasks.md`-only decisions into this register, the first of which is "Wire removal." If that
renumbering is executed before this ADR is formally appended to `DECISION_REGISTER.md`,
this ADR's number may need to shift. Treat **025 as provisional** until both efforts are
reconciled by whoever merges this ADR into the register; nothing in this document depends
on the specific number.

**On the "ARB review" clause specifically:** `DECISION_REGISTER.md`'s governance rule, quoted
above, names an ARB (Architecture Review Board) review as a co-requirement alongside a new
numbered ADR. No ARB process, charter, or membership is defined anywhere in this repository
— a search of `docs/` finds no other reference to one. This document does not invent one to
satisfy the letter of that clause; it simply records the gap here rather than silently
ignoring half of a stated governance requirement. Whatever review process this repository's
maintainers actually use to accept or reject a proposed ADR (however informal) is the "ARB
review" this ADR is subject to before its Status can move from Proposed to Frozen/Active.

---

## 1. Context

Phase 1 closed Awo's SQL-injection and transaction-panic-safety gaps
(`PHASE1_SECURITY_CLOSURE_REPORT.md`). Two P0s from the original architecture audit remain
open, tracked in `tasks.md` as 1.3 (bulk import bypasses validation/audit) and 1.4
(workflow-trigger durability). Two planning passes
(`PHASE2_ARCHITECTURE_PLAN.md`, `PHASE2_ARCHITECTURE_PLAN_REVIEW.md`) traced the current
mutation, hook, audit, event, outbox, and workflow-dispatch machinery against source and
found:

- `runtime.Pipeline` (`runtime/pipeline.go`) already implements exactly the sequence
  ADR-013 documents — `BeforeX` hooks outside the transaction, `PERSIST` → `RunAuditRecord`
  → `AfterX`/`AfterSave` inside `repo.WithTx`, workflow dispatch outside and after commit.
  This part of the architecture is sound and unchanged by this ADR.
- Two independent, unwired implementations of `def.ActionRuntime` exist
  (`runtime/action_runtime_impl.go`'s `RuntimeFactory`/`defaultActionRuntime`, and
  `runtime/runtime_action_context.go`'s `ActionContext`/`NewActionContext`) — the real
  action-dispatch path (`api/handler/crud.go`'s `EntityHandler.Action`) uses neither,
  leaving `def.ActionContext.Runtime` permanently nil despite `docs/13-actions/
  ACTION_RUNTIME_REFERENCE.md` (frozen v1.0) documenting `ActionRuntime` as mandatory.
- `events.Publisher`'s documented contract ("the context must carry a transacted
  connection," `events/events.go:65-69`) is violated by its only implementation —
  `OutboxWriter.Publish` (`events/outbox/relay.go`) acquires a fresh connection instead of
  joining the caller's transaction.
- ADR-007 and ADR-008 specify two different tables (`workflow_outbox`, `event_outbox`),
  two different durability models (post-commit-with-bounded-retry for workflow starts;
  same-transaction for domain events), and an `EventBroker`/Kafka/NATS/Redis-Pub/Sub
  design. **Neither table, nor the `EventBroker` design, is wired into the live migration
  path.** The real, applied code (`events/`, `events/outbox/`) is a third, different
  design — an `events.Subscriber`-based in-process relay with no message broker — using a
  table named `events_outbox` (plural "events," matching neither ADR-007's
  `workflow_outbox` nor ADR-008's `event_outbox` singular-"event" naming), with a
  transaction-join bug in its only write path (`OutboxWriter.Publish`).
- **A governance-relevant discovery from this ADR's own review pass, corrected here from
  an earlier draft's inaccurate claim:** a table literally named `events_outbox`, with a
  schema closely matching the live `events.DomainEvent` struct, already exists as SQL text
  in `migrations/20260706000003_create_platform_support.up.sql`. That top-level
  `migrations/` directory has no `//go:embed` directive and no `migration.Register()` call
  anywhere in the repository — confirmed by exhaustive search — so this schema is never
  applied by any live bootstrap or migration-runner path today. It is orphaned SQL text,
  not a deployed table. This ADR's earlier draft stated flatly that "no `events_outbox`
  migration exists anywhere," which was false as written; the accurate claim is that no
  *applied* `events_outbox` table exists. §7 and §22 give this orphaned file an explicit
  disposition rather than silently ignoring it.
- `EntityService.startWorkflows` (`api/service/entity.go`) calls the raw Temporal SDK
  client directly, bypassing the already-correct `workflow.WorkflowExecutor` abstraction,
  with a literal `// TODO: write to outbox table for guaranteed retry` next to a log
  message that already claims the outbox exists.
- No Temporal worker is started by either binary (`cmd/server/main.go`,
  `cmd/awo/serve_impl.go`) — both only ever construct a client-side `temporalclient.Client`.

This ADR resolves the contradiction between ADR-007 and ADR-008, defines the single
mechanism that replaces both, and specifies exactly what Phase 2 builds to close both P0s
without overclaiming guarantees the resulting architecture cannot deliver.

## 2. Problem

Stated as the precise property that must hold and currently does not: **if a mutation
commits and produces a durable side effect (an audit record, a domain event, a workflow
dispatch), that side effect must commit atomically with the mutation, or the mutation must
not commit.** Today: audit already satisfies this (ADR-005/ADR-014, verified unchanged).
Domain events and workflow dispatch do not — `events.Publisher`'s implementation doesn't
honor its own contract, and `EntityService.startWorkflows` doesn't use the outbox pattern
at all, contradicting its own inline comment.

## 3. Architectural Decision

Adopt one durable event mechanism — an `events_outbox` table and its relay — as the sole
handoff boundary between a committed mutation and any downstream processing (domain-event
delivery, workflow dispatch alike). Repair and adopt `runtime.ActionContext` as the
concrete implementation of `def.ActionRuntime` injected into every action invocation.
Extend `EntityService.Create`/`Update`/`Delete` to write to the same outbox mechanism
`ActionContext.Publish` uses, in the same transaction `RunAuditRecord` already runs in.
Route all workflow dispatch — from both `EntityService` and `ActionContext` — through this
outbox, never directly to Temporal from a transaction-bound code path.

## 4. Canonical Mutation Contract

Not "all mutations call one function." The contract every **externally-triggered,
business-entity mutation** (HTTP CRUD, custom actions once wired, bulk import once
migrated) must satisfy:

**A. Before any persistence:** an authenticated actor or an explicit, typed system actor
(ADR-015's `SystemActor` constants — `SystemBootstrap`, `SystemMigration`,
`SystemOutboxRelay`, `SystemScheduler` already exist for exactly this purpose) must be
established; tenant context (`tenant.TenantContext`, unchanged since Phase 1) must be set;
organisation context where the entity's `EntityScope` (ADR-022) requires it (subject to the
`ScopeOrganization` RLS blocker, §21 — not resolved by this ADR); authorization
(`api/authz.RequirePermission`, unchanged); the compiled `EntityDefinition`/`EntitySchema`
resolved (`compiler.CompiledSchema`, unchanged); input validated
(`runtime.Pipeline.validateFields`, unchanged).

**B. At commit time, atomically (same transaction):** the business mutation; the audit
record, if required (ADR-005/ADR-014, unchanged — governed by ADR-017's category-based
failure policy, unchanged); the durable outbox record, if the mutation produces a domain
event or a workflow trigger — **this ADR's addition**, and (§10) *not* subject to ADR-017's
suppress-on-failure leniency: an outbox-insert failure always aborts the transaction,
regardless of the entity's audit category, because durability of dispatch is a different
invariant than durability of the audit trail.

**C. Workflow execution must never determine transaction success.** The transaction's
fate depends only on A and B above. Whether Temporal ever receives the dispatch, and
whether it ever runs, happens strictly after commit, on a separate, retryable path (§13).

**D. Direct Temporal calls from any transaction-bound mutation path are prohibited.**
`EntityService.startWorkflows`'s current direct `temporalclient.Client.ExecuteWorkflow`
call, and `ActionContext.StartWorkflow`'s equally direct call to `a.executor.Start`, both
violate this and are the two call sites this ADR requires be rewritten (§13).

**E. The durable outbox row is the only handoff boundary** between "the mutation is done"
and "something downstream should happen." No other channel (in-process callback, direct
SDK call, hook side effect) may carry a durability promise across process/crash
boundaries.

**F. Dispatch is at-least-once, never exactly-once** (§11) — stated here as a contract
term, not an implementation detail, because it constrains G.

**G. Every event consumer (`events.Subscriber` implementation, and the workflow-dispatch
subscriber this ADR adds) must be idempotent** — re-processing the same event a second time
must not produce a different or duplicated real-world effect. This mirrors, and is governed
by the same reasoning as, `docs/02-pipeline/HOOK_CONTRACT.md`'s existing normative rule for
`after_save` hooks.

## 5. Transaction Ownership

**Unchanged from ADR-013, restated for precision, not re-decided.** The driver
(`contrib/pgx`) owns and manages the transaction mechanically via
`Repository.WithTx`; the service layer (`EntityService`, and — once repaired —
`ActionContext.Tx`, which is itself a pass-through to the same driver-level `Tx`/`WithTx`)
orchestrates when a transaction opens. This ADR adds exactly one new step to the sequence
ADR-013 already documents, at the exact point ADR-014 already places `RunAuditRecord`:

```
pipeline.RunBeforeCreate(pctx)                    // OUTSIDE TX — unchanged

repo.WithTx(ctx, func(txCtx context.Context) error {
    // TX begins here (inside driver.WithTx) — unchanged
    repo.Create(txCtx, input)                     // PERSIST — unchanged
    pipeline.RunAuditRecord(txCtx, ...)           // AUDIT RECORD — unchanged (ADR-005/014)
    pipeline.RunAfterCreate(txCtx, created)        // after_save hooks — unchanged
    publisher.Publish(txCtx, outboxEvent)          // OUTBOX RECORD — NEW, this ADR
    return nil
})                                                  // TX commits; or rolls back on error — unchanged

relay.dispatch(...)                                // OUTSIDE TX, separate process/goroutine — NEW, replaces direct startWorkflows() call
```

**Enforcement, not just convention:** the new outbox-write call sits inside the same
callback body, between the same two lines (`RunAuditRecord`/`RunAfterX` and the callback's
`return nil`) that already make audit atomicity structural rather than a code-review
convention. There is no separate "commit the outbox" step to accidentally diverge from —
`Repository.WithTx`'s existing deferred-rollback-unless-committed shape (Phase 1's panic-
safety fix) already makes "everything in this callback commits together or not at all" the
only way the code can be written. No new enforcement primitive (linter, wrapper type) is
introduced or needed.

`ActionContext.Tx` (`runtime/runtime_action_context.go:163-165`) delegates to the same
driver-level mechanism via its injected `txFn` — it is not a second transaction owner, it
is the same one, reached through a different Go type.

## 6. Audit Atomicity

**Unchanged, ADR-005/ADR-014/ADR-017 remain fully in force.** This ADR's only addition:
the new outbox-record write is **not** governed by ADR-017's category-based
propagate-or-suppress policy. An outbox-insert failure always propagates (aborts the
transaction), for every entity, regardless of `EntityAuditConfig.Category`. Rationale:
ADR-017's suppress-for-non-Admin/Security policy exists because a missing audit record for
an ordinary data entity is an accepted, logged, non-fatal compliance gap — but a missing
outbox record for a mutation that was *supposed* to dispatch a workflow or fire a domain
event is not a compliance gap, it is the exact bug (P0-B) this ADR exists to close. The two
failure policies are deliberately different because the two things they guard are not
equivalent in severity. Audit and outbox both write via the same transaction-carrying
`ctx`; they are not the same call, and one's success does not depend on the other's.

## 7. Outbox Semantics

One table, `events_outbox`, replacing both `workflow_outbox` (ADR-007) and `event_outbox`
(ADR-008). This name and most of these columns are not new — they match the live,
currently-applied `events/outbox/relay.go` code exactly (its `poll()` SELECT and
`OutboxWriter.Publish`'s INSERT both already reference a table named `events_outbox` with
a column named `type`), and closely match the orphaned, unapplied schema already sitting
in `migrations/20260706000003_create_platform_support.up.sql` (see §1, §22). This ADR's
schema is a superset of both — it adds columns neither has, but changes no existing column
name the live code depends on:

```sql
CREATE TABLE events_outbox (
    id              uuid PRIMARY KEY,          -- events.DomainEvent.ID (UUIDv7)
    tenant_id       uuid NOT NULL,             -- uuid.Nil permitted for platform-level events
    type            text NOT NULL,             -- events.DomainEvent.Type; column name matches the live relay.go code exactly — do not rename to event_type
    entity_name     text NOT NULL,
    record_id       uuid NOT NULL,
    actor_id        uuid,                      -- def.Actor.UserID/ServiceAccountID; NULL for system actors
    system_actor    text,                      -- ADR-015 audit.SystemActor's string form, e.g. "system:outbox-relay"; NULL for human/service-account actors. See the note below on representation and dependency direction.
    action_name     text,
    correlation_id  text,                      -- opaque identifier, NOT a uuid column — see note below
    payload         jsonb,
    occurred_at     timestamptz NOT NULL,
    delivered_at    timestamptz,               -- NULL = pending
    attempts        int NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    last_error      text,
    dead_at         timestamptz                -- NULL until moved to terminal failure (§9.4, §16)
);
CREATE INDEX events_outbox_pending ON events_outbox (next_attempt_at)
    WHERE delivered_at IS NULL AND dead_at IS NULL;
```

`id`, `tenant_id`, `type`, `entity_name`, `record_id`, `actor_id`, `payload`, `occurred_at`
map directly to the existing `events.DomainEvent` Go struct (`events/events.go:35-63`) — no
change to that type's already-shipped fields or their names. **The column is named `type`,
matching `events.DomainEvent.Type` and the live `relay.go` SQL exactly — this ADR does not
rename it to `event_type`, and no change to `relay.go`'s existing SELECT/INSERT column
lists for this column is required.** `system_actor` and `correlation_id` are additions this
ADR requires on both the table and `events.DomainEvent` (a small, additive struct change,
not a breaking one). `next_attempt_at`/`dead_at` are new columns supporting §9.4/§16's
retry/dead-letter policy, which the real relay code does not implement today (flat
`attempts < 5` cutoff, no backoff, no terminal state).

**`correlation_id` is `text`, not `uuid`.** `audit.RequestContext.RequestID`
(`audit/request_context.go:15`), the field this column is populated from when present, is
itself a plain Go `string` holding an arbitrary client-or-middleware-supplied
`X-Request-ID` header value — it is never validated or guaranteed to be UUID-shaped.
Correlation, causation, and request identifiers throughout this ADR are treated as **opaque
identifiers for tracing, not as typed UUIDs** — a `uuid` column would reject a legitimate,
non-UUID `X-Request-ID` value outright. This ADR does not introduce a causation-ID column
(§8 explains why); if one is added in a future ADR, it should follow the same `text`
convention for the same reason.

**`system_actor` is a string designation, not an identifier, and this is the one
authoritative representation used throughout this document.** ADR-015's `audit.SystemActor`
(`audit/record.go:63`) is a `string`-based type whose values look like `"system:outbox-relay"`
— a category label, not a UUID identity, and this ADR does not pretend otherwise anywhere.
(An earlier draft of this ADR inconsistently referred to this field as "`SystemActorID`" in
its Consequences section — that name implied a UUID identifier and has been corrected; the
one true name, used everywhere in this document, is `system_actor`, matching the SQL column
above.) **Dependency-direction note:** `events/events.go`'s own package doc comment states
"Dependency: events → def only." This ADR does **not** introduce an `events → audit`
dependency to obtain this type — `events.DomainEvent`'s new field is typed as a plain `string`
(mirroring how `events.EventType` itself is already a locally-defined `string` type, not an
import from elsewhere), populated at the call site from `string(someSystemActorValue)`
where the caller already has an `audit.SystemActor` value in hand. `events` itself never
imports `audit`. This preserves the package's stated dependency boundary exactly; no
exception is needed or granted.

**Exactly one workflow-relevant addition to `EventType`:** `WorkflowTriggerFired`
(alongside the existing `EventCreated`/`EventUpdated`/`EventDeleted`/`EventActionFired`,
`events/events.go:22-30`), whose `payload` carries the JSON-marshaled
`def.ActionWorkflowSpec` (`WorkflowFn`, `TaskQueue`, `WorkflowID`, `Input` — already an
existing type, `def/action_runtime.go`). No separate workflow-specific table, columns, or
`EventBroker` design is introduced — this is the mechanism by which §3's "no direct
Temporal calls" requirement is satisfied structurally.

**Payload content restriction (carried forward from ADR-008, not weakened by this ADR):**
`payload` MUST NOT contain fields marked `Sensitive: true` on their `FieldDef`, full
unrestricted record snapshots, secrets, credentials, or access/refresh tokens. Because this
ADR makes an outbox write part of every qualifying mutation (§3, §5) rather than an
opt-in, rarely-exercised path, this restriction is more load-bearing than it was under
ADR-008 and must be enforced mechanically, not left to each call site's discretion: any
payload built from entity field data (`record.Data`/`record.CustomFields`) MUST be passed
through the existing `audit.Sanitizer.Strip(entityName, snapshot)` (`audit/sanitizer.go:55`
— already used on the audit-record path for exactly this purpose) before being marshaled
into `payload`. This ADR does not introduce a second, independent sanitization
implementation — reusing `audit.Sanitizer` is required, not merely suggested. (This does
create a call-time dependency from whatever constructs the outbox payload on `audit`'s
`Sanitizer` type — an application-level dependency at the call site, not a new import edge
on `events.DomainEvent` or `events.Publisher` themselves, which stay exactly as
dependency-free as before.)

## 8. Event Schema

Covered in §7's table and the `events.DomainEvent` additions. Restated per the challenge's
explicit field list: event ID (`id`, UUIDv7 — exists), event type (`type` — exists,
column name unchanged from the live code, see §7), aggregate/entity identity
(`entity_name`+`record_id` — exists), tenant ID (`tenant_id` — exists), org ID
(**deliberately not added** — see §17/§21: `ScopeOrganization` is unresolved and broken;
adding `org_id` now would be premature infrastructure for a scope that cannot yet safely
exist), actor identity (`actor_id`+`system_actor` — the former exists, the latter is this
ADR's addition, both together implementing ADR-015's actor model faithfully for
background-originated events; `system_actor` is a string category label like
`"system:outbox-relay"`, never a UUID identity — see §7), correlation ID (`correlation_id`,
`text` — this ADR's addition; an opaque tracing identifier, not a validated UUID, since its
source, `audit.RequestContext.RequestID`, is an arbitrary client-supplied string — see §7),
causation ID (**deliberately not added** — no event chain in the current architecture is
deep enough to need one; adding it speculatively would be exactly the unjustified
abstraction this framework's own design principles warn against; if a future ADR adds one,
it should be `text` for the same reason `correlation_id` is), payload (`payload` — exists,
subject to the mandatory sanitization requirement in §7), created timestamp (`occurred_at`
— exists).

## 9. Relay Semantics

The existing `Relay`/`OutboxWriter` design (`events/outbox/relay.go`) is retained with
five required corrections, not replaced:

1. **`OutboxWriter.Publish` must join the caller's transaction** — use
   `tx.QuerierFromContext(ctx)` (`tx/tx.go`), not `tx.ConnFromContext`. `tx.ConnFromContext`
   returns a bare `tx.Conn` (only `InTx() bool` — it cannot execute SQL); `tx.QuerierFromContext`
   returns a `tx.Querier`, which has the `ExecSQL(ctx, sql, args...) (int64, error)` method
   `Publish` actually needs. This is not a new pattern invented for this ADR — it is the
   exact mechanism `awo/audit` already uses to write audit records into the caller's
   transaction without importing `pgx` at all: `audit/pg_writer.go:94` and
   `audit/transactional_writer.go:234` both call `tx.QuerierFromContext(ctx)` and then
   `q.ExecSQL(...)`. (`contrib/pgx`'s own `setTenantContext`, cited by an earlier draft of
   this section, is not an analogous example — it lives *inside* the driver and receives an
   already-resolved `execer` as a direct function parameter; it never needs to extract
   anything from `ctx`, because it *is* the code that put the connection there in the first
   place.) `OutboxWriter.Publish` should follow the `audit` package's pattern exactly:
   resolve `q, ok := tx.QuerierFromContext(ctx)`, return an error if `ok` is false (per
   `events.Publisher`'s own documented contract — no active transaction is a caller error,
   not a silent success), and call `q.ExecSQL(ctx, insertSQL, args...)` for the INSERT.
   **`OutboxWriter`'s `pool *pgxpool.Pool` field is no longer needed by `Publish` once this
   lands** — it remains necessary for `Relay.poll()` (an unrelated, non-transactional,
   background read path that has no caller transaction to join) and for `NewWriter`'s
   constructor only if `OutboxWriter` and `Relay` continue to share one constructor
   argument for convenience; this is an implementation detail, not an architectural one.
   This is the single fix that makes §5/§6's atomicity claims true; without it, nothing
   else in this ADR is actually durable. The required data flow, restated as the
   invariant it is:

   ```
   mutation transaction (repo.WithTx)
       ↓
   audit write        (pipeline.RunAuditRecord, via tx.QuerierFromContext — unchanged)
       ↓
   outbox write        (publisher.Publish, via tx.QuerierFromContext — this ADR's fix)
       ↓
   COMMIT
       ↓
   relay (separate process/goroutine, separate connection, no relationship to the
          mutation's own transaction beyond reading its already-committed result)
   ```

   No step in this chain may open an independent connection for the mutation's own outbox
   write — `Publish`, when called from inside a `repo.WithTx` callback, must always resolve
   and use the same `tx.Querier` the audit write already used, never a fresh one from the
   pool.
2. **Row claiming stays as-is**: the session-level advisory lock
   (`pg_try_advisory_lock`, held for one `poll()` cycle's full duration) plus
   `SELECT ... FOR UPDATE SKIP LOCKED` correctly serializes concurrent relay instances
   cluster-wide — confirmed by direct trace, not changed by this ADR. (Note for
   implementers, not a required change: the row-level `FOR UPDATE` lock itself is released
   the instant its own implicit auto-commit statement completes, before `deliver()` ever
   runs — it is the session-level advisory lock that actually does the serializing work
   here; this is correct as-is, `SKIP LOCKED` is merely decorative given the advisory lock,
   not a bug.)
3. **Per-event dispatch timeout, required, new:** `deliver()`'s loop must wrap each
   `Subscriber.HandleEvent`/`WorkflowExecutor.Start` call in a bounded
   `context.WithTimeout`. Today, the entire poll cycle — advisory lock, connection, and up
   to 50 events' worth of external calls — runs serially with no per-call bound; adding
   Temporal dispatch (a network call to an external system with a different failure profile
   than an in-process `Subscriber`) to this same loop without a timeout risks one slow or
   hung Temporal call blocking delivery of every other pending event, including unrelated
   domain events, for an unbounded duration.
4. **Retry/backoff/dead-letter, required, new:** replace the flat `attempts < 5` cutoff
   with `next_attempt_at`-scheduled exponential backoff (the specific schedule either
   frozen doc proposed — 1s/2s/4s/8s/16s, capped 5 minutes — is reasonable and not
   contested by anything found in source; this ADR adopts it) and a genuine terminal state
   (`dead_at` set, row excluded from the pending index) once backoff is exhausted, closing
   the gap between the package's own doc comment (claims a "dead-letter table" exists) and
   its actual behavior (a poison row simply stops being retried, silently, forever, with no
   operator-visible signal).
5. **Per-entity event ordering is NOT guaranteed, and this ADR does not attempt to provide
   it.** `poll()`'s `ORDER BY occurred_at ASC` combined with `SKIP LOCKED` orders each
   individual batch's *claiming* by timestamp, but does not prevent two events for the
   *same* entity from landing in different poll batches (e.g., a fast-following second
   mutation's event is written and claimable before the first mutation's event has finished
   `deliver()`-ing, if the first is retried/slow), nor does it prevent concurrent relay
   instances (multiple processes, each serialized internally by its own advisory lock, but
   not serialized against each other beyond that) from processing different entities'
   events out of causal order relative to each other. Nothing in the current entity model
   requires strict per-record event ordering, so this ADR accepts it as a stated limitation
   rather than adding sequencing machinery (a per-entity ordering key, a single-writer
   partition, etc.) to solve a problem no consumer has today. A future `Subscriber` that
   genuinely requires ordered delivery for one entity type must implement its own
   sequencing (e.g., reading `occurred_at` and re-ordering within its own buffer) — the
   relay does not provide FIFO semantics, globally or per-entity.

## 10. Workflow Intent vs. Execution

**Workflow intent** is a durably committed `events_outbox` row of type
`WorkflowTriggerFired` — created by the mutation's own transaction, per §3/§5. **Workflow
dispatch** is the relay's `WorkflowTriggerFired`-subscribed handler calling
`workflow.WorkflowExecutor.Start` — happening strictly after commit, retried independently
of the mutation that created the intent. **Workflow execution** is Temporal actually
running the workflow function — which requires a running worker polling the relevant task
queue. **No worker exists today** (confirmed: `workflow.NewWorker` has zero production
callers in either binary) and building one is explicitly out of this ADR's and Phase 2's
scope (§19). This ADR's guarantee stops at dispatch: a committed mutation's workflow intent
will be durably retried until Temporal acknowledges receiving the start request, or until
the event is dead-lettered (§9.4) after exhausting retries. Whether Temporal then executes
it depends entirely on a worker existing, which is a later phase's concern.

Both current direct-to-Temporal call sites must be rewritten to produce intent instead of
attempting execution: `EntityService.startWorkflows` (`api/service/entity.go`) and
`ActionContext.StartWorkflow` (`runtime/runtime_action_context.go:187-202` — found during
the architecture review to have the identical bug as `startWorkflows`, despite living
inside the "canonical" action-runtime abstraction: it calls `a.executor.Start` immediately,
synchronously, regardless of whether it's invoked inside or outside a `Tx` callback, with
no outbox indirection at all).

**Workflow event identity:** the `events_outbox` row's own `id`. **Workflow ID**: unchanged
— `workflow/id.go`'s `BuildID` convention (`{tenantID}.{entityType}.{recordID}.{event}`),
carried in the event payload's `def.ActionWorkflowSpec.WorkflowID`. **Task queue, input
payload:** unchanged, carried the same way. **Tenant context:** the event row's own
`tenant_id`, restored by the relay before dispatch (§12). **Retry responsibility:** the
relay (§9.4), not Temporal and not the original mutation. **Duplicate handling:** Temporal's
own `WorkflowID`-based deduplication (§11) — a second `Start` call with the same
`WorkflowID` against an already-running or completed execution does not create a duplicate.

## 10a. `ActionContext.Publish`'s Payload Serialization Bug (repair required, not fixed by this document)

Found during this ADR's own review pass, independent of the direct-Temporal-call bug §10
already covers: `ActionContext.Publish` (`runtime/runtime_action_context.go:170-183`) — the
very method this ADR requires be adopted as canonical — currently builds its
`events.DomainEvent.Payload` as:

```go
Payload: []byte(fmt.Sprintf("%v", event.Payload)),
```

`events.DomainEvent.Payload`'s own doc comment (`events/events.go:57-59`) states the
required format explicitly: "Format: JSON-encoded map[string]any." `fmt.Sprintf("%v", ...)`
does not produce JSON — for a `map[string]any` it produces Go's default verbose
representation (e.g. `map[foo:bar]`), which is not valid JSON and will not round-trip
through any JSON-consuming `Subscriber` or downstream tooling. This is a real, silent
data-corruption bug in the code this ADR is asking implementers to promote to the
canonical path, not a hypothetical one.

**Required as part of this ADR's implementation:** `ActionContext.Publish` must serialize
`event.Payload` with `json.Marshal` (or an equivalent canonical JSON encoder), consistent
with how `OutboxWriter.Publish` already correctly marshals its own payload
(`events/outbox/relay.go:208`, `json.Marshal(e.Payload)`) — `ActionContext.Publish` should
match that existing, correct sibling implementation rather than inventing a different
serialization approach. Per §7, this payload is also subject to the mandatory
`audit.Sanitizer.Strip` pass before marshaling if it is built from entity field data. This
ADR does not perform this fix (no Go source is modified by this document); it is recorded
here as a required Phase 2 implementation item with its own acceptance criterion (§23).

## 11. At-Least-Once Semantics

**Stated explicitly, as the contract requires (§4.F):** dispatch is at-least-once.
Duplicate dispatch is possible and must be tolerated, not prevented. The precise crash
window that produces a duplicate: `deliver()` succeeds (the external call — a `Subscriber`
or, once §10 lands, `WorkflowExecutor.Start` — completes) but the relay process crashes
before the subsequent `UPDATE ... SET delivered_at = NOW()` commits; on restart, the row is
still `delivered_at IS NULL` and will be redelivered.

**For workflow-start dispatch specifically**, this is safe without any new mechanism:
Temporal's `WorkflowID`-based start deduplication (already relied upon by ADR-007's own
stated consequence: "Temporal start is idempotent via WorkflowID deduplication. If the
outbox worker retries a dispatch, Temporal returns the existing workflow run") absorbs the
duplicate `Start` call — this ADR does not weaken or change that reliance, it inherits it.
Implementers should set an explicit `WorkflowIDReusePolicy` on the Temporal SDK options
used by `workflow.TemporalExecutor` rather than relying on the SDK's implicit default,
documenting the choice at implementation time — this ADR does not mandate a specific policy
value, only that the choice be explicit and recorded.

**For generic domain events**, no framework-level deduplication is provided. Per §4.G,
every `Subscriber` implementation must be idempotent on its own terms (matching
`HOOK_CONTRACT.md`'s existing normative rule for `after_save` hooks, extended here to
`Subscriber`s by the same reasoning). This ADR does not introduce a generic consumer-side
dedup cache — no current `Subscriber` implementation exists to justify one, and adding it
speculatively would be scope creep beyond either P0.

**This ADR does not claim, and implementers must not claim in documentation, tests, or
code comments:** exactly-once delivery, exactly-once workflow execution, or exactly-once
dispatch (dispatch is at-least-once and *safe to retry*, which is a different, weaker, and
achievable claim).

## 12. Idempotency Requirements

Full API-level idempotency (`X-Idempotency-Key`) is governed entirely by the existing,
separate, Frozen ADR-009 and is unaffected by this ADR — this ADR's mutation-pipeline
changes do not touch that mechanism, and ADR-009's Redis-based cache-and-replay design is
not superseded, extended, or duplicated here. Within this ADR's own scope: event identity
(`events_outbox.id`, UUIDv7, already unique by primary key), workflow identity
(`WorkflowID`, already enforced by Temporal per §11), and relay-attempt identity (the row's
own `id` + `attempts` counter, no new identity needed) are the only identities this ADR's
mechanism requires, and all three already have a natural uniqueness source without a new
PostgreSQL constraint beyond the table's own primary key.

## 13. Tenant/Context Propagation

**Must be durably stored on the outbox row** (§7/§8): `tenant_id`, `actor_id`/
`system_actor` (ADR-015), `correlation_id`. **Must be reconstructed at dispatch time, never
persisted:** the acting principal's permissions/roles. A relayed event may be delivered
minutes, hours, or (per §9.4's backoff schedule) up to the dead-letter threshold after the
causing mutation — a role revoked in the interim must take effect, so any `Subscriber` or
workflow-dispatch handler performing an authorization-sensitive action must re-resolve
authorization fresh from `actor_id`, never reuse a cached permission snapshot.

**The relay must restore `tenant.TenantContext` from the event row's own `tenant_id`**
before invoking any `Subscriber` or `WorkflowExecutor.Start` — closing the gap `tasks.md`
1.4 already identified in `Relay.deliver`, which today passes the relay loop's own
(system) context through unchanged. **A workflow dispatched from an event carrying
Tenant A's `tenant_id` must never execute with any other tenant's context** — this is
enforced structurally by always deriving the dispatch-time `tenant.TenantContext` from the
row being processed in that iteration of the delivery loop, never from any ambient or
cached value the relay goroutine might otherwise carry across iterations.

**Not included, and why:** service-account ID as a field distinct from `actor_id` (no
current use case distinguishes it — `def.Actor` already unifies human/service-account
identity per ADR-015, and `events.DomainEvent.ActorID` already follows that model);
locale/timezone (no current `Subscriber` or workflow trigger is locale-sensitive; the one
plausible candidate, `platform/notifications`, resolves locale from the recipient's own
user record at delivery time, not from the triggering event); request ID as a field
distinct from `correlation_id` (treated as the same concept for this ADR's purposes —
`audit.RequestContext.RequestID`, when present, populates `correlation_id`); `org_id` (see
§8 — deferred until the `ScopeOrganization` blocker is resolved, §21).

## 14. Bulk Import Contract

**Architectural contract, not implementation.** Bulk import must preserve every invariant
§4.A and §4.B require for a single-record mutation — authentication, tenant/org isolation,
authorization, validation, naming-series allocation, immutable/protected-field rules,
hooks, audit, and outbox/workflow semantics — **per batch-flush transaction, not
necessarily per HTTP request and not necessarily as one transaction for the whole import.**
This ADR does not require literally invoking the HTTP CRUD handler for every row, and does
not require a single all-or-nothing transaction for an arbitrarily large import (the
existing per-`BatchSize`-flush transaction granularity, default 100 rows,
`ioport/importer.go`, is retained).

**Guaranteed, per flush:** all rows in a flush are validated (via `runtime.Pipeline`'s
existing `RunBeforeCreate`/hook stages) before that flush's batch insert executes;
validation failures are recorded in `ImportResult.Errors` and either abort the whole import
(`SkipErrors: false`) or are excluded from that flush's insert while the rest of the flush
proceeds (`SkipErrors: true`) — this ADR does not change which of these two is the default,
only that the check now actually happens. **Guaranteed, per successfully-inserted row:**
exactly one audit record and exactly one outbox event, written inside that row's flush
transaction, identical in shape to what a single `POST` would have produced — a downstream
`Subscriber` must not be able to distinguish an imported record from an API-created one.
**Not guaranteed:** atomicity across flushes (a 10,000-row import that fails on flush 47
leaves flushes 1-46 committed) — this is the existing, retained behavior, not a regression;
whole-import retry/resumability is explicitly deferred (§19) as a self-contained, later-
phase feature, not a Phase 2 requirement. **Not guaranteed:** duplicate-import detection —
re-running the same import file twice produces two sets of records unless the import's own
data contains a natural uniqueness constraint the underlying table enforces; this ADR does
not add import-level deduplication.

## 15. Hook Semantics

Unchanged: nine hook types (`BeforeValidate`, `BeforeCreate`, `AfterCreate`, `BeforeUpdate`,
`AfterUpdate`, `BeforeDelete`, `AfterDelete`, `BeforeSave`, `AfterSave` — `def/hook.go`),
all invoked by `runtime/pipeline.go`, all panic-safe via `safeCall`'s `recover()`
(converting a panic to `*HookPanicError`, never crashing the process or corrupting
transaction state). `docs/02-pipeline/HOOK_CONTRACT.md` under-documents this (shows six of
nine hook types, and a one-argument `BeforeUpdate`/`AfterUpdate` signature where the real
interfaces take two, `prev *EntityRecord` included) — flagged as a doc-correction task
(§20), not an architectural change this ADR makes.

**No `AfterCommit` hook type is introduced.** The two needs that might suggest one —
publish an event, start a workflow — are both served by calling the outbox `Publisher`
from inside the existing `AfterCreate`/`AfterSave` stage (still pre-commit, same
transaction, per §5), not from a hypothetical post-commit stage. A genuine post-commit hook
would need its own delivery guarantee against the exact crash it's supposed to survive —
which is precisely the problem the outbox already solves. Introducing a separate
`AfterCommit` hook mechanism would create a second, competing "guaranteed after commit"
channel alongside the outbox, contradicting §4.E's single-handoff-boundary requirement.

**Hook recursion:** governed unchanged by the existing, Frozen ADR-010 — same-entity hook
recursion is a runtime panic, not a supported pattern. Cross-entity recursive mutation from
a hook (Entity A's `AfterCreate` mutating Entity B) remains permitted, and — per direct
trace this pass — is not exercised by any hook implementation currently in the codebase; it
inherits transactional consistency automatically via `Repository.WithTx`'s existing
`existing.InTx()` nested-call detection, unchanged by this ADR.

## 16. Failure Semantics

Every scenario the review required, in one matrix. "Where" identifies which component
observes the failure first; "TX state" describes the mutation transaction's fate;
"Duplicate risk" states plainly whether at-least-once redelivery/redispatch is possible for
that scenario, per §11 — never claiming exactly-once anywhere in this table.

| # | Scenario | Where it occurs | Transaction state | Retry behavior | Final durable state | Duplicate dispatch possible? | Security implication |
|---|---|---|---|---|---|---|---|
| 1 | Validation fails (`BeforeValidate`/field validation/`BeforeCreate`/`BeforeUpdate`) | Pipeline, **before** any transaction opens (ADR-013, unchanged) | No transaction exists yet — nothing to roll back | Not retried automatically; the caller (HTTP handler, import row) receives the error and decides whether to resubmit | No mutation, no audit record, no outbox row. Nothing persisted. | No — nothing was dispatched | None — this is the existing, unmodified pre-transaction validation gate |
| 2 | Mutation (PERSIST) fails | Inside `repo.WithTx`, at the `repo.Create`/`Update`/`Delete` call | Rolls back (unchanged) | Caller-level only (same as row 1) | No audit record, no outbox row | No | None |
| 3 | Audit write fails, category = Admin/Security | Inside `repo.WithTx`, at `RunAuditRecord` | Rolls back (ADR-017, unchanged) | Caller-level only | Mutation and outbox row both undone with the rollback | No | None — ADR-017's existing propagate policy, unchanged |
| 4 | Audit write fails, other category | Inside `repo.WithTx`, at `RunAuditRecord` | Does **not** roll back (ADR-017, unchanged) | N/A — logged at WARN, not retried | Mutation and outbox row both commit; no audit record | No (this row is about audit, not dispatch) | Accepted, pre-existing compliance trade-off (ADR-017) — unrelated to and unweakened by this ADR |
| 5 | Outbox insertion fails, any audit category | Inside `repo.WithTx`, at `publisher.Publish` | **Rolls back, unconditionally** (§6 — this ADR's addition; not subject to row 4's leniency) | Caller-level only | Mutation and audit record both undone with the rollback | No | This ADR's core new invariant: a mutation whose downstream dispatch could not be made durable must not appear to have succeeded |
| 6 | DB transaction rolls back (any cause, generally) | `repo.WithTx`'s deferred rollback (Phase 1 panic-safety fix, unchanged) | Rolled back by construction | Caller-level only | Nothing persisted — rows 2, 3, and 5 above are all specific instances of this general case | No | Phase 1's panic-safety guarantee (connection always released) is unaffected by anything in this ADR |
| 7 | Process crashes before commit | Anywhere before `COMMIT` returns | Nothing persisted (PostgreSQL WAL guarantee) | N/A | Nothing persisted | No | None |
| 8 | Process crashes after commit | After `COMMIT` returns, before the relay's next poll | N/A — already committed | Relay's next poll cycle (≤ `pollInterval`, currently 500ms) picks up the row | Entity, audit record, and outbox row all durably committed together; only *delivery* is pending | No (not yet dispatched at all) | None |
| 9 | Relay crashes before dispatch | `Relay.poll()`, before `deliver()` is called for a claimed row | N/A (mutation already committed, unrelated to this crash) | Row remains `delivered_at IS NULL`; any relay instance's next poll (via the advisory lock) picks it up | Unchanged — no data lost | No (dispatch never happened) | None |
| 10 | Relay crashes after dispatch, before marking delivered | Between `deliver()` returning success and the `UPDATE ... SET delivered_at` committing | N/A | Row still `delivered_at IS NULL`; redelivered on next poll | Eventually `delivered_at` set once a delivery attempt's `UPDATE` succeeds | **Yes** — this is the canonical at-least-once window (§11) | Safe for workflow starts (Temporal `WorkflowID` dedup, §11); requires `Subscriber`-side idempotence for generic domain events (§4.G) — a non-idempotent `Subscriber` could double-process here |
| 11 | External dispatch (`Subscriber.HandleEvent`/`WorkflowExecutor.Start`) times out | Inside the per-event `context.WithTimeout` (§9.3) | N/A | `attempts` increments; `next_attempt_at` scheduled per backoff (§9.4) | Row stays pending until backoff exhausts or a later attempt succeeds | Possible if the timed-out call actually reached the external system before timing out locally (e.g. Temporal received the start but the acknowledgment was lost) — this is exactly why workflow starts rely on `WorkflowID` dedup rather than assuming a timeout means "definitely didn't happen" | Same as row 10 |
| 12 | External dispatch returns an explicit error (includes "Temporal unavailable" as one instance of this row, not a separate case) | Same as row 11 | N/A | Same as row 11 | Same as row 11 | Depends on whether the external system's error means "definitely not received" (safe to retry with no duplicate risk) or "ambiguous" (treat as row 11) — this ADR does not require distinguishing the two for Phase 2; treating every error as "ambiguous, retry" is the conservative, safe default | Same as row 10 |
| 13 | Event exhausts the retry policy | `poll()`'s attempt-counting logic, after `attempts` crosses the configured cutoff | N/A | No further automatic retry | `dead_at` set; excluded from the pending index; operator-queryable, not silently dropped (§9.4) | No further dispatch attempts occur, so no further duplicate risk from this event — but if an *earlier* attempt in this event's own history actually succeeded externally before this event was (incorrectly) still marked pending, that earlier duplicate already happened per row 10/11's logic, independent of exhaustion | None new |
| 14 | Malformed/poison event (payload fails to unmarshal, or a `WorkflowTriggerFired` event's `ActionWorkflowSpec` is invalid) | Inside `Subscriber.HandleEvent`/the workflow-dispatch handler, on every attempt | N/A | Indistinguishable from an ordinary handler error at the relay's level — retried per rows 11/12 until it exhausts and reaches row 13 | Same terminal state as row 13 (`dead_at`) — Phase 2 does not add a separate "fail fast, skip backoff" path for malformed events; a poison event simply spends its full retry budget before terminating, which is accepted as a bounded, known cost (at most the backoff schedule's total duration, capped by MaxAttempts) rather than an unbounded one | No | A malformed event that somehow encodes attacker-controlled data is still subject to whatever the `Subscriber`/dispatch handler does with it — this ADR does not weaken any existing input-validation boundary; the event payload itself is application data, not a new trust boundary crossing |
| 15 | **Tenant context cannot be reconstructed at dispatch time** — the relay's `tenant.WithContext(ctx, tenant.TenantContext{TenantID: e.TenantID})` restoration (§13) is followed by a `Subscriber`/`WorkflowExecutor.Start` call that fails because `set_tenant_context()` rejects the tenant | The `Subscriber`/dispatch handler's own repository call, once it attempts to use the restored tenant context | N/A (relay has no transaction of its own around dispatch) | **Split by cause, not treated as one generic retry case (see discussion below the table)** | See below | See below | See below |

**Row 15 requires a deterministic policy, not "just retry," because the underlying causes
have materially different correct outcomes:**

- **Transient infrastructure failure** (the database is briefly unreachable, a connection
  pool is exhausted) — indistinguishable, from the relay's point of view, from any other
  transient error. **Policy: treat as row 11/12 — retry with backoff.** This is the
  scenario retry-with-backoff already correctly handles; nothing tenant-specific about it.
- **The tenant exists but is permanently not `ACTIVE`** (`set_tenant_context()` raises
  `P0002`, Phase 1's hardened tenant-lifecycle check — the tenant was suspended or archived
  sometime between the mutation committing and this delivery attempt, which is a real
  possibility given the retry window can span the full backoff schedule). Retrying this
  with the same backoff schedule as a transient error is **wrong**: the condition will not
  spontaneously resolve itself the way a transient infrastructure blip does, and an
  indefinitely-retried, permanently-failing event either wastes retry budget for no benefit
  or (worse) sits pending forever if nothing ever exhausts it. **Policy: `P0002` from
  `set_tenant_context()` is NOT treated as an ordinary dispatch failure — it dead-letters
  the event immediately** (sets `dead_at`, skips the remaining backoff schedule), with
  `last_error` recording the rejection reason, so an operator can see exactly why and decide
  whether to manually re-queue once/if the tenant is reactivated.
- **The tenant does not exist at all** (`set_tenant_context()` raises `P0001` — should be
  rare, since the mutation that created the event necessarily had a valid tenant context at
  write time, but a tenant record could theoretically be hard-deleted afterward). **Policy:
  same as the non-ACTIVE case above — immediate dead-letter, not retried.** This is not a
  condition backoff can fix either.
- **A malformed/corrupt `tenant_id` value on the row itself** (should be structurally
  impossible — the column is `uuid NOT NULL` and is always populated from the mutation's own
  validated `tenant.TenantContext` at write time — but if it were ever observed, e.g. via
  direct database tampering or a future bug) is a **security/context violation, not a
  retryable failure**: **Policy: immediate dead-letter, and this specific condition should
  be logged at a level that pages an operator**, since it indicates either data corruption
  or an attempted bypass of the tenant boundary, not an ordinary operational hiccup.

**In every branch of row 15, the one invariant that must never be violated:** a dispatch
attempt must never proceed with a fallback, default, or "best guess" tenant context when
the real one cannot be established — every branch above ends in either "retry with the
correct tenant context re-derived fresh on the next attempt" or "dead-letter without ever
dispatching," never "dispatch anyway with degraded context." This is the concrete
implementation of §13's "a workflow dispatched from an event carrying Tenant A's `tenant_id`
must never execute with any other tenant's context" requirement for the one failure mode
where that requirement is actually at risk of being violated by an under-specified retry
loop.

## 17. Core/Adapters Boundary

**Core** (infrastructure-agnostic, already correctly placed, unchanged by this ADR):
`events.DomainEvent`/`Publisher`/`Subscriber`/`Bus` (`events/events.go` — pure `def`-level
semantics, no infrastructure type leaks into any interface signature, confirmed by direct
read); `workflow.WorkflowExecutor` (`NoopExecutor` proves the abstraction is genuinely
infrastructure-independent); `def.ActionRuntime` and its dependency-injection seams
(`runtime.EntityDriver`/`EventBus`/`WorkflowRuntime`/`NotificationService`/
`CacheInvalidator` in `action_runtime_impl.go`, or the more directly-wired equivalents in
`ActionContextConfig`) — every method signature on these is expressed in terms of `def`
types plus stdlib, meaning implementing packages never need to import `runtime` back
(verified directly this pass: no circular-dependency risk from wiring real adapters,
confirmed two independent ways — Go's structural interface satisfaction, and
`bootstrap`'s position at the top of the import graph with zero importers of its own
outside `cmd/*`).

**Adapter** (infrastructure-specific, correctly isolated behind the above interfaces):
`events/outbox` (the PostgreSQL-backed `Relay`/`OutboxWriter` — a naming/location
inconsistency worth noting opportunistically, not fixing now: it behaves exactly like a
`contrib/pgx`-style adapter but doesn't live under `contrib/`); `workflow.TemporalExecutor`
(correctly named/scoped already). **This ADR does not relocate `events/outbox` into
`contrib/pgx`** — that is a cosmetic, non-functional change explicitly deferred (§19), not
a boundary violation requiring correction now.

**Where an existing interface already leaks a PostgreSQL-specific type:** none found in the
interfaces this ADR touches (`events.Publisher`/`Subscriber`, `workflow.WorkflowExecutor`,
`def.ActionRuntime`) — all are already clean. `router.RegisterOptions.Temporal`
(`api/router/router.go`), typed as the raw `temporalclient.Client` rather than
`workflow.WorkflowExecutor`, is the one wiring-layer leak this ADR requires correcting
(§13/implementation), since it is the same call site the outbox-routing fix must already
touch.

## 18. Security Invariants

Every Phase 1 invariant remains unmodified and unaffected by this ADR's changes: tenant
lifecycle enforcement, RLS, tenant-context propagation and isolation, session revocation,
protected-field write rejection (`Repository.checkWritableField`), dynamic-SQL validation,
and `WithTx` panic-safety are all orthogonal to the outbox/action-runtime changes this ADR
specifies — none of the files this ADR touches (`events/outbox/relay.go`,
`api/service/entity.go`, `runtime/runtime_action_context.go`, `api/handler/crud.go`,
`router.go`) intersect the files Phase 1 hardened for those properties. The one place this
ADR's own design must itself preserve a security invariant: §13's tenant-context
restoration is not optional — a relay or workflow-dispatch path that failed to restore the
correct tenant context per event would be a new, Phase-2-introduced security regression,
not a pre-existing one; implementation must include the adversarial test named in
`PHASE2_ARCHITECTURE_PLAN_REVIEW.md` §20 (a dispatched event's handler must see the
event's own tenant, never the relay loop's ambient context).

## 19. Explicit Non-Goals

This ADR, and the Phase 2 work it authorizes, does **not** guarantee or deliver:

- Exactly-once event delivery or exactly-once workflow execution (§11 — at-least-once,
  explicitly).
- Temporal worker availability, or any real, deterministic workflow/activity function
  (§10 — no worker exists, none is built by this ADR).
- Full API-level idempotency (`X-Idempotency-Key`) — governed entirely by the separate,
  unaffected, Frozen ADR-009.
- `ScopeOrganization`/`ScopeOrganizationTree` RLS correctness — a separate, tracked,
  CRITICAL-severity blocker (`tasks.md` 1.18) that this ADR does not resolve and does not
  depend on (§8/§13's deliberate exclusion of `org_id` from the outbox schema).
- Search (does not exist, unrelated to this ADR).
- OpenAPI deduplication (`api/openapi` vs `generator/openapi`, unrelated to this ADR).
- Migration-engine completeness (schema-diff/rename safety, directory consolidation —
  unrelated).
- General scheduler completeness (`scheduler.Cancel()`'s existing bug — unrelated,
  zero production callers of the scheduler exist regardless).
- Notification delivery completeness (`platform/notifications`' possibly-dead
  `DispatchHook.AfterCreate`, flagged for separate confirmation — unrelated to this ADR's
  mechanism, since notification dispatch today is synchronous and in-transaction, not
  outbox-routed, and this ADR does not change that).
- Bulk-import resumability/retry-the-whole-job semantics (§14 — explicitly deferred).
- A generic, framework-wide idempotency/deduplication cache for arbitrary `Subscriber`s
  (§11/§12 — deliberately not built; each `Subscriber` owns its own idempotence).

## 20. Consequences

- `events.DomainEvent` gains two `string`-typed fields, `SystemActor` and `CorrelationID`
  (matching §7's `system_actor`/`correlation_id` columns exactly — not `SystemActorID`, and
  not `uuid`-typed) — additive, non-breaking for any existing (currently nonexistent, since
  no `Subscriber` is wired in production) consumer.
- `EntityService.startWorkflows` and `ActionContext.StartWorkflow` both change from
  direct-Temporal-call to outbox-publish — both call sites must be updated together (fixing
  one without the other leaves the other as a live P0-B regression).
- `ActionContext.Publish`'s payload serialization changes from `fmt.Sprintf("%v", ...)` to
  `json.Marshal` (§10a) — a bug fix bundled with this ADR's implementation, not a separate
  initiative, since it lives in the same method being wired for the first time.
- `runtime/action_runtime_impl.go`'s `RuntimeFactory`/`defaultActionRuntime` should be
  deleted once `ActionContext` is confirmed wired and working — leaving two unwired
  implementations of the same interface indefinitely is a maintenance hazard this ADR's
  implementation should close, not preserve.
- `OutboxWriter`'s constructor no longer needs to retain its `pool` field for `Publish`'s
  own sake once §9.1's fix lands (only `Relay.poll()` still needs pool access) — a minor,
  non-breaking internal simplification implementers may make opportunistically.
- `docs/02-pipeline/HOOK_CONTRACT.md` needs correction (all nine hook types, correct
  `BeforeUpdate`/`AfterUpdate` signatures) — a small, direct consequence of this work
  touching the pipeline, not a separate initiative.
- `router.RegisterOptions.Temporal` changes type from `temporalclient.Client` to
  `workflow.WorkflowExecutor` — a breaking change for any external caller of
  `router.Register` (none exist outside `cmd/server`/`cmd/awo`, confirmed).
- `platform/metadata`/`registry`/`settings`'s pre-existing lack of any transactional
  safety net becomes more visible by contrast once the canonical path gains one — this ADR
  does not fix them, but recommends they be logged as their own `tasks.md` item (§21).

## 21. Alternatives Considered

- **Keep two separate outbox tables, one per ADR-007/ADR-008 as originally specified.**
  Rejected: neither exists in code today, so there is no migration-compatibility cost to
  unifying them, and maintaining two parallel durable-delivery mechanisms for the same
  underlying "something must reliably happen after this mutation" problem directly
  contradicts this framework's own single-source-of-truth principle, doubling the surface
  area for the exact class of drift (`OutboxWriter.Publish`'s contract violation) that
  motivated this ADR in the first place.
- **Kafka/NATS/Redis-Pub/Sub `EventBroker` per ADR-008.** Rejected for Phase 2: no such
  infrastructure is deployed or referenced anywhere else in this codebase; the real,
  already-partially-correct `events.Subscriber` in-process model requires no new
  infrastructure dependency and directly serves both P0s. Not rejected permanently — an
  external broker adapter could be added later behind the same `events.Publisher`/`Bus`
  interfaces without another architectural change, since those interfaces are already
  infrastructure-agnostic (§17).
- **A dedicated `AfterCommit` hook type.** Rejected (§15) — creates a second competing
  durability channel alongside the outbox for no capability the outbox doesn't already
  provide.
- **Route bulk import through a literal per-row HTTP-handler-equivalent call
  (`EntityService.Create` verbatim, one call per row, one transaction per row).** Rejected
  (§14) — correct in spirit (same validation/hooks/audit/outbox per row) but abandons the
  existing, working per-flush batching for no atomicity gain (a 100-row single-transaction
  flush and 100 single-row transactions provide the same per-row guarantees; only the
  former preserves existing bulk-insert performance).
- **A repository-wrapper pattern for outbox writes** (a `driver.EntityRepository[T]`
  wrapper that calls the inner `Create`/`Update` then writes an outbox row afterward).
  Rejected for the exact reason ADR-014 already rejected this pattern for audit writing: a
  post-call write from a wrapper sits outside the transaction `repo.WithTx` manages,
  reproducing the same correctness failure ADR-013/ADR-014 already ruled out, applied here
  to a new mechanism instead of a new instance of the same mistake.

## 22. Migration Strategy

Not a data migration in the row-migration sense — no application has ever written a real
`events_outbox` row anywhere (`OutboxWriter.Publish` has no production callers today), so
there is no existing data to carry forward regardless of which schema file is treated as
authoritative.

**The orphaned migration must be explicitly reconciled, not silently ignored or silently
duplicated.** `migrations/20260706000003_create_platform_support.up.sql` already contains
an `events_outbox` table definition (§1) — unapplied (no `//go:embed`/
`migration.Register()` wires this directory into any live path, confirmed by exhaustive
search), but present in the repository and structurally similar to this ADR's schema
(same table name, same core columns, using `type` not `event_type` — one more independent
confirmation that `type` is the correct column name, §7). Implementation must choose
exactly one of the following, not leave both files coexisting silently:

- **(Preferred)** Treat the orphaned file as superseded historical scaffolding: leave it in
  place (per this ADR's general policy of not deleting superseded artifacts, §21), but add
  a comment at its top noting it is unapplied and superseded by whichever migration
  actually implements §7's schema, then write a **new**, properly `//go:embed`-wired
  migration (following the same pattern as `migration/bootstrap/migrations.go` or one of
  the `platform/*/migrations` packages) containing the full schema from §7, including the
  columns the orphaned file lacks (`system_actor`, `correlation_id`, `next_attempt_at`,
  `dead_at`) and the corrected index.
- **(Acceptable alternative)** If a future decision (outside this ADR's scope) resolves
  `tasks.md` 5.2's open question about the top-level `migrations/` directory's intended
  role by actually wiring it in, then this file may be edited in place to match §7's schema
  instead of writing a new file — but only as part of that broader decision, not as an
  incidental side effect of this ADR's implementation.
- **Not acceptable:** applying the orphaned file as-is (it lacks four required columns and
  uses the old flat retry cutoff) and separately also creating a new, differently-shaped
  `events_outbox` migration — two competing schema definitions for the same table name is
  exactly the class of drift this ADR exists to eliminate (§21's rejected alternative:
  "keep two separate outbox tables").

**The orphaned file's `attempts < 5` partial-index predicate must not be carried forward
unchanged.** That index (`events_outbox_pending_idx ON events_outbox (occurred_at ASC)
WHERE delivered_at IS NULL AND attempts < 5`) hardcodes the exact flat retry cutoff this
ADR replaces with `next_attempt_at`-scheduled backoff and an explicit `dead_at` terminal
state (§9.4, §16 row 13). Whichever migration path above is chosen, the live schema's
partial index must end up matching §7's `events_outbox_pending` definition
(`WHERE delivered_at IS NULL AND dead_at IS NULL`) — an index still predicated on
`attempts < 5` after this ADR's implementation would silently reintroduce the exact
poison-event gap §9.4 closes, by excluding "dead" rows from the pending index using the old
condition instead of the new one.

A schema migration (per the above) plus a behavioral cutover: `EntityService.startWorkflows`
and `ActionContext.StartWorkflow` both switch from direct Temporal calls to outbox-publish in
the same change (§20 — must not be split across separate releases, since a partial cutover
leaves one call site as a live regression). No backward-compatibility shim is needed for
either call site's external callers, since both are internal framework code with no
external contract change (the HTTP-facing behavior — a workflow eventually starts after a
qualifying mutation — is unchanged; only its durability improves). The two frozen ADRs this
supersedes (§21 above; ADR-007, ADR-008) are marked superseded in
`docs/00-overview/DECISION_REGISTER.md` when this ADR is formally appended there — not
performed by this document itself.

## 23. Acceptance Criteria

Grouped by the ADR section each criterion closes, so a later implementation session can
check each independently as PASS/FAIL against a concrete test or artifact.

**Transaction access and atomicity (§5, §6, §9.1):**
- [ ] `OutboxWriter.Publish` uses `tx.QuerierFromContext(ctx)` and `q.ExecSQL(...)` — not
      `pool.Acquire`, not `tx.ConnFromContext` — verified by reading the implementation, not
      only by test behavior.
- [ ] `OutboxWriter.Publish` returns an error (not a silent no-op) when `ctx` carries no
      active transaction, matching `events.Publisher`'s documented contract.
- [ ] A forced-rollback test proves mutation + audit record + outbox row commit or roll
      back together — no state where one of the three persists without the other two.

**Schema and naming consistency (§7):**
- [ ] `events_outbox` exists with exactly the columns in §7: `id`, `tenant_id`, `type`
      (not `event_type`), `entity_name`, `record_id`, `actor_id`, `system_actor` (`text`),
      `action_name`, `correlation_id` (`text`, not `uuid`), `payload`, `occurred_at`,
      `delivered_at`, `attempts`, `next_attempt_at`, `dead_at`, `last_error`.
- [ ] `events/outbox/relay.go`'s existing SELECT/INSERT column lists compile and run
      unmodified against the new schema for every column they already reference (`type`
      included) — confirming no accidental rename broke the live code.
- [ ] `events.DomainEvent` gains `SystemActor string` and `CorrelationID string` fields —
      not `SystemActorID`, not `uuid.UUID`-typed — and `events/events.go`'s package doc
      comment ("Dependency: events → def only") remains true after the change (`events`
      does not import `audit`).

**Orphaned migration reconciliation (§22):**
- [ ] `migrations/20260706000003_create_platform_support.up.sql`'s existing `events_outbox`
      definition has an explicit, recorded disposition (superseded-with-a-comment, or edited
      in place per an explicit broader decision) — not left silently coexisting with a
      second, differently-shaped definition.
- [ ] The live schema's pending-events partial index is predicated on
      `delivered_at IS NULL AND dead_at IS NULL` — the orphaned file's
      `attempts < 5` predicate does not survive into the applied schema.

**Payload security (§7, §10a):**
- [ ] Any outbox payload built from entity field data (`record.Data`/`CustomFields`) is
      passed through `audit.Sanitizer.Strip` before being marshaled — verified by a test
      that publishes a mutation on an entity with a `Sensitive: true` field and asserts the
      persisted `payload` column does not contain that field's value.
- [ ] `ActionContext.Publish` serializes `event.Payload` with `json.Marshal` (or an
      equivalent canonical encoder) — not `fmt.Sprintf("%v", ...)`.
- [ ] A round-trip test: publish an event through `ActionContext.Publish`, read the
      persisted `payload` column back, and `json.Unmarshal` it successfully into the
      original shape — proving the stored bytes are valid, parseable JSON, not Go's default
      verbose formatting.

**ActionContext adoption (§4.D, §10, §20):**
- [ ] `runtime.ActionContext.Publish` and (post-repair) `StartWorkflow` both route through
      the same fixed `Publisher` — `StartWorkflow` no longer calls `a.executor.Start`
      directly from any transaction-bound path.
- [ ] `EntityHandler.Action` constructs and populates `ActionContext.Runtime` for every
      real invocation — `def.ActionContext.Runtime` is no longer always nil.
- [ ] `runtime/action_runtime_impl.go`'s `RuntimeFactory`/`defaultActionRuntime` is removed
      once `ActionContext` is confirmed as the sole implementation — no two competing
      `def.ActionRuntime` implementations remain in the repository.

**Bulk import (§14, closes P0-A):**
- [ ] Bulk import writes one outbox event and one audit record per successfully-inserted
      row, batched per existing flush granularity; required/immutable-field violations are
      rejected per §14's `SkipErrors` semantics, not silently persisted.

**Workflow dispatch durability (§10, §11, closes P0-B):**
- [ ] A `WorkflowTriggerFired` event, dispatched by the relay through
      `workflow.WorkflowExecutor.Start`, survives a simulated crash between commit and
      dispatch, eventually reaching Temporal, with no duplicate *execution* on retry
      (duplicate *dispatch attempts* are expected and tolerated per §11 — the test asserts
      Temporal's own dedup absorbs them, not that the relay never retries).
- [ ] `router.RegisterOptions.Temporal` is `workflow.WorkflowExecutor`, not a raw SDK
      client; no code path outside `events/outbox`'s relay subscriber calls
      `WorkflowExecutor.Start`/the raw Temporal client directly.
- [ ] An explicit test or code comment records that `WorkflowIDReusePolicy` was a deliberate
      choice on `workflow.TemporalExecutor`'s SDK options, not an unexamined default (§11).

**Relay behavior (§9):**
- [ ] `deliver()`'s dispatch loop enforces a per-event timeout; a hung `Subscriber`/executor
      call does not block delivery of other pending events in the same batch (test: one
      slow/hanging subscriber, one normal subscriber, assert the normal one is still
      processed within the expected window).
- [ ] A dead-lettered event (`dead_at` set) is operator-queryable, not silently invisible.
- [ ] A test explicitly documents (via a passing assertion, not just a comment) that two
      events for the same entity, delivered in separate poll batches, are not guaranteed to
      be processed in `occurred_at` order — confirming §9.5's stated limitation is real
      behavior, not just a stated intention.

**Tenant-context failure handling (§13, §16 row 15):**
- [ ] A test simulating dispatch against a SUSPENDED or ARCHIVED tenant's event confirms
      the event is dead-lettered immediately (no backoff retries consumed), with
      `last_error` recording the rejection.
- [ ] A test simulating dispatch against a nonexistent tenant confirms the same immediate
      dead-letter behavior.
- [ ] A test confirms an ordinary transient failure during tenant-context restoration
      (e.g. a simulated connection error) is retried with backoff, not dead-lettered
      immediately — proving the two branches of row 15 are actually distinguished in code,
      not collapsed into one generic handler.
- [ ] A test proves a dispatched event's `Subscriber`/workflow handler observes only the
      event's own `tenant_id` — never the relay loop's ambient or a previous iteration's
      leftover context (the adversarial test §18 already requires in prose, now tracked
      here as a checkable item).

**Documentation and governance:**
- [ ] `docs/02-pipeline/HOOK_CONTRACT.md` corrected to list all nine real hook types with
      correct signatures.
- [ ] This ADR is appended to `docs/00-overview/DECISION_REGISTER.md` with its final,
      reconciled number (§ "A note on this ADR's number"), and ADR-007/ADR-008 marked
      superseded there, not deleted.

**Regression:**
- [ ] Full Phase 1 regression suite (tenant/RLS/session/connection-pool/dynamic-SQL tests)
      remains green throughout implementation.
- [ ] `gofmt`/`go vet`/`go build`/`go test ./...` clean at every implementation step,
      matching the verification discipline established in Phase 1.
