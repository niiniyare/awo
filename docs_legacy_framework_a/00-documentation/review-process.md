> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Review Process"
id: gov-review
status: accepted
category: SPEC
stability: STABLE
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[Documentation Architecture](documentation-architecture.md)"
  - "[Stability Model](stability-model.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Review Process

**Status: Accepted | Stability: Stable**

This document specifies the review process for new and updated documentation, approval requirements by stability level, and the change authority hierarchy.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. Change Authority by Stability Level

| Document Stability | Change Type | Required Approvers |
|---|---|---|
| EXPERIMENTAL | Any | 1 maintainer |
| STABLE | Additive, clarifications | 1 maintainer |
| STABLE | Removal, restructure | 2 maintainers + architecture review |
| FROZEN | Clarifying edits | 2 maintainers |
| FROZEN | Substantive change | RFC + majority architecture vote |
| FROZEN | Breaking change | New ADR + full architecture review |

---

## 2. Review Process Steps

### For EXPERIMENTAL and STABLE Additive Changes

1. Author opens PR with new or updated document
2. PR description explains: what changed, why, which existing documents are affected
3. Checklist (see §4) completed by author
4. One maintainer reviews: correctness, standards compliance, cross-references
5. Merge after approval

### For STABLE Structural Changes

1. Author opens a discussion issue describing the proposed change
2. Architecture team discusses (async, 5-day window)
3. If consensus: author opens PR
4. Two maintainers review
5. Merge after both approvals

### For FROZEN Document Changes

1. Author writes an RFC (use [RFC Template](rfc-template.md))
2. RFC submitted as PR for discussion
3. Architecture team votes (majority required)
4. If approved: RFC merged, then document change PR opened
5. Two maintainers review document change PR
6. Merge after both approvals

---

## 3. ADR Process

ADRs follow a specialized process (see [ADR Template](adr-template.md)):

1. Author drafts ADR with status `proposed`
2. ADR submitted as PR — open for 7-day comment period
3. Architecture team discusses alternatives and decision
4. If accepted: status changed to `accepted`; merged; frozen immediately
5. If rejected: status changed to `rejected`; merged as historical record

Rejected ADRs MUST be merged (not deleted) — the decision-making process is part of the historical record.

---

## 4. Author Checklist

Before submitting a documentation PR, the author MUST verify:

- [ ] Frontmatter complete: `title`, `id`, `status`, `category`, `stability`, `audience`, `since`, `normative-level`, `related`
- [ ] `related:` uses markdown link format `"[Title](path)"`
- [ ] RFC 2119 language (MUST/SHALL) only in `normative` documents
- [ ] All cross-references use relative paths
- [ ] All terms used are defined in GLOSSARY.md
- [ ] Code examples compile (or are explicitly marked as pseudocode)
- [ ] Mermaid diagrams render without errors
- [ ] "Related Documents" section present and complete
- [ ] No broken links (test with markdown link checker)
- [ ] Document correctly placed in the dependency order (no forward references to unreleased docs)

---

## 5. Reviewer Checklist

Reviewers MUST verify:

- [ ] Content is technically accurate
- [ ] Content does not contradict any FROZEN document
- [ ] Content does not contradict any Architecture Law or Invariant
- [ ] All Glossary terms used correctly
- [ ] Cross-references reciprocal (if A links B, B links A)
- [ ] Stability level appropriate for the content maturity
- [ ] No RFC 2119 language in informative documents

---

## 6. Emergency Changes

For security-related corrections to any stability level:
- One maintainer approval sufficient
- Tagged `security-fix` in PR
- Architecture team notified in parallel (not as blocker)

Security fixes take precedence over all change authority rules. A security-incorrect FROZEN document MUST be corrected immediately.

---

## Related Documents

- [Documentation Architecture](documentation-architecture.md) — governance hierarchy
- [Stability Model](stability-model.md) — what each stability level means
- [ADR Template](adr-template.md) — ADR process details
- [RFC Template](rfc-template.md) — RFC process details
