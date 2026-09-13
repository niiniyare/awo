> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Threat Model"
id: sec-004
status: accepted
category: SPEC
stability: STABLE
audience: [framework-authors, operators]
since: "1.0"
normative-level: informative
related:
  - "[Hardening Guide](hardening-guide.md)"
  - "[Input Validation](input-validation.md)"
  - "[RLS Deep Dive](../05-persistence/rls-deep-dive.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Threat Model

**SEC-004 | Status: Accepted | Stability: Stable**

Awo's threat model: what we protect, who the adversaries are, and what controls are in place.

---

## 1. Assets

| Asset | Sensitivity | Protection |
|---|---|---|
| Tenant financial data (invoices, payments, ledger) | Critical | RLS, RBAC, audit log |
| Employee PII (salary, tax ID, phone) | Critical | Sensitive field exclusion, RBAC |
| Session tokens | Critical | Redis-only, never logged, 256-bit entropy |
| API client secrets | Critical | bcrypt-stored, one-time display |
| Tenant admin credentials | High | bcrypt ≥ cost 12, MFA optional |
| Platform admin credentials | High | bcrypt + mandatory MFA |
| Business config (settings, feature flags) | Medium | RBAC (tenant.admin only) |
| Audit log | High (integrity) | Append-only, no app UPDATE/DELETE |
| amis page schemas | Low | Redis cache, tenant-scoped |

---

## 2. Adversary Profiles

### External Attacker (unauthenticated)

**Goal**: Gain access to tenant data or disrupt service.

**Capabilities**: Internet access, knowledge of common web vulnerabilities, automated tooling.

**Controls**:
- All endpoints require authentication (no public data endpoints)
- Rate limiting on auth endpoints (20 req/min per IP)
- Input validation at all entry points (Filter DSL prevents SQL injection)
- Request size limits (4MB default)
- No stack traces in responses

### Authenticated Tenant User (low privilege)

**Goal**: Access data outside their permission scope (other tenants, other users' data).

**Capabilities**: Valid session token, API access, knowledge of entity IDs.

**Controls**:
- RLS enforces tenant isolation at DB level — no amount of filter manipulation bypasses it
- PolicyFunc enforces row-level access within tenant (e.g., OwnerOnly)
- RBAC denies operations the user doesn't have permission for
- Entity IDs are UUID v7 (non-guessable) — enumeration requires O(2^128) guesses

### Authenticated Tenant Admin

**Goal**: Access other tenants' data; escalate to platform admin.

**Capabilities**: Full CRUD within their tenant, ability to create custom roles.

**Controls**:
- RLS is enforced regardless of role — tenant.admin cannot see another tenant's data at the DB level
- Custom roles cannot include `role:platform-admin` — platform admin is a platform-level concept
- Tenant admin cannot modify the `tenants` table or other global tables

### Compromised Application Instance

**Goal**: Access all tenants' data via the compromised instance.

**Capabilities**: Read environment variables, make DB queries, read Redis.

**Controls**:
- DB queries without `set_tenant_context` are filtered by RLS to no rows (NULL tenant ID matches no rows)
- Redis session tokens are opaque — they cannot be used to forge arbitrary sessions
- Secret rotation (JWT key, session key) immediately invalidates all sessions
- Audit log is append-only — attacker cannot erase their tracks

### Malicious Module Author

**Goal**: Insert backdoor into a module that steals data or bypasses auth.

**Capabilities**: Can write hooks, activities, handlers — executed with app privileges.

**Controls**:
- Code review required for all module merges
- App DB role has no `BYPASSRLS` — hooks execute as app_role, subject to RLS
- No module can import platform secrets directly (environment variable access is limited to the startup config struct)
- Architecture laws prohibit raw SQL in hooks — Filter DSL prevents injection even in hook code

---

## 3. Attack Vectors and Mitigations

### SQL Injection

**Vector**: Malicious filter parameters in API requests.

**Mitigation**: Filter DSL translates predicates to parameterized queries. No string interpolation of user values occurs at any layer. Raw SQL in business logic is prohibited by CLAUDE.md.

**Residual risk**: Negligible — requires bypassing both DSL sanitization and RLS.

### Cross-Tenant Data Access

**Vector**: Crafting requests to access another tenant's entity by guessing UUID.

**Mitigation**: UUID v7 is non-guessable (128 bits of entropy). Even if guessed, RLS rejects the query if tenant context doesn't match. RBAC would also deny the operation.

**Residual risk**: Low — requires 2^128 guesses AND bypassing RLS.

### Session Hijacking

**Vector**: Stealing a valid session token from network traffic or logs.

**Mitigation**: TLS required for all traffic. Session tokens are never logged. Tokens are 256-bit random — not guessable. Redis TTL limits exposure window.

**Residual risk**: Low — requires network interception (TLS) or log access (prevented by sensitive field policy).

### Privilege Escalation via Custom Role

**Vector**: Tenant admin creates a role with `role:platform-admin` permissions.

**Mitigation**: `role:platform-admin` is not a Casbin role — it is a flag on the `iam_user` record that only platform admins can set. Custom roles cannot include platform-level permissions.

### Audit Log Tampering

**Vector**: Covering tracks after a breach by modifying audit log entries.

**Mitigation**: App role has no `UPDATE` or `DELETE` on `iam_audit_log`. Modification requires direct DB access as the migration role or superuser — detected by DB-level audit (pgaudit).

### Temporal Workflow Injection

**Vector**: Triggering unintended workflows via crafted API requests.

**Mitigation**: Workflow triggers are declared in `EntityDefinition` — not configurable at runtime. API clients cannot specify which workflow to start. Workflow IDs are deterministically derived from entity type + record ID.

---

## 4. Out of Scope

The following threats are explicitly out of scope for the application layer:

| Threat | Responsibility |
|---|---|
| Physical server access | Data center / cloud provider |
| Kubernetes node compromise | Platform/infrastructure team |
| PostgreSQL superuser access | Database administrator |
| Certificate authority compromise | PKI administrator |
| DDoS at network layer | Cloud provider / CDN |

For these threats, operators follow their organization's infrastructure security runbooks.

---

## 5. Security Review Triggers

A security review is required when:
- Adding any new authentication mechanism
- Changing middleware order
- Adding any endpoint that does not require authentication
- Modifying RLS policies or `set_tenant_context` stored procedure
- Adding any external API integration that sends tenant data
- Changing session TTL or token format
- Adding file upload functionality

---

## Related Documents

- [Hardening Guide](hardening-guide.md) — operational controls implementing this threat model
- [Input Validation](input-validation.md) — injection prevention detail
- [RLS Deep Dive](../05-persistence/rls-deep-dive.md) — cross-tenant isolation implementation
- [ADR-005: Server-Side Sessions](../17-adr/adr-016-server-side-sessions.md) — session security rationale
