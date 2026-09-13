> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "ADR-014: Casbin for RBAC"
id: adr-014
status: accepted
category: ADR
stability: FROZEN
audience: [framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[RBAC](../07-iam/rbac.md)"
  - "[Architecture Laws](../02-architecture/laws.md)"
  - "[Security Model](../15-security/security-model.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# ADR-014: Casbin for RBAC

**Status**: Accepted | **Stability**: Frozen

---

## Context

Awo requires role-based access control with:
- Multi-tenant domain isolation (a role in Tenant A grants no access in Tenant B)
- Role hierarchy (tenant.admin inherits all permissions from tenant.user)
- Fine-grained object-level permissions (`finance_invoice:create` separate from `finance_invoice:delete`)
- Runtime policy changes (adding/revoking permissions without redeploy)
- Auditability (which policies exist at any point in time)

---

## Decision

Use **Casbin** (`github.com/casbin/casbin/v2`) with the PRBAC (Policy Role-Based Access Control) model for all authorization decisions.

Policy model (`model.conf`):

```ini
[request_definition]
r = sub, dom, obj, act

[policy_definition]
p = sub, dom, obj, act

[role_definition]
g = _, _, _    # role inheritance within a domain

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub, r.dom) && r.dom == p.dom && r.obj == p.obj && r.act == p.act
```

- `sub`: `user:{uuid}` or `role:{name}`
- `dom`: tenant UUID or `_platform_`
- `obj`: entity type name
- `act`: `read`, `write`, `create`, `delete`, `submit`, etc.

---

## Why Casbin

1. **Domain support built-in**: `g = _, _, _` provides domain-scoped role inheritance — tenant isolation in RBAC without custom code.

2. **Policy storage via adapter**: Casbin's SQL adapter stores policies in PostgreSQL. Policy changes take effect immediately via `e.LoadPolicy()`.

3. **Role inheritance**: `g alice role:tenant.user tenant-uuid` means Alice inherits all permissions of `role:tenant.user` within that tenant. Multi-level inheritance works without custom resolution code.

4. **Separation of RBAC from row-level security**: Casbin governs operation-level access (can this actor create invoices?). PolicyFunc governs row-level access (which specific invoices can this actor see?). Both are required.

5. **Auditability**: Policy rules are rows in a `casbin_rules` table — queryable, auditable, and version-controlled via migrations for system roles.

---

## Alternatives Considered

### Custom permission table with code-side enforcement (Rejected)

Many Go apps use a simple `permissions` table and check `WHERE user_id = $1 AND resource = $2`. This works but:
- Role inheritance requires recursive queries or in-memory resolution
- No domain separation built-in
- Rewriting permission evaluation logic is fragile

### OPA (Open Policy Agent) (Rejected)

OPA is powerful and expressive. Rejected because:
- Requires a separate sidecar process or library embedding
- Policy language (Rego) is a specialization unfamiliar to most Go developers
- Overkill for Awo's relatively uniform permission model

### RBAC via middleware only, no library (Rejected)

Simple but: no role inheritance, no domain support, permission logic scattered across handlers.

---

## Consequences

### Positive

- System role policies seeded at tenant provisioning; enforced uniformly across all entities
- Permission changes (grant/revoke) are immediate via `e.LoadPolicy()`
- Role hierarchy handles escalating access cleanly (`role:tenant.admin` inherits all of `role:tenant.user`)

### Negative

- Casbin policy evaluation adds ~0.1ms per request (cached in memory; not a concern)
- Casbin's model config syntax is opaque to developers unfamiliar with it — documented in RBAC (IAM-001)
- Casbin `g` assertions are not intuitive for multi-tenant domains; the domain parameter must always be included

### Cache

Casbin uses in-memory policy cache. Policy changes require `e.LoadPolicy()` to propagate. This is called:
- On permission change via admin API
- Every 5 minutes (background refresh)
- On deployment restart

Worst case: 5-minute delay before a revoked permission stops working. Acceptable for an ERP context; would not be acceptable for a banking core system.

---

## Related

- [RBAC](../07-iam/rbac.md) — IAM-001: full Casbin policy model specification
- [Security Model](../15-security/security-model.md) — RBAC as Layer 3 of defense
