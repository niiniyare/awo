# Field Types Reference

**Classification:** Reference — Tier 1
**Owner:** `01-entity/FIELD_TYPES_REFERENCE.md`
**Status:** Frozen at v1.0
**Package:** `awo.so/awo/def`

---

## Purpose

This document is the exhaustive reference for all `FieldType` constants. For each type, it specifies:
- The PostgreSQL column type generated
- The Go value type in `EntityRecord.Data`
- The SDUI widget kind generated
- The Filter DSL predicates applicable
- Constraints and validation behavior
- Examples

---

## FieldDef Structure

```go
type FieldDef struct {
    // Required
    Name string    // snake_case; unique within the entity
    Type FieldType // one of the constants below

    // Display
    Label      string // human-readable; derived from Name if empty
    Placeholder string
    HelpText   string

    // Constraints
    Required   bool
    Unique     bool
    Immutable  bool   // set-once; rejected on update
    Sensitive  bool   // excluded from logs and standard responses
    Searchable bool   // generates GIN trgm index

    // Type-specific
    Default     func() any    // called at ASSEMBLE stage when field is absent
    MaxLen      int           // FieldTypeData: max varchar length
    Min         any           // FieldTypeInt, FieldTypeFloat, FieldTypeCurrency
    Max         any           // FieldTypeInt, FieldTypeFloat, FieldTypeCurrency
    Options     []string      // FieldTypeSelect, FieldTypeMultiSelect
    LinkTarget  string        // FieldTypeLink, FieldTypeLinkList: qualified entity name
    Series      string        // FieldTypeNamingSeries: pattern string
    Validators  []FieldValidator

    // Metadata
    AuditEnabled bool  // default true; set false to exclude field from audit diff
}
```

---

## Scalar Types

### FieldTypeData

```
PostgreSQL:  varchar(n)         n = MaxLen if set; 255 if not set
Go type:     string
Widget:      NodeText (input)
```

Short strings. Use for names, codes, IDs, email addresses, phone numbers.

**Constraints:**
- `MaxLen` sets the varchar length. Default is 255.
- `Searchable: true` generates a GIN trigram index (`gin_trgm_ops`) enabling `ILIKE '%value%'` at index speed.
- `Unique: true` generates a unique index on `(tenant_id, field_name)` for tenant-scoped uniqueness.

**Filter predicates:** `Eq`, `Neq`, `In`, `NotIn`, `IsNull`, `IsNotNull`, `Contains`, `StartsWith`, `EndsWith`

**Example:**
```go
{Name: "email", Type: def.FieldTypeData, Required: true, Unique: true, MaxLen: 320}
```

---

### FieldTypeSmallText

```
PostgreSQL:  varchar(1024)
Go type:     string
Widget:      NodeTextArea (2 rows)
```

Medium prose. Use for short descriptions, addresses, notes that fit on one screen. No B-tree index.

**Filter predicates:** `Eq`, `Neq`, `IsNull`, `IsNotNull`, `Contains` (table scan; no trgm index)

---

### FieldTypeLongText

```
PostgreSQL:  text
Go type:     string
Widget:      NodeTextArea (4 rows)
```

Unrestricted free-form text. No index. Use full-text search (tsvector) for FTS requirements, not Filter DSL.

**Filter predicates:** `IsNull`, `IsNotNull` only (other predicates cause full table scans)

---

### FieldTypeInt

```
PostgreSQL:  bigint
Go type:     int64
Widget:      NodeNumber (integer step)
```

For counts, quantities, sequences, version numbers. MUST NOT be used for monetary values.

**Constraints:** `Min`, `Max` generate SQL CHECK constraints.

**Filter predicates:** `Eq`, `Neq`, `Gt`, `Gte`, `Lt`, `Lte`, `In`, `NotIn`, `IsNull`, `IsNotNull`, `Between`

---

### FieldTypeFloat

```
PostgreSQL:  double precision
Go type:     float64
Widget:      NodeNumber (decimal step)
```

For scientific measurements, percentages, ratios, non-monetary decimal values. MUST NOT be used for monetary values (use `FieldTypeCurrency`).

**Warning:** IEEE 754 floating-point arithmetic is imprecise. Never sum float fields for financial reporting.

**Filter predicates:** `Eq`, `Neq`, `Gt`, `Gte`, `Lt`, `Lte`, `Between`

---

### FieldTypeCurrency

```
PostgreSQL:  numeric(20,4)
Go type:     decimal.Decimal   (shopspring/decimal)
Widget:      NodeNumber (currency step=0.0001)
```

**The only correct type for monetary values.** Stores up to 16 integer digits and 4 decimal places. `decimal.Decimal` provides arbitrary-precision arithmetic.

**Constraints:** `Min`, `Max` generate SQL CHECK constraints using `numeric` comparison.

**Serialization:** Returns as a string in JSON responses: `"1234.5600"`. Never as a float.

**Filter predicates:** `Eq`, `Neq`, `Gt`, `Gte`, `Lt`, `Lte`, `Between`

**Example:**
```go
{Name: "total", Type: def.FieldTypeCurrency, Required: true, Min: decimal.Zero}
```

---

### FieldTypeBool

```
PostgreSQL:  boolean
Go type:     bool
Widget:      NodeSwitch
```

**Filter predicates:** `Eq` (with true/false), `IsNull`, `IsNotNull`

---

### FieldTypeDate

```
PostgreSQL:  date
Go type:     time.Time  (date part only; time components are zero)
Widget:      NodeDate
```

Calendar dates without time component. Use for birth dates, invoice dates, due dates.

**Filter predicates:** `Eq`, `Neq`, `Gt`, `Gte`, `Lt`, `Lte`, `Between`, `IsNull`, `IsNotNull`

---

### FieldTypeDateTime

```
PostgreSQL:  timestamptz
Go type:     time.Time
Widget:      NodeDateTime
```

Timestamps with timezone. Always stored as UTC in PostgreSQL. Serialized as EAT-offset ISO 8601 for Kenyan tenants (`+03:00`).

**Filter predicates:** `Eq`, `Neq`, `Gt`, `Gte`, `Lt`, `Lte`, `Between`, `IsNull`, `IsNotNull`

---

### FieldTypeTime

```
PostgreSQL:  time
Go type:     time.Time  (time part only; date components are zero)
Widget:      NodeDateTime (time-only mode)
```

Time-of-day values. Use for shift start/end times, scheduled times.

**Filter predicates:** `Eq`, `Neq`, `Gt`, `Gte`, `Lt`, `Lte`, `Between`

---

## Structured Types

### FieldTypeSelect

```
PostgreSQL:  varchar(n) with CHECK (col IN ('opt1', 'opt2', ...))
Go type:     string
Widget:      NodeSelect (static options)
```

Single-value enumeration. Options are declared in `FieldDef.Options`.

**Constraints:**
- `Options` MUST be non-empty. The compiler generates a SQL CHECK constraint.
- `Default` SHOULD be set to one of the options. If not set, the field is empty on creation.
- Adding new options requires a migration to update the CHECK constraint.
- Removing options requires a data migration before the constraint update.

**Filter predicates:** `Eq`, `Neq`, `In`, `NotIn`, `IsNull`, `IsNotNull`

**Example:**
```go
{
    Name:    "status",
    Type:    def.FieldTypeSelect,
    Options: []string{"Draft", "Submitted", "Paid", "Cancelled"},
    Default: func() any { return "Draft" },
    Required: true,
}
```

---

### FieldTypeMultiSelect

```
PostgreSQL:  text[]  (array column)
Go type:     []string
Widget:      NodeSelect (multi=true)
```

Multiple-value enumeration. Each selected value is stored as an element in a PostgreSQL array.

**Filter predicates:** `Contains` (array contains value), `IsNull`, `IsNotNull`

---

### FieldTypeNamingSeries

```
PostgreSQL:  varchar(64)
Go type:     string  (populated by AfterCreate hook)
Widget:      NodeText (read-only)
```

Auto-incrementing pattern-based identifiers. Pattern examples:
- `INV-{YYYY}-{SEQ:5}` → `INV-2026-00042`
- `ORD-{MM}-{YYYY}-{SEQ:6}` → `ORD-07-2026-000001`

The value is `NULL` before the `AfterCreate` stage. The naming series allocator runs in `AfterCreate` within the transaction.

**Tokens:**
- `{YYYY}` — 4-digit year
- `{MM}` — 2-digit month
- `{DD}` — 2-digit day
- `{SEQ:N}` — zero-padded sequence counter, N digits wide

**Counter semantics:** The counter is stored in Redis under `naming:{pattern_hash}:{tenant_id}:{period}`. Atomic increment via `INCR`. Resets at the start of the period (monthly for `{MM}`, yearly for `{YYYY}`).

**Tenant override:** Setting `TenantOverridable: true` allows tenants to configure a custom prefix via the Settings module.

See [`07-naming/NAMING_SERIES_SPEC.md`](../07-naming/NAMING_SERIES_SPEC.md) for the complete specification.

---

### FieldTypeJSON

```
PostgreSQL:  jsonb  (GIN indexed)
Go type:     map[string]any
Widget:      NodeEditor (JSON editor)
```

Freeform JSONB documents. GIN indexed for containment queries.

**When to use:** Configuration blobs, metadata, extension fields, integration payloads.

**When NOT to use:** Do not store financial amounts, FK references, or structured data that needs filtering as JSON. Use typed fields for those.

**Filter predicates:** `IsNull`, `IsNotNull` (containment queries not yet exposed in Filter DSL)

---

## Relational Types

### FieldTypeLink

```
PostgreSQL:  uuid FK column + index
Go type:     uuid.UUID
Widget:      NodeSelect (server-side search autocomplete)
```

Foreign key to another entity. `LinkTarget` MUST be a qualified entity name that exists in the registry.

The compiler generates:
- A UUID column with FK constraint to the target entity's primary key
- A B-tree index on the FK column
- A `CompiledLookup` for the SDUI autocomplete widget

**Filter predicates:** `Eq`, `Neq`, `In`, `NotIn`, `IsNull`, `IsNotNull`

**Example:**
```go
{Name: "customer_id", Type: def.FieldTypeLink, LinkTarget: "crm_customer", Required: true}
```

---

### FieldTypeLinkList

```
PostgreSQL:  uuid[] (array of FK references)
Go type:     []uuid.UUID
Widget:      NodeSelect (multi=true, server-side search)
```

Multiple foreign key references. Stored as a PostgreSQL UUID array. Use when an entity must reference multiple records of another type.

**Alternative:** For large or ordered sets of references, consider a junction entity with two `FieldTypeLink` fields instead.

**Filter predicates:** `Contains` (array contains UUID), `IsNull`, `IsNotNull`

---

### FieldTypeDynamicLink

```
PostgreSQL:  two columns: link_type varchar(128), link_name varchar(255)
Go type:     struct{ Type string; Name string }
Widget:      Two fields rendered together
```

Polymorphic reference. Links to different entity types based on the `link_type` discriminator. Pattern: `link_type` contains the qualified entity name; `link_name` contains the referenced record ID.

Use sparingly. Prefer typed `FieldTypeLink` fields where the target entity is known at declaration time.

---

## Field Constraints

| Constraint | Behavior |
|-----------|---------|
| `Required: true` | VALIDATE stage returns `ValidationError` if field is absent or zero-value |
| `Unique: true` | Compiler generates unique index on `(tenant_id, field_name)`. PostgreSQL enforces at INSERT/UPDATE |
| `Immutable: true` | VALIDATE stage rejects updates that attempt to change this field. Set-once semantics |
| `Sensitive: true` | Excluded from: structured log output, API list responses (unless elevated permission), error messages |
| `Searchable: true` | Compiler generates GIN trigram index. Enables efficient `Contains`/`StartsWith`/`EndsWith` predicates |
| `MaxLen int` | Applies to `FieldTypeData` only. Sets varchar(n). Default 255 |
| `Min`, `Max` | Applies to `FieldTypeInt`, `FieldTypeFloat`, `FieldTypeCurrency`. Generates SQL CHECK constraint |

---

## Custom Field Validators

```go
type FieldValidator func(ctx context.Context, value any) error
```

Declared in `FieldDef.Validators`. Executed at the VALIDATE stage after built-in constraint checks. Return `nil` for valid values; return a `*ValidationError` for invalid values.

**Example:**
```go
func validateKENPhone(ctx context.Context, value any) error {
    phone, ok := value.(string)
    if !ok {
        return &def.ValidationError{Fields: map[string]string{"phone": "must be a string"}}
    }
    if !strings.HasPrefix(phone, "+254") {
        return &def.ValidationError{Fields: map[string]string{"phone": "must start with +254"}}
    }
    return nil
}

// In FieldDef:
{Name: "phone", Type: def.FieldTypeData, Validators: []def.FieldValidator{validateKENPhone}}
```

---

## References

- `awo/def/field.go` — FieldDef and FieldType declarations
- [`01-entity/ENTITY_DEFINITION_SPEC.md`](ENTITY_DEFINITION_SPEC.md) — EntityDefinition interface
- [`06-filter/FILTER_DSL_REFERENCE.md`](../06-filter/FILTER_DSL_REFERENCE.md) — Filter constructors
- [`10-sdui/SDUI_FIELD_WIDGET_MAP.md`](../10-sdui/SDUI_FIELD_WIDGET_MAP.md) — FieldType to widget mapping
