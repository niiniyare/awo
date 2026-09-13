# Awo Framework — Decision Register

**Classification:** Constitutional — Tier 0
**Owner:** `00-overview/DECISION_REGISTER.md`
**Status:** Active — see `awo/tasks.md` for implementation phases
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
| ADR-013 | Transaction ownership: driver (`contrib/pgx`) owns and manages TX | Frozen |
| ADR-014 | Audit integration: pipeline AUDIT RECORD stage via `AuditWriter`; no repository wrapper | Frozen |
| ADR-015 | Unified actor model: `def.Actor` is the single actor type; no parallel audit actor | Frozen |
| ADR-016 | Audit configuration: `EntityAuditConfig` registry in `awo/audit`; no `def` modification | Frozen |
| ADR-017 | Audit failure policy: category-driven; ADMIN/SECURITY propagate; others suppress+log+meter | Frozen |
| ADR-018 | Audit storage: `platform_audit_log`, monthly range partitions, no RLS, global table | Frozen |
| ADR-019 | Dual audit elimination: unified `platform_audit_log` supersedes `iam_audit_log` + SQL triggers | Frozen |
| ADR-020 | Audit partition maintenance: pg_cron extension; no application-layer scheduler | Frozen |
| ADR-021 | Session.Metadata: extensible map for framework-reserved context keys | Active |
| ADR-022 | EntityScope: 5-level data isolation enum on EntityDefinition | Active |
| ADR-023 | AllowAudit: opt-out bool on EntityDefinition supersedes ADR-016 audit.Register pattern for disable | Active |
| ADR-024 | SessionValidator interface: decouples auth middleware from IAM concrete type | Active |

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

**Decision:** `auth.Session` struct (amended by ADR-021):

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
    Metadata         map[string]any  // ADR-021: framework-reserved; keys prefixed "awo:"
}
```

Methods: `IsExpired(now time.Time) bool`, `ToActor() *def.Actor`, `ToViewer() auth.ViewerContext`.

Storage: Redis `session:{token}` (JSON), TTL = ExpiresAt − now.

**Consequence:** Session struct is framework-private. Module code never constructs Sessions. See ADR-021 for Metadata usage constraints.

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

**Original decision:** Deferred to v1.1.

**Amended (Phase 8):** Organization hierarchy is now a framework-native platform entity implemented via `platform_organization` with `ltree`-backed path column. `ScopeOrganization` and `ScopeOrganizationTree` scopes (ADR-022) depend on it. The `tenant_id` FK remains the primary isolation unit; organization hierarchy adds a second filtering dimension within a tenant.

**Implementation:** `platform/registry` module registers `platform_organization`. `ScopeOrganizationTree` queries use ltree `<@` ancestor operator. See `awo/tasks.md` Phase 8.

---

## ADR-013: Transaction Ownership

**Decision:** The service layer (`api/service.EntityService`) orchestrates all database transactions for entity mutations. Transactions are opened via `repo.WithTx()` — a method on `driver.EntityRepository[T]` implemented by `contrib/pgx`. The driver does NOT open transactions automatically for individual `Create`, `Update`, or `Delete` calls — those are auto-commit unless wrapped in `WithTx`.

The actual execution sequence in `EntityService.Create`:

```
pipeline.RunBeforeCreate(pctx)               // OUTSIDE TX

repo.WithTx(ctx, func(txCtx context.Context) error {
    // TX begins here (inside driver.WithTx)
    repo.Create(txCtx, input)               // PERSIST — uses TX from ctx
    pipeline.RunAuditRecord(txCtx, ...)     // AUDIT RECORD — inside TX
    pipeline.RunAfterCreate(txCtx, created) // after_save hooks — inside TX
    return nil
})                                           // TX commits; or rolls back on error

startWorkflows(ctx, ...)                     // OUTSIDE TX
```

The `context.Context` carries the active transaction connection (set by `repo.WithTx` via `tx.WithConn`). Any database operation inside the `WithTx` callback that extracts the connection from `ctx` automatically participates in the same transaction.

**Repository wrapper rejected:** A wrapper implementing `driver.EntityRepository[T]` that calls the inner driver's `Create()` and then writes an audit record afterward writes the audit record OUTSIDE the transaction boundary managed by `repo.WithTx`. This is a correctness failure. Repository wrapper pattern MUST NOT be used for audit integration.

**Correct integration:** `pipeline.RunAuditRecord` is called by `EntityService` inside `repo.WithTx`, between `repo.Create/Update/Delete` and `pipeline.RunAfterXxx`. The `Pipeline` holds the `AuditWriter` and calls it from within the TX-carrying context. See ADR-014.

---

## ADR-014: Audit Integration Point

**Decision:** The AUDIT RECORD pipeline stage (ADR-005) is the sole integration point for audit writing. The pipeline holds a reference to an `AuditWriter` interface. The driver calls `pipeline.RunAuditRecord(ctx, record)` from within its open transaction. The `AuditWriter` implementation executes an INSERT into `platform_audit_log` using the transaction-carrying context.

```go
// awo/audit — public contract
type AuditWriter interface {
    // Write appends one audit record. ctx must carry an active database
    // transaction; the write participates in that transaction.
    // If the entity's audit config suppresses audit, Write returns nil
    // without writing.
    Write(ctx context.Context, record AuditRecord) error
}
```

The pipeline wires the `AuditWriter` at construction time. Module code never calls `AuditWriter` directly.

**Rejected:** Repository wrapper (`AuditingRepository[T]`). With Model B confirmed (ADR-013), a post-call audit write is outside the committed TX. The wrapper approach is architecturally unsound at this stack layer.

**Rejected:** Optional `AfterSave` hook for audit writing. ADR-005 mandates audit as a non-optional pipeline stage to prevent silent compliance gaps.

**Consequence:** `AuditWriter` is injected into the pipeline at bootstrap. Any system that does not inject an `AuditWriter` must provide a `NoopAuditWriter`. There is no default global writer.

---

## ADR-015: Unified Actor Model

**Decision:** `def.Actor` (defined in `awo/def`) is the single actor type used everywhere in the framework, including audit records. No parallel `audit.Actor` type exists. The `audit` package imports `def.Actor` — not the reverse.

```go
// def.Actor is the canonical actor (ADR-003)
type Actor struct {
    UserID           uuid.UUID  // uuid.Nil for service accounts
    ServiceAccountID uuid.UUID  // uuid.Nil for human users
    TenantID         uuid.UUID
    Roles            []string
}
```

Background operations (bootstrap, migrations, outbox relay, cron jobs) use typed `SystemActor` constants defined in `awo/audit`:

```go
// awo/audit — SystemActor values for non-human execution contexts
// These are stored in AuditRecord.SystemActor; AuditRecord.Actor is nil.
type SystemActor string

const (
    SystemBootstrap   SystemActor = "system:bootstrap"
    SystemMigration   SystemActor = "system:migration"
    SystemOutboxRelay SystemActor = "system:outbox-relay"
    SystemScheduler   SystemActor = "system:scheduler"
)
```

**Rejected:** A separate `audit.Actor` struct. Two actor models require two construction paths, two serialization formats, and perpetual translation code. The `def.Actor` already captures all required identity fields.

**Consequence:** `audit.AuditRecord.Actor` is `*def.Actor` (nil for system operations). `audit.AuditRecord.SystemActor` is `SystemActor` (empty string for human/service-account operations). Exactly one of the two is set per record.

---

## ADR-016: Audit Entity Configuration

**Decision:** The `def.SystemDefinition` and `def.CustomDefinition` structs are frozen kernel types (v1.0). Adding audit-specific fields to them would violate the freeze. Instead, audit configuration for an entity is declared in `awo/audit` via `EntityAuditConfig`:

```go
// awo/audit
type EntityAuditConfig struct {
    EntityName   string        // qualified name, e.g. "finance_invoice"
    Enabled      bool          // default: true; false suppresses all audit for entity
    Category     EventCategory // DATA | AUTH | ACCESS | ADMIN | WORKFLOW | SYSTEM | OUTBOUND | SECURITY
    SensitiveFields []string   // field names to strip before snapshot (in addition to def.FieldDef.Sensitive)
    // FailurePolicy is derived from Category; not configurable per entity.
}

// Register declares audit configuration for an entity.
// Call from init() alongside def.Register().
func Register(cfg EntityAuditConfig) { ... }
```

The `audit.RiskScorer` reads sensitivity configuration from both `def.FieldDef.Sensitive` (compile-time) and `audit_sensitive_fields` (runtime DB config). The `EntityAuditConfig` registry is the authoritative source for per-entity overrides.

**Amended by ADR-023:** `def.SystemDefinition.DisableAudit bool` (opt-out) is added to the struct to allow high-volume entities to skip audit without a separate registry call. The `audit.Register()` pattern remains for per-entity category/sensitivity configuration. `DisableAudit: true` is equivalent to `EntityAuditConfig{Enabled: false}` and takes precedence.

**Rejected:** Adding `AuditEnabled bool` and `AuditCategory` fields to `def.SystemDefinition`. Frozen kernel — no modifications permitted post-v1.0.

**Rejected:** Reading audit config entirely from the database at query time. Startup warm-up cache is required (synchronous, before HTTP server starts) to avoid per-request DB reads on the audit path.

**Consequence:** Module authors call `audit.Register(EntityAuditConfig{...})` from `init()`. The audit package reads this registry. Entities without an explicit registration use defaults (Enabled: true, Category: DATA).

---

## ADR-017: Audit Failure Policy

**Decision:** Whether an audit write failure propagates (aborts the originating mutation) or is suppressed (logged, metered, but mutation proceeds) is determined by the event category of the audit record.

| Category | Failure Policy | Rationale |
|----------|---------------|-----------|
| ADMIN | **Propagate** — mutation aborts | Admin actions without a trail are a security violation |
| SECURITY | **Propagate** — mutation aborts | Security events must be recorded or denied |
| DATA | Suppress + log + meter | High-volume; partial audit gap acceptable at ERP scale |
| AUTH | Suppress + log + meter | Auth events written by IAM service; failure is non-critical |
| ACCESS | Suppress + log + meter | Access denials are already tracked at PolicyEvaluator layer |
| WORKFLOW | Suppress + log + meter | Workflow outbox provides independent durability |
| SYSTEM | Suppress + log + meter | System events are operational, not compliance-critical |
| OUTBOUND | Suppress + log + meter | Outbound events tracked by event_outbox independently |

Suppress means: the `AuditWriter.Write` error is logged at ERROR level, a `audit_write_failure_total` Prometheus counter is incremented (labeled by category), and nil is returned to the caller.

Propagate means: the `AuditWriter.Write` error is returned unmodified. The driver rolls back the transaction.

**Rejected:** Propagate-all (write fails → mutation aborts for every entity). Renders high-volume DATA operations fragile against audit table I/O spikes.

**Rejected:** Suppress-all. Allows silent compliance gaps on ADMIN and SECURITY operations, which is a regulatory violation.

---

## ADR-018: Audit Storage Architecture

**Decision:** A single `platform_audit_log` table replaces the two existing audit implementations (`iam_audit_log` Go entity and `audit_log` SQL trigger table). The unified table is:

- **Global** — no RLS, no per-tenant isolation at the table level. Tenant scoping is enforced by permission checks (IAM) and query filters (application layer).
- **Monthly range-partitioned** on `created_at`. Bootstrap creates current month + 2 months ahead. A scheduled job creates future partitions and archives old ones.
- **Append-only** — `awo_app` role has INSERT + SELECT only. UPDATE and DELETE are reserved for the `audit_retention_role` PostgreSQL role (GDPR anonymization only).
- **Schema:** See `12-audit/AUDIT_STORAGE.md` for full DDL and partition management.

Columns required beyond the existing `audit_log` schema (ADR-005):
- `severity` TEXT — CRITICAL | HIGH | MEDIUM | LOW | INFO
- `risk_score` SMALLINT — 0–100 composite score
- `event_category` TEXT — one of 8 categories (ADR-017)
- `compliance_flags` JSONB — GDPR/PCI-DSS/KRA-eTIMS flags from config
- `session_id` TEXT — HMAC-SHA256 of session token (never raw token)
- `context` JSONB — event-type-specific context (request, system, workflow)
- `system_actor` TEXT — populated for system operations; NULL for human/SA

**Rejected:** Keeping both `iam_audit_log` and `audit_log`. Two disconnected implementations produce inconsistent compliance reports, duplicate storage, and divergent schemas.

**Rejected:** Full hash chain per record (tamper evidence v2). Deferred to v2 — SHA-256 integrity checkpoints per partition are sufficient for v1 tamper evidence.

---

## ADR-019: Dual Audit Elimination

**Decision:** The legacy dual audit system is superseded by the unified `platform_audit_log` (ADR-018):

| Legacy | Status | Migration |
|--------|--------|-----------|
| `iam_audit_log` Go entity (`awo/platform/audit/definition.go`) | **Retired** | Historical records migrated to `platform_audit_log` |
| SQL trigger `audit_log` table (migrations 000448–000451) | **Retired** | SQL triggers removed; trigger management functions dropped |
| `awo/platform/audit/` package | **Repurposed** | Becomes thin adapter exposing `platform_audit_log` read API |

Migration is gated by the `feature.unified_audit.enabled` feature flag in `platform_audit_config`. When the flag is false, the legacy system remains active. When true, the unified system is active. Blue-green migration completes when all historical records are migrated and the flag is permanently set to true.

**Consequence:** The `audit_log` SQL trigger infrastructure (functions `enable_audit_on_table`, `audit_trigger_function`, `get_audit_statistics`, etc.) is removed via a migration. The Go `audit.LogDefinition` entity registration is removed. The IAM module's `iam.audit_log.read` permission is replaced by `platform.audit_log.read`.

---

## ADR-020: Audit Partition Maintenance

**Decision:** Monthly `platform_audit_log` partitions are created by a `pg_cron` scheduled job, not by application code or Temporal workflows. The pg_cron job runs on the 25th of each month and calls a PostgreSQL stored function `platform_create_next_audit_partition()`.

```sql
-- Scheduled in migration 000452
SELECT cron.schedule(
    'create-audit-partitions',
    '0 9 25 * *',
    $$SELECT platform_create_next_audit_partition()$$
);
```

The stored function uses `IF NOT EXISTS` semantics — running it multiple times is safe.

If pg_cron is unavailable in the deployment environment, the partition maintenance function must be called manually (or via external cron) on the 25th of each month. This is documented as an operational requirement.

**Rejected:** Application-layer partition creation at bootstrap (creates a 2-month lookahead only; does not handle ongoing monthly creation). **Rejected:** Temporal scheduled workflow (Temporal is optional/degraded-ok; partition maintenance must be reliable even when Temporal is unavailable).

**Consequence:** Deployment environments must have the pg_cron extension installed and enabled. If not, `cron.schedule()` in the migration will fail — conditionally execute it with `DO $$ BEGIN ... EXCEPTION WHEN undefined_function THEN NULL; END $$`.

---

## ADR-021: Session.Metadata Extension Field

**Decision:** `auth.Session` gains a `Metadata map[string]any` field (JSON: `"metadata,omitempty"`). This field is reserved for framework-internal use. All keys MUST be prefixed with `"awo:"`. Module code MUST NOT write to Metadata; it is populated by framework middleware (e.g., `"awo:request_id"`, `"awo:device_fingerprint"`).

**Rationale:** Avoids future breaking changes to the frozen Session struct for framework-internal context propagation needs (audit enrichment, feature flag context, etc.).

**Constraint:** Metadata values MUST be JSON-serializable. Metadata is included in Redis storage (TTL-bounded). No sensitive data in Metadata — use `awo:` prefix as a namespace fence.

---

## ADR-022: EntityScope — Data Isolation Boundaries

**Decision:** `EntityDefinition` gains two new methods:

```go
EntityScope() Scope    // returns isolation level; default ScopeTenant
AllowAudit() bool      // returns false only for DisableAudit: true entities
```

`def.Scope` type with 5 constants:

```go
const (
    ScopeSystem           Scope = "system"           // global; no tenant_id, no RLS
    ScopeTenant           Scope = "tenant"           // default; RLS on tenant_id
    ScopeOrganization     Scope = "organization"     // RLS on tenant_id + org filter
    ScopeOrganizationTree Scope = "organization_tree"// RLS on tenant_id + ltree ancestor
    ScopeUser             Scope = "user"             // private to creating user
)
```

`EntitySchema` in `awo/compiler` carries `Scope def.Scope` and `AllowAudit bool` propagated at compile time.

**Consequence:** The `EntityDefinition` interface grows from 14 to 16 methods. The public contracts table is updated. Existing implementations that embed `SystemDefinition` or `CustomDefinition` get these methods for free (default: `ScopeTenant`, `AllowAudit: true`).

---

## ADR-023: DisableAudit on def Structs

**Decision:** `SystemDefinition.DisableAudit bool` and `CustomDefinition.DisableAudit bool` fields are added to the frozen structs as the canonical opt-out mechanism. ADR-016's `audit.Register(EntityAuditConfig{Enabled: false})` pattern remains valid but `DisableAudit: true` is preferred for entities that are high-frequency by design.

**Rationale:** Requiring a separate `audit.Register()` call for structural opt-outs (not runtime config) creates split config: the entity definition says nothing about audit, but a separate init() call controls it. The bool field co-locates the intent.

**Consequence:** `AllowAudit() bool` on `EntityDefinition` delegates to `!DisableAudit`. The audit pipeline reads `EntitySchema.AllowAudit` (compiled from the definition) rather than querying the `audit.EntityAuditConfig` registry for the enabled flag. The registry continues to handle Category and SensitiveFields.

---

## ADR-024: SessionValidator Interface

**Decision:** `auth.SessionValidator` interface is introduced in `awo/auth`:

```go
type SessionValidator interface {
    ValidateToken(ctx context.Context, token string) (*Session, error)
    ValidateAPIToken(ctx context.Context, rawToken string) (*Session, error)
}
```

The `api/middleware` auth middleware depends on `auth.SessionValidator`, not `*iam.Module`. The `iam.Module` implements `SessionValidator`. This breaks the circular dependency: `api/middleware` → `awo/auth` (no import of `platform/iam`).

**Consequence:** Bootstrap wires `iamModule` as `SessionValidator` when IAM is available. In dev mode (auth disabled), a `NoopSessionValidator` or nil check skips validation entirely. The middleware layer is now testable without IAM.

---

## Public Contracts (Frozen)

The following are public contracts that cannot change without a new ADR and breaking-change notice:

| Contract | Location | Frozen Since |
|---------|----------|-------------|
| `EntityDefinition` interface (16 methods) | `awo/def` | v1.0 (amended ADR-022) |
| `def.EntityRecord` data accessors | `awo/def` | v1.0 |
| `filter.*` function signatures | `awo/filter` | v1.0 |
| `def.ActionRuntime` interface (11 methods) | `awo/def` | v1.0 |
| `def.Actor` struct (post-ADR-003) | `awo/def` | v1.0 |
| `auth.Session` struct | `awo/auth` | v1.0 (amended ADR-021) |
| `auth.SessionValidator` interface | `awo/auth` | v1.0 (ADR-024) |
| `auth.ViewerContext` interface | `awo/auth` | v1.0 |
| `widget.Node` struct + `NodeKind` constants | `awo/sdui/widget` | v1.0 |
| `event_outbox` table schema | PostgreSQL | v1.0 |
| `workflow_outbox` table schema | PostgreSQL | v1.0 |
| `audit.AuditWriter` interface | `awo/audit` | v1.0 |
| `audit.AuditRecord` struct | `awo/audit` | v1.0 |
| `audit.EntityAuditConfig` registry API | `awo/audit` | v1.0 |
| `platform_audit_log` table schema (base columns) | PostgreSQL | v1.0 |
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
- [`12-audit/AUDIT_ARCH.md`](../12-audit/AUDIT_ARCH.md) — Unified audit architecture (ADR-013 through ADR-019)
- [`12-audit/AUDIT_SPEC.md`](../12-audit/AUDIT_SPEC.md) — Audit pipeline stage specification (ADR-005)
