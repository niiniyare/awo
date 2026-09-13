> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Outbox Pattern"
id: wf-004
status: accepted
category: SPEC
stability: FROZEN
audience: [framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[Temporal Integration](temporal-integration.md)"
  - "[Activities](activities.md)"
  - "[Architecture Laws](../02-architecture/laws.md)"
  - "[EntityDefinition](../03-kernel/entity-def.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Outbox Pattern

**WF-004 | Status: Accepted | Stability: Frozen**

This document specifies the transactional outbox implementation: schema, relay behavior, at-least-once delivery semantics, failure handling, and the framework-private boundary. This specification is FROZEN — changes require a new ADR and version bump.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. Purpose

The outbox pattern solves a fundamental distributed systems problem: ensuring a database write and a subsequent external call (starting a Temporal workflow) either both happen or neither happens, without distributed transactions.

Naive approach — write to DB, then call Temporal — fails if the process crashes between the two operations, leaving the entity record committed but no workflow started.

The outbox pattern:
1. Write entity record + outbox entry in the **same PostgreSQL transaction**
2. A relay process reads unprocessed outbox entries and calls Temporal
3. On success, mark entry as processed
4. On failure, retry the relay

This guarantees at-least-once workflow start: the workflow may be started more than once on Temporal failure, but never zero times if the entity record was committed.

**See [LAW-006](../02-architecture/laws.md#law-006) and [LAW-018](../02-architecture/laws.md#law-018) for the architectural mandates governing this pattern.**

---

## 2. Outbox Table Schema

The outbox table is framework-private — module authors MUST NOT read from or write to it directly (LAW-018).

```sql
CREATE TABLE workflow_outbox (
    id              uuid            PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       uuid            NOT NULL REFERENCES tenants(id),
    entity_type     text            NOT NULL,
    entity_id       uuid            NOT NULL,
    event           text            NOT NULL,         -- e.g. "on_submit"
    workflow_fn     text            NOT NULL,         -- registered workflow function name
    workflow_id     text            NOT NULL UNIQUE,  -- pre-computed canonical ID
    task_queue      text            NOT NULL,
    input_payload   jsonb           NOT NULL,         -- serialized workflow input
    status          text            NOT NULL DEFAULT 'PENDING',  -- PENDING | PROCESSING | DONE | FAILED
    attempts        int             NOT NULL DEFAULT 0,
    last_attempt_at timestamptz,
    last_error      text,
    created_at      timestamptz     NOT NULL DEFAULT now(),
    processed_at    timestamptz
);

-- Status constraint
ALTER TABLE workflow_outbox
    ADD CONSTRAINT outbox_status_valid
    CHECK (status IN ('PENDING', 'PROCESSING', 'DONE', 'FAILED'));

-- Index for relay query: pending entries ordered by creation time
CREATE INDEX outbox_pending_idx ON workflow_outbox (status, created_at)
    WHERE status IN ('PENDING', 'FAILED');

-- workflow_id is globally unique: enforced at DB level
-- (duplicate workflow start = Temporal idempotency; outbox-level unique key prevents relay from
-- inserting the same entry twice due to retry)
```

RLS is NOT applied to `workflow_outbox` — the relay process operates with platform-level access, not tenant-scoped access. The relay reads across all tenants.

---

## 3. Atomic Write

The framework's entity persistence layer writes the outbox entry inside the same PostgreSQL transaction as the entity record mutation. This is the critical invariant (INV-005).

```go
// Framework-internal (not module code)
func (r *entityRepository) commitWithOutbox(
    ctx context.Context,
    tx pgx.Tx,
    record *EntityRecord,
    triggers []WorkflowTrigger,
) error {
    // Entity record already written to the transaction.

    for _, trigger := range triggers {
        input, err := trigger.InputBuilder(record, TriggerContext{
            TenantID: record.TenantID,
            Actor:    ActorFromCtx(ctx),
        })
        if err != nil {
            return fmt.Errorf("commitWithOutbox: build input for %s: %w", trigger.WorkflowFn, err)
        }

        payload, err := json.Marshal(input)
        if err != nil {
            return fmt.Errorf("commitWithOutbox: marshal input for %s: %w", trigger.WorkflowFn, err)
        }

        workflowID := buildCanonicalWorkflowID(record, trigger)

        _, err = tx.Exec(ctx, `
            INSERT INTO workflow_outbox
                (tenant_id, entity_type, entity_id, event, workflow_fn,
                 workflow_id, task_queue, input_payload, status)
            VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'PENDING')
            ON CONFLICT (workflow_id) DO NOTHING
        `,
            record.TenantID,
            record.EntityType,
            record.ID,
            trigger.On,
            trigger.WorkflowFn,
            workflowID,
            trigger.TaskQueue,
            payload,
        )
        if err != nil {
            return fmt.Errorf("commitWithOutbox: insert outbox entry: %w", err)
        }
    }

    return nil
}
```

`ON CONFLICT (workflow_id) DO NOTHING` handles the case where the same event is triggered twice (e.g., a retry of the entire create operation) — the outbox entry is deduplicated by the pre-computed workflow ID.

---

## 4. Relay Process

The relay runs as a goroutine started during worker startup. It polls for pending outbox entries and dispatches them to Temporal.

```go
// Framework-internal relay loop
func (r *OutboxRelay) Run(ctx context.Context) {
    ticker := time.NewTicker(r.pollInterval)  // typically 2-5 seconds
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            if err := r.processBatch(ctx); err != nil {
                slog.Error("outbox relay batch failed", "error", err)
            }
        }
    }
}

func (r *OutboxRelay) processBatch(ctx context.Context) error {
    entries, err := r.db.QueryOutboxPending(ctx, r.batchSize)
    if err != nil {
        return fmt.Errorf("relay: query pending: %w", err)
    }

    for _, entry := range entries {
        if err := r.dispatch(ctx, entry); err != nil {
            slog.Error("outbox dispatch failed",
                "workflow_id", entry.WorkflowID,
                "entity_type", entry.EntityType,
                "attempt", entry.Attempts+1,
                "error", err,
            )
            r.markFailed(ctx, entry, err)
        }
    }
    return nil
}
```

### Dispatch

```go
func (r *OutboxRelay) dispatch(ctx context.Context, entry OutboxEntry) error {
    // Mark PROCESSING to prevent concurrent relay pickup
    if err := r.db.MarkOutboxProcessing(ctx, entry.ID); err != nil {
        return fmt.Errorf("dispatch: mark processing: %w", err)
    }

    options := client.StartWorkflowOptions{
        ID:        entry.WorkflowID,
        TaskQueue: entry.TaskQueue,
        // WorkflowIDReusePolicy: WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE
        // (Temporal rejects if already started — outbox dedup is belt-and-suspenders)
    }

    run, err := r.temporalClient.ExecuteWorkflow(ctx, options, entry.WorkflowFn, entry.InputPayload)
    if err != nil {
        // Check if already running (idempotent: success)
        var alreadyStarted *serviceerror.WorkflowExecutionAlreadyStarted
        if errors.As(err, &alreadyStarted) {
            r.markDone(ctx, entry)
            return nil
        }
        return fmt.Errorf("dispatch: ExecuteWorkflow %s: %w", entry.WorkflowID, err)
    }

    r.markDone(ctx, entry, run.GetID(), run.GetRunID())
    return nil
}
```

---

## 5. At-Least-Once Semantics

The outbox provides **at-least-once** delivery. A workflow may be started more than once under failure scenarios:

| Scenario | Result |
|---|---|
| Process crashes before marking DONE | Relay retries on restart → Temporal rejects duplicate (WorkflowID already exists) → relay marks DONE |
| Temporal returns success but relay crashes before marking DONE | Same as above |
| Entity create retried by client; outbox entry already exists | `ON CONFLICT DO NOTHING` — no duplicate entry |
| Network partition during dispatch | Relay retries after poll interval |

Temporal's `WorkflowIDReusePolicy = REJECT_DUPLICATE` is the safety net: even if the relay dispatches twice, Temporal starts the workflow only once. The second dispatch receives `WorkflowExecutionAlreadyStarted`, which the relay treats as success.

Workflows MUST be designed for at-least-once start — their first activity step MUST be idempotent.

---

## 6. Retry Schedule

Failed entries (Temporal unreachable) are retried with exponential backoff:

| Attempt | Delay before retry |
|---|---|
| 1 | 5 seconds |
| 2 | 30 seconds |
| 3 | 2 minutes |
| 4 | 10 minutes |
| 5+ | 30 minutes (cap) |

After 48 hours without success, the entry is moved to `FAILED` status and an alert is raised. FAILED entries require manual investigation — Temporal may be degraded or the workflow function may have been removed.

```sql
-- Query for relay: PENDING or FAILED entries eligible for retry
SELECT * FROM workflow_outbox
WHERE status IN ('PENDING', 'FAILED')
  AND (last_attempt_at IS NULL OR last_attempt_at < now() - (
    CASE attempts
      WHEN 0 THEN INTERVAL '5 seconds'
      WHEN 1 THEN INTERVAL '30 seconds'
      WHEN 2 THEN INTERVAL '2 minutes'
      WHEN 3 THEN INTERVAL '10 minutes'
      ELSE INTERVAL '30 minutes'
    END
  ))
ORDER BY created_at ASC
LIMIT $1
```

---

## 7. PROCESSING Timeout

The relay marks entries `PROCESSING` before dispatching. If the relay process crashes during dispatch, entries remain `PROCESSING` indefinitely. A background cleanup query resets stale PROCESSING entries:

```sql
-- Reset PROCESSING entries older than 60 seconds (relay died mid-dispatch)
UPDATE workflow_outbox
SET status = 'PENDING', last_error = 'Relay died during dispatch; reset for retry'
WHERE status = 'PROCESSING'
  AND last_attempt_at < now() - INTERVAL '60 seconds';
```

This runs as part of the relay's startup and periodically during operation.

---

## 8. Monitoring

The outbox backlog is a critical operational metric:

```
# Pending count (should be near zero in steady state)
SELECT COUNT(*) FROM workflow_outbox WHERE status = 'PENDING';

# Failed count (requires manual intervention)
SELECT COUNT(*) FROM workflow_outbox WHERE status = 'FAILED';

# Age of oldest pending entry (SLA monitoring)
SELECT EXTRACT(EPOCH FROM now() - MIN(created_at)) AS oldest_pending_seconds
FROM workflow_outbox WHERE status = 'PENDING';
```

Recommended alerts:
- `outbox_pending_count > 100` for more than 5 minutes → Temporal connection issue
- `outbox_failed_count > 0` → manual review required
- `outbox_oldest_pending_seconds > 300` → relay may have crashed

---

## 9. Framework-Private Boundary (LAW-018)

Module authors MUST NOT:
- Read from `workflow_outbox`
- Write to `workflow_outbox` directly
- Inspect outbox status in business logic
- Depend on outbox processing timing

The outbox is an implementation detail of the framework's workflow dispatch mechanism. Module authors declare `WorkflowTriggers` in their `EntityDefinition` — the framework handles all outbox operations. This boundary exists so the outbox schema can evolve without affecting module code.

---

## Related Documents

- [Temporal Integration](temporal-integration.md) — workflow ID format, task queues
- [Activities](activities.md) — activities that execute within dispatched workflows
- [EntityDefinition §8](../03-kernel/entity-def.md#8-workflow-triggers) — WorkflowTrigger declaration
- [Architecture Laws](../02-architecture/laws.md) — LAW-006 (atomic outbox), LAW-018 (framework-private)
- [Invariants](../02-architecture/invariants.md) — INV-005 (every committed mutation has outbox entry)
- [Glossary](../GLOSSARY.md) — Outbox Pattern, Temporal, Workflow, At-Least-Once
