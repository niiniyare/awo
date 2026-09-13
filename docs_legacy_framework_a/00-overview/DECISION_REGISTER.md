> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

# Awo Framework — Decision Register

**Classification:** Constitutional — Tier 0
**Owner:** `00-overview/DECISION_REGISTER.md`
**Status:** Frozen at v1.0
**Source:** `awo/docs/ARCH_FREEZE_REVIEW.md` (full rationale and rejected alternatives)

---

## Purpose

This document is the canonical record of all architectural decisions (ADRs) for the Awo Framework. Every decision here is binding. No architectural decision may be made that contradicts these ADRs without an ARB review and a new numbered ADR appended to this document.

---

## ADR Index

| ADR | Decision | Status |
|-----|----------|--------|
| ADR-001 | Authorization: PermissionSet (declaration) + PolicyEvaluator (enforcement) | Frozen |
| ADR-002 | ViewerContext: typed context key, not explicit parameter | Frozen |
| ADR-003 | Actor: remove `IsPlatformAdmin bool`; add `ServiceAccountID uuid.UUID` | Frozen |
| ADR-004 | Session: frozen struct in `awo/auth` | Frozen |
| ADR-005 | Audit: mandatory pipeline stage, not optional hook | Frozen |
| ADR-006 | SDUI: WidgetTree IR required; no direct amis JSON emission | Frozen |
| ADR-007 | Workflow dispatch: durable `workflow_outbox` table | Frozen |
| ADR-008 | Event outbox: `event_outbox` schema is the public contract | Frozen |
| ADR-009 | Idempotency: `X-Idempotency-Key` header + Redis 24h cache | Frozen |
| ADR-010 | Hook recursion on same entity = runtime panic | Frozen |
| ADR-011 | Compiler output: `CasbinPolicies` renamed to `CapabilityGrants` | Frozen |
| ADR-012 | Organization hierarchy: deferred to v1.1 | Frozen |

---

## ADR-001: Authorization Model

**Decision:** Two distinct layers:

- **Declaration layer** (`awo/def`): `PermissionSet` on `EntityDefinition`. Module authors declare **permission identifiers** — stable names for capabilities (e.g., `"finance.invoice.create"`). No roles, no subjects, no backend constructs. Pure Go struct. Fully engine-agnostic.

- **Enforcement layer** (`awo/auth`): `PolicyEvaluator` interface. Runtime consults it per request. Default implementation: Casbin. Replaceable by any engine (OPA, ReBAC, custom).

```go
type PolicyEvaluator interface {
    CanPerform(ctx context.Context, viewer ViewerContext, object string, action string) (bool, error)
}
```

**Rejected:** Replacing PermissionSet with a capability graph. Mixing declaration and runtime enforcement is the mistake Frappe made.

**Consequence:** PolicyEvaluator is an implementation detail. Module authors never touch it. Casbin can be replaced without changing any entity definitions.

---

## ADR-002: ViewerContext

**Decision:** ViewerContext is embedded in `context.Context` via a typed struct key (`viewerKey{}`). Extracted via `auth.ViewerFromContext()`. Panics if absent — middleware guarantees presence.

```go
type ViewerContext interface {
    TenantID() uuid.UUID
    UserID() uuid.UUID
    ServiceAccountID() uuid.UUID
    Roles() []string
    HasRole(role string) bool
    IsPlatformAdmin() bool
    Actor() *def.Actor
}
```

**Rejected:** Explicit ViewerContext parameter on every repository method — doubles method signature size, makes mocking harder.

**Consequence:** Repository interface remains clean. ViewerContext is always present or the process panics (fail-fast is correct).

---

## ADR-003: Actor Model

**Decision:** Remove `IsPlatformAdmin bool` from `def.Actor`. Add `ServiceAccountID uuid.UUID`. `IsPlatformAdmin()` becomes a method that checks the `Roles` slice.

```go
type Actor struct {
    UserID           uuid.UUID  // uuid.Nil for service accounts
    ServiceAccountID uuid.UUID  // uuid.Nil for humans
    TenantID         uuid.UUID
    Roles            []string
}

func (a *Actor) IsPlatformAdmin() bool { /* checks Roles for "role:platform-admin" */ }
func (a *Actor) IsServiceAccount() bool { return a.ServiceAccountID != uuid.Nil }
```

**Rejected:** Keeping `IsPlatformAdmin bool`. It creates two authorization paths. Privileges belong in Roles.

**Breaking change:** Removes `IsPlatformAdmin bool` field. Any code reading this field must be updated before v1.0.

---

## ADR-004: Session Model

**Decision:** Frozen `auth.Session` struct:

```go
type Session struct {
    Token            string
    UserID           uuid.UUID
    ServiceAccountID uuid.UUID
    TenantID         uuid.UUID
    Roles            []string
    ExpiresAt        time.Time
    IssuedAt         time.Time
    DeviceID         string
    IPAddress        string
    RequestID        string
}
```

Methods: `IsExpired(now time.Time) bool`, `ToActor() *def.Actor`, `ToViewer() auth.ViewerContext`.

Storage: Redis `session:{token}` (JSON), TTL = ExpiresAt − now.

**Consequence:** Session struct is framework-private. Module code never constructs Sessions.

---

## ADR-005: Audit as Mandatory Pipeline Stage

**Decision:** Audit is a mandatory pipeline stage, not an optional hook. The stage is inserted by the runtime between PERSIST and after_save, inside the same transaction:

```
PERSIST → AUDIT RECORD (inside TX) → after_save → [TX commits]
```

Controlled by `EntityDefinition.AuditEnabled bool` (default: `true`). `AuditEnabled: false` is legal only for high-volume, non-sensitive entities (e.g., metrics snapshots, counters).

**Rejected:** Opt-in hooks. Silent compliance gaps are categorically wrong for an ERP framework handling financial data.

**Consequence:** `audit_log` table will be large. Plan for monthly partitioning. The legal compliance is non-negotiable.

---

## ADR-006: SDUI WidgetTree IR

**Decision:** The SDUI generator MUST NOT emit amis JSON directly. An intermediate representation (`awo/sdui/widget.Node`) is required. The amis renderer (`awo/sdui/amis`) converts WidgetTree to amis JSON.

Adding a renderer is O(1). Removing amis is O(1 renderer). Generators need not change.

**Rejected:** Direct amis JSON emission. Permanently couples the framework to one UI renderer. Any amis upgrade requires auditing all generators.

**Consequence:** `awo/sdui/widget` package has zero internal dependencies. `awo/sdui/amis` imports only `awo/sdui/widget`. Generators import only `awo/sdui/widget`.

---

## ADR-007: Workflow Outbox

**Decision:** `StartWorkflow` writes to a `workflow_outbox` table (PostgreSQL), not directly to Temporal. An outbox worker reads pending records and dispatches them to Temporal. This provides durable retry if Temporal is transiently unavailable.

```sql
CREATE TABLE workflow_outbox (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   uuid NOT NULL,
    entity_type text NOT NULL,
    record_id   uuid NOT NULL,
    workflow_fn text NOT NULL,
    task_queue  text NOT NULL,
    workflow_id text NOT NULL,
    input       jsonb NOT NULL,
    status      text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'dispatched', 'failed')),
    attempts    int NOT NULL DEFAULT 0,
    last_error  text,
    created_at  timestamptz NOT NULL DEFAULT now(),
    dispatch_at timestamptz NOT NULL DEFAULT now()
);
```

**Rejected:** Direct `temporal.StartWorkflow()` in request handler. No retry if Temporal is down during the call.

**Consequence:** Temporal start is idempotent via WorkflowID deduplication. If the outbox worker retries a dispatch, Temporal returns the existing workflow run.

---

## ADR-008: Event Outbox Schema Is the Public Contract

**Decision:** The `event_outbox` table schema is a public contract. It MUST NOT be changed without an ADR and migration:

```sql
CREATE TABLE event_outbox (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   uuid NOT NULL,
    topic       text NOT NULL,
    payload     jsonb NOT NULL,
    status      text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'delivered', 'failed')),
    attempts    int NOT NULL DEFAULT 0,
    last_error  text,
    created_at  timestamptz NOT NULL DEFAULT now(),
    deliver_at  timestamptz NOT NULL DEFAULT now()
);
```

The `EventBroker` interface is pluggable: KafkaBroker, NATSBroker, RedisPubSubBroker, NoopBroker.

**Consequence:** External consumers can reliably poll `event_outbox` as a CDC source if needed.

---

## ADR-009: Idempotency Keys

**Decision:** The framework provides server-side idempotency for POST and PATCH operations via the `X-Idempotency-Key` HTTP header. Redis stores: `idempotency_cache:{tenant_id}:{key}` → `IdempotencyRecord{StatusCode, Body, CompletedAt}`. TTL: 24 hours.

On cache hit: return cached response; do not invoke the handler. Response includes `X-Idempotency-Replay: true`.

Redis failure → HTTP 503. Idempotency cannot be guaranteed without Redis; fail closed.

**Rejected:** Client-side-only idempotency. Network retries cause duplicate records without server-side deduplication.

---

## ADR-010: Hook Recursion Policy

**Decision:** If a hook on entity A triggers a mutation on entity A (directly or via chain), the runtime MUST panic with a diagnostic message identifying the recursion chain.

Hook recursion on the same entity type is never correct. It indicates a design error that must be caught at development time.

**Rejected:** Silently ignoring the recursive call (causes infinite loops in production).

---

## ADR-011: Compiler Output Naming

**Decision:** `CompiledSchema.CasbinPolicies []CasbinPolicy` is renamed to `CompiledSchema.CapabilityGrants []CapabilityGrant`. The compiler output MUST NOT reference a specific authorization engine. `PermissionSet` MUST contain permission identifiers — never roles.

```go
type CapabilityGrant struct {
    Permission string  // e.g. "finance.invoice.create" (from PermissionSet)
    Entity     string  // qualified entity name, e.g. "finance_invoice"
    Action     string  // "create" | "read" | "update" | "delete" | custom action name
}
```

The `PolicyEvaluator` implementation (ADR-001) loads `CapabilityGrants` and translates them into its engine's format. The Casbin implementation loads them as `p` assertions binding permission identifiers to entity+action pairs. A separate role-to-permission mapping (managed by the IAM module) is loaded as `g` assertions.

**Rejected:** Keeping `CasbinPolicies`. The compiler must not depend on the Casbin package or its policy format.

**Rejected:** Storing role names in `PermissionSet`. Role-to-permission mapping belongs in the IAM module, not in EntityDefinition. EntityDefinition must remain authorization-backend agnostic.

---

## ADR-012: Organization Hierarchy

**Decision:** Multi-branch, multi-org, and divisional hierarchies within a single tenant are deferred to v1.1. The v1.0 model is single-org per tenant.

**Rationale:** Getting the foundational multi-tenancy model right (one process, many tenants, RLS) is the priority. Adding sub-org hierarchy before the base is stable risks premature design.

**Not deferred:** The `tenant_id` FK on every table remains the isolation unit. Sub-org filtering (if needed in v1.0) is handled by custom `PolicyFunc` predicates, not schema changes.

---

## Public Contracts (Frozen)

The following are public contracts that cannot change without a new ADR and breaking-change notice:

| Contract | Location | Frozen Since |
|---------|----------|-------------|
| `EntityDefinition` interface (14 methods) | `awo/def` | v1.0 |
| `def.EntityRecord` data accessors | `awo/def` | v1.0 |
| `filter.*` function signatures | `awo/filter` | v1.0 |
| `def.ActionRuntime` interface (11 methods) | `awo/def` | v1.0 |
| `def.Actor` struct (post-ADR-003) | `awo/def` | v1.0 |
| `auth.Session` struct | `awo/auth` | v1.0 |
| `auth.ViewerContext` interface | `awo/auth` | v1.0 |
| `widget.Node` struct + `NodeKind` constants | `awo/sdui/widget` | v1.0 |
| `event_outbox` table schema | PostgreSQL | v1.0 |
| `workflow_outbox` table schema | PostgreSQL | v1.0 |
| Entity naming convention `{module}_{noun}` | Convention | Forever |
| `awo.so/awo/*` package paths | Go module | Forever |

---

## Full Rationale

See `awo/docs/ARCH_FREEZE_REVIEW.md` for the full rationale, rejected alternatives, and long-term consequence analysis for each decision.

---

## References

- `awo/docs/ARCH_FREEZE_REVIEW.md` — Constitutional source document
- [`00-overview/ARCH_OVERVIEW.md`](ARCH_OVERVIEW.md) — System architecture
- [`00-overview/PACKAGE_DEPENDENCY_MAP.md`](PACKAGE_DEPENDENCY_MAP.md) — Package DAG
