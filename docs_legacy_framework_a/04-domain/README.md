> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Domain Layer — Section Overview"
id: domain-000-readme
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors, framework-authors, contributors]
since: "1.0"
normative-level: informative
related:
  - "[EntityDefinition](../03-kernel/entity-def.md)"
  - "[Five-Layer Architecture](../02-architecture/five-layer.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Domain Layer

**Section 04 | Domain Layer**

The Domain Layer is the stateless core of every module. It contains declarations and logic that define what entities are, how they behave, who may access them, and what happens as they move through their lifecycle.

Domain Layer code has zero external dependencies — it imports only `awo/def/`, `awo/filter/`, and the Go standard library. This property enables the entire domain layer to be tested without running infrastructure.

---

## Contents

| Document | ID | Purpose | Stability |
|---|---|---|---|
| [Fields](fields.md) | DOM-001 | FieldDef, all FieldType handlers, constraints, serialization | FROZEN |
| [Edges](edges.md) | DOM-002 | EdgeDef, edge types, explicit loading, cascade rules | FROZEN |
| [Hooks](hooks.md) | DOM-003 | Lifecycle hook system, HookRegistration, execution order | FROZEN |
| [Policy Functions](policies.md) | DOM-004 | Row-level filter injection, PolicyFunc composition | FROZEN |
| [Actions](actions.md) | DOM-005 | Custom operations, ActionDef, ActionContext, ActionResult | STABLE |
| [Naming Series](naming-series.md) | DOM-006 | Format string syntax, atomic counter, tenant override, ResetOnYear | STABLE |
| [Field Validators](validators.md) | DOM-007 | FieldValidator interface, built-in validators, composition, testing | STABLE |
| [Entity Events](events.md) | DOM-008 | Lifecycle events, workflow triggers, conditional dispatch, audit log | STABLE |
| [Permission Sets](permissions.md) | DOM-009 | PermissionSet structure, RBAC compilation, action permissions, SDUI gating | STABLE |
| [Naming Series Advanced](naming-series-advanced.md) | DOM-010 | Branch prefixes, yearly/monthly reset, multi-segment formats, conditional series | STABLE |
| [Edge Patterns](edge-patterns.md) | DOM-011 | One-to-many, polymorphic, many-to-many, self-referential, cross-module edges | STABLE |
| [Custom Actions](custom-actions.md) | DOM-012 | ActionDef declaration, ActionContext, state transitions, workflow triggers, bulk actions | STABLE |
| [Policy Patterns](policy-patterns.md) | DOM-013 | OwnerOnly, BranchScoped, role-conditional, composite, time-bounded policies | STABLE |

---

## Prerequisites

- [EntityDefinition](../03-kernel/entity-def.md) — the struct that contains all domain declarations
- [Five-Layer Architecture](../02-architecture/five-layer.md) §4 — Domain Layer responsibilities and import rules
- [Architecture Laws](../02-architecture/laws.md) — LAW-007 (hook order), LAW-008 (imports), LAW-013 (sensitive fields), LAW-017 (cross-module hooks)
- [Glossary](../GLOSSARY.md)
