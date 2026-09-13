> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Awo Framework Glossary"
id: gloss-001
status: accepted
category: GLOSSARY
stability: evolving
audience: [all]
since: "1.0"
normative-level: normative
---

# Awo Framework Glossary

**GLOSS-001 | Status: Accepted | Stability: Evolving**

This document defines every term used across Awo Framework documentation with canonical meaning. All other documents MUST use terms exactly as defined here. When a document introduces a term not defined here, the author MUST submit a glossary addition before the document may be accepted.

Terms are sorted alphabetically. Each definition is the single source of truth for that concept. Cross-references appear as `→ Term` inline.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, SHOULD NOT, RECOMMENDED, MAY, and OPTIONAL in this document are to be interpreted as described in [RFC 2119](https://www.rfc-editor.org/rfc/rfc2119).

---

## A

**Action**
A named, permission-gated operation on a specific → Entity record beyond standard CRUD. Declared as an → ActionDef on the → EntityDefinition. Auto-generates an HTTP route at `POST /api/v1/entities/{entity-type}/{id}/{action-name}`. Actions receive a pre-resolved, permission-checked → ActionContext and return an → ActionResult.

**ActionContext**
The runtime argument passed to every → Action handler. Contains: a scoped → EntityRepository with tenant isolation and permissions already applied; the target record UUID; and the authenticated → Actor from the → Session context. Action handlers must not perform their own tenant or authentication wiring.

**ActionDef**
The declaration struct for a single → Action within an → EntityDefinition. Fields: `Name` (stable identifier used in URL path), `Method`, `Label`, `Permission` (required Casbin role/permission string), `HandlerFunc`.

**ActionResult**
The return value of an → Action handler. Contains a human-readable message and optional structured payload. The framework serializes it into the standard → Response Envelope with HTTP 200 or 202 depending on whether a → Workflow was triggered.

**Activity** (Temporal)
A function that performs I/O or external side effects within a → Workflow. Activities are retried independently on failure. All database writes, email sends, HTTP calls to external services, and file operations MUST be implemented as activities, never inside workflow functions directly. See: → Workflow, → Saga Pattern.

**Actor**
The authenticated principal making a request. An actor is resolved from the → Session token at the start of every request. Contains: user UUID, tenant UUID, role set, and branch scope (when applicable). All → Permission evaluations and → Privacy Policy injections use the actor from context.

**ADR** (Architecture Decision Record)
A document that records a significant architectural decision: the context, the options considered, the decision made, and the consequences. ADRs are append-only — once accepted, they are never edited to change their decision. Superseded ADRs are marked deprecated but not deleted. See: `→ Document Category`.

**Aggregate**
A read-only computed value over a set of → Entity records. Defined via `AggregateSpec` passed to `EntityRepository.Aggregate()`. Supported operations include: SUM, COUNT, AVG, MIN, MAX on typed → Field values. Aggregates are always tenant-scoped and respect → Privacy Policies.

**amis**
The open-source React-based schema renderer (Baidu) used as the → SDUI rendering engine in Awo's reference implementation. amis interprets → Page Schema JSON to produce fully interactive UI without custom JavaScript. The amis SDK is pinned in `web/sdk/` and must never be auto-updated. See: → SDUI, → Page Schema, → Page Builder.

**amis SDK**
The pinned copy of the amis JavaScript and CSS bundle located at `web/sdk/`. It must not be updated without a full compatibility audit of all → Page Builder output. The SDK version is fixed per Awo release.

**API Layer**
The second layer in Awo's → Five-Layer Architecture. Implemented with Fiber v2. Contains thin route handlers (~50 lines maximum), the → Middleware Pipeline, and request/response marshaling. The API layer must not contain business logic; it delegates to the → Domain Layer.

**Architectural Invariant**
A property of the framework that must hold unconditionally across all implementations, configurations, and versions within a major version. Invariants are stated normatively in `awo/docs/02-architecture/invariants.md`. Violation of an architectural invariant is a defect regardless of whether the violation produces observable misbehavior. See: → Architecture Law.

**Architecture Law**
A formally numbered rule (LAW-NNN) that is permanently binding on all framework contributors and implementors. Laws differ from → Architectural Invariants in that laws govern authoring behavior (what contributors must do), while invariants govern runtime behavior (what the running system must uphold). Laws are enumerated in `awo/docs/02-architecture/laws.md`.

**Audience**
A frontmatter field in every Awo documentation document. Declares who the document is written for. Valid values: `framework-authors`, `module-authors`, `application-developers`, `operators`, `contributors`, `maintainers`, `core-team`, `all`. Documents are written for their declared audience; language and assumed knowledge are calibrated accordingly.

**Audit Log**
The tamper-evident record of every data-mutating operation on tenant-scoped data. Maintained by the `audit` → Platform Module. Every → Create, Update, and Delete operation produces an audit entry. Audit log records are system entities written in the same transaction as the mutation. The audit log is a legal compliance record and must never be truncated or modified by application code.

---

## B

**BeforeCreate Hook**
A → Hook that executes before an → EntityRecord is persisted for the first time. Runs outside the database transaction. May abort the operation by returning a → ValidationError or → BusinessError. Receives the record in its pre-persisted state; changes to the record at this stage affect what is written.

**BeforeValidate Hook**
A → Hook that executes before field-level validation. Runs outside the database transaction. Used to normalize or transform input before constraints are checked. May not abort the operation directly but may modify the record.

**BulkCreate**
The `EntityRepository.BulkCreate(ctx, []CreateInput)` method. Persists multiple → Entity records in a single database round-trip. Must be used instead of looping `Create` when inserting more than a small number of records to avoid → N+1 writes. Subject to the same → Hook execution and → Tenant isolation as single-record `Create`.

**BulkUpdate**
The `EntityRepository.BulkUpdate(ctx, Filter, Patch)` method. Applies a set of field patches to all records matching a → Filter in a single statement. Hooks are not invoked per-record on bulk updates — use with caution in business logic.

**BusinessError**
A domain-rule violation error. HTTP status set by the error itself (400, 409, etc.). Contains: `Code` (machine-readable, e.g. `invoice.already_submitted`), `Message` (user-facing), `Status` (HTTP). Must be returned via `errors.As` unwrapping at the handler layer; never use a type switch on errors directly.

---

## C

**Capability Token**
A namespaced string declared in a → Module Manifest that advertises what a module provides or requires. Format: `{reverse-DNS-domain}/{capability-name}`. Used by the → Entity Registry to verify module dependencies at → Initialization Phase. Example: `so.awo/capability/finance.ledger`.

**Casbin**
The RBAC enforcement library used by Awo's → IAM module. Casbin evaluates `(subject, domain, object, action)` policy tuples. Domain is the tenant UUID (for tenant-scoped policies) or `_platform_` (for platform-admin policies). Object is the → Entity type name. Subject is `user:{uuid}` or `role:{name}`.

**Compilation Phase**
The stage between the end of the → Initialization Phase and the start of the → Runtime Phase. During compilation, the → Entity Registry invokes the compiler subsystem to build → CompiledSchema from all registered → EntityDefinition instances. No registrations are accepted after compilation begins. The resulting CompiledSchema is immutable for the lifetime of the process. See: → LAW-003, → LAW-004.

**CompiledSchema**
The immutable, process-lifetime snapshot of all registered → EntityDefinitions, produced by the compiler subsystem at the end of → Compilation Phase. The CompiledSchema is the single source of truth for the runtime. All route generation, permission evaluation, SDUI schema generation, and migration validation derive from CompiledSchema. Its content hash uniquely identifies the schema version. See: → LAW-004, → LAW-010, → LAW-019.

**Content Hash** (Schema)
The SHA-256 hash of the canonical serialization of the → CompiledSchema. Used as the schema identity across cluster instances. All instances of the same binary running the same configuration must produce identical content hashes. See: → LAW-010, → LAW-019, → Schema Divergence.

**CreateInput**
The typed struct passed to `EntityRepository.Create()`. Carries field values for the new record. The structure of CreateInput is defined by the framework kernel and extended by field type handlers. CreateInput does not accept raw SQL or arbitrary key-value maps; all fields must be declared in the → EntityDefinition.

**Currency** (Field Type)
A → Field type that stores monetary values. Backed by `numeric(20,4)` in PostgreSQL. Go representation: `decimal.Decimal`. Money MUST always use this type. Using `float`, `double precision`, or any floating-point type for monetary values is prohibited unconditionally.

**Custom Entity**
An → Entity whose records are stored as JSONB documents in a tenant-scoped table. Used when schema flexibility is required, when the entity is tenant-specific, and when the entity does not participate in financial calculations or IAM. Custom entities have no typed SQL columns beyond system columns (`id`, `tenant_id`, `created_at`, `updated_at`). Escalate to → System Entity when record volume exceeds 10M, when fields are used in financial calculations, or when FK constraints to system entity PKs are required.

**Custom Field**
A → Field added to an existing entity at runtime by a tenant administrator, without migration or redeployment. On → System Entities, stored in the `custom_fields jsonb` column. On → Custom Entities, stored as additional keys in the same JSONB document. The → Metadata Module manages custom field definitions. API, SDUI, and Filter DSL treat custom fields identically to declared fields.

---

## D

**Data Layer**
See: → Store Layer.

**Domain Layer**
The third layer in Awo's → Five-Layer Architecture. Contains → EntityDefinitions, → Hooks, validators, and → Policy Functions. Must be stateless. Has zero external dependencies at the interface level. No layer above the domain layer may be imported from the domain layer.

**Driver**
An interface that abstracts an infrastructure dependency. Core driver interfaces include: `driver.EntityStore` (PostgreSQL), `driver.SessionStore` (Redis), `driver.WorkflowClient` (Temporal), `driver.EventBus`. Drivers are never concrete types in domain or application code; only the → Store Layer and framework internals import driver implementations. See: → LAW-009.

**DynamicLink** (Field Type)
A → Field type representing a polymorphic foreign reference. Stores two columns: `link_type` (the target entity type name) and `link_name` (the target record identifier). Used when a field may reference records from multiple entity types. Contrast with → Link, which references a single entity type.

---

## E

**Edge**
A declared relationship from one → Entity to another. Defined by → EdgeDef on the → EntityDefinition. Edges are loaded explicitly via → QueryOption; they are never lazy-loaded. See: → LAW (never lazy-load edges).

**EdgeDef**
The declaration struct for a single → Edge on an → EntityDefinition. Fields: `Name`, `Target` (entity type name), `Type` (OneToOne, OneToMany, ManyToMany), `CascadeDelete` (bool).

**Entity**
A named, structured data object managed by the framework. Every entity has a stable name (format: `{module}_{noun}`), typed fields, declared edges, lifecycle hooks, permission policies, and workflow triggers — all declared in a single → EntityDefinition. Entity names are globally unique and are embedded in migration filenames, Temporal workflow IDs, Redis cache keys, and Casbin policies. Entity names must never be renamed after first use. See: → LAW-011.

**Entity Name**
The stable string identifier for an → Entity. Format: `{module}_{noun}`, all lowercase, underscores only. Examples: `finance_invoice`, `inventory_stock_move`, `iam_user`. Embedded in URL paths, Temporal workflow IDs, Redis keys, and Casbin policies. Globally unique within a deployment. Must never be renamed after first use. See: → LAW-011.

**EntityDefinition**
The central primitive of the Awo Framework. A single `EntityDefinition` declaration drives five subsystems simultaneously: persistence routing, API generation, SDUI generation, permission evaluation, and workflow triggering. Registered at startup via `def.Register()`. The EntityDefinition is the only way to introduce an entity into the framework; direct table creation or route registration is prohibited.

**EntityRecord**
The runtime representation of a single persisted → Entity instance. Contains: `ID` (UUID v7), `TenantID`, typed field values, computed edge references (when loaded), and lifecycle metadata (`CreatedAt`, `UpdatedAt`, `CreatedBy`). EntityRecords are always tenant-scoped; the framework rejects any operation on an EntityRecord without a resolved → TenantContext.

**Entity Registry**
The framework subsystem responsible for accepting → EntityDefinition registrations during the → Initialization Phase, validating constraints (name uniqueness, field type legality, edge target existence), and invoking the compiler to produce → CompiledSchema at the end of the Initialization Phase. After compilation, the registry is frozen; no further registrations are accepted. See: → LAW-003.

**EntityRepository**
The typed interface through which all entity persistence operations are performed. Provides: `Get`, `Query`, `Exists`, `Count`, `Aggregate`, `Create`, `Update`, `Delete`, `BulkCreate`, `BulkUpdate`, `WithTx`. Never exposes raw SQL, ORM types, connection management, or lazy loading. All business logic and application code must interact with persistence exclusively through this interface. See: → LAW (never use ORM types directly).

---

## F

**Feature Flag**
A named, per-tenant boolean switch managed by the → Feature Flags Module. Evaluation order: system default → tenant override → user override. Cached in Redis at `eval:{SHA256(flag+tenant+user)}` with 5-minute TTL. Used to gate UI elements (absent from → Page Schema when off), API behavior, and module activation. Flag changes invalidate cache immediately.

**Feature Flags Module**
The → Platform Module (`internal/platform/flags`) that manages → Feature Flags. Provides per-tenant on/off switches without deployment. Flags are Redis-cached; the module handles cache invalidation on flag changes.

**Field**
A single typed data attribute on an → Entity. Declared in the `Fields []FieldDef` of an → EntityDefinition. Every field has a stable name (lowercase, underscores), a → FieldType, and optional constraints (`Required`, `Unique`, `Immutable`, `Sensitive`, `Searchable`, `MaxLen`, `Min`, `Max`, custom validators). Field names are embedded in migration column names and API payloads; they must not be renamed after first use.

**FieldDef**
The declaration struct for a single → Field. Fields: `Name`, `Type` (→ FieldType), `Label`, `Required`, `Unique`, `Immutable`, `Sensitive`, `Searchable`, optional type-specific configuration (e.g. `Options` for Select, `LinkTarget` for Link, `Series` for NamingSeries).

**FieldType**
An open string type identifying the kind of a → Field. Open string means it is not a closed enum; framework drivers register canonical types (`Data`, `SmallText`, `LongText`, `Int`, `Float`, `Currency`, `Bool`, `Date`, `DateTime`, `Time`, `Select`, `MultiSelect`, `NamingSeries`, `JSON`, `Link`, `LinkList`, `DynamicLink`). External drivers may register additional types. The compiler verifies that every FieldType used in a registered EntityDefinition has a registered handler. See: → BLOCK-002 resolution.

**Filter**
A declarative predicate tree used to specify which → Entity records a query or bulk operation targets. Composed using the → Filter DSL. Filters are tenant-scoped automatically; tenants cannot construct filters that escape their → RLS boundary. The Filter wire format is versioned; filters without a version field are rejected by the runtime. See: → LAW-014.

**Filter DSL**
The framework's declarative query language for specifying → Filters. Provides: `Eq`, `NotEq`, `Gt`, `Gte`, `Lt`, `Lte`, `In`, `NotIn`, `IsNull`, `IsNotNull`, `Contains`, `StartsWith`, `And`, `Or`, `Not`. DSL expressions compose into a predicate tree serialized to a versioned wire format. The DSL does not permit raw SQL predicates.

**Five-Layer Architecture**
The strict top-down dependency hierarchy of the Awo Framework: UI Layer → API Layer → Domain Layer → Workflow Layer → Store Layer. No layer may import from a layer above it. Violations of this rule are compile-time errors in a conforming build. See: → LAW-008.

**FROZEN** (Stability Class)
The highest stability class. Documents or interfaces marked FROZEN may not change their normative content. Breaking changes require a major version increment and formal deprecation. All → Architecture Laws and → Documentation Laws carry FROZEN stability. See: → Stability Class.

---

## G

**Global Table**
A PostgreSQL table that is not → tenant-scoped and therefore has no → RLS policy. Global tables are readable by the application role but not writable by application code during requests. Examples: `tenants`, `iam_audit_log`, `timezones`, `currencies`, `countries`, `paye_bands`, `platform_admins`.

**golang-migrate**
The migration tool used by Awo for all schema changes. Applies `.up.sql` and `.down.sql` file pairs in version-timestamp order. All schema changes must go through golang-migrate; direct DDL execution against the production database outside the migration runner is prohibited. See: → Migration.

---

## H

**Hook**
A named extension point in the → EntityRecord Lifecycle. Hooks execute at defined stages: `BeforeValidate`, `BeforeCreate` (before first persist), `AfterCreate` (inside TX), `BeforeUpdate`, `AfterUpdate` (inside TX), `BeforeDelete`, `AfterDelete` (inside TX), `BeforeSave` (before any write), `AfterSave` (inside TX). Hook execution order is deterministic: declaration order within a module, topological sort across modules. See: → HookRegistration, → LAW-007.

**HookPolicy**
A declaration on an → EntityDefinition specifying whether external → Modules may register → Hooks on that entity. When `HookPolicy` is `Open`, other modules may register hooks via → HookRegistration. When `HookPolicy` is `Closed` (default), only the owning module may register hooks. See: → LAW-017, → BLOCK-009 resolution.

**HookRegistration**
The struct used to attach a → Hook implementation to a specific entity's lifecycle stage. Contains: entity name, stage, priority, handler, and the registering module's identity. The framework maintains a deterministically ordered list of HookRegistrations per entity per stage. HookRegistration replaces the earlier `HookSet` struct design.

---

## I

**IAM Module**
The → Platform Module (`internal/platform/iam`) that manages users, roles, permissions, sessions, and authentication. Provides → RBAC via → Casbin, → Session management via Redis, and credential verification. The IAM module uses only → System Entities (Users and Sessions) because IAM data cannot tolerate JSONB corruption risk.

**Immutable** (Field Constraint)
A field constraint indicating the field value may be set on create but never changed on update. The framework rejects any → UpdateInput that includes an immutable field with a value different from the persisted value.

**Initialization Phase**
The startup stage during which → EntityDefinition registrations are accepted by the → Entity Registry. Begins when the process starts. Ends when `EntityRegistry.Compile()` is called, after which no further registrations are accepted. All `def.Register()` calls must occur during this phase, typically in `init()` functions. See: → LAW-003.

---

## J

**JSONB Storage**
PostgreSQL's binary JSON column type. Used as the storage mechanism for → Custom Entity records and → Custom Fields on → System Entities. JSONB columns are GIN-indexed automatically by the framework for all entities. JSONB storage does not provide typed column constraints or FK enforcement.

---

## L

**LAW** (Architecture Law)
See: → Architecture Law.

**Lifecycle Stage**
An open string type identifying a specific point in the → EntityRecord Lifecycle. Canonical stages: `before_validate`, `before_save`, `before_create`, `after_create`, `before_update`, `after_update`, `before_delete`, `after_delete`, `after_save`. External → Modules may register custom stages but must not conflict with canonical stage names. See: → BLOCK-003 resolution.

**Link** (Field Type)
A → Field type representing a typed foreign key reference to a specific → Entity type. Generates a FK column and index. The target entity type is specified in `FieldDef.LinkTarget`. Contrast with → DynamicLink for polymorphic references.

**LinkList** (Field Type)
A → Field type representing a list of foreign key references to a specific → Entity type. Stored as an array column.

---

## M

**Metadata Module**
The → Platform Module (`internal/platform/metadata`) that manages → Custom Field definitions. Allows tenant administrators to add fields to any entity at runtime without migration or redeployment. Custom field definitions are stored as → System Entities; the custom field values are stored in entity JSONB columns.

**Middleware Pipeline**
The fixed-order request processing chain applied to every inbound HTTP request. Order (immutable): Request ID injection → Structured logging → Panic recovery → CORS → Tenant resolution → `set_tenant_context()` → Session validation → Rate limiting. The order of this pipeline is a security guarantee and must not be made configurable at runtime. See: → Tenant Resolution.

**Migration**
A pair of SQL files (`.up.sql` and `.down.sql`) that represent an atomic schema change. Managed by → golang-migrate. Files are named with a 14-digit Unix timestamp followed by a description slug: `20241215143022_create_contact.up.sql`. Migrations are append-only; existing migration files must never be edited after they have been applied to any environment. See: → LAW-012.

**Module**
A cohesive unit of domain functionality that is registered with the framework via a → Module Manifest. A module owns one or more → Entities, their migrations, hooks, services, handlers, and workflows. Modules do not import from each other; cross-module interaction occurs only through the → Entity Registry and → Filter DSL. Two module categories exist: → Platform Module (unconditional) and business modules (optional, tenant-activated).

**Module Manifest**
A declaration struct that describes a → Module to the → Entity Registry. Contains: module identity (reverse-DNS name), version, declared → Capability Tokens provided and required, and the list of → EntityDefinition registrations. The registry uses manifests to detect dependency mismatches and version conflicts at → Initialization Phase.

**Module Registry**
The → Platform Module (`internal/platform/registry`) that tracks which business modules are installed and activated per tenant. Module activation state is stored as → System Entities. The module registry does not control code loading (all modules are compiled in); it controls feature availability per tenant.

---

## N

**NamingSeries** (Field Type)
A → Field type that generates sequential, human-readable identifiers. Format: `{PREFIX}-{YYYY}-{SEQ:N}` where `YYYY` is the current year and `SEQ:N` is a zero-padded sequence number with N digits. The sequence counter is atomic per tenant per series prefix. Prefix overrides are allowed per tenant when `TenantOverridable: true`.

**N+1 Query**
An anti-pattern where N additional queries are issued to load related data for N parent records. Prohibited in Awo. All related data must be loaded via → QueryOption at the time of the parent query, not in per-record loops afterward.

**NotFoundError**
A sentinel error returned when a requested → Entity record does not exist or is not visible to the requesting → Actor (due to → Privacy Policy filtering). HTTP status 404. Must be indistinguishable from a genuine missing-record response (to prevent enumeration of records the actor cannot see).

---

## O

**Outbox Pattern** (Transactional Outbox)
The mechanism by which → Workflow dispatch is made reliable. When an entity record is created or mutated, an outbox entry is written in the same PostgreSQL transaction as the entity record. After the transaction commits, a background relay process reads the outbox and dispatches → Workflow starts to Temporal. This guarantees that a committed entity record always has its associated workflow eventually started, even if the process crashes between commit and dispatch. See: → LAW-006.

---

## P

**Page Builder**
A Go function that constructs a → Page Schema for a specific entity view (list, create, edit, detail). Declared in `PageBuilderSet` on the → EntityDefinition. If not declared, the framework auto-generates a default page schema. Page builders are invoked at schema-serve time, not at request time; results are cached in Redis. Permission checks run at schema-serve time.

**Page Schema**
A JSON document conforming to the → amis schema format that fully describes a UI view. Page schemas are generated by → Page Builders, cached in Redis at `page:{entity}:{version}:{tenant}` with a 5-minute TTL, and served to the browser by the API layer. The browser renders the schema using the → amis SDK without custom JavaScript.

**PageBuilderSet**
The struct on an → EntityDefinition that declares optional → Page Builder overrides for each view type. Fields: `List`, `Create`, `Edit`, `Detail`. Unspecified views use framework-generated defaults.

**Permission**
An `(action, entity-type)` tuple that an → Actor must be granted (directly or via role inheritance) to perform an operation. Permissions are evaluated by → Casbin against the registered policy set. The → EntityDefinition declares which roles are granted each standard permission (`Create`, `Read`, `Write`, `Delete`) and any action-specific permissions. Permission checks occur in the → API Layer before any → Domain Layer code executes.

**PermissionError**
A sentinel error returned when an → Actor is not granted a required → Permission. HTTP status 403. Stack traces and internal details must not be included in the response; only the error code and a generic message.

**Platform Admin**
A privileged actor whose role (`role:platform-admin`) bypasses → Casbin entirely, granting full access across all tenants and all platform configuration. Platform admin credentials must never be issued to tenant-level users. Platform admin actions are recorded in the → Audit Log.

**Platform Module**
A → Module in `internal/platform/` that is unconditionally present in every Awo deployment. The seven platform modules are: Tenant, IAM, Feature Flags, Settings, Audit Log, Metadata, Module Registry. Platform modules use identical patterns to business modules (EntityDefinition, Register, PolicyFunc, HookDef, migrations). There is no special framework path for platform modules.

**Policy Function** (PolicyFunc)
A Go function declared on an → EntityDefinition that injects row-level → Filter predicates into every query for that entity, based on the current → Actor. PolicyFunctions enforce fine-grained, per-row access control beyond the operation-level gate provided by → RBAC. PolicyFunctions are invoked automatically by the framework; they cannot be bypassed by application code.

**Privacy Policy**
See: → Policy Function.

---

## Q

**QueryOption**
A modifier passed to `EntityRepository.Query()` to control query behavior. Options include: loading → Edges (explicitly, never lazy), field selection, sort order, pagination cursors. Edges must always be loaded via QueryOption, never via subsequent per-record calls.

---

## R

**RBAC** (Role-Based Access Control)
The operation-level permission system used by Awo. Implemented via → Casbin. RBAC gates whether an → Actor may perform an operation (create, read, write, delete, or a named action) on a given → Entity type. RBAC is not row-level; row-level filtering is handled by → Privacy Policies and → RLS.

**Redis**
The in-memory data store used for: → Session token storage, → Feature Flag evaluation cache, → Page Schema cache, and rate limit counters. Redis is not the source of truth for any persistent data — PostgreSQL is. Redis failure causes hard failure for session validation (correct security behavior). Redis failure degrades gracefully for feature flags and page schemas.

**Request ID**
A UUID injected at the first middleware stage of every request. Value comes from the `X-Request-ID` header if present, or is generated. Included in all structured log entries and error responses. Used for distributed tracing correlation.

**Response Envelope**
The standard JSON wrapper for all API responses. Success: `{"data": {...}, "meta": {...}}`. Error: `{"error": {"code": "...", "message": "...", "fields": {...}}}`. Internal stack traces must never appear in responses. See: → BusinessError, → ValidationError.

**RLS** (Row-Level Security)
PostgreSQL's built-in row filtering mechanism. Every tenant-scoped table in Awo has `FORCE ROW LEVEL SECURITY` and a policy that filters rows to the current tenant via `current_tenant_id()`. The application's → `set_tenant_context()` call sets the PostgreSQL transaction-local variable that RLS policies read. RLS is the authoritative tenant isolation enforcement point; application-layer filtering is supplementary. See: → LAW-015.

**Row-Level Security**
See: → RLS.

**RFC** (Request for Comments)
A document proposing a significant change to the framework's design, contracts, or governance. RFCs go through a formal review process before being accepted (becoming an → ADR) or rejected (archived). RFC authors must declare the motivation, proposed change, alternatives considered, and migration path.

**Runtime Phase**
The stage after → Compilation Phase during which the process serves requests. The → CompiledSchema is immutable during the runtime phase. No → EntityDefinition registrations are accepted. The Fiber HTTP server and Temporal worker run during this phase.

---

## S

**Saga Pattern**
A distributed transaction pattern used in → Workflows. Each → Activity registers a compensating action. If the workflow fails at step N, compensating actions for steps N-1 through 1 are executed in reverse order. The framework provides a `SagaCompensator` helper. Sagas ensure that distributed mutations either fully complete or are fully undone.

**Schema Divergence**
A condition where two instances of the same binary produce different → CompiledSchema content hashes. Schema divergence is a defect condition; it means the two instances are not serving equivalent behavior and cannot safely share a load balancer. See: → LAW-019, → BLOCK-010 resolution.

**SDUI** (Server-Driven UI)
An architectural pattern where the server generates complete UI descriptions (JSON schemas) rather than serving data for client-side rendering logic. In Awo, the → amis renderer interprets → Page Schemas produced by → Page Builders. SDUI eliminates the need for custom JavaScript for standard ERP views.

**Sensitive** (Field Constraint)
A field constraint indicating the field value must never appear in logs, error messages, or standard API responses unless the caller explicitly requests it. Applied automatically to fields such as passwords, tokens, API keys, and PII. The framework enforces exclusion from log output. See: → LAW-013.

**Session**
The server-side record of an authenticated → Actor's current login. Stored in Redis at `session:{token}`. Contains: user UUID, tenant UUID, role set, expiry timestamp. Session validation occurs in the → Middleware Pipeline on every request. Redis failure causes all session validation to fail (correct security behavior: cannot authenticate without session store).

**SessionStore**
The → Driver interface for session storage operations. The reference implementation uses Redis. Application code must not access Redis or any session store directly; all session operations go through the SessionStore interface.

**set_tenant_context()**
A PostgreSQL stored procedure that sets the transaction-local variable `app.current_tenant_id` to the current tenant's UUID. Called by `store.SetTenantContextFromCtx(ctx)` in the → Middleware Pipeline after tenant resolution. This procedure is the single RLS enforcement point. It validates that the tenant exists and has status `ACTIVE` before setting the variable. The `TRUE` flag makes the setting transaction-local, resetting automatically on COMMIT or ROLLBACK. Must be called before any tenant-scoped query executes. See: → LAW-005.

**Settings Module**
The → Platform Module (`internal/platform/settings`) that provides hierarchical configuration: system default → tenant override → branch override. Used for tenant-specific behavioral configuration that does not require a feature flag or migration.

**Store Layer**
The fifth (lowest) layer in Awo's → Five-Layer Architecture. Contains → Driver implementations and the → EntityRepository implementation. The store layer is the only layer permitted to interact with external data stores (PostgreSQL, Redis). It translates → Filter expressions and → QueryOptions into driver-specific operations.

**SystemViewer**
A constructed → ViewerContext used for operations that occur outside a user request: background jobs, migration scripts, workflow activities, administrative tooling. The SystemViewer has defined semantics: it carries a designated system tenant scope (for platform operations) or a specific tenant scope (for tenant-scoped background operations). SystemViewer operations are recorded in the → Audit Log with the system actor as the subject. See: → BLOCK-004 resolution.

**System Entity**
An → Entity whose records are stored in typed SQL columns. Used when: financial integrity is required, inventory accuracy is required, IAM data is involved, or write frequency exceeds hundreds per second. The following entities are mandatorily system entities: LedgerEntry, StockMove, Payment, User, Tenant, JournalEntry, TaxEntry. System entities always have a `custom_fields jsonb` column for → Custom Fields.

---

## T

**Task Queue** (Temporal)
A named queue that routes → Workflow and → Activity executions to a specific set of workers. Declared in → WorkflowTrigger. Task queue names use dot-notation: `{module}.{entity}.{event}`. Example: `finance.invoice.submit`.

**Temporal**
The durable workflow execution platform used by Awo's → Workflow Layer. Temporal provides: workflow durability across crashes, automatic replay from checkpoints, complete event history, and long-running workflow support (approval chains spanning days). All async, multi-step business processes are implemented as Temporal workflows. See: → Activity, → Workflow, → Saga Pattern.

**Tenant**
The top-level isolation unit in Awo. Every piece of data, every operation, and every user belongs to exactly one tenant. Tenant isolation is structural: the framework assumes one process serves many isolated tenants simultaneously. Tenants are identified by UUID and exist in the global `tenants` table (no RLS). See: → Tenant Status Machine, → RLS.

**Tenant Admin**
The system role (`role:tenant.admin`) that grants full access within one tenant. Tenant admins cannot access other tenants or platform configuration. Seeded at tenant bootstrap; cannot be deleted.

**Tenant Context**
See: → TenantContext.

**TenantContext**
The runtime representation of which tenant a request or operation targets. Resolved by the → Tenant Resolution middleware and attached to the request `context.Context`. All → EntityRepository operations and `set_tenant_context()` calls consume the TenantContext from context. Operations without a valid TenantContext are rejected by the framework. Contrast with → SystemViewer for non-request operations.

**Tenant Identification**
The process of determining which tenant an inbound request belongs to. Resolution order (priority): `X-Tenant-ID` header (UUID) → `tenant_id` query parameter (webhooks/legacy only; disable in production) → subdomain parsing (`bo.`, `portal.`, `app.`, `api.` prefixes). The first successful resolution wins. Failed resolution returns HTTP 400.

**Tenant Module**
The → Platform Module (`internal/platform/tenant`) that manages the → Tenant lifecycle state machine, tenant provisioning, and tenant-level configuration. The Tenant entity is a → System Entity.

**Tenant Resolution**
The fifth stage of the → Middleware Pipeline. Determines the current → TenantContext from the inbound request, then calls `set_tenant_context()` to enforce → RLS for the duration of the request's database operations.

**Tenant Status Machine**
The lifecycle state transitions for a → Tenant record. Valid transitions: `PENDING → ACTIVE`, `PENDING → ARCHIVED`, `ACTIVE → SUSPENDED`, `ACTIVE → ARCHIVED`, `SUSPENDED → ACTIVE` (payment resolved), `SUSPENDED → ARCHIVED` (grace period expired). `ARCHIVED` is a terminal state. HTTP responses by status: `PENDING → 503 + Retry-After: 60`, `SUSPENDED → 402`, `ARCHIVED → 410`.

**TriggerEvent**
An open string type identifying which → Lifecycle Stage fires a → WorkflowTrigger. Canonical values: `on_create`, `on_update`, `on_delete`, `on_submit`, `on_cancel`. External modules may define custom trigger events. See: → BLOCK-003 resolution.

---

## U

**UpdateInput**
The typed struct passed to `EntityRepository.Update()`. Carries field patches for an existing record. Only specified fields are updated; omitted fields retain their current values. → Immutable fields are rejected if present with a changed value.

**UUID v7**
The UUID version used for all → Entity record identifiers in Awo. UUID v7 is time-ordered (monotonically increasing within a millisecond), client-generatable, and suitable for use as a B-tree index primary key. UUID v4 must not be used for new record identifiers.

---

## V

**ValidationError**
A field-level input error. HTTP status 422. Contains a `Fields` map from field name to user-facing message. Used when input fails structural or business-rule validation before the record reaches the persistence layer. See: → Response Envelope.

**ViewerContext**
The abstract representation of the principal performing an operation. Either a resolved → Actor (from a user session) or a → SystemViewer (for background operations). All EntityRepository operations receive a ViewerContext. The ViewerContext is always tenant-scoped, either to a specific tenant or to the platform scope.

---

## W

**WithTx**
The `EntityRepository.WithTx(ctx, fn)` method. Executes `fn` within a database transaction. All EntityRepository operations inside `fn` share the same transaction. The transaction is committed when `fn` returns nil and rolled back on any non-nil return. → Workflows must not be started inside a `WithTx` block; Temporal dispatch occurs after the transaction commits.

**Workflow**
A durable, long-running process implemented using → Temporal. Workflow functions must be deterministic: they must not use `time.Now()`, `time.Sleep()`, `rand`, direct I/O, or standard goroutines. These operations are performed by → Activities. Workflow state persists across process crashes via Temporal's event sourcing model.

**Workflow ID**
The globally unique identifier for a specific → Workflow execution. Format: `{tenant-uuid}.{entity-type}.{record-id}.{event}.{workflow-function}`. Example: `abc123.finance_invoice.inv456.on_submit.InvoiceSubmissionWorkflow`. Embedded in Temporal's event history (retained for months) and in the entity record. See: → LAW-016.

**Workflow Layer**
The fourth layer in Awo's → Five-Layer Architecture. Contains → Temporal workflow functions, → Activity implementations, and saga compensators. The workflow layer may call down to the → Store Layer via Activity functions. It must not import from the → API Layer.

**WorkflowTrigger**
A declaration on an → EntityDefinition that binds a → TriggerEvent on the entity's lifecycle to the start of a → Workflow. Contains: the trigger event, the workflow function name, the → Task Queue name, and an input builder function. The framework starts the workflow outside the database transaction, after commit. Failure to start the workflow is recorded in a retry queue; it does not roll back the entity mutation. See: → LAW-006, → Outbox Pattern.

---

## Z

**Zero-Downtime Migration**
A migration strategy that avoids locking production tables. Required patterns: adding a nullable column before adding constraints; using `CREATE INDEX CONCURRENTLY` for all production index additions; using the add-dual-write-backfill-switch-drop sequence for renames. Single-step renames in production are prohibited. See: → Migration.

---

## Appendix: Term Index by Category

### Core Primitives
EntityDefinition, EntityRecord, EntityRepository, CompiledSchema, Entity Registry, FieldDef, FieldType, EdgeDef, HookRegistration, PolicyFunc, ActionDef, WorkflowTrigger, ModuleManifest

### Architecture
Five-Layer Architecture, Architecture Law, Architectural Invariant, Domain Layer, API Layer, Store Layer, Workflow Layer, SDUI, Driver

### Tenancy
Tenant, TenantContext, ViewerContext, SystemViewer, RLS, set_tenant_context(), Tenant Status Machine, Tenant Resolution, Tenant Identification

### Persistence
System Entity, Custom Entity, JSONB Storage, EntityRepository, Filter, Filter DSL, QueryOption, BulkCreate, BulkUpdate, Migration, Zero-Downtime Migration

### Fields and Types
FieldType, Currency, NamingSeries, Link, LinkList, DynamicLink, Custom Field, Sensitive, Immutable, Searchable

### Lifecycle
Lifecycle Stage, Hook, HookRegistration, HookPolicy, TriggerEvent, BeforeCreate Hook, BeforeValidate Hook, Outbox Pattern

### Workflows
Temporal, Workflow, Activity, Saga Pattern, Workflow ID, Task Queue, WorkflowTrigger

### SDUI
amis, amis SDK, SDUI, Page Schema, Page Builder, PageBuilderSet

### IAM and Security
RBAC, Casbin, Actor, Session, SessionStore, Permission, Policy Function, Privacy Policy, Platform Admin

### Infrastructure
Redis, PostgreSQL, PgBouncer, golang-migrate, Feature Flag, Request ID

### Errors
ValidationError, BusinessError, NotFoundError, PermissionError, Response Envelope

### Documentation
ADR, RFC, Architecture Law, Documentation Law, Stability Class, FROZEN, Document Category, Module Manifest

---

*This glossary is the single source of canonical term definitions for the Awo Framework. All other documents defer to definitions stated here.*
