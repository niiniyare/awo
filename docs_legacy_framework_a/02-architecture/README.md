> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Architecture — Section Overview"
id: arch-000-readme
status: accepted
category: GUIDE
stability: STABLE
audience: [framework-authors, module-authors, contributors]
since: "1.0"
normative-level: informative
related:
  - "[Architecture Overview](../01-introduction/architecture-overview.md)"
  - "[Architecture Laws](laws.md)"
  - "[Architecture Invariants](invariants.md)"
  - "[Five-Layer Architecture](five-layer.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Architecture

**Section 02 | Normative Architecture**

This section contains the normative architectural specification of the Awo Framework. Where Section 01 explains the architecture (informative), this section governs it (normative).

Documents here define rules that are permanently binding on all contributors, all module authors, all driver implementors, and all deployment operators. Violation of these rules is a defect regardless of whether the violation produces observable misbehavior today.

---

## Contents

| Document | ID | Purpose | Stability |
|---|---|---|---|
| [Architecture Laws](laws.md) | ARCH-001 | Twenty numbered laws permanently binding on all framework code | FROZEN |
| [Architecture Invariants](invariants.md) | ARCH-002 | Runtime properties that must hold unconditionally in every deployment | FROZEN |
| [Five-Layer Architecture](five-layer.md) | ARCH-003 | Normative specification of the five-layer dependency model | FROZEN |
| [Module Boundary Rules](module-boundaries.md) | ARCH-004 | Cross-module import rules, service interface pattern, enforcement | FROZEN |
| [Five-Layer Dependency Rules](five-layer-dependencies.md) | ARCH-006 | Precise import rules per layer: permitted, prohibited, CI enforcement | FROZEN |

---

## Relationship to Other Sections

The Architecture Laws in this section are the highest-authority normative rules in the framework, second only to approved ADRs. Every other document in `awo/docs/` must conform to the laws stated here.

When reading any other section:

- If a document's content conflicts with an Architecture Law, the document is wrong.
- If a document's content conflict with an Architecture Invariant, the document is wrong.
- If source code conflicts with either, the source code is wrong.

---

## Prerequisites

- [Philosophy](../01-introduction/philosophy.md)
- [Design Goals](../01-introduction/design-goals.md)
- [Architecture Overview](../01-introduction/architecture-overview.md)
- [Glossary](../GLOSSARY.md)
