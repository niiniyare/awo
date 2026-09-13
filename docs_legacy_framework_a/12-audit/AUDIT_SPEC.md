> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

# Audit Specification

**Classification:** Specification — Tier 1
**Owner:** `12-audit/AUDIT_SPEC.md`
**Status:** Frozen at v1.0 (ADR-005)
**Package:** `awo.so/awo/audit`

---

## Purpose

This document specifies the audit pipeline stage (ADR-005), the `AuditRecord` schema, the `AuditEnabled` flag on `EntityDefinition`, and the immutability and compliance properties of the audit log.

---

## 1. ADR-005: Audit Is a Mandatory Pipeline Stage

Before ADR-005, audit logging was an optional `after_save` hook. This created two problems:
1. Hook authors could forget to add audit hooks.
2. Hooks could be removed without removing audit side effects.

ADR-005 makes audit a non-optional pipeline stage between `PERSIST` and `after_save`. The audit record is written inside the same database transaction as the entity record. If the entity write succeeds but the audit write fails, the transaction rolls back — no silent audit gaps.

The pipeline order is:

```
PERSIST → AUDIT RECORD (inside TX) → after_save → [TX commits]
```

---

## 2. AuditEnabled Flag

`EntityDefinition` carries `AuditEnabled bool` (default: `true`). Setting it to `false` suppresses audit records for that entity entirely:

```go
var SystemLogDefinition = def.SystemDefinition{
    Name:         "system_log",
    Module:       "platform",
    AuditEnabled: false,  // high-volume, no audit needed
    // ...
}
```

Entities that MUST NOT set `AuditEnabled: false`:
- Any entity in the Finance module (regulatory compliance)
- `User`, `Tenant`, `Role` (IAM audit requirement)
- `JournalEntry`, `LedgerEntry`, `Payment` (financial integrity)
- Any entity with `Sensitive: true` fields (sensitive field access must be logged)

---

## 3. AuditRecord Schema

```sql
CREATE TABLE audit_log (
    id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    uuid        NOT NULL,
    entity_type  text        NOT NULL,   -- qualified entity name, e.g. "finance_invoice"
    record_id    uuid        NOT NULL,   -- primary key of the affected record
    operation    text        NOT NULL    CHECK (operation IN ('create', 'update', 'delete')),
    actor_id     uuid,                   -- user UUID; NULL for service account operations
    service_acct uuid,                   -- service account UUID; NULL for user operations
    before_data  jsonb,                  -- NULL on create
    after_data   jsonb,                  -- NULL on delete
    changed_fields text[],              -- field names that changed (update only)
    request_id   text,                   -- X-Request-ID from middleware
    ip_address   inet,                   -- client IP from request
    created_at   timestamptz NOT NULL DEFAULT now()
);

-- No UPDATE or DELETE on this table — ever.
-- Access: INSERT for awo_app role; SELECT for audit readers; no UPDATE/DELETE for any role.
CREATE INDEX audit_log_entity ON audit_log (entity_type, record_id, created_at DESC);
CREATE INDEX audit_log_actor  ON audit_log (actor_id, created_at DESC) WHERE actor_id IS NOT NULL;
CREATE INDEX audit_log_tenant ON audit_log (tenant_id, created_at DESC);
```

`audit_log` is a global table — no RLS, no `tenant_id` FK constraint. It is accessible before per-tenant schemas load, and it records events for all tenants in one place for compliance reporting.

---

## 4. AuditRecord (Go Struct)

```go
// Package: awo.so/awo/audit

// AuditRecord is written by the framework for every entity mutation
// when AuditEnabled is true on the EntityDefinition.
type AuditRecord struct {
    ID            uuid.UUID
    TenantID      uuid.UUID
    EntityType    string          // qualified name e.g. "finance_invoice"
    RecordID      uuid.UUID
    Operation     OperationType   // "create" | "update" | "delete"
    ActorID       *uuid.UUID      // nil for service account operations
    ServiceAcctID *uuid.UUID      // nil for user operations
    BeforeData    map[string]any  // nil on create
    AfterData     map[string]any  // nil on delete
    ChangedFields []string        // populated on update only
    RequestID     string
    IPAddress     string
    CreatedAt     time.Time
}
```

---

## 5. What Is Included in Before/After Data

`BeforeData` and `AfterData` contain the full entity record as a JSON object, **excluding**:
- Fields with `Sensitive: true` — MUST be excluded from audit records
- `custom_fields` JSONB blob — included as-is (no per-key sensitivity filtering)
- Computed/virtual fields — not persisted, not in audit

The audit writer calls `EntityRecord.ToAuditMap()` which applies the sensitivity exclusion. The exclusion is derived from field metadata at compile time and cannot be overridden per-record.

---

## 6. Changed Fields Computation

For update operations, `ChangedFields` contains the names of fields whose value changed between `BeforeData` and `AfterData`. Comparison is deep equality via `reflect.DeepEqual` after JSON round-trip normalisation.

Fields that are not present in the update patch but whose persisted value did not change are NOT included in `ChangedFields`.

---

## 7. Service Account Operations

Operations triggered by service accounts (machine-to-machine API clients) populate `ServiceAcctID` and leave `ActorID` nil. The audit trail remains complete — the service account identity is the audit principal.

Platform admin operations populate both `ActorID` (the human admin) and leave `ServiceAcctID` nil.

---

## 8. Tamper Evidence

The `audit_log` table has no `UPDATE` or `DELETE` grants for the `awo_app` database role. The application cannot overwrite or delete audit records. Deletion requires direct superuser access, which is logged by PostgreSQL's own audit extension.

For legally mandated audit retention, configure PostgreSQL point-in-time recovery (PITR) with retention matching the compliance requirement (minimum 7 years for KRA eTIMS).

---

## 9. Normative Requirements

- The audit record MUST be written within the same database transaction as the entity record.
- Sensitive fields MUST be excluded from `before_data` and `after_data`.
- `AuditEnabled: false` MUST NOT be set on financial, IAM, or sensitive entities.
- `audit_log` MUST NOT have RLS enabled — it is a global table.
- The `awo_app` role MUST NOT have `UPDATE` or `DELETE` privileges on `audit_log`.
- `changed_fields` MUST be computed and populated for every update operation.

---

## References

- `awo/audit/audit.go` — AuditRecord struct, writer interface
- [`12-audit/AUDIT_QUERY_PATTERNS.md`](AUDIT_QUERY_PATTERNS.md) — How to query audit log
- [`04-multitenancy/GLOBAL_TABLES.md`](../04-multitenancy/GLOBAL_TABLES.md) — audit_log as global table
- [`01-entity/FIELD_TYPES_REFERENCE.md`](../01-entity/FIELD_TYPES_REFERENCE.md) — Sensitive field flag
- ADR-005 in [`00-overview/DECISION_REGISTER.md`](../00-overview/DECISION_REGISTER.md)
