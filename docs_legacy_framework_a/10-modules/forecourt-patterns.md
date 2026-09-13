> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Forecourt Module Patterns"
id: mod-008
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors]
since: "1.0"
normative-level: informative
related:
  - "[Finance Patterns](finance-patterns.md)"
  - "[Inventory Patterns](inventory-patterns.md)"
  - "[Business Module Catalog](business-modules.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Forecourt Module Patterns

**MOD-008 | Status: Accepted | Stability: Stable**

Patterns for fuel retail operations: shift management, pump meter readings, fuel inventory reconciliation, and daily settlement.

---

## 1. Core Entities

| Entity | Type | Purpose |
|---|---|---|
| `forecourt_shift` | System | Shift record: attendant, open/close time, expected vs actual cash |
| `forecourt_pump` | System | Pump definition: number, fuel type, active status |
| `forecourt_meter_reading` | System | Opening/closing totalizer per pump per shift |
| `forecourt_sale` | System | Individual fuel dispensing transaction |
| `forecourt_dip_reading` | Custom | Manual tank dip measurement for reconciliation |
| `forecourt_nozzle_test` | Custom | Nozzle calibration test record |

System entities (forecourt_shift, pump, meter_reading, sale) because they feed cash reconciliation and inventory accounting — financial integrity required.

---

## 2. Shift Lifecycle

```
OPEN → CLOSING → CLOSED
```

```go
var ShiftDefinition = def.SystemDefinition{
    Name:   "forecourt_shift",
    Module: "forecourt",
    Fields: []def.FieldDef{
        {Name: "shift_number",   Type: def.FieldNamingSeries, Series: "SHF-{YYYY}-{SEQ:6}"},
        {Name: "attendant",      Type: def.FieldLink, LinkTarget: "iam_user", Required: true},
        {Name: "station",        Type: def.FieldLink, LinkTarget: "inventory_location", Required: true},
        {Name: "opened_at",      Type: def.FieldDateTime, Immutable: true},
        {Name: "closed_at",      Type: def.FieldDateTime},
        {Name: "status",         Type: def.FieldSelect,
            Options: []string{"Open", "Closing", "Closed"}, Default: "Open"},
        {Name: "opening_cash",   Type: def.FieldCurrency, Required: true},
        {Name: "closing_cash",   Type: def.FieldCurrency},
        {Name: "expected_cash",  Type: def.FieldCurrency},  // computed on close
        {Name: "cash_variance",  Type: def.FieldCurrency},  // closing_cash - expected_cash
        {Name: "notes",          Type: def.FieldLongText},
    },
    Permissions: def.PermissionSet{
        Create: []string{"role:forecourt.supervisor", "role:tenant.admin"},
        Read:   []string{"role:forecourt.attendant", "role:forecourt.supervisor", "role:tenant.admin"},
        Write:  []string{"role:forecourt.supervisor", "role:tenant.admin"},
        Delete: []string{"role:tenant.admin"},
    },
}
```

---

## 3. Meter Reading Capture

Each pump has an opening and closing totalizer per shift. Volume sold = closing − opening.

```go
var MeterReadingDefinition = def.SystemDefinition{
    Name:   "forecourt_meter_reading",
    Module: "forecourt",
    Fields: []def.FieldDef{
        {Name: "shift",          Type: def.FieldLink, LinkTarget: "forecourt_shift",
            Required: true, Immutable: true},
        {Name: "pump",           Type: def.FieldLink, LinkTarget: "forecourt_pump",
            Required: true, Immutable: true},
        {Name: "reading_type",   Type: def.FieldSelect,
            Options: []string{"Opening", "Closing"}, Required: true, Immutable: true},
        {Name: "totalizer",      Type: def.FieldCurrency, Required: true},
            // Stored as decimal — totalizer values in litres with 4dp precision
        {Name: "recorded_at",    Type: def.FieldDateTime, Required: true},
        {Name: "recorded_by",    Type: def.FieldLink, LinkTarget: "iam_user"},
    },
    Hooks: def.HookSet{
        BeforeCreate: []def.BeforeCreateHook{&MeterReadingValidator{}},
    },
    Permissions: def.PermissionSet{
        Create: []string{"role:forecourt.attendant", "role:forecourt.supervisor", "role:tenant.admin"},
        Read:   []string{"role:forecourt.attendant", "role:forecourt.supervisor", "role:tenant.admin"},
        Write:  []string{"role:system.internal_only"},  // immutable once recorded
        Delete: []string{"role:tenant.admin"},
    },
}
```

### Meter Reading Validation Hook

```go
type MeterReadingValidator struct {
    Repo def.EntityRepository[MeterReading]
}

func (h *MeterReadingValidator) BeforeCreate(ctx context.Context, record *def.EntityRecord) error {
    pumpID, _ := record.Fields["pump"].(uuid.UUID)
    shiftID, _ := record.Fields["shift"].(uuid.UUID)
    readingType, _ := record.Fields["reading_type"].(string)

    // Prevent duplicate reading type per pump per shift
    exists, err := h.Repo.Exists(ctx, filter.And(
        filter.Eq("pump", pumpID),
        filter.Eq("shift", shiftID),
        filter.Eq("reading_type", readingType),
    ))
    if err != nil {
        return fmt.Errorf("MeterReadingValidator.BeforeCreate: check duplicate: %w", err)
    }
    if exists {
        return &def.ValidationError{
            Fields: map[string]string{
                "reading_type": fmt.Sprintf("%s reading already recorded for this pump and shift", readingType),
            },
        }
    }

    // Closing totalizer must be ≥ opening totalizer
    if readingType == "Closing" {
        opening, err := h.getOpeningTotalizer(ctx, pumpID, shiftID)
        if err != nil {
            return fmt.Errorf("MeterReadingValidator.BeforeCreate: get opening: %w", err)
        }
        closing, _ := record.Fields["totalizer"].(decimal.Decimal)
        if closing.LessThan(opening) {
            return &def.ValidationError{
                Fields: map[string]string{
                    "totalizer": "Closing totalizer cannot be less than opening totalizer",
                },
            }
        }
    }

    return nil
}

func (h *MeterReadingValidator) getOpeningTotalizer(ctx context.Context, pumpID, shiftID uuid.UUID) (decimal.Decimal, error) {
    readings, _, err := h.Repo.Query(ctx, filter.And(
        filter.Eq("pump", pumpID),
        filter.Eq("shift", shiftID),
        filter.Eq("reading_type", "Opening"),
    ))
    if err != nil {
        return decimal.Zero, fmt.Errorf("getOpeningTotalizer: %w", err)
    }
    if len(readings) == 0 {
        return decimal.Zero, &def.BusinessError{
            Code:    "forecourt.no_opening_reading",
            Message: "No opening reading found for this pump and shift",
            Status:  400,
        }
    }
    return readings[0].Fields["totalizer"].(decimal.Decimal), nil
}
```

---

## 4. Shift Close Action

```go
Actions: []def.ActionDef{
    {
        Name:        "close",
        Method:      def.ActionMethodPost,
        Label:       "Close Shift",
        Permission:  "role:forecourt.supervisor",
        HandlerFunc: CloseShiftAction,
    },
},
```

```go
func CloseShiftAction(ctx context.Context, action def.ActionContext) (*def.ActionResult, error) {
    shift, err := action.Repo.Get(ctx, action.RecordID)
    if err != nil {
        return nil, fmt.Errorf("CloseShiftAction: get shift: %w", err)
    }

    status, _ := shift.Fields["status"].(string)
    if status != "Open" {
        return nil, &def.BusinessError{
            Code:    "forecourt.shift_not_open",
            Message: "Only open shifts can be closed",
            Status:  409,
        }
    }

    // Compute expected cash from meter readings × fuel prices
    expectedCash, err := computeExpectedCash(ctx, action.RecordID, action.Services)
    if err != nil {
        return nil, fmt.Errorf("CloseShiftAction: compute expected cash: %w", err)
    }

    closingCash, _ := action.Input["closing_cash"].(decimal.Decimal)
    variance := closingCash.Sub(expectedCash)

    _, err = action.Repo.Update(ctx, action.RecordID, def.UpdateInput{
        Fields: map[string]any{
            "status":        "Closing",
            "closed_at":     time.Now().UTC(),
            "closing_cash":  closingCash,
            "expected_cash": expectedCash,
            "cash_variance": variance,
        },
    })
    if err != nil {
        return nil, fmt.Errorf("CloseShiftAction: update shift: %w", err)
    }

    // Trigger ShiftSettlementWorkflow — posts journal entries, updates inventory
    return &def.ActionResult{
        Message:    "Shift closing initiated",
        WorkflowID: fmt.Sprintf("%s.forecourt_shift.%s.on_close", shift.TenantID, action.RecordID),
    }, nil
}
```

---

## 5. Fuel Inventory Reconciliation

Fuel inventory uses stock moves (Inventory module) keyed to shift close:

```go
// In ShiftSettlementActivity (Temporal activity)
func (a *Activities) ReconcileFuelInventoryActivity(ctx context.Context, input ShiftSettlementInput) error {
    // For each pump in the shift:
    //   1. Get opening + closing meter readings
    //   2. Compute volume sold = closing - opening (litres)
    //   3. Create stock_move: OUT from fuel tank location
    //      quantity = volume sold, uom = "Litres", product = fuel product for pump

    pumps, err := a.PumpRepo.Query(ctx, filter.Eq("station", input.StationID))
    if err != nil {
        return fmt.Errorf("ReconcileFuelInventoryActivity: list pumps: %w", err)
    }

    for _, pump := range pumps {
        volumeSold, err := a.computeVolumeSold(ctx, pump.ID, input.ShiftID)
        if err != nil {
            return fmt.Errorf("ReconcileFuelInventoryActivity: volume for pump %s: %w", pump.ID, err)
        }
        if volumeSold.IsZero() {
            continue
        }

        _, err = a.StockMoveRepo.Create(ctx, def.CreateInput{
            Fields: map[string]any{
                "product":       pump.Fields["fuel_product"],
                "from_location": pump.Fields["tank_location"],
                "to_location":   "consumption",  // virtual consumption location
                "quantity":      volumeSold,
                "uom":           "Litres",
                "reference":     fmt.Sprintf("SHF/%s", input.ShiftNumber),
                "move_type":     "Out",
            },
        })
        if err != nil {
            return fmt.Errorf("ReconcileFuelInventoryActivity: create stock_move: %w", err)
        }
    }

    return nil
}
```

---

## 6. Daily Settlement Workflow

```go
func ShiftSettlementWorkflow(ctx workflow.Context, input ShiftSettlementInput) error {
    ao := workflow.ActivityOptions{
        StartToCloseTimeout: 5 * time.Minute,
        RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 3},
    }
    ctx = workflow.WithActivityOptions(ctx, ao)

    compensator := saga.NewCompensator()

    // Step 1: Reconcile fuel inventory (stock moves)
    err := workflow.ExecuteActivity(ctx, a.ReconcileFuelInventoryActivity, input).Get(ctx, nil)
    if err != nil {
        return fmt.Errorf("ShiftSettlementWorkflow: reconcile inventory: %w", err)
    }
    compensator.Add(func(ctx workflow.Context) error {
        return workflow.ExecuteActivity(ctx, a.ReverseInventoryReconciliationActivity, input).Get(ctx, nil)
    })

    // Step 2: Post revenue journal entries
    err = workflow.ExecuteActivity(ctx, a.PostShiftRevenueActivity, input).Get(ctx, nil)
    if err != nil {
        compensator.Compensate(ctx)
        return fmt.Errorf("ShiftSettlementWorkflow: post revenue: %w", err)
    }

    // Step 3: Mark shift as Closed
    err = workflow.ExecuteActivity(ctx, a.FinalizeShiftActivity, input).Get(ctx, nil)
    if err != nil {
        compensator.Compensate(ctx)
        return fmt.Errorf("ShiftSettlementWorkflow: finalize: %w", err)
    }

    return nil
}
```

---

## 7. Price Management

Fuel prices are managed as a system entity with effective dating:

```go
var FuelPriceDefinition = def.SystemDefinition{
    Name:   "forecourt_fuel_price",
    Module: "forecourt",
    Fields: []def.FieldDef{
        {Name: "fuel_type",      Type: def.FieldSelect,
            Options: []string{"Petrol", "Diesel", "Kerosene"}, Required: true},
        {Name: "price_per_litre", Type: def.FieldCurrency, Required: true},
        {Name: "effective_from", Type: def.FieldDateTime, Required: true},
        {Name: "effective_to",   Type: def.FieldDateTime},  // null = currently active
        {Name: "set_by",         Type: def.FieldLink, LinkTarget: "iam_user"},
    },
    Permissions: def.PermissionSet{
        Create: []string{"role:forecourt.supervisor", "role:tenant.admin"},
        Read:   []string{"role:forecourt.attendant", "role:forecourt.supervisor", "role:tenant.admin"},
        Write:  []string{"role:forecourt.supervisor", "role:tenant.admin"},
        Delete: []string{"role:tenant.admin"},
    },
}
```

Active price lookup:

```go
func GetActiveFuelPrice(ctx context.Context, repo def.EntityRepository[FuelPrice], fuelType string) (decimal.Decimal, error) {
    now := time.Now().UTC()
    prices, _, err := repo.Query(ctx, filter.And(
        filter.Eq("fuel_type", fuelType),
        filter.LtEq("effective_from", now),
        filter.Or(
            filter.IsNull("effective_to"),
            filter.GtEq("effective_to", now),
        ),
    ))
    if err != nil {
        return decimal.Zero, fmt.Errorf("GetActiveFuelPrice: %w", err)
    }
    if len(prices) == 0 {
        return decimal.Zero, &def.BusinessError{
            Code:    "forecourt.no_active_price",
            Message: fmt.Sprintf("No active price for fuel type: %s", fuelType),
            Status:  400,
        }
    }
    return prices[0].Fields["price_per_litre"].(decimal.Decimal), nil
}
```

---

## Related Documents

- [Finance Patterns](finance-patterns.md) — journal entry posting used in shift settlement
- [Inventory Patterns](inventory-patterns.md) — stock moves for fuel reconciliation
- [Business Module Catalog](business-modules.md) — forecourt module in context
- [Actions](../04-domain/actions.md) — CloseShiftAction pattern
