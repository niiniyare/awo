> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Document Types"
id: gov-document-types
status: accepted
category: SPEC
stability: STABLE
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[Documentation Architecture](documentation-architecture.md)"
  - "[Documentation Standards](documentation-standards.md)"
  - "[Stability Model](stability-model.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Document Types

**Status: Accepted | Stability: Stable**

This document defines the categories of documents in the Awo documentation set, their purpose, and the rules that govern each type.

---

## 1. Document Categories

Every document declares its `category` in frontmatter. The five categories are:

### SPEC

A **specification** is a normative document that defines what the system MUST do. Specifications use RFC 2119 language (MUST, SHALL, SHOULD). Compliance with a SPEC is required.

- Target audience: implementors (framework authors, module authors)
- Key question: "What must be true?"
- Examples: EntityDefinition spec, Filter DSL spec, RLS spec

### GUIDE

A **guide** is an informative document that explains how to accomplish a goal. Guides do not use RFC 2119 language (no MUST/SHALL). Guides show recommended patterns, not requirements.

- Target audience: practitioners learning the system
- Key question: "How do I do this?"
- Examples: Module Developer Guide, Getting Started

### ADR

An **Architecture Decision Record** documents a significant design decision. ADRs are always FROZEN after acceptance. They record the context, alternatives considered, decision made, and consequences.

- Target audience: contributors understanding the "why"
- Key question: "Why was this decision made?"
- Examples: ADR-001 through ADR-008

### TEMPLATE

A **template** is a document intended to be copied and filled in. Templates are not specifications — they provide structure for new documents.

- Target audience: document authors
- Examples: ADR template, RFC template

### OVERVIEW

An **overview** is a section README — a navigation document that lists and links to the documents in a section. Overviews have no normative content.

- Target audience: all readers navigating the documentation
- Examples: Every `README.md` in a section directory

---

## 2. Normative vs. Informative

Every document also declares `normative-level`:

| Level | Meaning |
|---|---|
| `normative` | Compliance required; implementors MUST follow |
| `informative` | Informational only; describes, recommends, but does not require |

All SPEC documents are normative. All GUIDE, ADR, TEMPLATE, and OVERVIEW documents are informative.

---

## 3. Category Rules

### SPEC Documents

- MUST use RFC 2119 language for requirements
- MUST declare `normative-level: normative`
- MUST include a "Related Documents" section
- MUST cross-reference any Architecture Laws they enforce
- SHOULD include code examples for every normative rule

### GUIDE Documents

- MUST NOT use RFC 2119 language (no MUST/SHALL)
- MUST declare `normative-level: informative`
- SHOULD include a running example
- SHOULD cross-reference the relevant SPEC documents

### ADR Documents

- MUST follow the ADR template format
- MUST declare `stability: FROZEN` once accepted
- MUST NOT be modified after acceptance (new ADR supersedes)
- MUST include: Context, Options Considered, Decision, Consequences, Revisit Trigger

### TEMPLATE Documents

- Are themselves marked `stability: STABLE` (the template can evolve)
- Contain embedded document examples in code blocks

### OVERVIEW Documents

- Contain only navigation content (tables, links, brief descriptions)
- No normative content
- No code examples

---

## 4. Choosing the Right Type

| You want to... | Write a... |
|---|---|
| Define what the system must do | SPEC |
| Explain how to use the system | GUIDE |
| Record a design decision and its rationale | ADR |
| Provide a starting point for a new document | TEMPLATE |
| Link to documents in a section | OVERVIEW (README.md) |

---

## Related Documents

- [Documentation Architecture](documentation-architecture.md) — overall documentation structure
- [Documentation Standards](documentation-standards.md) — writing rules that apply to all types
- [Stability Model](stability-model.md) — how stability applies to each document type
- [ADR Template](adr-template.md) — template for ADR documents
