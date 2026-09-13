> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Entity Events"
id: dom-008
status: accepted
category: SPEC
stability: STABLE
audience: [module-authors]
since: "1.0"
normative-level: normative
related:
  - "[Hooks](hooks.md)"
  - "[Actions](actions.md)"
  - "[Temporal Integration](../09-workflow/temporal-integration.md)"
  - "[Webhooks](../11-api/webhooks.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Entity Events

**DOM-008 | Status: Accepted | Stability: Stable**

This document specifies the entity event system: the lifecycle events that trigger workflows and webhooks, event payload structure, and ordering guarantees.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. Lifecycle Events

Every entity mutation emits one of the following events:

| Event constant | Trigger | WorkflowTrigger `On` field |
|---|---|---|
| `EventOnCreate` | Entity record created | `entity.EventOnCreate` |
| `EventOnUpdate` | Entity record updated (any field) | `entity.EventOnUpdate` |
| `EventOnDelete` | Entity record deleted | `entity.EventOnDelete` |
| `EventOnSubmit` | Custom action `submit` executed | `entity.EventOnSubmit` |
| `EventOnCancel` | Custom action `cancel` executed | `entity.EventOnCancel` |
| `EventOnAction` | Any named custom action executed | `entity.EventOnAction("{action-name}")` |

Custom action events use the action's name: `EventOnAction("approve")` fires when the `approve` action is executed.

---

## 2. Declaring Workflow Triggers

```go
WorkflowTriggers: []entity.WorkflowTrigger{
    {
        On:         entity.EventOnSubmit,
        WorkflowFn: "InvoiceSubmissionWorkflow",
        TaskQueue:  "finance.invoice.submit",
        InputBuilder: func(rec *entity.EntityRecord, tc entity.TriggerContext) (any, error) {
            amount, _ := rec.Fields["total_kes"].(decimal.Decimal)
            return InvoiceSubmissionInput{
                TenantID:  rec.TenantID,
                InvoiceID: rec.ID,
                Amount:    amount,
            }, nil
        },
    },
    {
        On:         entity.EventOnCreate,
        WorkflowFn: "InvoiceCreatedNotificationWorkflow",
        TaskQueue:  "finance.notifications",
        // No InputBuilder: uses default (TenantID + RecordID)
    },
},
```

### TriggerContext

`TriggerContext` provides additional context to the `InputBuilder`:

```go
type TriggerContext struct {
    Actor     Actor      // user who triggered the event
    EventName string     // e.g., "submit", "approve"
    Timestamp time.Time  // event time (UTC)
}
```

---

## 3. Event Ordering Guarantees

Events are dispatched via the transactional outbox pattern (WF-004). Guarantees:

| Property | Guarantee |
|---|---|
| At-least-once | The workflow start is retried until Temporal acknowledges it |
| Ordering | Within one entity, events dispatch in commit order |
| Cross-entity ordering | Not guaranteed — use Temporal signals for cross-entity sequencing |

Events are NOT dispatched synchronously during the HTTP request. The outbox relay dispatches them asynchronously — the HTTP response returns before the workflow starts. Typical delay: <1 second under normal load.

---

## 4. Multiple Triggers per Event

Multiple workflow triggers can fire on the same event:

```go
WorkflowTriggers: []entity.WorkflowTrigger{
    {On: entity.EventOnCreate, WorkflowFn: "LeaveRequestApprovalWorkflow", TaskQueue: "hr.leave"},
    {On: entity.EventOnCreate, WorkflowFn: "LeaveBalanceReservationWorkflow", TaskQueue: "hr.balance"},
},
```

Both workflows start independently. They do not share a transaction — either can fail independently. Design each workflow to be idempotent.

---

## 5. Conditional Triggers

Use the `Condition` field to fire workflows only when certain criteria are met:

```go
WorkflowTriggers: []entity.WorkflowTrigger{
    {
        On:         entity.EventOnCreate,
        WorkflowFn: "HighValueInvoiceReviewWorkflow",
        TaskQueue:  "finance.review",
        Condition: func(rec *entity.EntityRecord, tc entity.TriggerContext) bool {
            amount, _ := rec.Fields["total_kes"].(decimal.Decimal)
            return amount.GreaterThan(decimal.NewFromFloat(500000))
        },
        InputBuilder: func(rec *entity.EntityRecord, tc entity.TriggerContext) (any, error) {
            return InvoiceReviewInput{TenantID: rec.TenantID, InvoiceID: rec.ID}, nil
        },
    },
},
```

`Condition` is evaluated synchronously before the outbox entry is written. If it returns false, no outbox entry is created — the workflow never starts.

---

## 6. Webhook Events

Entity events also drive webhook delivery. Every lifecycle event for an entity type that has webhook subscribers triggers a webhook delivery:

```
finance_invoice.created   → POST to all subscribers of this event
finance_invoice.submitted → POST to all subscribers of this event
```

Webhook delivery is separate from workflow triggering — both may be configured for the same event. A webhook subscriber and a workflow trigger are independent.

---

## 7. Event Payload in Outbox

The outbox entry stores the full event payload at the time of commit:

```go
type OutboxEntry struct {
    ID         uuid.UUID
    EntityType string
    EntityID   uuid.UUID
    EventName  string
    TenantID   uuid.UUID
    ActorID    uuid.UUID
    WorkflowFn string
    TaskQueue  string
    Payload    json.RawMessage  // serialized workflow input
    Status     string           // PENDING, PROCESSING, DONE, FAILED
    CreatedAt  time.Time
}
```

The payload is the output of `InputBuilder` — serialized to JSON. If the entity changes after the event is committed but before the workflow starts, the workflow receives the values at the time of the event (snapshot semantics).

---

## 8. Audit Log Integration

Every entity event automatically creates an audit log entry:

```sql
-- iam_audit_log schema (excerpt)
INSERT INTO iam_audit_log (tenant_id, entity_type, entity_id, event, actor_id, before_state, after_state, timestamp)
VALUES ($1, $2, $3, $4, $5, $6, $7, NOW());
```

`before_state` and `after_state` are JSON snapshots of the entity before and after the mutation. `Sensitive` fields are excluded from both snapshots.

Module code does not write audit log entries directly — the framework writes them automatically for every entity mutation.

---

## Related Documents

- [Hooks](hooks.md) — hook execution order relative to events
- [Actions](actions.md) — custom actions that emit `EventOnAction`
- [Outbox Pattern](../09-workflow/outbox-pattern.md) — how events are durably dispatched
- [Webhooks](../11-api/webhooks.md) — how entity events drive webhook delivery
- [Architecture Laws](../02-architecture/laws.md) — LAW-006 (outbox for workflow dispatch), LAW-018 (outbox is framework-private)
