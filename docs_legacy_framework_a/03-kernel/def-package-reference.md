> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "def Package Reference"
id: kern-008
status: accepted
category: SPEC
stability: FROZEN
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[EntityDefinition](entity-def.md)"
  - "[Hook Pipeline](hook-pipeline.md)"
  - "[Route Generation](route-generation.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# `def` Package Reference

**KERN-008 | Status: Accepted | Stability: Frozen**

Complete type and function reference for `awo.so/awo/def` — the package that contains all EntityDefinition primitives consumed by module authors.

---

## 1. Import Path

```go
import "awo.so/awo/def"
```

This is the only framework package module authors import at the domain layer. All entity-shape declarations, field types, hook interfaces, and lifecycle types live here.

---

## 2. Core Types

### `EntityDefinition`

The central declaration that drives all five subsystems.

```go
type EntityDefinition struct {
    // Identity
    Name        string   // stable, snake_case, module_noun format
    Module      string   // owning module name
    Label       string   // human-readable singular
    LabelPlural string   // human-readable plural

    // Storage routing
    StorageModel StorageModel  // StorageSQL | StorageJSONB

    // Shape
    Fields  []FieldDef
    Edges   []EdgeDef

    // Behaviour
    Hooks       HookSet
    Policy      PolicyFunc
    Actions     []ActionDef
    Permissions PermissionSet

    // Async
    WorkflowTriggers []WorkflowTrigger

    // UI
    PageBuilders PageBuilderSet
}
```

### `StorageModel`

```go
type StorageModel int

const (
    StorageSQL  StorageModel = iota // typed columns, SQL-backed
    StorageJSONB                    // JSONB document in entity_records table
)
```

---

## 3. FieldDef

```go
type FieldDef struct {
    Name     string
    Type     FieldType
    Label    string    // defaults to Title(Name)

    // Constraints
    Required  bool
    Unique    bool
    Immutable bool     // set-once; reject on update
    Sensitive bool     // excluded from logs and standard API output

    // Validation
    MaxLen     int
    Min        float64
    Max        float64
    Validators []FieldValidator

    // Type-specific
    Options    []string // FieldTypeSelect, FieldTypeMultiSelect
    Default    any
    LinkTarget string   // FieldTypeLink: target entity name (string, not type)
    Series     string   // FieldTypeNamingSeries: format string

    // Indexing
    Searchable bool     // GIN trgm index on Data/SmallText fields
}
```

### `FieldType` Constants

| Constant | SQL Type | Go Type | Notes |
|---|---|---|---|
| `FieldTypeData` | `varchar(n)` | `string` | Short strings |
| `FieldTypeSmallText` | `varchar(1024)` | `string` | Medium prose |
| `FieldTypeLongText` | `text` | `string` | Free-form |
| `FieldTypeInt` | `bigint` | `int64` | Never money |
| `FieldTypeFloat` | `double precision` | `float64` | Science/percentages only |
| `FieldTypeCurrency` | `numeric(20,4)` | `decimal.Decimal` | All monetary values |
| `FieldTypeBool` | `boolean` | `bool` | — |
| `FieldTypeDate` | `date` | `time.Time` | Date only |
| `FieldTypeDateTime` | `timestamptz` | `time.Time` | UTC stored, EAT serialized |
| `FieldTypeTime` | `time` | `time.Time` | Time of day |
| `FieldTypeSelect` | `varchar(50)` + CHECK | `string` | Options list |
| `FieldTypeMultiSelect` | `text[]` | `[]string` | Multiple values |
| `FieldTypeJSON` | `jsonb` | `map[string]any` | Freeform |
| `FieldTypeLink` | `uuid` FK | `uuid.UUID` | FK to entity |
| `FieldTypeLinkList` | join table | `[]uuid.UUID` | Many refs |
| `FieldTypeDynamicLink` | two columns | `DynamicRef` | Polymorphic |
| `FieldTypeNamingSeries` | `varchar(50)` | `string` | Auto-generated ID |

---

## 4. EdgeDef

```go
type EdgeDef struct {
    Name          string
    Target        string    // target entity name (string)
    Type          EdgeType
    CascadeDelete bool
    FKField       string    // field on target that references this entity
}

type EdgeType int

const (
    EdgeOneToMany  EdgeType = iota
    EdgeManyToMany
)
```

---

## 5. Hook Interfaces

All hook interfaces live in `def`. Implementations live in module `hooks.go` files.

```go
// BeforeCreateHook runs before validation, outside any transaction
type BeforeCreateHook interface {
    BeforeCreate(ctx context.Context, rec *EntityRecord) error
}

// BeforeSaveHook runs after validation, before TX opens
type BeforeSaveHook interface {
    BeforeSave(ctx context.Context, rec *EntityRecord) error
}

// AfterSaveHook runs inside the TX, after PERSIST
// An error here causes rollback
type AfterSaveHook interface {
    AfterSave(ctx context.Context, rec *EntityRecord) error
}

// BeforeDeleteHook runs before deletion, outside TX
type BeforeDeleteHook interface {
    BeforeDelete(ctx context.Context, rec *EntityRecord) error
}

// AfterDeleteHook runs inside the delete TX
type AfterDeleteHook interface {
    AfterDelete(ctx context.Context, rec *EntityRecord) error
}

// HookSet groups all hook lists for a definition
type HookSet struct {
    BeforeCreate []BeforeCreateHook
    BeforeSave   []BeforeSaveHook
    AfterSave    []AfterSaveHook
    BeforeDelete []BeforeDeleteHook
    AfterDelete  []AfterDeleteHook
}
```

---

## 6. PolicyFunc

```go
// PolicyFunc returns a filter that is AND-ed into every query for this entity.
// Called at request time with the current context.
// Must be pure — no I/O, no repository calls.
type PolicyFunc func(ctx context.Context) *filter.Filter
```

---

## 7. EntityRecord

The runtime representation of any entity instance, regardless of storage model.

```go
type EntityRecord struct {
    ID       uuid.UUID
    TenantID uuid.UUID

    // Internal fields map — field name → value
    fields map[string]any
}

// Getters
func (r *EntityRecord) Get(field string) any
func (r *EntityRecord) GetString(field string) string
func (r *EntityRecord) GetInt(field string) int64
func (r *EntityRecord) GetDecimal(field string) decimal.Decimal
func (r *EntityRecord) GetBool(field string) bool
func (r *EntityRecord) GetUUID(field string) uuid.UUID
func (r *EntityRecord) GetTime(field string) time.Time

// Setter
func (r *EntityRecord) Set(field string, value any)

// Mutation input types
type CreateInput struct {
    Fields map[string]any
}

type UpdateInput struct {
    Fields map[string]any
}

type Patch = map[string]any
```

---

## 8. ActionDef

```go
type ActionDef struct {
    Name        string
    Method      ActionMethod   // ActionMethodPost | ActionMethodGet
    Label       string
    Permission  string         // role name that can call this action
    VisibleWhen string         // amis expression for button visibility
    Bulk        bool           // if true: route is /bulk/{action}
    InputSchema map[string]any // JSON Schema for request body validation
    HandlerFunc ActionHandlerFunc
}

type ActionHandlerFunc func(ctx context.Context, action ActionContext) (*ActionResult, error)

type ActionContext struct {
    Record        *EntityRecord
    RecordID      uuid.UUID
    BulkRecordIDs []uuid.UUID   // non-nil only for bulk actions
    Actor         session.Actor
    Repo          EntityRepository
    TemporalClient client.Client
    Body          map[string]any
    Params        map[string]string
    FiberCtx      *fiber.Ctx
}

type ActionResult struct {
    Record     *EntityRecord
    Message    string
    WorkflowID string
    HTTPStatus int   // default 200
}
```

---

## 9. WorkflowTrigger

```go
type WorkflowTrigger struct {
    On           LifecycleEvent
    WorkflowFn   string           // registered Temporal workflow function name
    TaskQueue    string
    Condition    func(*EntityRecord) bool   // nil = always trigger
    InputBuilder func(*EntityRecord, TriggerContext) (any, error)
}

type LifecycleEvent string

const (
    EventOnCreate  LifecycleEvent = "on_create"
    EventOnUpdate  LifecycleEvent = "on_update"
    EventOnDelete  LifecycleEvent = "on_delete"
    EventOnSubmit  LifecycleEvent = "on_submit"   // status → Submitted
    EventOnApprove LifecycleEvent = "on_approve"
    EventOnCancel  LifecycleEvent = "on_cancel"
)
```

---

## 10. PermissionSet

```go
type PermissionSet struct {
    Create []string   // role names
    Read   []string
    Write  []string
    Delete []string
    // Action permissions declared per-ActionDef
}
```

---

## 11. Registration

```go
// Register adds an EntityDefinition to the global registry.
// Must be called from a package-level init() function.
// Panics if called after registry is sealed (post-startup).
func Register(def *EntityDefinition)
```

---

## Related Documents

- [EntityDefinition](entity-def.md) — conceptual overview and usage guide
- [Hook Pipeline](hook-pipeline.md) — hook execution order, TX boundaries
- [Route Generation](route-generation.md) — how ActionDef generates routes
- [filter Package Reference](../05-persistence/filter-package-reference.md) — the Filter type used in PolicyFunc
