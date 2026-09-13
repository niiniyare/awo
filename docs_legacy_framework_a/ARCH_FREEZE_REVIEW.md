> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

# Awo Framework — Architecture Freeze Review

**Classification:** Architecture Review Board (ARB) Decision Document
**Date:** 2026-07-20
**Status:** Pre-freeze
**Scope:** All architectural decisions required before v1.0 implementation begins
**Assumes:** Implementation audit, invariant audit, correctness audit, distributed systems audit, evolution audit — all complete and findings understood

---

## Phase 1 — Architecture Freeze Readiness

### What Is Mature Enough to Freeze Forever

The following subsystems are designed to production quality and can be frozen immediately:

| Subsystem | Evidence of Maturity |
|-----------|---------------------|
| EntityDefinition contract | Interface is stable, SystemDefinition/CustomDefinition implement cleanly, validated by Registry |
| Field system | FieldDef, FieldType constants, constraint semantics are complete and tested against Finance module |
| Filter DSL | `filter.*` package is composable, engine-agnostic, implements `def.ActionFilter` cleanly |
| Hook pipeline | Lifecycle stages are correct: ASSEMBLE → before_validate → VALIDATE → before_save → [TX] → PERSIST → after_save → [TX commits] → Workflow |
| NamingSeries | Pattern lexer, renderer, and atomic counter semantics are complete |
| Registry validation | Build/BuildFrom, mandatory entity enforcement, field/edge validation — complete |
| RLS enforcement model | `set_tenant_context()` + PostgreSQL FORCE ROW LEVEL SECURITY is the correct, non-negotiable model |
| Multi-tenancy boundary | Tenant-per-context, not tenant-per-schema — correct for the scale profile |
| Cache interface | `cache.Cache` and `cache.Counter` are minimal, correct, implementation-agnostic |
| EntityRecord | Data map, typed accessors, CreatedAt/UpdatedAt — stable |

### What Remains Architecturally Undecided

Fourteen decisions remain open. They are ordered by severity:

1. **Authorization model unification** — PermissionSet declares roles as `[]string`, but no PolicyEvaluator interface exists; Casbin is assumed but not abstracted
2. **ViewerContext interface** — authorization context propagates through `context.Context` with no typed contract
3. **Grant model** — no `Grant` type; capabilities are expressed as string role names
4. **Audit ownership** — pipeline stage vs. opt-in hook is unresolved
5. **WidgetTree IR** — SDUI generator emits amis JSON directly; no intermediate representation
6. **Workflow outbox** — `StartWorkflow` failures have no durable retry
7. **Event outbox** — `Publish()` is described as outbox-backed but the outbox schema is not defined
8. **Idempotency model** — Create operations have no idempotency key support
9. **Actor.IsPlatformAdmin** — boolean flag on Actor is an architectural violation; should be a role
10. **Service account model** — no first-class service account type; machines act as users
11. **Session model** — no Session struct in the framework layer
12. **Hook recursion policy** — no documented rule or runtime guard against recursive hook calls
13. **Organization hierarchy** — multi-branch, multi-org within a tenant not designed
14. **Compiler output naming** — `CasbinPolicies` in CompiledSchema couples the compiler to Casbin

### Which Decisions Become Irreversible After v1.0

These are the decisions that **cannot be changed without breaking external module authors**:

- `EntityDefinition` interface method set — adding required methods breaks all implementations
- `def.EntityRecord` field types in `Data map[string]any` — changing type conventions breaks all hooks
- `filter.*` function signatures — external modules build filters; changing Eq/And/Or signatures breaks callers
- `def.ActionRuntime` interface — action handlers implement against this; adding methods breaks existing handlers
- `def.Actor` struct fields — action handlers read Actor; removing or renaming fields breaks handlers
- `def.PermissionSet` struct — module authors declare this; removing fields breaks declarations
- `awo.so/awo/*` package paths — changing the module path breaks all imports
- Entity naming convention (`{module}_{noun}`) — embedded in migration filenames, workflow IDs stored in Temporal history for years
- Hook interface signatures (`BeforeCreate(ctx, record)`) — module hook structs implement these

---

## Phase 2 — Critical Decisions

### Decision 1: Authorization Model — PermissionSet Is the Declaration Layer, PolicyEvaluator Is the Enforcement Layer

**The confusion:** Two audits identified a "PermissionSet vs CapabilitySet duality." This was misdiagnosed. There is no duality. There are two distinct layers that must be explicitly separated.

**Layer 1 — Declaration (frozen):** `PermissionSet` on `EntityDefinition`. Module authors declare which role subjects can perform which operations. This is a Go struct in `awo/def`. It is already correct.

**Layer 2 — Enforcement (not yet defined):** A `PolicyEvaluator` interface in `awo/auth`. The runtime consults it at request time. The default implementation is Casbin-backed.

```go
// awo/auth/evaluator.go
package auth

import "context"

// PolicyEvaluator is the single enforcement point for all capability decisions.
// The default implementation uses Casbin. Replace for ReBAC, OPA, or custom engines.
type PolicyEvaluator interface {
    // CanPerform returns true if the viewer can perform action on object within
    // the tenant encoded in ctx. object is a qualified entity name.
    // action is one of: "create", "read", "write", "delete", or a custom action name.
    CanPerform(ctx context.Context, viewer ViewerContext, object string, action string) (bool, error)
}
```

**Decision:** Keep `PermissionSet` as-is. Add `PolicyEvaluator` to `awo/auth`. The compiler reads `PermissionSet` declarations and produces a `CapabilityManifest` (rename from `CasbinPolicies`) that the runtime loads into the `PolicyEvaluator` implementation.

**Reject:** Replacing PermissionSet with a capability graph. PermissionSet is a declaration; a capability graph is a runtime structure. They serve different purposes. Frappe made this mistake (mixing declaration and runtime), requiring complex role inheritance resolution at request time.

**Long-term consequence:** PolicyEvaluator is replaceable by any engine. PermissionSet declarations are engine-agnostic.

**Changes public API:** Yes — adds `awo/auth` package. No breaking changes to existing declarations.

**Migration difficulty after v1.0:** Negligible — PolicyEvaluator is an implementation detail. Module authors never touch it.

---

### Decision 2: ViewerContext — Typed Context Key, Not Interface Parameter

**Current state:** Authorization context is absent. `ActionEntityRepo` methods take `context.Context`. The repo "applies RLS automatically via set_tenant_context" but application-layer authorization is unspecified.

**Option A — Explicit parameter:** `repo.Get(ctx, id, viewer ViewerContext)`. Forces callers to pass ViewerContext explicitly. Catches missing authorization at compile time.

**Option B — Context key (recommended):** ViewerContext is embedded in `context.Context` via a typed key. The runtime guarantees it is present. Repository implementations extract it.

**Decision:** Option B — context key with a typed accessor.

```go
// awo/auth/viewer.go
package auth

import (
    "context"
    "github.com/google/uuid"
)

// ViewerContext is the authorization surface for every operation.
// The middleware pipeline constructs it from the session and injects it
// into the request context before any handler or hook executes.
// All repository operations extract it from context to perform authorization.
type ViewerContext interface {
    // TenantID returns the tenant this viewer operates within.
    TenantID() uuid.UUID

    // UserID returns the user UUID, or uuid.Nil for service accounts.
    UserID() uuid.UUID

    // ServiceAccountID returns the service account UUID, or uuid.Nil for human users.
    ServiceAccountID() uuid.UUID

    // Roles returns the full set of role names assigned to this viewer.
    Roles() []string

    // HasRole returns true if the viewer holds the named role.
    HasRole(role string) bool

    // IsPlatformAdmin returns true if the viewer holds "role:platform-admin".
    // Replaces the Actor.IsPlatformAdmin boolean field.
    IsPlatformAdmin() bool
}

type viewerKey struct{}

// WithViewer embeds a ViewerContext in ctx.
func WithViewer(ctx context.Context, v ViewerContext) context.Context {
    return context.WithValue(ctx, viewerKey{}, v)
}

// ViewerFromContext extracts the ViewerContext from ctx.
// Panics if absent — the middleware guarantees presence.
func ViewerFromContext(ctx context.Context) ViewerContext {
    v, ok := ctx.Value(viewerKey{}).(ViewerContext)
    if !ok {
        panic("auth: ViewerContext missing from context — middleware not applied")
    }
    return v
}
```

**Reject Option A:** Making ViewerContext an explicit parameter on every repository method doubles the method signature size and makes mocking significantly harder. Kubernetes, Ent, and most production frameworks use context-embedded authorization contexts.

**Long-term consequence:** Repository interface remains clean. ViewerContext is always present or the process panics (fail-fast, correct behavior).

**Changes public API:** Adds `awo/auth` package. `ActionEntityRepo` does not change (already guaranteed to have viewer context by the runtime). `EntityRepository` in the store layer extracts viewer from context.

**Migration difficulty after v1.0:** Medium if a ViewerContext method needs to be added (interface extension). Mitigated by keeping ViewerContext minimal.

---

### Decision 3: Actor Model — Remove IsPlatformAdmin, Add ServiceAccountID

**Current state:** `def.Actor{UserID, TenantID, Roles []string, IsPlatformAdmin bool}`

**Problems:**
1. `IsPlatformAdmin bool` is an architectural violation — it is a privilege, not an identity attribute. It should be expressed as a role in `Roles`.
2. No service account type — machines must impersonate users.

**Decision:** Replace `Actor` with the following:

```go
// awo/def/actor.go
type Actor struct {
    // UserID is the UUID of the human user. uuid.Nil for service accounts.
    UserID uuid.UUID

    // ServiceAccountID is the UUID of the service account. uuid.Nil for humans.
    // Exactly one of UserID or ServiceAccountID is non-nil.
    ServiceAccountID uuid.UUID

    // TenantID is the tenant the actor operates within.
    TenantID uuid.UUID

    // Roles is the complete set of role names for this actor.
    // Platform admins carry "role:platform-admin" here.
    Roles []string
}

// IsServiceAccount returns true when the actor is a machine identity.
func (a *Actor) IsServiceAccount() bool {
    return a.ServiceAccountID != uuid.Nil
}

// IsPlatformAdmin returns true when the actor carries the platform-admin role.
func (a *Actor) IsPlatformAdmin() bool {
    for _, r := range a.Roles {
        if r == "role:platform-admin" {
            return true
        }
    }
    return false
}
```

**Reject:** Keeping IsPlatformAdmin bool. It is a shortcut that creates two authorization paths. The platform-admin privilege check in the authorization middleware becomes `viewer.HasRole("role:platform-admin")`, which is uniformly handled by PolicyEvaluator bypass logic.

**Long-term consequence:** `Actor` can be extended (add `DeviceID`, `IPAddress`) without breaking existing code. All privilege checks go through Roles.

**Changes public API:** Yes — removes `IsPlatformAdmin bool` from Actor. Breaking change for any code that reads this field. Must be changed before v1.0.

---

### Decision 4: Session Model — Frozen Struct in awo/auth

**Decision:**

```go
// awo/auth/session.go
type Session struct {
    Token            string
    UserID           uuid.UUID
    ServiceAccountID uuid.UUID // uuid.Nil for human sessions
    TenantID         uuid.UUID
    Roles            []string
    ExpiresAt        time.Time
    IssuedAt         time.Time
    DeviceID         string
    IPAddress        string
    RequestID        string
}

// IsExpired returns true when the session has passed its expiry time.
func (s *Session) IsExpired(now time.Time) bool { return now.After(s.ExpiresAt) }

// ToActor converts the session to an Actor for hook and action handler injection.
func (s *Session) ToActor() *def.Actor {
    return &def.Actor{
        UserID:           s.UserID,
        ServiceAccountID: s.ServiceAccountID,
        TenantID:         s.TenantID,
        Roles:            s.Roles,
    }
}
```

Redis key: `session:{token}`. JSON-serialised. TTL = ExpiresAt − now.

**Long-term consequence:** Session struct is private to the framework. Module code never constructs Sessions. If fields are added, they are backward compatible.

---

### Decision 5: Audit — Pipeline Stage, Not Optional Hook

**Current state:** Audit is described as a platform module using opt-in hooks.

**Decision:** Audit is a mandatory pipeline stage inserted by the runtime between PERSIST and after_save. Module authors cannot suppress it.

```
PERSIST → AUDIT RECORD → after_save → [TX commits]
```

The audit stage is controlled by `EntityDefinition.AuditEnabled bool` (default: `true`). Setting `AuditEnabled: false` is legal only for high-frequency read-only entities where audit volume would be prohibitive (e.g., counters, metrics snapshots).

The audit record is written within the same transaction. If the transaction rolls back, the audit record is also rolled back. This is correct — a rolled-back write should not produce an audit entry.

```go
// Added to SystemDefinition and CustomDefinition:
AuditEnabled bool // default true; set false only for non-sensitive high-volume entities
```

**Reject:** Opt-in hooks. The audit trail exists for legal compliance and security forensics. Modules that forget to add the audit hook create silent compliance gaps. This is categorically wrong for an ERP framework handling financial data.

**Long-term consequence:** The `audit_log` table will be large. Infrastructure must plan for partitioning by month or tenant. This is a known and acceptable trade-off.

**Changes public API:** Adds `AuditEnabled bool` to EntityDefinition declaration structs. Non-breaking (defaults true).

---

### Decision 6: SDUI — WidgetTree IR Is Required Before v1.0

**Current state:** `sdui.Generator` emits `map[string]any` amis JSON directly. Zero abstraction.

**Why this is the highest-risk open decision:** Replacing amis without a WidgetTree IR requires rewriting every page generator. The amis SDK is pinned and cannot be auto-updated. React/amis will eventually reach end-of-life. Without an IR, the framework is permanently coupled to one UI renderer.

**Decision:** Introduce `awo/sdui/widget` package with a `WidgetTree` IR. The generator produces WidgetTree; a renderer converts it to amis JSON.

```go
// awo/sdui/widget/tree.go

// Node is a UI widget. The tree represents the complete page schema.
type Node struct {
    Kind       NodeKind
    ID         string
    Label      string
    Name       string          // field binding
    Required   bool
    ReadOnly   bool
    Hidden     bool
    Props      map[string]any  // renderer-specific extras
    Children   []*Node
    DataSource *DataSource     // for list/select nodes with remote data
    Actions    []*ActionNode   // for action buttons
}

// NodeKind is the semantic widget type.
type NodeKind string

const (
    NodePage      NodeKind = "page"
    NodeForm      NodeKind = "form"
    NodeList      NodeKind = "list"
    NodeField     NodeKind = "field"
    NodeSelect    NodeKind = "select"
    NodeDate      NodeKind = "date"
    NodeDateTime  NodeKind = "datetime"
    NodeText      NodeKind = "text"
    NodeTextArea  NodeKind = "textarea"
    NodeNumber    NodeKind = "number"
    NodeSwitch    NodeKind = "switch"
    NodeEditor    NodeKind = "editor"     // JSON editor
    NodeSection   NodeKind = "section"    // visual grouping
    NodeTabs      NodeKind = "tabs"
    NodeTable     NodeKind = "table"      // child record table
    NodeButton    NodeKind = "button"
    NodeDialog    NodeKind = "dialog"
)
```

The amis renderer (`awo/sdui/amis`) converts `*widget.Node` trees to `map[string]any`.

**Reject:** Continuing to emit amis JSON directly. The amis SDK is pinned by version. Every upgrade requires a full compatibility audit. Without an IR, every upgrade is O(all page generators). With an IR, upgrades are O(one renderer).

**Long-term consequence:** A React Native renderer, a PDF renderer, or an alternative web renderer can be added without touching any EntityDefinition or generator code.

**Changes public API:** `sdui.Generator.GetPage()` return type changes from `map[string]any` to `*widget.Node` (with a `sdui.RenderAmis(node)` convenience wrapper for backward compatibility during transition).

**Migration difficulty after v1.0:** High if WidgetTree is not frozen now. Low if frozen now.

---

### Decision 7: Workflow Outbox — Required Infrastructure Contract

**Current state:** `ActionRuntime.StartWorkflow()` calls Temporal directly. On failure, the error is logged. The entity record is already saved. The workflow never starts and no retry occurs.

**Decision:** Workflow starts are always written to a `workflow_outbox` table within the entity's transaction, then dispatched by an outbox worker after commit. The outbox worker retries with exponential backoff (1s, 2s, 4s, up to 5 minutes) for 24 hours before marking the outbox record as `failed`.

```sql
-- Part of platform migration
CREATE TABLE workflow_outbox (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       uuid NOT NULL,
    entity_name     text NOT NULL,
    record_id       uuid NOT NULL,
    workflow_fn     text NOT NULL,
    task_queue      text NOT NULL,
    workflow_id     text,
    input           jsonb,
    status          text NOT NULL DEFAULT 'pending'
                        CHECK (status IN ('pending', 'dispatched', 'failed')),
    attempts        int  NOT NULL DEFAULT 0,
    last_error      text,
    created_at      timestamptz NOT NULL DEFAULT now(),
    next_attempt_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX workflow_outbox_pending ON workflow_outbox (next_attempt_at)
    WHERE status = 'pending';
```

The `ActionRuntime.StartWorkflow()` implementation writes to this table, not directly to Temporal. The outbox worker is a separate goroutine in the server process.

**Reject:** Direct Temporal calls with log-on-failure. This is a data consistency violation. The entity record exists; the workflow that should process it does not run. In financial contexts, this is a silent corruption.

**Long-term consequence:** All workflow starts are durable. The outbox table provides a complete audit of every workflow dispatch attempt.

---

### Decision 8: Event Outbox — Schema Is the Contract

**Decision:** The event outbox table schema is part of the platform migration and is the public contract:

```sql
CREATE TABLE event_outbox (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   uuid NOT NULL,
    topic       text NOT NULL,
    payload     jsonb NOT NULL,
    status      text NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'delivered', 'failed')),
    attempts    int  NOT NULL DEFAULT 0,
    last_error  text,
    created_at  timestamptz NOT NULL DEFAULT now(),
    deliver_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX event_outbox_pending ON event_outbox (deliver_at)
    WHERE status = 'pending';
```

`ActionRuntime.Publish()` writes to this table within the current transaction. An outbox worker delivers events to the configured message broker (Kafka, NATS, Redis PubSub — implementation detail). The framework provides the worker; the broker adapter is pluggable via an `EventBroker` interface in `awo/outbox`.

---

### Decision 9: Idempotency — X-Idempotency-Key at the HTTP Layer

**Decision:** The HTTP middleware layer supports idempotency for all POST and PATCH requests via `X-Idempotency-Key` header. Duplicate requests with the same key (scoped to tenant) within 24 hours return the stored response without re-executing the handler.

Storage: `idempotency_cache:{tenant_id}:{key}` in Redis. TTL: 24 hours. Value: HTTP status + response body JSON.

Module authors do not need to implement idempotency themselves. The framework handles it transparently.

This is required for safety in workflow contexts where activities retry HTTP calls to the ERP API.

---

### Decision 10: Hook Recursion Policy — Detected at Runtime, Fatal

**Decision:** The pipeline runtime maintains a goroutine-local entity name stack (via `context.WithValue` chain counting). If a hook on entity `A` triggers a Create/Update/Delete on entity `A` through any code path, the runtime panics with a descriptive message.

```
PANIC: hook recursion detected: finance_journal_entry hook called Create on finance_journal_entry
```

Cross-entity operations from hooks are explicitly allowed. A hook on `finance_invoice` may call Create on `finance_journal_entry`. A hook on `finance_journal_entry` may NOT call Create on `finance_journal_entry`.

This rule is enforced at runtime because compile-time detection would require whole-program analysis.

**Reject:** Documenting the rule without enforcement. Documentation alone fails silently. Runtime panic fails loudly at development time, not in production.

---

### Decision 11: Compiler Output — Rename CasbinPolicies to CapabilityGrants

**Decision:** `CompiledSchema.CasbinPolicies []CasbinPolicy` becomes `CompiledSchema.CapabilityGrants []CapabilityGrant`.

```go
// awo/compiler/schema.go
type CapabilityGrant struct {
    Subject    string // "role:finance.accountant"
    Object     string // "finance_journal_entry"
    Action     string // "create", "read", "write", "delete", or custom action name
}
```

The Casbin implementation reads `CapabilityGrant` slices and loads them. If the framework is used with OPA or another engine, that engine reads the same `CapabilityGrant` slices. The compiler knows nothing about Casbin.

**Long-term consequence:** The compiler output is engine-agnostic. Replacing the authorization engine does not require recompiling entity definitions.

---

### Decision 12: Organization Hierarchy — Deferred to v1.1, Guarded by Feature Flag

**Decision:** Multi-org, multi-branch within a tenant is deferred to v1.1. The framework does not define organization hierarchy structures in v1.0. Modules that need branch scoping implement it as a custom entity with a BranchScoped PolicyFunc.

This decision is explicitly documented to prevent module authors from building incompatible hierarchy implementations that would conflict with the v1.1 design.

---

## Phase 3 — Public Contracts

### Freeze Forever at v1.0

These interfaces, types, and functions are public API. Any breaking change requires a major version increment.

| Contract | Location | Frozen |
|----------|----------|--------|
| `def.EntityDefinition` interface (all 14 methods) | `awo/def` | ✅ v1.0 |
| `def.SystemDefinition` struct fields | `awo/def` | ✅ v1.0 |
| `def.CustomDefinition` struct fields | `awo/def` | ✅ v1.0 |
| `def.FieldDef` struct fields | `awo/def` | ✅ v1.0 |
| `def.EdgeDef` struct fields | `awo/def` | ✅ v1.0 |
| `def.FieldType` constants | `awo/def` | ✅ v1.0 |
| `def.EdgeType` constants | `awo/def` | ✅ v1.0 |
| `def.EventType` constants | `awo/def` | ✅ v1.0 |
| `def.ActionMethod` constants | `awo/def` | ✅ v1.0 |
| `def.HookSet` struct | `awo/def` | ✅ v1.0 |
| `def.BeforeCreateHook` — `def.AfterDeleteHook` interfaces | `awo/def` | ✅ v1.0 |
| `def.ActionHandlerFunc` signature | `awo/def` | ✅ v1.0 |
| `def.ActionContext` struct | `awo/def` | ✅ v1.0 |
| `def.ActionResult` struct | `awo/def` | ✅ v1.0 |
| `def.EntityRecord` struct + Get/Set/GetString/GetInt/GetDecimal/GetUUID | `awo/def` | ✅ v1.0 |
| `def.Actor` struct (after removing IsPlatformAdmin, adding ServiceAccountID) | `awo/def` | ✅ v1.0 |
| `def.PermissionSet` struct | `awo/def` | ✅ v1.0 |
| `def.WorkflowTrigger` struct | `awo/def` | ✅ v1.0 |
| `def.PageBuilderSet`, `def.PageBuilder`, `def.PageContext` | `awo/def` | ✅ v1.0 |
| `def.Register()`, `def.All()`, `def.Seal()`, `def.Lookup()` | `awo/def` | ✅ v1.0 |
| `def.ActionRuntime` interface (all 11 methods) | `awo/def` | ✅ v1.0 |
| `def.ActionEntityRepo` interface | `awo/def` | ✅ v1.0 |
| `filter.*` package (all constructors and Filter struct) | `awo/filter` | ✅ v1.0 |
| `cache.Cache` interface | `awo/cache` | ✅ v1.0 |
| `cache.Counter` interface | `awo/cache` | ✅ v1.0 |
| `naming.NamingSeriesService` public methods (Allocate, Preview, Validate) | `awo/naming` | ✅ v1.0 |
| `registry.Registry` public methods | `awo/registry` | ✅ v1.0 |
| `auth.ViewerContext` interface | `awo/auth` | ✅ v1.0 |
| `auth.PolicyEvaluator` interface | `awo/auth` | ✅ v1.0 |
| `auth.Session` struct | `awo/auth` | ✅ v1.0 |
| `auth.WithViewer()` / `auth.ViewerFromContext()` | `awo/auth` | ✅ v1.0 |
| `widget.Node` struct and `widget.NodeKind` constants | `awo/sdui/widget` | ✅ v1.0 |
| `outbox.EventBroker` interface | `awo/outbox` | ✅ v1.0 |
| `audit.AuditRecord` struct + `audit.Writer` interface | `awo/audit` | ✅ v1.0 |

### Intentionally Private (Internal Framework Use Only)

| Contract | Why Private |
|----------|-------------|
| `compiler.CompiledSchema` | Framework implementation detail; module authors never reference it |
| `compiler.EntitySchema` | Implementation detail; sdui and runtime use it internally |
| `runtime.DefaultActionRuntime` | Implementation; only the `ActionRuntime` interface is public |
| `runtime.RuntimeFactory` | Wiring concern; not part of the module author API |
| `registry.validateDefinition` | Internal validation logic |
| `sdui.Generator` | Internal; modules use PageBuilderSet, not the generator directly |
| All `*_impl.go` files | Infrastructure implementations |

### Freeze Later (v1.1)

| Contract | Reason for Deferral |
|----------|---------------------|
| Organization hierarchy types | Design deferred to v1.1 |
| `compiler.MigrationSpec` | Migration generation is a v1.1 feature |
| ReBAC grant graph types | Requires v1.1 auth engine upgrade |

---

## Phase 4 — Package Boundaries

### Final Package Layout

```
awo/
├── def/              ONE RESPONSIBILITY: Framework vocabulary
│   │                 EntityDefinition, FieldDef, EdgeDef, HookSet, PermissionSet,
│   │                 ActionDef, WorkflowTrigger, PageBuilderSet, EntityRecord,
│   │                 Actor, ActionContext, ActionResult, ActionRuntime,
│   │                 ActionEntityRepo, ActionEvent, ActionWorkflowSpec,
│   │                 ActionNotification, ActionCache, FieldType/EdgeType/EventType constants
│   │                 Register/All/Seal/Lookup (global registry)
│   └── (no sub-packages)
│
├── filter/           ONE RESPONSIBILITY: Composable predicate DSL
│   │                 Filter struct, Eq/Neq/Gt/Gte/Lt/Lte/In/NotIn/IsNull/
│   │                 Contains/StartsWith/And/Or/Not/Between constructors
│   └── (no sub-packages)
│
├── auth/             ONE RESPONSIBILITY: Authorization and identity contracts
│   │                 ViewerContext interface, PolicyEvaluator interface,
│   │                 Session struct, Grant struct, WithViewer/ViewerFromContext
│   │                 DefaultViewer (concrete implementation)
│   └── (no sub-packages)
│
├── registry/         ONE RESPONSIBILITY: Definition validation and sealing
│   │                 Registry struct, Build/BuildFrom, validateDefinition
│   └── (no sub-packages)
│
├── compiler/         ONE RESPONSIBILITY: Compile EntityDefinitions → runtime descriptors
│   │                 CompiledSchema, EntitySchema, RouteDescriptor, CompiledLookup,
│   │                 CapabilityGrant (renamed from CasbinPolicy), Compile()
│   └── (no sub-packages)
│
├── runtime/          ONE RESPONSIBILITY: Execute the hook pipeline + wire framework services
│   │                 Pipeline, PipelineConfig, RuntimeFactory, DefaultActionRuntime,
│   │                 EntityDriver interface, WorkflowRuntime interface,
│   │                 NotificationService interface, CacheInvalidator interface,
│   │                 EventBus interface (thin facade over outbox)
│   └── (no sub-packages)
│
├── naming/           ONE RESPONSIBILITY: Naming series allocation
│   │                 NamingSeriesService, AllocateInput, NamingFieldContext,
│   │                 ParsePattern, Render, CounterKey, PreviewPattern
│   └── (no sub-packages)
│
├── cache/            ONE RESPONSIBILITY: Cache and counter contracts
│   │                 Cache interface, Counter interface, NoopCache, NoopCounter
│   └── (no sub-packages)
│
├── outbox/           ONE RESPONSIBILITY: Durable delivery contracts
│   │                 EventBroker interface, WorkflowDispatcher interface
│   │                 EventOutboxRecord struct, WorkflowOutboxRecord struct
│   └── (no sub-packages)
│
├── audit/            ONE RESPONSIBILITY: Audit record contract
│   │                 AuditRecord struct, Writer interface, NoopWriter
│   └── (no sub-packages)
│
└── sdui/             ONE RESPONSIBILITY: SDUI schema generation
    ├── widget/       ONE RESPONSIBILITY: WidgetTree IR
    │   │             Node struct, NodeKind constants, DataSource, ActionNode
    │   └── (no sub-packages)
    ├── generator.go  Produces *widget.Node trees from EntitySchema
    └── amis/         ONE RESPONSIBILITY: Render WidgetTree to amis JSON
                      Renderer struct, Render(*widget.Node) map[string]any
```

### Violations Identified and Resolved

| Violation | Resolution |
|-----------|------------|
| `awo/def` contained authorization concepts (PolicyFunc) but no PolicyEvaluator | PolicyEvaluator moved to `awo/auth`; PolicyFunc remains in `awo/def` as it is part of PermissionSet declaration |
| Authorization context (ViewerContext) had no package | Added to `awo/auth` |
| Session had no framework-level type | Added `auth.Session` |
| Workflow outbox had no package | Added `awo/outbox` |
| Audit had no framework-level contract | Added `awo/audit` |
| SDUI emitted amis JSON directly | Split into `awo/sdui/widget` (IR) and `awo/sdui/amis` (renderer) |
| `compiler.CasbinPolicies` coupled compiler to Casbin | Renamed to `compiler.CapabilityGrants` |
| No explicit `EventBroker` interface | Added to `awo/outbox` |

### Circular Dependency Analysis

The following dependency order is enforced. No cycles are permitted:

```
def       → (nothing)
filter    → (nothing)
cache     → (nothing)
auth      → def
outbox    → (nothing)
audit     → def
naming    → cache, def
registry  → def
compiler  → def, registry
sdui/widget → (nothing)
sdui/amis   → sdui/widget
sdui        → compiler, sdui/widget, sdui/amis, def, cache
runtime     → def, auth, compiler, naming, cache, outbox, audit
```

This is a strict DAG. No circular imports.

---

## Phase 5 — Documentation Readiness

| Subsystem | Status | Blocking Decision |
|-----------|--------|-------------------|
| EntityDefinition | ✅ Ready | — |
| Field System | ✅ Ready | — |
| Registry | ✅ Ready | — |
| Filter DSL | ✅ Ready | — |
| NamingSeries | ✅ Ready | — |
| Cache | ✅ Ready | — |
| Pipeline / Hook lifecycle | ✅ Ready | Hook recursion policy (D10) now resolved |
| Multi-tenancy / RLS | ✅ Ready | — |
| Compiler | ✅ Ready | CapabilityGrant rename (D11) now resolved |
| Actions | ✅ Ready | — |
| Authorization | ⚠ Requires writing | PolicyEvaluator (D1), ViewerContext (D2) decisions now made; spec must be written |
| Identity / Actor | ⚠ Requires writing | Actor model (D3) now decided; final struct must be written |
| Authentication / Session | ⚠ Requires writing | Session model (D4) now decided; spec must be written |
| Repository | ⚠ Requires writing | ViewerContext contract (D2) now decided; repository spec needed |
| Audit | ⚠ Requires writing | Audit as pipeline stage (D5) now decided; pipeline spec update needed |
| Workflow | ⚠ Requires writing | Outbox (D7) now decided; outbox spec must be written |
| SDUI / WidgetTree | ⚠ Requires writing | WidgetTree IR (D6) now decided; Node type catalog must be written |
| ViewerContext | ⚠ Requires writing | Decision made; interface spec must be written |
| Hooks | ✅ Ready | — |
| Runtime wiring | ⚠ Requires writing | RuntimeFactory DI spec needed |
| Organization hierarchy | ❌ Not ready | Deferred to v1.1 |

---

## Phase 6 — Stability Prediction

| Area | Probability of Change | Reason |
|------|-----------------------|--------|
| EntityDefinition interface | Very Low | Proven by Finance module; external implementors freeze this |
| FieldDef / FieldType | Very Low | All types exercised; addition-only from here |
| Filter DSL | Very Low | Composable; new predicates additive |
| NamingSeries | Very Low | Atomic counter pattern is complete |
| RLS / Multi-tenancy | Very Low | PostgreSQL FORCE RLS is the industry standard for this pattern |
| Cache interface | Very Low | Minimal interface; implementation detail hidden |
| Hook pipeline stages | Low | Order is correct; adding a stage between existing stages is a v2 concern |
| PermissionSet declaration | Low | Simple; well-understood |
| Actor struct (post-D3) | Low | Field additions are backward compatible |
| Session struct | Low | Standard pattern |
| PolicyEvaluator interface | Medium | First implementation will reveal edge cases; the interface may need 1-2 method additions |
| ViewerContext interface | Medium | Minimal interface at v1.0; organization hierarchy (v1.1) may require additions |
| WidgetTree Node types | Medium | New NodeKinds will be discovered during real UI development |
| RuntimeFactory wiring | High | DI complexity; concrete infrastructure adapters will reveal gaps |
| Workflow outbox | High | Temporal integration details will drive changes to outbox schema |
| Audit pipeline stage | Medium | AuditRecord field completeness will be discovered during implementation |
| SDUI amis renderer | High | amis SDK has many quirks; the renderer will accumulate edge-case logic |
| Organization hierarchy (v1.1) | Critical | Not designed; will require additions to multiple packages |

---

## Phase 7 — Architecture Decision Register

### ADR-001: Authorization Declaration vs Enforcement Separation

| Field | Value |
|-------|-------|
| **Decision ID** | ADR-001 |
| **Title** | PermissionSet declares; PolicyEvaluator enforces |
| **Status** | DECIDED |
| **Decision** | `PermissionSet` remains in `awo/def` as the declaration mechanism. `PolicyEvaluator` interface is defined in `awo/auth`. The compiler produces `CapabilityGrant` slices from `PermissionSet` declarations. The runtime loads `CapabilityGrant` slices into the `PolicyEvaluator` implementation. |
| **Alternatives Considered** | (A) Replace PermissionSet with a runtime capability graph — rejected: over-engineered for module authors. (B) Embed PolicyEvaluator in EntityDefinition — rejected: authorization is a framework concern, not a module concern. |
| **Consequences** | PolicyEvaluator is replaceable. The default Casbin implementation is one concrete class. Module authors never interact with PolicyEvaluator. |
| **Affected Packages** | `awo/def`, `awo/auth`, `awo/compiler`, `awo/runtime` |
| **Breaking Change Risk** | None — adds new package, renames internal type |
| **Implementation Priority** | P0 — required before any runtime code |
| **Documentation Priority** | P0 |

---

### ADR-002: ViewerContext — Typed Context Key

| Field | Value |
|-------|-------|
| **Decision ID** | ADR-002 |
| **Title** | ViewerContext embedded in context.Context via typed key |
| **Status** | DECIDED |
| **Decision** | `auth.ViewerContext` is an interface. The middleware injects it into every request context via `auth.WithViewer()`. Repository implementations extract it via `auth.ViewerFromContext()`. Absence of ViewerContext panics immediately. |
| **Alternatives Considered** | Explicit parameter on Repository methods — rejected: doubles method signatures, makes mocking complex. |
| **Consequences** | Repository interface stays clean. Authorization is guaranteed present or the process panics at development time. |
| **Affected Packages** | `awo/auth`, `awo/runtime`, `awo/def` (ActionEntityRepo uses it implicitly) |
| **Breaking Change Risk** | None for v1.0 — new package |
| **Implementation Priority** | P0 |
| **Documentation Priority** | P0 |

---

### ADR-003: Actor Model — No IsPlatformAdmin, Add ServiceAccountID

| Field | Value |
|-------|-------|
| **Decision ID** | ADR-003 |
| **Title** | Actor.IsPlatformAdmin removed; expressed as role |
| **Status** | DECIDED |
| **Decision** | Remove `IsPlatformAdmin bool`. Add `ServiceAccountID uuid.UUID`. Add `IsPlatformAdmin()` method checking for "role:platform-admin" in Roles. |
| **Alternatives Considered** | Keep IsPlatformAdmin bool — rejected: privilege as boolean field is an architectural violation that creates two authorization code paths. |
| **Consequences** | All privilege checks are uniform. Service accounts are first-class. |
| **Affected Packages** | `awo/def` |
| **Breaking Change Risk** | Breaking — field removed. Must change before any external code is written. |
| **Implementation Priority** | P0 |
| **Documentation Priority** | P0 |

---

### ADR-004: Session Model Frozen

| Field | Value |
|-------|-------|
| **Decision ID** | ADR-004 |
| **Title** | auth.Session struct is the canonical session representation |
| **Status** | DECIDED |
| **Decision** | `auth.Session` struct as specified in Phase 2, Decision 4. Stored in Redis as JSON. TTL = ExpiresAt - now. `ToActor()` produces `*def.Actor`. |
| **Alternatives Considered** | JWT-only (no Redis) — rejected: session revocation requires a store. |
| **Consequences** | Session revocation is O(1) Redis delete. Framework controls session shape. |
| **Affected Packages** | `awo/auth` |
| **Breaking Change Risk** | None — new package |
| **Implementation Priority** | P1 |
| **Documentation Priority** | P1 |

---

### ADR-005: Audit as Mandatory Pipeline Stage

| Field | Value |
|-------|-------|
| **Decision ID** | ADR-005 |
| **Title** | Audit record written as pipeline stage, not opt-in hook |
| **Status** | DECIDED |
| **Decision** | Audit writes between PERSIST and after_save within the transaction. Suppressed only when `EntityDefinition.AuditEnabled = false`. `awo/audit` defines `AuditRecord` and `Writer` interface. |
| **Alternatives Considered** | Opt-in hook — rejected: silent compliance gaps in ERP financial context. |
| **Consequences** | All mutations on audit-enabled entities produce audit records. Audit log table requires partitioning strategy. |
| **Affected Packages** | `awo/audit`, `awo/runtime`, `awo/def` |
| **Breaking Change Risk** | Additive — AuditEnabled field added to definition structs |
| **Implementation Priority** | P1 |
| **Documentation Priority** | P1 |

---

### ADR-006: WidgetTree IR Required Before v1.0

| Field | Value |
|-------|-------|
| **Decision ID** | ADR-006 |
| **Title** | SDUI generator produces WidgetTree; amis renderer converts to JSON |
| **Status** | DECIDED |
| **Decision** | `awo/sdui/widget` defines `Node` and `NodeKind`. The generator produces `*widget.Node`. `awo/sdui/amis` converts Node trees to `map[string]any`. |
| **Alternatives Considered** | Direct amis JSON emission — rejected: creates permanent amis coupling with no escape path. |
| **Consequences** | Replacing amis requires writing one new renderer. All generators remain stable. |
| **Affected Packages** | `awo/sdui`, `awo/sdui/widget`, `awo/sdui/amis` |
| **Breaking Change Risk** | Requires refactoring existing sdui/generator.go before v1.0 |
| **Implementation Priority** | P1 |
| **Documentation Priority** | P1 |

---

### ADR-007: Workflow Outbox Required

| Field | Value |
|-------|-------|
| **Decision ID** | ADR-007 |
| **Title** | Workflow starts are durable via workflow_outbox table |
| **Status** | DECIDED |
| **Decision** | `workflow_outbox` table as specified. `StartWorkflow()` writes to outbox within entity TX. Outbox worker dispatches to Temporal with exponential backoff. |
| **Alternatives Considered** | Direct Temporal call with log-on-failure — rejected: silent data consistency violation for financial workflows. |
| **Consequences** | Workflow starts are guaranteed-at-least-once. Temporal provides deduplication via workflow IDs. |
| **Affected Packages** | `awo/outbox`, `awo/runtime`, platform migrations |
| **Breaking Change Risk** | None for v1.0 — new infrastructure |
| **Implementation Priority** | P0 |
| **Documentation Priority** | P1 |

---

### ADR-008: Event Outbox Schema Is the Contract

| Field | Value |
|-------|-------|
| **Decision ID** | ADR-008 |
| **Title** | event_outbox table schema is frozen; broker is pluggable |
| **Status** | DECIDED |
| **Decision** | `event_outbox` table as specified. `EventBroker` interface in `awo/outbox`. Default implementation: no-op (logs). Production: Kafka or NATS adapter. |
| **Alternatives Considered** | Direct broker calls — rejected: loss of delivery guarantee on broker unavailability. |
| **Consequences** | Events are at-least-once delivered. Broker is replaceable without schema changes. |
| **Affected Packages** | `awo/outbox`, `awo/runtime`, platform migrations |
| **Breaking Change Risk** | None for v1.0 |
| **Implementation Priority** | P1 |
| **Documentation Priority** | P1 |

---

### ADR-009: Idempotency via X-Idempotency-Key

| Field | Value |
|-------|-------|
| **Decision ID** | ADR-009 |
| **Title** | HTTP middleware implements idempotency for POST/PATCH |
| **Status** | DECIDED |
| **Decision** | Middleware intercepts X-Idempotency-Key header. On duplicate within 24 hours, returns stored response. Redis key: `idempotency:{tenant_id}:{key}`. TTL: 24 hours. |
| **Alternatives Considered** | Per-endpoint implementation — rejected: every handler would re-implement this. |
| **Consequences** | Temporal workflow activities can safely retry API calls. No module author work required. |
| **Affected Packages** | HTTP middleware (not in awo/ — in cmd/server) |
| **Breaking Change Risk** | None |
| **Implementation Priority** | P2 |
| **Documentation Priority** | P2 |

---

### ADR-010: Hook Recursion Detected at Runtime, Fatal

| Field | Value |
|-------|-------|
| **Decision ID** | ADR-010 |
| **Title** | Same-entity recursive hook calls cause immediate panic |
| **Status** | DECIDED |
| **Decision** | The runtime tracks a per-context entity-name stack. Same-entity recursion panics with a diagnostic message. Cross-entity operations are permitted. |
| **Alternatives Considered** | Silent infinite loop protection via call counter — rejected: masks bugs; correct code never recurses. |
| **Consequences** | Recursive bugs surface immediately in development. Zero performance cost for non-recursive paths (one context.Value lookup per pipeline stage). |
| **Affected Packages** | `awo/runtime` |
| **Breaking Change Risk** | None |
| **Implementation Priority** | P1 |
| **Documentation Priority** | P1 |

---

### ADR-011: Compiler Output Decoupled from Casbin

| Field | Value |
|-------|-------|
| **Decision ID** | ADR-011 |
| **Title** | CasbinPolicy renamed to CapabilityGrant in compiler output |
| **Status** | DECIDED |
| **Decision** | `compiler.CapabilityGrant{Subject, Object, Action}`. The Casbin adapter reads CapabilityGrant slices; no Casbin imports in the compiler. |
| **Alternatives Considered** | Keep CasbinPolicy name — rejected: couples compiler output to an implementation detail. |
| **Consequences** | Authorization engine is replaceable without touching the compiler. |
| **Affected Packages** | `awo/compiler`, `awo/auth` (Casbin adapter) |
| **Breaking Change Risk** | Internal rename; no external callers yet |
| **Implementation Priority** | P1 |
| **Documentation Priority** | P1 |

---

### ADR-012: Organization Hierarchy Deferred to v1.1

| Field | Value |
|-------|-------|
| **Decision ID** | ADR-012 |
| **Title** | Multi-org/multi-branch within tenant is a v1.1 feature |
| **Status** | DECIDED |
| **Decision** | No organization hierarchy types in v1.0. Branch scoping via custom entities + PolicyFunc. The v1.1 design will build on ViewerContext additions. |
| **Alternatives Considered** | Design now — rejected: insufficient requirements; premature design would force breaking changes later. |
| **Consequences** | v1.0 is clean. v1.1 adds organization types and may require a non-breaking ViewerContext addition. |
| **Affected Packages** | None in v1.0 |
| **Breaking Change Risk** | v1.1 will extend ViewerContext interface (additive, non-breaking for implementations if using embedding) |
| **Implementation Priority** | N/A v1.0 |
| **Documentation Priority** | P3 — note in architecture docs as deferred |

---

## Phase 8 — Implementation Preconditions

The following must exist before a single line of framework implementation code is written.

### Tier 0 — Must Exist Before Day 1

| Document | Purpose | Owner |
|----------|---------|-------|
| `awo/docs/ARCH_OVERVIEW.md` | Framework principles, layer model, dependency DAG | Architecture |
| `awo/docs/auth/AUTHORIZATION_SPEC.md` | ViewerContext, PolicyEvaluator, CapabilityGrant complete specification | Architecture |
| `awo/docs/auth/ACTOR_MODEL.md` | Actor, Session, ViewerContext lifecycle | Architecture |
| `awo/docs/pipeline/LIFECYCLE_SPEC.md` | Complete pipeline stages, TX boundaries, hook order | Architecture |
| `awo/docs/pipeline/HOOK_CONTRACT.md` | Hook interface contracts, recursion policy, TX semantics | Architecture |
| `awo/docs/entity/ENTITY_DEFINITION_SPEC.md` | EntityDefinition interface, SystemDefinition, CustomDefinition, field validation rules | Architecture |
| `awo/docs/errors/ERROR_MODEL.md` | Error types, HTTP mapping, chain protocol, never-expose-internals rule | Architecture |

### Tier 1 — Must Exist Before Subsystem Implementation

| Document | Purpose | Needed Before |
|----------|---------|---------------|
| `awo/docs/compiler/COMPILE_SPEC.md` | Phase-by-phase compilation, output types, validation rules | `awo/compiler` implementation |
| `awo/docs/sdui/WIDGET_TREE_SPEC.md` | Full Node catalog, DataSource contract, ActionNode | `awo/sdui/widget` and `awo/sdui/amis` |
| `awo/docs/sdui/AMIS_RENDERER_SPEC.md` | NodeKind → amis type mapping, edge cases | `awo/sdui/amis` |
| `awo/docs/naming/NAMING_SERIES_SPEC.md` | Pattern token reference, counter key format, reset period | `awo/naming` (exists; verify complete) |
| `awo/docs/workflow/OUTBOX_SPEC.md` | workflow_outbox schema, worker retry policy, Temporal dispatch | `awo/outbox`, `awo/runtime` |
| `awo/docs/events/OUTBOX_SPEC.md` | event_outbox schema, EventBroker interface, delivery semantics | `awo/outbox`, `awo/runtime` |
| `awo/docs/audit/AUDIT_SPEC.md` | AuditRecord schema, pipeline stage position, AuditEnabled semantics | `awo/audit`, `awo/runtime` |
| `awo/docs/multitenancy/RLS_SPEC.md` | set_tenant_context contract, FORCE RLS policy, PgBouncer requirement | Infrastructure |
| `awo/docs/cache/CACHE_SPEC.md` | Cache interface, Counter interface, key naming conventions, TTL policy | Infrastructure |

### Tier 2 — Must Exist Before Integration

| Document | Purpose |
|----------|---------|
| `awo/docs/api/API_CONVENTIONS.md` | URL patterns, response envelope, error format, HTTP status mapping |
| `awo/docs/api/IDEMPOTENCY_SPEC.md` | X-Idempotency-Key protocol, Redis storage, TTL, replay semantics |
| `awo/docs/testing/TEST_STRATEGY.md` | Unit test patterns, integration test requirements, real PostgreSQL mandate |
| `awo/docs/migrations/MIGRATION_GUIDE.md` | File naming, zero-downtime patterns, RLS table requirements |

### State Machines Required

Before implementation, the following state machines must be formally specified (not narrative; actual state/transition tables):

1. Tenant lifecycle: `PENDING → ACTIVE → SUSPENDED → ARCHIVED`
2. Session lifecycle: `ACTIVE → EXPIRED → REVOKED`
3. WorkflowOutbox entry: `PENDING → DISPATCHED | FAILED`
4. EventOutbox entry: `PENDING → DELIVERED | FAILED`
5. EntityRecord pipeline: all stages with error transitions

### Sequence Diagrams Required

1. Complete HTTP request pipeline (tenant resolution → session validation → permission check → pipeline → response)
2. Action handler execution (permission check → ActionRuntime construction → handler → outbox writes → response)
3. Workflow start via outbox (entity TX → outbox write → commit → worker dispatch → Temporal)
4. Event publish via outbox (entity TX → outbox write → commit → broker delivery)
5. Cache invalidation flow (mutation → InvalidateCache → Redis prefix delete)

---

## Phase 9 — Architecture Freeze Checklist

### Core Type System

| Item | Status |
|------|--------|
| `def.EntityDefinition` interface final and frozen | ✅ Frozen |
| `def.SystemDefinition` / `def.CustomDefinition` fields final | ✅ Frozen |
| `def.FieldDef` fields final | ✅ Frozen |
| `def.EdgeDef` fields final | ✅ Frozen |
| `def.FieldType` constants complete | ✅ Frozen |
| `def.ActionRuntime` interface final | ✅ Frozen |
| `def.ActionEntityRepo` interface final | ✅ Frozen |
| `def.EntityRecord` typed accessors complete | ✅ Frozen |
| `def.Actor` struct updated (IsPlatformAdmin removed, ServiceAccountID added) | ⚠ Resolve before implementation |
| `def.HookSet` and all hook interfaces final | ✅ Frozen |

### Authorization Layer

| Item | Status |
|------|--------|
| `auth.ViewerContext` interface written | ⚠ Resolve before implementation |
| `auth.PolicyEvaluator` interface written | ⚠ Resolve before implementation |
| `auth.Session` struct written | ⚠ Resolve before implementation |
| `auth.CapabilityGrant` (renamed from CasbinPolicy) in compiler | ⚠ Resolve before implementation |
| Default Casbin adapter implementing PolicyEvaluator | Infrastructure (P1) |
| ViewerContext middleware injection specified | ⚠ Resolve before implementation |

### Infrastructure Contracts

| Item | Status |
|------|--------|
| `workflow_outbox` table schema defined | ⚠ Resolve before implementation |
| `event_outbox` table schema defined | ⚠ Resolve before implementation |
| `outbox.EventBroker` interface written | ⚠ Resolve before implementation |
| `audit.AuditRecord` struct written | ⚠ Resolve before implementation |
| `audit.Writer` interface written | ⚠ Resolve before implementation |
| Idempotency middleware specified | ⚠ Resolve before implementation |

### SDUI

| Item | Status |
|------|--------|
| `sdui/widget.Node` struct written | ⚠ Resolve before implementation |
| `sdui/widget.NodeKind` constants complete | ⚠ Resolve before implementation |
| `sdui/amis` renderer specification written | ⚠ Resolve before implementation |
| Existing `sdui/generator.go` refactored to produce WidgetTree | ⚠ Resolve before implementation |

### Package Structure

| Item | Status |
|------|--------|
| `awo/auth` package created | ⚠ Resolve before implementation |
| `awo/outbox` package created | ⚠ Resolve before implementation |
| `awo/audit` package created | ⚠ Resolve before implementation |
| `awo/sdui/widget` package created | ⚠ Resolve before implementation |
| `awo/sdui/amis` package created | ⚠ Resolve before implementation |
| No circular imports verified | ⚠ Resolve before implementation |

### Documentation

| Item | Status |
|------|--------|
| Tier 0 docs exist | ⚠ Resolve before implementation |
| State machines specified | ⚠ Resolve before implementation |
| Sequence diagrams specified | ⚠ Resolve before implementation |
| Error model documented | ⚠ Resolve before implementation |

### Deferrals Confirmed

| Item | Status |
|------|--------|
| Organization hierarchy explicitly deferred to v1.1 | ✅ Decided |
| `EntityDriver` pgx implementation in implementation backlog | ✅ Confirmed |
| Redis Counter implementation in implementation backlog | ✅ Confirmed |
| Integration test suite against real PostgreSQL in CI backlog | ✅ Confirmed |

---

## Phase 10 — Final Verdict

## Architecture Freeze Decision

### APPROVED WITH REQUIRED DECISIONS

The Awo Framework core architecture is sound. The EntityDefinition contract, Filter DSL, hook pipeline, NamingSeries, Registry, and multi-tenancy model are ready to freeze immediately.

**Twelve decisions were identified and resolved in this review.** They were previously open; they are now closed. They do not require further discussion.

**Before any implementation code is written**, the following artifacts must be produced by the architecture team (not the implementation team):

1. **`awo/auth` package** — write `ViewerContext`, `PolicyEvaluator`, `Session`, `Grant` as Go interface and struct definitions. No implementation. Interfaces only.

2. **`def.Actor` struct** — remove `IsPlatformAdmin bool`, add `ServiceAccountID uuid.UUID`, add `IsPlatformAdmin()` method.

3. **`awo/sdui/widget` package** — write `Node` struct and complete `NodeKind` constant set.

4. **`awo/outbox` package** — write `EventBroker` and `WorkflowDispatcher` interfaces; define outbox record structs.

5. **`awo/audit` package** — write `AuditRecord` struct and `Writer` interface.

6. **Rename `CasbinPolicies` → `CapabilityGrants`** in `awo/compiler/schema.go`.

7. **Write Tier 0 documentation** — seven documents listed in Phase 8 Tier 0.

These seven tasks are the minimum gate. After they are complete, implementation may begin in dependency order.

---

# Documentation Roadmap

## Directory Structure

```
awo/docs/
├── 00-overview/
│   ├── ARCH_OVERVIEW.md              (read first by everyone)
│   ├── PRINCIPLES.md                 (design principles and benchmarks)
│   ├── PACKAGE_DEPENDENCY_MAP.md     (DAG with no cycles)
│   └── DECISION_REGISTER.md          (ADR-001 through ADR-012, this document)
│
├── 01-entity/
│   ├── ENTITY_DEFINITION_SPEC.md     (EntityDefinition interface; the canonical reference)
│   ├── FIELD_TYPES_REFERENCE.md      (all FieldType constants with PG column, Go type, SDUI widget)
│   ├── EDGE_TYPES_REFERENCE.md       (EdgeType semantics, FK generation, cascade rules)
│   ├── NAMING_CONVENTIONS.md         (entity naming, field naming, action naming)
│   └── ENTITY_EXAMPLES.md            (Finance module as canonical worked example)
│
├── 02-pipeline/
│   ├── LIFECYCLE_SPEC.md             (ASSEMBLE → ... → Workflow, TX boundaries)
│   ├── HOOK_CONTRACT.md              (all hook interfaces, execution order, recursion policy)
│   ├── ERROR_MODEL.md                (ValidationError, BusinessError, NotFoundError, HTTP mapping)
│   └── PIPELINE_SEQUENCE.md          (sequence diagram: request → response)
│
├── 03-auth/
│   ├── AUTHORIZATION_SPEC.md         (PermissionSet, PolicyEvaluator, CapabilityGrant)
│   ├── ACTOR_MODEL.md                (Actor struct, ServiceAccount, roles, IsPlatformAdmin)
│   ├── SESSION_MODEL.md              (Session struct, Redis storage, revocation)
│   ├── VIEWER_CONTEXT.md             (ViewerContext interface, WithViewer, ViewerFromContext)
│   ├── RBAC_ROLES_REFERENCE.md       (built-in roles, role naming conventions)
│   └── CASBIN_ADAPTER.md             (default PolicyEvaluator implementation — implementation doc)
│
├── 04-multitenancy/
│   ├── RLS_SPEC.md                   (set_tenant_context, FORCE RLS, PgBouncer requirement)
│   ├── TENANT_LIFECYCLE.md           (state machine: PENDING→ACTIVE→SUSPENDED→ARCHIVED)
│   ├── TENANT_IDENTIFICATION.md      (header priority, subdomain parsing)
│   └── GLOBAL_TABLES.md             (tables exempt from RLS; rationale)
│
├── 05-compiler/
│   ├── COMPILE_SPEC.md               (phase-by-phase: Phase 1 routes, Phase 2 lookups, Phase 2.5 links)
│   ├── COMPILED_SCHEMA_REFERENCE.md  (CompiledSchema, EntitySchema, RouteDescriptor, CapabilityGrant)
│   └── VALIDATION_RULES.md           (all validation rules applied by Registry.Build)
│
├── 06-filter/
│   ├── FILTER_DSL_REFERENCE.md       (all constructors, examples, composition patterns)
│   └── FILTER_POLICY_PATTERNS.md     (PolicyFunc patterns: OwnerOnly, BranchScoped, TenantIsolation)
│
├── 07-naming/
│   ├── NAMING_SERIES_SPEC.md         (pattern syntax, all tokens, counter key format, reset periods)
│   └── NAMING_SERIES_EXAMPLES.md     (INV-{YYYY}-{SEQ:6}, fiscal year examples)
│
├── 08-workflow/
│   ├── TEMPORAL_INTEGRATION.md       (WorkflowTrigger, determinism rules, activity patterns)
│   ├── OUTBOX_SPEC.md                (workflow_outbox schema, worker retry policy, sequence diagram)
│   ├── SAGA_PATTERN.md               (compensating transactions, SagaCompensator, examples)
│   └── WORKFLOW_ID_CONVENTION.md     ({tenant}.{entity}.{record}.{event})
│
├── 09-events/
│   ├── EVENT_OUTBOX_SPEC.md          (event_outbox schema, EventBroker interface, delivery semantics)
│   └── DOMAIN_EVENTS_REFERENCE.md    (topic naming conventions, payload contracts)
│
├── 10-sdui/
│   ├── WIDGET_TREE_SPEC.md           (Node struct, all NodeKind values, DataSource, ActionNode)
│   ├── AMIS_RENDERER_SPEC.md         (NodeKind → amis type mapping, edge cases, caching)
│   ├── PAGE_BUILDER_GUIDE.md         (custom PageBuilder patterns, permission-gated elements)
│   └── SDUI_FIELD_WIDGET_MAP.md      (FieldType → NodeKind → amis type reference table)
│
├── 11-cache/
│   ├── CACHE_SPEC.md                 (Cache interface, Counter interface, key naming, TTL policy)
│   └── CACHE_KEY_REFERENCE.md        (all key patterns: session:, page:, eval:, rl:)
│
├── 12-audit/
│   ├── AUDIT_SPEC.md                 (AuditRecord schema, pipeline stage, AuditEnabled semantics)
│   └── AUDIT_QUERY_PATTERNS.md       (how to query the audit log; what is never logged)
│
├── 13-actions/
│   ├── ACTION_HANDLER_GUIDE.md       (ActionContext, ActionRuntime API, pattern library)
│   ├── ACTION_RUNTIME_REFERENCE.md   (all ActionRuntime methods; contracts and guarantees)
│   └── CUSTOM_ACTIONS_EXAMPLES.md    (Finance actions as canonical worked examples)
│
├── 14-api/
│   ├── API_CONVENTIONS.md            (URL structure, response envelope, HTTP status mapping)
│   ├── IDEMPOTENCY_SPEC.md           (X-Idempotency-Key protocol, Redis storage)
│   ├── PAGINATION_SPEC.md            (cursor vs offset, PageInfo struct)
│   └── ERROR_RESPONSE_FORMAT.md      (client-facing error envelope, field-level errors)
│
├── 15-migrations/
│   ├── MIGRATION_GUIDE.md            (file naming, up/down pairs, zero-downtime patterns)
│   ├── RLS_TABLE_TEMPLATE.md         (SQL template for every new tenant-scoped table)
│   └── MIGRATION_CHECKLIST.md        (mandatory items: RLS, tenant_id FK, indexes)
│
├── 16-testing/
│   ├── TEST_STRATEGY.md              (unit vs integration, real PostgreSQL mandate, table-driven)
│   ├── HOOK_TEST_PATTERNS.md         (mock EntityRepository, BeforeCreate test examples)
│   ├── ACTION_TEST_PATTERNS.md       (ActionRuntime mock, test helpers)
│   └── REGISTRY_TEST_PATTERNS.md     (BuildFrom for isolated test registries)
│
├── 17-observability/
│   ├── LOGGING_SPEC.md               (slog fields, mandatory context keys, sensitive exclusions)
│   ├── METRICS_SPEC.md               (Prometheus metrics, histogram names, label conventions)
│   └── HEALTH_CHECKS.md             (/health/live and /health/ready contracts)
│
├── 18-security/
│   ├── SECURITY_MODEL.md             (RLS, RBAC, session, rate limiting, threat model)
│   ├── SENSITIVE_FIELDS.md           (Sensitive: true semantics, log exclusion, response masking)
│   └── SECRET_MANAGEMENT.md          (env vars, Vault integration, never-in-logs rules)
│
└── 99-modules/
    ├── FINANCE_MODULE_SPEC.md        (24 entities, 3 migration files, action contracts)
    └── MODULE_AUTHOR_GUIDE.md        (how to build a module; EntityDefinition patterns)
```

## Document Dependency Order

Documents must be written in this order. No document should be written before its dependencies:

```
ARCH_OVERVIEW → PRINCIPLES → PACKAGE_DEPENDENCY_MAP → DECISION_REGISTER

ENTITY_DEFINITION_SPEC → FIELD_TYPES_REFERENCE
ENTITY_DEFINITION_SPEC → EDGE_TYPES_REFERENCE
ENTITY_DEFINITION_SPEC → NAMING_CONVENTIONS

ERROR_MODEL → LIFECYCLE_SPEC → HOOK_CONTRACT → PIPELINE_SEQUENCE

ACTOR_MODEL → SESSION_MODEL → VIEWER_CONTEXT → AUTHORIZATION_SPEC

RLS_SPEC → TENANT_LIFECYCLE → TENANT_IDENTIFICATION

ENTITY_DEFINITION_SPEC → COMPILE_SPEC → COMPILED_SCHEMA_REFERENCE → VALIDATION_RULES

FILTER_DSL_REFERENCE → FILTER_POLICY_PATTERNS

LIFECYCLE_SPEC → OUTBOX_SPEC (workflow)
AUTHORIZATION_SPEC → CASBIN_ADAPTER

WIDGET_TREE_SPEC → AMIS_RENDERER_SPEC → SDUI_FIELD_WIDGET_MAP → PAGE_BUILDER_GUIDE

LIFECYCLE_SPEC → AUDIT_SPEC

ACTION_RUNTIME_REFERENCE → ACTION_HANDLER_GUIDE → CUSTOM_ACTIONS_EXAMPLES

API_CONVENTIONS → IDEMPOTENCY_SPEC → PAGINATION_SPEC → ERROR_RESPONSE_FORMAT

MIGRATION_GUIDE → RLS_TABLE_TEMPLATE → MIGRATION_CHECKLIST

HOOK_CONTRACT → HOOK_TEST_PATTERNS
ACTION_HANDLER_GUIDE → ACTION_TEST_PATTERNS
```

## Reading Order (by Role)

**Framework implementor (builds awo/* packages):**
ARCH_OVERVIEW → PRINCIPLES → PACKAGE_DEPENDENCY_MAP → DECISION_REGISTER → ENTITY_DEFINITION_SPEC → LIFECYCLE_SPEC → HOOK_CONTRACT → AUTHORIZATION_SPEC → VIEWER_CONTEXT → COMPILE_SPEC → WIDGET_TREE_SPEC → OUTBOX_SPEC → AUDIT_SPEC

**Module author (builds internal/core/* modules):**
ARCH_OVERVIEW → ENTITY_DEFINITION_SPEC → FIELD_TYPES_REFERENCE → NAMING_CONVENTIONS → HOOK_CONTRACT → ERROR_MODEL → ACTION_HANDLER_GUIDE → FILTER_DSL_REFERENCE → NAMING_SERIES_SPEC → MIGRATION_GUIDE → FINANCE_MODULE_SPEC (as example)

**API consumer:**
API_CONVENTIONS → ERROR_RESPONSE_FORMAT → PAGINATION_SPEC → IDEMPOTENCY_SPEC

## Implementation Order

After documentation is complete, implement in this order:

```
1.  awo/def          — types only; no logic (mostly done)
2.  awo/filter       — DSL (done)
3.  awo/cache        — interfaces only (done)
4.  awo/auth         — ViewerContext, PolicyEvaluator, Session (new)
5.  awo/audit        — AuditRecord, Writer interface (new)
6.  awo/outbox       — EventBroker, WorkflowDispatcher interfaces (new)
7.  awo/naming       — NamingSeriesService (done)
8.  awo/registry     — Build, BuildFrom, validation (done)
9.  awo/compiler     — Compile, CompiledSchema (done; rename CasbinPolicies)
10. awo/sdui/widget  — Node, NodeKind (new)
11. awo/sdui/amis    — amis renderer (refactor existing)
12. awo/sdui         — Generator refactored to produce WidgetTree (refactor existing)
13. awo/runtime      — Pipeline, RuntimeFactory, DefaultActionRuntime (mostly done)
14. modules/finance  — canonical implementation (in progress)
15. Infrastructure   — pgx EntityDriver, Redis Counter, Casbin adapter, outbox worker
16. cmd/server       — wire everything; middleware pipeline; idempotency
```

---

*This document constitutes the formal Architecture Freeze Review for the Awo Framework v1.0. All twelve decisions listed in the Architecture Decision Register are final. No further architectural debate is required or appropriate. Implementation may proceed upon completion of the seven pre-implementation artifacts listed in Phase 10.*
