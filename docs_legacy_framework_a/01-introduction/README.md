> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Introduction — Section Overview"
id: intro-000-readme
status: accepted
category: GUIDE
stability: STABLE
audience: [all]
since: "1.0"
normative-level: informative
related:
  - "[Awo Glossary](../GLOSSARY.md)"
  - "[Architecture](../02-architecture/README.md)"
---

# Introduction

**Section 01 | Foundation**

This section establishes the conceptual foundation for the Awo Framework. It answers three questions that every other section depends on having answered first:

1. **Why does Awo exist?** What convictions led to its design? (`philosophy.md`)
2. **What does Awo aim to do?** What are its explicit goals and non-goals? (`design-goals.md`)
3. **What is Awo?** A high-level structural view of the system. (`architecture-overview.md`)

These documents are prerequisites for all subsequent sections. Readers who skip this section and proceed directly to kernel specifications or API documentation will encounter design decisions that appear arbitrary without the context this section provides.

---

## Contents

| Document | Purpose | Audience | Normative? |
|---|---|---|---|
| [Philosophy](philosophy.md) | Five architectural axioms that shape every design decision in Awo | All | No |
| [Design Goals](design-goals.md) | Explicit goals, non-goals, constraints, and success criteria | All | Yes |
| [Architecture Overview](architecture-overview.md) | 10,000-foot structural view: five layers, compilation pipeline, multi-tenancy, module system | All | No |
| [Glossary Quick Reference](glossary-quick-ref.md) | One-line definitions for the most-used Awo terms | All | No |
| [Quick Start](quick-start.md) | Create your first entity in 15 minutes: setup, migration, routes, UI | Module authors | No |

---

## Reading Order

Read sequentially:

1. [Philosophy](philosophy.md) — establishes the convictions behind the design
2. [Design Goals](design-goals.md) — translates convictions into concrete commitments
3. [Architecture Overview](architecture-overview.md) — shows how the commitments are realized structurally

---

## Prerequisites

None. This section has no prerequisites within the documentation. It is the entry point for all readers.

Readers are expected to be experienced Go developers familiar with HTTP servers, relational databases, and distributed systems at a conceptual level. Awo-specific terminology is defined in the [Glossary](../GLOSSARY.md) and introduced progressively throughout this section.

---

## What This Section Does Not Cover

This section does not specify behavior. It explains motivation and provides orientation. Normative specifications begin in:

- [`02-architecture/`](../02-architecture/) — Architecture Laws and Invariants
- [`03-kernel/`](../03-kernel/) — EntityDefinition, compilation, and the registry contract
- [`05-persistence/`](../05-persistence/) — EntityRepository interface specification
