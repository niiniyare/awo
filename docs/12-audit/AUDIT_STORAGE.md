# Audit Storage Specification

**Classification:** Specification — Tier 1
**Owner:** `12-audit/AUDIT_STORAGE.md`
**Status:** Approved — v1.0
**ADRs:** ADR-018, ADR-019

---

## Purpose

Specifies the `platform_audit_log` table schema, index strategy, partitioning, RLS policy set, retention policy, and archival. This document covers persistence only. For the write path and component architecture, see [`12-audit/AUDIT_ARCH.md`](AUDIT_ARCH.md).

---

## 1. Table Schema

```sql
-- platform_audit_log is a partitioned global table.
-- No RLS. Access controlled by permission checks + audit_retention_role.
-- Partition by range on created_at (monthly).

CREATE TABLE platform_audit_log (
    -- Identity
    id              uuid        NOT NULL DEFAULT gen_random_uuid(),
    tenant_id       uuid        NOT NULL,               -- tenant at time of event; NOT a FK
    request_id      text,                               -- X-Request-ID; NULL for system ops

    -- Entity
    entity_name     text        NOT NULL,               -- qualified name e.g. "finance_invoice"
    record_id       uuid,                               -- affected record PK; NULL for session/access events
    operation       text        NOT NULL                -- 'create'|'update'|'delete'|'action'|'login'|'logout'|'system'
                    CHECK (operation IN ('create','update','delete','action','login','logout','system')),

    -- Actor (exactly one of actor_id, service_account_id, system_actor is set)
    actor_id        uuid,                               -- human user UUID; NULL for SA/system ops
    service_account_id uuid,                            -- service account UUID; NULL for human/system ops
    system_actor    text,                               -- e.g. 'system:bootstrap'; NULL for human/SA ops

    -- Request context
    ip_address      text,                               -- client IP (IPv4 or IPv6); NULL for system ops
    session_id      text,                               -- HMAC-SHA256(token, secret); NULL for system ops

    -- Snapshots
    before_data     jsonb,                              -- NULL on create; sensitive fields stripped
    after_data      jsonb,                              -- NULL on delete; sensitive fields stripped
    changed_fields  text[],                             -- populated on update only

    -- Classification
    event_category  text        NOT NULL DEFAULT 'DATA'
                    CHECK (event_category IN ('DATA','AUTH','ACCESS','ADMIN','WORKFLOW','SYSTEM','OUTBOUND','SECURITY')),
    severity        text        NOT NULL DEFAULT 'INFO'
                    CHECK (severity IN ('CRITICAL','HIGH','MEDIUM','LOW','INFO')),
    risk_score      smallint    NOT NULL DEFAULT 0 CHECK (risk_score BETWEEN 0 AND 100),
    compliance_flags jsonb,                             -- {"GDPR": true, "PCI_DSS": false, "KRA_ETIMS": true}

    -- Event-type-specific context
    context         jsonb,                              -- schema varies by event_category (see §3)

    -- Partition key
    created_at      timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (id, created_at)    -- partition key must be in PK for partitioned tables
)
PARTITION BY RANGE (created_at);

-- Immutability: awo_app has INSERT + SELECT only.
-- UPDATE and DELETE reserved for audit_retention_role (GDPR anonymization only).
REVOKE UPDATE, DELETE ON platform_audit_log FROM awo_app;
GRANT SELECT, INSERT ON platform_audit_log TO awo_app;
```

---

## 2. Indexes

```sql
-- Primary access pattern: entity history
CREATE INDEX audit_entity_history
    ON platform_audit_log (entity_name, record_id, created_at DESC);

-- Actor audit trail
CREATE INDEX audit_actor
    ON platform_audit_log (actor_id, created_at DESC)
    WHERE actor_id IS NOT NULL;

-- Tenant-scoped audit queries
CREATE INDEX audit_tenant
    ON platform_audit_log (tenant_id, created_at DESC);

-- Compliance queries by category + severity
CREATE INDEX audit_category_severity
    ON platform_audit_log (event_category, severity, created_at DESC);

-- Request correlation
CREATE INDEX audit_request
    ON platform_audit_log (request_id)
    WHERE request_id IS NOT NULL;

-- Session correlation
CREATE INDEX audit_session
    ON platform_audit_log (session_id)
    WHERE session_id IS NOT NULL;

-- Risk score filtering (high-risk event queries)
CREATE INDEX audit_risk_score
    ON platform_audit_log (risk_score DESC, created_at DESC)
    WHERE risk_score >= 50;
```

All indexes are created on the parent table and inherited by partitions automatically.

---

## 3. Context JSONB Schemas

The `context` column carries event-type-specific data. Schema by `event_category`:

### DATA (entity mutations)

```json
{
  "operation_source": "api | workflow | migration",
  "workflow_id": "optional-temporal-workflow-id"
}
```

### AUTH (login / logout / session expiry)

```json
{
  "session_expires_at": "2026-07-27T10:00:00Z",
  "login_method": "password | sso | api_token",
  "failure_reason": "null | invalid_password | account_locked | ..."
}
```

### ACCESS (permission denied events)

```json
{
  "requested_permission": "finance.invoice.delete",
  "entity_name": "finance_invoice",
  "record_id": "uuid-or-null"
}
```

### ADMIN (admin operations)

```json
{
  "admin_action": "tenant_suspend | role_revoke | config_change | ...",
  "target_entity_name": "iam_tenant",
  "target_record_id": "uuid"
}
```

### WORKFLOW (workflow lifecycle events)

```json
{
  "workflow_id": "invoice-submit-uuid",
  "workflow_fn": "InvoiceWorkflow.SubmitInvoice",
  "task_queue": "default",
  "trigger": "entity_create | scheduled | manual"
}
```

### SYSTEM (background system operations)

```json
{
  "system_actor": "system:outbox-relay | system:bootstrap | ...",
  "operation_detail": "human-readable description"
}
```

---

## 4. Partitioning

### Partition Naming Convention

```
platform_audit_log_YYYY_MM
```

Examples: `platform_audit_log_2026_07`, `platform_audit_log_2026_08`.

### Partition Creation

Bootstrap creates the current month and two months ahead:

```sql
-- Example: bootstrap runs in July 2026
CREATE TABLE platform_audit_log_2026_07
    PARTITION OF platform_audit_log
    FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');

CREATE TABLE platform_audit_log_2026_08
    PARTITION OF platform_audit_log
    FOR VALUES FROM ('2026-08-01') TO ('2026-09-01');

CREATE TABLE platform_audit_log_2026_09
    PARTITION OF platform_audit_log
    FOR VALUES FROM ('2026-09-01') TO ('2026-10-01');
```

### Ongoing Partition Management

**Requires implementation verification (gap):** No scheduler exists in the current framework. A partition maintenance job must run on the 25th of each month to create the next month's partition. Options under consideration:

- `pg_cron` — PostgreSQL-native; no application code required
- Temporal scheduled workflow — consistent with existing workflow infrastructure
- External cron (ops-managed) — simplest but requires ops coordination

This gap must be resolved with an ADR before implementation begins. Until resolved, document as: *"Requires ops provisioning of partition maintenance job."*

### Default Partition (Safety Net)

```sql
-- Catches any inserts whose created_at falls outside all defined partitions.
-- Should never have rows in normal operation — its presence triggers an alert.
CREATE TABLE platform_audit_log_default
    PARTITION OF platform_audit_log DEFAULT;
```

---

## 5. RLS Policy Set

`platform_audit_log` is a global table — no per-row tenant isolation at the database level. Access control is application-layer (permission checks + query filters).

```sql
-- Disable RLS on the audit table (it is a global table by design — ADR-018).
-- Do NOT add RLS policies to platform_audit_log.
ALTER TABLE platform_audit_log DISABLE ROW LEVEL SECURITY;
```

The `current_tenant_id()` function is NOT called for audit table access. Tenant scoping for reads is applied at the application layer by the audit query handler.

**Rationale:** RLS requires `SET LOCAL app.current_tenant_id` per connection. Cross-tenant compliance queries (platform admin viewing all tenants) would require either disabling RLS per-query or using a superuser connection — both worse than application-layer filtering.

---

## 6. Integrity Checkpoints

```sql
CREATE TABLE platform_audit_checkpoint (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    partition_name  text        NOT NULL,   -- e.g. 'platform_audit_log_2026_07'
    row_count       bigint      NOT NULL,
    checksum        text        NOT NULL,   -- SHA-256 of sorted(id || created_at) for all rows
    checkpoint_at   timestamptz NOT NULL DEFAULT now(),
    created_by      text        NOT NULL    -- 'system:scheduler' or 'system:manual'
);

-- awo_app can insert checkpoints; cannot modify them.
REVOKE UPDATE, DELETE ON platform_audit_checkpoint FROM awo_app;
GRANT SELECT, INSERT ON platform_audit_checkpoint TO awo_app;
```

Checkpoints are written by the partition maintenance job (same job that creates new partitions). A checkpoint covers the state of the partition at checkpoint time. Missing rows added or deleted after the checkpoint are detectable by re-computing the checksum.

---

## 7. Retention Policy

**Application layer:** Never deletes audit records. No `DELETE` grants for `awo_app`.

**GDPR anonymization:** The `audit_retention_role` PostgreSQL role can UPDATE specific columns (e.g., NULLing `actor_id`, `ip_address`, `before_data`, `after_data`) to anonymize records for departed users. This is an explicit, audited operation — the anonymization action itself generates an audit record.

**Archival:** Cold storage archival is an ops concern. Recommended pattern: pg_dump partition → object storage (S3/GCS) → verify checksum → drop partition. Application code is not involved.

**Minimum retention (regulatory):**
- Kenya KRA eTIMS: 7 years for financial records
- GDPR: no minimum; right to erasure applies to personal data fields only
- PCI-DSS: 1 year online, 3 years total

---

## 8. Historical Data Migration Tables

During Phase 4 of the migration strategy, historical records from legacy tables are migrated:

```sql
-- Tracks migration progress per source table
CREATE TABLE platform_audit_migration_log (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    source_table    text        NOT NULL,   -- 'iam_audit_log' | 'audit_log'
    last_migrated_id uuid,                  -- last ID migrated from source
    rows_migrated   bigint      NOT NULL DEFAULT 0,
    status          text        NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending','in_progress','complete','failed')),
    started_at      timestamptz,
    completed_at    timestamptz,
    error           text
);
```

---

## References

- [`12-audit/AUDIT_ARCH.md`](AUDIT_ARCH.md) — Full audit architecture
- [`12-audit/AUDIT_SPEC.md`](AUDIT_SPEC.md) — AuditWriter interface
- [`12-audit/AUDIT_MIGRATION.md`](AUDIT_MIGRATION.md) — Migration from legacy system
- [`04-multitenancy/RLS_SPEC.md`](../04-multitenancy/RLS_SPEC.md) — RLS (audit is exempt)
- ADR-018 in [`00-overview/DECISION_REGISTER.md`](../00-overview/DECISION_REGISTER.md)
