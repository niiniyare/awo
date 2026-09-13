> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Inventory: FIFO Valuation and COGS"
id: mod-018
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors]
since: "1.0"
normative-level: normative
related:
  - "[Inventory Patterns](inventory-patterns.md)"
  - "[Finance Patterns](finance-patterns.md)"
  - "[Transactions](../05-persistence/transactions.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Inventory: FIFO Valuation and COGS

**MOD-018 | Status: Accepted | Stability: Stable**

First-In First-Out (FIFO) inventory valuation, cost of goods sold (COGS) calculation, and GL journal posting for inventory movements.

---

## 1. FIFO Principle

FIFO assumes the oldest stock is sold first. When a stock move exits inventory:

1. Find the oldest unexhausted stock receipt layers for the product
2. Consume layers in order, oldest first
3. COGS = sum of (cost_per_unit × quantity) across consumed layers
4. Update `quantity_remaining` on each consumed layer

---

## 2. Stock Valuation Layer Entity

```go
var StockValuationLayerDefinition = def.SystemDefinition{
    Name:        "inventory_stock_valuation_layer",
    Module:      "inventory",
    Label:       "Stock Valuation Layer",
    Fields: []def.FieldDef{
        {Name: "product",            Type: def.FieldLink,     LinkTarget: "inventory_product", Required: true, Immutable: true},
        {Name: "location",           Type: def.FieldLink,     LinkTarget: "inventory_location", Required: true, Immutable: true},
        {Name: "stock_move",         Type: def.FieldLink,     LinkTarget: "inventory_stock_move", Required: true, Immutable: true},
        {Name: "quantity",           Type: def.FieldCurrency, Required: true, Immutable: true},  // original quantity received
        {Name: "quantity_remaining", Type: def.FieldCurrency, Required: true},                   // decremented on consumption
        {Name: "cost_per_unit",      Type: def.FieldCurrency, Required: true, Immutable: true},
        {Name: "total_value",        Type: def.FieldCurrency, Required: true, Immutable: true},  // quantity × cost_per_unit
        {Name: "received_at",        Type: def.FieldDateTime, Required: true, Immutable: true},
        {Name: "exhausted_at",       Type: def.FieldDateTime},                                   // set when quantity_remaining = 0
    },
    Permissions: def.PermissionSet{
        Read:   []string{"role:inventory.viewer", "role:tenant.admin"},
        Create: []string{"role:system.internal_only"},
        Write:  []string{"role:system.internal_only"},
        Delete: []string{},  // never delete valuation layers — financial audit trail
    },
}
```

---

## 3. Posting a Stock Receipt (Incoming)

When inventory arrives (purchase receipt, production output):

```go
func (a *InventoryActivities) PostStockReceiptActivity(ctx context.Context, input StockReceiptInput) error {
    tenantCtx, _ := a.TenantStore.SetTenantContext(ctx, input.TenantID)

    return a.StockMoveRepo.WithTx(tenantCtx, func(txCtx context.Context, txRepo def.EntityRepository[StockMove]) error {
        // 1. Create stock move
        move, err := txRepo.Create(txCtx, def.CreateInput{
            Fields: map[string]any{
                "product":      input.ProductID,
                "from_location": "virtual_supplier",
                "to_location":  input.LocationID,
                "quantity":     input.Quantity,
                "unit_cost":    input.UnitCost,
                "reference":    input.PurchaseOrderRef,
                "move_type":    "receipt",
            },
        })
        if err != nil {
            return fmt.Errorf("PostStockReceiptActivity: create move: %w", err)
        }

        // 2. Create valuation layer
        _, err = a.ValuationLayerRepo.Create(txCtx, def.CreateInput{
            Fields: map[string]any{
                "product":            input.ProductID,
                "location":           input.LocationID,
                "stock_move":         move.ID,
                "quantity":           input.Quantity,
                "quantity_remaining": input.Quantity,
                "cost_per_unit":      input.UnitCost,
                "total_value":        input.Quantity.Mul(input.UnitCost),
                "received_at":        time.Now().UTC(),
            },
        })
        if err != nil {
            return fmt.Errorf("PostStockReceiptActivity: create layer: %w", err)
        }

        // 3. Post inventory GL journal
        return a.postInventoryJournal(txCtx, move, "debit_inventory", "credit_accounts_payable")
    })
}
```

---

## 4. Posting a Stock Issue (Outgoing / Sale)

FIFO consumption on stock exit:

```go
func (a *InventoryActivities) PostStockIssueActivity(ctx context.Context, input StockIssueInput) error {
    tenantCtx, _ := a.TenantStore.SetTenantContext(ctx, input.TenantID)

    return a.StockMoveRepo.WithTx(tenantCtx, func(txCtx context.Context, txRepo def.EntityRepository[StockMove]) error {
        // 1. Fetch FIFO layers — oldest first
        layers, _, err := a.ValuationLayerRepo.Query(txCtx,
            filter.And(
                filter.Eq("product", input.ProductID),
                filter.Eq("location", input.FromLocationID),
                filter.Gt("quantity_remaining", decimal.Zero),
            ),
            def.WithSort("received_at", "asc"),
        )
        if err != nil {
            return fmt.Errorf("PostStockIssueActivity: query layers: %w", err)
        }

        // 2. Consume layers FIFO
        remaining := input.Quantity
        var totalCOGS decimal.Decimal

        for _, layer := range layers {
            if remaining.IsZero() {
                break
            }

            consume := decimal.Min(remaining, layer.QuantityRemaining)
            cogs := consume.Mul(layer.CostPerUnit)
            totalCOGS = totalCOGS.Add(cogs)
            remaining = remaining.Sub(consume)

            newRemaining := layer.QuantityRemaining.Sub(consume)
            updateFields := map[string]any{"quantity_remaining": newRemaining}
            if newRemaining.IsZero() {
                updateFields["exhausted_at"] = time.Now().UTC()
            }

            if _, err = a.ValuationLayerRepo.Update(txCtx, layer.ID, def.UpdateInput{
                Fields: updateFields,
            }); err != nil {
                return fmt.Errorf("PostStockIssueActivity: update layer %s: %w", layer.ID, err)
            }
        }

        if !remaining.IsZero() {
            return &errors.BusinessError{
                Code:    "inventory.insufficient_stock",
                Message: fmt.Sprintf("Insufficient stock: %.4f units short", remaining),
                Status:  409,
            }
        }

        // 3. Create stock move
        move, err := txRepo.Create(txCtx, def.CreateInput{
            Fields: map[string]any{
                "product":       input.ProductID,
                "from_location": input.FromLocationID,
                "to_location":   "virtual_customer",
                "quantity":      input.Quantity,
                "cogs_value":    totalCOGS,
                "sale_ref":      input.SaleOrderRef,
                "move_type":     "issue",
            },
        })
        if err != nil {
            return fmt.Errorf("PostStockIssueActivity: create move: %w", err)
        }

        // 4. Post COGS GL journal: DR COGS / CR Inventory
        return a.postCOGSJournal(txCtx, move, totalCOGS)
    })
}
```

---

## 5. Current Inventory Value

```go
// AggregateSpec for current inventory value per product
result, err := a.ValuationLayerRepo.Aggregate(ctx,
    filter.And(
        filter.Eq("product", productID),
        filter.IsNull("exhausted_at"),
    ),
    def.AggregateSpec{
        Sums: []string{"total_value"},
        // quantity_remaining × cost_per_unit is more accurate but requires join
        // using total_value with quantity_remaining fraction handled separately
    },
)
```

For exact current value:

```sql
-- Raw SQL via migration role — reporting only, not business logic
SELECT
    product,
    SUM(quantity_remaining * cost_per_unit) AS current_value,
    SUM(quantity_remaining) AS current_quantity
FROM inventory_stock_valuation_layer
WHERE exhausted_at IS NULL
  AND tenant_id = current_tenant_id()
GROUP BY product;
```

---

## 6. COGS GL Journal

```go
func (a *InventoryActivities) postCOGSJournal(ctx context.Context, move StockMove, cogsValue decimal.Decimal) error {
    _, err := a.JournalEntryRepo.Create(ctx, def.CreateInput{
        Fields: map[string]any{
            "reference":   fmt.Sprintf("COGS-%s", move.ID),
            "description": fmt.Sprintf("COGS for stock move %s", move.ID),
            "lines": []map[string]any{
                {
                    "account": "5000", // COGS account
                    "debit":   cogsValue,
                    "credit":  decimal.Zero,
                    "narration": fmt.Sprintf("Cost of goods sold: %s", move.SaleRef),
                },
                {
                    "account": "1300", // Inventory asset account
                    "debit":   decimal.Zero,
                    "credit":  cogsValue,
                    "narration": fmt.Sprintf("Inventory reduction: %s", move.SaleRef),
                },
            },
        },
    })
    return err
}
```

---

## 7. Inventory Reconciliation

Monthly reconciliation verifies FIFO layer totals match GL inventory account balance:

```go
// DailyReconciliationWorkflow activity — see scheduled-workflows.md
func (a *InventoryActivities) ReconcileStockValuationActivity(ctx context.Context, input ReconciliationInput) ([]Discrepancy, error) {
    tenantCtx, _ := a.TenantStore.SetTenantContext(ctx, input.TenantID)

    // FIFO total
    fifoResult, _ := a.ValuationLayerRepo.Aggregate(tenantCtx,
        filter.IsNull("exhausted_at"),
        def.AggregateSpec{Sums: []string{"quantity_remaining"}},
    )

    // GL inventory account balance
    glResult, _ := a.JournalLineRepo.Aggregate(tenantCtx,
        filter.Eq("account", "1300"),
        def.AggregateSpec{Sums: []string{"debit", "credit"}},
    )

    glBalance := glResult.Sums["debit"].Sub(glResult.Sums["credit"])
    // fifoValue != glBalance → discrepancy → alert
    // ...
}
```

---

## Related Documents

- [Inventory Patterns](inventory-patterns.md) — stock move entity, location model, lot tracking
- [Finance Patterns](finance-patterns.md) — GL journal entries
- [Transactions](../05-persistence/transactions.md) — WithTx usage for atomic stock + layer updates
- [Scheduled Workflows](../09-workflow/scheduled-workflows.md) — daily reconciliation
