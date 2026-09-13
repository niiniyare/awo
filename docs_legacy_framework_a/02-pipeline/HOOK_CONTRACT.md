> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

# Hook Contract

**Classification:** Specification — Tier 1
**Owner:** `02-pipeline/HOOK_CONTRACT.md`
**Status:** Frozen at v1.0
**Package:** `awo.so/awo/def`

---

## Purpose

This document specifies all hook interfaces, the `HookSet` structure, execution order, and the rules that hook implementations MUST follow.

## Dependencies

- [`02-pipeline/LIFECYCLE_SPEC.md`](LIFECYCLE_SPEC.md) — Pipeline stages and TX boundaries
- [`02-pipeline/ERROR_MODEL.md`](ERROR_MODEL.md) — Error types hooks may return

---

## 1. HookSet

```go
type HookSet struct {
    BeforeCreate []BeforeCreateHook
    AfterCreate  []AfterCreateHook
    BeforeUpdate []BeforeUpdateHook
    AfterUpdate  []AfterUpdateHook
    BeforeDelete []BeforeDeleteHook
    AfterDelete  []AfterDeleteHook
}
```

Each field is a slice of hook implementations. Hooks within a slice execute in declaration order. The pipeline stops at the first hook that returns a non-nil error.

---

## 2. Hook Interfaces

### BeforeCreateHook

```go
type BeforeCreateHook interface {
    BeforeCreate(ctx context.Context, record *EntityRecord) error
}
```

**Pipeline stage:** `before_validate` (before field validation; no open transaction)

**Purpose:** Field normalization and pre-validation transformation. Examples:
- Normalizing a phone number format before it is validated
- Deriving a computed field value from other fields
- Converting user input (e.g., uppercase normalization)

**Return values:**
- `nil` — proceed to next hook
- `*ValidationError` — abort; return HTTP 422 with field-level messages
- Any other non-nil error — abort; return HTTP 500

**Transaction state:** No open transaction. Database reads are safe. Database writes are unsafe (no TX to roll back).

**Prohibition:** Do NOT perform slow operations (HTTP calls, network I/O) in `BeforeCreate`. The request TX has not started; blocking here causes high-latency responses.

---

### AfterCreateHook

```go
type AfterCreateHook interface {
    AfterCreate(ctx context.Context, record *EntityRecord) error
}
```

**Pipeline stage:** `after_save` (after PERSIST and AUDIT RECORD; inside open transaction)

**Purpose:** Post-persist side effects that MUST be atomic with the create. Examples:
- Assigning a naming series value (e.g., `INV-2026-00042`)
- Creating related records (e.g., creating a default invoice line)
- Writing journal entries for financial double-entry accounting
- Updating parent aggregate counts

**Return values:**
- `nil` — proceed to next hook; TX will commit
- Any non-nil error — TX rollback; return HTTP 500 (the create is undone)

**Transaction state:** Inside the open transaction. All database writes are atomic with the original create. If this hook returns an error, the record that was just created is also rolled back.

**record.ID:** The record's UUID primary key is populated by PERSIST before `AfterCreate` runs. Hooks may reference `record.ID`.

---

### BeforeUpdateHook

```go
type BeforeUpdateHook interface {
    BeforeUpdate(ctx context.Context, record *EntityRecord) error
}
```

**Pipeline stage:** `before_validate` (no open transaction)

**record contents:** Merged state — current stored values overlaid with the incoming patch values. `record.Meta.OperationType == OperationUpdate`.

**Purpose:** Validate cross-field consistency of the updated state. Example: if `end_date` is being set, verify it is after `start_date`.

**Return values:**
- `nil` — proceed
- `*ValidationError` — abort; HTTP 422
- `*BusinessError` — abort; HTTP with `BusinessError.Status`

---

### AfterUpdateHook

```go
type AfterUpdateHook interface {
    AfterUpdate(ctx context.Context, record *EntityRecord) error
}
```

**Pipeline stage:** `after_save` (inside open transaction)

**record contents:** Final persisted state after the update.

**Purpose:** Post-update side effects that must be atomic. Example: re-computing totals on a parent record when a child is updated.

**Return values:**
- `nil` — proceed; TX commits
- Any non-nil error — TX rollback; the update is undone

---

### BeforeDeleteHook

```go
type BeforeDeleteHook interface {
    BeforeDelete(ctx context.Context, record *EntityRecord) error
}
```

**Pipeline stage:** `before_validate` (no open transaction)

**record contents:** Current state of the record about to be deleted.

**Purpose:** Prevent deletion of records in invalid states. Examples:
- Blocking deletion of a submitted invoice
- Preventing deletion of a tenant that has active subscriptions

**Return values:**
- `nil` — allow delete
- `*BusinessError` — abort; return HTTP 400 / 409 / 422 per `BusinessError.Status`
- `*ValidationError` — abort; HTTP 422

---

### AfterDeleteHook

```go
type AfterDeleteHook interface {
    AfterDelete(ctx context.Context, record *EntityRecord) error
}
```

**Pipeline stage:** `after_save` (inside open transaction)

**record contents:** The record's final state before deletion.

**Purpose:** Clean up related data not covered by CASCADE rules. Example: archiving related records when the parent is deleted.

**Return values:**
- `nil` — proceed; TX commits; delete is final
- Any non-nil error — TX rollback; the delete is undone

---

## 3. Execution Order

For a Create operation with two `BeforeCreate` hooks and one `AfterCreate` hook:

```
before_validate stage:
  BeforeCreate[0].BeforeCreate(ctx, record)   → error stops here
  BeforeCreate[1].BeforeCreate(ctx, record)   → error stops here

VALIDATE (framework) → collect all field errors

AUTHORIZE (framework) → 403 if denied

[TX begins]
PERSIST (framework)
AUDIT RECORD (framework)

after_save stage:
  AfterCreate[0].AfterCreate(ctx, record)     → error → TX rollback

[TX commits]
```

**Invariant:** Hooks in a slice run in the order they appear in the `HookSet` field. Declaration order is execution order.

---

## 4. Implementing Hooks

Hooks MUST be implemented as Go structs with methods, not as closures, to allow dependency injection:

```go
// Correct: struct with injected dependencies
type InvoiceValidator struct {
    CustomerRepo def.ActionEntityRepo  // injected by tests
}

func (v *InvoiceValidator) BeforeCreate(ctx context.Context, rec *def.EntityRecord) error {
    customerID := rec.GetUUID("customer_id")
    if customerID == uuid.Nil {
        return &def.ValidationError{Fields: map[string]string{
            "customer_id": "Customer is required",
        }}
    }
    return nil
}

// Registration
var InvoiceDefinition = def.SystemDefinition{
    // ...
    Hooks: def.HookSet{
        BeforeCreate: []def.BeforeCreateHook{&InvoiceValidator{}},
    },
}
```

---

## 5. Context Usage in Hooks

Hooks receive a `context.Context` carrying:
- The `ViewerContext` (via `auth.ViewerFromContext(ctx)`)
- The cancellation signal
- The hook recursion stack (framework-internal)
- The transaction handle (in `after_save` hooks)

To access the viewer:
```go
func (h *MyHook) AfterCreate(ctx context.Context, rec *def.EntityRecord) error {
    viewer := auth.ViewerFromContext(ctx)
    // use viewer.TenantID(), viewer.UserID(), etc.
}
```

---

## 6. Normative Requirements

- Hook implementations MUST be goroutine-safe. The framework may call the same hook instance concurrently for different requests.
- Hook implementations MUST be stateless across requests. Any state MUST be injected at construction time.
- `before_validate` hooks (Before* interfaces) MUST NOT perform database writes.
- `after_save` hooks (After* interfaces) MUST be idempotent where possible (the TX may be retried by the runtime on certain transient failures).
- Hooks MUST NOT call `def.Register()` or modify the global registry.
- Hooks MUST NOT start goroutines that outlive the hook call.
- Hook implementations MUST satisfy the corresponding interface at compile time (use `var _ def.BeforeCreateHook = (*MyHook)(nil)`).

---

## 7. Testing Hooks

Test hooks in isolation using mock repositories. Do not test hooks via full HTTP requests unless testing the complete pipeline behavior.

```go
func TestInvoiceValidator_BeforeCreate(t *testing.T) {
    hook := &InvoiceValidator{}

    tests := []struct {
        name    string
        record  *def.EntityRecord
        wantErr bool
        wantField string
    }{
        {
            name: "valid record",
            record: &def.EntityRecord{Data: map[string]any{"customer_id": uuid.New()}},
            wantErr: false,
        },
        {
            name: "missing customer",
            record: &def.EntityRecord{Data: map[string]any{}},
            wantErr: true,
            wantField: "customer_id",
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := hook.BeforeCreate(context.Background(), tt.record)
            if (err != nil) != tt.wantErr {
                t.Fatalf("BeforeCreate() error = %v, wantErr %v", err, tt.wantErr)
            }
            if tt.wantErr && tt.wantField != "" {
                var ve *def.ValidationError
                if !errors.As(err, &ve) {
                    t.Fatalf("expected ValidationError, got %T", err)
                }
                if _, ok := ve.Fields[tt.wantField]; !ok {
                    t.Errorf("expected field %q in ValidationError", tt.wantField)
                }
            }
        })
    }
}
```

See [`16-testing/HOOK_TEST_PATTERNS.md`](../16-testing/HOOK_TEST_PATTERNS.md) for complete test patterns.

---

## References

- `awo/def/hook.go` — Interface declarations
- [`02-pipeline/LIFECYCLE_SPEC.md`](LIFECYCLE_SPEC.md) — When hooks execute
- [`02-pipeline/ERROR_MODEL.md`](ERROR_MODEL.md) — Error types
- [`16-testing/HOOK_TEST_PATTERNS.md`](../16-testing/HOOK_TEST_PATTERNS.md) — Testing patterns
