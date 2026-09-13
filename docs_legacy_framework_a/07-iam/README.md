> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "IAM — Section Overview"
id: iam-000-readme
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors, framework-authors, operators]
since: "1.0"
normative-level: informative
related:
  - "[RBAC](rbac.md)"
  - "[Sessions](sessions.md)"
  - "[Authentication](authentication.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# IAM

**Section 07 | Identity and Access Management**

The IAM module provides authentication, session management, and role-based access control for all tenant-scoped operations.

---

## Contents

| Document | ID | Purpose | Stability |
|---|---|---|---|
| [RBAC](rbac.md) | IAM-001 | Casbin model, role hierarchy, permission evaluation, system roles | FROZEN |
| [Sessions](sessions.md) | IAM-002 | Session lifecycle, Redis storage, token format, expiry | FROZEN |
| [Authentication](authentication.md) | IAM-003 | Credential verification, session creation, logout | STABLE |
| [Password Policy](password-policy.md) | IAM-004 | bcrypt hashing, complexity rules, temporary passwords, reset flow | STABLE |
| [Multi-Factor Authentication](mfa.md) | IAM-005 | TOTP enrollment, backup codes, MFA login flow, enforcement policy | STABLE |
| [API Clients](api-clients.md) | IAM-006 | Machine-to-machine auth, client credentials, scope-based authz | STABLE |
| [RBAC Deep Dive](rbac.md) | IAM-007 | Policy model, assertions, role hierarchy, storage, caching | STABLE |
| [Password Policy](password-policy.md) | IAM-008 | bcrypt cost, lockout, reset flow, password history | STABLE |
| [Session Management](session-management.md) | IAM-009 | Token format, creation, validation, sliding TTL, revocation | STABLE |
| [RBAC Deep Dive](rbac-deep-dive.md) | IAM-010 | Casbin model internals, role hierarchy, evaluation flow, storage, caching | STABLE |

---

## Prerequisites

- [Tenant Model](../06-tenancy/tenant-model.md) — tenant context required before IAM checks
- [Architecture Laws](../02-architecture/laws.md) — LAW-005 (context required), LAW-013 (sensitive fields)
- [Architecture Invariants](../02-architecture/invariants.md) — INV-007 (Redis required for session validation)
- [Glossary](../GLOSSARY.md) — Actor, Session, RBAC, Casbin, Role, Permission
