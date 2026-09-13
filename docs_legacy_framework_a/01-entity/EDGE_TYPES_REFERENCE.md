> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

# Edge Types Reference

**Classification:** Reference — Tier 1
**Owner:** `01-entity/EDGE_TYPES_REFERENCE.md`
**Status:** Frozen at v1.0
**Package:** `awo.so/awo/def`

---

## Purpose

This document specifies all `EdgeDef` fields and `EdgeType` constants. Edges declare relationships between entities. The compiler generates FK constraints, indexes, and cascade rules based on edge declarations.

---

## EdgeDef Structure

```go
type EdgeDef struct {
    Name          string    // snake_case relationship name; unique within the entity
    Target        string    // qualified entity name (e.g., "finance_invoice_line")
    Type          EdgeType  // one of: EdgeOneToMany, EdgeManyToOne, EdgeManyToMany
    CascadeDelete bool      // if true, deleting this entity deletes all Target records
    Label         string    // human-readable; derived from Name if empty
}
```

---

## EdgeType Constants

### EdgeOneToMany

```
Entity A has many Target records.
Target holds a FK column referencing Entity A.
```

The most common relationship type. Use when one record owns a collection of related records.

**Example:** An `Invoice` has many `InvoiceLine` records.

```go
// On InvoiceDefinition:
Edges: []def.EdgeDef{
    {
        Name:          "lines",
        Target:        "finance_invoice_line",
        Type:          def.EdgeOneToMany,
        CascadeDelete: true,  // deleting the invoice deletes all lines
    },
},
```

**Compiler output:**
- Verifies that `finance_invoice_line` has a `FieldTypeLink` field targeting `finance_invoice`
- Documents the relationship in the OpenAPI spec
- Generates the SDUI child table widget for the detail view

**CascadeDelete:** When `true`, the compiler generates `ON DELETE CASCADE` on the FK column in the target entity's migration. When `false`, the compiler generates `ON DELETE RESTRICT`.

---

### EdgeManyToOne

```
Entity A belongs to one Target record.
Entity A holds the FK column referencing Target.
```

The inverse of `EdgeOneToMany`. Usually declared on the "child" side of the relationship for clarity in the def.

**Example:** An `InvoiceLine` belongs to one `Invoice`.

```go
// On InvoiceLineDefinition:
Edges: []def.EdgeDef{
    {
        Name:   "invoice",
        Target: "finance_invoice",
        Type:   def.EdgeManyToOne,
    },
},
```

**Note:** The FK column itself is declared as a `FieldTypeLink` field. The `EdgeManyToOne` declaration is documentary — it tells the SDUI generator and OpenAPI spec about the relationship.

---

### EdgeManyToMany

```
Entity A has many Target records.
Target has many Entity A records.
A junction table manages the relationship.
```

Use when two entities have an unordered, bidirectional many-to-many relationship.

**Example:** A `Product` belongs to many `Categories`; a `Category` contains many `Products`.

```go
// On ProductDefinition:
Edges: []def.EdgeDef{
    {
        Name:   "categories",
        Target: "catalog_category",
        Type:   def.EdgeManyToMany,
    },
},
```

**Implementation:** The developer MUST create a junction entity (e.g., `catalog_product_category`) with two `FieldTypeLink` fields. The `EdgeManyToMany` declaration documents the logical relationship.

**CascadeDelete:** Has no direct effect on many-to-many edges. Cascade behavior is declared on the junction entity's `FieldTypeLink` fields.

---

## CascadeDelete Semantics

| Value | Behavior |
|-------|---------|
| `true` | Target records are deleted when the owning entity is deleted. Generates `ON DELETE CASCADE` on the FK |
| `false` (default) | Delete is rejected if Target records exist. Generates `ON DELETE RESTRICT` |

**Warning:** Cascade deletes are irreversible. Use them only when the target records have no independent existence (e.g., invoice lines owned by an invoice). For entities that may be referenced by multiple owners or that have their own lifecycle, use `RESTRICT` and handle deletion explicitly.

---

## Edge Resolution at Compile Time

The compiler validates all edge targets at compile time:

1. The `Target` qualified name MUST exist in the registry.
2. For `EdgeOneToMany`: the target entity MUST have a `FieldTypeLink` field that targets this entity. If not found, the compiler emits a warning (not an error — the link may be on the other side).
3. For `EdgeManyToOne`: the declaration is documentary only; no structural validation beyond target existence.

Unresolved targets cause a compilation error that is fatal at startup.

---

## Edges vs. FieldTypeLink

Both edges and `FieldTypeLink` fields express relationships. They serve different purposes:

| Concern | Use |
|---------|-----|
| FK column in the database | `FieldTypeLink` field on the "many" side |
| Relationship documentation in OpenAPI | `EdgeDef` on either or both sides |
| SDUI child table rendering | `EdgeOneToMany` on the parent |
| Cascade delete | `EdgeDef.CascadeDelete: true` |
| Autocomplete widget | `FieldTypeLink` field with `CompiledLookup` |

A complete relationship typically requires both:
- A `FieldTypeLink` field on the child entity
- An `EdgeOneToMany` or `EdgeManyToOne` declaration on the appropriate side

---

## Lazy Loading Prohibition

Edges MUST NOT be lazy-loaded. Declaring an edge does not automatically load related records in query responses. Related data MUST be explicitly requested via query options.

This is enforced by architectural convention, not the type system. Violating this rule produces N+1 query patterns.

---

## References

- `awo/def/edge.go` — EdgeDef and EdgeType declarations
- [`01-entity/ENTITY_DEFINITION_SPEC.md`](ENTITY_DEFINITION_SPEC.md) — EntityDefinition interface
- [`15-migrations/MIGRATION_GUIDE.md`](../15-migrations/MIGRATION_GUIDE.md) — FK constraint generation
