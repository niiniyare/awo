> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Fields"
id: dom-001
status: accepted
category: SPEC
stability: FROZEN
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[EntityDefinition](../03-kernel/entity-def.md)"
  - "[Edges](edges.md)"
  - "[Filter DSL](../05-persistence/filter-dsl.md)"
  - "[Architecture Laws](../02-architecture/laws.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Fields

**DOM-001 | Status: Accepted | Stability: Frozen**

This document specifies the `FieldDef` type, all canonical `FieldType` handlers, field constraints, serialization rules, and field naming requirements.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, SHOULD NOT, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## Table of Contents

1. [FieldDef](#1-fielddef)
2. [Field Constraints](#2-field-constraints)
3. [Scalar Field Types](#3-scalar-field-types)
4. [Structured Field Types](#4-structured-field-types)
5. [Relational Field Types](#5-relational-field-types)
6. [Custom Field Types](#6-custom-field-types)
7. [Serialization Rules](#7-serialization-rules)
8. [Naming Rules](#8-naming-rules)
9. [System Columns](#9-system-columns)

---

## 1. FieldDef

```go
type FieldDef struct {
    Name        string      // required; snake_case; stable
    Type        FieldType   // required; open string type
    Label       string      // optional; defaults to Title(Name)
    Required    bool        // reject nil/empty on create
    Unique      bool        // database unique constraint
    Immutable   bool        // reject changes on update
    Sensitive   bool        // exclude from logs and standard responses
    Searchable  bool        // add GIN trigram index; only for Data type
    MaxLen      int         // max character length; for Data/SmallText/LongText
    Min         float64     // minimum value; for Int/Float
    Max         float64     // maximum value; for Int/Float
    Default     any         // literal default; applied if field absent on create
    Options     []string    // enum options; for Select/MultiSelect
    LinkTarget  string      // target entity name; for Link/LinkList
    Series      string      // format string; for NamingSeries
    TenantOverridable bool  // allow tenant to override prefix; for NamingSeries
    Validators  []FieldValidator
}
```

---

## 2. Field Constraints

### Required

When `Required: true`, the framework rejects any `CreateInput` where the field is absent or nil. On `UpdateInput`, a `Required` field may be omitted (meaning: do not change the existing value).

### Unique

Adds a `UNIQUE` constraint on the database column (for System Entities) or a unique index on the JSONB path (for Custom Entities). The store layer translates a unique constraint violation (`23505`) into a `BusinessError{Code: "duplicate", Status: 409}`.

### Immutable

The field value may be set on create and is thereafter read-only. The store layer rejects any `UpdateInput` containing an `Immutable` field with a value different from the persisted value, returning `BusinessError{Code: "immutable_field", Status: 400}`.

Common uses: record numbers (`NamingSeries`), creation timestamps embedded as data fields, foreign keys that define the record's ownership.

### Sensitive

The field value MUST NOT appear in:
- Structured log entries
- Error messages returned to callers
- Standard API responses (unless the endpoint explicitly serves the field)
- Audit log entries (the audit log records the field name was changed, not the new value)

The framework's log middleware automatically redacts `Sensitive` fields from logged `EntityRecord` and `CreateInput`/`UpdateInput` values. See [LAW-013](../02-architecture/laws.md#law-013-sensitive-fields-are-never-logged).

### Searchable

Adds a PostgreSQL GIN trigram index on the column, enabling efficient substring search via the `Contains` and `StartsWith` filter operators. Valid only for `Data` type. The `pg_trgm` extension must be installed.

### Validators

Custom validation functions called during the `before_validate` lifecycle stage. Each `FieldValidator` receives the field value and may return a `ValidationError`.

```go
type FieldValidator interface {
    Validate(ctx context.Context, value any) error
}
```

---

## 3. Scalar Field Types

### Data

```
FieldType: "Data"
PostgreSQL: varchar(n)  — n defaults to 255, overridden by MaxLen
Go type: string
```

Short strings. Indexed by default with a B-tree index. When `Searchable: true`, adds a GIN trigram index for substring search.

Use for: names, codes, short identifiers, reference numbers, email addresses, phone numbers.

### SmallText

```
FieldType: "SmallText"
PostgreSQL: varchar(1024)
Go type: string
```

Medium prose. No B-tree index (too large for efficient B-tree use). No GIN index generated automatically.

Use for: descriptions, addresses, short notes. Not for free-form text — use LongText.

### LongText

```
FieldType: "LongText"
PostgreSQL: text
Go type: string
```

Unbounded text. No index. For full-text search on LongText fields, add a `tsvector` generated column in a migration.

Use for: notes, comments, document bodies, configuration blobs that must remain human-readable.

### Int

```
FieldType: "Int"
PostgreSQL: bigint
Go type: int64
```

64-bit integer. Use for: counts, quantities, sequence numbers, years. MUST NOT be used for monetary values.

### Float

```
FieldType: "Float"
PostgreSQL: double precision
Go type: float64
```

IEEE 754 double-precision float. Use for: percentages, ratios, scientific measurements, latitude/longitude. MUST NOT be used for monetary values under any circumstances. See [INV-008](../02-architecture/invariants.md#inv-008-all-monetary-arithmetic-uses-exact-decimal-representation).

### Currency

```
FieldType: "Currency"
PostgreSQL: numeric(20,4)
Go type: decimal.Decimal  (shopspring/decimal)
```

Exact decimal with 20 significant digits and 4 decimal places. The ONLY correct field type for monetary values.

Serialization: JSON string in decimal format (`"1234.5600"`). API responses may include locale-formatted string (`"KES 1,234.5600"`) in a separate display field, but the raw value MUST remain the decimal string.

The `decimal.Decimal` type provides: exact arithmetic, comparison, rounding with configurable mode, and JSON marshaling/unmarshaling. All monetary arithmetic MUST use this type throughout the stack.

### Bool

```
FieldType: "Bool"
PostgreSQL: boolean
Go type: bool
```

Boolean flag. Serialized as JSON `true`/`false`.

### Date

```
FieldType: "Date"
PostgreSQL: date
Go type: time.Time (date portion only; time component zeroed)
```

Calendar date without time. Serialized as `"2024-12-15"` (ISO 8601 date).

### DateTime

```
FieldType: "DateTime"
PostgreSQL: timestamptz
Go type: time.Time
```

Timestamp with timezone. Stored internally as UTC. Serialized as ISO 8601 with East Africa Time (EAT, UTC+3) offset for Kenyan tenants: `"2024-12-15T14:30:00+03:00"`.

The EAT offset is applied at serialization time based on the tenant's locale configuration. The stored value is always UTC.

### Time

```
FieldType: "Time"
PostgreSQL: time
Go type: time.Time (time portion only; date component set to zero date)
```

Time of day without date. Serialized as `"14:30:00"`.

---

## 4. Structured Field Types

### Select

```
FieldType: "Select"
PostgreSQL: varchar(n) with CHECK (col IN (...))
Go type: string
```

Enumerated string. `Options []string` is required and must contain at least one value. The PostgreSQL `CHECK` constraint enforces that the stored value is always one of the declared options.

`Default` may be set to any value in `Options`. If `Default` is not set and `Required` is false, the field defaults to NULL.

### MultiSelect

```
FieldType: "MultiSelect"
PostgreSQL: text[]
Go type: []string
```

Array of enumerated strings. Each element must be in `Options`. No direct database `CHECK` constraint — validated at the application layer during `before_validate`.

Serialized as a JSON array of strings: `["Tag1", "Tag2"]`.

### NamingSeries

```
FieldType: "NamingSeries"
PostgreSQL: varchar(64) UNIQUE NOT NULL
Go type: string
```

Auto-generated sequential identifier. Format specified in `Series` field:

| Placeholder | Meaning |
|---|---|
| `{YYYY}` | 4-digit current year |
| `{YY}` | 2-digit current year |
| `{MM}` | 2-digit current month |
| `{DD}` | 2-digit current day |
| `{SEQ:N}` | Zero-padded sequence number, N digits wide |

Examples:
- `"INV-{YYYY}-{SEQ:5}"` → `"INV-2024-00042"`
- `"PO-{YY}{MM}-{SEQ:4}"` → `"PO-2412-0017"`

The sequence counter is atomic per tenant per series prefix. It resets at the period implied by the format string (yearly if `{YYYY}` present, monthly if `{MM}` present, never if neither).

When `TenantOverridable: true`, tenants may configure a different prefix via the Settings module. The `{SEQ:N}` part is not overridable.

`NamingSeries` fields are assigned by the framework in the `after_create` hook, after the record receives its ID. The field is always `Immutable` (set-once behavior is enforced regardless of the `Immutable` flag setting).

### JSON

```
FieldType: "JSON"
PostgreSQL: jsonb
Go type: map[string]any or json.RawMessage
```

Freeform JSONB. GIN indexed automatically. Use for: unstructured configuration blobs, flexible metadata, integration payloads.

Prefer declaring structured fields over `JSON` fields wherever possible. `JSON` fields are opaque to the Filter DSL's type system.

---

## 5. Relational Field Types

### Link

```
FieldType: "Link"
PostgreSQL: uuid REFERENCES {target_table}(id)
Go type: uuid.UUID
```

A typed foreign key. `LinkTarget` must name a registered entity. The framework generates:
- A `uuid` column
- A foreign key constraint referencing the target table's `id` column
- A B-tree index on the column

When the record is serialized in API responses, the linked record's `Label` field (or `Name` if no `Label`) may be included as a display value alongside the UUID.

### LinkList

```
FieldType: "LinkList"
PostgreSQL: uuid[]
Go type: []uuid.UUID
```

Array of foreign keys. `LinkTarget` required. No database FK constraint (array FK constraints are not supported in PostgreSQL). Referential integrity enforced at the application layer.

Use sparingly. Prefer `Edge` declarations with a join table for relationships that need FK enforcement.

### DynamicLink

```
FieldType: "DynamicLink"
PostgreSQL: two columns: {name}_type varchar(128), {name}_name uuid
Go type: DynamicLinkValue{Type string, ID uuid.UUID}
```

Polymorphic reference. Stores two values: the target entity type name and the target record ID. Used when a field may reference records from multiple entity types.

Example: an `attachment` entity might have a `DynamicLink` field `parent` that can reference either a `finance_invoice` or an `hr_employee`.

The filter DSL supports `DynamicLink` predicates using a compound key: `filter.DynamicLinkEq("parent", "finance_invoice", invoiceID)`.

---

## 6. Custom Field Types

Third-party modules may register additional field types by calling `def.RegisterFieldTypeHandler(typeName, handler)` from their `init()` function before any EntityDefinition using that type is registered.

The `FieldTypeHandler` interface requires implementing:
- `ColumnType() string` — PostgreSQL column DDL (e.g., `"uuid"`, `"jsonb"`, `"decimal(10,2)"`)
- `GoType() reflect.Type` — the Go type for reading from the database
- `Serialize(v any) (json.RawMessage, error)` — JSON serialization
- `Deserialize(raw json.RawMessage) (any, error)` — JSON deserialization
- `SDUIFieldSchema(def FieldDef) (json.RawMessage, error)` — amis field schema
- `FilterPredicates() []string` — supported filter operators

Custom field types must be documented in the registering module's documentation.

---

## 7. Serialization Rules

| FieldType | JSON wire format | Example |
|---|---|---|
| Data, SmallText, LongText | `string` | `"Invoice 42"` |
| Int | `number` | `42` |
| Float | `number` | `3.14` |
| Currency | `string` (decimal) | `"1234.5600"` |
| Bool | `boolean` | `true` |
| Date | `string` (ISO 8601 date) | `"2024-12-15"` |
| DateTime | `string` (ISO 8601 with TZ) | `"2024-12-15T14:30:00+03:00"` |
| Time | `string` (HH:MM:SS) | `"14:30:00"` |
| Select | `string` | `"Submitted"` |
| MultiSelect | `string[]` | `["Tag1","Tag2"]` |
| NamingSeries | `string` | `"INV-2024-00042"` |
| JSON | any valid JSON value | `{"key":"value"}` |
| Link | `string` (UUID) | `"a1b2c3d4-..."` |
| LinkList | `string[]` (UUIDs) | `["a1b2...","b2c3..."]` |
| DynamicLink | `object` | `{"type":"finance_invoice","id":"a1b2..."}` |

`Sensitive` field values are replaced with `null` in API responses unless the requesting actor has explicit permission to read the sensitive field.

---

## 8. Naming Rules

Field names MUST:
- Be lowercase
- Use underscores as separators (`snake_case`)
- Be unique within the entity
- Be stable after first deployment (embedded in migration column names and API payloads)

Field names MUST NOT:
- Use reserved system column names: `id`, `tenant_id`, `created_at`, `updated_at`, `created_by`, `custom_fields`
- Contain uppercase, hyphens, spaces, or special characters
- Begin with an underscore

---

## 9. System Columns

These columns are automatically added to every entity by the framework. They MUST NOT be declared in `Fields`:

| Column | Type | Notes |
|---|---|---|
| `id` | `uuid` (v7) | Primary key; time-ordered UUID |
| `tenant_id` | `uuid` | Foreign key to `tenants.id`; RLS enforcement column |
| `created_at` | `timestamptz` | Set on create; UTC; never updated |
| `updated_at` | `timestamptz` | Updated on every mutation |
| `created_by` | `uuid` | Actor UUID at creation time |
| `custom_fields` | `jsonb` | Tenant-defined custom field values; GIN indexed |

System columns are always included in API responses. They are not subject to field-level permission checks (all authenticated actors may read them).

---

## Related Documents

- [EntityDefinition](../03-kernel/entity-def.md) — Fields are declared within EntityDefinition
- [Edges](edges.md) — Relationships between entities
- [Filter DSL](../05-persistence/filter-dsl.md) — How fields are queried
- [Architecture Laws](../02-architecture/laws.md) — LAW-013 (sensitive fields)
- [Architecture Invariants](../02-architecture/invariants.md) — INV-008 (Currency type mandatory for money)
- [Glossary](../GLOSSARY.md) — FieldDef, FieldType, Sensitive, Currency, NamingSeries
