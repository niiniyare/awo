> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Projects Module Patterns"
id: mod-010
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors]
since: "1.0"
normative-level: informative
related:
  - "[Finance Patterns](finance-patterns.md)"
  - "[HR Patterns](hr-patterns.md)"
  - "[Business Module Catalog](business-modules.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Projects Module Patterns

**MOD-010 | Status: Accepted | Stability: Stable**

Patterns for project management: project lifecycle, task tracking, time logging, expense allocation, and project billing.

---

## 1. Core Entities

| Entity | Type | Purpose |
|---|---|---|
| `project` | System | Project header: client, budget, dates, status |
| `project_task` | System | Work item with assignee, estimate, actual hours |
| `project_timesheet` | System | Time log entries (hours × billing rate) |
| `project_expense` | Custom | Out-of-pocket expenses charged to a project |
| `project_milestone` | Custom | Key dates and completion criteria |

System entities: timesheets feed revenue recognition and billing — financial integrity required.

---

## 2. Project Entity

```go
var ProjectDefinition = def.SystemDefinition{
    Name:   "project",
    Module: "projects",
    Fields: []def.FieldDef{
        {Name: "code",           Type: def.FieldNamingSeries, Series: "PROJ-{YYYY}-{SEQ:4}"},
        {Name: "name",           Type: def.FieldData, Required: true, Searchable: true},
        {Name: "client",         Type: def.FieldLink, LinkTarget: "crm_customer", Required: true},
        {Name: "status",         Type: def.FieldSelect,
            Options: []string{"Planning", "Active", "On Hold", "Completed", "Cancelled"}, Default: "Planning"},
        {Name: "start_date",     Type: def.FieldDate, Required: true},
        {Name: "end_date",       Type: def.FieldDate},
        {Name: "budget_kes",     Type: def.FieldCurrency},
        {Name: "billed_kes",     Type: def.FieldCurrency},  // computed: sum of invoiced timesheets
        {Name: "billing_type",   Type: def.FieldSelect,
            Options: []string{"Fixed Price", "Time & Materials", "Retainer"}},
        {Name: "project_manager", Type: def.FieldLink, LinkTarget: "iam_user"},
        {Name: "description",    Type: def.FieldLongText},
    },
    Edges: []def.EdgeDef{
        {Name: "tasks",       Target: "project_task",      Type: def.EdgeOneToMany, CascadeDelete: true},
        {Name: "timesheets",  Target: "project_timesheet",  Type: def.EdgeOneToMany},
        {Name: "milestones",  Target: "project_milestone",  Type: def.EdgeOneToMany, CascadeDelete: true},
    },
    Permissions: def.PermissionSet{
        Create: []string{"role:projects.manager", "role:tenant.admin"},
        Read:   []string{"role:projects.member", "role:projects.manager", "role:tenant.admin"},
        Write:  []string{"role:projects.manager", "role:tenant.admin"},
        Delete: []string{"role:tenant.admin"},
    },
    Policy: def.PolicyFunc(func(ctx context.Context) def.Filter {
        actor := session.ActorFromContext(ctx)
        if actor.HasRole("role:projects.manager") || actor.HasRole("role:tenant.admin") {
            return filter.All()  // managers see all projects
        }
        // Members see only projects they're assigned to (via tasks)
        return filter.In("id", subquery.ProjectsForMember(actor.UserID))
    }),
}
```

---

## 3. Task Entity with Progress Tracking

```go
var ProjectTaskDefinition = def.SystemDefinition{
    Name:   "project_task",
    Module: "projects",
    Fields: []def.FieldDef{
        {Name: "project",       Type: def.FieldLink, LinkTarget: "project",
            Required: true, Immutable: true},
        {Name: "title",         Type: def.FieldData, Required: true, Searchable: true},
        {Name: "assignee",      Type: def.FieldLink, LinkTarget: "iam_user"},
        {Name: "status",        Type: def.FieldSelect,
            Options: []string{"Todo", "In Progress", "Review", "Done"}, Default: "Todo"},
        {Name: "priority",      Type: def.FieldSelect,
            Options: []string{"Low", "Medium", "High", "Critical"}, Default: "Medium"},
        {Name: "estimated_hours", Type: def.FieldFloat},
        {Name: "logged_hours",  Type: def.FieldFloat},  // computed from timesheets
        {Name: "due_date",      Type: def.FieldDate},
        {Name: "completed_at",  Type: def.FieldDateTime},
        {Name: "description",   Type: def.FieldLongText},
    },
    Hooks: def.HookSet{
        BeforeSave: []def.BeforeSaveHook{&TaskCompletionHook{}},
    },
}

type TaskCompletionHook struct{}

func (h *TaskCompletionHook) BeforeSave(ctx context.Context, record *def.EntityRecord, isUpdate bool) error {
    if !isUpdate {
        return nil
    }
    status, _ := record.Fields["status"].(string)
    if status == "Done" {
        if record.Fields["completed_at"] == nil {
            record.Fields["completed_at"] = time.Now().UTC()
        }
    } else {
        record.Fields["completed_at"] = nil  // clear if moved back
    }
    return nil
}
```

---

## 4. Timesheet Logging

```go
var TimesheetDefinition = def.SystemDefinition{
    Name:   "project_timesheet",
    Module: "projects",
    Fields: []def.FieldDef{
        {Name: "project",      Type: def.FieldLink, LinkTarget: "project",
            Required: true, Immutable: true},
        {Name: "task",         Type: def.FieldLink, LinkTarget: "project_task"},
        {Name: "employee",     Type: def.FieldLink, LinkTarget: "hr_employee",
            Required: true, Immutable: true},
        {Name: "date",         Type: def.FieldDate, Required: true},
        {Name: "hours",        Type: def.FieldFloat, Required: true},
        {Name: "description",  Type: def.FieldSmallText, Required: true},
        {Name: "billing_rate", Type: def.FieldCurrency},  // KES/hour at time of logging
        {Name: "amount_kes",   Type: def.FieldCurrency},  // hours × billing_rate
        {Name: "billable",     Type: def.FieldBool, Default: true},
        {Name: "invoiced",     Type: def.FieldBool, Default: false},
        {Name: "invoice",      Type: def.FieldLink, LinkTarget: "finance_invoice"},
    },
    Hooks: def.HookSet{
        BeforeCreate: []def.BeforeCreateHook{&TimesheetRateHook{}},
    },
    Policy: def.PolicyFunc(func(ctx context.Context) def.Filter {
        actor := session.ActorFromContext(ctx)
        if actor.HasRole("role:projects.manager") || actor.HasRole("role:tenant.admin") {
            return filter.All()
        }
        // Members see only their own timesheets
        return filter.Eq("employee", actor.LinkedEmployeeID)
    }),
}

// TimesheetRateHook auto-fills billing rate from employee's current rate
type TimesheetRateHook struct {
    EmployeeRepo def.EntityRepository[HREmployee]
}

func (h *TimesheetRateHook) BeforeCreate(ctx context.Context, record *def.EntityRecord) error {
    hours, _ := record.Fields["hours"].(float64)
    if hours <= 0 {
        return &def.ValidationError{
            Fields: map[string]string{"hours": "Hours must be greater than zero"},
        }
    }

    // Fill billing rate from employee record if not provided
    if record.Fields["billing_rate"] == nil {
        empID, _ := record.Fields["employee"].(uuid.UUID)
        emp, err := h.EmployeeRepo.Get(ctx, empID)
        if err != nil {
            return fmt.Errorf("TimesheetRateHook.BeforeCreate: get employee: %w", err)
        }
        rate, _ := emp.Fields["billing_rate_kes"].(decimal.Decimal)
        record.Fields["billing_rate"] = rate
    }

    rate, _ := record.Fields["billing_rate"].(decimal.Decimal)
    hoursDecimal := decimal.NewFromFloat(hours)
    record.Fields["amount_kes"] = rate.Mul(hoursDecimal)

    return nil
}
```

---

## 5. Project Billing Action

For Time & Materials projects, billing creates a Finance invoice from unbilled timesheets:

```go
Actions: []def.ActionDef{
    {
        Name:        "bill",
        Label:       "Create Invoice from Timesheets",
        Permission:  "role:projects.manager",
        HandlerFunc: BillProjectAction,
    },
},
```

```go
func BillProjectAction(ctx context.Context, action def.ActionContext) (*def.ActionResult, error) {
    project, err := action.Repo.Get(ctx, action.RecordID)
    if err != nil {
        return nil, fmt.Errorf("BillProjectAction: get project: %w", err)
    }

    billingType, _ := project.Fields["billing_type"].(string)
    if billingType != "Time & Materials" {
        return nil, &def.BusinessError{
            Code:    "projects.not_t_and_m",
            Message: "Billing action is only for Time & Materials projects",
            Status:  400,
        }
    }

    // Fetch unbilled billable timesheets
    timesheets, _, err := action.Services.TimesheetRepo.Query(ctx, filter.And(
        filter.Eq("project", action.RecordID),
        filter.Eq("billable", true),
        filter.Eq("invoiced", false),
    ))
    if err != nil {
        return nil, fmt.Errorf("BillProjectAction: fetch timesheets: %w", err)
    }
    if len(timesheets) == 0 {
        return nil, &def.BusinessError{
            Code:    "projects.no_unbilled_timesheets",
            Message: "No unbilled timesheets found for this project",
            Status:  400,
        }
    }

    // Sum total
    total := decimal.Zero
    for _, ts := range timesheets {
        amount, _ := ts.Fields["amount_kes"].(decimal.Decimal)
        total = total.Add(amount)
    }

    // Create Finance invoice via FinanceService interface (cross-module boundary)
    invoiceID, err := action.Services.FinanceService.CreateProjectInvoice(ctx, finance.ProjectInvoiceInput{
        CustomerID:  project.Fields["client"].(uuid.UUID),
        ProjectID:   action.RecordID,
        TotalKES:    total,
        Timesheets:  timesheets,
    })
    if err != nil {
        return nil, fmt.Errorf("BillProjectAction: create invoice: %w", err)
    }

    // Mark timesheets as invoiced
    _, err = action.Services.TimesheetRepo.BulkUpdate(ctx, filter.And(
        filter.Eq("project", action.RecordID),
        filter.Eq("billable", true),
        filter.Eq("invoiced", false),
    ), def.Patch{"invoiced": true, "invoice": invoiceID})
    if err != nil {
        return nil, fmt.Errorf("BillProjectAction: mark invoiced: %w", err)
    }

    return &def.ActionResult{
        Message: fmt.Sprintf("Invoice created for KES %.2f", total),
        Data:    map[string]any{"invoice_id": invoiceID},
    }, nil
}
```

---

## 6. Project Dashboard SDUI

```go
func BuildProjectDashboard(ctx context.Context, actor session.Actor) ([]byte, error) {
    psc := sdui.NewPageSchemaContext(ctx, actor)

    return psc.Marshal(amis.Page(amis.PageProps{
        Title: "Project Overview",
        Body: amis.Grid([]amis.Panel{
            // Status breakdown
            amis.Panel("Project Status", amis.Chart(amis.ChartProps{
                Type: "pie",
                API:  "GET /api/v1/entities/project/aggregate?group_by=status",
            })),
            // My tasks
            amis.Panel("My Tasks", amis.CRUD(amis.CRUDProps{
                API: "GET /api/v1/entities/project_task?assignee=__current_user__&status=In Progress,Todo",
                Columns: []amis.Column{
                    {Name: "project.name", Label: "Project"},
                    {Name: "title", Label: "Task"},
                    {Name: "priority", Label: "Priority"},
                    {Name: "due_date", Label: "Due", Type: "date"},
                },
            })),
            // Hours logged this month
            amis.Panel("Hours This Month", amis.Stat(amis.StatProps{
                API:   "GET /api/v1/entities/project_timesheet/aggregate?op=sum&field=hours&period=this_month",
                Label: "Total Hours",
            })),
        }),
    }))
}
```

---

## Related Documents

- [Finance Patterns](finance-patterns.md) — invoice creation from project billing
- [HR Patterns](hr-patterns.md) — employee-user link, billing rates
- [Business Module Catalog](business-modules.md) — projects module dependencies
- [Dashboard Patterns](../08-sdui/dashboard-patterns.md) — stat cards and chart patterns
