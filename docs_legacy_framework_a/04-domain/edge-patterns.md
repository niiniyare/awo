> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Edge Patterns"
id: dom-011
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors]
since: "1.0"
normative-level: informative
related:
  - "[Edges](edges.md)"
  - "[Filter DSL](../05-persistence/filter-dsl.md)"
  - "[Query Options](../05-persistence/query-options.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Edge Patterns

**DOM-011 | Status: Accepted | Stability: Stable**

Common patterns for declaring and using edges: parent-child, polymorphic links, many-to-many, and cross-module edges.

---

## 1. One-to-Many with Cascade Delete

The most common pattern — a parent owns child records:

```go
Edges: []def.EdgeDef{
    {
        Name:          "lines",
        Target:        "finance_invoice_line",
        Type:          def.EdgeOneToMany,
        ForeignKey:    "invoice",         // field on the child entity
        CascadeDelete: true,              // deleting invoice deletes all lines
    },
},
```

Loading the edge:

```go
invoice, err := repo.Get(ctx, invoiceID, def.WithEdge("lines"))
lines := invoice.Edges["lines"].([]InvoiceLine)
```

---

## 2. Many-to-One (Link / FK)

A child pointing to its parent (or any FK relationship):

```go
Fields: []def.FieldDef{
    {
        Name:       "customer",
        Type:       def.FieldLink,
        LinkTarget: "crm_customer",
        Required:   true,
    },
},
```

Loading the linked entity:

```go
invoice, err := repo.Get(ctx, invoiceID, def.WithEdge("customer"))
customer := invoice.Edges["customer"].(*Customer)
```

One query is issued: `SELECT * FROM crm_customer WHERE id = $invoice.customer_id`.

---

## 3. Optional Link

Not all invoices have a project:

```go
{
    Name:       "project",
    Type:       def.FieldLink,
    LinkTarget: "project",
    Required:   false,  // Default — edge is nil when not set
},
```

```go
invoice, err := repo.Get(ctx, invoiceID, def.WithEdge("project"))
project, hasProject := invoice.Edges["project"].(*Project)
if hasProject {
    // Use project
}
```

---

## 4. Many-to-Many via Join Entity

Temporal cannot be expressed as a direct EdgeDef — model as two one-to-many edges through a join entity:

```go
// project_member join entity
var ProjectMemberDefinition = def.SystemDefinition{
    Name:   "project_member",
    Module: "projects",
    Fields: []def.FieldDef{
        {Name: "project",  Type: def.FieldLink, LinkTarget: "project",  Required: true, Immutable: true},
        {Name: "employee", Type: def.FieldLink, LinkTarget: "hr_employee", Required: true, Immutable: true},
        {Name: "role",     Type: def.FieldSelect, Options: []string{"Lead", "Member", "Observer"}},
        {Name: "joined_at", Type: def.FieldDateTime},
    },
}

// On the project entity:
Edges: []def.EdgeDef{
    {Name: "members", Target: "project_member", Type: def.EdgeOneToMany, ForeignKey: "project"},
},
```

To get all employees on a project:

```go
project, _ := repo.Get(ctx, projectID, def.WithEdge("members"))
members := project.Edges["members"].([]ProjectMember)
// members[i].Fields["employee"] is the employee UUID
// Load employee records separately if needed
employeeIDs := make([]uuid.UUID, len(members))
for i, m := range members {
    employeeIDs[i] = m.Fields["employee"].(uuid.UUID)
}
employees, _, _ := employeeRepo.Query(ctx, filter.In("id", employeeIDs))
```

---

## 5. Polymorphic / DynamicLink

When a record can link to different entity types (e.g., an attachment on any entity):

```go
var AttachmentDefinition = def.CustomDefinition{
    Name:   "attachment",
    Module: "platform",
    Fields: []def.FieldDef{
        {Name: "linked_entity_type", Type: def.FieldData, Required: true, Immutable: true},
            // e.g. "finance_invoice", "crm_customer"
        {Name: "linked_entity_id",   Type: def.FieldData, Required: true, Immutable: true},
            // UUID string of the linked record
        {Name: "filename",  Type: def.FieldData, Required: true},
        {Name: "file_url",  Type: def.FieldData, Sensitive: true},
        {Name: "mime_type", Type: def.FieldData},
        {Name: "size_bytes", Type: def.FieldInt},
    },
    Policy: def.PolicyFunc(func(ctx context.Context) def.Filter {
        // Actor can only see attachments for entities they have access to
        // This is enforced by the parent entity's RBAC — attachment read is
        // implicitly granted when the parent entity can be read
        return filter.All()  // Parent RBAC is the effective gate
    }),
}
```

Loading attachments for an invoice:

```go
attachments, _, _ := attachmentRepo.Query(ctx, filter.And(
    filter.Eq("linked_entity_type", "finance_invoice"),
    filter.Eq("linked_entity_id", invoiceID.String()),
))
```

---

## 6. Self-Referential Edge (Tree / Hierarchy)

For hierarchical structures like org charts or category trees:

```go
var DepartmentDefinition = def.SystemDefinition{
    Name:   "hr_department",
    Module: "hr",
    Fields: []def.FieldDef{
        {Name: "name",   Type: def.FieldData, Required: true},
        {Name: "parent", Type: def.FieldLink, LinkTarget: "hr_department"},
            // Null = root department
        {Name: "level",  Type: def.FieldInt, Default: 0},
    },
    Edges: []def.EdgeDef{
        {Name: "children", Target: "hr_department", Type: def.EdgeOneToMany, ForeignKey: "parent"},
    },
}
```

Loading one level of children:

```go
dept, _ := deptRepo.Get(ctx, deptID, def.WithEdge("children"))
children := dept.Edges["children"].([]Department)
```

For full tree traversal, use a recursive activity (not a query — avoids N+1 at depth):

```go
func (a *Activities) LoadDepartmentTreeActivity(ctx context.Context, rootID uuid.UUID) (DepartmentTree, error) {
    // Use PostgreSQL recursive CTE for efficient tree loading
    // This is one of the few cases where raw pgx SQL is appropriate
    // (recursive CTE cannot be expressed in the Filter DSL)
}
```

---

## 7. Cross-Module Edges

An entity in one module linking to an entity in another module:

```go
// finance_invoice links to crm_customer (different modules)
{
    Name:       "customer",
    Type:       def.FieldLink,
    LinkTarget: "crm_customer",  // Framework resolves via EntityRegistry, not import
}
```

The framework resolves cross-module links via the EntityRegistry — no Go import of the CRM module is required in the Finance module. The registry knows the schema of `crm_customer` and can load it.

Loading cross-module edges works identically to same-module edges:

```go
invoice, _ := repo.Get(ctx, invoiceID, def.WithEdge("customer"))
```

---

## Related Documents

- [Edges](edges.md) — EdgeDef specification
- [Query Options](../05-persistence/query-options.md) — `WithEdge`, `WithEdgeFilter`, `WithEdgeSort`
- [Module Boundaries](../02-architecture/module-boundaries.md) — why cross-module edges don't require imports
- [Filter DSL](../05-persistence/filter-dsl.md) — filtering on FK fields
