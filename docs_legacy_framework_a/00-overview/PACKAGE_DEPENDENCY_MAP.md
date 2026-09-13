> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

# Awo Framework — Package Dependency Map

**Classification:** Reference — Tier 0
**Owner:** `00-overview/PACKAGE_DEPENDENCY_MAP.md`
**Status:** Frozen at v1.0

---

## Purpose

This document is the authoritative record of the `awo/*` package dependency graph. Every package boundary decision is recorded here with its rationale. No circular dependency is permitted. Any proposed change to the dependency graph MUST be recorded as an ADR before implementation.

---

## Dependency Graph (Acyclic)

```
Level 0 — No dependencies (import nothing within awo/*)
┌──────────┬──────────┬──────────┬──────────┐
│  def     │  filter  │  cache   │  outbox  │
└──────────┴──────────┴──────────┴──────────┘
         (also: sdui/widget — no internal deps)

Level 1 — Depend only on Level 0
┌──────────┐
│  audit   │  → def
└──────────┘
┌──────────┐
│  auth    │  → def
└──────────┘

Level 2 — Depend on Level 0 and Level 1
┌──────────┐
│  naming  │  → cache, def
└──────────┘
┌──────────┐
│ registry │  → def
└──────────┘

Level 3 — Depend on Level 0, 1, and 2
┌──────────┐
│ compiler │  → def, registry
└──────────┘
┌──────────────┐
│  sdui/amis   │  → sdui/widget
└──────────────┘

Level 4 — Framework integration layer
┌──────────┐
│   sdui   │  → compiler, sdui/widget, sdui/amis, def, cache
└──────────┘

Level 5 — Runtime (depends on all lower levels)
┌──────────┐
│ runtime  │  → def, auth, compiler, naming, cache, outbox, audit
└──────────┘
```

---

## Package Specifications

### `awo/def` — Framework Vocabulary

**One responsibility:** All types that module authors use to declare entities and implement hooks.

**Exports:**
- `EntityDefinition` interface (14 methods)
- `SystemDefinition`, `CustomDefinition` structs
- `FieldDef`, `EdgeDef`, `ActionDef`, `WorkflowTrigger`, `PageBuilderSet`
- `FieldType`, `EdgeType`, `EventType`, `ActionMethod` constants
- `HookSet` and all hook interfaces (`BeforeCreateHook`, `AfterCreateHook`, etc.)
- `ActionHandlerFunc`, `ActionContext`, `ActionResult`, `ActionRuntime`, `ActionEntityRepo`
- `ActionEvent`, `ActionWorkflowSpec`, `ActionNotification`, `ActionCache`
- `EntityRecord` with typed accessors (`Get`, `Set`, `GetString`, `GetInt`, `GetDecimal`, `GetUUID`)
- `Actor` struct with `IsPlatformAdmin()`, `IsServiceAccount()` methods
- `RecordMeta`, `TriggerContext`, `OperationType`
- `PermissionSet`
- `Register()`, `All()`, `Seal()`, `Lookup()` — global definition registry functions
- `QualifiedName()`, `LocalName()`, `DeriveLabel()`, `PluralizeLocal()`, `OpenAPITag()` — name derivation helpers

**Imports within awo/\*:** None.

**Why nothing:** The `def` package is imported by every other framework package and by every module author. Any import it acquires becomes a transitive dependency of all modules. Keeping it clean prevents import cycles and keeps compile times minimal.

---

### `awo/filter` — Composable Predicate DSL

**One responsibility:** All `Filter` constructors and the `Filter` struct.

**Exports:**
- `Filter` struct with `Kind`, `Field`, `Value`, `Lo`, `Hi`, `In`, `Sub`
- `Kind` type and all `Kind*` constants (`KindEq`, `KindAnd`, etc.)
- Constructors: `Eq`, `Neq`, `Gt`, `Gte`, `Lt`, `Lte`, `Between`, `In`, `NotIn`, `InUUIDs`, `InStrings`, `IsNull`, `IsNotNull`, `Contains`, `StartsWith`, `EndsWith`, `And`, `Or`, `Not`
- Custom-field constructors: `CustomEq`, `CustomGt`, `CustomLt`, `CustomIn`, `CustomIsNull`
- `Filter.String()` for debugging

**Imports within awo/\*:** None.

**Why nothing:** `filter` is used by module authors in `PolicyFunc` implementations. It is also used by the store layer to translate predicates to SQL. If it imported `def`, there would be a potential cycle. By staying clean, it can be used by any package.

---

### `awo/cache` — Cache and Counter Contracts

**One responsibility:** The `Cache` and `Counter` interfaces and their noop implementations.

**Exports:**
- `Cache` interface: `Get(ctx, key, dst)`, `Set(ctx, key, value, ttl)`, `Delete(ctx, key)`, `DeletePrefix(ctx, prefix)`
- `Counter` interface: `Increment(ctx, key, delta)`, `Get(ctx, key)`
- `NoopCache` — implements Cache with no-ops; used in tests
- `NoopCounter` — implements Counter with no-ops; used in tests
- `ErrMiss` — sentinel error returned by `Cache.Get` on miss

**Imports within awo/\*:** None.

**Why nothing:** The `cache` package defines the contract that the Redis implementation satisfies. It is consumed by `naming` (Counter) and by module action handlers via `ActionRuntime.Cache()`. Keeping it import-free prevents cycles.

---

### `awo/outbox` — Durable Delivery Contracts

**One responsibility:** Interface contracts for the event and workflow outbox workers.

**Exports:**
- `EventBroker` interface: `Publish(ctx, topic, tenantID, payload)` — delivers events to a message broker
- `WorkflowDispatcher` interface: `Dispatch(ctx, record WorkflowOutboxRecord)` — starts Temporal workflows
- `EventOutboxRecord` struct (mirrors the `event_outbox` table schema)
- `WorkflowOutboxRecord` struct (mirrors the `workflow_outbox` table schema)

**Imports within awo/\*:** None.

**Why nothing:** The outbox package defines contracts that infrastructure implementations satisfy. It must not import `def` (would create a cycle via `runtime`).

---

### `awo/audit` — Audit Record Contract

**One responsibility:** The `AuditRecord` type and the `Writer` interface.

**Exports:**
- `AuditRecord` struct: `ID`, `EntityName`, `RecordID`, `TenantID`, `ActorID`, `Operation`, `OccurredAt`, `Before map[string]any`, `After map[string]any`
- `Writer` interface: `Write(ctx, record AuditRecord) error`
- `NoopWriter` — implements Writer with no-ops; used for `AuditEnabled: false` entities

**Imports within awo/\*:** `def` (for `OperationType`).

---

### `awo/auth` — Authorization and Identity Contracts

**One responsibility:** ViewerContext propagation, Session struct, and PolicyEvaluator interface.

**Exports:**
- `ViewerContext` interface: `TenantID()`, `UserID()`, `ServiceAccountID()`, `Roles()`, `HasRole()`, `IsPlatformAdmin()`
- `WithViewer(ctx, viewer)` — embeds ViewerContext in context
- `ViewerFromContext(ctx)` — extracts ViewerContext; panics if absent
- `Session` struct: `Token`, `UserID`, `ServiceAccountID`, `TenantID`, `Roles`, `ExpiresAt`, `IssuedAt`, `DeviceID`, `IPAddress`, `RequestID`
- `Session.IsExpired(now)`, `Session.ToActor()`
- `PolicyEvaluator` interface: `CanPerform(ctx, viewer, object, action) (bool, error)`
- `DefaultViewer` — concrete implementation of `ViewerContext` constructed from a `Session`

**Imports within awo/\*:** `def` (for `Actor` in `Session.ToActor()`).

**Why only def:** `auth` is consumed by `runtime` and by the middleware layer. If it imported `registry` or `compiler`, it would create a cycle.

---

### `awo/naming` — Naming Series

**One responsibility:** Pattern allocation, counter management, and preview for `FieldTypeNamingSeries` fields.

**Exports:**
- `NamingSeriesService` struct with `Allocate(ctx, input)`, `Preview(pattern)`, `Validate(pattern)`
- `AllocateInput` struct: `Pattern`, `TenantID`, `EntityName`, `Year`, `Month`
- `ParsePattern(pattern)` — returns a `[]Token` (literal, YYYY, MM, DD, SEQ)
- `Render(tokens, ctx NamingFieldContext)` — produces the series value without allocating
- `CounterKey(pattern, tenantID, resetPeriod)` — derives the Redis key for a series counter

**Imports within awo/\*:** `cache` (Counter), `def` (EventType for trigger context).

---

### `awo/registry` — Definition Validation and Sealing

**One responsibility:** Validate a set of `EntityDefinition` values and produce a sealed, immutable registry.

**Exports:**
- `Registry` struct
- `Build()` — creates a registry from globally registered definitions (via `def.All()`)
- `BuildFrom(defs)` — creates an isolated registry from an explicit slice (for testing)
- `Registry.All()` — returns all definitions in deterministic order
- `Registry.Lookup(qualifiedName)` — O(1) definition lookup
- `Registry.Seal()` — makes the registry immutable

**Imports within awo/\*:** `def`.

---

### `awo/compiler` — Compilation

**One responsibility:** Transform a sealed `Registry` into a `CompiledSchema`.

**Exports:**
- `Compile(reg)` — the main compilation entry point
- `CompiledSchema` struct with `Entities`, `ByName`, `Routes`, `CapabilityGrants`, `Diagnostics`
- `EntitySchema` struct — compiled representation of one `EntityDefinition`
- `RouteDescriptor` struct — one auto-generated HTTP route
- `CapabilityGrant` struct — one authorization assertion (renamed from CasbinPolicy per ADR-011)
- `CompiledLookup` struct — metadata for Link/LinkList field autocomplete
- `Diagnostics` type — warnings and errors from the compilation phase

**Imports within awo/\*:** `def`, `registry`.

---

### `awo/sdui/widget` — WidgetTree IR

**One responsibility:** The intermediate representation between the SDUI generator and any renderer.

**Exports:**
- `Node` struct: `Kind`, `ID`, `Label`, `Name`, `Required`, `ReadOnly`, `Hidden`, `Props`, `Children`, `DataSource`, `Actions`
- `NodeKind` type and all constants: `NodePage`, `NodeForm`, `NodeList`, `NodeField`, `NodeSelect`, `NodeDate`, `NodeDateTime`, `NodeText`, `NodeTextArea`, `NodeNumber`, `NodeSwitch`, `NodeEditor`, `NodeSection`, `NodeTabs`, `NodeTable`, `NodeButton`, `NodeDialog`
- `DataSource` struct: `URL`, `Method`, `SendOn`, `LabelField`, `ValueField`
- `ActionNode` struct: `Label`, `ActionType`, `Level`, `Href`, `API`

**Imports within awo/\*:** None.

---

### `awo/sdui/amis` — amis Renderer

**One responsibility:** Convert a `*widget.Node` tree to amis JSON (`map[string]any`).

**Exports:**
- `Render(node *widget.Node) map[string]any` — converts WidgetTree to amis JSON
- `NodeKindToAmisType` — mapping table used internally

**Imports within awo/\*:** `sdui/widget`.

---

### `awo/sdui` — SDUI Generation

**One responsibility:** Generate WidgetTree schemas from `EntitySchema` definitions.

**Exports:**
- `Generator` struct
- `Generator.GetPage(ctx, entityName, view, viewer)` — returns `*widget.Node`
- `Generator.RenderAmis(node)` — convenience wrapper: `widget.Node` → amis JSON
- `PageView` type: `PageViewList`, `PageViewCreate`, `PageViewEdit`, `PageViewDetail`

**Imports within awo/\*:** `compiler`, `sdui/widget`, `sdui/amis`, `def`, `cache`.

---

### `awo/runtime` — Pipeline Execution

**One responsibility:** Execute the hook pipeline for every mutation and wire all framework services into `DefaultActionRuntime`.

**Exports (interfaces only; implementations are internal):**
- `Pipeline` struct — executes the 9 pipeline stages
- `RuntimeFactory` — constructs `DefaultActionRuntime` per request
- `EntityDriver` interface — the store layer adapter (pgx implementation is internal)
- `WorkflowRuntime` interface — the Temporal adapter
- `NotificationService` interface — the notification adapter
- `CacheInvalidator` interface — the SDUI cache invalidation adapter

**Imports within awo/\*:** `def`, `auth`, `compiler`, `naming`, `cache`, `outbox`, `audit`.

---

## Prohibited Dependencies

The following dependencies are permanently prohibited:

| Package | MUST NOT import |
|---------|----------------|
| `def` | Any `awo/*` package |
| `filter` | Any `awo/*` package |
| `cache` | Any `awo/*` package |
| `outbox` | Any `awo/*` package |
| `sdui/widget` | Any `awo/*` package |
| `audit` | `auth`, `registry`, `compiler`, `runtime`, `sdui/*`, `naming` |
| `auth` | `registry`, `compiler`, `runtime`, `sdui/*`, `naming`, `cache`, `outbox`, `audit` |
| `registry` | `compiler`, `runtime`, `sdui/*`, `naming`, `cache`, `outbox`, `audit`, `auth` |
| `compiler` | `runtime`, `sdui/*`, `naming`, `cache`, `outbox`, `audit`, `auth` |
| Any package | Its own dependents (no cycles) |

---

## References

- [`00-overview/ARCH_OVERVIEW.md`](ARCH_OVERVIEW.md) — Structural overview
- [`00-overview/DECISION_REGISTER.md`](DECISION_REGISTER.md) — ADR-001 through ADR-012
- Source: `awo/def/`, `awo/filter/`, `awo/auth/`, `awo/compiler/`, `awo/registry/`
