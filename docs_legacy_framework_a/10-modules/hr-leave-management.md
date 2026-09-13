> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "HR: Leave Management"
id: mod-017
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors]
since: "1.0"
normative-level: normative
related:
  - "[HR Patterns](hr-patterns.md)"
  - "[Workflow Engine](../09-workflow/workflow-engine.md)"
  - "[Signal Patterns](../09-workflow/signal-patterns.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# HR: Leave Management

**MOD-017 | Status: Accepted | Stability: Stable**

Leave request lifecycle, approval workflow, leave balance tracking, and public holiday calendar integration.

---

## 1. Entities

### `hr_leave_type`

Defines leave categories for a tenant:

```go
var LeaveTypeDefinition = def.SystemDefinition{
    Name:        "hr_leave_type",
    Module:      "hr",
    Label:       "Leave Type",
    Fields: []def.FieldDef{
        {Name: "name",             Type: def.FieldData,   Required: true},
        {Name: "days_per_year",    Type: def.FieldInt,    Required: true},
        {Name: "carry_over_days",  Type: def.FieldInt,    Default: 0},
        {Name: "requires_approval",Type: def.FieldBool,   Default: true},
        {Name: "is_paid",          Type: def.FieldBool,   Default: true},
        {Name: "min_days_notice",  Type: def.FieldInt,    Default: 3},
        {Name: "color",            Type: def.FieldData},  // for calendar UI
    },
}
```

### `hr_leave_balance`

Per-employee annual leave balance:

```go
var LeaveBalanceDefinition = def.SystemDefinition{
    Name:        "hr_leave_balance",
    Module:      "hr",
    Fields: []def.FieldDef{
        {Name: "employee",     Type: def.FieldLink, LinkTarget: "hr_employee", Required: true, Immutable: true},
        {Name: "leave_type",   Type: def.FieldLink, LinkTarget: "hr_leave_type", Required: true, Immutable: true},
        {Name: "year",         Type: def.FieldInt,  Required: true, Immutable: true},
        {Name: "allocated",    Type: def.FieldInt,  Required: true},  // total days for the year
        {Name: "used",         Type: def.FieldInt,  Default: 0},
        {Name: "pending",      Type: def.FieldInt,  Default: 0},      // approved but future
        {Name: "carry_over",   Type: def.FieldInt,  Default: 0},
    },
}
```

### `hr_leave_request`

```go
var LeaveRequestDefinition = def.SystemDefinition{
    Name:        "hr_leave_request",
    Module:      "hr",
    Label:       "Leave Request",
    Fields: []def.FieldDef{
        {Name: "employee",     Type: def.FieldLink, LinkTarget: "hr_employee", Required: true, Immutable: true},
        {Name: "leave_type",   Type: def.FieldLink, LinkTarget: "hr_leave_type", Required: true, Immutable: true},
        {Name: "start_date",   Type: def.FieldDate, Required: true},
        {Name: "end_date",     Type: def.FieldDate, Required: true},
        {Name: "days",         Type: def.FieldInt,  Required: true},  // computed, excluding weekends + holidays
        {Name: "status",       Type: def.FieldSelect, Required: true,
            Options: []string{"Draft", "Pending", "Approved", "Rejected", "Cancelled"},
            Default: "Draft"},
        {Name: "approver",     Type: def.FieldLink, LinkTarget: "hr_employee"},
        {Name: "reason",       Type: def.FieldSmallText},
        {Name: "rejection_reason", Type: def.FieldSmallText},
        {Name: "approved_at",  Type: def.FieldDateTime},
    },
    Hooks: def.HookSet{
        BeforeCreate: []def.BeforeCreateHook{&LeaveBalanceValidator{}},
    },
    WorkflowTriggers: []def.WorkflowTrigger{
        {
            On:         def.EventOnSubmit,
            WorkflowFn: "LeaveApprovalWorkflow",
            TaskQueue:  "hr.leave",
        },
    },
}
```

---

## 2. LeaveBalanceValidator Hook

```go
type LeaveBalanceValidator struct {
    BalanceRepo  def.EntityRepository[LeaveBalance]
    HolidayRepo  def.EntityRepository[HRHoliday]
}

func (v *LeaveBalanceValidator) BeforeCreate(ctx context.Context, rec *def.EntityRecord) error {
    startDate, _ := rec.GetDate("start_date")
    endDate, _ := rec.GetDate("end_date")
    leaveTypeID, _ := rec.GetUUID("leave_type")
    employeeID, _ := rec.GetUUID("employee")

    // Calculate working days (exclude weekends and public holidays)
    holidays, _, _ := v.HolidayRepo.Query(ctx, filter.And(
        filter.GtEq("date", startDate),
        filter.LtEq("date", endDate),
    ))
    workingDays := calculateWorkingDays(startDate, endDate, holidays)
    rec.Set("days", workingDays)

    // Check balance
    year := startDate.Year()
    balances, _, err := v.BalanceRepo.Query(ctx, filter.And(
        filter.Eq("employee", employeeID),
        filter.Eq("leave_type", leaveTypeID),
        filter.Eq("year", year),
    ))
    if err != nil {
        return fmt.Errorf("LeaveBalanceValidator: %w", err)
    }
    if len(balances) == 0 {
        return &errors.ValidationError{
            Fields: map[string]string{"leave_type": "No leave allocation found for this year"},
        }
    }

    balance := balances[0]
    available := balance.Allocated + balance.CarryOver - balance.Used - balance.Pending
    if workingDays > available {
        return &errors.ValidationError{
            Fields: map[string]string{
                "days": fmt.Sprintf("Insufficient balance: %d days available, %d requested", available, workingDays),
            },
        }
    }

    return nil
}
```

---

## 3. Leave Approval Workflow

Uses Temporal signals for the approval gate:

```go
func LeaveApprovalWorkflow(ctx workflow.Context, input LeaveApprovalInput) error {
    ao := workflow.ActivityOptions{StartToCloseTimeout: 30 * time.Second}
    ctx = workflow.WithActivityOptions(ctx, ao)

    // 1. Reserve balance (move days from available to pending)
    if err := workflow.ExecuteActivity(ctx, a.ReserveLeaveBalanceActivity, input).Get(ctx, nil); err != nil {
        return err
    }

    // 2. Notify approver
    _ = workflow.ExecuteActivity(ctx, a.NotifyApproverActivity, input).Get(ctx, nil)

    // 3. Wait for approval signal (or timeout after 7 days)
    var decision ApprovalDecision
    signalChan := workflow.GetSignalChannel(ctx, "leave.approval_decision")

    selector := workflow.NewSelector(ctx)
    selector.AddReceive(signalChan, func(c workflow.ReceiveChannel, more bool) {
        c.Receive(ctx, &decision)
    })
    selector.AddFuture(workflow.NewTimer(ctx, 7*24*time.Hour), func(f workflow.Future) {
        decision = ApprovalDecision{Action: "auto_reject", Reason: "No response within 7 days"}
    })
    selector.Select(ctx)

    // 4. Apply decision
    if decision.Action == "approve" {
        return workflow.ExecuteActivity(ctx, a.ApproveLeaveRequestActivity, ApproveInput{
            TenantID:       input.TenantID,
            LeaveRequestID: input.LeaveRequestID,
            ApproverID:     decision.ApproverID,
        }).Get(ctx, nil)
    }

    // Reject — release reserved balance
    return workflow.ExecuteActivity(ctx, a.RejectLeaveRequestActivity, RejectInput{
        TenantID:       input.TenantID,
        LeaveRequestID: input.LeaveRequestID,
        Reason:         decision.Reason,
    }).Get(ctx, nil)
}
```

### Sending Approval Signal

```go
// POST /api/v1/entities/hr_leave_request/{id}/approve
func ApproveLeaveAction(ctx context.Context, action def.ActionContext) (*def.ActionResult, error) {
    wfID := fmt.Sprintf("%s.hr_leave_request.%s.on_submit",
        action.Actor.TenantID, action.RecordID)

    err := action.TemporalClient.SignalWorkflow(ctx, wfID, "",
        "leave.approval_decision",
        ApprovalDecision{Action: "approve", ApproverID: action.Actor.UserID},
    )
    if err != nil {
        return nil, fmt.Errorf("ApproveLeaveAction: signal: %w", err)
    }

    return &def.ActionResult{Message: "Leave request approved"}, nil
}
```

---

## 4. Leave Balance Management

### Annual Allocation (Scheduled Workflow)

At the start of each year, allocate leave days for all employees:

```go
func AllocateAnnualLeaveActivity(ctx context.Context, input TenantYearInput) error {
    tenantCtx, _ := a.TenantStore.SetTenantContext(ctx, input.TenantID)

    employees, _, _ := a.EmployeeRepo.Query(tenantCtx, filter.Eq("status", "Active"))
    leaveTypes, _, _ := a.LeaveTypeRepo.Query(tenantCtx, filter.Eq("is_active", true))

    for _, employee := range employees {
        for _, leaveType := range leaveTypes {
            // Check if balance already exists (idempotent)
            exists, _ := a.BalanceRepo.Exists(tenantCtx, filter.And(
                filter.Eq("employee", employee.ID),
                filter.Eq("leave_type", leaveType.ID),
                filter.Eq("year", input.Year),
            ))
            if exists {
                continue
            }

            // Carry over from previous year
            carryOver := calculateCarryOver(ctx, a, tenantCtx, employee.ID, leaveType, input.Year-1)

            _, _ = a.BalanceRepo.Create(tenantCtx, def.CreateInput{
                Fields: map[string]any{
                    "employee":   employee.ID,
                    "leave_type": leaveType.ID,
                    "year":       input.Year,
                    "allocated":  leaveType.DaysPerYear,
                    "carry_over": carryOver,
                },
            })
        }
        activity.RecordHeartbeat(ctx, fmt.Sprintf("processed employee %s", employee.ID))
    }
    return nil
}
```

---

## 5. Leave Calendar SDUI

```go
func BuildLeaveCalendarPage(ctx context.Context, actor session.Actor) ([]byte, error) {
    return json.Marshal(amis.Page{
        Title: "Leave Calendar",
        Body: amis.Calendar{
            Type: "calendar",
            API:  "/api/v1/reports/hr/leave-calendar",
            // Calendar returns events in amis format: {start, end, title, color}
        },
    })
}
```

The `/api/v1/reports/hr/leave-calendar` handler returns all approved leave requests as calendar events, coloured by leave type.

---

## Related Documents

- [HR Patterns](hr-patterns.md) — employee entity, payroll integration
- [Signal Patterns](../09-workflow/signal-patterns.md) — Temporal signal usage
- [Scheduled Workflows](../09-workflow/scheduled-workflows.md) — annual leave allocation
- [SDUI Report View Patterns](../08-sdui/report-view-patterns.md) — calendar and report views
