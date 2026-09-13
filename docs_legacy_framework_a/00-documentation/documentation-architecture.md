> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Awo Framework — Documentation Architecture Specification"
id: das-001
status: accepted
category: governance
stability: frozen
audience: [documentation-authors, contributors, maintainers, core-team]
since: "1.0"
normative-level: normative
---

# Awo Framework Documentation Architecture Specification

**DAS-001 | Status: Accepted | Stability: Frozen**

This document is the constitution of the Awo Framework documentation system. It defines the rules, structures, standards, and governance that every current and future document in `awo/docs/` must follow. No documentation may be written, accepted, or modified without conforming to this specification.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, SHOULD NOT, RECOMMENDED, MAY, and OPTIONAL in this document are to be interpreted as described in [RFC 2119](https://www.rfc-editor.org/rfc/rfc2119).

This document itself is subject to its own rules. Where this document uses MUST, those are documentation laws. Where it uses SHOULD, those are strong recommendations with documented exceptions. Where it uses MAY, those are permitted variations.

---

## Table of Contents

1. [Documentation Philosophy](#1-documentation-philosophy)
2. [Documentation Taxonomy](#2-documentation-taxonomy)
3. [Documentation Dependency Graph](#3-documentation-dependency-graph)
4. [Documentation Metadata Standard](#4-documentation-metadata-standard)
5. [Document Lifecycle](#5-document-lifecycle)
6. [Stability Model](#6-stability-model)
7. [Cross-Reference Policy](#7-cross-reference-policy)
8. [Terminology Governance](#8-terminology-governance)
9. [Diagram Standards](#9-diagram-standards)
10. [Writing Standards](#10-writing-standards)
11. [Architecture Laws for Documentation](#11-architecture-laws-for-documentation)
12. [Documentation Governance](#12-documentation-governance)
13. [The `00-documentation/` Root Section](#13-the-00-documentation-root-section)
14. [Quality Gates](#14-quality-gates)

---

## 1. Documentation Philosophy

### 1.1 Purpose

The Awo Framework documentation is the authoritative specification of the framework. Its primary purpose is to define what Awo is, how it works, why it is designed the way it is, and what guarantees it makes — permanently.

The documentation is not a supplement to the source code. It is the specification that the source code implements. When the documentation and the implementation conflict, the documentation is authoritative. The implementation must be corrected to match.

This distinction is not academic. It has consequences:

- A behavior present in the source code but absent from the documentation is **not** a public feature. It is an implementation detail that may change without notice.
- A guarantee stated in the documentation must be upheld by every implementation, including future ones. The guarantee cannot be removed without a major version change and an explicit deprecation process.
- An architectural invariant documented as permanent is permanently binding on all contributors, regardless of whether the source code currently enforces it.

### 1.2 What the Documentation Is

The Awo documentation is:

- **A specification.** It defines what Awo is, not how it happens to be implemented today.
- **A contract.** Guarantees made here are binding on all framework versions within a major version.
- **A theory book.** It explains the reasoning behind every significant design decision.
- **A reference.** It provides complete, authoritative information for every public API, concept, and configuration.
- **A governance record.** It records what was decided, what was rejected, and why.
- **A teaching instrument.** It enables experienced Go developers to understand the framework without reading source code.

### 1.3 What the Documentation Is Not

The Awo documentation is not:

- **A code walkthrough.** Documentation does not narrate implementation. It specifies behavior.
- **A generated API reference.** Auto-generated godoc is a supplement, not a substitute for specification.
- **A blog post.** Documentation does not speculate, editorialize, or express opinions without clearly marking them as such.
- **A changelog.** Version history belongs in `CHANGELOG.md` and ADRs, not in specification documents.
- **A tutorial collection.** Tutorials are one document type among many. Tutorial content must never contaminate specification documents.
- **A marketing document.** Documentation does not compare Awo favorably to alternatives using subjective language.
- **A temporary artifact.** Every document that is accepted and stable is expected to remain correct for at least one major version lifetime (the target is 20 years).

### 1.4 Guiding Principles

**P1 — Single Source of Truth**

Every concept, term, invariant, and guarantee has exactly one canonical location in the documentation. Other documents reference that location. They do not restate it. Restatement creates drift — over time, two descriptions of the same thing diverge and contradict each other. This principle prevents that drift architecturally.

*Why it matters:* In a 200-file documentation corpus spanning 20 years and dozens of contributors, duplicated explanations will diverge. Architectural invariants restated in multiple places will eventually be updated in one place and forgotten in another, silently creating contradictions. One location. One truth.

**P2 — Specification Before Implementation**

Documentation is written before or concurrent with implementation. Documentation that is written after implementation to describe what was built is a changelog, not a specification. Post-hoc documentation describes the current implementation — which may be wrong. Specification-first documentation describes the intended behavior — which the implementation must match.

*Why it matters:* When documentation is written after implementation, the documentation inherits the bugs of the implementation. When documentation is written before implementation, the implementation must conform to the specification, and deviations are visible.

**P3 — Theory Before Mechanics**

Every specification must be preceded by a theory document that explains why the thing exists and what problem it solves. A reader who understands the theory can derive much of the specification independently. A reader who memorizes the specification without theory cannot adapt to edge cases, cannot evaluate proposed changes, and cannot maintain the framework in 2038.

*Why it matters:* Frameworks fail not when engineers cannot find the API, but when they cannot understand the design well enough to use it correctly. Theory documents reduce the number of support questions, the number of incorrect uses, and the maintenance burden on the core team.

**P4 — Stable Terminology**

Terms, once defined, are permanently defined. Their meaning does not change. If a concept evolves beyond what its original term captures, a new term is introduced and the old term is deprecated — never redefined. Every document uses terms exactly as defined in the Glossary.

*Why it matters:* Terminology drift is invisible and catastrophic. When "Entity" means something slightly different in the kernel specification than in the application developer guide, readers combine these definitions in their mental model and reach incorrect conclusions. A decade of terminology drift produces a documentation corpus that cannot be relied upon.

**P5 — Long-Term Maintainability**

Every documentation decision is evaluated against the question: "Will this be correct in 2045?" Documentation that depends on current implementation details will be wrong when the implementation changes. Documentation that describes architectural principles will remain correct as long as the principles hold. Prefer principles over details.

*Why it matters:* The purpose of this documentation system is to serve the framework for two decades. Short-term convenience that produces long-term ambiguity is rejected.

**P6 — Normative Clarity**

Every normative claim — every statement about what the framework guarantees, what it requires, what it prohibits — MUST use RFC 2119 language (MUST, SHOULD, MAY). Every informative claim — every explanation, example, or guidance — MUST NOT use RFC 2119 language. This distinction prevents readers from misidentifying guidance as requirements.

*Why it matters:* When a documentation corpus mixes "should" (guidance) with "SHOULD" (normative recommendation), readers cannot distinguish strong guidance from binding requirements. This ambiguity produces incorrect implementations and incorrect expectations.

**P7 — Documentation as Architecture**

Documentation is a first-class engineering artifact. It is designed, reviewed, versioned, and maintained with the same rigor as source code. A documentation PR that introduces a new concept without defining it in the Glossary is as defective as a source PR that introduces a public function without a type signature. The quality gates for documentation are as strict as the quality gates for code.

*Why it matters:* Documentation that is treated as secondary produces documentation that is inconsistent, incomplete, and unmaintainable. Documentation that is treated as architecture produces documentation that is the foundation of the ecosystem.

---

## 2. Documentation Taxonomy

Every document belongs to exactly one category. Categories are not organizational directories — they are semantic classifications that determine writing style, authority, lifetime, and change policy.

### 2.1 Category Definitions

---

#### SPEC — Normative Specification

**Purpose:** Defines what the framework guarantees. Every SPEC statement is binding on all implementations. SPECs use RFC 2119 language throughout.

**Authority:** Highest. SPECs define the framework. Implementations that deviate from SPECs are incorrect.

**Audience:** Framework implementors, driver authors, specification committee, auditors.

**Expected Lifetime:** Permanent within a major version. Changes require an RFC and a major version bump for breaking changes.

**Change Frequency:** Extremely rare. Only when the specification is incorrect or the framework evolves.

**Required RFC 2119 usage:** Yes. Every normative statement uses MUST, SHOULD, or MAY.

**Examples:**
- `03-kernel/types/filter.md` — Filter DSL specification
- `03-kernel/invariants.md` — Architecture law specifications
- `06-drivers/conformance/entity-store.md` — EntityStore conformance specification
- `02-theory/consistency-model.md` — Consistency guarantee specification

**Writing style:** Declarative, precise, impersonal. No tutorials. No examples unless they clarify a normative statement. No "we" or "you." Third person or imperative construction: "The compiler MUST..." not "You should make the compiler..."

---

#### THEORY — Theoretical Foundation

**Purpose:** Explains the reasoning, principles, and design decisions behind a concept. Enables readers to understand *why* the framework is designed as it is. Theory documents are not instructions — they are explanations.

**Authority:** High. Theory documents inform SPEC. They explain the rationale behind SPECs. If a SPEC conflicts with the THEORY that justifies it, the SPEC takes precedence, and the THEORY must be corrected.

**Audience:** All developers. Required reading before SPEC documents.

**Expected Lifetime:** Permanent. The reasoning behind a design decision does not change after the decision is made.

**Change Frequency:** Rare. Only when the theoretical understanding of a concept changes — not when the implementation changes.

**Required RFC 2119 usage:** No. THEORY documents explain; they do not prescribe.

**Examples:**
- `02-theory/compilation-theory.md` — Why metadata is compiled
- `02-theory/tenancy-model.md` — Why tenancy is structural
- `02-theory/consistency-model.md` — Consistency trade-offs

**Writing style:** Explanatory, reasoned, analytical. SHOULD explain trade-offs. SHOULD explain alternatives that were considered. SHOULD explain why the chosen approach is superior for Awo's design goals. MAY use first person plural ("We chose...") when attributing a design decision to the framework's design team.

---

#### REF — Reference

**Purpose:** Complete, authoritative lookup information for a specific API, concept, configuration, or behavior. REF documents are not read linearly — they are consulted.

**Authority:** High for the subject it covers. REF documents describe the current state of a stable API. SPECs define the guarantee; REF documents describe the implementation of that guarantee.

**Audience:** Developers using the framework — looking up a specific thing.

**Expected Lifetime:** Stable. Changes when the thing it describes changes.

**Change Frequency:** Moderate. Updated with each minor version that modifies the described API.

**Required RFC 2119 usage:** Selectively. Use MUST/SHOULD/MAY when describing behavioral requirements of the thing being referenced.

**Examples:**
- `12-reference/filter-dsl.md` — Filter operator reference
- `12-reference/error-catalog.md` — Error code reference
- `12-reference/environment-variables.md` — Configuration reference

**Writing style:** Encyclopedic, complete, structured. Every item has the same fields. No narrative. No introductions. Optimized for scanning and lookup.

---

#### GUIDE — Conceptual Guide

**Purpose:** Explains a concept, system, or subsystem for understanding. Guides help readers build a mental model. They are read linearly, unlike REF documents, but they do not teach tasks, unlike TUTORIALs.

**Authority:** Informative. No normative statements.

**Audience:** Application developers building with the framework.

**Expected Lifetime:** Stable with minor revisions.

**Change Frequency:** Low to moderate. Updated when the underlying concept changes significantly.

**Required RFC 2119 usage:** No.

**Examples:**
- `08-app-dev/entities/choosing-entity-type.md`
- `08-app-dev/policies/policy-composition.md`

**Writing style:** Explanatory, conceptual. Explain what something is, why it works the way it does, and how it fits into the larger framework. MAY use diagrams liberally. SHOULD include a "Common mistakes" section.

---

#### TUTORIAL — Task-Oriented Tutorial

**Purpose:** Teaches a specific task from start to finish. Tutorials are prescriptive and sequential. A reader who follows a tutorial exactly will accomplish one specific outcome.

**Authority:** Informative. No normative statements.

**Audience:** New application developers learning the framework.

**Expected Lifetime:** Evolving. Tutorials are updated frequently as APIs and conventions change.

**Change Frequency:** High. Must be kept synchronized with the current API.

**Required RFC 2119 usage:** No.

**Examples:**
- `08-app-dev/getting-started.md`
- `17-appendices/examples/finance-invoice/README.md`

**Writing style:** Instructional, sequential, first-person plural ("Let's create...") or second-person ("Create a file..."). Every step produces a visible, testable result. MUST include the final state of all files created. SHOULD include what to do when things go wrong.

---

#### ADR — Architecture Decision Record

**Purpose:** Permanent record of a significant architectural decision. An ADR documents what was decided, why it was decided, what alternatives were rejected, and what the long-term implications are.

**Authority:** Historical. ADRs are immutable records, not prescriptions. The decisions they record are prescriptions, enforced by SPEC documents.

**Audience:** Future core team, contributors proposing changes, architects evaluating the framework.

**Expected Lifetime:** Permanent. ADRs are never deleted and never modified after acceptance.

**Change Frequency:** Zero after acceptance. An ADR is written once, accepted, and frozen forever.

**Required RFC 2119 usage:** No. ADRs describe decisions; they do not prescribe behavior.

**Examples:**
- `13-adrs/ADR-001-metadata-first-architecture.md`
- `13-adrs/ADR-003-postgresql-rls-isolation.md`
- `13-adrs/rejected/REJ-001-go-plugin-package.md`

**Writing style:** Historical, analytical, structured. Every ADR uses the standard ADR template (defined in `13-adrs/template.md`). MUST include: context, decision, rationale, alternatives considered, implications, status.

---

#### RFC — Request for Comments

**Purpose:** Proposal for a significant change to the framework, documentation, or governance. RFCs are the formal mechanism for proposing breaking changes, new major APIs, or changes to frozen documents.

**Authority:** Provisional. An RFC becomes authoritative only after it is accepted and its contents are incorporated into SPEC documents.

**Audience:** Core team, contributors, ecosystem stakeholders.

**Expected Lifetime:** Permanent as a historical record. The RFC itself does not prescribe behavior — the SPEC documents it produces do.

**Change Frequency:** RFCs are open for comment during their review period. After acceptance or rejection, they are frozen.

**Required RFC 2119 usage:** Yes, within the proposal itself.

**Examples:**
- `00-documentation/rfcs/RFC-001-hook-registration-model.md`

**Writing style:** Formal proposal format. MUST include: problem statement, proposed solution, alternatives considered, migration strategy, compatibility implications, open questions.

---

#### LAW — Architecture Law

**Purpose:** Formal statement of an invariant that must never be violated. Architecture Laws are the highest-authority normative statements in the framework. They constrain the framework itself — no part of the framework may violate an Architecture Law.

**Authority:** Supreme. Architecture Laws cannot be overridden by any other document. Changing an Architecture Law requires a unanimous core team decision and a major version bump.

**Audience:** All contributors, reviewers, core team.

**Expected Lifetime:** Permanent.

**Change Frequency:** Effectively zero. Laws are written once, reviewed exhaustively, and frozen.

**Required RFC 2119 usage:** Mandatory throughout.

**Examples:**
- `17-appendices/formal/architecture-laws.md`
- `03-kernel/invariants.md` (the kernel-specific laws)

**Writing style:** Formal, terse, precise. Each law has: identifier (LAW-NNN), statement (one sentence), motivation (one paragraph), enforcement mechanism (how it is checked), consequences (what happens when it is violated).

---

#### GLOSSARY — Canonical Term Definition

**Purpose:** The single location where every term used in the documentation is defined precisely. No term is defined outside the Glossary. Every other document links to the Glossary for definitions.

**Authority:** Definitional. The Glossary defines the meaning of language used throughout the documentation. If a document's usage of a term conflicts with the Glossary definition, the document is incorrect.

**Audience:** Everyone. The Glossary is read before any other document.

**Expected Lifetime:** Permanent. Definitions, once accepted, do not change.

**Change Frequency:** New terms are added regularly. Existing definitions are effectively frozen.

**Required RFC 2119 usage:** No. Definitions are definitional, not prescriptive.

**Writing style:** Encyclopedic. Every entry has: term, definition (precise, one paragraph), disambiguation (what it is not), related terms, where it is used.

---

#### ANTI — Anti-Pattern

**Purpose:** Documents a common incorrect usage of the framework, explains why it is incorrect, and provides the correct alternative. Anti-patterns are explicitly educational — they teach by showing what not to do.

**Authority:** Informative. Anti-patterns do not prescribe behavior — they warn against incorrect behavior.

**Audience:** Application developers.

**Expected Lifetime:** Stable. Anti-patterns are updated when the framework changes in ways that alter what is incorrect.

**Change Frequency:** Low. Added when recurring incorrect usage patterns are observed.

**Required RFC 2119 usage:** No.

**Examples:**
- `17-appendices/anti-patterns/float-for-money.md`
- `17-appendices/anti-patterns/bypassing-rls.md`

**Writing style:** Warning-first. Opens with a clear statement of what the anti-pattern is. Shows the incorrect code. Explains exactly why it is incorrect and what will go wrong. Shows the correct alternative. References the authoritative document.

---

#### APPENDIX — Supplementary Material

**Purpose:** Supporting material that does not fit into the primary documentation structure. Examples, formal grammars, state machine diagrams, worked examples, benchmarks.

**Authority:** Informative, unless explicitly marked SPEC (for formal grammars and state machines).

**Audience:** Varies by appendix.

**Expected Lifetime:** Varies. Examples evolve; formal grammars are frozen.

**Change Frequency:** Varies.

**Required RFC 2119 usage:** Only in formally marked SPEC appendices.

---

### 2.2 Category Summary Table

| Code | Name | Authority | RFC 2119? | Lifetime | Change Rate |
|------|------|-----------|-----------|----------|-------------|
| SPEC | Normative Specification | Highest | Required | Permanent | Extremely rare |
| THEORY | Theory | High | No | Permanent | Rare |
| REF | Reference | High | Selective | Stable | Moderate |
| GUIDE | Conceptual Guide | Informative | No | Stable | Low–moderate |
| TUTORIAL | Tutorial | Informative | No | Evolving | High |
| ADR | Architecture Decision Record | Historical | No | Permanent | Zero post-acceptance |
| RFC | Request for Comments | Provisional | Yes | Permanent | Frozen post-decision |
| LAW | Architecture Law | Supreme | Mandatory | Permanent | Effectively zero |
| GLOSSARY | Canonical Term Definition | Definitional | No | Permanent | New entries only |
| ANTI | Anti-Pattern | Informative | No | Stable | Low |
| APPENDIX | Supplementary Material | Varies | Varies | Varies | Varies |

---

## 3. Documentation Dependency Graph

Documents have dependency relationships. A document B depends on document A if B requires the reader to have read A, or if B references definitions or concepts introduced in A.

### 3.1 The Dependency Hierarchy

The following hierarchy is absolute. A document at level N MUST NOT depend on a document at level M where M > N. No circular dependencies are permitted.

```
Level 0 — META (self-referential)
└── 00-documentation/documentation-architecture.md  (this document)
    └── 00-documentation/metadata-standard.md
    └── 00-documentation/terminology-governance.md
    └── 00-documentation/diagram-standards.md
    └── 00-documentation/writing-standards.md
    └── 00-documentation/architecture-laws.md

Level 1 — FOUNDATION (no framework knowledge required)
└── docs/GLOSSARY.md
    └── 01-introduction/philosophy.md
    └── 01-introduction/design-goals.md
    └── 01-introduction/non-goals.md

Level 2 — ORIENTATION (requires Level 1)
└── 01-introduction/architecture-overview.md
    └── 01-introduction/why-go.md
    └── 01-introduction/why-postgresql.md
    └── 01-introduction/why-compile-metadata.md
    └── 01-introduction/why-drivers.md
    └── 01-introduction/comparison.md

Level 3 — THEORY (requires Levels 1–2)
└── 02-theory/entity-model.md
    └── 02-theory/metadata-model.md
    └── 02-theory/tenancy-model.md
    └── 02-theory/security-model.md
    └── 02-theory/storage-model.md
    └── 02-theory/compilation-theory.md
    └── 02-theory/execution-model.md
    └── 02-theory/policy-model.md
    └── 02-theory/workflow-model.md
    └── 02-theory/event-model.md
    └── 02-theory/sdui-model.md
    └── 02-theory/consistency-model.md
    └── 02-theory/extension-philosophy.md
    └── 02-theory/failure-philosophy.md
    └── 02-theory/performance-philosophy.md

Level 4 — KERNEL SPECIFICATION (requires Level 3)
└── 03-kernel/stability-policy.md
    └── 03-kernel/versioning.md
    └── 03-kernel/breaking-change-policy.md
    └── 03-kernel/types/record.md
    └── 03-kernel/types/viewer-context.md
    └── 03-kernel/types/filter.md
    └── 03-kernel/types/field-def.md
    └── 03-kernel/types/field-type.md
    └── 03-kernel/types/edge-def.md
    └── 03-kernel/types/lifecycle-stage.md
    └── 03-kernel/types/hook-registration.md
    └── 03-kernel/types/policy-func.md
    └── 03-kernel/types/policy-result.md
    └── 03-kernel/types/action-def.md
    └── 03-kernel/types/trigger-event.md
    └── 03-kernel/types/workflow-trigger.md
    └── 03-kernel/types/create-input.md
    └── 03-kernel/types/update-input.md
    └── 03-kernel/types/page-info.md
    └── 03-kernel/types/aggregate-spec.md
    └── 03-kernel/types/aggregate-result.md
    └── 03-kernel/types/ui-page.md
    └── 03-kernel/types/module-manifest.md
    └── 03-kernel/types/capability-token.md
    └── 03-kernel/types/entity-def.md
    └── 03-kernel/types/compiled-schema.md
    └── 03-kernel/invariants.md

Level 5 — SUBSYSTEM SPECIFICATION (requires Level 4)
└── 04-compiler/**
    └── 05-runtime/**
    └── 06-drivers/driver-contract.md
    └── 06-drivers/optional-interfaces.md
    └── 06-drivers/conformance/**

Level 6 — FORMAL ARTIFACTS (requires Level 5)
└── 17-appendices/formal/architecture-laws.md
    └── 17-appendices/formal/filter-grammar.md
    └── 17-appendices/formal/state-machines.md

Level 7 — APPLICATION DEVELOPMENT (requires Levels 4–5)
└── 08-app-dev/**
    └── 15-platform-modules/**
    └── 16-api-clients/**

Level 8 — OPERATIONS AND SECURITY (requires Levels 5–7)
└── 09-operations/**
    └── 10-diagnostics/**
    └── 11-security/**

Level 9 — REFERENCE (requires Levels 4–8)
└── 12-reference/**

Level 10 — EXAMPLES AND APPENDICES (requires Levels 7–9)
└── 17-appendices/examples/**
    └── 17-appendices/patterns/**
    └── 17-appendices/anti-patterns/**

Level 11 — ADRs AND HISTORY (independent, but informed by all)
└── 13-adrs/**

Level 12 — CONTRIBUTING AND META (requires all others)
└── 14-contributing/**
    └── docs/README.md
```

### 3.2 Dependency Rules

**DLAW-DEP-001:** A document at level N MUST NOT import a definition from a document at level M where M > N.

**DLAW-DEP-002:** ADRs are level-independent. They record history. They MAY reference documents at any level without creating a dependency violation.

**DLAW-DEP-003:** GLOSSARY.md is the zero-dependency root. It MUST NOT reference any framework document other than other Glossary entries.

**DLAW-DEP-004:** TUTORIAL documents (level 7+) MAY reference SPEC documents at any level. A tutorial that teaches an application developer to write a hook SHOULD reference the hook-registration SPEC. The SPEC does not reference the tutorial.

**DLAW-DEP-005:** No circular references are permitted between any two documents. If document A depends on B and B depends on A, one of them is misclassified. Resolve by extracting the shared concept into a common lower-level document.

### 3.3 Why No Circular Dependencies?

Documentation circular dependencies produce the same problem as code circular dependencies: the reader cannot determine where to start. If understanding A requires understanding B which requires understanding A, neither can be understood. The strict hierarchy ensures there is always a path from zero knowledge to complete understanding.

---

## 4. Documentation Metadata Standard

Every Markdown document in `awo/docs/` MUST begin with a YAML frontmatter block. The frontmatter block is machine-readable and is used by:

- The documentation build system (link validation, dependency graph generation)
- Quality gate scripts (metadata completeness checks)
- The review workflow (status tracking)
- The diagram generation system (which documents need updated diagrams)

### 4.1 Required Fields

Every document MUST include all required fields. A PR that introduces a document missing required fields MUST be rejected.

```yaml
---
title: "Human-readable title of the document"
id: "unique-document-identifier"
status: "draft | review | accepted | stable | deprecated | archived"
category: "spec | theory | ref | guide | tutorial | adr | rfc | law | glossary | anti | appendix"
stability: "frozen | stable | evolving | experimental | draft"
audience: ["list", "of", "audiences"]
since: "version when this document was introduced"
normative-level: "normative | informative | mixed"
---
```

### 4.2 Optional Fields

These fields SHOULD be included when applicable.

```yaml
---
# Optional: specify the document level in the dependency hierarchy
level: 4

# Optional: list documents this document explicitly depends on
depends-on:
  - "03-kernel/types/record.md"
  - "GLOSSARY.md"

# Optional: list documents that are related but not dependencies
related:
  - "02-theory/consistency-model.md"
  - "05-runtime/transaction-boundaries.md"

# Optional: for SPEC documents, the RFC or ADR that authorized this spec
authorized-by: "RFC-003"

# Optional: when the document was last reviewed for accuracy
last-reviewed: "2025-07"

# Optional: for deprecated documents, what replaces them
superseded-by: "03-kernel/types/hook-registration.md"

# Optional: for ADRs, the decision date
decided: "2025-07-06"

# Optional: for experimental documents, when they are expected to stabilize
stabilizes-in: "1.2"
---
```

### 4.3 Field Specifications

**`title`** (required)

The title displayed at the top of the document. MUST match the `# H1` heading of the document content. MUST be unique across all documents. MUST be human-readable and describe the document's content precisely.

**`id`** (required)

A stable, lowercase, hyphen-separated identifier for the document. Used in cross-references and link validation. MUST be unique across all documents. MUST NOT change after a document reaches `accepted` status. Examples: `kernel-filter-spec`, `tutorial-getting-started`, `adr-001`.

**`status`** (required)

The document's current lifecycle state. One of: `draft`, `review`, `accepted`, `stable`, `deprecated`, `archived`. Full semantics defined in Section 5.

**`category`** (required)

The document category as defined in Section 2. One of: `spec`, `theory`, `ref`, `guide`, `tutorial`, `adr`, `rfc`, `law`, `glossary`, `anti`, `appendix`. The category determines the writing style, authority, and change policy.

**`stability`** (required)

The document's stability level as defined in Section 6. One of: `frozen`, `stable`, `evolving`, `experimental`, `draft`.

**`audience`** (required)

A list of intended reader roles. Valid values: `all`, `application-developers`, `driver-authors`, `framework-contributors`, `operators`, `security-auditors`, `core-team`, `module-authors`, `api-clients`.

**`since`** (required)

The framework version in which this document was introduced. Format: `"1.0"`, `"1.2"`, `"2.0"`. Documents introduced before v1.0 are marked `"1.0"`.

**`normative-level`** (required)

Whether the document makes normative claims. `normative` means RFC 2119 language is used and claims are binding. `informative` means no normative claims are made. `mixed` means some sections are normative (and are clearly marked as such within the document).

---

## 5. Document Lifecycle

Every document has a lifecycle from initial proposal through eventual archival. The lifecycle is sequential. States are not skipped.

### 5.1 Lifecycle States

```
PROPOSED → DRAFT → REVIEW → ACCEPTED → STABLE → DEPRECATED → ARCHIVED
```

---

**PROPOSED**

A document is `proposed` when the need for it is identified but writing has not begun. Proposals SHOULD be filed as GitHub issues with the `doc:proposed` label. The proposal MUST state: what document is needed, which category it belongs to, which audience it serves, which existing documents it depends on, and why it is needed.

*Transition to DRAFT:* A maintainer approves the proposal and assigns a writer.

---

**DRAFT**

A document is in `draft` when it is being actively written. DRAFT documents are committed to branches (never directly to main). DRAFT documents MUST include `status: draft` in their frontmatter. DRAFT documents MUST NOT be linked from stable documents as authoritative references.

DRAFT documents MAY be incomplete. They SHOULD have all required metadata fields, even if some are marked `TBD`. The draft MUST include a complete outline before any full sections are written.

*Transition to REVIEW:* The author submits a PR. The PR MUST pass all quality gate checks. The author MUST explicitly request review from at least two people: one domain expert (who knows the subject being documented) and one documentation reviewer (who evaluates writing quality, consistency, and architecture compliance).

---

**REVIEW**

A document is in `review` when a PR has been submitted and is awaiting reviewer approval. Reviewers evaluate: technical accuracy, consistency with other documents, terminology compliance, metadata completeness, style compliance, and quality gate passage.

Review comments MUST be addressed before acceptance. If a review takes longer than 14 days without resolution, the PR is escalated to the core team.

*Transition to ACCEPTED:* Two approvals received (domain expert + documentation reviewer). All quality gate checks passing. Author has addressed all review comments.

---

**ACCEPTED**

A document is `accepted` when it has been merged to main and is part of the official documentation. ACCEPTED documents are considered accurate and authoritative for their subject. ACCEPTED documents MAY still evolve — acceptance is not stabilization.

For `frozen` stability-class documents, `accepted` is effectively the final state before `deprecated`.

For `evolving` stability-class documents, `accepted` means the document is accurate for the current version and will be updated as the framework evolves.

*Transition to STABLE:* After one full version cycle has passed without significant changes, and after all cross-references to the document have been verified as correct.

---

**STABLE**

A document is `stable` when it has been `accepted` and verified through at least one release cycle. STABLE documents are the target state for all SPEC and THEORY documents. STABLE documents SHOULD only change when the underlying concept or specification changes.

Changes to STABLE documents require an approved PR with two reviewers. For `frozen` stability documents, changes additionally require an RFC.

*Transition to DEPRECATED:* When the subject of the document is deprecated in the framework. A `deprecated` notice MUST be added to the top of the document explaining what replaces it.

---

**DEPRECATED**

A document is `deprecated` when the thing it describes is no longer the recommended approach. DEPRECATED documents remain accessible for historical reference. They display a prominent deprecation notice. They MUST NOT be linked from non-deprecated documents as authoritative references.

DEPRECATED documents are maintained only for historical accuracy. Bug fixes to deprecated documents are accepted. New features are not.

*Transition to ARCHIVED:* After the deprecated framework feature has been removed (typically at the next major version boundary).

---

**ARCHIVED**

A document is `archived` when the thing it describes no longer exists in the current framework version. ARCHIVED documents are moved to a dedicated archive directory or versioned site. They are not visible in the default documentation view.

ARCHIVED documents are never deleted. They serve as the historical record of how the framework worked in prior versions.

### 5.2 Who Controls State Transitions

| Transition | Who Can Approve |
|---|---|
| PROPOSED → DRAFT | Any maintainer |
| DRAFT → REVIEW | The author (self-transition by submitting PR) |
| REVIEW → ACCEPTED | Two reviewers (domain + documentation) |
| ACCEPTED → STABLE | Documentation maintainer, after one release cycle |
| STABLE → DEPRECATED | Core team decision |
| DEPRECATED → ARCHIVED | Core team, at major version boundary |

---

## 6. Stability Model

Stability is a property of a document that describes how likely it is to change and what process is required to change it. Stability is independent of lifecycle status — a document can be ACCEPTED with EXPERIMENTAL stability, or STABLE with FROZEN stability.

### 6.1 Stability Levels

**FROZEN**

A `frozen` document will not change after it reaches `accepted` status, except to correct factual errors. Even corrections require an RFC. The document's content is a permanent commitment.

Documents with `frozen` stability include:
- All of `03-kernel/types/` (kernel type specifications)
- `03-kernel/invariants.md`
- `17-appendices/formal/architecture-laws.md`
- `17-appendices/formal/filter-grammar.md`
- `17-appendices/formal/state-machines.md`
- All ADRs (including rejected designs)
- All Architecture Laws

*Why:* These documents describe permanent decisions. The ecosystem builds on them. If they change, the ecosystem breaks. The `frozen` designation makes this commitment explicit.

**STABLE**

A `stable` document changes only when the underlying specification or concept changes. Changes require two reviewers and MUST not alter existing normative claims (only add new ones, correct errors, or improve clarity without changing meaning).

Documents with `stable` stability include:
- All THEORY documents
- `06-drivers/driver-contract.md`
- `06-drivers/conformance/**`
- `09-operations/disaster-recovery/partition-behavior.md`

*Why:* These documents are authoritative enough that change is disruptive, but the underlying concepts may genuinely evolve over time.

**EVOLVING**

An `evolving` document changes regularly with the framework. The content is accurate for the current version but SHOULD be expected to be updated with each minor version.

Documents with `evolving` stability include:
- All TUTORIAL documents
- All GUIDE documents in `08-app-dev/`
- All of `09-operations/`
- `14-contributing/**`
- `17-appendices/examples/**`

*Why:* These documents serve current users of the current version. Keeping them accurate is more important than stability.

**EXPERIMENTAL**

An `experimental` document describes something that has not yet stabilized. Content MAY change at any time without notice. Experimental documents display a prominent notice.

Documents with `experimental` stability include:
- `04-compiler/ir.md` (until the IR stabilizes at v1.2)
- `04-compiler/plugin-passes.md` (same timeline)

*Why:* Publishing experimental documentation is better than no documentation. The `experimental` designation protects readers from building on unstable foundations.

**DRAFT**

Documents in the writing phase. `draft` stability is temporary — it becomes one of the above levels upon acceptance.

### 6.2 Stability Inheritance Rule

If a document has `frozen` stability, every normative claim within it is `frozen`. If a document has `stable` stability, its normative claims are `stable`. Documents MUST NOT make normative claims that are more stable than the document itself.

Example: A `guide` document (category) with `evolving` stability MUST NOT make normative claims using MUST/SHALL. Such claims belong in a SPEC document with `frozen` or `stable` stability.

---

## 7. Cross-Reference Policy

### 7.1 Canonical Concept Location

Every concept has exactly one canonical location. The canonical location is the document where the concept is primarily defined. All other documents that need to discuss the concept reference the canonical location.

The canonical location of a term's definition is always `GLOSSARY.md`. The canonical location of a type's specification is always its corresponding file in `03-kernel/types/`. The canonical location of a theoretical explanation is always its file in `02-theory/`.

**DLAW-REF-001:** No document MUST define a term that is defined in the GLOSSARY.

**DLAW-REF-002:** No document MUST provide a complete specification of a kernel type outside `03-kernel/types/`.

**DLAW-REF-003:** When a document needs to discuss a concept whose canonical home is elsewhere, it MUST reference the canonical document, not restate the concept.

### 7.2 Cross-Reference Format

All cross-references MUST use relative Markdown links:

```markdown
<!-- Correct: relative link from current file's directory -->
See the [Filter DSL specification](../../03-kernel/types/filter.md) for the complete operator list.

<!-- Correct: link to a specific section -->
See [Compilation Phase 2](../../04-compiler/phases/02-name-resolution.md) for how references are resolved.

<!-- Incorrect: absolute path (breaks when repo moves) -->
See [Filter DSL](/awo/docs/03-kernel/types/filter.md).

<!-- Incorrect: URL to external site (breaks, cannot be verified) -->
See the filter documentation at https://...
```

**DLAW-REF-004:** All cross-references MUST use relative paths. Absolute paths are forbidden. External URLs to framework documentation are forbidden (the documentation is self-contained).

**DLAW-REF-005:** External URLs to non-framework resources (RFCs, Go language specification, PostgreSQL documentation) are permitted but MUST include the full resource name as link text so that the reference remains meaningful if the URL becomes invalid.

### 7.3 First-Use Rule

When a term is used for the first time in a document, it MUST be linked to its Glossary def. Subsequent uses in the same document do not require links.

```markdown
<!-- First use in document: link to glossary -->
Every [EntityDefinition](../../GLOSSARY.md#entitydefinition) drives five subsystems.

<!-- Subsequent uses: no link required -->
An EntityDefinition that declares a Currency field will...
```

### 7.4 Diagram References

Diagrams MUST be referenced in text. A diagram that appears on a page without a textual reference explaining its content is not accessible and does not serve its purpose.

```markdown
<!-- Correct: text introduces the diagram -->
The following diagram shows the relationship between the Registry (mutable, pre-compile)
and the CompiledSchema (immutable, post-compile). Note that no mutation path exists
from CompiledSchema back to Registry.

```mermaid
...
```

### 7.5 ADR References

When a design decision is made based on an ADR, the document implementing that decision SHOULD reference the ADR:

```markdown
<!-- Reference ADR for a design decision -->
The hook registration model uses an open string type for LifecycleStage
rather than a struct with fixed fields.
See [ADR-011: Hook Registration Model](../../13-adrs/ADR-011-hook-registration-model.md)
for the rationale.
```

ADR references are informational. They explain *why* the design exists. They do not affect the normative content of the document.

---

## 8. Terminology Governance

Terminology is the most important consistency mechanism in a long-lived documentation corpus. Term drift is invisible and catastrophic. This section defines the rules that prevent it.

### 8.1 Term Ownership

Every term has exactly one def. That definition lives in `GLOSSARY.md`. No term is defined anywhere else. No term is partially defined in one place and extended in another.

**DLAW-TERM-001:** A term is defined in exactly one place: `GLOSSARY.md`.

**DLAW-TERM-002:** No document outside `GLOSSARY.md` MUST provide a definition of a term. It MAY describe how a term applies in context, but MUST reference the Glossary for the definition itself.

### 8.2 Canonical Term Format

For each term, the Glossary entry contains:

```markdown
### EntityDefinition

**Category:** Core Concept
**Status:** Stable
**Since:** 1.0

The central metadata primitive of the Awo Framework. An `EntityDefinition` is a
Go value (typically a package-level variable) that declares everything the framework
needs to know about a kind of managed data: its fields, edges, policies, hooks,
actions, workflow triggers, and UI representation.

An `EntityDefinition` is not a Go struct def. It is a description of a
data concept, written in Awo's metadata language, that the compiler transforms into
routes, SQL templates, hook chains, policy evaluators, and UI trees.

**Not to be confused with:**
- `EntityRecord` — a specific instance of an entity (data, not description)
- `CompiledEntity` — the compiler's internal representation of an EntityDefinition
- Go struct — EntityDefinitions may describe entities whose data is stored as JSONB

**Related terms:** [Entity](#entity), [CompiledSchema](#compiledschema),
[Registry](#registry), [FieldDef](#fielddef)
```

### 8.3 Aliases and Synonyms

Some terms have informal equivalents used in conversation. These MUST be explicitly mapped to the canonical term in the Glossary and nowhere else.

```markdown
### Hook Chain

**Canonical term for:** BeforeCreateHook list, AfterSave hooks, lifecycle hooks

**Informal synonyms:** "hooks," "lifecycle hooks," "entity hooks"

**Usage note:** Always use "hook chain" when referring to the ordered sequence of hooks
compiled for a specific entity and lifecycle stage. Use "hook" when referring to a single
hook implementation.
```

**DLAW-TERM-003:** Informal synonyms MUST be listed in the Glossary. Documentation authors MUST use the canonical term, not the synonym. Synonyms exist in the Glossary only to help readers recognize terms they have encountered informally.

### 8.4 Capitalization

**Rule 1:** A term that refers to a specific Awo concept (a type, a system, a pattern) is capitalized when used as a proper noun.

```
Correct:   The EntityDefinition drives five subsystems.
Incorrect: The entity definition drives five subsystems.

Correct:   The CompiledSchema is immutable.
Incorrect: The compiled schema is immutable.

Correct:   Register the entity definition with the registry.
Incorrect: Register the EntityDefinition with the Registry.
```

The distinction: when referring to *a specific thing* (a particular entity definition, the registry instance), use the proper noun form. When using the concept generically ("the framework compiles entity definitions"), the common noun form is also acceptable if it avoids awkward repetition.

**Rule 2:** Package names (`def/`, `registry/`, `compiler/`) are always lowercase and formatted as code.

**Rule 3:** Architecture Laws are referenced by their identifier (LAW-001) and by their short name in Title Case: "The Immutable Compiled Schema Law."

### 8.5 Deprecated Terms

When a term is deprecated, the following process applies:

1. Add a deprecation notice to the Glossary entry specifying what replaces it.
2. Update all documents that use the deprecated term to use the replacement term.
3. Keep the deprecated entry in the Glossary for two major versions, then archive it.
4. The deprecated entry MUST link to its replacement.

**DLAW-TERM-004:** Deprecated terms MUST NOT be used in new documents. They MAY appear in ADRs and historical documents.

### 8.6 Version-Specific Terminology

Some terms evolve between major versions. The Glossary handles this by maintaining version-specific definitions when necessary:

```markdown
### FieldType

**Since:** 1.0

_(v1.0+)_ An open string type in `def/` representing the kind of data a field holds.
Third-party modules may define custom FieldType constants by registering a FieldType
handler with the compiler. Built-in FieldTypes are defined as constants in `def/`.
```

When a term's meaning changes between major versions, the Glossary creates a new entry for the new version's definition and archives the old one.

### 8.7 Abbreviations

Abbreviations MUST be defined on first use in every document, even if they are defined in the Glossary:

```markdown
<!-- First use: expand then abbreviate -->
The Row-Level Security (RLS) policy is enforced by the database.
```

Abbreviations that MUST always be expanded on first use: RLS, SDUI, IR, ADR, RFC, DI, CRUD, ERP, SLO, SLI, TTL.

---

## 9. Diagram Standards

### 9.1 Diagram Principles

Diagrams explain. They do not decorate. A diagram that requires more explanation than it provides is not useful. A diagram that makes a complex relationship immediately visible is essential.

Every diagram in the Awo documentation MUST:
- Explain something that is significantly harder to explain in prose alone
- Be referenced in the surrounding text
- Have a caption that states what the diagram shows (not what to do with it)
- Be accurate — diagrams that contradict the specification they illustrate are worse than no diagram

### 9.2 Allowed Formats

**Mermaid** is the standard diagram format. Mermaid diagrams are embedded directly in Markdown and render in all major documentation systems (GitHub, GitLab, Docusaurus, mdBook with plugin). They are version-controlled alongside the documentation they illustrate. When the specification changes, the diagram must be updated in the same PR.

Mermaid diagram types in use:

| Type | When to Use |
|---|---|
| `flowchart` | Process flows, decision trees, data flow |
| `sequenceDiagram` | Request/response flows, lifecycle sequences, protocol interactions |
| `classDiagram` | Type relationships, interface hierarchies |
| `stateDiagram-v2` | Lifecycle state machines |
| `erDiagram` | Entity relationships, data model |
| `graph` | Dependency graphs, package relationships |

**SVG** is permitted for diagrams that exceed Mermaid's capabilities (complex package dependency graphs, architectural overviews with many components). SVG files MUST be stored in `17-appendices/diagrams/`. SVG diagrams MUST have a corresponding Markdown source file in the same directory describing how the diagram was generated (so it can be regenerated when it goes stale).

**No raster images** (PNG, JPEG) for technical diagrams. Raster images cannot be version-controlled meaningfully, cannot be searched, and degrade when scaled. Exception: screenshots used in tutorials showing external tool output.

### 9.3 Diagram-to-Document Relationship

A diagram MUST live in the document that most directly references it, unless:

- The diagram is referenced by more than two documents (in which case it lives in `17-appendices/diagrams/` and is linked from each referencing document)
- The diagram represents a cross-cutting concern (in which case it lives in the most authoritative document that covers that concern)

**DLAW-DIAG-001:** A diagram embedded in a document MUST be accurate for the version of the specification in that document. When the specification changes, the diagram MUST be updated in the same commit.

**DLAW-DIAG-002:** Standalone SVG diagrams in `17-appendices/diagrams/` MUST include a Mermaid equivalent or a reconstruction specification so they can be regenerated when they go stale.

### 9.4 Diagram Naming

Diagrams stored in `17-appendices/diagrams/` follow this naming convention:

```
{section-number}-{subject}-{type}.{ext}

Examples:
04-compiler-compilation-flow-flowchart.md       (Mermaid source)
05-runtime-request-lifecycle-sequence.md        (Mermaid source)
03-kernel-package-dependencies-graph.svg        (SVG for complex graph)
02-theory-tenant-isolation-overview.md          (Mermaid source)
```

### 9.5 Diagram Captions

Every diagram MUST have a caption formatted as:

```markdown
*Figure N: [What this diagram shows]. [What the reader should take away.]*
```

Example:

```markdown
*Figure 1: Compilation pipeline from EntityDefinition input to CompiledSchema output.
The Registry accepts mutations only during Phase 1 (Registration). After Phase 10
(Schema Seal), all mutations are rejected and the schema is immutable.*
```

---

## 10. Writing Standards

### 10.1 Voice and Tone

**Voice:** Active voice. The framework does things. Implementations do things. Avoid passive constructions that obscure who is responsible.

```
Correct:   The compiler validates all entity names at Phase 2.
Incorrect: Entity names are validated during compilation.

Correct:   The EntityStore driver MUST return ErrMissingTenantContext if called
           without tenant context.
Incorrect: ErrMissingTenantContext is returned when tenant context is absent.
```

**Tone:** Technical and neutral. No marketing language, no advocacy, no enthusiasm. State facts. State requirements. State trade-offs. Do not sell the framework — describe it.

**Person:**
- SPEC documents: third person or imperative ("The driver MUST..." / "Implementations SHALL...")
- THEORY documents: first person plural when attributing design decisions ("We chose... because..."), third person otherwise
- GUIDE documents: second person ("When you define a policy...") or third person
- TUTORIAL documents: second person imperative ("Create a file...", "Run...")
- ADR documents: first person plural for the decision ("We decided...", "The team concluded...")

### 10.2 RFC 2119 Language Usage

RFC 2119 keywords MUST appear in uppercase when used normatively. They MUST NOT appear in uppercase when used colloquially.

```
Normative (uppercase — binding):
The driver MUST return an error if tenant context is not set.
Implementations SHOULD cache compiled schemas for reuse across requests.
Module authors MAY register custom compiler passes.

Colloquial (lowercase — guidance, not binding):
In most cases you should prefer system entities for financial data.
You may want to add a custom compiler pass for compliance checking.
```

RFC 2119 keywords MUST only appear in documents with `normative-level: normative` or in normative sections of `normative-level: mixed` documents. These keywords MUST NOT appear in TUTORIAL, GUIDE, or ANTI documents.

### 10.3 Section Ordering

Every SPEC document MUST follow this section order:

1. Purpose (one paragraph: what this document specifies and why)
2. Prerequisites (what to read before this document)
3. Overview (conceptual explanation before detail)
4. Specification (the normative content)
5. Invariants (what must always hold)
6. Error conditions (what happens when the specification is violated)
7. Compatibility (what can change without breaking, what cannot)
8. Related documents

Every THEORY document MUST follow:

1. Problem statement (what problem does this concept solve?)
2. Design goals (what constraints shaped the solution?)
3. The concept (explanation)
4. Why this approach (trade-offs, alternatives)
5. Implications (what this means for other parts of the framework)
6. Related documents

Every TUTORIAL document MUST follow:

1. What you will build (outcome, not process)
2. Prerequisites (what you need to have set up)
3. Steps (numbered, sequential, each producing a testable result)
4. Complete working example (all files in their final state)
5. What to do when things go wrong (common failure modes)
6. Next steps (where to go after completing this tutorial)

### 10.4 Headings

- H1 (`#`): Document title. Exactly one per document. Matches the `title` frontmatter field.
- H2 (`##`): Major sections. Numbered (1, 2, 3...) in SPEC and THEORY documents. Unnumbered in TUTORIAL and GUIDE documents.
- H3 (`###`): Subsections.
- H4 (`####`): Sub-subsections. Use sparingly. If more than three H4 headings appear under one H3, consider restructuring.
- H5+ (`#####`): Forbidden. Restructure instead.

Headings MUST be descriptive. "Overview" is an acceptable H2 at the start of a document. "Overview" as an H3 under another section is not — it describes nothing.

### 10.5 Code Blocks

All code MUST appear in fenced code blocks with a language identifier:

````markdown
```go
type FieldType string

const (
    FieldTypeData     FieldType = "data"
    FieldTypeCurrency FieldType = "currency"
)
```
````

```sql
ALTER TABLE finance_invoice ENABLE ROW LEVEL SECURITY;
```

```yaml
title: "Example document"
status: draft
```

Code blocks in SPEC documents SHOULD show the interface or type signature, not an implementation. Code blocks in TUTORIAL documents SHOULD show complete, working, copy-pasteable code.

**DLAW-CODE-001:** Code in SPEC documents MUST accurately represent the actual API. If the API changes, the code in the SPEC MUST be updated in the same PR.

### 10.6 Warnings and Notes

Three callout types are defined:

```markdown
> **Note:** Additional context that helps understanding but is not critical.

> **Important:** Information the reader MUST NOT overlook to use the feature correctly.

> **Warning:** Information about consequences that may be irreversible or cause data loss.
```

Callouts MUST NOT be used for normative statements. Normative statements belong in the specification prose with RFC 2119 language.

### 10.7 Examples in Specification Documents

Examples in SPEC documents serve one purpose: to clarify a normative statement that is ambiguous without an example. They are not tutorials. They MUST:

- Be the minimum necessary to illustrate the point
- Be marked as examples, not as prescriptive code
- Accurately reflect the current API
- Not introduce new concepts not already defined

```markdown
For example, a filter that selects all active invoices with a total greater than
10,000 KES is expressed as:

```go
filter.And(
    filter.Eq("status", "Active"),
    filter.Gt("total_kes", decimal.New(10000, 0)),
)
```

This example does not constitute a recommendation to structure filters this way in
production code; it illustrates the composition of `And`, `Eq`, and `Gt` operators.
```

---

## 11. Architecture Laws for Documentation

These laws are the documentation system's counterpart to the framework's Architecture Laws. They are binding on all documentation authors and reviewers. Violation of any Documentation Law is grounds for PR rejection.

Each law has an identifier (DLAW-NNN), a statement, a motivation, and consequences of violation.

---

**DLAW-001 — One Definition Per Term**

*Statement:* Every term used in the documentation MUST be defined exactly once, in `GLOSSARY.md`. No other document MUST define a term.

*Motivation:* Multiple definitions of the same term will diverge. Over years of independent editing, the Glossary definition and the inline definition will describe subtly different things. Readers who encounter both will hold an inconsistent mental model.

*Consequence:* A PR that introduces a definition of a term outside the Glossary is rejected. If the Glossary does not have the term, the term is added to the Glossary in the same PR, not defined inline.

---

**DLAW-002 — Tutorials Never Define Architecture**

*Statement:* TUTORIAL documents MUST NOT make normative architectural claims. They MUST reference the authoritative SPEC or THEORY document for any architectural concept they use.

*Motivation:* Tutorials are written to be accessible and are updated frequently. If architectural truths are stated in tutorials, they will be stated imprecisely (for accessibility) and will drift from the SPEC during updates. Readers who learn architecture from tutorials learn it wrong.

*Consequence:* A tutorial that says "the Registry must be frozen before..." instead of linking to the relevant SPEC is defective. The tutorial must instead say "the Registry must be frozen (see [Registry Lifecycle](link)) before..."

---

**DLAW-003 — Specifications Never Contain Tutorials**

*Statement:* SPEC documents MUST NOT contain tutorial content (step-by-step instructions, "let's build...", prescriptive code walkthroughs).

*Motivation:* Specifications define what is true. Tutorials teach tasks. A specification that contains a tutorial trains readers to read specifications as tutorials — which causes them to miss normative statements that are not framed as steps.

*Consequence:* Any step-by-step instruction ("First, define the entity. Next, add a field...") in a SPEC document is removed and placed in an appropriate TUTORIAL or GUIDE document, referenced from the SPEC.

---

**DLAW-004 — Theory Never References Application Examples**

*Statement:* THEORY documents MUST NOT reference application-level examples (specific entities, specific modules, specific ERP workflows) to explain theoretical concepts.

*Motivation:* Theory documents are read before implementation guides. Referencing application examples in theory documents creates a dependency cycle (theory → examples → guides → theory). It also obscures the theory behind implementation detail.

*Consequence:* A theory document that explains compilation using the `finance_invoice` entity as its example is defective. The theory document uses abstract examples ("an entity named E with fields F1 and F2") or no examples.

---

**DLAW-005 — ADRs Are Immutable**

*Statement:* Once an ADR (or rejected design record) reaches `accepted` status, its content MUST NOT be modified. New information about the decision is recorded in a new ADR that references the original.

*Motivation:* ADRs are the historical record of why things are the way they are. A modified ADR no longer records what was actually decided — it records what someone later wanted to have decided. This destroys the value of the historical record.

*Consequence:* Any PR that modifies an accepted ADR (other than fixing a typographical error in a way that does not change meaning) is rejected. If the decision was wrong, a new ADR documents the new decision and references the old one.

---

**DLAW-006 — RFC 2119 Keywords Only in Normative Documents**

*Statement:* The uppercase words MUST, MUST NOT, SHALL, SHALL NOT, SHOULD, SHOULD NOT, MAY appearing in a document's prose are interpreted as RFC 2119 keywords. These words MUST NOT appear in uppercase in documents with `normative-level: informative`.

*Motivation:* If "MUST" appears in a tutorial or guide, readers believe they are reading a normative requirement. They will implement accordingly — and may implement incorrectly when they encounter a case the tutorial author did not anticipate.

*Consequence:* Any PR to a TUTORIAL, GUIDE, or ANTI document that introduces RFC 2119 uppercase keywords is rejected. Lowercase versions ("you must", "you should") are permitted in informative documents.

---

**DLAW-007 — Every Document Has Complete Metadata**

*Statement:* Every document committed to `awo/docs/` MUST have a complete YAML frontmatter block with all required fields filled in (not marked TBD). Incomplete metadata is a PR blocker.

*Motivation:* Metadata enables automated tooling (link checking, dependency graph generation, quality gates). Documents with incomplete metadata are invisible to tooling and may have broken links or incorrect stability classifications that are not caught until a reader encounters them.

*Consequence:* The CI quality gate checks all required metadata fields. A PR that adds a document with missing metadata fields fails CI and is not merged.

---

**DLAW-008 — No Orphan Documents**

*Statement:* Every document MUST be reachable from `docs/README.md` by following links through section READMEs. A document that is not linked from any navigation path is an orphan and MUST NOT be merged.

*Motivation:* Orphan documents are documentation that does not exist from the reader's perspective. They accumulate, go stale, contradict accepted documents, and are discovered only when someone is searching for something else.

*Consequence:* Adding a document requires adding it to the relevant section README. CI checks that every file in `docs/` is reachable from `docs/README.md`.

---

**DLAW-009 — Diagrams Are Synchronized With Specifications**

*Statement:* When a specification changes in a way that affects a diagram, the diagram MUST be updated in the same PR as the specification change.

*Motivation:* A diagram that contradicts the specification it illustrates is worse than no diagram. It teaches readers the wrong mental model with false visual authority.

*Consequence:* A PR that changes a specification without updating affected diagrams is rejected if the diagram now contradicts the specification.

---

**DLAW-010 — Frozen Documents Require RFC to Change**

*Statement:* A document with `stability: frozen` MUST NOT be modified (except for typographical corrections that do not change meaning) without an approved RFC that explicitly authorizes the change.

*Motivation:* Frozen documents represent permanent commitments. Modifying them without a formal review process undermines those commitments. The RFC process ensures that changes to frozen documents are deliberate, reviewed, and recorded.

*Consequence:* A PR that modifies a frozen document without an approved, linked RFC is rejected.

---

**DLAW-011 — Stability Flows Downward**

*Statement:* A document MUST NOT make normative claims that are more stable than the document itself. A guide (evolving) cannot contain a normative claim that should be `frozen`. That claim belongs in a SPEC document.

*Motivation:* If a guide says "the workflow ID format is permanently `{tenant}.{type}.{id}.{event}.{fn}`," readers will rely on this as a frozen guarantee. When the guide is updated (because guides evolve), the frozen guarantee may be accidentally modified. Guarantees belong in frozen SPEC documents.

*Consequence:* A PR that introduces a normative guarantee (using RFC 2119 language or explicit "permanent" / "frozen" language) into an evolving document is rejected unless the document's stability is simultaneously upgraded and the change goes through the SPEC review process.

---

**DLAW-012 — Examples Reflect the Current API**

*Statement:* All code examples in documentation MUST reflect the current version of the API they illustrate. Stale examples are defects.

*Motivation:* A developer who copies a stale example produces code that does not compile or does not behave as documented. This is a defect with no useful error message — the code is wrong in a way that appears correct.

*Consequence:* Every release cycle includes an examples audit. PRs that change public APIs MUST simultaneously update all examples that use those APIs.

---

## 12. Documentation Governance

### 12.1 Proposing New Documents

New documents are proposed as GitHub issues tagged `doc:proposed`. The proposal must include:

1. **Title:** Proposed document title
2. **Category:** Which document category (SPEC, THEORY, GUIDE, etc.)
3. **Stability:** Proposed stability level
4. **Audience:** Who will read it
5. **Dependency level:** Where in the dependency hierarchy it belongs
6. **Justification:** Why this document is needed (what gap it fills)
7. **Dependencies:** Which existing documents it will depend on
8. **Conflicts:** Does it duplicate or conflict with any existing document?

A maintainer reviews the proposal within 7 days. If approved, the proposal is assigned to a writer and transitions to DRAFT.

### 12.2 Changing Existing Documents

Changes to existing documents are submitted as PRs. The change category determines the review requirements:

| Document Stability | Change Type | Required Reviews |
|---|---|---|
| `frozen` | Typographic correction | 1 reviewer |
| `frozen` | Any content change | Rejected without RFC |
| `stable` | Clarification (no semantic change) | 2 reviewers |
| `stable` | Semantic change | 2 reviewers + domain expert |
| `evolving` | Any change | 1 reviewer |
| `experimental` | Any change | 1 reviewer |

### 12.3 RFC Process

An RFC is required for:
- Any change to a `frozen` document
- Adding a new section to `03-kernel/types/`
- Changing the metadata standard
- Changing the documentation dependency hierarchy
- Adding or modifying a Documentation Law
- Deprecating a SPEC document

**RFC lifecycle:**

```
OPEN (30 days minimum comment period)
  ↓
REVIEW (core team discussion)
  ↓
ACCEPTED or REJECTED (core team vote, 2/3 majority)
  ↓
IMPLEMENTED (changes merged, RFC archived)
```

An RFC document is placed in `00-documentation/rfcs/RFC-NNN-title.md` and follows the RFC template.

### 12.4 ADR Process

An ADR is written when:
- A significant architectural decision is made
- A design is rejected (REJ prefix)
- A previous decision is revisited and upheld or overturned

ADRs are drafted in `13-adrs/` and follow the ADR template (`13-adrs/template.md`). ADRs are reviewed by at least two core team members. Once accepted, they are immutable.

### 12.5 Release Synchronization

Documentation is synchronized with framework releases as follows:

| Release type | Documentation requirement |
|---|---|
| Patch (x.x.N) | Correct errors in existing documents only. No new SPEC content. |
| Minor (x.N.0) | New GUIDE and TUTORIAL content. New REF entries. SPEC additions (additive only). |
| Major (N.0.0) | May include breaking changes to SPEC. Requires RFC for each breaking change. Previous version docs archived. |

Every release MUST include a documentation audit verifying that all examples compile against the released version.

### 12.6 Documentation Versioning

The `awo/docs/` directory always reflects the current unreleased main branch. Released version documentation is served from versioned directories or a versioned documentation hosting system.

Version markers in documents use the `since` frontmatter field. Content added in v1.2 is marked:

```markdown
> **Added in v1.2:** The `stabilizes-in` frontmatter field is optional and indicates
> when an experimental document is expected to reach stable status.
```

Removed content is not deleted from SPEC documents — it is marked deprecated with `since-removed: "1.2"` and explained in the CHANGELOG.

---

## 13. The `00-documentation/` Root Section

A `00-documentation/` section MUST exist as the first section of `awo/docs/`. It contains the meta-layer: the governance, standards, and self-documentation of the documentation system itself.

Its number prefix (`00`) ensures it sorts before all framework documentation sections, communicating that it is the prerequisite for contributing to any other section.

### 13.1 Directory Structure

```
awo/docs/00-documentation/
├── README.md
├── documentation-architecture.md    ← this document
├── documentation-philosophy.md
├── document-types.md
├── metadata-standard.md
├── document-lifecycle.md
├── stability-model.md
├── terminology-governance.md
├── diagram-standards.md
├── writing-standards.md
├── cross-reference-policy.md
├── architecture-laws.md
├── review-process.md
├── versioning-policy.md
├── governance.md
├── quality-gates.md
├── style-guide.md
├── adr-template.md
├── rfc-template.md
└── rfcs/
    └── README.md
```

### 13.2 File Specifications

**`README.md`**
Entry point for documentation contributors. Explains what this section is, why it exists, and which file to read first. Lists all files with one-sentence descriptions.
Category: GUIDE | Stability: stable | Audience: documentation-authors, contributors

**`documentation-architecture.md`** *(this document)*
The complete documentation constitution. All governance rules and standards.
Category: SPEC | Stability: frozen | Audience: all

**`documentation-philosophy.md`**
The seven principles (Section 1.4) expanded into a standalone readable document. More discursive than the specification, intended to build conviction rather than enumerate rules.
Category: THEORY | Stability: frozen | Audience: all

**`document-types.md`**
The complete taxonomy (Section 2) as a standalone reference document. Checklists for each document type: what fields are required, what writing style is required, what the review process is.
Category: REF | Stability: stable | Audience: documentation-authors

**`metadata-standard.md`**
The complete frontmatter specification (Section 4) with examples, field validation rules, and the CI check specification.
Category: SPEC | Stability: stable | Audience: documentation-authors

**`document-lifecycle.md`**
The lifecycle model (Section 5) as a reference for authors and reviewers. Includes the transition checklist for each state change.
Category: REF | Stability: stable | Audience: documentation-authors, maintainers

**`stability-model.md`**
The stability levels (Section 6) with the list of which sections belong at which stability level, updated as sections are added.
Category: REF | Stability: stable | Audience: documentation-authors, reviewers

**`terminology-governance.md`**
The terminology rules (Section 8) with examples of correct and incorrect usage. Serves as the reference when a reviewer is uncertain whether a term is being used correctly.
Category: SPEC | Stability: stable | Audience: documentation-authors, reviewers

**`diagram-standards.md`**
The diagram rules (Section 9) with examples of good and bad diagrams. Includes the Mermaid reference for each supported diagram type.
Category: REF | Stability: stable | Audience: documentation-authors

**`writing-standards.md`**
The writing conventions (Section 10) as a practical style guide. Includes before/after examples for voice, tone, RFC 2119 usage, and code blocks.
Category: REF | Stability: stable | Audience: documentation-authors

**`cross-reference-policy.md`**
The cross-reference rules (Section 7) with examples of correct link formats, first-use rules, and diagram references.
Category: SPEC | Stability: stable | Audience: documentation-authors

**`architecture-laws.md`**
All 12 Documentation Laws (Section 11) as a standalone reference. Linked from every PR template and review checklist.
Category: LAW | Stability: frozen | Audience: all

**`review-process.md`**
The complete review workflow for documentation PRs. Who reviews what, what they look for, how to give feedback, how to resolve disagreements.
Category: GUIDE | Stability: evolving | Audience: reviewers, documentation-authors

**`versioning-policy.md`**
How documentation versioning works, what changes at each release type, how archived versions are maintained.
Category: SPEC | Stability: stable | Audience: core-team, maintainers

**`governance.md`**
The governance model (Section 12): how documents are proposed, changed, deprecated. The RFC and ADR processes. Release synchronization.
Category: SPEC | Stability: stable | Audience: core-team, maintainers, contributors

**`quality-gates.md`**
The complete PR quality checklist (Section 14). Machine-readable format for CI scripts. Updated when new checks are added.
Category: REF | Stability: evolving | Audience: documentation-authors, CI system

**`style-guide.md`**
A condensed, practical writing guide for documentation authors. The most frequently consulted document in this section. Distills writing-standards.md into quick-reference checklists.
Category: GUIDE | Stability: evolving | Audience: documentation-authors

**`adr-template.md`**
The mandatory template for all ADR documents. Every ADR MUST use this template.
Category: REF | Stability: stable | Audience: core-team, contributors

**`rfc-template.md`**
The mandatory template for all RFC documents.
Category: REF | Stability: stable | Audience: core-team, contributors

**`rfcs/README.md`**
Index of all RFCs, their status, and links.
Category: REF | Stability: evolving | Audience: all

---

## 14. Quality Gates

Every documentation PR MUST pass all applicable quality gates before merge. Quality gates are automated where possible, manual where not.

### 14.1 Automated Gates (CI)

These checks run automatically on every PR touching `docs/`:

```
GATE-AUTO-001  All required frontmatter fields are present and non-empty
GATE-AUTO-002  Document ID is unique across all documents in docs/
GATE-AUTO-003  Document status is a valid lifecycle state
GATE-AUTO-004  Document category is a valid category code
GATE-AUTO-005  Document stability is a valid stability level
GATE-AUTO-006  All relative links in the document resolve to existing files
GATE-AUTO-007  All links to external resources return HTTP 200 (weekly CI run)
GATE-AUTO-008  Document is reachable from docs/README.md via navigation links
GATE-AUTO-009  No RFC 2119 uppercase keywords in informative documents
GATE-AUTO-010  All code blocks have a language identifier
GATE-AUTO-011  Document H1 heading matches frontmatter title field
GATE-AUTO-012  No broken anchor references (links to #sections)
GATE-AUTO-013  Mermaid diagram syntax is valid (rendered without error)
GATE-AUTO-014  Documents with stability: frozen have not been modified without
               a linked RFC in the PR description
GATE-AUTO-015  No term defined outside GLOSSARY.md (heuristic: check for
               "**Definition:**" or "is defined as" outside GLOSSARY)
```

### 14.2 Manual Reviewer Checklist

Reviewers verify these items for every PR:

**Terminology**
- [ ] All terms used are defined in GLOSSARY.md
- [ ] Terms are used consistently with their Glossary definitions
- [ ] No new term is introduced without a Glossary PR
- [ ] Capitalization follows terminology governance rules
- [ ] RFC 2119 keywords are used correctly (uppercase only in normative documents)

**Content**
- [ ] The document's content matches its declared category (SPEC, THEORY, GUIDE, etc.)
- [ ] The document's scope does not overlap with any existing document's scope
- [ ] All normative claims use RFC 2119 language
- [ ] No concept is fully explained that has a canonical home elsewhere
- [ ] Stability of normative claims matches or is lower than document stability

**Cross-References**
- [ ] New terms are linked to GLOSSARY on first use
- [ ] Concepts with canonical homes elsewhere are referenced, not restated
- [ ] ADRs are referenced where relevant
- [ ] Related documents are listed in frontmatter

**Diagrams**
- [ ] Every diagram is referenced in surrounding prose
- [ ] Every diagram has a caption
- [ ] Diagrams are accurate for the current specification
- [ ] If specification changed, diagrams are updated in the same PR

**Structure**
- [ ] Section ordering matches the standard for the document category
- [ ] Headings are descriptive (not "Overview" or "Introduction" except as H2)
- [ ] No H5+ headings
- [ ] Code examples are realistic and compile against current API

**Lifecycle**
- [ ] Frontmatter status matches actual document state
- [ ] If status changed, the transition was authorized by the appropriate party
- [ ] If stability is frozen and content changed, an RFC is linked

**Architecture Laws**
- [ ] DLAW-001: No term defined outside GLOSSARY ✓
- [ ] DLAW-002: Tutorial does not define architecture ✓
- [ ] DLAW-003: Specification contains no tutorial content ✓
- [ ] DLAW-004: Theory contains no application examples ✓
- [ ] DLAW-005: ADR was not modified if accepted ✓
- [ ] DLAW-006: RFC 2119 keywords in normative documents only ✓
- [ ] DLAW-007: All required metadata fields present ✓
- [ ] DLAW-008: Document is reachable from README ✓
- [ ] DLAW-009: Diagrams synchronized with specifications ✓
- [ ] DLAW-010: Frozen documents not modified without RFC ✓
- [ ] DLAW-011: Stability flows downward (no evolving doc with frozen claims) ✓
- [ ] DLAW-012: All code examples reflect current API ✓

### 14.3 Domain Expert Review Checklist

For SPEC and THEORY documents, a domain expert additionally verifies:

- [ ] All technical claims are accurate
- [ ] The specification is complete (no edge cases left unspecified)
- [ ] The specification is consistent with the source code behavior (or the source code deviation is documented as a bug)
- [ ] The specification is consistent with all other SPEC documents it references
- [ ] No normative claims contradict an Architecture Law

### 14.4 Blocking vs. Non-Blocking Gates

All automated gates (GATE-AUTO-001 through GATE-AUTO-015) are blocking. A PR that fails any automated gate MUST NOT be merged.

Manual reviewer items are blocking for SPEC and LAW documents. For GUIDE and TUTORIAL documents, items may be raised as non-blocking comments if the reviewer judges the deviation to be minor — but the deviation MUST be logged as a separate issue and addressed within one release cycle.

---

## Appendix A — Summary Reference

**Documentation Laws (DLAW)**

| ID | Summary |
|---|---|
| DLAW-001 | One definition per term, in GLOSSARY only |
| DLAW-002 | Tutorials do not define architecture |
| DLAW-003 | Specifications contain no tutorial content |
| DLAW-004 | Theory does not reference application examples |
| DLAW-005 | ADRs are immutable after acceptance |
| DLAW-006 | RFC 2119 keywords only in normative documents |
| DLAW-007 | Every document has complete metadata |
| DLAW-008 | No orphan documents |
| DLAW-009 | Diagrams synchronized with specifications |
| DLAW-010 | Frozen documents require RFC to change |
| DLAW-011 | Stability flows downward |
| DLAW-012 | Examples reflect the current API |

**Document Categories**

| Code | Name | RFC 2119? | Lifetime |
|---|---|---|---|
| SPEC | Normative Specification | Required | Permanent |
| THEORY | Theory | No | Permanent |
| REF | Reference | Selective | Stable |
| GUIDE | Conceptual Guide | No | Stable |
| TUTORIAL | Tutorial | No | Evolving |
| ADR | Architecture Decision Record | No | Permanent |
| RFC | Request for Comments | Yes | Permanent |
| LAW | Architecture Law | Mandatory | Permanent |
| GLOSSARY | Canonical Term Definition | No | Permanent |
| ANTI | Anti-Pattern | No | Stable |
| APPENDIX | Supplementary Material | Varies | Varies |

**Stability Levels**

| Level | Change policy | Examples |
|---|---|---|
| frozen | RFC required even for corrections | Kernel specs, ADRs, Laws |
| stable | Two reviewers required | Theory, driver contracts |
| evolving | One reviewer required | Guides, tutorials, operations |
| experimental | One reviewer required | IR spec, plugin pass spec |
| draft | No restriction | In-progress work |

---

*This document is the foundation of the Awo Framework documentation system. It is itself subject to the rules it defines. It has `stability: frozen`. Changes require an RFC. It will remain valid as long as the Awo Framework exists.*

*DAS-001 | Accepted | 2025-07-06*
