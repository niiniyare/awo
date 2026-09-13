# AWO Framework — Unified Audit System: Implementation Specification

**Status:** Definitive architecture specification
**Supersedes:** Legacy migrations 000448–000451, `awo/platform/audit` package (current)

---

## Part 0 — Foundational Decisions

### D1: Hook mechanism
**Decision: Repository wrapper (`AuditingRepository[T]`).**

Rejected: global hook registry (requires new unbuilt runtime primitive). Rejected: modifying HookSet
(couples audit to entity DSL). The `AuditingRepository[T]` implements `driver.EntityRepository[T]`,
wraps any inner repository, delegates all calls, and after each successful mutating call writes an
audit event. It receives the entity's `SystemDefinition` at construction (for sensitive field list)
and an `audit.Writer` via DI. Wire injects the wrapper transparently. No new framework primitives.

### D2: Writer synchrony
**Decision: `TransactionalWriter` — synchronous, no channels, no goroutines.**

Rejected: AsyncWriter with in-process channel (no durability guarantee on crash, no shutdown
coordination). Rejected: outbox-within-audit (audit is infrastructure, not domain events — must not
share write path). `TransactionalWriter` writes within the caller's transaction when one is present,
and via a dedicated pool connection when none is present.

### D3: Failure policy
**Decision: Per-category, deterministic — not runtime-configurable.**

| Category | Policy     | Rationale                                         |
|----------|------------|---------------------------------------------------|
| ADMIN    | Propagated | Admin action must abort if audit fails            |
| SECURITY | Propagated | Security event must be recorded or action blocked |
| DATA     | Suppressed | Business must not fail due to audit               |
| AUTH     | Suppressed | Login must succeed even if audit write fails      |
| ACCESS   | Suppressed | 403 must be returned even if audit write fails    |
| WORKFLOW | Suppressed | Temporal activities cannot roll back              |
| SYSTEM   | Suppressed | Bootstrap proceeds even if audit fails            |
| OUTBOUND | Suppressed | Outbox relay must continue                        |

### D4: API token audit
**Decision: Aggregation table for per-call volume; individual events for anomalies only.**

`platform_audit_token_usage` stores daily counters (UPSERT). Individual `auth.api_token.used`
events written to `platform_audit_log` only on: first-ever use, first use from new source IP,
first daily use, anomaly threshold breach.

### D5: Session audit location
**Decision: IAM service layer (`AuthService`), not `RedisSessionStore`.**

`RedisSessionStore` must not depend on `audit.Writer` (which depends on PostgreSQL). An outage in
PostgreSQL would make session operations fail. `AuthService` has full context and is the correct
location.

### D6: COMPLIANCE event category
**Decision: Removed. Express compliance via `compliance_flags JSONB` only.**

Eight categories: DATA, AUTH, ACCESS, ADMIN, WORKFLOW, SYSTEM, OUTBOUND, SECURITY.

### D7: AuditMode in EntityDefinition
**Decision: Removed from DSL. `AuditEnabled bool` only (default true).**

ADMIN and SECURITY events are always audited regardless of this flag.

### D8: Diff table
**Decision: JSONB-only for v1. Flat diff child table is v2 enterprise extension.**

### D9: Hash chain / tamper evidence
**Decision: v1 uses periodic SHA-256 checkpoints in `platform_audit_checkpoint`. Full hash chain
is v2 enterprise feature.**

### D10: SystemActor
**Decision: Required. String constants in `awo/audit`. Non-human execution contexts must produce
identifiable audit events.**

---

## Part 1 — Overall Architecture

### Package structure

```
awo/
└── audit/
    ├── audit.go          Package declaration, constants, initialization
    ├── event.go          EventType constants, Category constants, Severity constants
    ├── actor.go          Actor struct, ActorType enum, SystemActor constants
    ├── entry.go          Entry struct, validation
    ├── writer.go         Writer interface + failure policy
    ├── transactional.go  TransactionalWriter (primary implementation)
    ├── noop.go           NoopWriter (tests)
    ├── multi.go          MultiWriter (fan-out for SIEM extension)
    ├── scorer.go         RiskScorer interface + CachingRiskScorer
    ├── compliance.go     ComplianceDetector interface + DB-backed implementation
    ├── sanitizer.go      Strip sensitive fields, compute diff
    ├── repository.go     AuditingRepository[T] wrapper
    └── context.go        Context keys for actor propagation
```

`awo/audit` lives at `awo.so/awo/audit` — not `awo/platform/audit`. Audit is framework
infrastructure, not a platform module.

### Dependency graph

```
awo/audit
  → awo/def         EntityDefinition, FieldDef
  → awo/driver      EntityRepository interface (for AuditingRepository type)
  → awo/cache       RiskScorer cache
  → pgx/v5          write path
  → slog            failure logging

awo/platform/iam    → awo/audit   auth event writes
awo/api/middleware  → awo/audit   ACCESS DENY writes
awo/api/router      → awo/audit   AuditingRepository construction
awo/contrib/pgx     → awo/audit   entity definition to AuditingRepository
awo/bootstrap       → awo/audit   init Writer, RiskScorer, ComplianceDetector
```

`awo/audit` does NOT import: `awo/auth`, `awo/platform/iam`, `awo/contrib/redis`, any business
module.

### Initialization sequence

1. PostgreSQL pool available (bootstrap phase 2)
2. Redis client available (bootstrap phase 3)
3. `audit.Init(pool, cache)` — warms RiskScorer and ComplianceDetector caches **synchronously**.
   Bootstrap blocks until warm. Prevents first writes being scored incorrectly.
4. `audit.NewWriter(pool, failurePolicies)` registered in Wire
5. HTTP server starts (bootstrap phase 6)

Cache warm-up: `SELECT * FROM audit_sensitive_tables; SELECT * FROM audit_sensitive_fields;`
These tables have tens to low hundreds of rows. Cold start: under 10ms.

### Shutdown

`TransactionalWriter` is stateless — no goroutines, no channels, no buffers. No shutdown
coordination needed. pgx pool handles connection drain on `pool.Close()`.

---

## Part 2 — Hook Integration

### AuditingRepository[T]

Wire produces:
```
pgxpool.Pool
  → contrib/pgx.Repository[T]           (inner repo)
  → audit.AuditingRepository[T]         wraps inner repo
  → handler receives AuditingRepository as driver.EntityRepository[T]
```

Handlers call `repo.Create(ctx, input)` — audit happens inside wrapper before returning. No
handler changes required.

### Sequences

**Create:**
```
Handler → AuditingRepository.Create(ctx, input)
  → inner.Create(ctx, input) → record, err
  → if err != nil: return err (no audit — entity not created)
  → strippedAfter = sanitizer.Strip(record, sensitiveFields)
  → riskScore = scorer.Score(entry)
  → flags = compliance.Flags(entityName, nil, strippedAfter)
  → writer.Write(ctx, Entry{
        EventType: EventEntityCreated, Category: CategoryDATA,
        Actor: ActorFromContext(ctx), TenantID: TenantFromContext(ctx),
        EntityName: def.Name, RecordID: record.ID,
        After: strippedAfter, RiskScore: riskScore, ComplianceFlags: flags,
    })
  → if write error: log + meter (DATA = suppressed)
← record, nil
```

**Update:**
```
Handler → AuditingRepository.Update(ctx, id, patch)
  → inner.Get(ctx, id) → beforeRecord
  → inner.Update(ctx, id, patch) → afterRecord, err
  → if err != nil: return err
  → strippedBefore = sanitizer.Strip(beforeRecord, sensitiveFields)
  → strippedAfter  = sanitizer.Strip(afterRecord, sensitiveFields)
  → diff = sanitizer.Diff(strippedBefore, strippedAfter)   ← AFTER stripping
  → if diff is empty AND not ForceAuditUpdates: skip write
  → writer.Write(ctx, Entry{EventEntityUpdated, Before: strippedBefore,
                             After: strippedAfter, Diff: diff, ...})
← afterRecord, nil
```

One extra `Get` per update for the before snapshot. At ERP scale (100 TPS) = 100 extra reads/sec —
acceptable. PostgreSQL `UPDATE ... RETURNING` with before-state CTE is the optimization path if
this becomes a bottleneck.

**Delete:**
```
Handler → AuditingRepository.Delete(ctx, id)
  → inner.Get(ctx, id) → beforeRecord
  → inner.Delete(ctx, id) → err
  → if err != nil: return err
  → strippedBefore = sanitizer.Strip(beforeRecord, sensitiveFields)
  → writer.Write(ctx, Entry{EventEntityDeleted, Before: strippedBefore, ...})
← nil
```

**Soft delete:** Treated as Update (sets `deleted_at`). EventType: `entity.updated`. Diff shows
`{"deleted_at": {"old": null, "new": "..."}}`.

**Hard delete:** Sequence above. Before snapshot is the only remaining record of deleted data.

**Restore:** Treated as Update (sets `deleted_at = NULL`). Context: `{"restore": true}`.

**Bulk operations:**
```
Handler → AuditingRepository.BulkUpdate(ctx, filter, patch)
  → inner.BulkUpdate(ctx, filter, patch) → count, err
  → if err != nil: return err
  → writer.Write(ctx, Entry{
        EventType: EventEntityBulkUpdated, Category: CategoryDATA,
        EntityName: def.Name,
        Context: {"filter": filter, "patch": strippedPatch, "affected_count": count},
    })
← count, nil
```

One summary event. No per-row events. `patch` has sensitive keys stripped before inclusion in
context.

**Reads:** Not audited at DATA level by default. `AuditReads: true` in EntityDefinition enables
ACCESS-category read events for sensitive entities (v2 feature).

---

## Part 3 — Writer Architecture

### Interface

```go
type Writer interface {
    Write(ctx context.Context, e Entry) error
}
```

Single method. The `ctx` carries the pgx transaction (if present), tenant ID, request ID.
Whether the caller propagates or suppresses the returned error is determined by the failure policy
**inside** the `TransactionalWriter`, not by the call site. Call sites never implement suppression
logic.

### TransactionalWriter fields

```
pool         *pgxpool.Pool
policies     map[Category]FailurePolicy
scorer       RiskScorer
compliance   ComplianceDetector
sanitizer    *Sanitizer
logger       *slog.Logger
metrics      WriterMetrics
```

### Transaction behavior

Writer inspects context for a pgx transaction. If present: writes inside that transaction (audit
row committed/rolled back with caller's transaction). If absent: acquires dedicated pool connection
and writes as standalone INSERT.

- DATA events: share entity transaction. Rollback erases audit row. Correct.
- AUTH events: no transaction → standalone INSERT.
- ACCESS events: no transaction (fires in middleware before handler) → standalone INSERT.
- ADMIN events: share role-assignment transaction.

### Connection management

`pool.Acquire(ctx)` with 500ms timeout for standalone writes. Pool reserves minimum 2 connections
for audit. Prevents audit writes competing with business queries for last connection.

### Idempotency

`Entry.ClientEventID uuid.UUID` — if set, used as the row's `id`. Duplicate primary key on retry
returns nil (event already recorded). If zero: `gen_random_uuid()` — no idempotency guarantee.
Temporal activities always set `ClientEventID` from deterministic workflow context.

### Retry policy

No automatic retries. Failed write returns immediately. Retries at caller's discretion.
Automatic retry inside `Write` risks adding latency when DB is already overloaded.

### Thread safety

Safe for concurrent use. Each `Write` call acquires its own connection or uses the context
transaction. Scorer and compliance detector use read-only cached data. Sanitizer is stateless.
No shared mutable state.

### Failure execution path

```
1. Validate Entry (required fields)
2. Strip sensitive fields, compute diff (sanitizer)
3. Score risk (scorer)
4. Detect compliance flags (compliance detector)
5. Execute INSERT
6. Success: increment success counter; return nil
7. Failure:
   a. policy = policies[entry.Category]
   b. slog.Error(...) — always logged
   c. Increment awo_audit_write_errors_total{category, error_type} — always metered
   d. FailPropagated: return error
   e. FailSuppressed: return nil
```

---

## Part 4 — Audit Storage

### Table: `platform_audit_log`

```sql
CREATE TABLE platform_audit_log (
  -- Identity
  id               UUID         NOT NULL DEFAULT gen_random_uuid(),
  schema_version   SMALLINT     NOT NULL DEFAULT 1,

  -- Tenant (NULL for system/bootstrap events; no FK — audit must survive tenant deletion)
  tenant_id        UUID,

  -- Event classification (all derived by Writer from EventType constant — callers do not set)
  event_type       VARCHAR(100) NOT NULL,
  event_category   VARCHAR(20)  NOT NULL,
  operation        VARCHAR(20)  NOT NULL,
  severity         VARCHAR(10)  NOT NULL DEFAULT 'INFO',

  -- Actor
  actor_type       VARCHAR(20)  NOT NULL,  -- 'user','service_account','api_token','system'
  actor_user_id    UUID,        -- bare UUID, no FK (GDPR anonymization nulls this)
  actor_sa_id      UUID,        -- bare UUID, no FK
  actor_token_id   UUID,        -- bare UUID, no FK
  actor_ip         INET,        -- native INET for network-range queries
  actor_session    VARCHAR(64), -- HMAC-SHA256 of session token (not raw SHA-256)

  -- Subject
  entity_name      VARCHAR(100),
  record_id        UUID,        -- UUID type, not TEXT, for efficient index lookups

  -- Risk and compliance
  risk_score       SMALLINT     NOT NULL DEFAULT 0,   -- 0-100; SMALLINT saves 2B vs INT
  compliance_flags JSONB        NOT NULL DEFAULT '{}',

  -- Authorization decision (ACCESS events only)
  decision         VARCHAR(5),  -- 'ALLOW', 'DENY', NULL

  -- Full payload (see context schema by event type below)
  context          JSONB        NOT NULL DEFAULT '{}',

  -- Partition key
  created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW()

) PARTITION BY RANGE (created_at);
```

**Column rationale (key points):**

`id UUID` — random UUID prevents sequence-gap tampering detection attacks.

`schema_version SMALLINT` — present from day one. Starts at 1. Historical migrated rows from
`iam_audit_log` receive `schema_version = 0`. Version 1+ rows use this specification.

`tenant_id UUID` — no FK. Audit must survive tenant deletion (GDPR anonymization, not cascade).
FK with ON DELETE CASCADE destroys compliance history. FK with no-action blocks tenant deletion.

`event_category / operation` — both derived by Writer. `operation` is low-cardinality
(15 distinct values vs hundreds of event types) enabling efficient index-based queries
("all deletes in tenant X") using `WHERE operation = 'DELETE'`.

`actor_user_id / actor_sa_id / actor_token_id` — no FK constraints. GDPR anonymization nulls
these fields without violating referential integrity.

`actor_ip INET` — native INET type enables `WHERE actor_ip << '10.0.0.0/8'` network queries.

`actor_session VARCHAR(64)` — HMAC-SHA256 of session token using server-side secret. HMAC (not
raw SHA-256) prevents rainbow-table attacks against known token values.

`record_id UUID` — UUID type (not TEXT) for native UUID comparison in indexes.

`risk_score SMALLINT` — 2 bytes sufficient for 0-100 range. Saves 2GB vs INTEGER at 1B rows.

`compliance_flags JSONB` — extensible to any regulatory framework without DDL changes.

`decision VARCHAR(5)` — 'ALLOW'(5), 'DENY'(4), NULL. String not boolean: NULL = "not applicable"
is unambiguous. Readable in forensic reports.

No `updated_at` or `deleted_at` — absence enforces immutability at schema level.

### Context JSONB schema by event type (schema_version = 1)

**DATA events (entity.created / entity.updated / entity.deleted):**
```json
{
  "before": {},
  "after": {},
  "diff": {"field_name": {"old": "...", "new": "..."}},
  "request": {"id": "...", "method": "POST", "path": "/api/v1/..."},
  "restore": false
}
```

**AUTH events:**
```json
{
  "method": "password|oauth|api_key|pat",
  "failure_reason": "invalid_credentials|account_locked|rate_limited",
  "attempted_email_hash": "hmac(...)",
  "request": {"id": "...", "user_agent": "...", "origin": "..."}
}
```

**ACCESS events:**
```json
{
  "permission_id": "finance.invoice.delete",
  "entity_name": "finance_invoice",
  "record_id": "...",
  "decision_reason": "role 'viewer' lacks 'finance.invoice.delete'"
}
```

**ADMIN events:**
```json
{
  "subject_type": "user|role|permission|policy",
  "subject_id": "...",
  "change": {"added": [], "removed": []},
  "reason": "promoted by tenant owner"
}
```

**WORKFLOW events:**
```json
{
  "workflow_id": "...",
  "workflow_type": "finance.invoice.submit",
  "activity_id": "...",
  "run_id": "...",
  "initiator_user_id": "...",
  "attempt": 1
}
```

**SYSTEM events:**
```json
{
  "system_actor": "framework.bootstrap|framework.migration|...",
  "details": "migration 000512 applied",
  "version": "1.2.3"
}
```

**OUTBOUND events:**
```json
{
  "destination_hash": "sha256(webhookURL)",
  "event_name": "finance.invoice.posted",
  "delivery_attempt": 1,
  "response_status": 200
}
```

**SECURITY events:**
```json
{
  "threat_type": "rate_limit_exceeded|csrf_rejected",
  "threshold": {"limit": 100, "window_seconds": 60, "count": 150},
  "blocked": true
}
```

### Indexes

```sql
-- Primary access pattern: tenant + time
CREATE INDEX idx_pal_tenant_time
  ON platform_audit_log (tenant_id, created_at DESC);

-- Operation filter (partial: only high-signal operations)
CREATE INDEX idx_pal_tenant_operation
  ON platform_audit_log (tenant_id, operation, created_at DESC)
  WHERE operation IN ('DELETE','LOGIN','LOGOUT','BLOCK','FAIL');

-- Actor queries: "what did user X do?"
CREATE INDEX idx_pal_actor_user
  ON platform_audit_log (actor_user_id, created_at DESC)
  WHERE actor_user_id IS NOT NULL;

-- Service account queries
CREATE INDEX idx_pal_actor_sa
  ON platform_audit_log (actor_sa_id, created_at DESC)
  WHERE actor_sa_id IS NOT NULL;

-- Entity record history: "audit trail for record Y"
CREATE INDEX idx_pal_record
  ON platform_audit_log (tenant_id, entity_name, record_id, created_at DESC)
  WHERE record_id IS NOT NULL;

-- High-severity alert filter (partial: most rows are not HIGH/CRITICAL)
CREATE INDEX idx_pal_severity
  ON platform_audit_log (tenant_id, severity, created_at DESC)
  WHERE severity IN ('HIGH','CRITICAL');

-- Risk threshold queries (partial: most rows have risk_score < 30)
CREATE INDEX idx_pal_risk
  ON platform_audit_log (tenant_id, risk_score DESC, created_at DESC)
  WHERE risk_score >= 30;

-- Compliance flag containment (GIN, partial: most rows have empty flags)
CREATE INDEX idx_pal_compliance
  ON platform_audit_log USING GIN (compliance_flags)
  WHERE compliance_flags <> '{}'::jsonb;
```

Indexes defined on parent table — PostgreSQL propagates to all partitions automatically.
`event_type` not indexed directly (covered by `operation`).
`actor_session` not indexed (forensic tool, ad-hoc query only).

### Partitioning

Monthly RANGE partitions on `created_at`:
```sql
CREATE TABLE platform_audit_log_y2026m07
  PARTITION OF platform_audit_log
  FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');
```

Bootstrap creates partitions for current month + 2 months ahead (idempotent IF NOT EXISTS check).
Maintenance job creates next 3 months' partitions on the 1st of each month.

### Retention

Per-category defaults stored in `platform_audit_config`:

| Category | Default retention |
|----------|-------------------|
| DATA     | 7 years (2555 days) |
| ADMIN    | 7 years |
| SECURITY | 7 years |
| AUTH     | 1 year |
| ACCESS   | 1 year |
| WORKFLOW | 1 year |
| SYSTEM   | 180 days |
| OUTBOUND | 90 days |

Partition retention = max(retention across categories present in partition). Mixed-category
partitions are retained for the longest category's period. Operational simplicity over storage
optimization.

```sql
CREATE TABLE platform_audit_config (
  key        TEXT        NOT NULL PRIMARY KEY,
  value      TEXT        NOT NULL,
  tenant_id  UUID,       -- NULL = global default
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- key format: "retention.{CATEGORY}.days"
```

### Archive and integrity verification

Archive format: gzip-compressed JSONL, one row per line.
File name: `platform_audit_log_y{YYYY}m{MM}_{sha256_first8}.jsonl.gz`

Integrity checkpoint table:
```sql
CREATE TABLE platform_audit_checkpoint (
  partition_name  TEXT        NOT NULL PRIMARY KEY,
  row_count       BIGINT      NOT NULL,
  sha256_hash     VARCHAR(64) NOT NULL,
  archived_at     TIMESTAMPTZ NOT NULL,
  archive_uri     TEXT        NOT NULL,
  verified_at     TIMESTAMPTZ,
  verified_ok     BOOLEAN
);
-- RLS: INSERT (system_role only), SELECT (all). NO UPDATE or DELETE.
```

Verification job runs 7 days after archival: downloads archive, recomputes SHA-256, updates
`verified_ok`. Alert if `verified_ok = FALSE`.

Maintenance sequence per partition:
1. Compute SHA-256 of partition data (full SELECT → JSONL)
2. Upload to cold storage
3. Write checkpoint row
4. Verify (7 days later): download + recompute hash
5. DROP TABLE partition after verification confirmed

### Schema evolution

`schema_version` in each row. When `context` JSONB structure changes: increment constant.
Old rows queryable with old parsers using `WHERE schema_version = 1`.
DDL changes: add columns as nullable with defaults only.

---

## Part 5 — Performance Architecture

### Write amplification

1 entity CRUD → 1 audit write. 2× total writes. Both in same transaction.
Audit write overhead per operation: ~2–5ms under normal load (index maintenance dominant).

### TPS scenarios

| TPS (entity ops) | Audit writes/sec | PostgreSQL load | Recommendation |
|-----------------|-----------------|-----------------|----------------|
| 10              | 10              | Negligible      | Standard config |
| 100             | 100             | Low-moderate    | Monitor p99 latency |
| 1,000           | 1,000           | Significant     | `synchronous_commit=off` for DATA; 5 dedicated audit connections |
| 10,000          | 10,000          | Exceeds single instance | Separate audit PostgreSQL instance via `OutboxWriter` |

At 1,000 TPS: `synchronous_commit = off` for audit pool connection. Accepts ~100ms loss window
on crash for DATA-category events. ADMIN/SECURITY always use `synchronous_commit = on`.

At 800 TPS sustained: begin planning separate audit PostgreSQL instance. `OutboxWriter`
implementation replaces `TransactionalWriter` — interface unchanged.

### Bulk imports

Import handlers bypass `AuditingRepository` for the per-row loop, use inner repository directly,
then write one `entity.bulk_imported` summary event at completion. Framework provides
`BulkImportContext` wrapper to signal this pattern.

### API token validation

Aggregation UPSERT to `platform_audit_token_usage` — not per-call INSERT. Individual event only
on anomaly (new IP, daily first-use, threshold breach). This reduces write volume by orders of
magnitude for high-frequency token validation.

### Optimization order (by deployment trigger)

1. 200 TPS: `synchronous_commit = off` for DATA-category audit pool connection
2. 500 TPS: remove GIN compliance index if compliance queries not in use
3. 800 TPS: convert `idx_pal_record` to BRIN (append-only data, adequate selectivity)
4. 1,000 TPS: separate audit PostgreSQL instance + `OutboxWriter`

---

## Part 6 — Authentication Auditing

### Event log vs aggregated metric

| Flow | Model | Rationale |
|------|-------|-----------|
| Login success | Event log | Per-attempt forensics required |
| Login failure | Event log | Security forensics, lockout analysis |
| Logout | Event log | Session lifecycle |
| Session creation | Event log | Discrete security event |
| Session deletion (single) | Event log | Revocation is discrete |
| Session deletion (bulk) | Event log | One summary with count |
| Password reset (requested) | Event log | Security forensics |
| Password reset (completed) | Event log | ADMIN category |
| Password change | Event log | HIGH severity |
| API token validation (routine) | Aggregated | Volume prohibits per-call events |
| API token validation (anomaly) | Event log | New IP, daily first-use, threshold |
| API token issuance | Event log | Security event |
| API token revocation | Event log | Security event |
| Account lockout triggered | Event log | SECURITY category |
| Account unlocked | Event log | ADMIN category |

All auth events written in `AuthService`. Not in middleware. Not in `RedisSessionStore`.

### Login success
```
EventType: EventAuthLoginSuccess
Category:  CategoryAUTH
Actor:     Actor{Type: ActorUser, UserID: &userID, IP: ip}
TenantID:  tenantID
Severity:  INFO
Context:   {"method": "password", "request": {...}}
Policy:    Suppressed (login proceeds on write failure)
```

### Login failure
```
EventType: EventAuthLoginFailed
Category:  CategorySECURITY
Actor:     Actor{Type: ActorSystem, IP: ip}  // no UserID — credentials unverified
TenantID:  tenantID
Severity:  WARN (CRITICAL if account locked)
Context:   {"method": "password", "failure_reason": "invalid_credentials",
            "attempted_email_hash": hmac(email)}
```

`actor_user_id = NULL` on failure — cannot attribute attempt to a user (may not exist).
`attempted_email_hash` = HMAC of provided email for correlation without storing plaintext.

### Logout
```
writer.Write(ctx, Entry{EventAuthLogout, ...})
// Written BEFORE SessionStore.Delete()
// If session delete fails: audit record exists for investigation
// If audit write fails (suppressed): logout still proceeds
SessionStore.Delete(ctx, session)
```

### API token aggregation
```sql
INSERT INTO platform_audit_token_usage
  (token_id, tenant_id, usage_date, call_count, last_used_at, last_ip)
VALUES ($1, $2, CURRENT_DATE, 1, NOW(), $3)
ON CONFLICT (token_id, usage_date) DO UPDATE SET
  call_count   = platform_audit_token_usage.call_count + 1,
  last_used_at = NOW(),
  last_ip      = EXCLUDED.last_ip;
```

Individual `platform_audit_log` event written when:
- `call_count` was 0 before (first use today) → INFO
- `last_ip` changed → WARN (new source IP)
- Rate threshold exceeded → HIGH

### Account lockout
```
EventType: EventSecurityAccountLockout
Category:  CategorySECURITY
Severity:  HIGH
Operation: OpBLOCK
Context:   {"failed_attempts": 10, "window_seconds": 300, "lockout_duration_seconds": 900}
```

---

## Part 7 — Authorization Auditing

### Permission denied

Written in `RequirePermission` middleware after Casbin returns false. Written BEFORE 403 returned.
```
EventType:  EventAuthzDeny
Category:   CategoryACCESS
Operation:  OpBLOCK
Severity:   WARN (HIGH if same permission denied 5+ times in 1 minute)
Decision:   "DENY"
Context:    {"permission_id": ..., "entity_name": ..., "record_id": ...,
             "decision_reason": "role 'X' lacks 'Y'"}
Policy:     Suppressed (403 returned regardless of audit write result)
```

### Permission granted

Not audited by default. Audited when:
- Reading `platform_audit_log` itself (always)
- Entity has `AuditReads: true` (v2 feature)
- Platform admin accesses any entity in any tenant → ACCESS-ALLOW, INFO

### Role assignment

`RoleAssignmentService.AssignRole()` produces two events in the same transaction:

1. Generic entity event from `AuditingRepository.Create()` → `entity.created` with before/after
2. Explicit ADMIN event with human-readable forensic summary:
```
EventType: EventAdminRoleAssigned
Category:  CategoryADMIN
Severity:  HIGH
Context:   {"subject_user_id": ..., "role_id": ..., "role_name": ..., "assigned_by": ...}
Policy:    Propagated (role assignment aborts if ADMIN event cannot be written)
```

### Privilege escalation

Detected in service layer before assignment committed. If assigning user grants permissions
exceeding their own:
```
EventType: EventSecurityPrivilegeEscalation
Category:  CategorySECURITY
Severity:  CRITICAL
Context:   {"escalated_permissions": [...]}
```

Transaction rolled back. SECURITY event written on separate connection (original tx already
rolling back, event must still be recorded). Policy: Propagated — escalation blocked if event
cannot be written.

### Emergency access (platform admin cross-tenant)

```
EventType: EventAdminEmergencyAccess
Category:  CategoryADMIN
Severity:  CRITICAL
Context:   {"accessed_tenant_id": ..., "reason": "...", "approved_by": "..."}
Policy:    Propagated — emergency access blocked if audit write fails
```

---

## Part 8 — Background Execution Auditing

### SystemActor model

```go
type ActorType    string
type SystemActorID string

const (
    ActorUser           ActorType = "user"
    ActorServiceAccount ActorType = "service_account"
    ActorAPIToken       ActorType = "api_token"
    ActorSystem         ActorType = "system"
)

const (
    SystemBootstrap      SystemActorID = "framework.bootstrap"
    SystemMigration      SystemActorID = "framework.migration"
    SystemOutboxRelay    SystemActorID = "framework.outbox_relay"
    SystemTemporalWorker SystemActorID = "framework.temporal_worker"
    SystemScheduler      SystemActorID = "framework.scheduler"
    SystemAuditRetention SystemActorID = "framework.audit_retention"
    SystemAuditPartition SystemActorID = "framework.audit_partition"
)
```

System actors: `actor_user_id = NULL`, `actor_sa_id = NULL`, `actor_token_id = NULL`,
`actor_type = 'system'`. The `context.system_actor` carries the `SystemActorID` string.

### Context propagation

```go
// Call at the start of every background goroutine loop iteration:
ctx = audit.WithSystemActor(ctx, audit.SystemOutboxRelay)
// TransactionalWriter reads actor via audit.ActorFromContext(ctx)
```

### Temporal

Temporal worker initializes its own `audit.Writer` at startup using a separate pgx pool (audit
pool). Worker does not share the business pool.

Workflow audit events require `InitiatorUserID uuid.UUID` in all workflow input structs
(framework convention). This provides the human initiator identity for WORKFLOW events even
though the actor is `SystemTemporalWorker`.

Activity failures:
```
EventType: EventWorkflowActivityFailed
Severity:  WARN (retrying) or HIGH (terminal)
Context:   {"attempt": N, "max_attempts": M, "error": "..."}
```

### Bootstrap

At end of `bootstrap.Run()`, before HTTP server starts:
```
EventType: EventSystemBootstrap
Category:  CategorySYSTEM
Actor:     SystemActor(SystemBootstrap)
TenantID:  NULL
Context:   {"version": "1.2.3", "migration_count": 42, "duration_ms": 2340}
```

Uses `system_role` PostgreSQL connection. `tenant_id = NULL`.

### Migration runner

```
EventType: EventSystemMigration
Category:  CategorySYSTEM
Actor:     SystemActor(SystemMigration)
TenantID:  NULL
Context:   {"migration_id": "000512", "description": "...", "direction": "up", "duration_ms": 450}
```

---

## Part 9 — Sensitive Data Handling

### Definitive ordering

Executed by `TransactionalWriter.Write()` for any event with Before/After data:

```
1. Receive Entry{Before: rawBefore, After: rawAfter}
2. sensitiveFields = union(
     entityDef.SensitiveFieldNames(),       // FieldDef.Sensitive == true
     scorer.CachedSensitiveFieldNames(),    // from audit_sensitive_fields config table
   )
3. strippedBefore = sanitizer.Strip(rawBefore, sensitiveFields)
4. strippedAfter  = sanitizer.Strip(rawAfter, sensitiveFields)
5. diff = sanitizer.Diff(strippedBefore, strippedAfter)   ← computed AFTER stripping
6. context["before"] = strippedBefore
7. context["after"]  = strippedAfter
8. context["diff"]   = diff
9. INSERT into platform_audit_log
```

`ComputeDiff` is internal to the sanitizer. NOT exposed as a public API. Callers pass raw
Before/After. The sanitizer owns stripping and diffing exclusively. This is not optional — the
only way to guarantee sensitive field names never appear as diff keys.

### Sanitizer.Strip

Input: `any`. Output: `map[string]any`.

1. If struct: reflect to `map[string]any` using lowercase snake_case field names
2. For each key: if key in `sensitiveFields`, replace value with `"[REDACTED]"`
3. Recurse one level deep for nested objects only
4. Return sanitized map

One-level recursion covers embedded sub-objects. Full recursion is skipped (expensive, infinite
risk on self-referential structures, not needed for flat EntityDefinition field lists).

### Encrypted fields

`Encrypted: true` fields: store ciphertext in snapshot. Auditor sees ciphertext changed but
cannot read value without decryption key. Correct — audit trail is not the decryption facility.

`Sensitive: true AND Encrypted: true`: replaced with `"[ENCRYPTED+REDACTED]"`. No ciphertext
in audit trail.

### Global sensitive field names (seeded by platform migration)

`password_hash`, `token_hash`, `secret`, `api_key_hash`, `private_key`, `password`, `token`,
`refresh_token`, `credit_card`, `card_number`, `cvv`, `card_expiry`, `ssn`, `passport_number`,
`national_id`, `date_of_birth`.

These are stripped regardless of EntityDefinition. Config table is the backstop for fields that
entity authors forget to mark.

### GDPR anonymization

`audit.AnonymizeForUser(ctx, pool, userID uuid.UUID) error`:
- Sets `actor_user_id = NULL` on all rows where `actor_user_id = userID`
- Removes `actor_ip` from context JSON for those rows
- Removes `actor_session` for those rows
- Does NOT delete rows — deleting audit history is itself a compliance violation
- Requires `audit_retention_role` (only role with UPDATE permission)

Anonymization itself produces an ADMIN audit event:
```
EventType: EventAdminGDPRErasure
Category:  CategoryADMIN
Severity:  HIGH
Context:   {"erased_user_id_hash": hmac(userID), "rows_anonymized": N}
```

---

## Part 10 — RLS Integration

### Complete RLS policy set

```sql
ALTER TABLE platform_audit_log ENABLE ROW LEVEL SECURITY;
ALTER TABLE platform_audit_log FORCE  ROW LEVEL SECURITY;

-- Application INSERT: tenant-scoped (contrib/pgx sets current_tenant_id() per connection)
CREATE POLICY pal_app_insert ON platform_audit_log
  FOR INSERT TO application_role
  WITH CHECK (
    current_tenant_id() IS NOT NULL
    AND tenant_id = current_tenant_id()
  );

-- System INSERT: unrestricted (bootstrap, migration, system events with NULL tenant_id)
CREATE POLICY pal_system_insert ON platform_audit_log
  FOR INSERT TO system_role
  WITH CHECK (TRUE);

-- Application SELECT: tenant-scoped
CREATE POLICY pal_app_select ON platform_audit_log
  FOR SELECT TO application_role
  USING (
    current_tenant_id() IS NOT NULL
    AND tenant_id = current_tenant_id()
  );

-- Admin SELECT: cross-tenant (platform admin investigation)
-- Go layer additionally gates this behind "platform.audit_log.read" permission check
CREATE POLICY pal_admin_select ON platform_audit_log
  FOR SELECT TO admin_role
  USING (TRUE);

-- System SELECT: unrestricted (integrity jobs, retention jobs)
CREATE POLICY pal_system_select ON platform_audit_log
  FOR SELECT TO system_role
  USING (TRUE);

-- Retention DELETE: exclusively audit_retention_role
CREATE POLICY pal_retention_delete ON platform_audit_log
  FOR DELETE TO audit_retention_role
  USING (TRUE);

-- Retention UPDATE: GDPR anonymization only
-- Only UPDATE permitted is nulling actor fields. audit_retention_role is the only principal.
CREATE POLICY pal_retention_update ON platform_audit_log
  FOR UPDATE TO audit_retention_role
  USING (TRUE)
  WITH CHECK (TRUE);
```

No UPDATE or DELETE for `application_role` or `admin_role`. Absence of policy = denied.

### GRANT statements

```sql
GRANT INSERT, SELECT          ON platform_audit_log TO application_role;
GRANT SELECT                  ON platform_audit_log TO admin_role;
GRANT SELECT                  ON platform_audit_log TO readonly_role;
GRANT INSERT, SELECT          ON platform_audit_log TO system_role;
GRANT SELECT, UPDATE, DELETE  ON platform_audit_log TO audit_retention_role;
```

### NULL tenant_id

Events with `tenant_id = NULL` are visible only to `system_role` and `admin_role`.
`application_role` policy includes `current_tenant_id() IS NOT NULL` — when this returns NULL
(misconfigured connection), policy evaluates to FALSE. Application role sees NO rows.
Misconfigured connection cannot accidentally read system events.

### Platform admin read

Go layer detects `viewer.IsPlatformAdmin() == true` and runs `SET LOCAL ROLE admin_role` for
the audit query connection. RLS `pal_admin_select` policy then applies (cross-tenant SELECT).

### Support tables RLS

`platform_audit_checkpoint`: INSERT (system_role), SELECT (all). No UPDATE/DELETE.
`platform_audit_config`: SELECT (application_role, admin_role), INSERT/UPDATE (admin_role). No DELETE.
`platform_audit_token_usage`: INSERT/UPDATE (application_role own tokens), SELECT (application_role own, admin_role cross-tenant). No DELETE.
`audit_sensitive_tables` / `audit_sensitive_fields`: SELECT (application_role), INSERT/UPDATE (system_role for migration-time registration).

---

## Part 11 — Operational Model

### Metrics

```
awo_audit_writes_total{category, operation, severity}    Counter
awo_audit_write_errors_total{category, error_type}       Counter
awo_audit_write_latency_seconds{category}                Histogram [1ms,5ms,10ms,50ms]
awo_audit_partition_row_count{partition}                 Gauge (maintenance job)
awo_audit_partition_age_days{partition}                  Gauge
awo_audit_checkpoint_verified{partition, status}         Gauge (1=ok, 0=failed)
awo_audit_retention_deleted_total{partition}             Counter
awo_audit_token_anomalies_total{tenant_id}               Counter
```

### Alerts

| Alert | Condition | Severity | Action |
|-------|-----------|----------|--------|
| AuditWriteErrorRate | rate(write_errors[5m]) > 0.01 | Warning | Investigate DB connectivity |
| AuditWriteErrorBurst | rate(write_errors[1m]) > 0.1 | Critical | PagerDuty — audit gap in progress |
| AuditPartitionMissing | Next month's partition absent 7 days before month-end | Critical | Create partition immediately |
| AuditCheckpointFailed | verified{status="failed"} > 0 | Critical | Security team — potential tampering |
| AuditRetentionStale | Last retention job > 35 days ago | Warning | Maintenance job failed |
| AuditWriteLatencyHigh | p99 > 50ms | Warning | Index bloat or DB overload |

### Health check

`/health/ready` verifies `platform_audit_log` is writable: attempt INSERT with ROLLBACK.
If INSERT fails (wrong role, missing partition, RLS misconfiguration): return 503.

Check runs at startup (blocks server start if fails) and every 60 seconds.

### Failure recovery

DB outage (15 minutes):
- DATA events: suppressed, logged. Gap is permanently unrecoverable.
- ADMIN/SECURITY events: propagated — these operations fail during outage. Intended behavior.
- On recovery: SYSTEM event written: `EventSystemAuditGapDetected` with
  `context.gap_start`, `context.gap_end`, `context.estimated_missing_events`.

### Logging

Failed audit writes:
```json
{
  "level": "ERROR",
  "msg": "audit write failed",
  "event_type": "entity.created",
  "category": "DATA",
  "entity_name": "finance_invoice",
  "error": "connection pool exhausted",
  "suppressed": true,
  "duration_ms": 501
}
```

---

## Part 12 — Migration Strategy

### Feature flag

All migration stages controlled by `platform_audit_config`:
```
key: "feature.unified_audit.enabled"      value: "false"
key: "feature.unified_audit.phase"        value: "disabled|canary|staged|full"
key: "feature.unified_audit.auth_events"  value: "false"
key: "feature.unified_audit.access_events" value: "false"
```

`AuditingRepository` wrapper checks flag at construction (60-second TTL cache).
When `false`: wrapper delegates without writing — zero overhead.

### Phase 0 — Pre-migration (DB only, no code deployment)

Migrations:
- `000500_platform_audit_log.up.sql` — main table, partitions (current + 2 months), RLS, indexes
- `000501_platform_audit_support.up.sql` — token_usage, checkpoint, config tables, default config
- `000502_audit_retention_role.up.sql` — idempotent; verify grants on new table

Set feature flag: `enabled = false`.
Verify: `SELECT * FROM platform_audit_log LIMIT 1` succeeds from `application_role`.

### Phase 1 — Code deployment (backward-compatible)

- Deploy `awo/audit` package
- Wire DI: `AuditingRepository` wrappers present but feature flag = false → no-op
- Old `platform/audit/Writer` struct remains live
- Verify: zero rows written to `platform_audit_log`

### Phase 2 — Canary

- Set `phase = "canary"` → enables for `platform_settings` entity only
- Monitor 24 hours: `awo_audit_writes_total` increasing, errors = 0
- Rollback: set `enabled = false`

### Phase 3 — Staged rollout (entity CRUD)

- Set `enabled = true`
- All `AuditingRepository` wrappers activate
- Old `platform/audit/Writer` still live for backward compat
- Duration: one release cycle (2 weeks)
- Rollback: set `enabled = false`

### Phase 4 — Auth and access events

- Set `auth_events = true`, `access_events = true`
- Auth event writes deployed in `AuthService`
- DENY event writes deployed in `RequirePermission` middleware
- Monitor: `awo_audit_writes_total{category="AUTH"}` and `{category="ACCESS"}` increasing
- Rollback: set flags to `false`

### Phase 5 — Deprecate old infrastructure

- Remove all calls to old `platform/audit/Writer`
- Rename `iam_audit_log` → `iam_audit_log_deprecated`
- Historical migration: Temporal workflow reads `iam_audit_log_deprecated` in batches of 1000,
  transforms to `platform_audit_log` schema (sets `schema_version = 0` for legacy rows)
- After row-count and spot-check validation: `DROP TABLE iam_audit_log_deprecated`
- Drop SQL triggers: `SELECT disable_audit_on_schema('public')`
- Drop trigger functions (000451)
- This is the point of no return

**Historical data mapping:**

| iam_audit_log | platform_audit_log | Transform |
|---------------|-------------------|-----------|
| id | id | Direct |
| actor_id | actor_user_id | UUID parse |
| actor_email | context.actor_email | Move to context |
| operation | operation | "create"→"CREATE" etc. |
| entity_name | entity_name | Direct |
| record_id | record_id | UUID parse |
| before_snapshot | context.before | Move to context |
| after_snapshot | context.after | Move to context |
| diff | context.diff | Move to context |
| ip_address | actor_ip | Direct |
| request_id | context.request.id | Move to context |
| created_at | created_at | Direct |
| (none) | event_type | "entity.{operation}" |
| (none) | event_category | "DATA" |
| (none) | schema_version | 0 (marks as legacy) |
| (none) | actor_type | "user" |

### Phase 6 — Background and workflow coverage

- SystemActor context propagation in all background goroutines
- Temporal activity audit writes
- Outbox relay audit writes
- Bootstrap SYSTEM event
- Partition maintenance job deployed as Temporal workflow
- Additive — no rollback needed if events are wrong (correctable)

### Blue-green deployment

Feature flag enables blue-green: blue fleet runs `enabled = false`, green runs `enabled = true`.
Cutover: switch traffic to green, monitor 15 minutes, complete.
Rollback: switch traffic back to blue.

Dual-write window (Phase 3): old Writer → `iam_audit_log`, new Writer → `platform_audit_log`.
Different tables — no conflicts. Historical migration runs after Phase 5 when old table is frozen.

---

## Part 13 — Enterprise Features Classification

### Required for v1

Single canonical table, monthly partitions, RLS (5 policies), risk scoring (DB-backed cached),
compliance flag detection, immutability, sensitive field stripping, SystemActor model,
`platform_audit_checkpoint`, failure policy (suppressed/propagated), metrics, auth event logging
(login/logout/token issuance), ACCESS/DENY logging, ADMIN event logging (role changes), API token
aggregation table, GDPR anonymization path, partition maintenance job.

### Recommended for v2

Field-level diff table (`platform_audit_log_diff`), `audit.ExportForUser()` GDPR export endpoint,
per-entity audit verbosity (Metadata/Summary/Full), `audit_retention_config` per-tenant overrides,
read audit for sensitive entities (`AuditReads: true`), archive to S3/GCS, audit statistics
function, SIEM webhook output interface (`audit.MultiWriter`), record state replay API.

### Enterprise extension (v3+)

Cryptographic hash chain (per-row SHA-256 chaining), ed25519 row signatures, CDC streaming
(Debezium → Kafka), formal audit schema registry, automated compliance reports (SOX/GDPR Article
30), immutable external storage (S3 Object Lock), AI-assisted anomaly detection.

### Experimental

Differential privacy for aggregate audit reports, zero-knowledge audit proof, federated audit
across multiple ERP instances.

---

## Part 14 — Final Architecture Decisions Summary

| Question | Decision | Rejected |
|----------|----------|---------|
| Hook mechanism | Repository wrapper | Global hook registry, HookSet modification |
| Writer synchrony | TransactionalWriter (sync) | AsyncWriter (channel), outbox-within-audit |
| Failure — DATA | Suppressed + logged + metered | Propagated (breaks availability) |
| Failure — ADMIN/SECURITY | Propagated | Suppressed (security gap) |
| API token audit | Aggregation table + anomaly events | Per-call INSERT |
| Session audit location | AuthService layer | RedisSessionStore adapter |
| COMPLIANCE category | Removed | Keeping (redundant with compliance_flags) |
| AuditMode in EntityDef | Removed (AuditEnabled bool only) | Sync/async/disabled enum |
| Diff table | v1 JSONB only, v2 flat table | v1 flat table (premature) |
| Hash chain | v1 checkpoint model, v2 full chain | v1 per-row chain (partitioning incompatible) |
| SystemActor | Required, defined in awo/audit | Leaving system events unattributed |
| Partitioning | Monthly from day one | Deferred to v2 (retrofit impossible) |
| Diff ordering | Strip → then Diff (internal) | Public ComputeDiff API |
| Tenant FK | No FK constraint | FK with CASCADE (destroys history) |
| Actor FKs | No FK constraints | FK constraints (block GDPR anonymization) |

---

## Part 15 — Implementation Roadmap

### Phase 1 — Foundation
**Goal:** Storage and core package. No audit writes yet.

Deliverables: `awo/audit` package (all files), `platform_audit_log` table with partitions/RLS/
indexes, `platform_audit_token_usage`, `platform_audit_checkpoint`, `platform_audit_config`,
`audit.Init()` in bootstrap, `NoopWriter` for tests, feature flag = false.

Migrations: 000500–000502

Tests:
- Sanitizer: strip ordering, diff computation, recursive strip, sensitive field list union
- TransactionalWriter: failure policy (mock pgx), suppressed vs propagated per category
- RiskScorer: config table seeded in test DB, verify scoring algorithm
- Integration: INSERT into partition, RLS enforcement (wrong role, wrong tenant)

Rollback: drop 000500–000502 tables. Remove `audit.Init()` from bootstrap.

Success criteria: `/health/ready` audit check passes, zero rows in `platform_audit_log`,
all unit+integration tests pass.

---

### Phase 2 — Entity CRUD Coverage
**Goal:** All entity Create/Update/Delete automatically audited.

Deliverables: `audit.AuditingRepository[T]`, Wire DI updates for all entity repositories,
`AuditEnabled bool` in `EntityDefinition`, canary for `platform_settings`, staged rollout.

Migrations: none (code only)

Tests:
- Create: audit row in same transaction, correct before/after
- Update: before snapshot via Get, sensitive fields stripped, diff correct
- Delete: before snapshot captured before deletion
- BulkUpdate: exactly one summary event
- Rollback: entity rollback also rolls back audit row
- Write failure (DATA): entity write succeeds, error logged

Rollback: set feature flag `enabled = false`.

Success criteria: 100% of entity CRUD produces audit rows, zero sensitive fields in snapshots,
p99 entity latency increase < 10ms.

---

### Phase 3 — Auth and Access Coverage
**Goal:** Authentication and authorization events audited.

Deliverables: auth event writes in `AuthService` (10 event types), DENY event writes in
`RequirePermission` middleware, API token aggregation writes, anomaly detection logic.

Migrations: none (code only)

Tests:
- Login success: correct actor, IP, method in context
- Login failure: NULL actor_user_id, attempted_email_hash in context
- DENY: event written before 403 returned
- Auth write failure: login still succeeds
- Token validation: 10,000 calls → 1 row in token_usage, 1 anomaly event on new IP
- Logout ordering: audit written before session delete

Rollback: feature flags `auth_events = false`, `access_events = false`.

Success criteria: all auth flows produce events, no per-call INSERT for routine token validation,
auth event write failure does not block login.

---

### Phase 4 — Background and Lifecycle Coverage
**Goal:** System actors, Temporal, outbox, maintenance job.

Deliverables: `audit.WithSystemActor()` in all background goroutines, Temporal worker audit init,
workflow/activity audit events, outbox relay audit events, bootstrap SYSTEM event, partition
maintenance Temporal workflow, GDPR anonymization implementation, SQL trigger infrastructure
removal (point of no return).

Migrations:
- `000510_drop_audit_triggers.up.sql`: `SELECT disable_audit_on_schema('public')`
- `000511_drop_audit_trigger_functions.up.sql`: drop SQL trigger functions

Tests:
- Temporal: workflow audit event with correct initiator, activity failure event
- Outbox relay: OUTBOUND event on dispatch success and failure
- Bootstrap: SYSTEM event with NULL tenant_id, visible to admin_role only
- GDPR: anonymization nulls PII fields, does not delete rows, produces its own ADMIN event
- Partition maintenance: creates next-month partition, archives old, updates checkpoint
- Checkpoint verification: SHA-256 computed correctly

Rollback before trigger drop: possible via feature flags.
Rollback after trigger drop: re-run 000451 and re-enable schema audit. Complex — requires
one full release cycle of stability before executing trigger drop.

Success criteria: all background paths produce SystemActor events, maintenance job completes
without errors, checkpoints verify correctly, SQL trigger infrastructure removed.

---

### Phase 5 — Query API and Enterprise Features
**Goal:** Complete audit query API, GDPR export, SDUI history, monitoring.

Deliverables: `GET /api/v1/platform/audit-log` with cursor pagination, SDUI History tab on
entity detail views, `audit.ExportForUser()` GDPR export, `get_audit_statistics()` SQL function
adapted for `platform_audit_log`, `MultiWriter` implementation, weekly checkpoint cron, operational
runbooks.

Migrations:
- `000520_audit_query_indexes.up.sql`: additional indexes based on observed Phase 2–4 query patterns
- `000521_audit_statistics.up.sql`: adapt statistics function for new table

Tests:
- Query API: date-range filters, entity filter, actor filter, pagination (no duplicates)
- Tenant isolation: tenant user cannot see other tenant's audit events
- Platform admin: cross-tenant query works
- GDPR export: all events for user, < 30s for 10,000 events
- SDUI: history tab renders for record with 100+ events
- Cursor pagination: stable across concurrent writes

Rollback: no destructive schema changes. Feature flag controls API exposure.

Success criteria: audit query < 100ms for single-partition date-range, history tab active on all
entity detail views, GDPR paths tested, operational runbooks reviewed, all Phase 1–4 criteria
still met.

**Implementation readiness post-Phase 5: 9/10**
**Enterprise readiness post-Phase 5: 7/10** (remaining: hash chain, SIEM streaming, immutable
archive — v3 enterprise extension)
**Long-term maintainability post-Phase 5: 9/10**
