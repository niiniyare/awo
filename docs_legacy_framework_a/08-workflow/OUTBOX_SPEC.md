> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

# Workflow Outbox Specification

**Classification:** Specification — Tier 1
**Owner:** `08-workflow/OUTBOX_SPEC.md`
**Status:** Frozen at v1.0 (ADR-007)

---

## Purpose

This document specifies the `workflow_outbox` table, the outbox worker, and the retry protocol for durable Temporal workflow dispatch.

---

## 1. Problem Statement

Without the outbox, `ActionRuntime.StartWorkflow()` calls Temporal directly. If the entity transaction commits but Temporal is unavailable, the workflow never starts. In financial contexts, a committed invoice with no processing workflow is silent data corruption.

The outbox pattern solves this: workflow starts are written to a database table within the entity's transaction. An outbox worker reads the table and dispatches to Temporal with retry. If the entity transaction rolls back, the outbox record also rolls back. If Temporal is unavailable, the record waits in the outbox until Temporal recovers.

---

## 2. Schema (ADR-007)

```sql
CREATE TABLE workflow_outbox (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       uuid NOT NULL,
    entity_name     text NOT NULL,
    record_id       uuid NOT NULL,
    workflow_fn     text NOT NULL,
    task_queue      text NOT NULL,
    workflow_id     text,           -- NULL uses auto-generated ID convention
    input           jsonb,
    status          text NOT NULL DEFAULT 'pending'
                        CHECK (status IN ('pending', 'dispatched', 'failed')),
    attempts        int  NOT NULL DEFAULT 0,
    last_error      text,
    created_at      timestamptz NOT NULL DEFAULT now(),
    next_attempt_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX workflow_outbox_pending
    ON workflow_outbox (next_attempt_at)
    WHERE status = 'pending';
```

---

## 3. Record Creation

The runtime writes to `workflow_outbox` OUTSIDE the entity's transaction, after the entity transaction commits:

```
[Entity TX commits]
        │
        ▼
Runtime writes to workflow_outbox
(in a separate, short-lived transaction)
```

The outbox record write uses `INSERT ... RETURNING id`. If this write fails (e.g., PostgreSQL connection dropped between entity TX commit and outbox write), the outbox write is retried by the runtime for a short window (3 attempts, 100ms apart). After that, the failure is logged with `entity_name`, `record_id`, and `workflow_fn` for manual recovery.

---

## 4. Status Transitions

```
pending → dispatched   (outbox worker successfully called Temporal.StartWorkflow)
pending → pending      (retry scheduled: next_attempt_at is updated)
pending → failed       (24h elapsed with no successful dispatch)
```

**`failed` is terminal for automated retry.** Manual intervention (re-queue or discard) is required for failed records.

---

## 5. Outbox Worker

The outbox worker is a goroutine launched at startup. It polls `workflow_outbox` for pending records:

```go
// Poll interval: 1 second
SELECT id, tenant_id, entity_name, record_id, workflow_fn,
       task_queue, workflow_id, input
FROM workflow_outbox
WHERE status = 'pending'
  AND next_attempt_at <= now()
ORDER BY next_attempt_at ASC
LIMIT 100;
```

For each record:
1. Call `Temporal.StartWorkflow(workflowFn, taskQueue, workflowID, input)`
2. On success: `UPDATE workflow_outbox SET status='dispatched', attempts=attempts+1 WHERE id=$1`
3. On failure: compute `next_attempt_at` using exponential backoff; `UPDATE ... SET attempts=attempts+1, last_error=$1, next_attempt_at=$2`

---

## 6. Retry Policy

| Attempt | Delay before next attempt |
|---------|--------------------------|
| 1 | 1 second |
| 2 | 2 seconds |
| 3 | 4 seconds |
| 4 | 8 seconds |
| 5 | 16 seconds |
| 6–N | Capped at 5 minutes |

After 24 hours with no successful dispatch:
- `UPDATE workflow_outbox SET status='failed' WHERE id=$1`
- Alert fired via observability system

---

## 7. WorkflowID Convention

When `WorkflowTrigger.WorkflowID` is empty, the runtime generates:
```
{tenant_id}.{qualified_entity_name}.{record_id}.{event_name}
```

Example:
```
abc123-def4-5678-9012-abcdefabcdef.finance_invoice.inv456-abc1-2345-6789-0123456789ab.on_submit
```

This convention:
- Guarantees global uniqueness (tenant + entity + record + event)
- Enables Temporal's workflow ID deduplication (idempotent start)
- Provides a human-readable audit trail in Temporal Web UI

---

## 8. Idempotency

If the outbox worker is interrupted between dispatching to Temporal and marking the record `dispatched`, it will retry. This means `StartWorkflow` may be called multiple times for the same record.

**Temporal handles this via workflow ID deduplication.** If a workflow with the same ID is already running, `StartWorkflow` returns the existing workflow without starting a new one. This is why the workflow ID convention MUST be deterministic.

Workflow functions MUST be idempotent with respect to restart from history — this is a Temporal guarantee, not a requirement on module authors.

---

## References

- `awo/outbox/workflow.go` — WorkflowOutboxRecord, WorkflowDispatcher interface
- [`08-workflow/TEMPORAL_INTEGRATION.md`](TEMPORAL_INTEGRATION.md) — How triggers declare workflow starts
- [`08-workflow/WORKFLOW_ID_CONVENTION.md`](WORKFLOW_ID_CONVENTION.md) — ID format
- ADR-007 in [`00-overview/DECISION_REGISTER.md`](../00-overview/DECISION_REGISTER.md)
