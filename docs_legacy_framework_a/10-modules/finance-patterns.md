> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Finance Module Patterns"
id: mod-004
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors]
since: "1.0"
normative-level: informative
related:
  - "[Business Module Catalog](business-modules.md)"
  - "[Fields](../04-domain/fields.md)"
  - "[Hooks](../04-domain/hooks.md)"
  - "[Sagas](../09-workflow/sagas.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Finance Module Patterns

**MOD-004 | Status: Accepted | Stability: Stable**

Patterns for financial integrity, double-entry accounting, VAT compliance, and payment processing in the Finance module.

---

## 1. Currency Field Usage

All monetary amounts MUST use `FieldCurrency` → `numeric(20,4)` → `decimal.Decimal`. Never `float64`.

```go
// CORRECT
{Name: "amount", Type: def.FieldCurrency, Required: true}
// Go type: decimal.Decimal
// SQL type: numeric(20,4)

// WRONG
{Name: "amount", Type: def.FieldFloat}  // float64 — precision loss on addition
```

Arithmetic on `decimal.Decimal`:

```go
import "github.com/shopspring/decimal"

subtotal := decimal.NewFromFloat(0)
for _, line := range lines {
    subtotal = subtotal.Add(line.Amount)
}
vat := subtotal.Mul(decimal.NewFromFloat(0.16))
total := subtotal.Add(vat)
```

Never convert `decimal.Decimal` to `float64` for intermediate calculations. Convert to `float64` only for display formatting if `String()` is insufficient.

---

## 2. Double-Entry Ledger Pattern

Direct writes to `finance_ledger_entry` are not permitted from business module code. Use the Finance service layer:

```go
// internal/core/finance/service.go

type JournalEntryInput struct {
    TenantID    uuid.UUID
    Date        time.Time
    Reference   string
    Description string
    Lines       []JournalLine
}

type JournalLine struct {
    AccountCode string
    Debit       decimal.Decimal
    Credit      decimal.Decimal
    CostCenter  string
}

func (s *FinanceService) PostJournalEntry(ctx context.Context, input JournalEntryInput) error {
    // Validate: sum(debits) == sum(credits)
    totalDebit  := decimal.Zero
    totalCredit := decimal.Zero
    for _, line := range input.Lines {
        totalDebit  = totalDebit.Add(line.Debit)
        totalCredit = totalCredit.Add(line.Credit)
    }
    if !totalDebit.Equal(totalCredit) {
        return &errors.BusinessError{
            Code:    "journal.unbalanced",
            Message: fmt.Sprintf("Debit %s ≠ Credit %s", totalDebit, totalCredit),
            Status:  400,
        }
    }

    // Persist as atomic transaction
    return s.JournalRepo.WithTx(ctx, func(ctx context.Context, repo entity.EntityRepository[Journal]) error {
        journal, err := repo.Create(ctx, entity.CreateInput{Fields: map[string]any{
            "date":        input.Date,
            "reference":   input.Reference,
            "description": input.Description,
            "status":      "Posted",
        }})
        if err != nil {
            return fmt.Errorf("PostJournalEntry: create journal: %w", err)
        }

        for _, line := range input.Lines {
            _, err := s.LedgerRepo.Create(ctx, entity.CreateInput{Fields: map[string]any{
                "journal_id":   journal.ID,
                "account_code": line.AccountCode,
                "debit":        line.Debit,
                "credit":       line.Credit,
                "cost_center":  line.CostCenter,
            }})
            if err != nil {
                return fmt.Errorf("PostJournalEntry: create ledger line: %w", err)
            }
        }
        return nil
    })
}
```

The `WithTx` wrapper ensures the journal header and all ledger lines commit atomically or all roll back.

---

## 3. Invoice Submission Workflow

The canonical finance workflow — invoice submission triggers payment processing and ledger posting:

```go
// workflows/invoice_submission.go

type InvoiceSubmissionInput struct {
    TenantID  uuid.UUID
    InvoiceID uuid.UUID
}

func InvoiceSubmissionWorkflow(ctx workflow.Context, input InvoiceSubmissionInput) error {
    saga := &workflow.SagaCompensator{}

    ao := workflow.ActivityOptions{
        StartToCloseTimeout: 60 * time.Second,
        RetryPolicy: &temporal.RetryPolicy{MaxAttempts: 3},
    }
    ctx = workflow.WithActivityOptions(ctx, ao)

    var activities *InvoiceActivities

    // Step 1: Validate credit limit
    err := workflow.ExecuteActivity(ctx, activities.CheckCreditLimitActivity, input).Get(ctx, nil)
    if err != nil {
        return fmt.Errorf("credit limit check: %w", err)
    }

    // Step 2: Post AR ledger entry
    err = workflow.ExecuteActivity(ctx, activities.PostARLedgerEntryActivity, input).Get(ctx, nil)
    if err != nil {
        return fmt.Errorf("post AR entry: %w", err)
    }
    saga.AddCompensation(func(ctx workflow.Context) error {
        return workflow.ExecuteActivity(ctx, activities.ReverseARLedgerEntryActivity, input).Get(ctx, nil)
    })

    // Step 3: Generate tax entry (eTIMS)
    err = workflow.ExecuteActivity(ctx, activities.GenerateTaxEntryActivity, input).Get(ctx, nil)
    if err != nil {
        _ = saga.Compensate(ctx)
        return fmt.Errorf("generate tax entry: %w", err)
    }

    // Step 4: Send invoice email
    err = workflow.ExecuteActivity(ctx, activities.SendInvoiceEmailActivity, input).Get(ctx, nil)
    if err != nil {
        // Email failure is non-fatal — log and continue
        workflow.GetLogger(ctx).Error("failed to send invoice email", "err", err)
    }

    return nil
}
```

---

## 4. VAT / Tax Entry Pattern

For KRA eTIMS compliance, every taxable transaction must produce a `finance_tax_entry`:

```go
func (a *InvoiceActivities) GenerateTaxEntryActivity(ctx context.Context, input InvoiceSubmissionInput) error {
    // Idempotency: check if tax entry already exists for this invoice
    exists, err := a.TaxEntryRepo.Exists(ctx, filter.Eq("invoice_id", input.InvoiceID))
    if err != nil {
        return err
    }
    if exists {
        return nil  // already created — idempotent
    }

    invoice, err := a.InvoiceRepo.Get(ctx, input.InvoiceID)
    if err != nil {
        return fmt.Errorf("GenerateTaxEntryActivity: get invoice: %w", err)
    }

    vatAmount := invoice.TotalKES.Mul(decimal.NewFromFloat(0.16 / 1.16))  // extract VAT from VAT-inclusive amount

    _, err = a.TaxEntryRepo.Create(ctx, entity.CreateInput{Fields: map[string]any{
        "invoice_id":  input.InvoiceID,
        "tax_type":    "VAT",
        "tax_rate":    "0.16",
        "tax_amount":  vatAmount,
        "etims_ref":   "",  // populated by eTIMS integration activity
        "status":      "Pending",
    }})
    return err
}
```

`finance_tax_entry` records are immutable after creation. The eTIMS reference is populated by a separate activity that calls the KRA API.

---

## 5. Payment Reconciliation

Matching payments to invoices:

```go
// Action: POST /api/v1/entities/finance_payment/{id}/reconcile
func ReconcilePaymentAction(ctx context.Context, action entity.ActionContext) (*entity.ActionResult, error) {
    var input struct {
        InvoiceIDs []uuid.UUID `json:"invoice_ids"`
    }
    if err := action.BindInput(&input); err != nil {
        return nil, err
    }

    payment, err := action.Repo.Get(ctx, action.RecordID)
    if err != nil {
        return nil, err
    }

    // Validate: sum of invoice outstanding amounts ≤ payment amount
    totalOutstanding := decimal.Zero
    for _, invoiceID := range input.InvoiceIDs {
        invoice, err := action.InvoiceRepo.Get(ctx, invoiceID)
        if err != nil {
            return nil, err
        }
        if invoice.Status != "Submitted" {
            return nil, &errors.BusinessError{
                Code:    "payment.invoice_not_submitted",
                Message: fmt.Sprintf("Invoice %s is not in Submitted status.", invoice.Number),
                Status:  400,
            }
        }
        totalOutstanding = totalOutstanding.Add(invoice.OutstandingAmount)
    }

    if payment.Amount.LessThan(totalOutstanding) {
        return nil, &errors.BusinessError{
            Code:    "payment.insufficient",
            Message: "Payment amount is less than total outstanding for selected invoices.",
            Status:  400,
        }
    }

    // Mark invoices Paid within a transaction
    err = action.Repo.WithTx(ctx, func(ctx context.Context, _ entity.EntityRepository[Payment]) error {
        for _, invoiceID := range input.InvoiceIDs {
            _, err := action.InvoiceRepo.Update(ctx, invoiceID, entity.UpdateInput{
                Fields: map[string]any{"status": "Paid", "payment_id": payment.ID},
            })
            if err != nil {
                return err
            }
        }
        return nil
    })
    if err != nil {
        return nil, err
    }

    return &entity.ActionResult{Message: "Payment reconciled successfully."}, nil
}
```

---

## 6. Period Locking

Prevent modifications to closed financial periods:

```go
// BeforeCreate hook on finance_ledger_entry
func (h *PeriodLockGuard) BeforeCreate(ctx context.Context, record *entity.EntityRecord) error {
    entryDate, _ := record.Fields["date"].(time.Time)

    locked, err := h.PeriodLockRepo.Exists(ctx,
        filter.And(
            filter.Lte("period_start", entryDate),
            filter.Gte("period_end", entryDate),
            filter.Eq("status", "Locked"),
        ),
    )
    if err != nil {
        return fmt.Errorf("PeriodLockGuard: check period: %w", err)
    }
    if locked {
        return &errors.BusinessError{
            Code:    "finance.period_locked",
            Message: fmt.Sprintf("The period containing %s is locked and cannot receive new entries.", entryDate.Format("2006-01-02")),
            Status:  409,
        }
    }
    return nil
}
```

---

## Related Documents

- [Business Module Catalog](business-modules.md) — Finance module entity list
- [Sagas](../09-workflow/sagas.md) — saga compensation pattern
- [Hooks](../04-domain/hooks.md) — `before_create`, `before_save`
- [Fields](../04-domain/fields.md) — `FieldCurrency` specification
