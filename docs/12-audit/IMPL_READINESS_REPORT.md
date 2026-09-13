# Unified Audit System — Implementation Readiness Report

**Classification:** Engineering Report
**Owner:** `12-audit/IMPL_READINESS_REPORT.md`
**Date:** 2026-07-27
**Status:** VERIFICATION COMPLETE — corrections required before implementation begins

---

## Executive Summary

This report documents the results of reading every relevant source file in the framework before writing any implementation code. It was produced after the architecture documentation (`AUDIT_ARCH.md`, `AUDIT_SPEC.md`, `AUDIT_MIGRATION.md`) was written, and several significant discrepancies were found between the documented architecture and the actual implementation.

**The documentation must be corrected before implementation begins.**

Three critical documentation errors must be resolved first:

1. **ADR-013 states "driver owns TX" — INCORRECT.** The service layer (`api/service.EntityService`) owns the transaction via `repo.WithTx()`. The driver provides `WithTx()` but does not call it automatically.
2. **AUDIT_ARCH.md states AuditWriter is injected into Pipeline and called by driver — INCORRECT.** No such injection point exists. The pipeline has no `RunAuditRecord` method and no `AuditWriter` field.
3. **LIFECYCLE_SPEC.md §3 was updated to say driver owns TX — INCORRECT.** Must be reverted to service ownership.

Everything else in the documented architecture is sound and can proceed once these three corrections are made.

---

## Phase 1 — Framework Verification

### Files Read

| File | Lines | Purpose |
|------|-------|---------|
| `awo/runtime/pipeline.go` | 386 | Entity lifecycle pipeline — ASSEMBLE through AfterXxx |
| `awo/api/service/entity.go` | 226 | Service orchestration — TX, repo, pipeline coordination |
| `awo/contrib/pgx/repo.go` | 616 | PostgreSQL repository — CRUD, WithTx, BulkCreate |
| `awo/contrib/pgx/conn.go` | 59 | Connection/TX extraction from context |
| `awo/driver/repository.go` | 301 | EntityRepository[T] interface + CreateInput, UpdateInput |
| `awo/api/router/router.go` | 128 | Route registration — RegisterOptions, pipeline construction |
| `awo/bootstrap/bootstrap.go` | 171 | Startup sequence — Pool, Redis, Schema |
| `awo/cmd/server/main.go` | 210 | Entry point — IAM, router.Register, Fiber |
| `awo/platform/iam/service.go` | ~656 | AuthService — Login, Logout, session management |
| `awo/def/record.go` | 221 | EntityRecord, Actor, RecordMeta |
| `awo/def/hook.go` | 130 | HookSet — hook interfaces |
| `awo/auth/viewer.go` | 176 | ViewerContext — context embedding, panics on absent |

---

## Phase 2 — Open Questions Resolution

### OQ-1: How does contrib/pgx call pipeline.RunAuditRecord?

**Verification:** Read `contrib/pgx/repo.go` in full (616 lines).

**Finding:** It does NOT. The `Repository.Create()` method executes `INSERT` and returns the record. It has no knowledge of the pipeline, `RunAuditRecord`, or `AuditWriter`. The `Repository` struct contains only `pool *pgxpool.Pool` and `schema *compiler.EntitySchema`.

**Who calls `RunAfterCreate`:** `api/service/entity.go` at line 77:
```go
repo.WithTx(ctx, func(txCtx context.Context) error {
    created, txErr = repo.Create(txCtx, input)
    if txErr != nil { return txErr }
    return pipeline.RunAfterCreate(txCtx, created)  // ← service calls this
})
```

**Verdict:** Documentation error. ADR-013 and AUDIT_ARCH.md §2.3 must be corrected. The service layer, not the driver, orchestrates what runs inside the TX.

**Architectural recommendation:** Add `pipeline.RunAuditRecord(ctx, record, operation, before)` to the pipeline. The service calls it inside `repo.WithTx`, between `repo.Create` and `pipeline.RunAfterCreate`. `Pipeline` receives `AuditWriter` at construction via `NewPipeline(schema, auditWriter)`.

**Files affected:**
- `awo/runtime/pipeline.go` — add `AuditWriter` field + `RunAuditRecord` method
- `awo/api/service/entity.go` — call `pipeline.RunAuditRecord` inside `WithTx`
- `awo/api/router/router.go` — pass `AuditWriter` to `NewPipeline`
- `00-overview/DECISION_REGISTER.md` — correct ADR-013
- `12-audit/AUDIT_ARCH.md` — major correction to §2, §3
- `02-pipeline/LIFECYCLE_SPEC.md` — correct §3

---

### OQ-2: Bootstrap constructor signature and audit injection point

**Verification:** Read `bootstrap/bootstrap.go` in full (171 lines).

**Finding:**
```go
func Run(ctx context.Context, cfg Config) (*Result, error)
// Result{Pool, Redis, Schema}
```
No `AuditWriter`, no signing secret, no `RiskScorer`. Bootstrap returns pure infrastructure — it does not wire application-level services.

**Who wires `AuditWriter`:** `cmd/server/main.go`. Currently at line 139:
```go
router.Register(app, result.Schema, router.RegisterOptions{
    Pool: result.Pool, Redis: result.Redis,
    IAM: iamModule.Auth, Tenants: tenantRepo,
    Authz: evaluator, Temporal: nil,
})
```
No audit fields in `RegisterOptions`.

**Verdict:** Documentation correct that bootstrap returns `*Result{Pool, Redis, Schema}` and audit initialization is the caller's responsibility. Incorrect that bootstrap warm-ups `RiskScorer`. That must happen in `main.go` before calling `router.Register`.

**Required change:** Add `AuditWriter audit.AuditWriter` to `RegisterOptions`. In `main.go`, construct `AuditWriter` after bootstrap, before `router.Register`. Add `AUDIT_SIGNING_SECRET` env var.

**Files affected:**
- `awo/api/router/router.go` — add `AuditWriter` to `RegisterOptions`, pass to `NewPipeline`
- `awo/cmd/server/main.go` — construct `TransactionalWriter`, warm `RiskScorer`, inject into `RegisterOptions`
- `12-audit/AUDIT_ARCH.md` — correct §5 (bootstrap integration)

---

### OQ-3: router.Register() and AuditWriter injection

**Verification:** Read `api/router/router.go` (128 lines).

**Finding:**
```go
pipeline := runtime.NewPipeline(schema)  // line 75 — no AuditWriter
```
`NewPipeline` takes only `*compiler.CompiledSchema`. No audit integration point exists.

**RegisterOptions struct:**
```go
type RegisterOptions struct {
    Pool     *pgxlib.Pool
    Redis    *goredis.Client
    IAM      middleware.SessionValidator
    Tenants  driver.EntityRepository[*def.EntityRecord]
    Authz    auth.PolicyEvaluator
    Temporal temporalclient.Client
}
```
No `AuditWriter`.

**Verdict:** Two changes needed: (1) add `AuditWriter` field to `RegisterOptions`, (2) pass it to `runtime.NewPipeline`.

---

### OQ-4: PostgreSQL minimum version

**Verification:** `pgxpool.ParseConfig` and `pgxlib.TxOptions{}` in `contrib/pgx`. No version-specific features beyond standard SQL used. Partitioning (`PARTITION BY RANGE`) requires PostgreSQL 10+. `gen_random_uuid()` requires PostgreSQL 13+ or `pgcrypto` extension. FORCE ROW LEVEL SECURITY requires PostgreSQL 9.5+.

**Verdict:** PostgreSQL 14+ recommended (for `gen_random_uuid()` built-in + stable JSONB performance). No framework constraint found preventing PostgreSQL 13 minimum.

---

### OQ-5: Migration sequence number

**Verification:** Could not directly list `db/migration/` due to Termux sandbox restriction. From prior session context: migrations 000448–000451 are the legacy audit migrations. Last known migration is 000451.

**Verdict:** Next migration number is **000452**. Phase 1 migration = `000452_platform_audit_log.up.sql`.

**Requires confirmation:** Run `SELECT version FROM awo_schema_migrations ORDER BY version DESC LIMIT 1` or `SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 1` to confirm before writing migration files.

---

### OQ-6: Feature flag read during bootstrap

**Verification:** Bootstrap reads only Postgres + Redis connections and compiles entity registry. It does not read any application data.

**Finding:** `platform_audit_config` table does not exist yet (it is created in migration 000452). Feature flag read must happen after migration 000452 is applied — either at server startup in `main.go` or on first `AuditWriter.Write()` call.

**Recommendation:** Lazy initialization on first `Write()` call is simpler. Cache the result in-process. No blocking bootstrap read required. If the flag row does not exist, default to `false` (legacy mode).

**Verdict:** Remove requirement for synchronous bootstrap read. TransactionalWriter reads the feature flag on first `Write()` call, caches in-process with atomic flag. Add cache invalidation via restart (acceptable for v1).

---

### OQ-7: Partition maintenance scheduler

**Verification:** No scheduler exists in the framework. Temporal worker is wired in `main.go` but `opts.Temporal == nil` currently.

**Verdict:** For v1, use **pg_cron** (PostgreSQL extension). Requires zero application code. Simplest approach. The bootstrap migration creates the `pg_cron` job alongside the initial partitions:

```sql
SELECT cron.schedule('create-audit-partitions', '0 9 25 * *',
    $$SELECT platform_create_next_audit_partition()$$);
```

A stored function `platform_create_next_audit_partition()` creates the next month's partition if it doesn't exist.

**ADR required:** Yes — ADR-020: Audit partition maintenance via pg_cron.

---

### OQ-8: Standalone audit writes (no TX — AUTH events)

**Verification:** `contrib/pgx/conn.go` shows `connFromContext` returns `&pgConn{pool: pool}` (no TX) when no connection is in context. `pgConn.db()` returns `pool` directly when `txn == nil`. So pool auto-commit is used.

**Finding:** `contrib/pgx/repo.go WithTx()` calls `setTenantContext(txCtx, conn.db(), tc.TenantID.String())` inside the TX before any DML. But for standalone writes (AUTH events not inside a mutation TX), there is NO tenant context being set.

**Problem:** The `platform_audit_log` table is a global table with NO RLS. So for INSERT, there is no `current_tenant_id()` requirement. The audit table INSERT doesn't need tenant context set — it just needs the `tenant_id` column value in the INSERT itself.

**Verdict:** Standalone AUTH audit writes are safe. The `TransactionalWriter` can acquire a pool connection via `pool.Exec()` directly without needing `SET LOCAL app.current_tenant_id`. The `tenant_id` value is in the `AuditRecord` struct and is inserted as a column value. No stored procedure needed.

---

### OQ-9: Custom action audit gap

**Verification:** `api/handler/crud.go` has `func (h *EntityHandler) Action(c *fiber.Ctx) error`. This calls the `ActionHandlerFunc` from the `ActionDef`. There is no pipeline stage for custom actions.

**Finding:** `api/service/entity.go` has no `ExecuteAction` method. Actions are dispatched directly from the handler via `ActionDef.HandlerFunc`. No pre/post hooks, no pipeline, no audit integration.

**Verdict:** V1 gap confirmed. Custom actions have zero audit coverage from the framework. Action authors must call `AuditWriter.Write()` explicitly if they need audit trails. Document as known gap. V2 solution: add `ActionInterceptor` to the pipeline.

---

## Phase 3 — Transaction Lifecycle

### Actual execution flow (verified from source)

#### Create

```
HTTP POST /api/v1/{module}/{resource}
  │
  ▼ middleware: TenantResolver → RequireAuth → RateLimit → RequirePermission
  │
  ▼ handler.EntityHandler.Create(c *fiber.Ctx)
  │  extracts: data, actor from ViewerContext
  │
  ▼ EntityService.Create(ctx, data, actor)
  │
  │ pipeline.RunBeforeCreate(&CreateContext)          [OUTSIDE TX]
  │   ├── applyDefaults
  │   ├── applyNamingSeries
  │   ├── BeforeValidate hooks
  │   ├── VALIDATE (required fields, type checks)
  │   ├── BeforeSave hooks
  │   └── BeforeCreate hooks
  │
  │ repo.WithTx(ctx, func(txCtx) error {             [TX begins]
  │   │  setTenantContext(txCtx, conn.db(), tenantID) (inside WithTx setup)
  │   │
  │   │  repo.Create(txCtx, CreateInput)             [PERSIST]
  │   │    └── INSERT INTO table ... RETURNING id
  │   │    └── SELECT * WHERE id = $1 (getWithDB)
  │   │
  │   │  ← AUDIT RECORD must go here →               [PLANNED ADDITION]
  │   │
  │   │  pipeline.RunAfterCreate(txCtx, created)     [after_save inside TX]
  │   │    ├── AfterSave hooks
  │   │    └── AfterCreate hooks
  │   │
  │   └── return nil                                 [TX commits]
  │   })                                             [or rolls back on error]
  │
  │ startWorkflows(ctx, OnCreate, created, actor)    [OUTSIDE TX]
  │
  ▼ HTTP 201 response

[AUDIT RECORD must execute between repo.Create and pipeline.RunAfterCreate]
[inside the WithTx callback — this is the only correct integration point]
```

#### Update

```
EntityService.Update(ctx, id, data, actor)
  │
  ├── repo.Get(ctx, id)                              [OUTSIDE TX — fetch current]
  │
  ├── pipeline.RunBeforeUpdate(pctx, current)        [OUTSIDE TX]
  │
  ├── repo.WithTx(ctx, func(txCtx) error {          [TX begins]
  │     repo.Update(txCtx, id, UpdateInput)          [PERSIST — SQL UPDATE]
  │     ← AUDIT RECORD (before=current, after=updated) →  [PLANNED]
  │     pipeline.RunAfterUpdate(txCtx, updated, current)
  │   })                                             [TX commits/rollbacks]
  │
  └── (no workflow start in current implementation)
```

#### Delete

```
EntityService.Delete(ctx, id, actor)
  │
  ├── repo.Get(ctx, id)                              [OUTSIDE TX — fetch before snapshot]
  │
  ├── pipeline.RunBeforeDelete(ctx, current)         [OUTSIDE TX]
  │
  ├── repo.WithTx(ctx, func(txCtx) error {          [TX begins]
  │     repo.Delete(txCtx, id)                       [PERSIST — SQL DELETE]
  │     ← AUDIT RECORD (before=current, after=nil) →  [PLANNED]
  │     pipeline.RunAfterDelete(txCtx, current)
  │   })                                             [TX commits/rollbacks]
  │
  └── (no workflow start for delete)
```

#### BulkCreate

```
repo.BulkCreate(ctx, []CreateInput)
  │
  └── repo.WithTx(ctx, func(txCtx) error {
        for each input:
          repo.Create(txCtx, input)                  [each INSERT in same TX]
      })
```

**Gap:** `BulkCreate` bypasses `EntityService` entirely — no pipeline hooks, no validation, no audit. This is documented in `driver/repository.go` ("Hooks run for each record") but there is no service-level `BulkCreate` that calls pipeline. Framework-level `BulkCreate` with audit is a v2 concern.

#### Custom Actions

```
handler.EntityHandler.Action(c *fiber.Ctx)
  │
  └── ActionDef.HandlerFunc(ActionContext)
        [completely custom — no pipeline, no audit, no TX management]
```

### Where AuditWriter Must Execute

**The only correct integration point is inside `EntityService`'s `repo.WithTx` callback, after `repo.Create/Update/Delete` and before `pipeline.RunAfterXxx`.**

This guarantees:
- ✅ Same transaction (txCtx carries the TX connection)
- ✅ Rollback safety (if audit write fails and policy = propagate, return error → WithTx rolls back → entity record also rolled back)
- ✅ No duplicate writes (called exactly once per mutation)
- ✅ No partial commits (all writes in same TX)
- ✅ No ordering violations (PERSIST first, then AUDIT, then after_save hooks)

**Implementation mechanism:**

The audit write should be added to `Pipeline` as `RunAuditRecord`, with `Pipeline` holding an `AuditWriter`. The service calls:

```go
repo.WithTx(ctx, func(txCtx context.Context) error {
    created, txErr = repo.Create(txCtx, input)
    if txErr != nil { return txErr }
    if err := pipeline.RunAuditRecord(txCtx, created, nil, record.Data); err != nil {
        return err  // failure policy applied inside RunAuditRecord
    }
    return pipeline.RunAfterCreate(txCtx, created)
})
```

Why in Pipeline (not directly in EntityService):
- Keeps audit as a declared pipeline stage (ADR-005)
- EntityService stays focused on orchestration
- `RunAuditRecord` can skip when `AuditWriter` is `NoopAuditWriter`
- Consistent with existing `RunBeforeCreate`/`RunAfterCreate` naming

---

## Phase 4 — Integration Points

### Complete integration map

| Location | File | Current behavior | Required change | Risk |
|---|---|---|---|---|
| `Pipeline` struct | `runtime/pipeline.go` | No `AuditWriter` field | Add `AuditWriter audit.AuditWriter` + `RunAuditRecord()` method | Low |
| `NewPipeline` | `runtime/pipeline.go` | Takes only `*compiler.CompiledSchema` | Add `AuditWriter audit.AuditWriter` parameter | Low — single call site in router |
| `EntityService.Create` | `api/service/entity.go` | No audit call in `WithTx` | Add `pipeline.RunAuditRecord(txCtx, created, nil, beforeSnap)` | Low |
| `EntityService.Update` | `api/service/entity.go` | No audit call in `WithTx` | Add `pipeline.RunAuditRecord(txCtx, updated, current.Data, updated.Data)` | Low |
| `EntityService.Delete` | `api/service/entity.go` | No audit call in `WithTx` | Add `pipeline.RunAuditRecord(txCtx, current, current.Data, nil)` | Low |
| `RegisterOptions` | `api/router/router.go` | No `AuditWriter` field | Add `AuditWriter audit.AuditWriter` | Low |
| `router.Register` | `api/router/router.go` | `runtime.NewPipeline(schema)` | `runtime.NewPipeline(schema, opts.AuditWriter)` | Low |
| `main.go` | `cmd/server/main.go` | No audit construction | Construct `TransactionalWriter`, warm `RiskScorer`, inject into `RegisterOptions` | Medium |
| `main.go` imports | `cmd/server/main.go` | `_ "awo.so/awo/platform/audit"` (side-effect import) | Phase 5: remove when `LogDefinition` entity is retired | Low (later) |
| `AuthService` | `platform/iam/service.go` | Writes to `iam_sessions`, `iam_login_audits` | Phase 3 of migration: also write AUTH events to `platform_audit_log` | Medium |
| `authz/RequirePermission` | `api/authz/authz.go` | Returns 403/500 | Phase 3: write ACCESS events on deny | Low |
| Outbox relay | `events/outbox/` | Dispatches to Temporal | Phase 3: write SYSTEM events per dispatch | Low |

### What the documentation missed

1. **`api/service` is the correct TX owner** — not in docs at all; service package was undocumented
2. **`BulkCreate` bypasses pipeline entirely** — not documented as a gap
3. **`BulkUpdate` explicitly documents "Hooks do NOT run"** — audit gap undocumented
4. **`AUTHORIZE` stage is in `authz/RequirePermission` middleware**, not in `EntityService` — architecture docs say AUTHORIZE is a pipeline stage but it's actually middleware applied per route
5. **Workflow start in `EntityService.startWorkflows` writes directly to Temporal** (not outbox yet — `// TODO: write to outbox table`)

---

## Phase 5 — Dependency Graph

### Verified dependency graph

```
awo/def
  ↑ (imports def)
awo/filter
  ↑
awo/driver
  ↑
awo/compiler (imports: def, filter, driver)
  ↑
awo/runtime (imports: compiler, def, naming)     ← add: awo/audit
  ↑
awo/contrib/pgx (imports: compiler, def, driver, filter, runtime, tx)
  ↑
awo/api/service (imports: compiler, def, driver, filter, runtime)
  ↑
awo/api/handler (imports: compiler, def, driver, filter, api/service)
  ↑
awo/api/router (imports: compiler, def, driver, auth, cache, contrib/pgx,
                         contrib/redis, api/authz, api/handler, api/service, runtime)
  ↑
awo/cmd/server/main.go (imports: api/router, bootstrap, auth, contrib/pgx,
                                  contrib/redis, events/outbox, iam, ...)
```

### New package: `awo/audit`

```
awo/audit imports:
  awo/def          ✓ — def.Actor, def.FieldDef
  awo/compiler     ✓ — compiler.EntitySchema, SensitiveFields
  contrib/pgx      ✗ — CANNOT import (circular: contrib/pgx imports runtime; runtime imports audit)
```

**Circular import blocker:** `awo/audit` needs to write to PostgreSQL but cannot import `contrib/pgx` (circular via runtime). Solution: `awo/audit` uses raw `pgx/v5` directly (not via contrib/pgx Repository). The `TransactionalWriter` accepts a `pgxpool.Pool` and uses `pgx.Tx` from context directly.

```
awo/audit imports:
  awo/def
  awo/compiler
  github.com/jackc/pgx/v5          ← raw pgx, not awo/contrib/pgx
  github.com/jackc/pgx/v5/pgxpool
  awo/tx                           ← to extract TX from context
```

**`awo/tx` package:** `contrib/pgx/conn.go` imports `awo.so/awo/tx` for `tx.WithConn` and `tx.ConnFromContext`. This is a low-level package. `awo/audit` can also import `awo/tx` to extract the pgx TX from context.

**Corrected dependency graph for audit:**

```
awo/def ←── awo/audit ─→ awo/compiler
                 │
                 └──────→ awo/tx
                 │
                 └──────→ github.com/jackc/pgx/v5 (raw)
                 │
                 └──────→ github.com/jackc/pgx/v5/pgxpool
```

**No circular imports.** ✓

### Forbidden imports verification

| Import | From | Verdict |
|--------|------|---------|
| `awo/audit` → `awo/runtime` | audit | FORBIDDEN — circular |
| `awo/audit` → `awo/driver` | audit | FORBIDDEN — circular |
| `awo/audit` → `awo/contrib/pgx` | audit | FORBIDDEN — circular |
| `awo/audit` → `awo/api/*` | audit | FORBIDDEN — layering |
| `awo/audit` → `awo/platform/*` | audit | FORBIDDEN — platform above audit |
| `awo/def` → `awo/audit` | def | FORBIDDEN — def is frozen kernel |
| `awo/runtime` → `awo/audit` | runtime | ALLOWED (add) |
| `awo/api/service` → `awo/audit` | service | NOT NEEDED — service uses Pipeline |
| `awo/api/router` → `awo/audit` | router | ALLOWED (for type in RegisterOptions) |
| `main.go` → `awo/audit` | main | ALLOWED |

---

## Phase 6 — Migration Readiness

### Next migration number

**Assumed:** 000452. Confirm before writing migrations.

### Migration plan (corrected)

#### Migration 000452: `platform_audit_log.up.sql`

Creates the unified audit infrastructure:

```sql
-- 1. Feature flag table
CREATE TABLE platform_audit_config (
    key         text PRIMARY KEY,
    value       text NOT NULL,
    updated_at  timestamptz NOT NULL DEFAULT now()
);
INSERT INTO platform_audit_config (key, value)
VALUES ('feature.unified_audit.enabled', 'false');

-- 2. Main audit table (partitioned)
CREATE TABLE platform_audit_log (...) PARTITION BY RANGE (created_at);

-- 3. Initial partitions (current month + 2 ahead)
CREATE TABLE platform_audit_log_YYYY_MM ...;

-- 4. Default partition (safety net)
CREATE TABLE platform_audit_log_default PARTITION OF platform_audit_log DEFAULT;

-- 5. Indexes (all on parent, inherited by partitions)
CREATE INDEX ...;

-- 6. Checkpoints table
CREATE TABLE platform_audit_checkpoint (...);

-- 7. Historical migration tracking
CREATE TABLE platform_audit_migration_log (...);

-- 8. Partition maintenance function
CREATE OR REPLACE FUNCTION platform_create_next_audit_partition() RETURNS void ...;

-- 9. pg_cron job (if pg_cron extension available)
-- SELECT cron.schedule(...) -- conditional on extension availability

-- 10. Permissions
REVOKE UPDATE, DELETE ON platform_audit_log FROM awo_app;
GRANT SELECT, INSERT ON platform_audit_log TO awo_app;
-- Create audit_retention_role if not exists
DO $$ BEGIN
    CREATE ROLE audit_retention_role;
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;
GRANT UPDATE, DELETE ON platform_audit_log TO audit_retention_role;

-- 11. Permission seed
INSERT INTO iam_role_permissions (role, permission)
VALUES
    ('role:platform-admin', 'platform.audit_log.read'),
    ('role:tenant.admin', 'platform.audit_log.read')
ON CONFLICT DO NOTHING;
```

**Rollback (000452.down.sql):**
```sql
DROP TABLE IF EXISTS platform_audit_migration_log;
DROP TABLE IF EXISTS platform_audit_checkpoint;
DROP TABLE IF EXISTS platform_audit_log CASCADE;  -- drops all partitions
DROP TABLE IF EXISTS platform_audit_config;
DROP FUNCTION IF EXISTS platform_create_next_audit_partition();
DELETE FROM iam_role_permissions WHERE permission = 'platform.audit_log.read';
```

**Zero-downtime:** Yes. Creates new tables only. No changes to existing tables.

#### Migration 000453: `platform_audit_remove_legacy.up.sql`

Phase 5 of migration strategy. Not written until Phase 3 (unified system) is stable for ≥7 days.

**DO NOT WRITE THIS MIGRATION UNTIL PHASE 3 STABILITY IS CONFIRMED.**

### RLS verification

`platform_audit_log` has `DISABLE ROW LEVEL SECURITY`. Global table. Confirmed in `AUDIT_STORAGE.md`. No `current_tenant_id()` call needed for INSERT. Tenant isolation is by `tenant_id` column value + application-layer permission check.

---

## Phase 7 — Testing Strategy

### Unit tests

#### `awo/audit` package (target: 95% coverage)

| Test | Scenario | Mock requirements |
|------|----------|-------------------|
| `TestSanitizer_StripSensitiveFields` | Fields with `Sensitive: true` are excluded | CompiledSchema mock |
| `TestSanitizer_StripAdditionalFields` | `EntityAuditConfig.AdditionalSensitiveFields` excluded | EntityAuditConfig registry |
| `TestSanitizer_DiffUsesStrippedData` | `ChangedFields` computed after stripping | N/A |
| `TestSanitizer_SensitiveFieldNotInChangedFields` | Sensitive field change not in ChangedFields | N/A |
| `TestRiskScorer_BaseScores` | Create=10, Update=20, Delete=30 | DB mock (table weights) |
| `TestRiskScorer_SeverityThresholds` | Score→CRITICAL/HIGH/MEDIUM/LOW/INFO | N/A |
| `TestRiskScorer_CappedAt100` | Composite score never exceeds 100 | N/A |
| `TestEntityAuditConfig_DefaultEnabled` | Unregistered entity → Enabled=true, Category=DATA | N/A |
| `TestEntityAuditConfig_ExplicitConfig` | Registered config is returned | N/A |
| `TestEntityAuditConfig_PanicDuplicateRegister` | Duplicate registration panics | N/A |
| `TestNoopAuditWriter` | Always returns nil | N/A |
| `TestFailurePolicy_AdminPropagates` | ADMIN category error is returned | N/A |
| `TestFailurePolicy_DataSuppresses` | DATA category error → nil + log + meter | N/A |
| `TestSessionID_HMACNotRaw` | SessionID ≠ raw token | N/A |

#### `awo/runtime` (pipeline changes — target: 90% coverage)

| Test | Scenario |
|------|----------|
| `TestRunAuditRecord_WritesAuditRecord` | AuditWriter.Write called once per Create |
| `TestRunAuditRecord_SkippedWhenDisabled` | Enabled=false → Write not called |
| `TestRunAuditRecord_FailurePropagatesForAdmin` | ADMIN write failure → pipeline returns error |
| `TestRunAuditRecord_FailureSuppressedForData` | DATA write failure → pipeline continues |
| `TestRunAuditRecord_BeforeSnapshotNilOnCreate` | before=nil for Create |
| `TestRunAuditRecord_AfterSnapshotNilOnDelete` | after=nil for Delete |
| `TestPipeline_NilAuditWriterPanics` | NewPipeline with nil auditWriter panics |

#### `awo/api/service` (transaction correctness — target: 90%)

| Test | Scenario |
|------|----------|
| `TestCreate_AuditRecordInSameTx` | Audit INSERT uses same tx connection |
| `TestCreate_AuditFailure_AdminRollsBack` | ADMIN entity: audit failure → entity record rolled back |
| `TestCreate_AuditFailure_DataSucceeds` | DATA entity: audit failure → entity record committed |
| `TestUpdate_BeforeSnapshotIsCurrentRecord` | before snapshot = record before UPDATE |
| `TestDelete_AfterSnapshotNil` | after snapshot = nil for Delete |

### Integration tests (real PostgreSQL required)

| Test | Scenario | Requirement |
|------|----------|-------------|
| `TestAuditWriter_InsertInTransaction` | Record appears in platform_audit_log when TX commits | PostgreSQL 14+ |
| `TestAuditWriter_RolledBackWithEntity` | platform_audit_log empty when TX rolls back | PostgreSQL 14+ |
| `TestAuditLog_NoRLS` | platform_audit_log accessible across tenants | RLS disabled confirmed |
| `TestPartition_CurrentMonthExists` | Partition for current month created by migration | PostgreSQL 14+ |
| `TestPartition_DefaultPartitionIsEmpty` | No rows in platform_audit_log_default in normal ops | N/A |
| `TestMigration_000452_RollbackSafe` | 000452.down.sql drops all audit tables cleanly | N/A |
| `TestSensitiveField_NotInAuditLog` | field with Sensitive:true absent from before_data/after_data | N/A |
| `TestCrossEntityAuditVisibility` | platform-admin query returns records for all tenants | N/A |

### Race tests

```go
func TestTransactionalWriter_ConcurrentWrites(t *testing.T) {
    // 100 goroutines each writing an audit record concurrently
    // All should succeed; no data races
}

func TestEntityAuditConfig_ConcurrentConfigRead(t *testing.T) {
    // Multiple goroutines calling ConfigFor() concurrently
    // No races; registry is read-only after init()
}
```

### Chaos tests

| Scenario | Expected behavior |
|----------|------------------|
| Audit DB INSERT fails (network error) — DATA entity | Entity committed; `audit_write_failure_total` incremented |
| Audit DB INSERT fails (network error) — ADMIN entity | Entity rolled back; HTTP 500 |
| PostgreSQL goes down mid-mutation | TX rollback; no partial records |
| `audit_sensitive_tables` config table missing | RiskScorer warm-up fails; panic during bootstrap |

---

## Phase 8 — Final Implementation Blueprint

### Prerequisite: ADR-020 (partition maintenance)

Write and append ADR-020 to `DECISION_REGISTER.md`: Audit partition maintenance via pg_cron. This must be approved before writing migration 000452.

### Implementation phases

#### Phase A: Core `awo/audit` package (no framework changes)

**Files to create:**
1. `awo/audit/doc.go` — package documentation
2. `awo/audit/record.go` — `AuditRecord`, `OperationType`, `EventCategory`, `Severity`, `SystemActor`
3. `awo/audit/writer.go` — `AuditWriter` interface, `NoopAuditWriter`
4. `awo/audit/config.go` — `EntityAuditConfig`, `Register()`, `ConfigFor()` registry
5. `awo/audit/sanitizer.go` — `Sanitizer` struct, `Strip()`, `ComputeChangedFields()`
6. `awo/audit/scorer.go` — `RiskScorer`, `NewRiskScorer()`, `Score()`, DB config warm-up
7. `awo/audit/transactional_writer.go` — `TransactionalWriter` implementing `AuditWriter`
8. `awo/audit/failure.go` — failure policy dispatch by `EventCategory`

**Tests:** `awo/audit/*_test.go` — all unit tests from Phase 7.

**Dependencies:** `awo/def`, `awo/compiler`, `awo/tx`, raw `pgx/v5`.

**Compile guard:** Must compile cleanly before Phase B.

---

#### Phase B: Pipeline integration

**Files to modify:**
1. `awo/runtime/pipeline.go`
   - Add `auditWriter audit.AuditWriter` field to `Pipeline`
   - Update `NewPipeline(schema *compiler.CompiledSchema, aw audit.AuditWriter) *Pipeline`
   - Add `RunAuditRecord(ctx context.Context, record *def.EntityRecord, before, after map[string]any) error`

**`RunAuditRecord` implementation sketch:**
```go
func (p *Pipeline) RunAuditRecord(ctx context.Context, record *def.EntityRecord, before, after map[string]any) error {
    es, err := p.lookupSchema(record.EntityName)
    if err != nil { return err }

    ar := audit.AuditRecord{
        ID:            uuid.New(),
        TenantID:      record.TenantID,
        EntityName:    record.EntityName,
        RecordID:      record.ID,
        Operation:     toAuditOp(record.Meta.OperationType),
        Actor:         record.Meta.Actor,
        BeforeData:    before,
        AfterData:     after,
        EventCategory: audit.ConfigFor(record.EntityName).Category,
        CreatedAt:     time.Now().UTC(),
    }
    // SessionID, IPAddress, RequestID from context
    if vc, ok := auth.TryViewerFromContext(ctx); ok {
        ar.IPAddress = vc.IPAddress()
        // ar.SessionID = audit.HMACSession(vc.SessionToken(), p.signingSecret)
    }
    return p.auditWriter.Write(ctx, ar)
}
```

**Tests:** `awo/runtime/pipeline_audit_test.go`.

---

#### Phase C: Service integration

**Files to modify:**
1. `awo/api/service/entity.go`
   - `Create`: add `pipeline.RunAuditRecord(txCtx, created, nil, created.Data)` after `repo.Create`
   - `Update`: capture `current.Data` as `beforeSnap` before `WithTx`, add `pipeline.RunAuditRecord(txCtx, updated, beforeSnap, updated.Data)` after `repo.Update`
   - `Delete`: add `pipeline.RunAuditRecord(txCtx, current, current.Data, nil)` after `repo.Delete`

**Tests:** `awo/api/service/entity_audit_test.go` — transaction correctness tests.

---

#### Phase D: Router and bootstrap wiring

**Files to modify:**
1. `awo/api/router/router.go`
   - Add `AuditWriter audit.AuditWriter` to `RegisterOptions`
   - Change `runtime.NewPipeline(schema)` → `runtime.NewPipeline(schema, opts.AuditWriter)`
   - If `opts.AuditWriter == nil`, use `audit.NoopAuditWriter{}`

2. `awo/cmd/server/main.go`
   - Add `AUDIT_SIGNING_SECRET` env var (required)
   - Construct `RiskScorer` (warm-up from DB — synchronous, timeout 10s)
   - Construct `TransactionalWriter`
   - Add `AuditWriter` to `router.RegisterOptions`

**Tests:** `awo/api/router/router_audit_test.go` — verify `NoopAuditWriter` used when `AuditWriter` nil.

---

#### Phase E: Migration 000452

**Files to create:**
1. `db/migration/000452_platform_audit_log.up.sql`
2. `db/migration/000452_platform_audit_log.down.sql`

**Contents:** Per `AUDIT_STORAGE.md` DDL. Must include pg_cron job (conditional on extension).

**Validation:** Apply in test database. Verify partitions created. Verify INSERT works. Verify rollback drops all tables cleanly.

---

#### Phase F: EntityAuditConfig registrations

**Files to modify:** Every entity's `definition_*.go` file (or add new `audit_config.go` per module):
- Finance module entities → `CategoryData`
- IAM entities (`iam_user`, `iam_tenant`, `iam_role`) → `CategoryAdmin`
- IAM auth events → `CategoryAuth` (handled directly by `AuthService`, not via registry)
- Platform entities → `CategorySystem`

For entities where `AuditEnabled` should be false (high-volume metrics), add `audit.Register(EntityAuditConfig{EntityName: "...", Enabled: false})`.

**Tests:** `awo/audit/config_test.go` — verify all module entities have registered configs.

---

#### Phase G: Feature flag enable (staging validation)

1. Apply migration 000452 to staging
2. Deploy Phase A–F code with `AUDIT_SIGNING_SECRET` set
3. Verify `audit_write_total` metric is 0 (flag still false)
4. Enable flag: `UPDATE platform_audit_config SET value='true' WHERE key='feature.unified_audit.enabled'`
5. Verify records appear in `platform_audit_log`
6. Validate for ≥24 hours before proceeding

---

### File creation order (dependency-safe)

```
1. db/migration/000452_platform_audit_log.{up,down}.sql
2. awo/audit/doc.go
3. awo/audit/record.go
4. awo/audit/config.go
5. awo/audit/writer.go
6. awo/audit/failure.go
7. awo/audit/sanitizer.go
8. awo/audit/scorer.go
9. awo/audit/transactional_writer.go
10. awo/audit/*_test.go (unit)
--- Phase A complete, compiles ---
11. awo/runtime/pipeline.go (modify: add AuditWriter, RunAuditRecord)
12. awo/runtime/pipeline_audit_test.go
--- Phase B complete, compiles ---
13. awo/api/service/entity.go (modify: add RunAuditRecord calls)
14. awo/api/service/entity_audit_test.go
--- Phase C complete, compiles ---
15. awo/api/router/router.go (modify: RegisterOptions + NewPipeline call)
16. awo/cmd/server/main.go (modify: AuditWriter construction + injection)
--- Phase D complete, compiles ---
17. EntityAuditConfig registrations (module definition files)
--- Phase F complete ---
```

---

### Rollback strategy

| Phase | Rollback |
|-------|----------|
| A (audit package) | Delete `awo/audit/`; no framework changes |
| B (pipeline) | Revert `runtime/pipeline.go`; NewPipeline signature change affects only Phase D |
| C (service) | Revert `api/service/entity.go`; isolated change |
| D (wiring) | Revert `router.go` + `main.go`; no DB changes |
| E (migration 000452) | Run `000452_platform_audit_log.down.sql` |
| G (flag enable) | `UPDATE platform_audit_config SET value='false'` |

---

## Documentation Corrections Required

The following documentation must be corrected BEFORE implementation begins:

### 1. `00-overview/DECISION_REGISTER.md` — ADR-013

**Current (incorrect):** "Transaction ownership: driver (`contrib/pgx`) owns and manages TX"

**Correct:** "Transaction ownership: service layer (`api/service.EntityService`) orchestrates TX via `repo.WithTx()`. The driver provides `WithTx()` but does not open transactions for individual `Create/Update/Delete` calls — those are auto-commit unless the service wraps them in `WithTx()`."

### 2. `02-pipeline/LIFECYCLE_SPEC.md` — §3

**Current (incorrect):** "driver opens TX, calls RunAuditRecord and RunAfterCreate from within TX"

**Correct:** "EntityService opens TX via `repo.WithTx()`. Inside the TX callback: `repo.Create()` runs, then `pipeline.RunAuditRecord()` runs, then `pipeline.RunAfterCreate()` runs. The TX commits when the callback returns nil."

### 3. `12-audit/AUDIT_ARCH.md` — §2, §3, §5

**Incorrect:** Component responsible is "driver"; AuditWriter "called by driver"; "driver calls pipeline.RunAuditRecord from within its TX"

**Correct:** Component responsible is `EntityService`; sequence is service-owned; `pipeline.RunAuditRecord` is called by service inside `repo.WithTx` callback.

---

## Risk Register

| Risk | Probability | Impact | Mitigation |
|------|------------|--------|------------|
| `NewPipeline` signature change breaks existing tests | HIGH | LOW | Update all `NewPipeline(schema)` calls to `NewPipeline(schema, audit.NoopAuditWriter{})` |
| `RiskScorer` warm-up panics if `audit_sensitive_tables` missing (migration not yet applied) | MEDIUM | HIGH | RiskScorer must handle "table not found" gracefully with default scores; panic is wrong |
| `platform_audit_log` partition missing for current month | LOW | CRITICAL | Bootstrap validation: check current-month partition exists, panic if absent |
| pg_cron not available in deployment environment | MEDIUM | MEDIUM | Document manual partition creation procedure; add monitoring alert |
| `AUDIT_SIGNING_SECRET` not set in production | MEDIUM | HIGH | Fail fast in `main.go`: `mustEnv("AUDIT_SIGNING_SECRET")` |
| Feature flag DB read fails (DB down) | LOW | MEDIUM | Default to `false` (legacy mode) on read failure; log WARNING |

---

## Implementation Checklist

### Before writing any code
- [ ] ADR-020 written and appended to DECISION_REGISTER.md (pg_cron decision)
- [ ] ADR-013 corrected in DECISION_REGISTER.md (service, not driver, owns TX)
- [ ] LIFECYCLE_SPEC.md §3 corrected (service orchestrates TX)
- [ ] AUDIT_ARCH.md §2, §3, §5 corrected (service integration point, not driver)
- [ ] Migration sequence confirmed (check actual highest migration number)
- [ ] PostgreSQL version confirmed in deployment environment

### Phase A (awo/audit package)
- [ ] All types defined (`AuditRecord`, `AuditWriter`, `EntityAuditConfig`, `SystemActor`, `EventCategory`, `Severity`)
- [ ] `NoopAuditWriter` implemented
- [ ] `EntityAuditConfig` registry with `Register()` and `ConfigFor()` — panic on post-bootstrap registration
- [ ] `Sanitizer.Strip()` — correct order: FieldDef.Sensitive → AdditionalSensitiveFields → DB config
- [ ] `Sanitizer.ComputeChangedFields()` — uses stripped maps, not raw
- [ ] `RiskScorer` — handles missing config table gracefully (returns default scores)
- [ ] `TransactionalWriter` — extracts pgx TX from context via `awo/tx`; falls back to pool for standalone writes
- [ ] Failure policy — ADMIN/SECURITY propagate; all others suppress+log+meter
- [ ] Unit tests passing (no real DB required)

### Phase B (pipeline)
- [ ] `NewPipeline(schema, AuditWriter)` — panics if AuditWriter is nil
- [ ] `pipeline.RunAuditRecord(ctx, record, before, after)` — correct actor extraction, category lookup, sensitive field handling
- [ ] All existing `NewPipeline(schema)` call sites updated to `NewPipeline(schema, audit.NoopAuditWriter{})`
- [ ] Unit tests passing

### Phase C (service)
- [ ] `Create` captures `beforeSnap` (nil for create) → `RunAuditRecord(txCtx, created, nil, created.Data)`
- [ ] `Update` captures `current.Data` as `beforeSnap` BEFORE `WithTx` → `RunAuditRecord(txCtx, updated, beforeSnap, updated.Data)`
- [ ] `Delete` captures `current.Data` as `beforeSnap` BEFORE `WithTx` → `RunAuditRecord(txCtx, current, beforeSnap, nil)`
- [ ] Integration tests confirm audit record and entity record in same TX

### Phase D (wiring)
- [ ] `RegisterOptions.AuditWriter` added; nil → `NoopAuditWriter{}`
- [ ] `main.go` constructs `TransactionalWriter` with pool + RiskScorer + signing secret
- [ ] `mustEnv("AUDIT_SIGNING_SECRET")` in `main.go`

### Phase E (migration)
- [ ] 000452 applied in test DB
- [ ] Partitions created for current month + 2 ahead
- [ ] Default partition exists
- [ ] INSERT test record succeeds
- [ ] 000452.down.sql drops all tables cleanly
- [ ] `audit_retention_role` created

### Phase F (entity configs)
- [ ] All Finance entities: `CategoryData`, `Enabled: true`
- [ ] IAM entities: `CategoryAdmin`, `Enabled: true`, sensitive fields listed
- [ ] High-volume non-sensitive entities: `Enabled: false` (if any)

### Phase G (enable in staging)
- [ ] Flag enabled; records appearing in `platform_audit_log`
- [ ] No `audit_write_failure_total` increments
- [ ] `audit_write_duration_seconds` p99 < 5ms
- [ ] ≥24 hour stable observation before proceeding
