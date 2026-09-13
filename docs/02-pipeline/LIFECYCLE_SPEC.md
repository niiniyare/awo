# Entity Lifecycle Specification

**Classification:** Specification — Tier 0
**Owner:** `02-pipeline/LIFECYCLE_SPEC.md`
**Status:** Frozen at v1.0
**Package:** `awo.so/awo/runtime`

---

## Purpose

This document specifies the complete ordered pipeline that executes for every entity mutation (Create, Update, Delete). It defines: the stages, what runs at each stage, the transaction boundary, what errors abort which stages, and the invariants the runtime guarantees.

## Scope

This specification covers the runtime pipeline for Create, Update, and Delete mutations. Read operations (Get, Query, Count, Exists) are not pipelined — they pass through authentication and authorization middleware and execute a filtered query.

## Dependencies

- [`01-entity/ENTITY_DEFINITION_SPEC.md`](../01-entity/ENTITY_DEFINITION_SPEC.md) — EntityDefinition and HookSet
- [`02-pipeline/ERROR_MODEL.md`](ERROR_MODEL.md) — ValidationError, BusinessError
- [`03-auth/AUTHORIZATION_SPEC.md`](../03-auth/AUTHORIZATION_SPEC.md) — PolicyEvaluator
- [`12-audit/AUDIT_SPEC.md`](../12-audit/AUDIT_SPEC.md) — AuditRecord pipeline stage

---

## 1. Pipeline Stages

The following sequence is executed for every mutation. Stages are ordered and non-configurable.

```
HTTP Request arrives
        │
        ▼
┌───────────────┐
│  ASSEMBLE     │  Build EntityRecord from request body
└───────────────┘
        │
        ▼
┌───────────────┐
│before_validate│  HookSet.BeforeCreate / BeforeUpdate / BeforeDelete
└───────────────┘  Module hooks — may return ValidationError
        │
        ▼
┌───────────────┐
│   VALIDATE    │  Built-in: required fields, type coercion, immutable check,
└───────────────┘  field validators. Returns ValidationError on failure.
        │
        ▼
┌───────────────┐
│  AUTHORIZE    │  PolicyEvaluator.CanPerform(ctx, viewer, entity, action)
└───────────────┘  Returns 403 on deny. Platform admin bypasses.
        │
        ▼
┌───────────────┐
│  before_save  │  HookSet.BeforeCreate / BeforeUpdate / BeforeDelete
└───────────────┘  Module hooks — may return BusinessError
        │
   ─────┼──── [TX begins here] ────────────────────────────────┐
        │                                                       │ PostgreSQL
        ▼                                                       │ transaction
┌───────────────┐                                               │
│    PERSIST    │  Write to PostgreSQL (SQL columns or JSONB)   │
└───────────────┘                                               │
        │                                                       │
        ▼                                                       │
┌───────────────┐                                               │
│ AUDIT RECORD  │  Write AuditRecord to audit_log (same TX)     │
└───────────────┘  Skipped if AuditEnabled: false               │
        │                                                       │
        ▼                                                       │
┌───────────────┐                                               │
│  after_save   │  HookSet.AfterCreate / AfterUpdate / AfterDelete │
└───────────────┘  Error → TX rollback                         │
        │                                                       │
   ─────┼──── [TX commits here] ─────────────────────────────┘
        │
        ▼
┌───────────────┐
│ Workflow start│  Outbox worker dispatches to Temporal (OUTSIDE TX)
└───────────────┘
        │
        ▼
HTTP Response
```

---

## 2. Stage Specifications

### ASSEMBLE

**What runs:** The runtime constructs an `EntityRecord` from the HTTP request body JSON.
- Field values are decoded and coerced to their `FieldType` Go types.
- Default values (`FieldDef.Default`) are applied for absent fields.
- `EntityRecord.Meta.Actor` is set from the `ViewerContext`.
- `EntityRecord.Meta.OperationType` is set to `Create`, `Update`, or `Delete`.
- For Update: the current record is fetched and merged with the patch.
- `EntityRecord.ID` is set from the URL parameter (Update/Delete) or generated as UUIDv7 (Create).

**Failure:** Assembly failure (malformed JSON, unknown field) returns HTTP 400 before any hooks run.

---

### before_validate

**What runs:** `HookSet.BeforeCreate` / `HookSet.BeforeUpdate` / `HookSet.BeforeDelete` hooks, in declaration order.

**Transaction state:** Outside TX.

**Abort via:** Return `*def.ValidationError` from any hook. The runtime stops at the first error and returns HTTP 422. Remaining hooks in the slice do not run.

**Purpose:** Field transformation before validation. Example: normalizing phone number format, deriving a field value from another field.

**Prohibition:** Do not perform database writes in `before_validate` hooks. The TX has not started. Database reads are permitted.

---

### VALIDATE

**What runs:** Built-in framework validation:
1. Required field check — returns `ValidationError` for each missing required field.
2. Type coercion validation — ensures each field value matches its `FieldType`.
3. Immutable field check (Update only) — returns `ValidationError` if any `Immutable: true` field differs from the stored value.
4. Length and range checks — `MaxLen`, `Min`, `Max`.
5. Custom field validators — `FieldDef.Validators`, called in declaration order.
6. Select option validation — rejects values not in `FieldDef.Options`.

**Transaction state:** Outside TX.

**Abort via:** Any validation failure returns HTTP 422 with a `ValidationError` containing field-level messages.

**Atomicity:** All validation errors are collected before returning. A response may contain multiple field errors.

---

### AUTHORIZE

**What runs:** `PolicyEvaluator.CanPerform(ctx, viewer, qualifiedEntityName, action)`.

`action` is one of: `"create"`, `"read"`, `"write"`, `"delete"`, or a custom action name.

**Platform admin bypass:** If `viewer.IsPlatformAdmin()` returns `true`, this stage is skipped entirely.

**Transaction state:** Outside TX.

**Abort via:** `CanPerform` returns `(false, nil)` → HTTP 403. `CanPerform` returns `(_, error)` → HTTP 500.

---

### before_save

**What runs:** `HookSet.BeforeCreate` / `HookSet.BeforeUpdate` / `HookSet.BeforeDelete` hooks, in declaration order.

**Transaction state:** Outside TX.

**Abort via:** Return `*def.BusinessError` from any hook. The runtime stops at the first error and returns HTTP 400 (or the status code in `BusinessError.Status`). Remaining hooks do not run.

**Purpose:** Business rule validation that requires database access. Example: checking a customer's credit limit before allowing invoice creation, validating inventory availability.

**Distinction from before_validate:** `before_validate` is for field transformation and simple validation. `before_save` is for domain rule enforcement that may require data access.

---

### PERSIST

**What runs:** The entity driver writes the record to PostgreSQL.
- Create: `INSERT INTO {table_name} (...) VALUES (...) RETURNING *`
- Update: `UPDATE {table_name} SET ... WHERE id = $1 AND tenant_id = current_tenant_id()`
- Delete: `DELETE FROM {table_name} WHERE id = $1 AND tenant_id = current_tenant_id()`

**Transaction state:** Inside TX.

**Failure:** PostgreSQL errors are mapped to domain errors by `parseDBError()`. Constraint violations (unique, check) produce `BusinessError`. Other errors produce an internal error.

---

### AUDIT RECORD

**What runs:** The runtime writes an `AuditRecord` to the `audit_log` table within the same transaction.

**AuditRecord contains:**
- `entity_name` — qualified name of the entity
- `record_id` — UUID of the affected record
- `tenant_id` — owning tenant
- `actor_id` — user ID or service account ID from `ViewerContext`
- `operation` — `"create"`, `"update"`, or `"delete"`
- `occurred_at` — current timestamp within the TX
- `before` — snapshot of field values before the operation (Update/Delete only)
- `after` — snapshot of field values after the operation (Create/Update only)

**Skip condition:** Skipped if `EntityDefinition.AuditEnabled` is `false`.

**Sensitive field handling:** Fields with `Sensitive: true` are excluded from the `before` and `after` snapshots.

**Transaction state:** Inside TX. If this write fails, the TX rolls back.

---

### after_save

**What runs:** `HookSet.AfterCreate` / `HookSet.AfterUpdate` / `HookSet.AfterDelete` hooks, in declaration order.

**Transaction state:** Inside TX.

**Abort via:** Return any error from any hook → TX rolls back → HTTP 500 (or appropriate status).

**Critical constraint:** Since `after_save` runs inside the TX, any error causes the entire mutation to roll back, including the PERSIST and AUDIT RECORD stages.

**Purpose:** Post-persist side effects that MUST be atomic with the mutation. Examples:
- Writing journal entries when an invoice is created (double-entry constraint)
- Updating an aggregate counter in a related record
- Assigning naming series values

**Prohibition:** Do not perform slow or unreliable operations (HTTP calls, file writes) in `after_save`. The TX is open and PostgreSQL holds locks. Slow `after_save` hooks cause lock contention.

---

### Workflow Start (Post-Commit)

**What runs:** For each matching `WorkflowTrigger` on the entity, the runtime writes a record to the `workflow_outbox` table. A separate goroutine (the outbox worker) reads pending records and dispatches to Temporal with exponential backoff retry.

**Transaction state:** OUTSIDE TX. The workflow dispatch is decoupled from the mutation transaction.

**Guaranteed semantics:** If the entity TX committed, the workflow outbox record exists. The outbox worker MUST eventually dispatch the workflow. "At least once" delivery — the workflow MUST be idempotent.

**Failure handling:** If Temporal is unreachable, the record remains in the outbox with `status = 'pending'`. The outbox worker retries with backoff: 1s, 2s, 4s, 8s, up to 5 minutes, for up to 24 hours. After 24 hours, the record is marked `status = 'failed'` and an alert is raised.

See [`08-workflow/OUTBOX_SPEC.md`](../08-workflow/OUTBOX_SPEC.md).

---

## 3. Transaction Boundary and Ownership

**Transaction ownership (ADR-013):** The service layer (`api/service.EntityService`) orchestrates database transactions for entity mutations. Transactions are opened via `repo.WithTx()`. The driver (`contrib/pgx`) provides `WithTx()` but does NOT open transactions automatically for individual `Create`, `Update`, or `Delete` calls — those are auto-commit unless the service wraps them.

The actual service execution sequence:

```
EntityService.Create(ctx, data, actor):

  pipeline.RunBeforeCreate(&CreateContext)          // OUTSIDE TX
    ├── ApplyDefaults, NamingSeries
    ├── BeforeValidate hooks
    ├── VALIDATE
    ├── BeforeSave hooks
    └── BeforeCreate hooks

  repo.WithTx(ctx, func(txCtx context.Context) error {
    // [TX begins — driver opens pgx transaction, stores in ctx]
    // setTenantContext(txCtx) called inside WithTx setup

    repo.Create(txCtx, CreateInput)                // PERSIST — inside TX
    pipeline.RunAuditRecord(txCtx, created, ...)   // AUDIT RECORD — inside TX
    return pipeline.RunAfterCreate(txCtx, created) // after_save hooks — inside TX
  })
  // [TX commits if callback returns nil; rolls back on any error]

  startWorkflows(ctx, OnCreate, created, actor)    // OUTSIDE TX
```

The `context.Context` carries the active transaction connection (set by `repo.WithTx` via `awo/tx.WithConn`). Any database operation inside the callback that extracts the connection from `ctx` automatically participates in the same transaction.

The transaction boundary from the pipeline stage perspective:

```
TX begins:      inside repo.WithTx callback, before repo.Create
TX commits:     when repo.WithTx callback returns nil
TX rolls back:  when any operation inside the callback returns an error
```

**Consequence for wrappers:** A wrapper implementing `driver.EntityRepository[T]` that calls the inner driver's `Create()` and then performs additional writes executes those writes after `repo.Create()` returns — either inside or outside the `WithTx` callback depending on where the wrapper is called. Wrapper-based audit integration is unreliable. See ADR-013 and ADR-014.

Hooks that run before `before_save` (inclusive) MUST NOT assume a TX is open.
Hooks that run from PERSIST onwards (AUDIT RECORD, after_save) run inside the `repo.WithTx`-managed TX.

---

## 4. Hook Recursion Policy (ADR-010)

The pipeline runtime maintains a per-goroutine entity name stack via `context.WithValue`. If a hook on entity `A` triggers a Create/Update/Delete on entity `A` through any code path, the runtime detects the recursion and panics:

```
PANIC: hook recursion detected: finance_journal_entry hook triggered Create on finance_journal_entry
Context stack: [finance_invoice → finance_journal_entry → finance_journal_entry]
```

Cross-entity operations from hooks are explicitly allowed. A hook on `finance_invoice` MAY call Create on `finance_journal_entry`. A hook on `finance_journal_entry` MUST NOT call Create on `finance_journal_entry`.

This panic is intentional and catches programming errors at development time.

---

## 5. Error Propagation

| Stage | Abort Error Type | HTTP Status |
|-------|-----------------|------------|
| ASSEMBLE | (parse error) | 400 |
| before_validate | `*ValidationError` | 422 |
| VALIDATE | `*ValidationError` | 422 |
| AUTHORIZE | (deny) | 403 |
| before_save | `*BusinessError` | Per status field |
| PERSIST | (constraint violation → `*BusinessError`) | 409 / 400 |
| AUDIT RECORD | (write failure) | 500 + rollback |
| after_save | (any error) | 500 + rollback |
| Workflow start | (none — async, outbox handles retry) | N/A |

---

## 6. Normative Requirements

- The pipeline MUST execute stages in the exact order specified above.
- The TX MUST begin before PERSIST and MUST commit after after_save.
- The AUDIT RECORD stage MUST execute within the TX.
- Workflow starts MUST execute outside the TX, after the TX commits.
- Hook recursion on the same entity MUST cause a runtime panic.
- The AUTHORIZE stage MUST NOT be skipped except for platform admin actors.
- The before_validate stage MUST NOT have an open TX.
- All validation errors MUST be collected before returning (not fail-on-first).

---

## References

- `awo/runtime/pipeline.go` — Implementation
- [`02-pipeline/HOOK_CONTRACT.md`](HOOK_CONTRACT.md) — Hook interface signatures
- [`02-pipeline/ERROR_MODEL.md`](ERROR_MODEL.md) — Error types and HTTP mapping
- [`12-audit/AUDIT_SPEC.md`](../12-audit/AUDIT_SPEC.md) — AuditRecord schema and AuditWriter interface
- [`12-audit/AUDIT_ARCH.md`](../12-audit/AUDIT_ARCH.md) — Full audit architecture
- [`08-workflow/OUTBOX_SPEC.md`](../08-workflow/OUTBOX_SPEC.md) — Workflow outbox
- ADR-013 in [`00-overview/DECISION_REGISTER.md`](../00-overview/DECISION_REGISTER.md) — Transaction ownership
- ADR-014 in [`00-overview/DECISION_REGISTER.md`](../00-overview/DECISION_REGISTER.md) — Audit integration point
