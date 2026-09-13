> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Inventory Module Patterns"
id: mod-006
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors]
since: "1.0"
normative-level: informative
related:
  - "[Business Module Catalog](business-modules.md)"
  - "[Finance Patterns](finance-patterns.md)"
  - "[Hooks](../04-domain/hooks.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Inventory Module Patterns

**MOD-006 | Status: Accepted | Stability: Stable**

Patterns for stock movement, valuation, lot tracking, and warehouse operations in the Inventory module.

---

## 1. Stock On Hand — Computed from Moves

Stock on hand is never stored as a mutable field. It is always computed as the sum of movements into a location minus movements out:

```sql
-- Stock on hand for a product at a location
SELECT
    product_id,
    location_id,
    SUM(CASE WHEN location_dest_id = location_id THEN qty_done ELSE 0 END) -
    SUM(CASE WHEN location_src_id  = location_id THEN qty_done ELSE 0 END) AS qty_on_hand
FROM inventory_stock_move
WHERE tenant_id = current_tenant_id()
  AND state = 'Done'
GROUP BY product_id, location_id;
```

This view is materialized and refreshed on every `inventory_stock_move` state transition to `Done`.

Module code accesses stock on hand via the service layer — not via direct SQL:

```go
qty, err := inventoryService.GetStockOnHand(ctx, productID, locationID)
```

---

## 2. Stock Move State Machine

```
Draft → Confirmed → Done
Draft → Cancelled
```

Only `Done` moves affect stock on hand. `Cancelled` moves are soft-deleted from the UI but retained for audit.

State transition hook:

```go
func (h *StockMoveTransitionGuard) BeforeSave(ctx context.Context, record *entity.EntityRecord) error {
    newState, _ := record.Fields["state"].(string)
    oldState, _ := record.Original["state"].(string)

    // Validate state transitions
    validTransitions := map[string][]string{
        "Draft":     {"Confirmed", "Cancelled"},
        "Confirmed": {"Done", "Cancelled"},
        "Done":      {},  // terminal
        "Cancelled": {},  // terminal
    }

    allowed, ok := validTransitions[oldState]
    if !ok {
        return nil  // new record
    }

    for _, s := range allowed {
        if s == newState {
            return nil
        }
    }

    return &errors.BusinessError{
        Code:    "inventory.invalid_transition",
        Message: fmt.Sprintf("Cannot transition stock move from %s to %s.", oldState, newState),
        Status:  400,
    }
}
```

---

## 3. FIFO Valuation

When a stock move is set to `Done`, the unit cost is computed using FIFO from the existing stock layers:

```go
func (a *InventoryActivities) PostStockValuationActivity(ctx context.Context, input StockMoveInput) error {
    // Idempotency check
    exists, err := a.ValuationRepo.Exists(ctx, filter.Eq("stock_move_id", input.MoveID))
    if err != nil { return err }
    if exists { return nil }

    move, err := a.MoveRepo.Get(ctx, input.MoveID)
    if err != nil { return err }

    // Fetch FIFO layers for this product (oldest first)
    layers, _, err := a.ValuationRepo.Query(ctx,
        filter.And(
            filter.Eq("product_id", move.ProductID),
            filter.Eq("location_id", move.LocationSrcID),
            filter.Gt("qty_remaining", "0"),
        ),
        entity.OrderBy("created_at", entity.Asc),
    )
    if err != nil { return err }

    // Consume layers FIFO
    remaining := move.QtyDone
    totalCost := decimal.Zero
    for _, layer := range layers {
        if remaining.IsZero() { break }
        consumed := decimal.Min(remaining, layer.QtyRemaining)
        totalCost = totalCost.Add(consumed.Mul(layer.UnitCost))
        remaining = remaining.Sub(consumed)

        // Update layer remaining quantity
        _, err = a.ValuationRepo.Update(ctx, layer.ID, entity.UpdateInput{
            Fields: map[string]any{"qty_remaining": layer.QtyRemaining.Sub(consumed)},
        })
        if err != nil { return err }
    }

    unitCost := decimal.Zero
    if move.QtyDone.IsPositive() {
        unitCost = totalCost.Div(move.QtyDone)
    }

    // Create new valuation layer at destination
    _, err = a.ValuationRepo.Create(ctx, entity.CreateInput{
        Fields: map[string]any{
            "stock_move_id": move.ID,
            "product_id":    move.ProductID,
            "location_id":   move.LocationDestID,
            "qty_done":      move.QtyDone,
            "qty_remaining": move.QtyDone,
            "unit_cost":     unitCost,
        },
    })
    return err
}
```

---

## 4. Lot / Serial Number Tracking

For products requiring traceability (pharmaceuticals, electronics, food):

```go
// inventory_lot entity
{Name: "product_id",   Type: def.FieldLink, LinkTarget: "inventory_product", Required: true},
{Name: "lot_number",   Type: def.FieldData, Required: true, Unique: true},
{Name: "expiry_date",  Type: def.FieldDate},
{Name: "manufacture_date", Type: def.FieldDate},
{Name: "qty_on_hand",  Type: def.FieldCurrency},  // computed field
```

Stock moves with lot tracking include `lot_id` reference. Receiving without a lot number on a lot-tracked product is rejected:

```go
func (h *LotTrackingGuard) BeforeCreate(ctx context.Context, record *entity.EntityRecord) error {
    productID := record.Fields["product_id"].(uuid.UUID)

    product, err := h.ProductRepo.Get(ctx, productID)
    if err != nil { return err }

    if product.TrackingMode == "Lot" && record.Fields["lot_id"] == nil {
        return &errors.ValidationError{
            Fields: map[string]string{"lot_id": "Lot number required for this product."},
        }
    }
    return nil
}
```

---

## 5. Reorder Point Alerts

A scheduled Temporal workflow checks stock levels against reorder points:

```go
// Scheduled daily at 06:00 EAT
func StockReorderCheckWorkflow(ctx workflow.Context, input ReorderCheckInput) error {
    ao := workflow.ActivityOptions{StartToCloseTimeout: 5 * time.Minute}
    ctx = workflow.WithActivityOptions(ctx, ao)

    var activities *InventoryActivities

    // Get all products below reorder point for this tenant
    return workflow.ExecuteActivity(ctx, activities.CheckAndAlertReorderPointsActivity, input).Get(ctx, nil)
}

func (a *InventoryActivities) CheckAndAlertReorderPointsActivity(ctx context.Context, input ReorderCheckInput) error {
    tenantCtx, err := a.TenantStore.SetTenantContext(ctx, input.TenantID)
    if err != nil { return err }

    // Products where qty_on_hand < reorder_point
    // Uses a DB view that joins inventory_product with computed stock levels
    lowStock, _, err := a.ProductRepo.Query(tenantCtx,
        filter.Lt("qty_on_hand", "reorder_point"),
    )
    if err != nil { return err }

    for _, product := range lowStock {
        err := a.NotificationService.SendLowStockAlert(tenantCtx, product)
        if err != nil {
            // Non-fatal: log and continue with other products
            slog.Error("failed to send low stock alert",
                "product_id", product.ID,
                "err", err,
            )
        }
    }
    return nil
}
```

Background jobs MUST call `SetTenantContext` before any repository operation (see multi-tenancy-patterns.md §3).

---

## 6. Finance Integration — COGS Journal Entry

When a sales delivery (stock move from warehouse to customer) is confirmed, a COGS ledger entry is posted:

```go
func (a *InventoryActivities) PostCOGSEntryActivity(ctx context.Context, input StockMoveInput) error {
    move, err := a.MoveRepo.Get(ctx, input.MoveID)
    if err != nil { return err }

    // Only for outgoing moves (warehouse → customer)
    if move.MoveType != "Outgoing" { return nil }

    unitCost, err := a.ValuationService.GetUnitCost(ctx, move.ProductID, move.LocationSrcID)
    if err != nil { return err }

    cogsAmount := unitCost.Mul(move.QtyDone)

    return a.FinanceService.PostJournalEntry(ctx, finance.JournalEntryInput{
        TenantID:    input.TenantID,
        Date:        move.DoneAt,
        Reference:   fmt.Sprintf("COGS/%s", move.Reference),
        Description: fmt.Sprintf("Cost of goods sold: %s", move.ProductName),
        Lines: []finance.JournalLine{
            {AccountCode: "5000", Debit:  cogsAmount, Credit: decimal.Zero},   // COGS expense
            {AccountCode: "1200", Debit:  decimal.Zero, Credit: cogsAmount},   // Inventory asset
        },
    })
}
```

Cross-module service calls (Inventory → Finance) go through the Finance service interface — Inventory does not import Finance repositories directly.

---

## Related Documents

- [Business Module Catalog](business-modules.md) — Inventory entity list
- [Finance Patterns](finance-patterns.md) — journal entry posting
- [Multi-Tenancy Patterns](../06-tenancy/multi-tenancy-patterns.md) — background job tenant context
- [Signal Patterns](../09-workflow/signal-patterns.md) — workflow patterns
