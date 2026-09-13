> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "ADR Template"
id: gov-adr-template
status: accepted
category: TEMPLATE
stability: STABLE
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: informative
related:
  - "[Architecture Decision Records](../17-adr/README.md)"
  - "[Documentation Architecture](documentation-architecture.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# ADR Template

Copy this template when writing a new Architecture Decision Record.

---

## How to Use This Template

1. Copy this file to `awo/docs/17-adr/adr-{NNN}-{title-slug}.md`
2. Fill in all sections
3. Submit as a pull request for architecture review
4. After acceptance, the ADR is frozen — do not modify content
5. If the decision is reversed, write a new ADR superseding this one

ADR numbers are assigned sequentially. Check the [ADR Index](../17-adr/README.md) for the next available number.

---

## Template

```markdown
---
title: "ADR-NNN: {Title}"
id: adr-NNN
status: proposed          # proposed | accepted | rejected | superseded
category: ADR
stability: FROZEN         # always FROZEN for accepted ADRs
audience: [module-authors, framework-authors]
since: "{version}"
normative-level: informative
related:
  - "[Related Document](../path/to/document.md)"
supersedes: ""            # if this ADR supersedes another: "ADR-NNN"
superseded-by: ""         # filled when this ADR is superseded
---

# ADR-NNN: {Title}

**Status:** Proposed
**Date:** YYYY-MM-DD
**Authors:** {Team or individuals}

---

## Context

{Describe the situation and the problem that needs to be solved.
What is the force that makes this decision necessary?
What are the constraints and goals?}

---

## Options Considered

### Option A: {Name}

{Describe the option}

Pros:
- {Benefit}

Cons:
- {Drawback}

### Option B: {Name}

{...}

### Option C: {Name}

{...}

---

## Decision

**{State the decision in one sentence.**

{Explain why this option was chosen over the alternatives.}

---

## Consequences

**Positive:**
- {Benefit that results from this decision}

**Negative:**
- {Drawback or constraint that results from this decision}

**Architecture Laws generated (if any):**
- {LAW-NNN or INV-NNN that exists because of this decision}

---

## Revisit Trigger

{Under what circumstances should this decision be reconsidered?
What would make a different option correct in the future?}
```

---

## Quality Checklist for ADR Review

- [ ] Context section explains the problem, not the solution
- [ ] At least two alternatives considered and rejected with clear reasoning
- [ ] Decision stated clearly in one sentence
- [ ] Consequences include both positive and negative
- [ ] Revisit trigger is specific and actionable (not "if things change")
- [ ] Related documents linked correctly
- [ ] No personal opinions — only technical tradeoffs
