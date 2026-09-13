> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Documentation Philosophy"
id: gov-philosophy
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: informative
related:
  - "[Documentation Architecture](documentation-architecture.md)"
  - "[Documentation Standards](documentation-standards.md)"
  - "[Introduction — Philosophy](../01-introduction/philosophy.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Documentation Philosophy

**Status: Accepted | Stability: Stable**

This document explains the principles that guide how Awo documentation is structured, written, and maintained. It is the "why" behind the rules in the Documentation Standards.

---

## 1. Documentation as Specification

In Awo, documentation is not secondary to code. For normative documents (SPECs), the documentation is the specification — the code is an implementation of the documentation.

When documentation and code disagree, the documentation is correct (unless the documentation is demonstrably wrong — then both must be fixed). This is the opposite of "the code is the truth" — it requires a higher standard of documentation quality than most projects maintain.

Why: ERP systems serve regulated industries. Financial audits require evidence that the system behaves as documented. Code alone is insufficient; the behavior must be specified, and the specification must be maintained as the system evolves.

---

## 2. Specification Before Implementation

Normative documents (SPECs) exist before the feature they describe is implemented. This is intentional: writing the specification forces clarity about what the system must do before any code is written.

The discipline: "If you can't specify it, you can't implement it correctly."

Specifications also enable parallel work: the domain team specifies; the implementation team implements. Both can proceed with confidence that the contract is clear.

---

## 3. Permanence of Design Decisions

Architecture Decision Records (ADRs) exist because design decisions made in haste, without recording the rationale, are revisited endlessly. Every significant decision generates pressure to reconsider it — often from contributors who weren't present when the decision was made.

ADRs give the decision a permanent record: the context, the alternatives considered, why they were rejected, and what would trigger revisiting the decision. A contributor who wants to change an architectural decision must first engage with the existing ADR — not re-discover the same trade-offs from scratch.

---

## 4. Terminology Precision

Vague terminology is a source of bugs. In a framework that drives five subsystems from a single `EntityDefinition`, confusion between "entity type" and "entity name" can cause developers to modify the wrong thing.

Awo maintains a Glossary of canonical terms. Every term used in documentation and code has a precise def. This is not pedantry — it is correctness. The Glossary is the shared vocabulary that makes communication about the framework unambiguous.

---

## 5. The Stability Contract

Stability annotations (EXPERIMENTAL, STABLE, FROZEN) are promises to consumers. A STABLE API may change only with deprecation notice. A FROZEN API changes only through the formal RFC process.

These promises exist because framework consumers — module authors — need predictability. A module author who builds on a STABLE API should not have their module broken by a casual refactor. The stability annotation is the framework's commitment to the module author.

---

## 6. Dependency Order Is Not Optional

Documents reference other documents. A document about hooks (DOM-003) cannot be understood without reading about fields (DOM-001) first. The documentation set is organized in dependency order: foundational concepts before derived concepts.

This order is also the writing order: no document should reference a concept that is not already defined in an earlier document. Violation of this rule produces documentation that can be read but not understood.

---

## 7. Documentation Rot

Documentation that does not keep pace with the system it describes becomes misleading — worse than no documentation. Every code change that alters documented behavior MUST include a documentation update.

The review process enforces this: a PR that changes behavior without updating the relevant specification document is incomplete. Documentation completeness is part of the definition of "done."

---

## Related Documents

- [Documentation Architecture](documentation-architecture.md) — the structural rules derived from these principles
- [Documentation Standards](documentation-standards.md) — the writing rules derived from these principles
- [Introduction — Philosophy](../01-introduction/philosophy.md) — the framework's philosophical axioms (parallel to this document for the framework itself)
