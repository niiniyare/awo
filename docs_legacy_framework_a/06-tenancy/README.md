> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Tenancy — Section Overview"
id: ten-000-readme
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors, framework-authors, operators]
since: "1.0"
normative-level: informative
related:
  - "[Tenant Model](tenant-model.md)"
  - "[Row-Level Security](rls.md)"
  - "[Tenant Lifecycle](tenant-lifecycle.md)"
  - "[Philosophy](../01-introduction/philosophy.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Tenancy

**Section 06 | Multi-Tenancy**

Multi-tenancy is the foundational structural property of the Awo Framework. This section specifies how tenant isolation is implemented, enforced, and maintained across the framework's layers.

---

## Contents

| Document | ID | Purpose | Stability |
|---|---|---|---|
| [Tenant Model](tenant-model.md) | TEN-001 | Tenant identification, context propagation, PgBouncer requirements | FROZEN |
| [Row-Level Security](rls.md) | TEN-002 | PostgreSQL RLS specification: policies, set_tenant_context(), global tables | FROZEN |
| [Tenant Lifecycle](tenant-lifecycle.md) | TEN-003 | Status machine, HTTP response codes, provisioning, archival | STABLE |
| [Multi-Tenancy Patterns](multi-tenancy-patterns.md) | TEN-004 | Cross-tenant ops, platform admin context, background jobs, anti-patterns | STABLE |
| [Tenant Provisioning](tenant-provisioning.md) | TEN-005 | Provisioning workflow, role seeding, module activation, admin user creation | STABLE |
| [Multi-Branch Tenancy](multi-branch.md) | TEN-006 | Branch entity, branch context, branch-scoped policies, settings, RBAC | STABLE |

---

## Prerequisites

- [Philosophy §1](../01-introduction/philosophy.md#1-tenancy-is-structural) — why tenancy is structural
- [Architecture Laws](../02-architecture/laws.md) — LAW-005 (tenant context required), LAW-015 (DB-level isolation)
- [Architecture Invariants](../02-architecture/invariants.md) — INV-001 (all queries under RLS)
- [Glossary](../GLOSSARY.md) — Tenant, TenantContext, RLS, set_tenant_context(), Tenant Status Machine
