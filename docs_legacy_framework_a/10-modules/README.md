> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Modules — Section Overview"
id: mod-000-readme
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: informative
related:
  - "[Module System](module-system.md)"
  - "[Platform Modules](platform-modules.md)"
  - "[EntityDefinition](../03-kernel/entity-def.md)"
  - "[Registry](../03-kernel/registry.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Modules

**Section 10 | Module System**

A module is a cohesive collection of entity definitions, hooks, policies, activities, and migrations that addresses a specific business domain. Every capability in Awo — including the built-in platform capabilities — is delivered through modules.

---

## Contents

| Document | ID | Purpose | Stability |
|---|---|---|---|
| [Module System](module-system.md) | MOD-001 | Module structure, manifest, registration, dependency resolution | STABLE |
| [Platform Modules](platform-modules.md) | MOD-002 | Built-in platform modules: Tenant, IAM, Flags, Settings, Audit, Metadata, Registry | STABLE |
| [Business Module Catalog](business-modules.md) | MOD-003 | All business modules, core entities, dependency graph | STABLE |
| [Finance Patterns](finance-patterns.md) | MOD-004 | Double-entry, invoice workflow, VAT, payment reconciliation | STABLE |
| [HR Patterns](hr-patterns.md) | MOD-005 | Employee-user link, leave workflows, attendance, payroll | STABLE |
| [Inventory Patterns](inventory-patterns.md) | MOD-006 | Stock moves, FIFO valuation, lot tracking, COGS | STABLE |
| [Settings Patterns](settings-patterns.md) | MOD-007 | Declaring, reading, and overriding tenant-configurable settings | STABLE |
| [Forecourt Patterns](forecourt-patterns.md) | MOD-008 | Shift management, meter readings, fuel reconciliation, settlement | STABLE |
| [Payroll Patterns](payroll-patterns.md) | MOD-009 | Payslip computation, PAYE, NSSF/NHIF, GL journal posting | STABLE |
| [Projects Patterns](projects-patterns.md) | MOD-010 | Project lifecycle, time logging, billing, project dashboard | STABLE |
| [Audit Log Module](platform-audit-module.md) | MOD-011 | Tamper-evident audit log: auto-capture, sensitive fields, retention | STABLE |
| [Metadata Module](platform-metadata-module.md) | MOD-012 | Runtime custom field extension: declaration, validation, storage, UI | STABLE |
| [Feature Flags Module](platform-flags-module.md) | MOD-013 | Flag declaration, evaluation, caching, lifecycle management | STABLE |
| [CRM Patterns](crm-patterns.md) | MOD-014 | Customer entity, credit limit, lead conversion, sales rep policy | STABLE |
| [Module Registry](platform-module-registry.md) | MOD-015 | Installed modules, tenant activation, dependency enforcement, runtime check | STABLE |
| [Finance: eTIMS Integration](finance-etims.md) | MOD-016 | KRA eTIMS submission, TaxEntry entity, compliance requirements | STABLE |
| [HR: Leave Management](hr-leave-management.md) | MOD-017 | Leave types, balance tracking, approval workflow, calendar | STABLE |
| [Inventory: FIFO Valuation](inventory-valuation.md) | MOD-018 | FIFO layers, stock receipt/issue, COGS GL journals, reconciliation | STABLE |

---

## Prerequisites

- [EntityDefinition](../03-kernel/entity-def.md) — the central primitive modules build on
- [Registry](../03-kernel/registry.md) — how modules register their definitions
- [Startup Sequence](../03-kernel/startup-sequence.md) — when module init() runs
- [Glossary](../GLOSSARY.md) — Module, ModuleManifest, Platform Module, Business Module
