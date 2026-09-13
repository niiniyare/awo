> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Kernel — Section Overview"
id: kern-000-readme
status: accepted
category: GUIDE
stability: STABLE
audience: [framework-authors, module-authors, contributors]
since: "1.0"
normative-level: informative
related:
  - "[Architecture Laws](../02-architecture/laws.md)"
  - "[Architecture Invariants](../02-architecture/invariants.md)"
  - "[Five-Layer Architecture](../02-architecture/five-layer.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Kernel

**Section 03 | Framework Kernel**

The framework kernel is the set of subsystems that operate before the first request arrives: the [EntityDefinition](../GLOSSARY.md#entitydefinition) declaration model, the [Entity Registry](../GLOSSARY.md#entity-registry), the compilation pipeline, and the startup sequence.

Every framework behavior during the Runtime Phase is derived from kernel outputs. The kernel is not accessed at request time — it produces the [CompiledSchema](../GLOSSARY.md#compiledschema) that all runtime subsystems consume.

---

## Contents

| Document | ID | Purpose | Stability |
|---|---|---|---|
| [EntityDefinition](entity-def.md) | KERN-001 | Complete specification of the central declaration primitive | FROZEN |
| [Compilation Pipeline](compilation-pipeline.md) | KERN-002 | Normative specification of the Initialization → Compile → Runtime phases | FROZEN |
| [Entity Registry](registry.md) | KERN-003 | Registry contract: registration acceptance, validation, and sealing | FROZEN |
| [Startup Sequence](startup-sequence.md) | KERN-004 | Hard dependency chain for process startup | STABLE |
| [Custom Entity Registry](custom-entity-registry.md) | KERN-005 | Runtime per-tenant schema registry, mutation lock, concurrency rules | STABLE |
| [Hook Pipeline](hook-pipeline.md) | KERN-006 | Exact pipeline: stages, TX boundaries, framework-injected hooks, delete flow | FROZEN |
| [Route Generation](route-generation.md) | KERN-007 | CRUD and action route auto-generation, permission middleware, schema routes | FROZEN |
| [def Package Reference](def-package-reference.md) | KERN-008 | Complete type reference: EntityDefinition, FieldDef, HookSet, PolicyFunc, ActionDef | FROZEN |
| [Wire Provider Sets](wire-provider-sets.md) | KERN-009 | Provider set organization, composition root, adding new module providers | STABLE |

---

## Reading Order

1. [EntityDefinition](entity-def.md) — understand the input to the kernel
2. [Entity Registry](registry.md) — understand how inputs are accumulated and validated
3. [Compilation Pipeline](compilation-pipeline.md) — understand how inputs become the CompiledSchema
4. [Startup Sequence](startup-sequence.md) — understand the full process startup from config to first request

---

## Prerequisites

- [Architecture Laws](../02-architecture/laws.md) — LAW-001 through LAW-004 govern the kernel directly
- [Architecture Invariants](../02-architecture/invariants.md) — INV-002 is established by the kernel
- [Architecture Overview](../01-introduction/architecture-overview.md) — §3 introduces the compilation pipeline
- [Glossary](../GLOSSARY.md) — especially: EntityDefinition, CompiledSchema, Entity Registry, Compilation Phase, Initialization Phase, Runtime Phase
