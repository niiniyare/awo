# Audit Query Patterns

**Classification:** Reference — Tier 2
**Owner:** `12-audit/AUDIT_QUERY_PATTERNS.md`
**Status:** Living document

---

## Purpose

This document provides canonical query patterns for reading the `audit_log` table. All queries target the `audit_log` global table directly — there is no `EntityRepository` abstraction over it.

---

## 1. Full History for a Record

```sql
SELECT
    id,
    operation,
    actor_id,
    service_acct,
    changed_fields,
    before_data,
    after_data,
    request_id,
    created_at
FROM audit_log
WHERE entity_type = 'finance_invoice'
  AND record_id   = '550e8400-e29b-41d4-a716-446655440000'
ORDER BY created_at ASC;
```

---

## 2. All Actions by a User (within a Tenant)

```sql
SELECT
    entity_type,
    record_id,
    operation,
    changed_fields,
    created_at
FROM audit_log
WHERE tenant_id = '7f3b4200-e29b-41d4-a716-446655440001'
  AND actor_id  = 'a1b2c300-e29b-41d4-a716-446655440002'
ORDER BY created_at DESC
LIMIT 100;
```

---

## 3. All Changes to a Specific Field

```sql
SELECT
    record_id,
    actor_id,
    before_data->>'status' AS before_status,
    after_data->>'status'  AS after_status,
    created_at
FROM audit_log
WHERE entity_type     = 'finance_invoice'
  AND tenant_id       = '7f3b4200-e29b-41d4-a716-446655440001'
  AND operation       = 'update'
  AND 'status'        = ANY(changed_fields)
ORDER BY created_at DESC;
```

---

## 4. All Deletes in a Time Window

```sql
SELECT
    entity_type,
    record_id,
    actor_id,
    before_data,
    created_at
FROM audit_log
WHERE tenant_id  = '7f3b4200-e29b-41d4-a716-446655440001'
  AND operation  = 'delete'
  AND created_at >= now() - INTERVAL '7 days'
ORDER BY created_at DESC;
```

---

## 5. Compliance Report: Invoice Mutations This Month

```sql
SELECT
    al.record_id,
    al.operation,
    al.actor_id,
    al.changed_fields,
    al.created_at
FROM audit_log al
WHERE al.entity_type = 'finance_invoice'
  AND al.tenant_id   = $1
  AND al.created_at  >= date_trunc('month', now())
  AND al.created_at  <  date_trunc('month', now()) + INTERVAL '1 month'
ORDER BY al.record_id, al.created_at;
```

---

## 6. Service Account Operations Only

```sql
SELECT
    entity_type,
    record_id,
    operation,
    service_acct,
    created_at
FROM audit_log
WHERE tenant_id    = $1
  AND service_acct IS NOT NULL
ORDER BY created_at DESC;
```

---

## 7. Go Query Helper

The framework provides `audit.QueryLog()` for application code that needs to surface audit history in the SDUI:

```go
// Package: awo.so/awo/audit

type QueryFilter struct {
    TenantID    uuid.UUID
    EntityType  string
    RecordID    *uuid.UUID
    ActorID     *uuid.UUID
    Operation   *string     // "create" | "update" | "delete"
    Since       *time.Time
    Until       *time.Time
    Limit       int         // default 50
    Offset      int
}

func QueryLog(ctx context.Context, db pgx.Querier, f QueryFilter) ([]*AuditRecord, error)
```

Use this helper rather than raw SQL from application code to benefit from consistent parameter binding and limit enforcement.

---

## 8. What Is Never Logged

The following are never present in `audit_log`:

| Excluded Item | Reason |
|--------------|--------|
| Fields with `Sensitive: true` | MUST be excluded per AUDIT_SPEC |
| Password hashes | Sensitive by definition |
| Session tokens | Never stored in entity records |
| Redis cache values | Cache is not the source of truth |
| Failed authentication attempts | Logged in application log (`slog`), not audit_log |
| Read operations (`GET`) | Audit log records mutations only |
| Platform admin login events | Logged in platform audit (separate table) |

---

## 9. Retention Policy

Audit records MUST NOT be deleted by application code. Retention is managed at the database level:

- **Minimum retention**: 7 years (KRA eTIMS compliance requirement for Kenyan tenants)
- **Archival strategy**: partition `audit_log` by year; move old partitions to cold storage
- **PITR**: PostgreSQL continuous archiving covers all `audit_log` writes

---

## References

- [`12-audit/AUDIT_SPEC.md`](AUDIT_SPEC.md) — Schema, pipeline stage, AuditEnabled
- [`04-multitenancy/GLOBAL_TABLES.md`](../04-multitenancy/GLOBAL_TABLES.md) — Global table access model
- `awo/audit/query.go` — QueryLog helper and QueryFilter
