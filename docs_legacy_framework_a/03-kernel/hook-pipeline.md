> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Hook Pipeline"
id: kern-006
status: accepted
category: SPEC
stability: FROZEN
audience: [framework-authors, module-authors]
since: "1.0"
normative-level: normative
related:
  - "[Hooks](../04-domain/hooks.md)"
  - "[EntityDefinition](entity-def.md)"
  - "[Startup Sequence](startup-sequence.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Hook Pipeline

**KERN-006 | Status: Accepted | Stability: Frozen**

The hook pipeline specifies the exact execution order, transaction boundaries, and error semantics for all lifecycle hooks on an EntityRecord mutation.

---

## 1. Full Mutation Pipeline

```
HTTP POST /api/v1/entities/{type}  (or PATCH, DELETE, action)
    ↓
[1] ASSEMBLE: framework builds EntityRecord from request body
    ↓
[2] before_validate hooks (in declaration order)
    │  ← Outside transaction
    │  ← ValidationError → HTTP 422, abort pipeline
    ↓
[3] VALIDATE: FieldDef constraints (Required, Unique, MaxLen, Regex, etc.)
    │  ← ValidationError → HTTP 422, abort pipeline
    ↓
[4] AUTHORIZE: Casbin.Enforce(actor, tenant, entityType, action)
    │  ← PermissionError → HTTP 403, abort pipeline
    ↓
[5] before_save hooks (in declaration order)
    │  ← Outside transaction
    │  ← BusinessError → HTTP 4xx, abort pipeline
    ↓
[6] TX BEGIN
    ↓
[7] PERSIST: INSERT or UPDATE in PostgreSQL
    │  ← DB error (unique violation etc.) → parsed to BusinessError → rollback → HTTP 4xx
    ↓
[8] after_save hooks (in declaration order)
    │  ← Inside transaction
    │  ← error → rollback → HTTP 5xx (or 4xx if BusinessError)
    ↓
[9] TX COMMIT
    ↓
[10] Temporal workflow start (outside TX, via outbox relay)
    ↓
[11] Return HTTP 201 / 200 / 202
```

---

## 2. Hook Types and Stages

| Hook type | Stage | Inside TX? | Abort via |
|---|---|---|---|
| `before_validate` | [2] | No | `ValidationError` |
| `before_save` | [5] | No | `BusinessError` |
| `after_save` | [8] | **Yes** | any error → rollback |

### before_validate

Use for fast, stateless checks that don't need DB access:

```go
type ContactNameValidator struct{}

func (h *ContactNameValidator) BeforeValidate(ctx context.Context, record *def.EntityRecord) error {
    name, _ := record.Fields["name"].(string)
    if len(strings.TrimSpace(name)) < 2 {
        return &def.ValidationError{
            Fields: map[string]string{
                "name": "Name must be at least 2 characters",
            },
        }
    }
    return nil
}
```

### before_save

Use for business rule enforcement requiring DB reads:

```go
type InvoicePeriodLockGuard struct {
    Repo def.EntityRepository[AccountingPeriod]
}

func (h *InvoicePeriodLockGuard) BeforeSave(ctx context.Context, record *def.EntityRecord, isUpdate bool) error {
    invoiceDate, _ := record.Fields["invoice_date"].(time.Time)

    locked, err := h.Repo.Exists(ctx, filter.And(
        filter.LtEq("start_date", invoiceDate),
        filter.GtEq("end_date", invoiceDate),
        filter.Eq("status", "Closed"),
    ))
    if err != nil {
        return fmt.Errorf("InvoicePeriodLockGuard.BeforeSave: check period: %w", err)
    }
    if locked {
        return &def.BusinessError{
            Code:    "finance.period_locked",
            Message: "The accounting period for this invoice date is closed",
            Status:  400,
        }
    }
    return nil
}
```

### after_save

Use for side effects that must be atomic with the save:

```go
type InvoiceAuditTrail struct {
    AuditRepo def.EntityRepository[AuditEntry]
}

func (h *InvoiceAuditTrail) AfterSave(ctx context.Context, record *def.EntityRecord, isUpdate bool) error {
    // This runs inside the DB transaction
    // If this fails, the invoice is not saved either
    _, err := h.AuditRepo.Create(ctx, def.CreateInput{
        Fields: map[string]any{
            "entity_type": "finance_invoice",
            "entity_id":   record.ID,
            "action":      "save",
        },
    })
    if err != nil {
        return fmt.Errorf("InvoiceAuditTrail.AfterSave: create audit: %w", err)
    }
    return nil
}
```

**Note**: The framework's automatic audit log hook also runs in `after_save`. Module hooks that add their own audit entries are writing supplementary audit data (the framework's entry is always created).

---

## 3. Execution Order

Within each hook stage, hooks run in declaration order:

```go
Hooks: def.HookSet{
    BeforeSave: []def.BeforeSaveHook{
        &PeriodLockGuard{},    // runs first
        &CreditLimitCheck{},   // runs second
        &DuplicateInvoiceCheck{}, // runs third
    },
},
```

The first hook to return an error aborts the remaining hooks in the same stage. The pipeline is then aborted.

---

## 4. Automatic Hooks (Framework-Injected)

The framework injects these hooks invisibly — module authors do not declare them:

| Stage | Hook | Purpose |
|---|---|---|
| `before_validate` | `ImmutableFieldGuard` | Rejects updates to fields with `Immutable: true` |
| `before_validate` | `SensitiveFieldLogger` | Ensures sensitive fields are masked in log context |
| `after_save` | `AutoAuditHook` | Creates audit log entry for every mutation |
| `after_save` | `NamingSeriesAssigner` | Assigns `NamingSeries` value on create |
| `after_save` | `OutboxEventWriter` | Writes workflow trigger to outbox table |

Framework hooks run after module-declared hooks within the same stage. The execution order is:

```
[module hook 1] → [module hook 2] → ... → [framework hook 1] → [framework hook 2]
```

This ensures module validation and business rules are checked before the framework performs its bookkeeping.

---

## 5. Delete Pipeline

```
DELETE /api/v1/entities/{type}/{id}
    ↓
[1] AUTHORIZE: Casbin.Enforce(actor, tenant, entityType, "delete")
    ↓
[2] before_delete hooks
    │  ← Outside transaction
    ↓
[3] TX BEGIN
    ↓
[4] Cascade delete child edges (EdgeDef with CascadeDelete: true)
    ↓
[5] SOFT DELETE: set deleted_at = now() (if deleted_at field exists)
    or HARD DELETE: DELETE FROM table (if no deleted_at field)
    ↓
[6] after_delete hooks
    │  ← Inside transaction
    ↓
[7] TX COMMIT
    ↓
[8] Return HTTP 204
```

---

## 6. Pipeline Performance

The pipeline executes sequentially within a request goroutine. Performance considerations:

- `before_validate` hooks should be sub-millisecond (no I/O)
- `before_save` hooks may make DB reads — keep them bounded (1-3 queries max per hook)
- `after_save` hooks run inside TX — keep them fast (1-2 simple writes max)
- Long `before_save` chains increase response latency for every mutation

For expensive asynchronous work (sending emails, calling external APIs), use `after_save` to write an outbox entry, then process in a Temporal activity.

---

## Related Documents

- [Hooks](../04-domain/hooks.md) — hook interface definitions and patterns
- [EntityDefinition](entity-def.md) — `HookSet` field on EntityDefinition
- [Transactions](../05-persistence/transactions.md) — `after_save` inside TX semantics
- [Entity Events](../04-domain/events.md) — `OutboxEventWriter` framework hook detail
