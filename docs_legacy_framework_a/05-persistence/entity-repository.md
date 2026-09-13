> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "EntityRepository"
id: pers-001
status: accepted
category: SPEC
stability: FROZEN
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[Filter DSL](filter-dsl.md)"
  - "[System Entities](system-entities.md)"
  - "[Custom Entities](custom-entities.md)"
  - "[Hooks](../04-domain/hooks.md)"
  - "[Tenancy Model](../06-tenancy/tenant-model.md)"
  - "[Architecture Laws](../02-architecture/laws.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# EntityRepository

**PERS-001 | Status: Accepted | Stability: Frozen**

This document is the normative specification of the `EntityRepository` interface — the sole mechanism through which all domain and application code accesses entity persistence.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## Table of Contents

1. [Interface Definition](#1-interface-definition)
2. [Read Methods](#2-read-methods)
3. [Write Methods](#3-write-methods)
4. [Transaction Methods](#4-transaction-methods)
5. [QueryOption](#5-queryoption)
6. [Input Types](#6-input-types)
7. [PageInfo](#7-pageinfo)
8. [AggregateSpec and AggregateResult](#8-aggregatespec-and-aggregateresult)
9. [Error Types](#9-error-types)
10. [Tenant Scoping](#10-tenant-scoping)
11. [Policy Function Integration](#11-policy-function-integration)
12. [Optimistic Locking](#12-optimistic-locking)
13. [Mock Implementation](#13-mock-implementation)

---

## 1. Interface Definition

```go
// EntityRepository is the typed interface for all entity persistence operations.
// All business logic and application code MUST interact with persistence
// exclusively through this interface.
//
// The interface is generic over the entity type T.
// T must implement the Entity interface.
//
// Stability: FROZEN — no new methods after v1.0.
// New capabilities via optional interface extensions (see LAW-009).
type EntityRepository[T Entity] interface {
    // Read operations
    Get(ctx context.Context, id uuid.UUID, opts ...QueryOption) (T, error)
    Query(ctx context.Context, f Filter, opts ...QueryOption) ([]T, PageInfo, error)
    Exists(ctx context.Context, f Filter) (bool, error)
    Count(ctx context.Context, f Filter) (int, error)
    Aggregate(ctx context.Context, f Filter, spec AggregateSpec) (AggregateResult, error)

    // Write operations
    Create(ctx context.Context, input CreateInput) (T, error)
    Update(ctx context.Context, id uuid.UUID, input UpdateInput) (T, error)
    Delete(ctx context.Context, id uuid.UUID) error
    BulkCreate(ctx context.Context, inputs []CreateInput) ([]T, error)
    BulkUpdate(ctx context.Context, f Filter, patch Patch) (int, error)

    // Transaction control
    WithTx(ctx context.Context, fn func(ctx context.Context, repo EntityRepository[T]) error) error
}
```

The interface is deliberately narrow. It excludes: raw SQL, ORM types, connection management, lazy loading, schema-specific predicates, and any operation that cannot be expressed through the domain types.

---

## 2. Read Methods

### Get

```go
Get(ctx context.Context, id uuid.UUID, opts ...QueryOption) (T, error)
```

Fetches a single record by UUID. Returns `NotFoundError` if:
- The record does not exist
- The record exists but is excluded by the entity's `PolicyFunc`
- The record exists but belongs to a different tenant

The two cases (does not exist / policy exclusion) are indistinguishable from the caller's perspective. This is intentional: distinguishing them would allow callers to enumerate records they cannot see.

**Edge loading:** Pass `WithEdge("edge_name")` options to load related records. Without explicit edge options, all edge fields are nil in the returned record.

### Query

```go
Query(ctx context.Context, f Filter, opts ...QueryOption) ([]T, PageInfo, error)
```

Fetches all records matching the Filter predicate, after applying the entity's PolicyFunc predicate (AND composed).

Returns an empty slice (not nil) when no records match. Never returns nil slice on success.

Options control: sort order, pagination, field selection, edge loading.

### Exists

```go
Exists(ctx context.Context, f Filter) (bool, error)
```

Returns `true` if any record satisfying the filter and PolicyFunc predicate exists. More efficient than `Count() > 0` for binary existence checks.

### Count

```go
Count(ctx context.Context, f Filter) (int, error)
```

Returns the count of records satisfying the filter and PolicyFunc predicate. Equivalent to `SELECT COUNT(*) WHERE filter AND policy_predicate`.

### Aggregate

```go
Aggregate(ctx context.Context, f Filter, spec AggregateSpec) (AggregateResult, error)
```

Computes an aggregate over matching records. Supported aggregate operations: SUM, COUNT, AVG, MIN, MAX. Always tenant-scoped and PolicyFunc-filtered. See [§8](#8-aggregatespec-and-aggregateresult).

---

## 3. Write Methods

### Create

```go
Create(ctx context.Context, input CreateInput) (T, error)
```

Creates a new entity record. Execution sequence:
1. `before_validate` hooks
2. Field validation (Required, Unique, type constraints, custom validators)
3. `before_create` and `before_save` hooks
4. BEGIN TRANSACTION
5. INSERT entity record
6. INSERT audit log entry
7. INSERT outbox entry (if WorkflowTriggers match)
8. `after_create` and `after_save` hooks (inside TX)
9. COMMIT
10. Outbox relay dispatches workflow (outside TX)

Returns the newly created record with system columns populated (`id`, `created_at`, etc.).

### Update

```go
Update(ctx context.Context, id uuid.UUID, input UpdateInput) (T, error)
```

Updates an existing record. The update is applied as a patch: only fields present in `UpdateInput.Fields` are changed. Fields absent from the input retain their current values.

Pre-update: fetches the record using `Get()` with the entity's PolicyFunc applied. Returns `NotFoundError` if the record is not visible to the actor.

Respects `Immutable` field constraint: rejects `UpdateInput` containing an Immutable field with a changed value.

### Delete

```go
Delete(ctx context.Context, id uuid.UUID) error
```

Deletes a record and all cascade-delete related records. Pre-delete: fetches the record via `Get()` for hook invocation. Returns `NotFoundError` if not visible.

Does not soft-delete. Records deleted via this method are permanently removed. Soft-delete behavior is implemented by adding a `deleted_at` DateTime field and using a PolicyFunc to exclude non-null `deleted_at` records.

### BulkCreate

```go
BulkCreate(ctx context.Context, inputs []CreateInput) ([]T, error)
```

Creates multiple records in a single database round-trip. Use instead of looping `Create` for batches larger than ~10 records.

Hooks are invoked per-record before the bulk INSERT (`before_validate`, `before_create`, `before_save`). The INSERT is a single multi-row statement. After-hooks (`after_create`, `after_save`) are invoked per-record after the INSERT, inside the transaction.

If any record fails validation, the entire batch is rejected before any INSERT occurs.

### BulkUpdate

```go
BulkUpdate(ctx context.Context, f Filter, patch Patch) (int, error)
```

Applies a field patch to all records matching the filter. Returns the count of updated records.

**Per-record hooks are not invoked for BulkUpdate.** Audit log entries are written for each affected record. Use BulkUpdate only where per-record hook behavior is not required; for hook-required bulk mutations, use `WithTx` and loop `Update`.

---

## 4. Transaction Methods

### WithTx

```go
WithTx(ctx context.Context, fn func(ctx context.Context, repo EntityRepository[T]) error) error
```

Executes `fn` within a database transaction. All repository operations performed by `fn` share the same transaction. Commits on nil return; rolls back on non-nil return.

**MUST NOT start Temporal workflows inside `WithTx`.** Temporal `StartWorkflow` calls must occur after the transaction commits. Use `ActionContext.TriggerWorkflow()` or the outbox relay. See [LAW-006](../02-architecture/laws.md#law-006-outbox-entry-and-entity-record-commit-atomically).

```go
err := invoiceRepo.WithTx(ctx, func(ctx context.Context, txRepo EntityRepository[Invoice]) error {
    invoice, err := txRepo.Create(ctx, invoiceInput)
    if err != nil {
        return err
    }
    for _, lineInput := range lineInputs {
        if _, err := lineRepo.Create(ctx, lineInput); err != nil {
            return err // causes rollback
        }
    }
    return nil // causes commit
})
```

---

## 5. QueryOption

QueryOptions modify `Get()` and `Query()` behavior:

```go
// Load an edge by name. The edge must be declared in EntityDefinition.Edges.
func WithEdge(edgeName string) QueryOption

// Sort by field. Ascending by default.
func OrderBy(fieldName string, direction SortDirection) QueryOption

// Cursor-based pagination.
func WithCursor(cursor string) QueryOption

// Limit number of returned records.
func Limit(n int) QueryOption

// Select specific fields only. Reduces data transfer.
func WithFields(fieldNames ...string) QueryOption

// Load a specific version (for optimistic locking).
func AtVersion(version int64) QueryOption
```

Multiple options compose: `repo.Query(ctx, f, OrderBy("created_at", Desc), Limit(50), WithEdge("customer"))`.

---

## 6. Input Types

### CreateInput

```go
type CreateInput struct {
    // Fields contains the values for the new record.
    // System columns (id, tenant_id, created_at, etc.) MUST NOT be included.
    Fields map[string]any

    // ID optionally pre-specifies the record UUID. If empty, a UUID v7 is generated.
    // Pre-specifying IDs is useful for idempotent operations.
    ID uuid.UUID
}
```

### UpdateInput

```go
type UpdateInput struct {
    // Fields contains the values to update. Only listed fields are changed.
    // Fields absent from this map retain their current values.
    Fields map[string]any

    // IfVersion enables optimistic locking. If non-zero, the update only proceeds
    // if the record's current version matches this value.
    // Returns OptimisticLockError if the version does not match.
    IfVersion int64
}
```

### Patch

```go
// Patch is used by BulkUpdate. Simpler than UpdateInput — no version checking.
type Patch struct {
    Fields map[string]any
}
```

---

## 7. PageInfo

```go
type PageInfo struct {
    // TotalCount is the total number of matching records (before pagination).
    TotalCount int

    // HasNextPage is true if there are more records after the current cursor.
    HasNextPage bool

    // HasPreviousPage is true if there are records before the current cursor.
    HasPreviousPage bool

    // StartCursor is the cursor for the first record in the current page.
    StartCursor string

    // EndCursor is the cursor for the last record in the current page.
    EndCursor string
}
```

Cursors are opaque strings. They encode sort key + record ID for stable pagination across mutations. Do not attempt to parse cursor values.

---

## 8. AggregateSpec and AggregateResult

```go
type AggregateSpec struct {
    Operation AggregateOp // Sum, Count, Avg, Min, Max
    Field     string      // field name to aggregate over
    GroupBy   []string    // optional; group results by these fields
}

type AggregateResult struct {
    // For ungrouped aggregates: single value
    Value decimal.Decimal

    // For grouped aggregates: one entry per group
    Groups []AggregateGroup
}

type AggregateGroup struct {
    Keys  map[string]any  // group key values
    Value decimal.Decimal
}
```

---

## 9. Error Types

| Error | HTTP Status | When returned |
|---|---|---|
| `NotFoundError` | 404 | Record not found or excluded by policy |
| `ValidationError` | 422 | Field-level constraint violation |
| `BusinessError` | varies | Domain rule violation (set by hook or store layer) |
| `OptimisticLockError` | 409 | IfVersion mismatch in UpdateInput |
| `PermissionError` | 403 | RBAC check failed (checked before repository) |

All errors implement `error` and are unwrapped using `errors.As`. Never use type switches on errors from repository methods.

---

## 10. Tenant Scoping

Every repository method requires a `context.Context` carrying a resolved [TenantContext](../GLOSSARY.md#tenantcontext). The repository implementation:
1. Extracts the TenantContext from ctx
2. Calls `set_tenant_context(tenant_uuid)` before executing any SQL
3. PostgreSQL RLS enforces tenant isolation at the database level

A context without a TenantContext causes the repository to return an error immediately. See [LAW-005](../02-architecture/laws.md#law-005-no-store-operation-without-tenant-context).

---

## 11. Policy Function Integration

The repository implementation automatically retrieves the entity's `PolicyFunc` from the `CompiledSchema` and invokes it with the current `ctx`. The returned filter predicate is ANDed with the caller-supplied filter before execution.

Module authors do not invoke `PolicyFunc` manually. It is applied transparently by the repository.

---

## 12. Optimistic Locking

System entities with high contention use optimistic locking to prevent lost updates:

```go
// Fetch with version
invoice, err := invoiceRepo.Get(ctx, invoiceID)
currentVersion := invoice.Version // int64 system column

// Update only if no concurrent modification
_, err = invoiceRepo.Update(ctx, invoiceID, entity.UpdateInput{
    Fields:    map[string]any{"status": "Approved"},
    IfVersion: currentVersion,
})
if errors.As(err, new(entity.OptimisticLockError)) {
    // Record was modified between Get and Update — retry or return conflict
}
```

Every System Entity has a `version bigint` system column incremented atomically on each update. Custom Entities do not support optimistic locking.

---

## 13. Mock Implementation

For unit testing domain code:

```go
type mockEntityRepository struct {
    records map[uuid.UUID]*entity.EntityRecord
    // Track calls for assertion
    createCalls []entity.CreateInput
    updateCalls []struct{ id uuid.UUID; input entity.UpdateInput }
}

func (m *mockEntityRepository) Get(ctx context.Context, id uuid.UUID, opts ...entity.QueryOption) (*entity.EntityRecord, error) {
    rec, ok := m.records[id]
    if !ok {
        return nil, &entity.NotFoundError{ID: id}
    }
    return rec, nil
}

// ... implement remaining methods
```

The mock satisfies `EntityRepository[T]`. Domain layer tests use the mock; integration tests use the real PostgreSQL-backed implementation.

---

## Related Documents

- [Filter DSL](filter-dsl.md) — the Filter type used in Query, Exists, Count, BulkUpdate
- [System Entities](system-entities.md) — SQL-backed entity persistence details
- [Custom Entities](custom-entities.md) — JSONB-backed entity persistence details
- [Tenancy Model](../06-tenancy/tenant-model.md) — set_tenant_context() and RLS
- [Hooks](../04-domain/hooks.md) — hooks invoked within Create/Update/Delete
- [Architecture Laws](../02-architecture/laws.md) — LAW-005, LAW-006
- [Glossary](../GLOSSARY.md) — EntityRepository, Filter, QueryOption, PageInfo, CreateInput, UpdateInput
