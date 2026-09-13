> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "RFC Template"
id: gov-rfc-template
status: accepted
category: TEMPLATE
stability: STABLE
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: informative
related:
  - "[Review Process](review-process.md)"
  - "[ADR Template](adr-template.md)"
  - "[Documentation Architecture](documentation-architecture.md)"
---

# RFC Template

An RFC (Request for Comments) is used to propose changes to FROZEN documents or significant architectural changes that are not yet ready to become ADRs. Copy this template when writing a new RFC.

---

## When to Write an RFC vs. an ADR

| RFC | ADR |
|---|---|
| Proposing a change (not yet decided) | Recording a decision (already made) |
| Seeking feedback on trade-offs | Documenting the chosen option and rationale |
| Changing a FROZEN document | Capturing why an option was rejected |
| Significant feature addition | Architectural decision with lasting consequences |

An RFC may result in an ADR once the RFC achieves consensus and the decision is made.

---

## Template

```markdown
---
title: "RFC-NNN: {Title}"
id: rfc-NNN
status: draft           # draft | open | accepted | rejected | withdrawn
category: RFC
stability: EXPERIMENTAL
audience: [module-authors, framework-authors]
since: "{version}"
normative-level: informative
related:
  - "[Document Being Changed](../path/to/document.md)"
---

# RFC-NNN: {Title}

**Status:** Draft
**Date:** YYYY-MM-DD
**Authors:** {Team or individuals}
**Comment Period:** {YYYY-MM-DD to YYYY-MM-DD} (7 days minimum)

---

## Summary

{One-paragraph summary of the proposed change.}

---

## Motivation

{Why is this change needed? What problem does it solve?
What happens if we don't make this change?}

---

## Detailed Design

{Precise description of the proposed change.
If changing a FROZEN document: show before/after.
If proposing a new feature: show the full design.}

### Before

```go
// current behavior
```

### After

```go
// proposed behavior
```

---

## Impact Analysis

### Breaking Changes

{List every breaking change. If none: "None."}

### Documents Affected

{List every document that must be updated if this RFC is accepted.}

### Code Affected

{List every interface, type, or function that changes.}

---

## Alternatives Considered

### Alternative A: {Name}

{Why was this rejected?}

### Alternative B: {Name}

{Why was this rejected?}

---

## Open Questions

{List questions that must be resolved before this RFC can be accepted.}

1. {Question}
2. {Question}

---

## Acceptance Criteria

{What must be true for this RFC to be considered accepted?}

- [ ] {Criterion}
- [ ] {Criterion}
```

---

## RFC Lifecycle

| Status | Meaning |
|---|---|
| `draft` | Being written; not open for comment yet |
| `open` | Open for comment (7-day minimum comment period) |
| `accepted` | Architecture team voted to accept |
| `rejected` | Architecture team voted to reject |
| `withdrawn` | Author withdrew before vote |

RFCs that are accepted SHOULD result in a corresponding ADR recording the decision. RFCs that are rejected are retained as historical record — do not delete.
