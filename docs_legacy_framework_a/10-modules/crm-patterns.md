> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "CRM Module Patterns"
id: mod-014
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors]
since: "1.0"
normative-level: informative
related:
  - "[Finance Patterns](finance-patterns.md)"
  - "[Projects Patterns](projects-patterns.md)"
  - "[Business Module Catalog](business-modules.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# CRM Module Patterns

**MOD-014 | Status: Accepted | Stability: Stable**

Patterns for customer relationship management: contacts, companies, leads, opportunities, and customer-linked invoicing.

---

## 1. Core Entities

| Entity | Type | Purpose |
|---|---|---|
| `crm_customer` | System | Company/individual customer record |
| `crm_contact` | Custom | Person associated with a customer |
| `crm_lead` | Custom | Potential customer, pre-qualification |
| `crm_opportunity` | Custom | Qualified sales opportunity with value |
| `crm_activity` | Custom | Calls, emails, meetings logged against customer |

`crm_customer` is a system entity because it is linked from `finance_invoice` (FK constraint required) and from `project` (FK). All other CRM entities are custom — they evolve frequently and don't participate in financial accounting.

---

## 2. Customer Entity

```go
var CustomerDefinition = def.SystemDefinition{
    Name:   "crm_customer",
    Module: "crm",
    Fields: []def.FieldDef{
        {Name: "name",          Type: def.FieldData, Required: true, Searchable: true},
        {Name: "type",          Type: def.FieldSelect,
            Options: []string{"Company", "Individual"}, Default: "Company"},
        {Name: "kra_pin",       Type: def.FieldData,
            Validators: []def.FieldValidator{def.KRAPINValidator{}}},
        {Name: "email",         Type: def.FieldData,
            Validators: []def.FieldValidator{def.EmailValidator{}}},
        {Name: "phone",         Type: def.FieldData},
        {Name: "address",       Type: def.FieldSmallText},
        {Name: "credit_limit",  Type: def.FieldCurrency},
        {Name: "payment_terms", Type: def.FieldInt, Default: 30},
            // Days
        {Name: "status",        Type: def.FieldSelect,
            Options: []string{"Active", "Inactive", "Blocked"}, Default: "Active"},
        {Name: "account_balance", Type: def.FieldCurrency},
            // Computed: sum of unpaid invoices (read-only, maintained by finance module)
    },
    Edges: []def.EdgeDef{
        {Name: "contacts",     Target: "crm_contact",     Type: def.EdgeOneToMany},
        {Name: "opportunities", Target: "crm_opportunity", Type: def.EdgeOneToMany},
        {Name: "invoices",     Target: "finance_invoice",  Type: def.EdgeOneToMany},
    },
    Permissions: def.PermissionSet{
        Create: []string{"role:crm.sales_rep", "role:tenant.admin"},
        Read:   []string{"role:crm.sales_rep", "role:finance.viewer", "role:tenant.admin"},
        Write:  []string{"role:crm.sales_rep", "role:tenant.admin"},
        Delete: []string{"role:tenant.admin"},
    },
}
```

---

## 3. Credit Limit Enforcement

A `BeforeSave` hook on `finance_invoice` enforces the customer's credit limit:

```go
type CreditLimitGuard struct {
    CustomerRepo def.EntityRepository[Customer]
    InvoiceRepo  def.EntityRepository[Invoice]
}

func (h *CreditLimitGuard) BeforeSave(ctx context.Context, record *def.EntityRecord, isUpdate bool) error {
    customerID, _ := record.Fields["customer"].(uuid.UUID)
    invoiceTotal, _ := record.Fields["total_kes"].(decimal.Decimal)

    customer, err := h.CustomerRepo.Get(ctx, customerID)
    if err != nil {
        return fmt.Errorf("CreditLimitGuard.BeforeSave: get customer: %w", err)
    }

    creditLimit, _ := customer.Fields["credit_limit"].(decimal.Decimal)
    if creditLimit.IsZero() {
        return nil  // No credit limit set → no enforcement
    }

    // Sum outstanding invoices for this customer
    outstanding, err := h.InvoiceRepo.Aggregate(ctx,
        filter.And(
            filter.Eq("customer", customerID),
            filter.In("status", []string{"Draft", "Submitted", "Approved"}),
        ),
        def.AggregateSpec{Op: "sum", Field: "total_kes"},
    )
    if err != nil {
        return fmt.Errorf("CreditLimitGuard.BeforeSave: aggregate outstanding: %w", err)
    }

    total := outstanding.Value.Add(invoiceTotal)
    if total.GreaterThan(creditLimit) {
        return &def.BusinessError{
            Code:    "crm.credit_limit_exceeded",
            Message: fmt.Sprintf("This invoice would exceed the customer's credit limit of KES %s", creditLimit),
            Status:  400,
        }
    }

    return nil
}
```

---

## 4. Lead-to-Customer Conversion Action

```go
Actions: []def.ActionDef{
    {
        Name:        "convert",
        Label:       "Convert to Customer",
        Permission:  "role:crm.sales_rep",
        HandlerFunc: ConvertLeadAction,
    },
},
```

```go
func ConvertLeadAction(ctx context.Context, action def.ActionContext) (*def.ActionResult, error) {
    lead, err := action.Repo.Get(ctx, action.RecordID)
    if err != nil {
        return nil, fmt.Errorf("ConvertLeadAction: get lead: %w", err)
    }

    status, _ := lead.Fields["status"].(string)
    if status != "Qualified" {
        return nil, &def.BusinessError{
            Code:    "crm.lead_not_qualified",
            Message: "Only qualified leads can be converted to customers",
            Status:  400,
        }
    }

    // Create customer from lead data
    customer, err := action.Services.CustomerRepo.Create(ctx, def.CreateInput{
        Fields: map[string]any{
            "name":  lead.Fields["company_name"],
            "email": lead.Fields["email"],
            "phone": lead.Fields["phone"],
            "type":  "Company",
        },
    })
    if err != nil {
        return nil, fmt.Errorf("ConvertLeadAction: create customer: %w", err)
    }

    // Update lead status and link to customer
    _, err = action.Repo.Update(ctx, action.RecordID, def.UpdateInput{
        Fields: map[string]any{
            "status":            "Converted",
            "converted_to":      customer.ID,
            "converted_at":      time.Now().UTC(),
            "converted_by":      action.Actor.UserID,
        },
    })
    if err != nil {
        return nil, fmt.Errorf("ConvertLeadAction: update lead: %w", err)
    }

    return &def.ActionResult{
        Message: fmt.Sprintf("Lead converted to customer: %s", customer.Fields["name"]),
        Data:    map[string]any{"customer_id": customer.ID},
    }, nil
}
```

---

## 5. Customer Policy (Sales Rep Scope)

Sales reps see only customers they own:

```go
Policy: def.PolicyFunc(func(ctx context.Context) def.Filter {
    actor := session.ActorFromContext(ctx)
    if actor.HasRole("role:tenant.admin") || actor.HasRole("role:crm.manager") {
        return filter.All()
    }
    // Sales reps see only their assigned customers
    return filter.Eq("assigned_rep", actor.UserID)
}),
```

---

## 6. Customer SDUI Detail Page

```go
func BuildCustomerDetailPage(ctx context.Context, actor session.Actor) ([]byte, error) {
    psc := sdui.NewPageSchemaContext(ctx, actor)

    return psc.Marshal(amis.Page(amis.PageProps{
        Title: "${name}",
        Body: amis.Tabs([]amis.Tab{
            {
                Title: "Overview",
                Body:  amis.Detail("GET /api/v1/entities/crm_customer/${id}"),
            },
            {
                Title: "Contacts",
                Body: amis.CRUD(amis.CRUDProps{
                    API: "GET /api/v1/entities/crm_contact?customer=${id}",
                }),
            },
            {
                Title: "Invoices",
                Body: amis.CRUD(amis.CRUDProps{
                    API: "GET /api/v1/entities/finance_invoice?customer=${id}",
                    Columns: []amis.Column{
                        {Name: "number", Label: "Invoice #"},
                        {Name: "total_kes", Label: "Amount", Type: "currency"},
                        {Name: "status", Label: "Status"},
                        {Name: "created_at", Label: "Date", Type: "date"},
                    },
                }),
            },
            {
                Title: "Opportunities",
                Body: amis.CRUD(amis.CRUDProps{
                    API: "GET /api/v1/entities/crm_opportunity?customer=${id}",
                }),
            },
        }),
    }))
}
```

---

## Related Documents

- [Finance Patterns](finance-patterns.md) — invoice FK to crm_customer
- [Projects Patterns](projects-patterns.md) — project FK to crm_customer
- [Business Module Catalog](business-modules.md) — CRM in module dependency graph
- [Field Validators](../04-domain/validators.md) — KRAPINValidator, EmailValidator
