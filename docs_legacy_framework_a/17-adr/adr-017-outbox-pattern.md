> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "ADR-017: Transactional Outbox for Workflow Dispatch"
id: adr-017
status: accepted
category: ADR
stability: STABLE
audience: [framework-authors, module-authors]
since: "1.0"
normative-level: normative
related:
  - "[Entity Events](../04-domain/events.md)"
  - "[Workflow Trigger](../09-workflow/README.md)"
  - "[Webhooks](../11-api/webhooks.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# ADR-017: Transactional Outbox for Workflow Dispatch

**Status: Accepted**

---

## Context

When an entity is created or updated, the framework must start a Temporal workflow. This involves two distinct I/O operations:

1. Commit the entity record to PostgreSQL (inside the business transaction)
2. Call `temporalClient.StartWorkflow(...)` (HTTP call to Temporal server)

If the PostgreSQL commit succeeds but the Temporal call fails (network blip, Temporal unavailable), the entity is saved but the workflow never starts. Depending on the workflow (invoice submission, provisioning, payroll), this silent failure causes data corruption or missed business process.

---

## Decision

Use the **Transactional Outbox pattern**: within the same PostgreSQL transaction that saves the entity, also insert a row into an `outbox_events` table. A separate relay process polls the outbox and dispatches events to Temporal. Only after successful dispatch is the outbox entry marked as delivered.

This makes workflow dispatch **at-least-once** with the entity commit — the two operations are atomic from the business perspective.

---

## Consequences

### Positive

**Atomic entity + event**: entity commit and outbox insert are in the same transaction. Either both succeed or neither does. No silent failures.

**At-least-once delivery**: if dispatch fails, the relay retries. Temporal workflows are idempotent by design (workflow ID deduplication) — duplicate dispatches are safe.

**Temporal can be unavailable temporarily**: entities are saved; outbox accumulates; relay drains when Temporal recovers. No data loss.

**Auditable**: the outbox table records every dispatch attempt — useful for debugging and compliance.

### Negative

**Relay process required**: a background goroutine (or separate service) must continuously poll the outbox. It is part of the startup sequence.

**Delivery latency**: outbox polling interval (default: 500ms) adds a small delay between commit and workflow start. For invoice submission workflows, sub-second delay is acceptable.

**Outbox table growth**: delivered entries accumulate. A cleanup job purges entries older than 7 days.

---

## Alternatives Considered

### Direct Temporal call inside transaction

Call `StartWorkflow` at the end of the database transaction (before COMMIT). Rejected:
- Temporal call failure causes transaction rollback → entity is not saved → user gets an error even though the business operation was valid
- Temporal call inside a DB transaction holds the connection for longer than necessary
- Network latency to Temporal (potentially a separate cluster) inside a DB transaction creates lock contention

### Direct Temporal call outside transaction

Call `StartWorkflow` after the transaction commits. Rejected:
- If the process crashes between COMMIT and `StartWorkflow`, the workflow never starts and there is no record of the intent
- Not recoverable without manual intervention

### Kafka / message broker

Route events through a message broker instead of a PostgreSQL outbox. Rejected:
- Introduces another infrastructure dependency (Kafka or similar)
- Transactional atomicity with the DB write requires Kafka transactions (complex, less portable)
- PostgreSQL outbox achieves the same result with one fewer infrastructure component

---

## Outbox Schema

```sql
CREATE TABLE outbox_events (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    uuid NOT NULL,
    entity_type  varchar(128) NOT NULL,
    entity_id    uuid NOT NULL,
    event_name   varchar(128) NOT NULL,
    payload      jsonb NOT NULL,
    workflow_fn  varchar(256),
    workflow_id  varchar(512) UNIQUE,  -- pre-computed Temporal workflow ID
    status       varchar(32) NOT NULL DEFAULT 'pending',  -- pending, delivered, failed
    attempts     int NOT NULL DEFAULT 0,
    last_error   text,
    created_at   timestamptz NOT NULL DEFAULT now(),
    delivered_at timestamptz
);

CREATE INDEX ON outbox_events(status, created_at) WHERE status = 'pending';
```

---

## Relay Implementation

```go
// Simplified relay — runs as a goroutine in the worker process
func (r *OutboxRelay) Run(ctx context.Context) {
    ticker := time.NewTicker(500 * time.Millisecond)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            r.drain(ctx)
        }
    }
}

func (r *OutboxRelay) drain(ctx context.Context) {
    // SELECT ... FOR UPDATE SKIP LOCKED — safe for multiple relay instances
    pending, err := r.fetchPendingBatch(ctx, 50)
    if err != nil {
        slog.Error("outbox relay: fetch pending", "err", err)
        return
    }

    for _, entry := range pending {
        if err := r.dispatch(ctx, entry); err != nil {
            r.markFailed(ctx, entry, err)
        } else {
            r.markDelivered(ctx, entry)
        }
    }
}
```

---

## Related Documents

- [Entity Events](../04-domain/events.md) — events that produce outbox entries
- [Scheduled Workflows](../09-workflow/scheduled-workflows.md) — scheduled workflows use Temporal Schedules, not outbox
- [Webhooks](../11-api/webhooks.md) — webhook delivery also uses the outbox pattern
