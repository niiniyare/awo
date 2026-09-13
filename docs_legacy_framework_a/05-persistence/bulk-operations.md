> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Bulk Repository Operations"
id: pers-012
status: accepted
category: SPEC
stability: STABLE
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[Entity Repository](entity-repository.md)"
  - "[Transactions](transactions.md)"
  - "[Bulk Operations API](../11-api/bulk-operations.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Bulk Repository Operations

**PERS-012 | Status: Accepted | Stability: Stable**

`BulkCreate` and `BulkUpdate` repository methods: semantics, performance, atomicity, and hook execution.

---

## 1. BulkCreate

```go
type EntityRepository[T Entity] interface {
    BulkCreate(ctx context.Context, inputs []CreateInput) ([]T, error)
    // ...
}
```

### Semantics

- **Atomic**: all records are created in a single transaction. If any fails, all are rolled back.
- **No partial success**: BulkCreate is all-or-nothing.
- **Hooks**: `before_validate` and `before_save` hooks run for each record. The first hook failure aborts the entire batch.
- **`after_save` hooks**: run for each record inside the transaction. One `after_save` failure rolls back all records.
- **NamingSeries**: assigned atomically for each record within the batch transaction. Sequence numbers are contiguous.
- **Max batch size**: 500 records. Larger batches must be split.

### Usage

```go
inputs := make([]def.CreateInput, len(rows))
for i, row := range rows {
    inputs[i] = def.CreateInput{
        Fields: map[string]any{
            "product":       row.ProductID,
            "quantity":      row.Quantity,
            "from_location": row.FromLocation,
            "to_location":   row.ToLocation,
        },
    }
}

created, err := stockMoveRepo.BulkCreate(ctx, inputs)
if err != nil {
    return fmt.Errorf("createStockMoves: %w", err)
}
```

### SQL Implementation

```sql
INSERT INTO stock_move (id, tenant_id, product, quantity, from_location, to_location, created_at, updated_at)
VALUES
    ($1, $tenant, $2, $3, $4, $5, now(), now()),
    ($6, $tenant, $7, $8, $9, $10, now(), now()),
    -- ...
RETURNING *;
```

Single `INSERT ... VALUES (...)` statement for the entire batch — efficient for large batches.

---

## 2. BulkUpdate

```go
BulkUpdate(ctx context.Context, f Filter, patch Patch) (int, error)
```

### Semantics

- **Atomic**: all matching records are updated in a single `UPDATE ... WHERE ...` statement.
- **Filter-based**: targets all records matching the filter, not a list of IDs.
- **Patch**: a map of field names to new values. All matched records get the same patch.
- **Hooks**: `before_save` and `after_save` hooks do NOT run for BulkUpdate (performance).
- **Returns**: count of updated records.

### Usage

```go
// Mark all Draft invoices for customer X as Cancelled
count, err := invoiceRepo.BulkUpdate(ctx,
    filter.And(
        filter.Eq("customer", customerID),
        filter.Eq("status", "Draft"),
    ),
    def.Patch{
        "status":       "Cancelled",
        "cancelled_at": time.Now().UTC(),
    },
)
if err != nil {
    return fmt.Errorf("cancelDraftInvoices: %w", err)
}
slog.Info("cancelled invoices", "count", count, "customer", customerID)
```

### SQL Implementation

```sql
UPDATE finance_invoice
SET status = $1, cancelled_at = $2, updated_at = now()
WHERE tenant_id = current_tenant_id()
  AND customer = $3
  AND status = $4
```

RLS (`tenant_id = current_tenant_id()`) is always enforced.

---

## 3. When to Use Bulk vs Loop

| Scenario | Use |
|---|---|
| Creating 5+ records of the same entity | `BulkCreate` |
| Updating same field on 5+ records | `BulkUpdate` |
| Creating 1-4 records | Individual `Create` |
| Records need different patches | Individual `Update` in loop |
| Records need `after_save` hooks to run | Individual `Create`/`Update` |
| Records need unique NamingSeries | `BulkCreate` (assigns contiguous sequence) |

---

## 4. Bulk Delete

There is no `BulkDelete` method. Delete operations are high-risk and always require individual authorization checks. For soft-delete patterns:

```go
// Soft delete via BulkUpdate
count, err := repo.BulkUpdate(ctx,
    filter.And(
        filter.Eq("status", "Draft"),
        filter.LtEq("created_at", cutoffDate),
    ),
    def.Patch{"deleted_at": time.Now().UTC()},
)
```

For hard deletes of multiple records, loop and use individual `Delete` calls — the authorization check and cascade logic run per record.

---

## 5. Bulk Import Pattern

For large imports (1000+ records) from CSV or external API:

1. Use `BulkCreate` in batches of 500
2. Run in a Temporal activity (not a request handler)
3. Use activity heartbeat to track progress
4. Return an import report (success count, error count, error details)

```go
func (a *Activities) ImportStockMovesActivity(ctx context.Context, input ImportInput) (ImportResult, error) {
    rows, err := a.parseCSV(input.FileURL)
    if err != nil {
        return ImportResult{}, fmt.Errorf("ImportStockMovesActivity: parse: %w", err)
    }

    const batchSize = 500
    var result ImportResult

    for i := 0; i < len(rows); i += batchSize {
        activity.RecordHeartbeat(ctx, fmt.Sprintf("Processing rows %d-%d", i, min(i+batchSize, len(rows))))

        batch := rows[i:min(i+batchSize, len(rows))]
        inputs := make([]def.CreateInput, len(batch))
        for j, row := range batch {
            inputs[j] = def.CreateInput{Fields: rowToFields(row)}
        }

        _, err := a.StockMoveRepo.BulkCreate(ctx, inputs)
        if err != nil {
            result.Errors = append(result.Errors, ImportError{
                RowStart: i, RowEnd: i + len(batch), Error: err.Error(),
            })
            continue
        }
        result.SuccessCount += len(batch)
    }

    return result, nil
}
```

---

## Related Documents

- [Entity Repository](entity-repository.md) — full interface including BulkCreate, BulkUpdate
- [Transactions](transactions.md) — BulkCreate uses a single transaction
- [Bulk Operations API](../11-api/bulk-operations.md) — API layer bulk endpoints
- [Workflow Error Handling](../09-workflow/error-handling.md) — heartbeat for long-running imports
