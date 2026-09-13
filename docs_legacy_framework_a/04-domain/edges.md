> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Edges"
id: dom-002
status: accepted
category: SPEC
stability: FROZEN
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[EntityDefinition](../03-kernel/entity-def.md)"
  - "[Fields](fields.md)"
  - "[EntityRepository](../05-persistence/entity-repository.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Edges

**DOM-002 | Status: Accepted | Stability: Frozen**

This document specifies the `EdgeDef` type, edge loading semantics, cascade delete behavior, and the prohibition on lazy loading.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, SHOULD NOT, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. EdgeDef

```go
type EdgeDef struct {
    Name          string    // required; snake_case; stable
    Target        string    // required; entity name of the related entity
    Type          EdgeType  // required: OneToOne, OneToMany, ManyToMany
    CascadeDelete bool      // delete related records when this record is deleted
    Label         string    // optional; used in SDUI list column headers
    ForeignKey    string    // optional; override FK column name (default: "{owning_entity}_id")
}
```

---

## 2. Edge Types

### OneToOne

One record in the owning entity has exactly one related record in the target entity. The FK column lives on the target entity's table (e.g., `invoice_pdf.invoice_id`).

```go
// invoice owns one invoice_pdf
EdgeDef{
    Name:   "pdf",
    Target: "finance_invoice_pdf",
    Type:   entity.OneToOne,
}
```

### OneToMany

One record in the owning entity has zero or more related records in the target entity. The FK column lives on the target entity's table.

```go
// invoice owns many invoice_lines
EdgeDef{
    Name:          "lines",
    Target:        "finance_invoice_line",
    Type:          entity.OneToMany,
    CascadeDelete: true,
}
```

### ManyToMany

Records in the owning entity share related records with other records. Requires an explicit join entity (not a framework-generated join table). The join entity must be declared as its own `EntityDefinition` with two `Link` fields.

```go
// product has many tags; tag has many products
// Requires: finance_product_tag EntityDefinition with fields {product_id, tag_id}
EdgeDef{
    Name:   "tags",
    Target: "inventory_product_tag",
    Type:   entity.ManyToMany,
}
```

---

## 3. Explicit Loading — No Lazy Loading

Edges are NEVER lazily loaded. This is an absolute constraint, not a default behavior.

To load an edge, pass one or more `WithEdge` QueryOptions to `Get()` or `Query()`:

```go
// Load invoice with lines and customer in one operation
invoice, err := invoiceRepo.Get(ctx, invoiceID,
    entity.WithEdge("lines"),
    entity.WithEdge("customer"),
)

// invoice.Edges["lines"]   — []EntityRecord (loaded)
// invoice.Edges["customer"] — EntityRecord (loaded)
```

An edge not included in `WithEdge` options has a nil value in `EntityRecord.Edges`. Accessing a nil edge is a programming error; the framework does not silently return empty slices for unloaded edges.

### Why No Lazy Loading

Lazy loading produces N+1 queries when records are displayed in lists. In an ERP list page displaying 50 invoices, lazy loading of the `customer` edge produces 51 queries (1 for the list + 50 for each customer). Explicit loading produces 2 queries (1 for invoices + 1 IN query for all customers).

At ERP scale — lists of hundreds of records, deeply nested relationships — N+1 queries produce unacceptable performance and unpredictable database load.

Explicit loading forces the access pattern to be declared at the call site, making query counts auditable and predictable.

---

## 4. Edge Loading Implementation

The store layer implements edge loading using one of two strategies:

**JOIN strategy** (for OneToOne, small OneToMany): a single SQL JOIN retrieves both the owning record and related records in one query.

**IN query strategy** (for OneToMany with high cardinality, ManyToMany): the owning records are fetched first, then related records are fetched with `WHERE foreign_key IN (id1, id2, ...)` and assembled in Go.

The strategy selection is an implementation detail of the store layer. Module authors observe only the result: related records available in `EntityRecord.Edges`.

---

## 5. Cascade Delete

When `CascadeDelete: true`, deleting the owning record automatically deletes all related records in the target entity through the edge.

Cascade delete is implemented at the database layer using `ON DELETE CASCADE` on the FK constraint. It is not implemented by the framework issuing multiple `DELETE` statements — the database handles it atomically.

Cascade delete fires the `before_delete` and `after_delete` hooks on the owning record only. Related records are deleted by the database without traversing their individual lifecycle hooks. If hook execution on deleted related records is required, implement it in the owning entity's `before_delete` hook using the `EntityRepository`.

### Cascade Delete and Audit Log

When cascade delete removes related records at the database level, the audit log does not automatically receive entries for each deleted related record. If audit log completeness for cascade-deleted records is required, implement explicit deletion with hook traversal in the owning entity's `before_delete` hook.

---

## 6. Edge Naming Rules

Edge names MUST:
- Be lowercase, snake_case
- Be unique within the entity
- Reflect the relationship semantically (e.g., `lines` for invoice lines, `customer` for the linked customer)
- Be stable after first deployment

Edge names MUST NOT conflict with field names on the same entity.

---

## 7. SDUI Representation

Edges in SDUI:

- `OneToMany` edges on a detail view are rendered as embedded sub-tables (list of related records inline)
- `OneToOne` edges on a detail view are rendered as an embedded detail panel
- `ManyToMany` edges are rendered as tag-select or multi-link fields

Custom rendering for any edge is achieved by overriding the relevant view in `PageBuilderSet`.

---

## Related Documents

- [EntityDefinition](../03-kernel/entity-def.md) — Edges declared within EntityDefinition
- [Fields](fields.md) — Link/LinkList/DynamicLink fields (not the same as Edges)
- [EntityRepository](../05-persistence/entity-repository.md) — QueryOption.WithEdge specification
- [Hooks](hooks.md) — Cascade delete hook interaction
- [Glossary](../GLOSSARY.md) — Edge, EdgeDef, QueryOption
