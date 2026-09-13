# Unified Audit Architecture

**Classification:** Specification — Tier 1
**Owner:** `12-audit/AUDIT_ARCH.md`
**Status:** Approved — v1.0
**Package:** `awo.so/awo/audit`
**ADRs:** ADR-005, ADR-013 through ADR-019

---

## Purpose

This document is the authoritative architecture specification for the Awo Unified Audit System. It supersedes the prior dual-implementation design (`iam_audit_log` Go entity + `audit_log` SQL triggers). It defines the component architecture, integration points, transaction lifecycle, dependency graph, configuration model, security properties, storage design, migration strategy, and operational model.

Implementation MUST follow this document. Any implementation decision not covered here requires an ADR before proceeding.

---

## Non-Goals

The following are explicitly out of scope for v1:

- Full cryptographic hash chain per record (tamper evidence v2; deferred to v2)
- Async audit write path (no channel-based buffering; sync write only)
- `AuditingRepository[T]` wrapper pattern (unsound with Model B TX ownership; rejected ADR-013)
- Custom action audit coverage (ActionDef handlers are not intercepted by the pipeline; tracked as gap)
- Streaming audit export (out-of-band; ops concern)
- Per-field change history UI (query pattern only; not a framework responsibility)

---

## 1. Architectural Overview

```
HTTP Request
     │
     ▼
Middleware (TenantResolver → RequireAuth → RateLimit → RequirePermission)
     │ ViewerContext in ctx
     ▼
Handler (api/handler.EntityHandler)
     │ extracts data + actor from ViewerContext
     ▼
EntityService.Create(ctx, data, actor)          ← service orchestrates lifecycle
     │
     │ [OUTSIDE TX]
     ├── pipeline.RunBeforeCreate()
     │     ├── ApplyDefaults + NamingSeries
     │     ├── BeforeValidate hooks
     │     ├── VALIDATE
     │     ├── BeforeSave hooks
     │     └── BeforeCreate hooks
     │
     │ repo.WithTx(ctx, func(txCtx) error {    ← service opens TX here
     │     │
     │     │ [INSIDE TX]
     │     ├── repo.Create(txCtx, input)        PERSIST (SQL INSERT)
     │     │
     │     ├── pipeline.RunAuditRecord(txCtx, record, before, after)
     │     │         │
     │     │         └── AuditWriter.Write(ctx, auditRecord)
     │     │                   ├── EntityAuditConfig check (Enabled?)
     │     │                   ├── Sanitizer.Strip (sensitive fields)
     │     │                   ├── RiskScorer.Score
     │     │                   ├── INSERT INTO platform_audit_log (same TX)
     │     │                   └── failure policy (ADR-017)
     │     │
     │     └── pipeline.RunAfterCreate(txCtx, record)
     │               └── AfterSave + AfterCreate hooks (same TX)
     │
     │ })                                       TX commits or rolls back
     │
     └── startWorkflows(ctx, ...)               OUTSIDE TX
```

---

## 2. Component Responsibilities

### 2.1 `awo/audit` Package

**Purpose:** Audit write path — record construction, sanitization, scoring, storage, and failure policy.

**Responsibilities:**
- Define `AuditWriter` interface (public contract; ADR-014)
- Define `AuditRecord` struct (public contract; ADR-015)
- Maintain `EntityAuditConfig` registry (ADR-016)
- Implement `TransactionalWriter` — the production `AuditWriter` implementation
- Implement `RiskScorer` — computes `Severity` and `RiskScore` from entity + operation + field metadata
- Implement `Sanitizer` — strips sensitive fields from snapshots
- Enforce failure policy by `EventCategory` (ADR-017)

**Does NOT:**
- Open database connections or transactions
- Know about the pipeline or runtime internals
- Know about Fiber, HTTP, or request handling
- Import from `awo/runtime` or `awo/driver`

**Dependencies (imports):**
```
awo/audit imports:
  awo/def          ← def.Actor, def.FieldDef.Sensitive
  awo/auth         ← (no import; AuditRecord.Actor is *def.Actor)
  contrib/pgx      ← for TransactionalWriter INSERT (pgx.Tx from context)
  (standard library only beyond this)
```

**Forbidden imports:**
```
awo/audit MUST NOT import:
  awo/runtime      ← circular dependency
  awo/driver       ← driver imports audit (not reverse)
  awo/api          ← audit is below API layer
  awo/platform/*   ← platform modules are above audit
```

### 2.2 `awo/runtime` (pipeline)

**Purpose:** Orchestrate the entity mutation lifecycle stages.

**Audit responsibility:** At the AUDIT RECORD stage, call `AuditWriter.Write(ctx, record)` with the record constructed from the pipeline's current entity state.

**AuditWriter injection:** The pipeline receives an `AuditWriter` at construction time:

```go
// awo/runtime
type Pipeline struct {
    auditWriter audit.AuditWriter
    // ...other fields
}

func NewPipeline(auditWriter audit.AuditWriter, ...) *Pipeline { ... }
```

If `auditWriter` is nil at construction time, the pipeline panics — there is no implicit noop. Callers must explicitly pass `audit.NoopAuditWriter{}` in tests and non-audit environments.

**AuditRecord construction (pipeline's role):**
The pipeline constructs the `AuditRecord` from:
- `EntityRecord.Meta.Actor` → `AuditRecord.Actor`
- `EntityRecord.Meta.OperationType` → `AuditRecord.Operation`
- Before/After snapshots from ASSEMBLE and PERSIST stages
- `ViewerContext` → `IPAddress`, raw session token (for HMAC hashing)
- `EntityDefinition.EntityName()` → `AuditRecord.EntityName`
- `EntityAuditConfig.Category` → `AuditRecord.EventCategory` (via `audit.ConfigFor`)

The pipeline does NOT score risk, strip sensitive fields, or apply failure policy. These are the `AuditWriter` implementation's responsibilities.

### 2.3 `api/service.EntityService`

**Purpose:** Orchestrate the complete entity mutation lifecycle — pipeline pre-persist stages, transaction management, PERSIST, AUDIT RECORD, after_save hooks, workflow dispatch.

**Transaction ownership (ADR-013):** `EntityService` opens the transaction via `repo.WithTx()`. Inside the `WithTx` callback it executes: `repo.Create/Update/Delete` (PERSIST), then `pipeline.RunAuditRecord` (AUDIT RECORD), then `pipeline.RunAfterXxx` (after_save). The TX commits when the callback returns nil; rolls back on any error.

**Audit call site (Create):**
```go
repo.WithTx(ctx, func(txCtx context.Context) error {
    created, err = repo.Create(txCtx, input)
    if err != nil { return err }
    if err = pipeline.RunAuditRecord(txCtx, created, nil, created.Data); err != nil {
        return err  // propagated or suppressed per ADR-017
    }
    return pipeline.RunAfterCreate(txCtx, created)
})
```

**Does NOT:**
- Know about `AuditWriter` directly — delegates to `pipeline.RunAuditRecord`
- Know about audit categories, risk scores, or failure policies

### 2.4 `contrib/pgx` (driver)

**Purpose:** PostgreSQL persistence. Provides `WithTx()` for transaction management and `Create/Update/Delete/Query` for DML.

**Audit responsibility:** None. The driver has no knowledge of audit. It provides `WithTx(ctx, fn)` which the service uses to scope a transaction. The driver sets `set_tenant_context` inside `WithTx` before calling `fn`.

**Transaction mechanics:** `repo.WithTx()` calls `pool.BeginTx()`, stores the TX in context via `tx.WithConn(ctx, pgConn)`, calls `fn(txCtx)`, then `Commit()` or `Rollback()`. Any code inside `fn` that extracts the connection from `txCtx` (including `audit.TransactionalWriter`) participates in the same transaction automatically.

### 2.4 `TransactionalWriter` (production AuditWriter)

**Purpose:** Write `AuditRecord` to `platform_audit_log` within the caller's transaction.

**Responsibilities:**
1. Call `audit.ConfigFor(record.EntityName)` — check `Enabled` flag
2. Call `Sanitizer.Strip(record, entitySchema)` — remove sensitive fields
3. Call `RiskScorer.Score(record)` — compute `Severity` and `RiskScore`
4. Extract TX connection from `ctx` — use the same connection the driver opened
5. Execute `INSERT INTO platform_audit_log (...) VALUES (...)`
6. Apply failure policy based on `record.EventCategory` (ADR-017)

**Does NOT:**
- Open new database connections
- Use a connection pool directly
- Buffer writes or use goroutines
- Retry failed writes (failure policy: propagate or suppress; no retry)

### 2.5 `RiskScorer`

**Purpose:** Assign `Severity` and `RiskScore` (0–100) to each audit record.

**Inputs:**
- Operation (Delete=30, Update=20, Create=10 base points)
- Table weight from `audit_sensitive_tables` DB config (0–50 additional points)
- Field weights from `audit_sensitive_fields` DB config (0–20 per sensitive field changed)
- `EventCategory` (SECURITY/ADMIN add +20 override)
- Score is capped at 100

**Severity thresholds:**
- Score ≥ 70 → CRITICAL
- Score ≥ 50 → HIGH
- Score ≥ 30 → MEDIUM
- Score ≥ 10 → LOW
- Score < 10 → INFO

**Cache:** `RiskScorer` caches `audit_sensitive_tables` and `audit_sensitive_fields` at startup. Cache is populated synchronously before the HTTP server starts (blocking warm-up). Cache is read-only after bootstrap.

**Requires implementation verification:** The DB config tables (`audit_sensitive_tables`, `audit_sensitive_fields`) are defined in migrations 000449. Verify migration sequencing before implementation.

### 2.6 `Sanitizer`

**Purpose:** Strip sensitive fields from `BeforeData` and `AfterData` before they are written to the audit record.

**Stripping sources (applied in order):**
1. `CompiledSchema` field metadata — `FieldDef.Sensitive == true`
2. `EntityAuditConfig.AdditionalSensitiveFields` — registration-time list
3. `audit_sensitive_fields` DB config — runtime-discovered PII (from RiskScorer cache)

**Output:** Stripped maps (new `map[string]any` values; original EntityRecord data is not mutated).

**Diff computation:** `ChangedFields` is computed by `Sanitizer` after stripping. Callers never compute diffs independently.

---

## 3. Sequence Diagrams

### 3.1 Entity Mutation with Audit (DATA category)

```
Handler      EntityService       repo.WithTx     Pipeline        AuditWriter      DB
  │               │                  │               │                │             │
  │ Create()      │                  │               │                │             │
  ├──────────────►│                  │               │                │             │
  │               │ RunBeforeCreate  │               │                │             │
  │               ├─────────────────────────────────►│                │             │
  │               │ [hooks, validate]│               │                │             │
  │               │◄─────────────────────────────────┤                │             │
  │               │                  │               │                │             │
  │               │ repo.WithTx()    │               │                │             │
  │               ├─────────────────►│               │                │             │
  │               │                  │ BEGIN TX       │                │             │
  │               │                  ├───────────────────────────────────────────────►
  │               │                  │ setTenantCtx   │                │             │
  │               │                  ├───────────────────────────────────────────────►
  │               │                  │               │                │             │
  │               │                  │ repo.Create    │                │             │
  │               │                  ├───────────────────────────────────────────────► INSERT
  │               │                  │               │                │             │
  │               │                  │ RunAuditRecord │                │             │
  │               │                  ├──────────────►│                │             │
  │               │                  │               │ Write()        │             │
  │               │                  │               ├───────────────►│             │
  │               │                  │               │                │ Strip+Score │
  │               │                  │               │                ├─────────────► INSERT audit
  │               │                  │               │                │ nil         │
  │               │                  │               │◄───────────────┤             │
  │               │                  │◄──────────────┤                │             │
  │               │                  │               │                │             │
  │               │                  │ RunAfterCreate │                │             │
  │               │                  ├──────────────►│                │             │
  │               │                  │ [after_save hooks, same TX]     │             │
  │               │                  │◄──────────────┤                │             │
  │               │                  │               │                │             │
  │               │                  │ COMMIT TX      │                │             │
  │               │                  ├───────────────────────────────────────────────►
  │               │◄─────────────────┤               │                │             │
  │◄──────────────┤                  │               │                │             │
 HTTP 201
```

### 3.2 Entity Mutation — ADMIN category, audit write fails

```
  ...same as above until INSERT audit fails...

  │               │                  │ RunAuditRecord │                │             │
  │               │                  ├──────────────►│                │             │
  │               │                  │               │ Write()        │             │
  │               │                  │               ├───────────────►│             │
  │               │                  │               │                ├─────────────► INSERT fails
  │               │                  │               │                │ error       │
  │               │                  │               │◄───────────────┤             │
  │               │                  │               │ (ADMIN: propagate error)     │
  │               │                  │◄─ error ──────┤                │             │
  │               │                  │ ROLLBACK TX    │                │             │
  │               │                  ├───────────────────────────────────────────────►
  │               │◄─ error ─────────┤               │                │             │
  │◄─ HTTP 500 ───┤                  │               │                │             │
```

### 3.3 Session Audit (AUTH category — IAM service, not pipeline)

Session login/logout events are NOT captured through the entity mutation pipeline (sessions are not entity CRUD operations). They are written by `AuthService` directly:

```
AuthService          AuditWriter             DB
    │                    │                    │
    │ (login succeeds)   │                    │
    │ Build AuditRecord  │                    │
    │ Category: AUTH     │                    │
    │ Operation: "login" │                    │
    │                    │                    │
    │ Write(ctx, record) │                    │
    ├───────────────────►│                    │
    │                    │ INSERT audit       │
    │                    ├───────────────────►│
    │                    │ (failure: suppress │
    │                    │  + log + meter)    │
    │                    │                    │
    │ (nil returned)     │                    │
    │◄───────────────────┤                    │
```

Note: For `AuthService` audit writes, the context does NOT carry a pgx TX (sessions are written independently of entity mutations). `TransactionalWriter` must detect the absence of a TX in ctx and execute a standalone INSERT using a connection acquired from the pool. It must also set `SET LOCAL app.current_tenant_id = $1` on the connection before the INSERT (required for the `system_insert` RLS policy).

---

## 4. Dependency Graph

```
awo/def
  ↑
awo/audit ←──────────────── awo/runtime (pipeline)
  ↑                              ↑
contrib/pgx (TransactionalWriter)  contrib/pgx (driver)
  ↑
awo/bootstrap (wires AuditWriter into Pipeline)
  ↑
awo/cmd/server/main.go
```

**Forbidden edges:**
- `awo/audit` → `awo/runtime` — circular
- `awo/audit` → `awo/driver` — circular (driver imports audit via pipeline)
- `awo/audit` → `awo/platform/*` — platform modules are above audit layer
- `awo/def` → `awo/audit` — def is frozen kernel; audit is infrastructure

---

## 5. Bootstrap Integration

**Verified:** `bootstrap.Run()` returns `*Result{Pool, Redis, Schema}`. Bootstrap is pure infrastructure — it does not construct application services. Audit wiring happens in `cmd/server/main.go` after bootstrap, before `router.Register`.

The `AuditWriter` is constructed in `main.go` and injected via `router.RegisterOptions`:

```go
// cmd/server/main.go (new additions)

result, err := bootstrap.Run(ctx, bootstrap.Config{...})

// Warm RiskScorer cache (synchronous — before HTTP server starts)
scorer, err := audit.NewRiskScorer(ctx, result.Pool)
if err != nil {
    slog.Error("audit RiskScorer warm-up failed", "err", err)
    os.Exit(1)
}

signingSecret := mustEnv("AUDIT_SIGNING_SECRET")
auditWriter := audit.NewTransactionalWriter(result.Pool, scorer, []byte(signingSecret))

// ... IAM, tenant repo, relay setup ...

router.Register(app, result.Schema, router.RegisterOptions{
    Pool:        result.Pool,
    Redis:       result.Redis,
    IAM:         iamModule.Auth,
    Tenants:     tenantRepo,
    Authz:       evaluator,
    Temporal:    nil,
    AuditWriter: auditWriter,   // ← new field
})
```

Inside `router.Register`, the pipeline is constructed as:
```go
pipeline := runtime.NewPipeline(schema, opts.AuditWriter)
// if opts.AuditWriter == nil: use audit.NoopAuditWriter{}
```

**Files requiring changes:**
- `awo/api/router/router.go` — add `AuditWriter audit.AuditWriter` to `RegisterOptions`; pass to `NewPipeline`
- `awo/runtime/pipeline.go` — `NewPipeline` takes `AuditWriter`; `Pipeline` struct holds it
- `awo/cmd/server/main.go` — construct `RiskScorer` + `TransactionalWriter`; inject into `RegisterOptions`

---

## 6. Configuration

### 6.1 EntityAuditConfig Registry

Declared in `init()`, alongside `def.Register()`:

```go
// finance/definition_invoice.go
func init() {
    def.Register(&InvoiceDefinition)
    audit.Register(audit.EntityAuditConfig{
        EntityName: "finance_invoice",
        Enabled:    true,
        Category:   audit.CategoryData,
    })
}

// platform/iam/definition_user.go
func init() {
    def.Register(&UserDefinition)
    audit.Register(audit.EntityAuditConfig{
        EntityName: "iam_user",
        Enabled:    true,
        Category:   audit.CategoryAdmin,
        AdditionalSensitiveFields: []string{"password_hash", "totp_secret"},
    })
}
```

### 6.2 DB Config Tables

Two tables (created in migration 000449) provide runtime sensitivity configuration:

- `audit_sensitive_tables` — per-table weight, category override, compliance flags
- `audit_sensitive_fields` — per-field weight, GDPR/PCI-DSS/KRA-eTIMS flags

These are read once at startup into `RiskScorer`'s cache. The cache is not updated at runtime. Restarts are required to pick up config changes.

### 6.3 Feature Flag

The `platform_audit_config` table controls the migration cutover flag:

```sql
feature.unified_audit.enabled = 'true' | 'false'
```

When `false`: legacy `iam_audit_log` + SQL triggers remain active; `TransactionalWriter.Write` is a no-op.
When `true`: unified system is active; legacy system is inactive.

**Requires implementation verification:** The `platform_audit_config` table must be read before the HTTP server starts. This is a synchronous DB read during bootstrap. Verify that the bootstrap sequence supports this.

---

## 7. Security Properties

### 7.1 Immutability

`platform_audit_log` is append-only for `awo_app` database role:
- `awo_app` has INSERT + SELECT only
- UPDATE and DELETE require `audit_retention_role` (a dedicated PostgreSQL role)
- `audit_retention_role` is used only for GDPR anonymization via explicitly audited ops

### 7.2 Session Token Protection

Raw session tokens are never stored in audit records. The `SessionID` field stores `HMAC-SHA256(token, server_signing_secret)`. This allows session correlation without token exposure.

### 7.3 Access Control

Read access to `platform_audit_log` is gated by `platform.audit_log.read` permission:
- `role:platform-admin` → granted (cross-tenant visibility)
- `role:tenant.admin` → granted (tenant-scoped view via application-layer filter)

No direct DB access by application roles beyond `awo_app`.

### 7.4 Tamper Evidence (v1)

Monthly SHA-256 integrity checkpoints are written to `platform_audit_checkpoint`:
- One checkpoint per partition per checkpoint interval
- Checkpoint = SHA-256 of all record IDs + timestamps in the partition at checkpoint time
- Allows detecting row deletions or insertions after checkpoint

Full hash chain (per-record cryptographic chaining) is deferred to v2.

---

## 8. Storage

Full storage specification, including DDL, partition management, indexes, and retention policy, is in [`12-audit/AUDIT_STORAGE.md`](AUDIT_STORAGE.md).

Summary:
- Table: `platform_audit_log`
- Partitioning: monthly range on `created_at`
- Bootstrap creates: current month + 2 months ahead
- Retention: never deleted by application (ops/PITR concern)
- GDPR anonymization: `audit_retention_role` can NULL specific fields via UPDATE

---

## 9. Events Not Captured by Pipeline (Audit Gaps)

The pipeline AUDIT RECORD stage covers entity Create/Update/Delete mutations only. The following events require explicit audit writes by their respective owners:

| Event | Owner | Method |
|-------|-------|--------|
| Login / logout | `AuthService` | `AuditWriter.Write` directly |
| Session expiry | `AuthService` (background job) | `AuditWriter.Write` directly |
| Permission denied | `RequirePermission` middleware | `AuditWriter.Write` directly |
| Custom action execution | `ActionDef` handler | **v1 gap — no automatic coverage** |
| Bootstrap migrations | migration runner | `AuditWriter.Write` with `SystemActor = SystemMigration` |
| Outbox relay dispatch | outbox worker | `AuditWriter.Write` with `SystemActor = SystemOutboxRelay` |

**Custom actions (v1 gap):** `ActionDef` handlers are not intercepted by the pipeline AUDIT RECORD stage. Custom action audit is the handler author's responsibility in v1. A framework-level solution (action interceptor) is deferred to v2.

---

## 10. Migration Strategy

Full migration plan is in [`12-audit/AUDIT_MIGRATION.md`](AUDIT_MIGRATION.md).

Summary of phases:

| Phase | Action | Gating Condition |
|-------|--------|-----------------|
| 1 | Create `platform_audit_log` + partitions + `platform_audit_config` | Migration runs unconditionally |
| 2 | Deploy `TransactionalWriter` with feature flag check (no-op when flag=false) | Flag = false; legacy still active |
| 3 | Enable unified system (flag=true); legacy system still running | Validate unified records are written |
| 4 | Historical migration: copy `iam_audit_log` + `audit_log` → `platform_audit_log` | Background job; non-blocking |
| 5 | Remove SQL trigger infrastructure + `iam_audit_log` entity | After Phase 4 complete |
| 6 | Remove `awo/platform/audit/definition.go`; repurpose package as read API | Final cleanup |

Rollback: Set feature flag back to false at any phase before Phase 5.

---

## 11. Operational Model

### 11.1 Metrics

| Metric | Type | Labels |
|--------|------|--------|
| `audit_write_total` | Counter | entity, category, operation, severity |
| `audit_write_failure_total` | Counter | entity, category, failure_policy |
| `audit_write_duration_seconds` | Histogram | entity, category |
| `audit_partition_rows_total` | Gauge | partition |
| `audit_risk_scorer_cache_age_seconds` | Gauge | — |

### 11.2 Alerts

| Alert | Condition | Severity |
|-------|-----------|----------|
| `AuditWriteFailureRatioHigh` | `rate(audit_write_failure_total[5m]) / rate(audit_write_total[5m]) > 0.01` | WARNING |
| `AuditPartitionMissing` | Next month's partition does not exist by 25th of current month | CRITICAL |
| `AuditRiskScorerStale` | `audit_risk_scorer_cache_age_seconds > 86400` | WARNING |

### 11.3 Partition Maintenance

Partitions must be created before the start of each month. The scheduled job (managed by the platform operations team, not the application) runs on the 25th of each month to create the following month's partition.

**Requires implementation verification:** No scheduler exists in the current framework. This is a gap. Options: (a) pg_cron extension, (b) Temporal scheduled workflow, (c) external cron. Decision required before implementation.

---

## 12. Future Extensions (Post-v1)

| Feature | Version | Description |
|---------|---------|-------------|
| Per-record hash chain | v2 | Cryptographic chaining for stronger tamper evidence |
| Custom action audit coverage | v2 | Framework-level action interceptor |
| Real-time audit streaming | v2 | Kafka/NATS topic per event category |
| Audit query API | v2 | Structured query interface for compliance tools |
| GDPR right-to-erasure workflow | v2 | Temporal workflow for bulk anonymization |
| Multi-region audit replication | v3 | Cross-region audit log sync |

---

## References

- [`12-audit/AUDIT_SPEC.md`](AUDIT_SPEC.md) — AuditWriter interface and AuditRecord schema
- [`12-audit/AUDIT_STORAGE.md`](AUDIT_STORAGE.md) — Storage DDL, partitioning, retention
- [`12-audit/AUDIT_MIGRATION.md`](AUDIT_MIGRATION.md) — Migration from legacy dual system
- [`02-pipeline/LIFECYCLE_SPEC.md`](../02-pipeline/LIFECYCLE_SPEC.md) — Pipeline with AUDIT RECORD stage
- [`00-overview/DECISION_REGISTER.md`](../00-overview/DECISION_REGISTER.md) — ADR-005, ADR-013 through ADR-019
- [`03-auth/SESSION_SPEC.md`](../03-auth/SESSION_SPEC.md) — Session model (SessionID HMAC derivation)
- [`04-multitenancy/RLS_SPEC.md`](../04-multitenancy/RLS_SPEC.md) — RLS policies (audit is global table)
- `awo/platform/audit/review.md` — Design rationale and engineering analysis
- `awo/platform/audit/review.v2.md` — Principal engineer review with blocker resolutions
