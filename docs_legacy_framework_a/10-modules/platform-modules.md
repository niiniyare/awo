> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Platform Modules"
id: mod-002
status: accepted
category: SPEC
stability: STABLE
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[Module System](module-system.md)"
  - "[Tenant Model](../06-tenancy/tenant-model.md)"
  - "[RBAC](../07-iam/rbac.md)"
  - "[Sessions](../07-iam/sessions.md)"
  - "[Architecture Laws](../02-architecture/laws.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Platform Modules

**MOD-002 | Status: Accepted | Stability: Stable**

This document describes each of the seven built-in platform modules: their purpose, the entities they own, and the capability tokens they provide.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. Overview

Platform modules are loaded unconditionally on every deployment. They cannot be deactivated per-tenant. Every business module declares `Requires` on at least `platform.tenancy` and `platform.iam`.

| Module | Package | Provides Token | Key Entities |
|---|---|---|---|
| Tenant | `internal/platform/tenant` | `platform.tenancy` | `tenant`, `tenant_config` |
| IAM | `internal/platform/iam` | `platform.iam` | `user`, `role`, `permission`, `session` |
| Feature Flags | `internal/platform/flags` | `platform.flags` | `feature_flag`, `flag_override` |
| Settings | `internal/platform/settings` | `platform.settings` | `setting_definition`, `setting_value` |
| Audit Log | `internal/platform/audit` | `platform.audit` | `iam_audit_log` |
| Metadata | `internal/platform/metadata` | `platform.metadata` | `custom_field_def`, `custom_field_value` |
| Module Registry | `internal/platform/registry` | `platform.registry` | `installed_module` |

---

## 2. Tenant Module

**Provides:** `platform.tenancy`

The Tenant module owns the top-level isolation boundary. Every other module's data is scoped to a tenant.

**Key responsibilities:**
- Tenant creation and lifecycle management (PENDING → ACTIVE → SUSPENDED → ARCHIVED)
- Tenant identification (subdomain, header, query param)
- `set_tenant_context()` stored procedure — the single RLS enforcement point
- Tenant provisioning workflow — seeds roles, settings, and activates default modules for a new tenant

**Core entities:**
- `tenant` — system entity; SQL columns; no RLS (global table accessible to platform admin role)
- `tenant_config` — per-tenant configuration key/value store; system entity

**Global tables (no RLS):** `tenants` is accessible without tenant context. Platform admin role reads across all tenants. Application role has no write access.

**Cross-module coupling:** Every module implicitly depends on the Tenant module. No explicit `DependsOn` is required — `platform.tenancy` is the baseline capability.

---

## 3. IAM Module

**Provides:** `platform.iam`

The IAM module owns user identity, role assignments, permissions, and session management within each tenant.

**Key responsibilities:**
- User accounts scoped to tenants
- Casbin policy storage and evaluation
- Session token issuance and validation (Redis-backed server-side sessions)
- Login, logout, password reset, forced password change on first login
- Role hierarchy management
- Platform admin bypass (role `platform-admin` skips Casbin entirely)

**Core entities:**
- `user` — system entity; SQL columns; bcrypt password hash stored here
- `role` — system entity; tenant-scoped role definitions
- `permission` — system entity; Casbin policy tuples `(subject, domain, object, action)`
- `session` — not stored in PostgreSQL; Redis-only (`session:{token}`)

**System roles seeded at tenant bootstrap:**

| Role | Description |
|---|---|
| `role:tenant.admin` | Full access within the tenant |
| `role:tenant.user` | Standard user; permissions customized per tenant |
| `role:api-client` | Machine-to-machine; limited scopes |

System roles MUST NOT be deleted. Module roles (e.g., `role:finance.accounts_payable`) are seeded by module provisioning workflows.

---

## 4. Feature Flags Module

**Provides:** `platform.flags`

The Feature Flags module provides per-tenant on/off switches for behavior gates without requiring code deploys.

**Key responsibilities:**
- Flag definition (system defaults) and per-tenant overrides
- Redis-cached evaluation: `eval:{sha256(flag+tenant+user)}`, 5-minute TTL
- Immediate cache invalidation on flag or override change
- Evaluation order: system default → tenant override → user override

**Core entities:**
- `feature_flag` — global flag definitions; owned by platform admin
- `flag_override` — tenant-level or user-level overrides

**Usage in business code:**

```go
// Flags evaluated for the current actor and tenant via PageSchemaContext
if psc.Flags.IsEnabled("finance.multi_currency") {
    // render currency selector
}
```

Flags are evaluated once per request, cached in the request context — not evaluated per check.

---

## 5. Settings Module

**Provides:** `platform.settings`

The Settings module provides a hierarchical configuration system: system defaults → tenant overrides → branch overrides.

**Key responsibilities:**
- Definition of settings with types, defaults, and validation rules
- Three-level hierarchy: system → tenant → branch
- Type-safe value retrieval (string, int, bool, decimal, JSON)
- Settings exposed in the admin UI without code change

**Core entities:**
- `setting_definition` — declares a setting: name, type, default value, description, scope
- `setting_value` — a tenant or branch override of a setting

**Example settings:**
- `finance.default_currency` — default: `"KES"`
- `invoice.auto_submit_threshold` — default: `"0"` (disabled)
- `hr.leave_approval_required` — default: `"true"`

Settings values are fetched via the Settings service:

```go
currency := settings.GetString(ctx, "finance.default_currency")
threshold := settings.GetDecimal(ctx, "invoice.auto_submit_threshold")
```

---

## 6. Audit Log Module

**Provides:** `platform.audit`

The Audit Log module records every data mutation for legal compliance and incident investigation.

**Key responsibilities:**
- Automatic audit entry for every entity create, update, delete, and action
- Tamper-evident records: entries are immutable (no UPDATE or DELETE on iam_audit_log)
- Tenant-scoped but accessible only to `role:tenant.admin` and `role:platform-admin`
- Audit log entries include: tenant, entity type, record ID, actor, operation, before/after JSONB, timestamp, request ID

**Core entity:**
- `iam_audit_log` — system entity; append-only; no RLS delete policy

**Sensitive field handling:** Fields declared `Sensitive: true` in `EntityDefinition` appear in audit log entries as `"[REDACTED]"` — consistent with [INV-006](../02-architecture/invariants.md#inv-006) and [LAW-013](../02-architecture/laws.md#law-013).

**Retention:** Audit log entries are retained according to the tenant's `audit.retention_days` setting. Purge is performed by a scheduled Temporal workflow, not by application code.

---

## 7. Metadata Module

**Provides:** `platform.metadata`

The Metadata module enables tenants to extend any entity with custom fields at runtime — without migrations or redeployment.

**Key responsibilities:**
- Custom field definition via the admin UI
- Custom field storage: `custom_fields jsonb` column on system entities; inline on custom entities
- Custom field validation (type coercion, required, regex)
- Filter DSL support for custom field predicates (`custom.field_name`)
- SDUI integration: custom fields appear automatically in generated schemas

**Core entities:**
- `custom_field_def` — tenant-scoped custom field definitions per entity type
- `custom_field_value` — not a separate table; stored in `custom_fields jsonb` column

**Custom field types available:** `Data`, `SmallText`, `LongText`, `Int`, `Float`, `Currency`, `Bool`, `Date`, `DateTime`, `Select`, `MultiSelect`

Custom fields are NOT available for: `Link`, `DynamicLink`, `NamingSeries`, `JSON` types — these require schema-level declaration.

---

## 8. Module Registry Module

**Provides:** `platform.registry`

The Module Registry module tracks which business modules are installed and activated for each tenant.

**Key responsibilities:**
- Per-tenant module activation state
- Module installation workflow (seeds roles, default settings, initial data)
- Module deactivation (marks inactive; does not delete data)
- SDUI navigation: only activated modules appear in the sidebar

**Core entity:**
- `installed_module` — tenant_id × module_name → status (ACTIVE, INACTIVE, INSTALLING, ERROR)

**Module installation workflow** (Temporal):
1. Mark `installed_module` as INSTALLING
2. Seed module-specific roles in Casbin
3. Seed module-specific default settings
4. Run module-specific provisioning hook (optional)
5. Activate module-specific feature flags
6. Mark `installed_module` as ACTIVE

Module deactivation is reversible — data is retained. Re-activation restores the module to the previous state.

---

## 9. Platform Module Interaction

Platform modules interact with each other through standard framework mechanisms — not direct imports:

```
Business module → uses Settings service (via settings.GetString)
Business module → calls audit.Logger (via audit.LogMutation — called by framework automatically)
Business module → reads Flags (via PageSchemaContext.Flags or flags.Evaluate)
Business module → never imports another business module directly
```

Business modules MUST NOT import each other. Cross-module data access goes through the `EntityRepository` interface using `Link` fields and explicit edge loading.

---

## Related Documents

- [Module System](module-system.md) — module structure, manifest, registration
- [Tenant Model](../06-tenancy/tenant-model.md) — Tenant module implementation detail
- [RBAC](../07-iam/rbac.md) — IAM module's Casbin implementation
- [Sessions](../07-iam/sessions.md) — IAM module's session management
- [Architecture Laws](../02-architecture/laws.md) — LAW-013 (sensitive fields), LAW-018 (outbox private)
- [Glossary](../GLOSSARY.md) — Platform Module, Business Module, ModuleManifest, Feature Flag
