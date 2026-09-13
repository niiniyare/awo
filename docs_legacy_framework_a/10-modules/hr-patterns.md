> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "HR Module Patterns"
id: mod-005
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors]
since: "1.0"
normative-level: informative
related:
  - "[Business Module Catalog](business-modules.md)"
  - "[Signal Patterns](../09-workflow/signal-patterns.md)"
  - "[Hooks](../04-domain/hooks.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# HR Module Patterns

**MOD-005 | Status: Accepted | Stability: Stable**

Patterns for employee management, leave workflows, attendance, and payroll integration in the HR module.

---

## 1. Employee-User Link

Every employee record is linked one-to-one to an IAM user:

```go
// hr_employee entity definition (excerpt)
Fields: []entity.FieldDef{
    {Name: "user_id",     Type: def.FieldLink, LinkTarget: "iam_user", Required: true, Immutable: true},
    {Name: "employee_no", Type: def.FieldNamingSeries, Series: "EMP-{SEQ:6}"},
    {Name: "department",  Type: def.FieldLink, LinkTarget: "hr_department"},
    {Name: "position",    Type: def.FieldLink, LinkTarget: "hr_position"},
    {Name: "hire_date",   Type: def.FieldDate, Required: true},
    {Name: "status",      Type: def.FieldSelect,
     Options: []string{"Active", "On Leave", "Terminated", "Probation"},
     Default: "Probation"},
}
```

The `user_id` link is `Immutable` — once set on employee creation, it cannot be changed. Employee termination changes `status`, not user_id.

### Employee Creation Hook

When an employee is created, the linked user's role MUST be set:

```go
func (h *EmployeeCreatedHook) AfterCreate(ctx context.Context, record *entity.EntityRecord) error {
    userID, _ := record.Fields["user_id"].(uuid.UUID)

    // Assign default employee role
    err := h.IAMService.AssignRole(ctx, userID, "role:tenant.user")
    if err != nil {
        return fmt.Errorf("EmployeeCreatedHook: assign role: %w", err)
    }
    return nil
}
```

This runs inside the entity's `after_save` transaction — if role assignment fails, the employee record creation rolls back.

---

## 2. Leave Request Workflow

Leave requests use multi-step approval (see signal-patterns.md for the full signal pattern):

```go
// hr_leave_request WorkflowTrigger
WorkflowTriggers: []entity.WorkflowTrigger{
    {
        On:         entity.EventOnCreate,
        WorkflowFn: "LeaveRequestApprovalWorkflow",
        TaskQueue:  "hr.leave.approval",
        InputBuilder: func(rec *entity.EntityRecord, tc entity.TriggerContext) (any, error) {
            return LeaveApprovalInput{
                TenantID:      rec.TenantID,
                LeaveID:       rec.ID,
                EmployeeID:    rec.Fields["employee_id"].(uuid.UUID),
                LeaveType:     rec.Fields["leave_type"].(string),
                StartDate:     rec.Fields["start_date"].(time.Time),
                EndDate:       rec.Fields["end_date"].(time.Time),
            }, nil
        },
    },
},
```

### Leave Balance Check (BeforeCreate Hook)

```go
func (h *LeaveBalanceGuard) BeforeCreate(ctx context.Context, record *entity.EntityRecord) error {
    employeeID := record.Fields["employee_id"].(uuid.UUID)
    leaveType  := record.Fields["leave_type"].(string)
    startDate  := record.Fields["start_date"].(time.Time)
    endDate    := record.Fields["end_date"].(time.Time)

    days := workingDaysBetween(startDate, endDate)

    balance, err := h.LeaveBalanceService.GetBalance(ctx, employeeID, leaveType)
    if err != nil {
        return fmt.Errorf("LeaveBalanceGuard: get balance: %w", err)
    }

    if balance < days {
        return &errors.ValidationError{
            Fields: map[string]string{
                "end_date": fmt.Sprintf("Insufficient %s leave balance. Available: %.1f days, Requested: %.1f days.", leaveType, balance, days),
            },
        }
    }
    return nil
}
```

---

## 3. Attendance Tracking

Clock-in/clock-out via API:

```go
// Action: POST /api/v1/entities/hr_attendance/{id}/clock-in
func ClockInAction(ctx context.Context, action entity.ActionContext) (*entity.ActionResult, error) {
    actor := session.ActorFromContext(ctx)

    // Find employee for this actor
    employees, _, err := action.EmployeeRepo.Query(ctx, filter.Eq("user_id", actor.UserID))
    if err != nil || len(employees) == 0 {
        return nil, &errors.BusinessError{Code: "hr.no_employee", Message: "No employee record found.", Status: 404}
    }
    employee := employees[0]

    // Check: not already clocked in
    existing, _, err := action.Repo.Query(ctx, filter.And(
        filter.Eq("employee_id", employee.ID),
        filter.Eq("date", time.Now().UTC().Truncate(24*time.Hour)),
        filter.IsNull("clock_out"),
    ))
    if err != nil {
        return nil, err
    }
    if len(existing) > 0 {
        return nil, &errors.BusinessError{
            Code:    "hr.already_clocked_in",
            Message: "You are already clocked in.",
            Status:  409,
        }
    }

    _, err = action.Repo.Create(ctx, entity.CreateInput{
        Fields: map[string]any{
            "employee_id": employee.ID,
            "date":        time.Now().UTC().Truncate(24 * time.Hour),
            "clock_in":    time.Now().UTC(),
        },
    })
    if err != nil {
        return nil, err
    }

    return &entity.ActionResult{Message: "Clocked in successfully."}, nil
}
```

---

## 4. Payroll Integration

HR module exposes a service interface consumed by the Payroll module:

```go
// internal/core/hr/service.go

type PayrollData struct {
    EmployeeID   uuid.UUID
    BasicSalary  decimal.Decimal
    WorkingDays  int
    DaysWorked   int
    LeaveDaysTaken int
}

func (s *HRService) GetPayrollData(ctx context.Context, tenantID uuid.UUID, month time.Time) ([]PayrollData, error) {
    // Compute from hr_contract and hr_attendance for the given month
    contracts, _, err := s.ContractRepo.Query(ctx, filter.And(
        filter.Lte("start_date", month),
        filter.Or(filter.IsNull("end_date"), filter.Gte("end_date", month)),
        filter.Eq("status", "Active"),
    ))
    if err != nil {
        return nil, fmt.Errorf("GetPayrollData: get contracts: %w", err)
    }

    var results []PayrollData
    for _, contract := range contracts {
        daysWorked, err := s.AttendanceRepo.Count(ctx, filter.And(
            filter.Eq("employee_id", contract.EmployeeID),
            filter.Gte("date", monthStart(month)),
            filter.Lte("date", monthEnd(month)),
            filter.IsNotNull("clock_out"),
        ))
        if err != nil {
            return nil, err
        }

        results = append(results, PayrollData{
            EmployeeID:  contract.EmployeeID,
            BasicSalary: contract.BasicSalary,
            WorkingDays: workingDaysInMonth(month),
            DaysWorked:  int(daysWorked),
        })
    }
    return results, nil
}
```

The Payroll module calls `HRService.GetPayrollData` — it does not query HR repositories directly. Cross-module data access goes through service interfaces, not repository cross-imports.

---

## 5. PAYE Computation

PAYE is computed from `paye_bands` global table (updated per Finance Act):

```go
func ComputePAYE(grossIncome decimal.Decimal) decimal.Decimal {
    // paye_bands is a global table: no tenant context needed
    // Fetch via a direct query with no RLS (global table)
    bands := fetchPayeBands()  // cached; refreshed on table change

    paye := decimal.Zero
    remaining := grossIncome
    for _, band := range bands {
        if remaining.IsZero() {
            break
        }
        taxable := decimal.Min(remaining, band.Width)
        paye = paye.Add(taxable.Mul(band.Rate))
        remaining = remaining.Sub(taxable)
    }
    return paye
}
```

PAYE bands change per Finance Act (typically annually). The `paye_bands` table is updated via a migration when KRA publishes new rates.

---

## Related Documents

- [Business Module Catalog](business-modules.md) — HR entity list
- [Signal Patterns](../09-workflow/signal-patterns.md) — approval workflow with signals
- [Finance Patterns](finance-patterns.md) — payroll journal entries
- [Hooks](../04-domain/hooks.md) — `after_create`, `before_create`
