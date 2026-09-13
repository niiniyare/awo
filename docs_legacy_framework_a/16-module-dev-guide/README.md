> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Module Developer Guide — Section Overview"
id: mdg-000-readme
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors]
since: "1.0"
normative-level: informative
related:
  - "[Module System](../10-modules/module-system.md)"
  - "[EntityDefinition](../03-kernel/entity-def.md)"
  - "[Architecture Laws](../02-architecture/laws.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Module Developer Guide

**Section 16 | Module Developer Guide**

This guide walks through building a complete Awo business module from scratch. It follows the dependency order of the framework: entity definition → fields → edges → hooks → policies → actions → workflows → SDUI → migrations → testing.

Each document in this section builds on the previous one, using a single running example: a **`crm_contact`** entity within a `crm` module.

---

## Contents

| Document | ID | Topic |
|---|---|---|
| [Getting Started](01-getting-started.md) | MDG-01 | Module scaffold, manifest, init() |
| [Define an Entity](02-define-entity.md) | MDG-02 | EntityDefinition basics, storage model choice |
| [Add Fields](03-add-fields.md) | MDG-03 | FieldDef, all field types, constraints |
| [Add Edges](04-add-edges.md) | MDG-04 | EdgeDef, one-to-many, many-to-many |
| [Add Hooks](05-add-hooks.md) | MDG-05 | Validation hooks, after-save hooks |
| [Add Policies](06-add-policies.md) | MDG-06 | PolicyFunc, row-level filtering |
| [Add Actions](07-add-actions.md) | MDG-07 | ActionDef, custom route handlers |
| [Add Workflows](08-add-workflows.md) | MDG-08 | WorkflowTrigger, activities, sagas |
| [Add SDUI](09-add-sdui.md) | MDG-09 | PageBuilderSet, custom views |
| [Add Migrations](10-add-migrations.md) | MDG-10 | SQL migration files, RLS, indexes |
| [Testing](11-testing.md) | MDG-11 | Unit tests for hooks, policies, activities |
| [Module Checklist](12-checklist.md) | MDG-12 | Pre-review checklist for new modules |
| [Worked Example](13-worked-example.md) | MDG-13 | Complete CRM Contact module — all files in final state |
| [Common Mistakes](14-common-mistakes.md) | MDG-14 | Catalog of common mistakes by category with correct patterns |
| [Testing Patterns](15-testing-patterns.md) | MDG-15 | Integration tests, hook chains, policy tests, workflow tests |
| [Module Checklist (Extended)](16-module-checklist.md) | MDG-16 | 10-gate pre-merge checklist covering all subsystems |
| [Wire Dependency Injection](17-wire-dependency-injection.md) | MDG-17 | Wire provider sets, repository injection, cross-module interfaces |
| [Migration Patterns](18-migration-patterns.md) | MDG-18 | SQL templates: system entity, add column, add index, child table, rename |
| [Cross-Module Service Guide](19-cross-module-service-guide.md) | MDG-19 | Declare, implement, bind, and test cross-module service interfaces |

---

## Running Example

All examples use a `crm` module with a `crm_contact` entity. By the end of the guide, this entity has:

- Fields: name, email, phone, status, assigned_to, notes
- Edge: one-to-many to `crm_interaction`
- Hook: email uniqueness validator
- Policy: OwnerOnly (contacts visible only to assigned rep)
- Action: `qualify` — promotes contact to active lead
- Workflow: sends welcome email after contact creation
- SDUI: custom detail view with interaction history tab
- Migration: `CREATE TABLE crm_contact` with full RLS block

---

## Prerequisites

Before starting this guide:

- [Philosophy](../01-introduction/philosophy.md) — understand the foundational axioms
- [Architecture Overview](../01-introduction/architecture-overview.md) — five-layer mental model
- [EntityDefinition](../03-kernel/entity-def.md) — complete field reference
- [Architecture Laws](../02-architecture/laws.md) — the 20 laws you must not violate
