> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

# Event Outbox Specification

**Classification:** Specification — Tier 1
**Owner:** `09-events/EVENT_OUTBOX_SPEC.md`
**Status:** Frozen at v1.0 (ADR-008)

---

## Purpose

This document specifies the `event_outbox` table, the `EventBroker` interface, and the delivery protocol for domain events.

---

## 1. Schema (ADR-008)

```sql
CREATE TABLE event_outbox (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   uuid NOT NULL,
    topic       text NOT NULL,
    payload     jsonb NOT NULL,
    status      text NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'delivered', 'failed')),
    attempts    int  NOT NULL DEFAULT 0,
    last_error  text,
    created_at  timestamptz NOT NULL DEFAULT now(),
    deliver_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX event_outbox_pending
    ON event_outbox (deliver_at)
    WHERE status = 'pending';
```

This schema is the public contract for the event outbox. It MUST NOT be changed without an ADR and a migration.

---

## 2. Writing Events

`ActionRuntime.Publish()` writes to `event_outbox` within the current request's transaction:

```go
err := action.Runtime.Publish(ctx, def.ActionEvent{
    Topic:   "finance.invoice.submitted",
    Payload: InvoiceSubmittedPayload{
        InvoiceID: rec.ID,
        Total:     rec.GetDecimal("total"),
        CustomerID: rec.GetUUID("customer_id"),
    },
    // TenantID is set automatically by the runtime
})
```

**Transactional guarantee:** If `Publish` is called from an `after_save` hook (inside the TX) and the TX later rolls back, the event record is also rolled back. Events are never orphaned.

**`deliver_at`:** Defaults to `now()`. Can be set in the future for delayed event delivery (e.g., send a reminder in 24 hours).

---

## 3. EventBroker Interface

```go
// Package: awo.so/awo/outbox

// EventBroker delivers events from the event_outbox to a message broker.
// The framework provides the outbox worker; the broker adapter is pluggable.
type EventBroker interface {
    // Publish delivers one event to the message broker.
    // Returns nil on success. Returns error on failure (will be retried).
    Publish(ctx context.Context, topic string, tenantID uuid.UUID, payload json.RawMessage) error
}
```

Implementations provided:
- `outbox.KafkaBroker` — Kafka-backed
- `outbox.NATSBroker` — NATS-backed
- `outbox.RedisPubSubBroker` — Redis Pub/Sub (for low-volume use cases)
- `outbox.NoopBroker` — discards all events (for development/testing)

---

## 4. Outbox Worker

A goroutine launched at startup polls `event_outbox`:

```sql
SELECT id, tenant_id, topic, payload
FROM event_outbox
WHERE status = 'pending'
  AND deliver_at <= now()
ORDER BY deliver_at ASC
LIMIT 100;
```

For each record:
1. `EventBroker.Publish(ctx, topic, tenantID, payload)`
2. On success: `UPDATE event_outbox SET status='delivered', attempts=attempts+1 WHERE id=$1`
3. On failure: exponential backoff update (same schedule as workflow outbox — 1s, 2s, 4s... cap 5min)

After 24 hours: `status='failed'`, alert fired.

---

## 5. Topic Naming Convention

```
{module}.{entity_local_name}.{event_verb}
```

| Pattern | Example |
|---------|---------|
| `{module}.{entity}.created` | `finance.invoice.created` |
| `{module}.{entity}.updated` | `finance.invoice.updated` |
| `{module}.{entity}.submitted` | `finance.invoice.submitted` |
| `{module}.{entity}.approved` | `finance.invoice.approved` |
| `{module}.{entity}.cancelled` | `finance.invoice.cancelled` |

Topics are immutable after the first event is published with that topic name. Consumers subscribe by topic; renaming a topic breaks all subscribers.

---

## 6. Payload Contract

Payloads MUST be JSON-serializable. They SHOULD include:
- The record's primary key (`id` or `invoice_id`)
- The `tenant_id`
- The `event_at` timestamp
- Relevant business fields needed by consumers

Payloads MUST NOT include:
- Sensitive fields (marked `Sensitive: true`)
- Full record snapshots (keep payloads small; consumers can fetch full records if needed)

---

## References

- `awo/outbox/event.go` — EventBroker interface, EventOutboxRecord
- [`09-events/DOMAIN_EVENTS_REFERENCE.md`](DOMAIN_EVENTS_REFERENCE.md) — Topic catalog
- ADR-008 in [`00-overview/DECISION_REGISTER.md`](../00-overview/DECISION_REGISTER.md)
