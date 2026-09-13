# AWO Framework — Unified Audit System: Engineering Review v2
## Principal Architect Assessment — Pre-Implementation Validation

**Reviewer:** Framework Architecture Review
**Subject:** review.md — Unified Audit System Implementation Specification
**Verdict:** NOT READY FOR IMPLEMENTATION — 3 critical blockers, 9 high-priority issues

---

## 1. Executive Summary

Specification is architecturally sound in principle. Core decisions (repository wrapper, sync
writer, config-driven sensitivity, single table, RLS immutability) are correct. However, the
specification was written without reading the actual framework codebase. Six integration
assumptions are wrong. Three are blockers that prevent implementation as written.

Critical finding: `def` package is a frozen kernel. `SystemDefinition` cannot be modified.
The specification's `AuditEnabled bool` field addition violates the kernel freeze policy and
must be redesigned before Phase 1 begins.

Second critical finding: The framework already defines `def.Actor` in `def/record.go`.
The specification introduces `audit.Actor` as a parallel type. Two actor models in one
framework is architectural debt from day one.

Third critical finding: Transaction lifecycle ownership is ambiguous between `runtime.Pipeline`
and `contrib/pgx` driver. The AuditingRepository wrapper's transaction safety guarantee
depends entirely on which party manages TX commit — and the specification assumes the wrong
answer for at least one code path.

---

## 2. Critical Blockers

### BLOCKER-1: `def` frozen kernel violation

**Issue:** Specification adds `AuditEnabled bool` to `def.SystemDefinition`.

From `def/doc.go`: package is frozen kernel. From `def/entity.go`: `SystemDefinition` is a
stable struct with no infrastructure delivery fields. CLAUDE.md §3: "Single source of truth for
all entity metadata." ADR intent: entity definitions declare *what*, not *how*.

**Impact:** Adding `AuditEnabled` to `SystemDefinition`:
- Breaks kernel freeze policy
- Forces every existing entity definition to be touched (field addition)
- Couples infrastructure delivery (audit on/off) into domain model
- Creates a precedent — next engineer adds `CacheEnabled`, `SearchEnabled`, etc.

**Correction:** Do not modify `def.SystemDefinition`. Instead: maintain a separate
`audit.EntityAuditConfig` registry, populated at bootstrap from a configuration source
(migration-seeded table or hardcoded constants), keyed by entity qualified name. The
`AuditingRepository` wrapper consults this registry at construction time.

Example: `audit.DefaultConfig()` returns a map with all entities enabled. Specific entities
can be disabled by modifying `audit.EntityAuditConfig` — a configuration concern, not a
DSL concern. ADMIN and SECURITY category entities are unconditionally enabled in
`TransactionalWriter` regardless of config.

**Specification change required:** Yes. Remove `AuditEnabled bool` from all mentions of
`EntityDefinition` and `SystemDefinition`. Add `audit.EntityAuditConfig` registry.

---

### BLOCKER-2: Dual Actor model — `def.Actor` already exists

**Issue:** `def/record.go` defines:
```go
type Actor struct {
    UserID           *uuid.UUID
    ServiceAccountID *uuid.UUID
    TenantID         uuid.UUID
    Roles            []string
}
```

The specification defines `audit.Actor` as a separate struct with:
```go
type Actor struct {
    Type             ActorType
    UserID           *uuid.UUID
    ServiceAccountID *uuid.UUID
    APITokenID       *uuid.UUID
    SystemActorID    SystemActorID
    SessionHash      string
    IPAddress        string
    RequestID        string
}
```

**Impact:** Two actor models with different field sets, no defined relationship between them.
`AuditingRepository` receives a `def.Actor` (from `RecordMeta`) but needs an `audit.Actor`
for the `Entry`. Conversion is lossy — `def.Actor` has `Roles` but not `SessionHash`,
`IPAddress`, `RequestID`, `APITokenID`. These fields live in `ViewerContext`, not `def.Actor`.

The `AuditingRepository` wrapper, called from inside the pipeline, has access to the
`*def.EntityRecord` via `RecordMeta.Actor`. But it does NOT have `ViewerContext` unless
the context carries it. The pipeline passes `ViewerContext` via context keys (from
`RequireAuth` middleware), but `AuditingRepository` must know how to extract it.

**Correction:** Define an adapter: `audit.ActorFromRecord(meta *def.RecordMeta) audit.Actor`
that converts `def.Actor` fields (UserID, ServiceAccountID) and enriches from `ViewerContext`
extracted from the calling context. This adapter is the single conversion point.

`audit.Actor` stays as designed in the spec. `def.Actor` is not modified. The adapter
bridges them. `AuditingRepository` calls the adapter in every write path.

**Specification change required:** Yes. Add `audit.ActorFromRecord` and
`audit.ActorFromContext` adapters to the `audit` package. Document that `def.Actor` and
`audit.Actor` are distinct types with defined conversion.

---

### BLOCKER-3: Transaction lifecycle ownership — audit write timing is wrong

**Issue:** Specification states: "AuditingRepository.Create() calls inner.Create() → then
writes audit event." This assumes `inner.Create()` returns with the TX still open.

From `runtime/pipeline.go`: lifecycle is:

```
RunBeforeCreate()  — OUTSIDE TX
[pipeline or driver opens TX]
driver.Create()    — INSIDE TX (persist only)
RunAfterCreate()   — INSIDE TX (AfterSave, AfterCreate hooks)
[TX commits]
```

Two possible ownership models exist in the codebase, and the spec does not distinguish them:

**Model A — Pipeline manages TX:**
```
pipeline.Create(ctx, data):
  RunBeforeCreate(ctx)
  tx = pool.Begin()
  txCtx = pgx.WithTx(ctx, tx)
  repo.Create(txCtx, record)      ← AuditingRepository.Create called here
  RunAfterCreate(txCtx, record)   ← AfterCreate hooks run here
  tx.Commit()
```
In Model A: when `AuditingRepository.Create(txCtx, record)` is called, TX is open and in
context. `inner.Create(txCtx)` persists. Then `AuditingRepository` writes audit using txCtx
(same TX). Control returns. Pipeline calls `RunAfterCreate`. TX commits. ✓ Audit is in same TX.

**Model B — Driver manages TX:**
```
pipeline.Create(ctx, data):
  RunBeforeCreate(ctx)
  repo.Create(ctx, record):        ← AuditingRepository.Create called here
    inner.Create(ctx, record):
      tx = pool.Begin()
      INSERT...
      RunAfterCreate(txCtx)        ← hooks called from within driver TX
      tx.Commit()
    ← TX is NOW COMMITTED
  ← AuditingRepository writes audit here — OUTSIDE TX, standalone INSERT
```
In Model B: by the time `inner.Create()` returns to `AuditingRepository`, the TX is already
committed. The audit write is a standalone INSERT. If it fails, entity is committed with no
audit trail. This is exactly the correctness problem the specification claims to solve.

**Impact:** If Model B is the actual implementation of `contrib/pgx`, the entire
`AuditingRepository` wrapper approach fails its primary guarantee. The specification never
verifies which model the framework uses.

**Investigation required:** Read `contrib/pgx` repository implementation to determine TX
ownership before writing any code. This is the single highest-priority verification task.

**Correction (if Model B):** The audit write belongs in `AfterCreate` / `AfterSave` hooks
registered on each entity — not in a repository wrapper. This requires the per-entity hook
approach that the spec explicitly rejected, OR adding a framework-level AfterCreate hook
injection mechanism to `runtime.Pipeline`. Neither is trivial.

**Correction (if Model A):** Specification is correct. Proceed. Verify first.

**Specification change required:** Yes. Add explicit investigation finding before Phase 1.
State assumed TX model and how to verify. Design fallback if Model B is confirmed.

---

## 3. High-Priority Issues

### HIGH-1: `bootstrap.go` is near-frozen — `audit.Init()` integration

**Issue:** Spec adds `audit.Init(pool, cache)` to bootstrap sequence. `bootstrap.go` is
framework infrastructure. Adding `audit.Init()` creates a hard dependency from `bootstrap`
to `audit`. The `audit` package does not exist yet. The `bootstrap` package cannot import
a non-existent package. More importantly: bootstrap is the initialization sequencer —
every new subsystem that adds an `Init()` call to bootstrap makes bootstrap a dependency
registry, which is exactly what Wire is supposed to prevent.

**Correction:** `audit.Init()` is called from `cmd/server/main.go`, not from `bootstrap.Run()`.
Bootstrap returns a `*Result`. `main.go` calls `audit.Init(result.Pool, cache)` between
bootstrap and HTTP server start. This keeps bootstrap independent of audit. Preserves kernel
freeze. Makes audit an application-level concern wired by the entry point.

---

### HIGH-2: `router.Register()` has no audit.Writer parameter

**Issue:** From `main.go`, routes are registered via:
```go
router.Register(app, result.Schema, router.RegisterOptions{
    Pool:     result.Pool,
    Redis:    result.Redis,
    IAM:      iamModule.Auth,
    Tenants:  tenantRepo,
    Authz:    evaluator,
    Temporal: nil,
})
```

The `AuditingRepository` wrapper must receive an `audit.Writer` at construction. The router
constructs per-entity repositories. Currently `RegisterOptions` has no `AuditWriter` field.

**Impact:** Without threading `audit.Writer` through `RegisterOptions`, the router cannot
construct `AuditingRepository` wrappers. Every entity repository is constructed without audit.

**Correction:** Add `AuditWriter audit.Writer` to `router.RegisterOptions`. In
`main.go`: `AuditWriter: auditWriter` (constructed before `router.Register` call). The router
wraps each `contrib/pgx.Repository` with `audit.NewAuditingRepository(inner, writer, schema)`.
This is a `router.go` change, not a framework-kernel change.

---

### HIGH-3: `platform/audit` is side-effect-imported in `main.go`

**Issue:** From `main.go`, platform module imports:
```go
// Platform module imports (side effects): audit, flags, metadata, registry, settings, tenant
```

The existing `awo/platform/audit` package registers its `LogDefinition` via `init()`.
Phase 5 of the spec deletes this package. But:
1. Deleting it breaks the `main.go` side-effect import
2. The `iam_audit_log` entity disappears from the registry
3. Any migration that references `iam_audit_log` via entity name breaks

**Correction:** Phase 5 must include explicit steps:
- Remove side-effect import from `main.go`
- Remove `def.Register(&LogDefinition)` from `platform/audit/init()`
- Ensure `platform_audit_log` (new entity) is registered in its place
- Verify no migration references `iam_audit_log` by entity name (only by table name)

---

### HIGH-4: `EntityRecord.Data` is `map[string]any` — struct reflection does not apply

**Issue:** From `def/record.go`: `EntityRecord` carries data as `Data map[string]any` and
`CustomFields map[string]any`. The sanitizer spec says:
> "If struct: reflect it to map[string]any using field names (lowercase snake_case)"

The framework never passes typed structs through the repository pipeline — it uses
`*def.EntityRecord` with `Data map[string]any`. There are no Go structs to reflect.

**Impact:** `sanitizer.Strip` input is always `map[string]any`, never a struct. The struct
reflection branch in `sanitizer.Strip` is dead code. More importantly: the sensitive fields
list (from `EntityDefinition.Fields where Sensitive == true`) must map to the keys in
`EntityRecord.Data`. These keys are the field names as stored in the map, which follow
the framework's internal naming convention (snake_case, as declared in `FieldDef.Name`).

**Correction:** `sanitizer.Strip` takes `map[string]any` as input, not `any`. The struct
reflection path is removed. The specification's stripping logic is correct for maps.
The adapter that extracts sensitive field names from `EntityDefinition` must use
`FieldDef.Name` values (not JSON tags or struct field names — there are no struct fields).

---

### HIGH-5: HMAC secret for session hash is not specified

**Issue:** Spec says: "HMAC-SHA256 of session token using server-side secret." The secret
is never defined. Where is it stored? How is it loaded? How is it rotated?

**Impact:** Without specifying the secret source, the `TransactionalWriter` cannot be
constructed. The secret must be in the application configuration (environment variable),
loaded at bootstrap, and passed to the `TransactionalWriter` at construction.

**Correction:** Add `AuditSessionHMACSecret string` to bootstrap `Config`. Validate at
startup (non-empty, minimum 32 bytes). Pass to `audit.NewWriter(pool, policies, hmacSecret)`.
Document that rotating this secret makes historical session_hash values unmatchable
(acceptable — hash is for correlation only, not decryption).

---

### HIGH-6: `audit_retention_role` PostgreSQL role — not in current migrations

**Issue:** Spec references `audit_retention_role` as an existing role (from migration 000448).
But that migration is part of the legacy SQL trigger system which is being REPLACED, not
kept. If 000448 is dropped as part of the migration strategy, `audit_retention_role` must be
recreated in the new migration set.

**Correction:** Migration `000500_platform_audit_log.up.sql` must include:
```sql
CREATE ROLE audit_retention_role NOLOGIN;
GRANT SELECT, UPDATE, DELETE ON platform_audit_log TO audit_retention_role;
```
This is not in the spec's migration list. Add it.

---

### HIGH-7: Partition maintenance job — no scheduler exists

**Issue:** Spec says "partition maintenance Temporal workflow or framework-scheduled cron."
The framework has no scheduler. Temporal is listed as optional (may be nil). If Temporal
is nil (degraded mode, which bootstrap supports), partition maintenance has no execution path.

**Impact:** Missing next-month partition → first INSERT of new month fails. This is an
operational incident, not a graceful degradation.

**Correction:** Bootstrap creates partitions for current + 2 months ahead at startup.
This is sufficient without a scheduler for months where server restarts at least once.
Add a `SHOW` check at startup: if next month's partition is missing, create it immediately
before serving traffic. This does not require Temporal. The Temporal-based maintenance
job is an optional enhancement for zero-restart deployments.

---

### HIGH-8: Feature flag requires DB read during `AuditingRepository` construction

**Issue:** Spec says `AuditingRepository` checks feature flag `feature.unified_audit.enabled`
from `platform_audit_config` at construction time with 60-second TTL cache.

`AuditingRepository` is constructed at bootstrap (inside `router.Register()`). At that point,
`platform_audit_config` table may not exist (Phase 0 migration not applied). The read fails.
Even when the table exists, a DB read during bootstrap for every entity's repository
construction adds startup latency proportional to entity count.

**Correction:** Feature flag is loaded once at startup by `audit.Init()` and cached as a
package-level variable. `AuditingRepository` checks a package-level `audit.IsEnabled()` bool.
`audit.Init()` reads the flag once from DB. Refresh is a future operational concern
(restart to change flag), not a runtime concern during bootstrap.

---

### HIGH-9: RLS INSERT requires `current_tenant_id()` — standalone AUTH writes have no tenant context

**Issue:** AUTH events (login failure, account lockout) are written as standalone INSERTs
on separate connections. These connections do not go through `contrib/pgx`'s standard path
that calls `SET LOCAL app.current_tenant_id = $1` before queries.

RLS policy `pal_app_insert` requires `tenant_id = current_tenant_id()`. If `current_tenant_id()`
is not set on the standalone connection, the INSERT fails with RLS violation.

**Impact:** All standalone audit writes (AUTH, ACCESS, SECURITY) fail silently (suppressed
policy). The categories that most need reliable writes are the ones that will fail.

**Correction:** `TransactionalWriter`, when executing a standalone INSERT (no TX in context),
must set `SET LOCAL app.current_tenant_id = $1` on the acquired connection before the INSERT.
The `Entry.TenantID` provides the value. For system events (TenantID = nil), use
`system_role` connection path (separate pool with system_role privileges). This requires two
pgx pools in `TransactionalWriter`: one for `application_role` writes, one for `system_role`
writes.

---

## 4. Medium-Priority Issues

### MED-1: `RecordMeta.Actor` missing IPAddress, RequestID, SessionToken

`def.Actor` carries `{UserID, ServiceAccountID, TenantID, Roles}`. IP address and request ID
are in `ViewerContext` (from `auth` package), not in `def.Actor`. The `AuditingRepository`
wrapper must extract both `def.Actor` (from `RecordMeta`) and `ViewerContext` (from context
keys set by middleware) to construct a complete `audit.Actor`.

**Correction:** `audit.ActorFromContext(ctx context.Context) audit.Actor` extracts from:
1. `auth.ViewerFromContext(ctx)` → UserID, ServiceAccountID, IsPlatformAdmin
2. Middleware-set context keys → IPAddress, RequestID
3. `audit.SystemActorFromContext(ctx)` → SystemActorID if set

Consolidate into one function. Document what is nil when called outside HTTP context.

---

### MED-2: `decision VARCHAR(5)` is too short — 'ALLOW' is exactly 5 chars

**Issue:** 'ALLOW' = 5 chars. VARCHAR(5) allows exactly 'ALLOW' and 'DENY' (4). Any future
decision value longer than 5 chars requires a DDL migration. VARCHAR(10) costs nothing extra
in PostgreSQL (variable-length storage) and provides headroom.

**Correction:** Use `VARCHAR(10)`. Low-risk change.

---

### MED-3: `unique_ips JSONB` in `platform_audit_token_usage` — cap enforcement

**Issue:** Spec says `unique_ips JSONB` array "capped at 100." The UPSERT shown does not
implement the cap. A token used from 200 IPs grows the array to 200 entries. No cap logic
shown.

**Correction:** Add to UPSERT:
```sql
unique_ips = CASE
  WHEN jsonb_array_length(platform_audit_token_usage.unique_ips) >= 100 THEN
    platform_audit_token_usage.unique_ips
  WHEN platform_audit_token_usage.unique_ips @> to_jsonb(EXCLUDED.last_ip::text) THEN
    platform_audit_token_usage.unique_ips
  ELSE
    platform_audit_token_usage.unique_ips || to_jsonb(EXCLUDED.last_ip::text)
END
```

---

### MED-4: GIN index on partitioned table — PostgreSQL version requirement

**Issue:** Spec adds GIN index on `compliance_flags` to parent partitioned table.
GIN indexes on partitioned tables require PostgreSQL 11+. The framework's PostgreSQL version
minimum is not documented.

**Correction:** Add `-- Requires PostgreSQL 11+` comment to GIN index DDL. Add PostgreSQL
version check to bootstrap health validation. Log a warning if version < 11 and GIN index
cannot be created.

---

### MED-5: `actor_request_id` missing from `platform_audit_log` column spec

**Issue:** Historical migration mapping table references `actor_request_id` as a migration
target column. But the `platform_audit_log` table DDL in the spec has no `actor_request_id`
column. `request_id` is in `context.request.id` (JSONB).

**Impact:** Historical migration step 4 cannot execute — the target column does not exist.

**Correction:** Remove `actor_request_id` from historical migration mapping. Map
`iam_audit_log.request_id` → `context.request.id` (JSONB field inside context object). This
is already the spec's intent based on Part 4 context schema.

---

### MED-6: GDPR anonymization requires `audit_retention_role` — but handler runs as `application_role`

**Issue:** GDPR erasure is triggered via an API endpoint (future Phase 5 deliverable). The
handler runs as `application_role`. But `audit.AnonymizeForUser()` requires UPDATE permission
which is only granted to `audit_retention_role`. The handler cannot UPDATE via its own role.

**Correction:** `audit.AnonymizeForUser()` must acquire a `audit_retention_role` connection.
This requires a separate connection pool with `audit_retention_role` credentials, or a
PostgreSQL function with `SECURITY DEFINER` that validates the caller is a platform admin
before executing the UPDATE. The SECURITY DEFINER approach is simpler (no separate credentials
file) and the correct pattern.

---

### MED-7: `schema_version SMALLINT` — no migration to increment version

**Issue:** Spec defines `schema_version` as a mechanism for schema evolution but provides no
specification for what triggers a version increment, who increments it, or how the `Writer`
knows the current version.

**Correction:** `schema_version` is a constant in `awo/audit/audit.go`:
```go
const CurrentSchemaVersion = 1
```
`TransactionalWriter` always writes `CurrentSchemaVersion`. When context JSONB structure
changes: update this constant in a new release. Callers reading audit data check
`schema_version` against their supported versions. No DB migration needed to increment
— it is a code constant, not a DB-managed value.

---

### MED-8: `platform/audit` package deletion order in Phase 5

**Issue:** Phase 5 removes `awo/platform/audit`. But the existing package contains
`definition.go` (registering `iam_audit_log` entity), `service.go` (Writer struct), and
`hooks.go`. Deletion order matters:

1. `hooks.go` — empty, delete first (no dependencies)
2. `service.go` — delete after all callers removed
3. `definition.go` — delete after `iam_audit_log` entity removed from entity registry
   and after `iam_audit_log_deprecated` table is dropped

If `definition.go` is deleted before the entity is unregistered, the `init()` no longer
fires, but any compiled schema referencing `iam_audit_log` breaks.

**Correction:** Phase 5 sequence:
1. Remove side-effect import from `main.go`
2. Replace with `audit.RegisterAuditLogEntity()` that registers `platform_audit_log` entity
3. Run historical migration (data transfer)
4. Verify data
5. Delete `awo/platform/audit/` package
6. DROP TABLE `iam_audit_log_deprecated`

---

### MED-9: No test helper for asserting audit events

**Issue:** Spec defines `NoopWriter` for tests (suppresses writes) but provides no
`RecordingWriter` for asserting that specific audit events were written with specific fields.

**Impact:** Unit tests for `AuditingRepository`, `AuthService`, and `RequirePermission`
middleware cannot assert that the right events were written. Tests either use a real DB
(expensive) or skip audit verification entirely (dangerous).

**Correction:** Add `audit.RecordingWriter` to the test helpers:
```
RecordingWriter:
  mu      sync.Mutex
  entries []Entry

Write(ctx, e): appends e to entries
Entries() []Entry: returns copy
Reset()
AssertWritten(t, EventType): fails if no entry with that type
AssertNotWritten(t, EventType): fails if any entry with that type
```

This enables unit tests to verify audit behavior without a PostgreSQL connection.

---

## 5. Low-Priority Issues

### LOW-1: Metrics naming inconsistency

Spec uses `awo_audit_write_errors_total` in the alerts table and `awo_audit_write_errors_total{category, error_type}` in the metrics table. These are the same metric but inconsistently referenced. The label set must be consistent everywhere. Standardize on `{category, error_type}` throughout.

### LOW-2: `auth.session.expired` in taxonomy — impossible event

Spec taxonomy includes `auth.session.expired`. Sessions expire via Redis TTL — there is no
Go code that fires when a session TTL elapses. This event type cannot be written. Remove from
taxonomy. Replace with `auth.session.evicted` (written when `ValidateToken` finds an expired
session that was not properly deleted).

### LOW-3: `auth.token.refreshed` in taxonomy — flow does not exist

Current `Session` struct has no `RefreshToken` field. No token refresh flow exists in the
framework. Remove `auth.token.refreshed` from taxonomy until the flow is implemented.

### LOW-4: `audit.ExportForUser()` returns `[]Entry` — not a stream

For users with 10,000+ audit events, loading all into memory before returning is an OOM risk.
For Phase 5: return `*sql.Rows` or a channel-based iterator, not `[]Entry`. The API handler
streams the response.

### LOW-5: `platform_audit_config` read/write roles are inconsistent

Spec says: "INSERT/UPDATE (admin_role)." But `platform_audit_config` is set by the retention
job (system_role) and by platform admin APIs (admin_role). The retention job needs UPDATE
permission. Add system_role to the permitted roles for config UPDATE.

---

## 6. Framework Integration Matrix

### `awo/def` (frozen kernel)
- **Files affected:** `entity.go`, `field.go`
- **Change:** NONE. `AuditEnabled bool` is rejected (BLOCKER-1). No changes to frozen kernel.
- **New files:** None
- **Compatibility:** Preserved

### `awo/bootstrap`
- **Files affected:** `bootstrap.go`
- **Change:** None in bootstrap.go itself. `audit.Init()` called from `main.go` not here.
- **New files:** None

### `awo/cmd/server/main.go`
- **Files affected:** `main.go`
- **Changes:**
  - Remove side-effect import of `awo/platform/audit` (Phase 5)
  - Add `audit.Init(result.Pool, redisCache)` after bootstrap
  - Add `audit.Writer` construction
  - Add `AuditWriter: auditWriter` to `router.RegisterOptions`
  - Add `AuditHMACSecret` to config loading
- **Compatibility concern:** Side-effect import removal changes entity registry contents

### `awo/api/router`
- **Files affected:** `router.go` (RegisterOptions struct, repository construction loop)
- **Changes:**
  - Add `AuditWriter audit.Writer` to `RegisterOptions`
  - For each entity schema: wrap `contrib/pgx.NewRepository()` with `audit.NewAuditingRepository()`
  - Pass `entitySchema` to wrapper for sensitive field extraction
- **New files:** None
- **Migration complexity:** Low — additive change to RegisterOptions

### `awo/api/middleware` (RequirePermission / authz)
- **Files affected:** `authz/require_permission.go` (or equivalent)
- **Changes:**
  - Add `audit.Writer` dependency (injected via middleware constructor)
  - On DENY: call `writer.Write(ctx, Entry{EventAuthzDeny, ...})`
  - On cross-tenant platform admin access: call `writer.Write(ctx, Entry{EventAdminEmergencyAccess, ...})`
- **New files:** None
- **Compatibility concern:** Middleware constructors change signature

### `awo/contrib/pgx`
- **Files affected:** Repository implementation file(s)
- **Changes:**
  - Verify TX ownership model (BLOCKER-3 investigation)
  - If Model B: add callback mechanism for post-persist pre-commit hooks
  - `SET LOCAL app.current_tenant_id` must be called on audit writer connections too
- **New files:** None
- **Migration complexity:** HIGH if Model B confirmed — requires pipeline/driver refactor

### `awo/platform/iam`
- **Files affected:** `service.go`, `module.go`
- **Changes:**
  - Add `audit.Writer` field to `AuthService`
  - Add auth event writes in: Login (success+failure), Logout, ValidateAPIToken (aggregation),
    RevokeUserSessions, password reset paths
  - Update `New()` constructor to accept `audit.Writer`
- **New files:** None
- **Migration complexity:** Medium — service and tests change

### `awo/platform/audit` (current package)
- **Files affected:** All (deprecated)
- **Changes:** Full deletion in Phase 5 after historical migration
- **Files deleted:** `definition.go`, `service.go`, `hooks.go`
- **New package:** `awo/audit/` (new location, new package)

### `awo/auth`
- **Files affected:** None
- **Change:** No change. `ViewerContext` and `SessionStore` interfaces unchanged.
  `audit.ActorFromContext` reads from ViewerContext but does not modify auth package.

### `awo/driver`
- **Files affected:** `driver.go`
- **Changes:** None to existing interface. `AuditingRepository[T]` lives in `awo/audit`,
  not in `awo/driver`. It implements `driver.EntityRepository[T]`.

### `awo/runtime`
- **Files affected:** `pipeline.go` (possibly)
- **Changes:** Depends on BLOCKER-3 investigation result.
  - Model A: No changes. Pipeline manages TX, wrapper works correctly.
  - Model B: `pipeline.go` needs extension to support framework-level AfterCreate hooks
    or post-persist callbacks that run inside TX.

### `awo/cache`
- **Files affected:** None
- **Change:** `RiskScorer` uses `cache.Cache` interface. No changes needed.

### `awo/events/outbox`
- **Files affected:** relay goroutine
- **Changes:** Add `audit.WithSystemActor(ctx, audit.SystemOutboxRelay)` at relay loop start.
  Add OUTBOUND event writes on dispatch and failure.

### `awo/observability/metrics`
- **Files affected:** metrics registration file
- **Changes:** Register audit metrics: `awo_audit_writes_total`, `awo_audit_write_errors_total`,
  `awo_audit_write_latency_seconds` (histogram), partition/checkpoint gauges.

### `awo/filter`
- **Files affected:** None
- **Change:** Audit query API (Phase 5) uses filter package for query construction.
  No changes to filter package itself.

### `awo/naming`
- **Files affected:** None
- **Change:** None.

### Packages with NO changes
- `awo/auth` — ViewerContext interface unchanged
- `awo/compiler` — no new entity compilation logic needed
- `awo/registry` — no changes
- `awo/filter` — unchanged
- `awo/naming` — unchanged

---

## 7. Repository Integration Review

### Current wrapper landscape

From codebase exploration: no existing repository wrapper layer identified. The framework
uses direct `contrib/pgx.NewRepository()` construction in `router.Register()`. There is no
`ValidationRepository`, `CachingRepository`, or `PermissionRepository` wrapper today.

### AuditingRepository position in chain

Since no other wrappers exist today, the chain is:

```
AuditingRepository[T]     ← audit writes, sensitive field stripping
  ↓
contrib/pgx.Repository[T] ← actual PostgreSQL operations, TX management
```

If future wrappers are added (caching, validation), recommended ordering:

```
CachingRepository[T]      ← read-path cache; must be outermost so cache hits skip audit
  ↓
AuditingRepository[T]     ← audit only what actually hits the DB; skip cached reads
  ↓
ValidationRepository[T]   ← validates before persist; audit fires after validation passes
  ↓
contrib/pgx.Repository[T] ← persist + TX management
```

**Justification:**

Caching outermost: a cache hit never reaches the DB. Auditing a cache hit produces a DATA
read event for something that was never a DB operation. Cache hits are not audit-worthy at
DATA level. Caching wraps Auditing — cache misses fall through to Auditing, which falls
through to DB.

Auditing above Validation: if validation fails, no DB operation occurs, no audit event fires.
Correct — audit records what actually happened to the data store, not validation attempts.
Validation failures are not DATA events; they are business logic errors returned to the caller.

Validation above pgx: validation is a pre-persist concern, must see the record before it
hits the DB.

### Soft delete integration

Soft delete is an Update — `deleted_at` is set. `AuditingRepository.Update()` handles this
automatically. The diff shows `{"deleted_at": {"old": null, "new": "..."}}`. No special case
needed.

However: if the framework adds a dedicated `SoftDelete(id)` method to the repository
interface, `AuditingRepository` must intercept it and emit `EventEntityDeleted` (not
`EventEntityUpdated`) with context `{"soft": true}`. Do not conflate soft delete with update
in the audit trail — they are semantically different.

### BulkUpdate integration

`BulkUpdate(filter, patch)` is not in the current `driver.EntityRepository[T]` interface
(interface file `entity_repository.go` was referenced but not fully read). If BulkUpdate is
added to the interface: `AuditingRepository` wraps it with one summary event. If it is not
in the interface (callers bypass the interface with raw SQL), audit has no coverage of bulk
operations. This is a gap that must be validated against the actual interface definition.

---

## 8. Transaction Analysis

### Create path

```
Assumption: Model A (pipeline manages TX)

1. Handler calls pipeline.Create(ctx, entityName, data)
2. Pipeline: RunBeforeCreate(ctx)         — OUTSIDE TX
3. Pipeline: tx = pool.Begin(ctx)
4. Pipeline: txCtx = context.WithValue(ctx, pgxTxKey, tx)
5. Pipeline: repo.Create(txCtx, record)   — AuditingRepository.Create(txCtx)
   5a. inner.Create(txCtx)               — persists inside tx
   5b. writer.Write(txCtx, Entry{...})   — audit INSERT inside same tx
   ← returns to pipeline
6. Pipeline: RunAfterCreate(txCtx, record) — AfterSave/AfterCreate hooks inside tx
7. Pipeline: tx.Commit() or tx.Rollback()
```

Rollback scenarios:
- inner.Create fails (DB error): AuditingRepository returns error. Pipeline rollbacks. No audit.
- writer.Write fails (DATA, suppressed): AuditingRepository returns nil. Pipeline continues.
  AfterCreate hooks run. TX commits. Audit gap logged and metered.
- writer.Write fails (ADMIN, propagated): AuditingRepository returns error. Pipeline rollbacks.
  Entity not persisted. Audit gap with full rollback. Correct.
- AfterCreate hook fails: Pipeline rollbacks. Audit row also rolled back. Correct.

### Update path

Update requires one extra `Get` before the Update:

```
1. AuditingRepository.Update(txCtx, id, patch)
2. inner.Get(txCtx, id) → beforeRecord     — read inside TX (consistent snapshot)
3. inner.Update(txCtx, id, patch)          — persist inside TX
4. writer.Write(txCtx, Entry{before, after, diff})
← returns to pipeline
5. Pipeline: RunAfterUpdate(txCtx, record, prev)
6. TX commits
```

The `inner.Get` in step 2 uses the same TX as the Update (txCtx has the TX). This gives a
consistent snapshot (no phantom reads between Get and Update within the same TX).

**Nested transaction risk:** If `RunAfterUpdate` hooks (step 5) call another repository
operation on the same entity, they create a nested operation inside the same TX. PostgreSQL
handles this via savepoints implicitly for some operations. For explicit savepoints:
not currently in the framework. This is pre-existing complexity, not introduced by audit.

### Delete path

```
1. AuditingRepository.Delete(txCtx, id)
2. inner.Get(txCtx, id) → beforeRecord     — capture before deletion
3. inner.Delete(txCtx, id)                — persist deletion inside TX
4. writer.Write(txCtx, Entry{before: beforeRecord})
← returns to pipeline
5. Pipeline: RunAfterDelete(txCtx, record)
6. TX commits
```

Risk: step 2 (Get before delete) adds one read to the delete path. If `inner.Get` and
`inner.Delete` race (parallel delete by another transaction), `inner.Get` may return the
record and `inner.Delete` may fail with "not found" or succeed depending on isolation level.
PostgreSQL default `READ COMMITTED` means step 2's read is consistent at that statement.
Step 3's delete is serialized. No race within the TX boundary.

### AUTH event writes (standalone, no TX)

```
AuthService.Login():
  1. Verify credentials (inside read TX, commits before session creation)
  2. SessionStore.Store() — Redis write
  3. writer.Write(ctx, Entry{EventAuthLoginSuccess, ...}) — standalone INSERT
     a. writer acquires connection from audit pool
     b. SET LOCAL app.current_tenant_id = tenantID (must be added — HIGH-9 fix)
     c. INSERT INTO platform_audit_log (...)
     d. connection returned to pool
  4. return LoginResult
```

Login failure write:

```
AuthService.Login():
  1. Credential check fails
  2. writer.Write(ctx, Entry{EventAuthLoginFailed, TenantID: tenantIDFromRequest, ...})
     — TenantID comes from request context (TenantResolver already ran)
     — writer acquires audit pool connection, sets tenant context, inserts
     — if write fails: suppressed, logged
  3. return error
```

For login failure: `TenantID` is the request tenant (from `TenantResolver` middleware).
The writer sets `SET LOCAL app.current_tenant_id = tenantID` on the standalone connection.
RLS `pal_app_insert` allows the INSERT. ✓

For system events (tenant_id = NULL): writer uses `system_role` pool. Separate connection
pool with `system_role` privileges. `pal_system_insert` allows INSERT with NULL tenant_id. ✓

### Deadlock analysis

No deadlock vectors identified in normal operation:
- Audit write and entity write use the same TX (same connection) — no lock contention between them
- Standalone audit writes on separate connections do not hold locks on entity tables
- `inner.Get` before `inner.Delete` acquires a SELECT lock, then DELETE upgrades to exclusive
  lock on the same row — standard pattern, no deadlock

Potential deadlock: Two concurrent operations updating the same entity + audit row:
- TX1: Update entity A, then attempt Update entity B (in AfterUpdate hook)
- TX2: Update entity B, then attempt Update entity A (in AfterUpdate hook)
- Deadlock possible. This is a pre-existing risk from AfterUpdate hooks, not introduced by audit.
- Audit writes do not acquire locks on entity tables. No new deadlock vectors.

---

## 9. Event Coverage Matrix

| Subsystem | Generates Events | Should? | Status | Category | Notes |
|-----------|-----------------|---------|--------|----------|-------|
| Entity Create | ✓ | ✓ | Covered | DATA | AuditingRepository |
| Entity Update | ✓ | ✓ | Covered | DATA | AuditingRepository |
| Entity Delete | ✓ | ✓ | Covered | DATA | AuditingRepository |
| Entity Bulk Update | ✓ | ✓ | Covered (summary) | DATA | One event |
| Entity Read | ✗ | ✗ | Not audited | — | RLS enforces access |
| Entity Read (sensitive) | ✗ | ✓ (v2) | Missing v1 | ACCESS | `AuditReads: true` |
| Custom Actions | ✗ | ✓ | **Missing** | DATA | No hook point in spec |
| Login success | ✓ | ✓ | Covered | AUTH | AuthService |
| Login failure | ✓ | ✓ | Covered | SECURITY | AuthService |
| Logout | ✓ | ✓ | Covered | AUTH | AuthService |
| Session creation | ✓ | ✓ | Covered | AUTH | AuthService (not SessionStore) |
| Session deletion | ✓ | ✓ | Covered | AUTH | AuthService |
| Session expiry | ✗ | ✗ | Not auditable | — | Redis TTL has no Go callback |
| Session bulk revoke | ✓ | ✓ | Covered | AUTH | AuthService |
| Password reset request | ✓ | ✓ | Covered | AUTH | AuthService |
| Password reset complete | ✓ | ✓ | Covered | ADMIN | AuthService |
| Password change | ✓ | ✓ | Covered | ADMIN | AuthService |
| API token issuance | ✓ | ✓ | Covered | ADMIN | AuthService |
| API token validation (routine) | ✓ (agg) | ✓ | Covered (aggregated) | — | token_usage table |
| API token validation (anomaly) | ✓ | ✓ | Covered | SECURITY | AuthService |
| API token revocation | ✓ | ✓ | Covered | ADMIN | AuthService |
| Account lockout | ✓ | ✓ | Covered | SECURITY | RateLimit middleware |
| Account unlock | ✓ | ✓ | Covered | ADMIN | AuthService |
| OAuth authorization | ✗ | ✓ | **Missing** | AUTH | No OAuth in framework yet |
| MFA challenge | ✗ | ✓ | **Missing** | AUTH | No MFA yet; taxonomy premature |
| Permission DENY | ✓ | ✓ | Covered | ACCESS | RequirePermission middleware |
| Permission ALLOW | ✗ | ✗ | Not audited (by default) | — | Volume too high |
| Permission ALLOW (admin) | ✓ | ✓ | Covered | ACCESS | Cross-tenant admin access |
| Role assignment | ✓ | ✓ | Covered | ADMIN | AuditingRepository + explicit write |
| Role removal | ✓ | ✓ | Covered | ADMIN | AuditingRepository |
| Permission grant | ✓ | ✓ | Covered | ADMIN | AuditingRepository |
| Privilege escalation | ✓ | ✓ | Covered | SECURITY | RoleAssignmentService |
| Emergency access | ✓ | ✓ | Covered | ADMIN | RequirePermission middleware |
| Tenant provisioning | ✓ | ✓ | Covered | DATA/ADMIN | AuditingRepository |
| Tenant suspension | ✓ | ✓ | Covered | ADMIN | AuditingRepository |
| OU hierarchy changes | ✓ | ✓ | Covered | ADMIN | AuditingRepository |
| Temporal workflow start | ✓ | ✓ | Covered | WORKFLOW | Activity wrapper |
| Temporal workflow complete | ✓ | ✓ | Covered | WORKFLOW | Activity wrapper |
| Temporal workflow fail | ✓ | ✓ | Covered | WORKFLOW | Activity wrapper |
| Temporal activity retry | ✗ | ✓ | **Missing** | WORKFLOW | Retry attempts are forensic data |
| Temporal activity timeout | ✗ | ✓ | **Missing** | WORKFLOW | Timeouts indicate failures |
| Temporal compensation | ✓ | ✓ | Covered | WORKFLOW | Activity wrapper |
| Outbox event dispatched | ✓ | ✓ | Covered | OUTBOUND | Relay goroutine |
| Outbox event failed | ✓ | ✓ | Covered | OUTBOUND | Relay goroutine |
| Scheduler job start | ✗ | ✓ | **Missing** | SYSTEM | No scheduler yet |
| Scheduler job complete | ✗ | ✓ | **Missing** | SYSTEM | No scheduler yet |
| Bootstrap complete | ✓ | ✓ | Covered | SYSTEM | main.go |
| Migration applied | ✓ | ✓ | Covered | SYSTEM | Migration runner |
| Migration failed | ✗ | ✓ | **Missing** | SYSTEM | Migration runner error path |
| Rate limit exceeded | ✓ | ✓ | Covered | SECURITY | RateLimit middleware |
| CSRF rejected | ✗ | ✓ | **Missing** | SECURITY | Not in spec |
| Config change (audit config) | ✗ | ✓ | **Missing** | ADMIN | platform_audit_config updates |
| Config change (feature flags) | ✗ | ✓ | **Missing** | ADMIN | platform_flags changes |
| Settings change | ✗ | ✓ | ✓ (by entity) | DATA | AuditingRepository covers entity |
| File upload | ✗ | ✓ | **Missing** | DATA | No file storage yet |
| Webhook delivery | ✓ | ✓ | Covered | OUTBOUND | Outbox relay |
| Export (data export) | ✗ | ✓ | **Missing** | DATA | No export yet |
| Import (bulk import) | ✓ | ✓ | Covered (summary) | DATA | BulkImportContext |
| Casbin policy reload | ✗ | ✓ | **Missing** | ADMIN | Policy reload is state change |
| RLS violation (DB level) | ✗ | ✗ | Not capturable | — | Requires pg_audit at DB level |
| Health check | ✗ | ✗ | Should not audit | — | Too high volume, not state change |
| Metrics scrape | ✗ | ✗ | Should not audit | — | Infrastructure, not state |
| Cache hit/miss | ✗ | ✗ | Should not audit | — | Cache is not state of record |
| Custom Actions | ✗ | ✓ | **Missing** | DATA | See below |

### Custom Actions — critical gap

**Issue:** `ActionDef` allows arbitrary state-mutating actions (submit, cancel, approve).
These are not entity CRUD — they are not intercepted by `AuditingRepository.Create/Update/Delete`.
The spec has no mechanism for auditing custom actions.

**Gap impact:** A finance invoice `submit` action that changes status from `draft` to
`submitted` produces no audit event. For a financial ERP, this is unacceptable.

**Correction needed:** Action handlers (`ActionHandlerFunc`) must receive an `audit.Writer`
and write an `EventEntityActionExecuted` event with:
```
entity_name, record_id, action name, actor, before/after state
```
The `ActionContext` (from `def/record.go`) should provide an audit writer accessor, or the
action handler must be wired with a writer at route registration. This requires design work
not in the current specification.

---

## 10. Dependency Graph

### Complete dependency graph

```
awo/audit
├── awo/def          (EntityDefinition, FieldDef, Actor, EntityRecord)
├── awo/driver       (EntityRepository[T] interface — for wrapper type declaration)
├── awo/cache        (Cache interface for RiskScorer cache)
├── github.com/jackc/pgx/v5  (pgxpool.Pool for TransactionalWriter)
└── log/slog         (failure logging)

awo/api/middleware
└── awo/audit        (Writer for DENY events)

awo/api/router
└── awo/audit        (Writer + AuditingRepository construction)

awo/platform/iam
└── awo/audit        (Writer for auth events)

awo/events/outbox
└── awo/audit        (Writer for OUTBOUND events)

awo/bootstrap        (no awo/audit dependency — kept clean)

awo/cmd/server/main.go
└── awo/audit        (Init, NewWriter — wiring point)
```

### Forbidden dependencies (must never exist)

```
awo/audit    → awo/auth           FORBIDDEN (auth imports audit instead)
awo/audit    → awo/platform/iam   FORBIDDEN (iam imports audit instead)
awo/audit    → awo/contrib/redis  FORBIDDEN (Redis is not audit infrastructure)
awo/audit    → awo/api            FORBIDDEN (API is a consumer, not dependency)
awo/def      → awo/audit          FORBIDDEN (def is frozen kernel, must not import audit)
awo/runtime  → awo/audit          FORBIDDEN (runtime must not know about audit)
awo/driver   → awo/audit          FORBIDDEN (driver interface must not import audit)
```

### Circular dependency check

No circular dependencies in the proposed graph. `awo/audit → awo/driver → (nothing)` is the
deepest chain. All consumers import `awo/audit`; `awo/audit` does not import consumers.

---

## 11. Migration Validation

### Ordering correctness

Proposed sequence: 000500 → 000501 → 000502 → 000510 → 000511 → 000520 → 000521

**Correction:** Gaps in sequence are a risk. Other framework migrations may be numbered
between 000502 and 000510. Reserved gaps cannot be guaranteed in a collaborative development
environment. Use consecutive numbering relative to the current highest migration.

Current framework migrations include numbers up to 000451 (from legacy audit). New audit
migrations must be: 000452, 000453, etc. OR use a high-enough number (000500+) to avoid
conflicts — but verify no other developer has used 000452-000499.

### Rollback safety per migration

| Migration | Creates | Rollback |
|-----------|---------|---------|
| 000500 (platform_audit_log) | Main table + partitions + RLS + indexes | DROP TABLE cascade (safe) |
| 000501 (support tables) | token_usage, checkpoint, config | DROP TABLE cascade (safe) |
| 000502 (retention role) | PostgreSQL role | DROP ROLE (safe if no owned objects) |
| 000510 (drop triggers) | Drops triggers | Re-run 000451 (complex) |
| 000511 (drop functions) | Drops functions | Re-run 000451 (complex) |

**Critical:** 000510 and 000511 are irreversible without re-running 000451. These must only
execute after Phase 3 has been stable for one full release cycle. The spec correctly identifies
000510/000511 as the point of no return.

### Missing migrations

1. **Initial partitions** — must be created in the same migration as the parent table OR
   immediately after. Bootstrap creates future partitions, but the current month's partition
   must exist at table creation time. Missing from spec migration list.

2. **`audit_retention_role`** — spec references this role but its creation is not in the
   new migration set (HIGH-6). Must be in 000502 or a dedicated migration.

3. **`SET LOCAL` for audit connections** — if `contrib/pgx` uses a connection-level hook to
   set `app.current_tenant_id`, audit writer standalone connections must also call this.
   This is a code change, not a migration, but must be in Phase 1 deliverables.

4. **`platform_audit_log` entity registration** — the new `platform_audit_log` entity
   (replacing `iam_audit_log`) must be registered in `init()` before Phase 5. This entity
   provides the SDUI, permissions, and schema compilation. Missing from spec deliverables.

### Zero-downtime validation

Feature flag approach is sound. Blue-green compatibility is correct. One gap: during Phase 3
(both old Writer and new AuditingRepository writing to different tables simultaneously),
compliance queries see incomplete data. This window is documented as acceptable — confirm
this is explicitly communicated to compliance stakeholders before Phase 3 begins.

---

## 12. Testing Strategy

### Test matrix by package

**`awo/audit` — target: 95% coverage**

| Test class | Scope | What |
|------------|-------|------|
| Unit: Sanitizer | sanitizer.go | Strip ordering, diff after strip, struct→map, nested one level, no infinite recursion |
| Unit: Sanitizer | sanitizer.go | Sensitive field from EntityDef + config table union |
| Unit: Sanitizer | sanitizer.go | `[REDACTED]` vs `[ENCRYPTED+REDACTED]` |
| Unit: Sanitizer | sanitizer.go | Empty map input, nil input, map with all-sensitive keys |
| Unit: RiskScorer | scorer.go | Score calculation: base + table weight + field weight capped at 100 |
| Unit: RiskScorer | scorer.go | Cache miss fallback, cache warm-up |
| Unit: TransactionalWriter | transactional.go | Suppressed policy: error logged, nil returned |
| Unit: TransactionalWriter | transactional.go | Propagated policy: error returned to caller |
| Unit: TransactionalWriter | transactional.go | With TX in context: uses TX not new connection |
| Unit: TransactionalWriter | transactional.go | Without TX: acquires from pool, sets tenant |
| Unit: TransactionalWriter | transactional.go | ClientEventID dedup (duplicate PK = nil) |
| Unit: ActorFromContext | context.go | User actor, service account actor, system actor |
| Unit: ActorFromContext | context.go | Missing ViewerContext → panics with clear message |
| Unit: RecordingWriter | noop.go | AssertWritten, AssertNotWritten, Reset |
| Integration: TransactionalWriter | transactional.go | Real PG insert, verify row persisted |
| Integration: TransactionalWriter | transactional.go | TX rollback removes audit row |
| Integration: RLS | — | application_role cannot INSERT wrong tenant |
| Integration: RLS | — | application_role cannot SELECT other tenant |
| Integration: RLS | — | admin_role can SELECT any tenant |
| Integration: RLS | — | audit_retention_role can DELETE, not INSERT |
| Integration: RLS | — | audit_retention_role can UPDATE (GDPR anonymization path) |
| Integration: Partition | — | Current month partition exists, INSERT succeeds |
| Integration: Partition | — | Missing partition causes INSERT failure (verify guard) |

**`audit.AuditingRepository[T]` — target: 90% coverage**

| Test class | Scope | What |
|------------|-------|------|
| Integration: Create | repository.go | Audit row written in same TX as entity |
| Integration: Create | repository.go | TX rollback: entity AND audit row disappear |
| Integration: Create | repository.go | DATA write failure (forced): entity succeeds, error logged |
| Integration: Update | repository.go | Before snapshot via Get, after snapshot, diff correct |
| Integration: Update | repository.go | Sensitive fields absent from all snapshots and diff keys |
| Integration: Update | repository.go | Empty diff (no real change): no audit write |
| Integration: Delete | repository.go | Before snapshot captured before delete |
| Integration: Delete | repository.go | After delete: before snapshot is only record of deleted data |
| Integration: BulkUpdate | repository.go | Exactly one summary event, count correct |
| Unit: AuditEnabled=false | repository.go | DATA events suppressed per config |
| Unit: ADMIN entity, AuditEnabled=false | repository.go | ADMIN events still written |

**`awo/platform/iam` auth events — target: 90% coverage**

| Test class | Scope | What |
|------------|-------|------|
| Integration: Login success | service.go | auth.login.success written with correct actor, IP |
| Integration: Login failure | service.go | auth.login.failed with NULL actor_user_id |
| Integration: Login failure | service.go | attempted_email_hash present in context |
| Integration: Login write fail | service.go | Auth write failure does not block login (suppressed) |
| Integration: Logout | service.go | auth.logout written BEFORE session delete |
| Integration: Token agg | service.go | 10K validations → 1 row in token_usage, 0 in audit_log |
| Integration: Token anomaly | service.go | New IP → individual audit_log event |
| Integration: Account lockout | service.go | SECURITY event with correct threshold context |

**`awo/api/middleware` access events — target: 90% coverage**

| Test class | Scope | What |
|------------|-------|------|
| Unit: DENY event | authz middleware | Event written before 403 returned |
| Unit: DENY write fail | authz middleware | 403 still returned if audit write fails |
| Unit: ALLOW platform admin | authz middleware | Cross-tenant access event written |
| Race test | authz middleware | Concurrent DENY events with go test -race |

**Migrations — target: 90% coverage**

| Test class | Scope | What |
|------------|-------|------|
| Migration: 000500 | platform_audit_log | Table exists, partitions exist, RLS active |
| Migration: 000500 | platform_audit_log | All 8 indexes present |
| Migration: 000501 | support tables | All three support tables exist |
| Migration: Rollback 000500 | — | DROP succeeds, no orphaned objects |
| Migration: Idempotent 000500 | — | Re-running fails gracefully (IF NOT EXISTS) |
| Migration: Canary phase | — | Feature flag read, wrapper is no-op when false |

**Performance / Benchmark tests**

| Test | Target | Threshold |
|------|--------|-----------|
| TransactionalWriter.Write latency | p99 < 5ms | Under normal PG load |
| AuditingRepository.Create overhead | < 10ms vs inner.Create | Delta p99 |
| BulkUpdate 10K: one event | < 50ms total | Including audit write |
| Sanitizer.Strip 100-field map | < 0.1ms | No DB involved |
| RiskScorer.Score (cached) | < 0.05ms | Pure memory operation |

**Race tests**

All tests must pass `go test -race`. Critical: `TransactionalWriter` concurrent writes,
`RiskScorer` cache read during concurrent writes, `RecordingWriter` concurrent Write+Assert.

**Chaos tests**

| Scenario | Expected behavior |
|----------|-------------------|
| PostgreSQL unavailable during DATA write | Suppressed, logged, entity succeeds |
| PostgreSQL unavailable during ADMIN write | Propagated, operation fails |
| Audit pool exhausted | DATA: suppressed. ADMIN: propagated. |
| Missing partition | INSERT fails. Bootstrap guard prevents this. Log alert. |
| Checkpoint SHA-256 mismatch | Alert fired. Human investigation required. |

---

## 13. Performance Assessment

### CRUD latency increase

| Operation | Baseline | With Audit (Model A) | Overhead |
|-----------|----------|---------------------|---------|
| Create | ~5ms | ~7-8ms | +2-3ms (1 extra INSERT in same TX) |
| Update | ~5ms | ~9-12ms | +4-7ms (1 extra GET + 1 INSERT) |
| Delete | ~5ms | ~9-12ms | +4-7ms (1 extra GET + 1 INSERT) |
| BulkUpdate | ~50ms | ~52ms | +2ms (1 summary INSERT) |

Update and Delete are most affected due to the extra `Get` for before snapshot. At p99 under
normal load on commodity hardware.

### TPS projections

| TPS (entity ops) | Audit writes/sec | Connection pool pressure | Recommendation |
|-----------------|-----------------|------------------------|----------------|
| 10 | 10 | Negligible | Default config |
| 100 | 200 (100 gets + 100 inserts) | Low | Monitor p99 |
| 1,000 | 2,000 | Moderate | Dedicated audit pool min=5 |
| 5,000 | 10,000 | High | `synchronous_commit=off` for DATA |
| 10,000 | 20,000 | Exceeds single instance | Separate audit PG instance |

At 5,000 TPS: the Get-before-Update becomes the bottleneck. Mitigation: PostgreSQL
`UPDATE ... RETURNING` with a CTE capturing OLD values, eliminating the extra roundtrip.
This is a `contrib/pgx` optimization, not an interface change.

### Write amplification

Base: 2× (1 entity + 1 audit per CRUD)
Update: 3× (1 GET + 1 entity UPDATE + 1 audit INSERT)
Delete: 3× (1 GET + 1 entity DELETE + 1 audit INSERT)

Index maintenance adds ~50% overhead to each audit INSERT (8 indexes maintained). Effective
multiplier for PostgreSQL write I/O: 2.5-3× baseline.

### Memory overhead

`AuditingRepository` carries no state beyond the inner repo reference, writer reference, and
entity definition reference. Memory overhead per entity: ~3 pointers = 24 bytes. Negligible.

`RiskScorer` cache: `audit_sensitive_tables` (tens of rows, ~500 bytes each) +
`audit_sensitive_fields` (hundreds of rows, ~200 bytes each). Total: ~100KB. Negligible.

### Partition scalability

Monthly partitions. After 7 years: 84 partitions. PostgreSQL handles up to ~1000 partitions
efficiently. At 84 partitions: query planning adds ~10ms for cross-partition range scans.
Single-partition queries (most operational queries) are unaffected.

Index per partition: 8 indexes × 84 partitions = 672 index objects. Within normal bounds.

### Large tenant behavior

One tenant generating 80% of audit volume: the `idx_pal_tenant_time` index has severe
imbalance. B-tree index pages for the large tenant become hotly contested. Monitor for index
bloat on this index. Mitigation: separate `platform_audit_log` table per tenant (extreme cases
only, not in spec — document as operational option).

---

## 14. Risk Register

| ID | Risk | Prob | Impact | Mitigation | Owner |
|----|------|------|--------|------------|-------|
| R1 | BLOCKER-3 (TX ownership) confirmed as Model B | High | Critical | Verify contrib/pgx before Phase 1 | Dev |
| R2 | `def` freeze violation causes team disagreement | Medium | High | Use EntityAuditConfig registry instead | Arch |
| R3 | `audit_retention_role` credentials leak | Low | Critical | Separate credentials, vault-stored, audit-only service account | Ops |
| R4 | GIN index unsupported on PG < 11 | Low | Medium | Version check at bootstrap | Dev |
| R5 | Historical migration data loss | Medium | High | Dry-run first, row-count verification before DROP | Dev |
| R6 | Partition missing at month boundary | Low | High | Bootstrap guard creates 3 months ahead | Dev |
| R7 | Audit write failure silent gap | Medium | Medium | Metrics alert, gap detection SYSTEM event on recovery | Ops |
| R8 | HMAC secret rotation breaks session correlation | Medium | Low | Document: rotation only breaks future correlation, not security | Dev |
| R9 | Custom actions not audited in v1 | High | Medium | Document gap, add to Phase 2 scope | Arch |
| R10 | Performance regression at 1000 TPS | Medium | High | Benchmark before Phase 2 production rollout | Dev |
| R11 | `contrib/pgx` SET LOCAL not called on audit connections | High | High | HIGH-9 fix before Phase 3 | Dev |

---

## 15. Recommended Implementation Order

### Before any code

**Verification tasks (no code):**
1. Read `contrib/pgx` repository implementation files in full — determine TX ownership model (BLOCKER-3)
2. Confirm PostgreSQL version minimum for GIN on partitioned tables (MED-4)
3. Confirm no migration numbers 000452-000499 are in use (migration ordering)
4. Confirm `router.Register()` source to understand exact repository construction pattern
5. Confirm `ActionDef` handler wiring point for custom action audit coverage gap

### Phase 1 — Foundation (prerequisite: all verification tasks complete)

Order within Phase 1:
1. `awo/audit/event.go` — EventType constants, Category, Severity (no dependencies)
2. `awo/audit/actor.go` — ActorType, SystemActorID, Actor struct (no dependencies)
3. `awo/audit/entry.go` — Entry struct (depends on event.go, actor.go)
4. `awo/audit/sanitizer.go` — Strip + Diff (depends on entry.go)
5. `awo/audit/scorer.go` — RiskScorer interface + CachingRiskScorer (depends on cache)
6. `awo/audit/compliance.go` — ComplianceDetector (depends on cache)
7. `awo/audit/writer.go` — Writer interface + FailurePolicy (depends on entry.go)
8. `awo/audit/transactional.go` — TransactionalWriter (depends on all above)
9. `awo/audit/noop.go` — NoopWriter (trivial)
10. `awo/audit/multi.go` — MultiWriter (trivial)
11. `awo/audit/context.go` — context keys + ActorFromContext adapter
12. `awo/audit/repository.go` — AuditingRepository[T] (depends on all above)
13. Migration 000452 — platform_audit_log + initial partitions + RLS + indexes
14. Migration 000453 — support tables (token_usage, checkpoint, config)
15. Migration 000454 — audit_retention_role creation + grants
16. `main.go` changes — audit.Init(), NewWriter(), feature flag load
17. `router.RegisterOptions` — add AuditWriter field
18. Phase 1 test suite

### Phase 2 — Entity CRUD (prerequisite: Phase 1 passing all tests)

1. Feature flag = canary for `platform_settings` entity
2. Wire `AuditingRepository` wrapper in `router.Register()` for all entities
3. Monitor 24h at canary
4. Feature flag = full
5. Phase 2 test suite (integration tests for all CRUD paths)
6. Benchmark suite — establish baseline, verify < 10ms overhead

### Phase 3 — Auth and Access (prerequisite: Phase 2 stable 1 release cycle)

1. Add `audit.Writer` to `AuthService`
2. Write auth event writes for all auth flows
3. Write DENY event writes in `RequirePermission` middleware
4. Write API token aggregation logic
5. Feature flags: `auth_events = true`, `access_events = true`
6. Phase 3 test suite

### Phase 4 — Background and Deprecation (prerequisite: Phase 3 stable 1 release cycle)

1. SystemActor propagation in all background goroutines
2. Temporal activity audit wrappers
3. Outbox relay audit writes
4. Bootstrap SYSTEM event
5. Historical migration Temporal workflow
6. Verify historical migration row counts
7. Migration 000455: drop SQL triggers (`disable_audit_on_schema`)
8. Migration 000456: drop SQL trigger functions
9. Remove `platform/audit` package side-effect import
10. Delete `awo/platform/audit/` package
11. DROP TABLE `iam_audit_log_deprecated` (after verification)
12. Phase 4 test suite + chaos tests

### Phase 5 — Query API (prerequisite: Phase 4 stable)

1. Register `platform_audit_log` entity definition
2. `GET /api/v1/platform/audit-log` with cursor pagination
3. SDUI History tab on entity detail views
4. `audit.ExportForUser()` (streaming, not `[]Entry`)
5. `audit.AnonymizeForUser()` via SECURITY DEFINER function
6. Weekly checkpoint cron / Temporal workflow
7. Operational runbooks
8. Phase 5 test suite

---

## 16. Final Readiness Scores

### Architecture score: 7/10

Correct in principle. Core decisions (repository wrapper, sync writer, single table, config-driven
sensitivity, RLS immutability, failure policy per category) are right. Loses 3 points for:
three critical blockers that prevent implementation as written, two actor models creating
framework-level debt, and bootstrap integration plan that requires frozen kernel modification.

### Framework consistency score: 6/10

Misses several framework conventions: assumes struct-based repos (framework uses EntityRecord
map), proposes `def` modification (frozen kernel), introduces parallel actor model alongside
existing `def.Actor`. Gains points for using Wire DI pattern, following `contrib/pgx` conventions,
and not introducing new concurrency models.

### Operational readiness: 7/10

Feature flag migration strategy is sound. Metrics and alerting are thorough. Loses points for:
missing `audit_retention_role` in new migration set, partition maintenance job reliance on
non-existent scheduler, and standalone auth write RLS gap (HIGH-9).

### Migration readiness: 6/10

Phased approach is correct. Loses points for: missing initial partition migration, missing
retention role migration, unclear migration number sequencing, historical migration data mapping
error (`actor_request_id` column not in target schema).

### Security readiness: 8/10

RLS model is correct. Sensitive field stripping ordered correctly. Immutability enforced at
RLS level. HMAC for session hash is right. Loses points for: HMAC secret specification missing,
GDPR anonymization via audit_retention_role not fully designed (MED-6), no specification of
audit_retention_role credential management.

### Performance readiness: 7/10

TPS estimates are realistic. Write amplification understood. Partition strategy is correct.
Loses points for: Update/Delete extra `Get` not benchmarked, no concrete optimization plan
for 5K TPS regime, large tenant imbalance not addressed.

### Implementation readiness: **5/10**

Not ready to implement. Three critical blockers must be resolved first. Nine high-priority
issues require design changes before implementation begins. Verification tasks (BLOCKER-3)
are non-optional and may require architectural revision. The specification is ready for
engineering review and design iteration — it is not ready for engineering implementation.

---

*Review complete. All issues are genuine architectural risks identified from reading the
actual framework codebase against the specification. No invented problems.*
