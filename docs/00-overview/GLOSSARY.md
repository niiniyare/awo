# Awo Framework — Glossary

**Classification:** Specification — Tier 0
**Owner:** `00-overview/GLOSSARY.md`
**Status:** Frozen at v1.0

This document is the single authority for all Awo Framework terminology. Every term defined here MUST be used consistently across all documentation, code comments, error messages, and commit messages. No other document may redefine a term defined here.

---

## A

**Action**
A custom HTTP endpoint declared on an `EntityDefinition` via `ActionDef`. Actions extend standard CRUD with domain-specific operations (e.g., `submit`, `approve`, `cancel`). Auto-generates route: `POST /api/v1/{module}/{entities}/{id}/{action-name}`. See [`13-actions/ACTION_HANDLER_GUIDE.md`](../13-actions/ACTION_HANDLER_GUIDE.md).

**ActionContext**
The parameter injected into every `ActionHandlerFunc`. Contains the request `context.Context`, the target `RecordID`, the `Actor`, the raw request `Body`, and the `ActionRuntime`. The framework constructs this before the handler is called. Declared in `awo/def`.

**ActionRuntime**
The service interface injected into every action handler. Provides: `Repo()`, `Tx()`, `Publish()`, `StartWorkflow()`, `Notify()`, `InvalidateCache()`, `Cache()`, `Clock()`, `Logger()`, `TenantID()`, `Actor()`. Declared in `awo/def`. The concrete implementation is `runtime.DefaultActionRuntime` (internal).

**Actor**
The authenticated principal who initiated an operation. Contains `UserID`, `ServiceAccountID`, `TenantID`, and `Roles`. Exactly one of `UserID` or `ServiceAccountID` is non-nil. `IsPlatformAdmin()` is a method that checks `Roles`, not a boolean field. Declared in `awo/def`. See ADR-003.

**AuditEnabled**
A boolean field on `SystemDefinition` and `CustomDefinition` (default: `true`). When `false`, the runtime skips the AUDIT RECORD pipeline stage for this entity. Setting `false` is legal only for non-sensitive high-volume entities where audit volume is prohibitive. See ADR-005.

**AuditRecord**
The structured record written by the AUDIT RECORD pipeline stage. Contains: entity name, record ID, operation type, tenant ID, actor ID, timestamp, before-state snapshot, after-state snapshot. Written within the same transaction as the mutation. Declared in `awo/audit`.

---

## C

**CapabilityGrant**
A compiled authorization assertion derived from a `PermissionSet` declaration. Contains `Subject` (role), `Object` (qualified entity name), and `Action` (operation). Replaces the old `CasbinPolicy` type (ADR-011). Lives in `awo/compiler`. The default `PolicyEvaluator` implementation loads `CapabilityGrant` slices from the `CompiledSchema` into Casbin.

**Cache**
The `cache.Cache` interface declared in `awo/cache`. Provides `Get`, `Set`, `Delete`, `DeletePrefix`. All keys are tenant-namespaced by the framework. The concrete implementation is Redis-backed (internal). See [`11-cache/CACHE_SPEC.md`](../11-cache/CACHE_SPEC.md).

**CompiledSchema**
The immutable output of `compiler.Compile()`. The single source of truth for all runtime subsystems. Contains `Entities`, `ByName`, `Routes`, `CapabilityGrants`, and `Diagnostics`. Safe for concurrent read access. Never mutated after `Compile()` returns.

**Counter**
The `cache.Counter` interface declared in `awo/cache`. Used by the naming series subsystem for atomic sequence allocation. Provides `Increment(key, delta)` and `Get(key)`.

**CustomDefinition**
A JSONB-backed entity declaration. Fields are stored in a `custom_fields jsonb` column on the `custom_entity_records` table. Use for tenant-specific, frequently-evolving schemas with no financial or inventory participation. Implements `EntityDefinition`. See [`01-entity/ENTITY_DEFINITION_SPEC.md`](../01-entity/ENTITY_DEFINITION_SPEC.md).

**CustomField**
A field added to an entity at runtime by a tenant administrator, without a migration or redeploy. Stored in the `custom_fields jsonb` column. Managed by the Metadata platform module. Accessible via `EntityRecord.CustomFields`.

---

## D

**Domain Event**
A named occurrence in the business domain, emitted via `ActionRuntime.Publish()`. Routed by `Topic` (e.g., `finance.invoice.submitted`). Delivered durably via the event outbox. See [`09-events/EVENT_OUTBOX_SPEC.md`](../09-events/EVENT_OUTBOX_SPEC.md).

---

## E

**EdgeDef**
A relationship declaration between two entities. Specifies the edge `Name`, `Target` (qualified entity name), `Type` (`OneToMany`, `ManyToOne`, `ManyToMany`), and `CascadeDelete` behavior.

**EntityDefinition**
The interface that drives all five framework subsystems: persistence routing, API generation, UI generation, permission evaluation, and workflow triggering. Implemented by `SystemDefinition` and `CustomDefinition`. Declared in `awo/def`. The central primitive of the Awo Framework. See [`01-entity/ENTITY_DEFINITION_SPEC.md`](../01-entity/ENTITY_DEFINITION_SPEC.md).

**EntityRecord**
The in-flight representation of a single entity record during the hook pipeline. Contains `ID`, `TenantID`, `EntityName`, `Data map[string]any`, `CustomFields map[string]any`, `Meta RecordMeta`, `CreatedAt`, and `UpdatedAt`. The `Data` map uses typed values per `FieldType` semantics. Declared in `awo/def`.

**EntitySchema**
The compiled representation of a single `EntityDefinition`. The sole runtime metadata authority for its entity. Populated by the compiler; runtime subsystems read `EntitySchema` directly and never call back to the `EntityDefinition` after startup.

**EventBroker**
The interface in `awo/outbox` that the outbox worker uses to deliver events to a message broker. The framework provides the outbox worker; the broker adapter (Kafka, NATS, Redis PubSub) is pluggable via this interface.

---

## F

**FieldDef**
A field declaration within an `EntityDefinition`. Specifies `Name`, `Type` (`FieldType` constant), constraints (`Required`, `Unique`, `Immutable`, `Sensitive`, `Searchable`), `Default`, `Validators`, and type-specific metadata (e.g., `LinkTarget`, `Options`, `Series`).

**FieldType**
A constant that determines the PostgreSQL column type, the Go value type in `EntityRecord.Data`, the SDUI widget kind, and the Filter DSL predicate set. Declared in `awo/def`. See [`01-entity/FIELD_TYPES_REFERENCE.md`](../01-entity/FIELD_TYPES_REFERENCE.md).

**Filter**
The predicate type used in all repository query methods. Declared in `awo/filter`. Constructors: `Eq`, `Neq`, `Gt`, `Gte`, `Lt`, `Lte`, `In`, `NotIn`, `IsNull`, `IsNotNull`, `Contains`, `StartsWith`, `EndsWith`, `Between`, `And`, `Or`, `Not`, and custom-field variants. See [`06-filter/FILTER_DSL_REFERENCE.md`](../06-filter/FILTER_DSL_REFERENCE.md).

---

## G

**Global Table**
A PostgreSQL table that does NOT have row-level security. Accessible by the application role without a tenant context. Examples: `tenants`, `audit_log`, `timezones`, `currencies`, `countries`. See [`04-multitenancy/GLOBAL_TABLES.md`](../04-multitenancy/GLOBAL_TABLES.md).

---

## H

**Hook**
A Go struct implementing one of the lifecycle hook interfaces (`BeforeCreateHook`, `AfterCreateHook`, etc.). Declared in `HookSet` on `EntityDefinition`. Executed by the runtime at specific pipeline stages. See [`02-pipeline/HOOK_CONTRACT.md`](../02-pipeline/HOOK_CONTRACT.md).

**HookSet**
The collection of hook implementations declared on an `EntityDefinition`. Fields: `BeforeCreate`, `AfterCreate`, `BeforeUpdate`, `AfterUpdate`, `BeforeDelete`, `AfterDelete` — each a slice of the corresponding hook interface. Declared in `awo/def`.

---

## I

**Idempotency Key**
The value of the `X-Idempotency-Key` HTTP header. When present on POST or PATCH requests, the framework stores the response in Redis for 24 hours and returns the stored response on duplicate requests without re-executing the handler. Scoped to `{tenant_id}:{key}`. See ADR-009.

**ImmutableField**
A field with `Immutable: true` in `FieldDef`. May be set on creation. Rejected on any update attempt. The runtime enforces this at the VALIDATE stage.

---

## L

**LedgerEntry**
A mandatory system entity. Represents a double-entry accounting debit or credit. Must always be a `SystemDefinition` — JSONB storage is not acceptable for financial data requiring SQL numeric constraints.

**LocalName**
The module-local entity identifier declared as `Name` in `SystemDefinition` or `CustomDefinition`. Snake_case, no module prefix. E.g., `invoice`, `org_assignment`. Never rename after data is persisted.

---

## M

**Module**
A cohesive business domain package within the Awo Framework. Examples: `finance`, `inventory`, `iam`, `platform`. Modules use the same `EntityDefinition` patterns as each other. No module has special framework access that others lack.

**Module Registry**
The platform module that tracks which business modules are installed and activated per tenant. Prevents unactivated modules from serving requests.

---

## N

**NamingSeries**
An auto-incrementing, pattern-based identifier for entity records. Pattern: `INV-{YYYY}-{SEQ:5}`. Allocated atomically using a Redis counter. Tenant-overridable prefix. Declared as `FieldTypeNamingSeries` on a `FieldDef`. See [`07-naming/NAMING_SERIES_SPEC.md`](../07-naming/NAMING_SERIES_SPEC.md).

---

## O

**Outbox**
A durability pattern where operations (events, workflow starts) are written to a database table within the same transaction as the triggering mutation. A worker process polls the table and performs the delivery with retry logic. Prevents silent data loss on external service failures. See [`08-workflow/OUTBOX_SPEC.md`](../08-workflow/OUTBOX_SPEC.md) and [`09-events/EVENT_OUTBOX_SPEC.md`](../09-events/EVENT_OUTBOX_SPEC.md).

---

## P

**Pipeline**
The ordered sequence of stages executed for every entity mutation: ASSEMBLE → before_validate → VALIDATE → AUTHORIZE → before_save → [TX begins] → PERSIST → AUDIT RECORD → after_save → [TX commits] → Temporal workflow start (outside TX). See [`02-pipeline/LIFECYCLE_SPEC.md`](../02-pipeline/LIFECYCLE_SPEC.md).

**PlatformAdmin**
An actor holding the `role:platform-admin` role. Bypasses all Casbin policy checks. Has access to all tenants and all platform configuration. The `Actor.IsPlatformAdmin()` method checks the `Roles` slice for this value. See ADR-003.

**PgBouncer**
The connection pooler required between the application and PostgreSQL. MUST be configured in **transaction mode** — session mode breaks the transaction-local `set_tenant_context()` reset.

**PolicyEvaluator**
The authorization enforcement interface in `awo/auth`. Decides whether a viewer can perform an action on an object. The default implementation loads `CapabilityGrant` slices into Casbin. Replaceable with OPA, ReBAC, or a custom engine. See ADR-001.

**PermissionSet**
The RBAC declaration on an `EntityDefinition`. Fields: `Create []string`, `Read []string`, `Write []string`, `Delete []string`, `Actions map[string][]string`. Values are role names (e.g., `"role:finance.accounts_payable"`). The compiler converts these to `CapabilityGrant` slices. Declared in `awo/def`.

---

## Q

**QualifiedName**
The globally unique entity identifier derived by the compiler: `{module}_{name}`. E.g., `finance_invoice`, `iam_user`, `platform_organization`. Used as: PostgreSQL table name (system entities), Casbin object, metric label, Redis namespace prefix. Never rename.

---

## R

**Registry**
The `registry.Registry` type that validates and seals a set of `EntityDefinition` values before compilation. `Build()` creates a registry from globally registered definitions. `BuildFrom()` creates an isolated registry for testing. `Seal()` makes the registry read-only.

**RLS (Row Level Security)**
PostgreSQL's mechanism for enforcing tenant isolation at the database level. Every tenant-scoped table MUST have `ENABLE ROW LEVEL SECURITY` and `FORCE ROW LEVEL SECURITY`. The policy predicate is `tenant_id = current_tenant_id()`. See [`04-multitenancy/RLS_SPEC.md`](../04-multitenancy/RLS_SPEC.md).

**RouteDescriptor**
A compiled HTTP route description produced by the compiler for each entity operation. Contains `Method`, `Path`, `EntityQualifiedName`, `Module`, `Resource`, `Operation`, `ActionName`, and `RequiredPermission`. Consumed by the API layer to register Fiber routes.

---

## S

**SDUI (Server-Driven UI)**
The architectural pattern where the server generates complete UI descriptions (as `WidgetTree` nodes) and the client renders them without custom JavaScript. The Awo Framework uses amis as the rendering engine. See [`10-sdui/WIDGET_TREE_SPEC.md`](../10-sdui/WIDGET_TREE_SPEC.md).

**ServiceAccount**
A machine identity. An `Actor` with `ServiceAccountID != uuid.Nil` and `UserID == uuid.Nil`. Service accounts authenticate via API keys. Used for M2M integrations and Temporal activities calling the API.

**Session**
The `auth.Session` struct: `Token`, `UserID`, `ServiceAccountID`, `TenantID`, `Roles`, `ExpiresAt`, `IssuedAt`, `DeviceID`, `IPAddress`, `RequestID`. Stored in Redis at `session:{token}`. The `ToActor()` method converts it to a `def.Actor`. See ADR-004.

**set_tenant_context()**
The PostgreSQL stored procedure that sets `app.current_tenant_id` as a transaction-local GUC variable. Called by the middleware pipeline on every request. Validates tenant exists and is ACTIVE. Required before any tenant-scoped query. See [`04-multitenancy/RLS_SPEC.md`](../04-multitenancy/RLS_SPEC.md).

**SensitiveField**
A field with `Sensitive: true` in `FieldDef`. Excluded from structured log output, API list responses (unless explicitly requested with elevated permissions), and error messages. Examples: `password_hash`, `api_key`.

**SystemDefinition**
A SQL-backed entity declaration. Each field maps to a typed PostgreSQL column. Required for financial, inventory, IAM, and payment data. Implements `EntityDefinition`. See [`01-entity/ENTITY_DEFINITION_SPEC.md`](../01-entity/ENTITY_DEFINITION_SPEC.md).

---

## T

**Tenant**
The top-level isolation boundary. A `Tenant` record in the `tenants` global table. Every tenant-scoped operation requires a resolved `TenantID` in the request context and a matching `set_tenant_context()` call. Tenant lifecycle: PENDING → ACTIVE → SUSPENDED → ARCHIVED.

**TenantID**
The `uuid.UUID` primary key of a `Tenant` record. Carried in `ViewerContext`, `Session`, `Actor`, `EntityRecord`, and every outbox record.

**Temporal**
The durable workflow orchestration platform used by Awo for long-running processes. Awo modules declare `WorkflowTrigger` values on `EntityDefinition` to bind Temporal workflows to lifecycle events. See [`08-workflow/TEMPORAL_INTEGRATION.md`](../08-workflow/TEMPORAL_INTEGRATION.md).

---

## V

**ViewerContext**
The authorization interface embedded in `context.Context` by the middleware pipeline. Provides `TenantID()`, `UserID()`, `ServiceAccountID()`, `Roles()`, `HasRole()`, `IsPlatformAdmin()`. Extracted by `auth.ViewerFromContext(ctx)`. Panics if absent. Declared in `awo/auth`. See ADR-002.

---

## W

**WidgetTree**
The intermediate representation (IR) between the SDUI generator and the amis renderer. A tree of `widget.Node` values, each with a `NodeKind` constant describing the semantic widget type. The amis renderer converts `*widget.Node` trees to `map[string]any` amis JSON. See ADR-006.

**WorkflowOutbox**
The `workflow_outbox` PostgreSQL table. Every `StartWorkflow()` call writes a record here within the entity's transaction. An outbox worker dispatches to Temporal after commit, with exponential backoff retry. See ADR-007.

**WorkflowTrigger**
A declaration on `EntityDefinition` that binds a Temporal workflow start to an entity lifecycle event (`EventOnCreate`, `EventOnUpdate`, `EventOnDelete`, `EventOnSubmit`, etc.). The `InputBuilder` function produces the workflow input from the `EntityRecord` and `TriggerContext`.

---

## Z

**Zero Value**
The typed zero for each `FieldType` in `EntityRecord.Data`. String → `""`, int64 → `0`, decimal → `decimal.Zero`, bool → `false`, UUID → `uuid.Nil`, time.Time → zero time. The `Get()` method returns `nil` for absent keys; typed accessors return the zero value.
