> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Audit Log Platform Module"
id: mod-011
status: accepted
category: SPEC
stability: STABLE
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[Platform Modules](platform-modules.md)"
  - "[Entity Events](../04-domain/events.md)"
  - "[Structured Logging](../13-observability/structured-logging.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Audit Log Platform Module

**MOD-011 | Status: Accepted | Stability: Stable**

The Audit Log module provides a tamper-evident, append-only record of every data mutation in the system. It is unconditional — every deployment, every tenant, every entity.

---

## 1. Purpose and Scope

The audit log answers: **who changed what, when, and from what to what**.

Captured events:
- Every entity Create, Update, Delete
- Every custom Action execution
- Authentication events (login, logout, failed attempts)
- Permission denials (Casbin enforcement failures)
- Session lifecycle (create, expire, revoke)
- Tenant lifecycle state changes

Not captured in audit log (covered by application log):
- Read operations (high volume, not mutations)
- Cache hits/misses
- Health check polls

---

## 2. Audit Log Entity

The `iam_audit_log` table is a global table (no RLS) — readable by platform admins across tenants for compliance. Tenant admins can read their own tenant's entries only (enforced in the service layer, not RLS).

```go
// In internal/platform/audit/def.go
var AuditLogDefinition = def.SystemDefinition{
    Name:   "iam_audit_log",
    Module: "audit",
    Fields: []def.FieldDef{
        {Name: "tenant_id",      Type: def.FieldData, Required: true, Immutable: true},
        {Name: "entity_type",    Type: def.FieldData, Required: true, Immutable: true},
        {Name: "entity_id",      Type: def.FieldData, Required: true, Immutable: true},
        {Name: "action",         Type: def.FieldSelect,
            Options: []string{"create", "update", "delete", "action", "login", "logout",
                "login_failed", "permission_denied", "session_created", "session_revoked"},
            Required: true, Immutable: true},
        {Name: "actor_id",       Type: def.FieldData, Immutable: true},  // user UUID or "system"
        {Name: "actor_type",     Type: def.FieldSelect,
            Options: []string{"user", "api_client", "system", "workflow"},
            Immutable: true},
        {Name: "ip_address",     Type: def.FieldData, Immutable: true},
        {Name: "user_agent",     Type: def.FieldData, Immutable: true},
        {Name: "request_id",     Type: def.FieldData, Immutable: true},
        {Name: "before_data",    Type: def.FieldJSON, Immutable: true},  // JSONB snapshot before change
        {Name: "after_data",     Type: def.FieldJSON, Immutable: true},  // JSONB snapshot after change
        {Name: "changed_fields", Type: def.FieldJSON, Immutable: true},  // []string field names changed
        {Name: "action_name",    Type: def.FieldData, Immutable: true},  // for custom actions
        {Name: "metadata",       Type: def.FieldJSON, Immutable: true},  // extra context (workflow ID etc.)
        {Name: "occurred_at",    Type: def.FieldDateTime, Required: true, Immutable: true},
    },
    Permissions: def.PermissionSet{
        Create: []string{"role:system.internal_only"},  // only framework writes audit log
        Read:   []string{"role:tenant.admin", "role:platform-admin"},
        Write:  []string{"role:system.internal_only"},
        Delete: []string{"role:system.internal_only"},  // effectively immutable
    },
}
```

---

## 3. Automatic Audit Log Creation

Every entity mutation automatically creates an audit log entry. This is wired in the framework's `after_save` hook pipeline — module authors do not write audit log code.

```go
// framework/audit/auto_hook.go (framework-internal)
type AutoAuditHook struct {
    AuditRepo def.EntityRepository[AuditLog]
}

func (h *AutoAuditHook) AfterSave(ctx context.Context, record *def.EntityRecord, isUpdate bool) error {
    actor := session.ActorFromContext(ctx)
    requestMeta := request.MetaFromContext(ctx)

    action := "create"
    if isUpdate {
        action = "update"
    }

    entry := def.CreateInput{
        Fields: map[string]any{
            "tenant_id":      record.TenantID,
            "entity_type":    record.EntityType,
            "entity_id":      record.ID.String(),
            "action":         action,
            "actor_id":       actor.UserID.String(),
            "actor_type":     actor.Type,
            "ip_address":     requestMeta.IPAddress,
            "user_agent":     requestMeta.UserAgent,
            "request_id":     requestMeta.RequestID,
            "before_data":    record.PreviousSnapshot,  // nil for creates
            "after_data":     sanitizeForAudit(record.Fields),
            "changed_fields": record.ChangedFields,
            "occurred_at":    time.Now().UTC(),
        },
    }

    if err := h.AuditRepo.Create(ctx, entry); err != nil {
        // Audit log failure must NOT fail the business operation
        // Log the audit failure, but return nil
        slog.Error("audit log write failed",
            "entity_type", record.EntityType,
            "entity_id",   record.ID,
            "err",         err,
        )
        return nil
    }

    return nil
}
```

**Key design decision**: Audit log failure does NOT abort the business transaction. The audit log is a best-effort compliance record; a failed audit write is alarmed and investigated, but does not block the user's operation.

---

## 4. Sensitive Field Handling

Fields marked `Sensitive: true` in `FieldDef` are excluded from `before_data` and `after_data`:

```go
func sanitizeForAudit(fields map[string]any) map[string]any {
    // The framework auto-strips sensitive fields before audit log creation
    // Module authors declare Sensitive: true on FieldDef — no per-module sanitization code
    sanitized := make(map[string]any, len(fields))
    for k, v := range fields {
        if !isSensitiveField(k) {
            sanitized[k] = v
        }
    }
    return sanitized
}
```

Examples of auto-excluded fields:
- `password_hash` — always excluded
- `session_token` — always excluded
- Any field with `Sensitive: true` in its `FieldDef`

---

## 5. Querying Audit Log

Module authors can query the audit log for compliance UI or entity history views:

```go
// Get full history for a specific record
func GetEntityHistory(ctx context.Context, repo def.EntityRepository[AuditLog], entityType string, entityID uuid.UUID) ([]AuditLog, error) {
    entries, _, err := repo.Query(ctx, filter.And(
        filter.Eq("entity_type", entityType),
        filter.Eq("entity_id", entityID.String()),
    ), def.WithSort("occurred_at", "desc"))
    if err != nil {
        return nil, fmt.Errorf("GetEntityHistory: %w", err)
    }
    return entries, nil
}
```

SDUI history panel pattern:

```go
// Include in entity detail page builder
if psc.IfPermitted("iam_audit_log", "read") {
    historyPanel := amis.Panel("Change History",
        amis.CRUD(amis.CRUDProps{
            API: fmt.Sprintf("GET /api/v1/entities/iam_audit_log?entity_type=%s&entity_id=${id}", entityType),
            Columns: []amis.Column{
                {Name: "occurred_at", Label: "When", Type: "datetime"},
                {Name: "actor_id",   Label: "By"},
                {Name: "action",     Label: "Action"},
                {Name: "changed_fields", Label: "Fields Changed", Type: "json"},
            },
        }),
    )
}
```

---

## 6. Audit Log Retention

Audit logs are retained per compliance policy:

| Tenant type | Retention |
|---|---|
| Standard | 2 years (configurable via platform setting) |
| Financial/regulated | 7 years minimum |
| Platform admin events | Indefinite |

Purge is executed via a Temporal scheduled workflow — never via application code or ad-hoc SQL.

```go
// Runs monthly, purges entries older than retention period
func AuditLogRetentionWorkflow(ctx workflow.Context) error {
    // For each tenant: fetch retention days setting, delete entries older than cutoff
    // Uses batched deletes to avoid long-running transactions
}
```

---

## 7. Tamper-Evidence

The audit log table has:
- `Write` permission set to `role:system.internal_only` — no user or API client can modify entries
- DB-level: no `UPDATE` or `DELETE` grants to the app role on `iam_audit_log` table (migration enforces this)
- All `Immutable: true` fields — framework rejects any update attempt at the domain layer

For high-assurance environments, each audit entry can be chained via a hash (SHA-256 of previous entry + current content). The platform setting `audit.chain_hashing` enables this. Verification: `GET /api/v1/platform/audit/verify-chain?tenant_id={id}`.

---

## Related Documents

- [Platform Modules](platform-modules.md) — audit module in context of all platform modules
- [Entity Events](../04-domain/events.md) — lifecycle events that trigger audit log entries
- [Structured Logging](../13-observability/structured-logging.md) — application log vs audit log distinction
- [Hardening Guide](../15-security/hardening-guide.md) — audit log immutability verification
