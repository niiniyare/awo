> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Payroll Module Patterns"
id: mod-009
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors]
since: "1.0"
normative-level: informative
related:
  - "[HR Patterns](hr-patterns.md)"
  - "[Finance Patterns](finance-patterns.md)"
  - "[Business Module Catalog](business-modules.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Payroll Module Patterns

**MOD-009 | Status: Accepted | Stability: Stable**

Patterns for payroll processing: payslip computation, PAYE deduction, NSSF/NHIF, net pay disbursement, and payroll journal posting.

---

## 1. Core Entities

| Entity | Type | Purpose |
|---|---|---|
| `payroll_run` | System | Payroll processing batch (month + year) |
| `payroll_payslip` | System | Individual employee payslip with all earnings/deductions |
| `payroll_component` | Custom | Configurable earning/deduction types (Housing, Transport, etc.) |
| `payroll_input` | Custom | Ad-hoc per-employee adjustments (overtime, advances) for one run |

System entities: payslips feed ledger entries and statutory returns (KRA iTax) — financial + regulatory compliance.

---

## 2. Payroll Run Entity

```go
var PayrollRunDefinition = def.SystemDefinition{
    Name:   "payroll_run",
    Module: "payroll",
    Fields: []def.FieldDef{
        {Name: "run_number",    Type: def.FieldNamingSeries, Series: "PR-{YYYY}-{MM}-{SEQ:3}"},
        {Name: "period_month",  Type: def.FieldInt, Required: true},  // 1–12
        {Name: "period_year",   Type: def.FieldInt, Required: true},
        {Name: "status",        Type: def.FieldSelect,
            Options: []string{"Draft", "Processing", "Processed", "Approved", "Disbursed"}, Default: "Draft"},
        {Name: "total_gross",   Type: def.FieldCurrency},
        {Name: "total_paye",    Type: def.FieldCurrency},
        {Name: "total_nssf",    Type: def.FieldCurrency},
        {Name: "total_nhif",    Type: def.FieldCurrency},
        {Name: "total_net",     Type: def.FieldCurrency},
        {Name: "processed_at",  Type: def.FieldDateTime},
        {Name: "approved_by",   Type: def.FieldLink, LinkTarget: "iam_user"},
    },
    Permissions: def.PermissionSet{
        Create: []string{"role:hr.payroll_admin", "role:tenant.admin"},
        Read:   []string{"role:hr.manager", "role:hr.payroll_admin", "role:tenant.admin"},
        Write:  []string{"role:hr.payroll_admin", "role:tenant.admin"},
        Delete: []string{"role:tenant.admin"},
    },
    Actions: []def.ActionDef{
        {
            Name:        "process",
            Label:       "Process Payroll",
            Permission:  "role:hr.payroll_admin",
            HandlerFunc: ProcessPayrollAction,
        },
        {
            Name:        "approve",
            Label:       "Approve & Disburse",
            Permissions: []string{"role:hr.manager", "role:tenant.admin"},
            HandlerFunc: ApprovePayrollAction,
        },
    },
}
```

---

## 3. Payslip Computation

```go
func ProcessPayrollAction(ctx context.Context, action def.ActionContext) (*def.ActionResult, error) {
    run, err := action.Repo.Get(ctx, action.RecordID)
    if err != nil {
        return nil, fmt.Errorf("ProcessPayrollAction: get run: %w", err)
    }

    status, _ := run.Fields["status"].(string)
    if status != "Draft" {
        return nil, &def.BusinessError{
            Code:    "payroll.already_processed",
            Message: "Payroll run has already been processed",
            Status:  409,
        }
    }

    // Mark as Processing, then dispatch Temporal workflow
    _, err = action.Repo.Update(ctx, action.RecordID, def.UpdateInput{
        Fields: map[string]any{"status": "Processing"},
    })
    if err != nil {
        return nil, fmt.Errorf("ProcessPayrollAction: update status: %w", err)
    }

    return &def.ActionResult{
        Message:    "Payroll processing started",
        WorkflowID: fmt.Sprintf("%s.payroll_run.%s.process", run.TenantID, action.RecordID),
    }, nil
}
```

### PayrollProcessingWorkflow

```go
func PayrollProcessingWorkflow(ctx workflow.Context, input PayrollInput) error {
    ao := workflow.ActivityOptions{
        StartToCloseTimeout: 30 * time.Minute,
        RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 3},
    }
    ctx = workflow.WithActivityOptions(ctx, ao)

    // Step 1: Fetch active employees
    var employeeIDs []uuid.UUID
    err := workflow.ExecuteActivity(ctx, a.FetchActiveEmployeesActivity, input).Get(ctx, &employeeIDs)
    if err != nil {
        return fmt.Errorf("PayrollProcessingWorkflow: fetch employees: %w", err)
    }

    // Step 2: Compute payslip for each employee (parallel child workflows or batch activity)
    var payslipResults []PayslipResult
    for _, empID := range employeeIDs {
        var result PayslipResult
        err = workflow.ExecuteActivity(ctx, a.ComputePayslipActivity, PayslipInput{
            TenantID:    input.TenantID,
            RunID:       input.RunID,
            EmployeeID:  empID,
            PeriodMonth: input.PeriodMonth,
            PeriodYear:  input.PeriodYear,
        }).Get(ctx, &result)
        if err != nil {
            return fmt.Errorf("PayrollProcessingWorkflow: compute payslip for %s: %w", empID, err)
        }
        payslipResults = append(payslipResults, result)
    }

    // Step 3: Aggregate totals on run record
    err = workflow.ExecuteActivity(ctx, a.UpdateRunTotalsActivity, UpdateRunTotalsInput{
        RunID:   input.RunID,
        Results: payslipResults,
    }).Get(ctx, nil)
    if err != nil {
        return fmt.Errorf("PayrollProcessingWorkflow: update totals: %w", err)
    }

    // Step 4: Mark run as Processed
    err = workflow.ExecuteActivity(ctx, a.MarkRunProcessedActivity, input.RunID).Get(ctx, nil)
    if err != nil {
        return fmt.Errorf("PayrollProcessingWorkflow: mark processed: %w", err)
    }

    return nil
}
```

---

## 4. PAYE Computation

PAYE uses the `paye_bands` global table (not tenant-scoped — statutory rates):

```go
func (a *Activities) ComputePayslipActivity(ctx context.Context, input PayslipInput) (PayslipResult, error) {
    employee, err := a.EmployeeRepo.Get(ctx, input.EmployeeID)
    if err != nil {
        return PayslipResult{}, fmt.Errorf("ComputePayslipActivity: get employee: %w", err)
    }

    basicSalary, _ := employee.Fields["basic_salary"].(decimal.Decimal)
    allowances, err := a.computeAllowances(ctx, input.EmployeeID, input.PeriodMonth, input.PeriodYear)
    if err != nil {
        return PayslipResult{}, fmt.Errorf("ComputePayslipActivity: allowances: %w", err)
    }

    grossPay := basicSalary.Add(allowances)

    // NSSF: 6% of basic, max KES 2,160
    nssf := basicSalary.Mul(decimal.NewFromFloat(0.06))
    nssfCap, _ := decimal.NewFromString("2160.0000")
    if nssf.GreaterThan(nssfCap) {
        nssf = nssfCap
    }

    // NHIF: slab-based (read from nhif_rates table or settings)
    nhif, err := a.computeNHIF(ctx, grossPay)
    if err != nil {
        return PayslipResult{}, fmt.Errorf("ComputePayslipActivity: nhif: %w", err)
    }

    // Taxable income = gross - NSSF
    taxableIncome := grossPay.Sub(nssf)

    // PAYE from paye_bands global table
    paye, err := a.computePAYE(ctx, taxableIncome)
    if err != nil {
        return PayslipResult{}, fmt.Errorf("ComputePayslipActivity: paye: %w", err)
    }

    // Personal relief: KES 2,400/month (statutory)
    personalRelief, _ := decimal.NewFromString("2400.0000")
    payeAfterRelief := paye.Sub(personalRelief)
    if payeAfterRelief.IsNegative() {
        payeAfterRelief = decimal.Zero
    }

    netPay := grossPay.Sub(nssf).Sub(nhif).Sub(payeAfterRelief)

    // Create payslip record
    payslip, err := a.PayslipRepo.Create(ctx, def.CreateInput{
        Fields: map[string]any{
            "run":          input.RunID,
            "employee":     input.EmployeeID,
            "basic_salary": basicSalary,
            "allowances":   allowances,
            "gross_pay":    grossPay,
            "nssf":         nssf,
            "nhif":         nhif,
            "paye":         payeAfterRelief,
            "net_pay":      netPay,
            "period_month": input.PeriodMonth,
            "period_year":  input.PeriodYear,
        },
    })
    if err != nil {
        return PayslipResult{}, fmt.Errorf("ComputePayslipActivity: create payslip: %w", err)
    }

    return PayslipResult{
        PayslipID: payslip.ID,
        Gross:     grossPay,
        PAYE:      payeAfterRelief,
        NSSF:      nssf,
        NHIF:      nhif,
        Net:       netPay,
    }, nil
}

func (a *Activities) computePAYE(ctx context.Context, taxableIncome decimal.Decimal) (decimal.Decimal, error) {
    // paye_bands is a global (non-RLS) table — query without tenant context
    bands, err := a.GlobalDB.Query(ctx, "SELECT upper_bound, rate FROM paye_bands ORDER BY upper_bound ASC")
    if err != nil {
        return decimal.Zero, fmt.Errorf("computePAYE: query bands: %w", err)
    }
    defer bands.Close()

    paye := decimal.Zero
    remaining := taxableIncome

    for bands.Next() {
        var upperBound decimal.Decimal
        var rate decimal.Decimal
        if err := bands.Scan(&upperBound, &rate); err != nil {
            return decimal.Zero, fmt.Errorf("computePAYE: scan: %w", err)
        }

        bandWidth := upperBound
        taxableInBand := remaining
        if taxableInBand.GreaterThan(bandWidth) {
            taxableInBand = bandWidth
        }

        paye = paye.Add(taxableInBand.Mul(rate).Div(decimal.NewFromFloat(100)))
        remaining = remaining.Sub(bandWidth)
        if remaining.IsNegative() || remaining.IsZero() {
            break
        }
    }

    return paye, nil
}
```

---

## 5. Payroll Journal Entry

After approval, the payroll run posts a journal entry to the GL:

```go
func (a *Activities) PostPayrollJournalActivity(ctx context.Context, input PayrollInput) error {
    // Idempotency: check if journal already posted
    exists, err := a.JournalRepo.Exists(ctx, filter.Eq("reference", fmt.Sprintf("PR/%s", input.RunNumber)))
    if err != nil {
        return fmt.Errorf("PostPayrollJournalActivity: check idempotency: %w", err)
    }
    if exists {
        return nil  // Already posted — safe to return
    }

    // Fetch run totals
    run, err := a.RunRepo.Get(ctx, input.RunID)
    if err != nil {
        return fmt.Errorf("PostPayrollJournalActivity: get run: %w", err)
    }

    grossPay, _ := run.Fields["total_gross"].(decimal.Decimal)
    paye, _ := run.Fields["total_paye"].(decimal.Decimal)
    nssf, _ := run.Fields["total_nssf"].(decimal.Decimal)
    nhif, _ := run.Fields["total_nhif"].(decimal.Decimal)
    netPay, _ := run.Fields["total_net"].(decimal.Decimal)

    // DR: Salaries Expense (gross)
    // CR: PAYE Payable
    // CR: NSSF Payable
    // CR: NHIF Payable
    // CR: Salaries Payable (net)
    return a.FinanceService.PostJournalEntry(ctx, finance.JournalEntryInput{
        Reference: fmt.Sprintf("PR/%s", input.RunNumber),
        Lines: []finance.JournalLine{
            {Account: "salaries_expense",  Debit: grossPay},
            {Account: "paye_payable",      Credit: paye},
            {Account: "nssf_payable",      Credit: nssf},
            {Account: "nhif_payable",      Credit: nhif},
            {Account: "salaries_payable",  Credit: netPay},
        },
    })
}
```

---

## 6. Payslip PDF Generation

Payslip PDFs are generated as a Temporal activity (I/O in activity, not workflow):

```go
func (a *Activities) GeneratePayslipPDFActivity(ctx context.Context, payslipID uuid.UUID) (string, error) {
    payslip, err := a.PayslipRepo.Get(ctx, payslipID)
    if err != nil {
        return "", fmt.Errorf("GeneratePayslipPDFActivity: get payslip: %w", err)
    }

    // Render template → PDF bytes
    pdfBytes, err := a.PDFRenderer.Render(ctx, "payslip", payslip.Fields)
    if err != nil {
        return "", fmt.Errorf("GeneratePayslipPDFActivity: render: %w", err)
    }

    // Store in object storage, return URL
    key := fmt.Sprintf("payslips/%s/%s.pdf", payslip.TenantID, payslipID)
    url, err := a.Storage.Put(ctx, key, pdfBytes, "application/pdf")
    if err != nil {
        return "", fmt.Errorf("GeneratePayslipPDFActivity: upload: %w", err)
    }

    return url, nil
}
```

---

## Related Documents

- [HR Patterns](hr-patterns.md) — employee records, leave, PAYE data sources
- [Finance Patterns](finance-patterns.md) — journal entry posting used for payroll GL entries
- [Business Module Catalog](business-modules.md) — payroll module dependencies
- [Scheduled Workflows](../09-workflow/scheduled-workflows.md) — monthly payroll automation
