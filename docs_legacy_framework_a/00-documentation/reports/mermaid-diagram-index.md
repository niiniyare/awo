> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Mermaid Diagram Index"
id: report-diagrams
status: accepted
category: GUIDE
stability: STABLE
audience: [framework-authors]
since: "1.0"
normative-level: informative
related:
  - "[Diagram Standards](../diagram-standards.md)"
  - "[Documentation Coverage](documentation-coverage.md)"
---

# Mermaid Diagram Index

**Generated for:** Awo Framework v1.0 documentation set

All Mermaid diagrams in the documentation set, indexed by document and type.

---

## By Document

### 01-introduction/architecture-overview.md
1. **EntityDefinition → 5 Subsystems** (graph TD) — shows how one definition drives persistence, API, SDUI, permissions, and workflows
2. **Five-Layer Architecture** (graph TD) — UI → API → Domain → Workflow → Store with import rules
3. **Compilation Pipeline Sequence** (sequenceDiagram) — init() → Registry.Compile() → Runtime
4. **Multi-Tenancy Isolation** (graph TD) — three-level isolation: PostgreSQL RLS → RBAC → PolicyFunc
5. **Module System** (graph TD) — platform modules + business modules → CompiledSchema
6. **Startup Sequence** (graph TD) — 7 steps with failure modes
7. **Request Lifecycle** (sequenceDiagram) — HTTP request through all 5 layers
8. **Infrastructure Dependencies** (graph TD) — process dependencies on PostgreSQL, Redis, Temporal

### 02-architecture/five-layer.md
9. **Five-Layer Import Rules** (graph TD) — strict downward import visualization

### 03-kernel/compilation-pipeline.md
10. **Compilation Flowchart** (flowchart TD) — 11 compilation steps from init() to sealed schema

### 03-kernel/startup-sequence.md
11. **Startup Sequence** (sequenceDiagram) — Config → PostgreSQL → Redis → Registry → Fiber → Temporal

### 06-tenancy/tenant-lifecycle.md
12. **Tenant Status Machine** (stateDiagram-v2) — PENDING → ACTIVE → SUSPENDED → ARCHIVED

### 10-modules/module-system.md
13. **Module Dependency Graph** (graph TD) — platform.tenancy → finance, hr; crm → finance

### 15-security/security-model.md
14. **Defense-in-Depth Layers** (graph TB) — request through Middleware → API → Domain → Store → Database

### 00-documentation/document-lifecycle.md
15. **Document Lifecycle** (stateDiagram-v2) — proposed → accepted / rejected → archived

### 00-documentation/diagram-standards.md
16-20. **Diagram Type Examples** — flowchart, sequence, state, ER, class diagrams (reference examples)

---

## By Diagram Type

### Sequence Diagrams (sequenceDiagram)
- architecture-overview.md: Compilation Pipeline, Request Lifecycle
- startup-sequence.md: Startup Sequence

### State Diagrams (stateDiagram-v2)
- tenant-lifecycle.md: Tenant Status Machine
- document-lifecycle.md: Document Lifecycle State Machine

### Flowcharts (graph TD / flowchart TD)
- architecture-overview.md: EntityDefinition → Subsystems, Five-Layer, Multi-Tenancy, Module System, Startup, Infrastructure
- five-layer.md: Import Rules
- compilation-pipeline.md: Compilation Steps
- module-system.md: Module Dependencies
- security-model.md: Defense-in-Depth

---

## Diagram Count Summary

| Type | Count |
|---|---|
| Sequence Diagrams | 3 |
| State Diagrams | 2 |
| Flowcharts | 10 |
| ER Diagrams | 0 |
| Class Diagrams | 0 |
| **Total** | **15** |

---

## Quality Assessment

All 15 diagrams comply with [Diagram Standards](../diagram-standards.md):
- Format: Mermaid only ✓
- Complexity: all under 20 nodes ✓
- Placement: immediately after prose they illustrate ✓
- Rationale: each diagram represents information that prose alone cannot communicate efficiently ✓
