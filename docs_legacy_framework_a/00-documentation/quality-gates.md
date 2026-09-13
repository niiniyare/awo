> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Quality Gates"
id: gov-quality-gates
status: accepted
category: SPEC
stability: STABLE
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[Review Process](review-process.md)"
  - "[Documentation Standards](documentation-standards.md)"
  - "[Metadata Standard](metadata-standard.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Quality Gates

**Status: Accepted | Stability: Stable**

This document specifies the automated and manual quality gates that all documentation must pass before merging.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. Automated Gates (CI)

The following checks run automatically on every documentation PR:

### Frontmatter Validation

All required fields present and valid:
- `title`: non-empty string
- `id`: unique across all documents; matches format
- `status`: valid enum value
- `category`: valid enum value
- `stability`: valid enum value
- `audience`: non-empty list, valid values
- `since`: matches framework version format
- `normative-level`: valid enum value
- `related`: list format (not map format)

### Cross-Reference Validation

All relative links in `related:` and document body resolve to existing files.

### Mermaid Syntax Validation

All Mermaid code blocks parse without errors using `mermaid-js/mermaid` CLI.

### Markdown Lint

Standard markdown lint rules:
- No bare URLs (must be `[text](url)` format)
- No trailing whitespace
- Single blank line between sections
- Consistent heading hierarchy (no skipped levels)

---

## 2. Manual Gates (Reviewer)

The following checks are performed by human reviewers:

### Content Accuracy

- Technical content is correct
- Code examples compile or are marked as pseudocode
- No contradiction with FROZEN documents or Architecture Laws

### Standards Compliance

- RFC 2119 language (MUST/SHALL) only in normative documents
- Prose style follows [Style Guide](style-guide.md)
- Terminology matches Glossary definitions
- Cross-references are reciprocal

### Completeness

- "Related Documents" section is complete
- All referenced Architecture Laws and Invariants are linked
- New terms added to Glossary before first use

### Appropriateness

- Stability level appropriate for content maturity
- Category matches content type
- Audience accurately reflects who should read this

---

## 3. Blocking vs. Non-Blocking Gates

### Blocking (PR cannot merge until resolved)

- Frontmatter validation failures
- Broken cross-reference links
- Mermaid syntax errors
- Missing required sections (Related Documents, for SPEC documents)
- Content that contradicts a FROZEN document or Architecture Law
- RFC 2119 language in informative documents

### Non-Blocking (noted in review, may defer)

- Style guide violations (if minor and content is clear)
- Diagram quality improvements
- Optional cross-references not yet added
- TODO comments in code examples (permitted with explicit `// TODO:` marker)

---

## 4. Release Gate

Before a MAJOR or MINOR version release, the following additional gates apply:

- No EXPERIMENTAL documents in sections that affect the release's features
- All new STABLE documents have been reviewed by at least two maintainers
- CHANGELOG updated with all documentation changes since last release
- Glossary updated with all new terms introduced in the release
- No broken links anywhere in the documentation set (full link check run)

---

## 5. Quality Metrics (Tracked, Not Gated)

The following metrics are tracked for documentation health but do not block PRs:

- **Coverage**: percentage of EntityDefinition fields documented in the Field Types reference
- **Freshness**: time since last review for STABLE and FROZEN documents
- **Cross-reference density**: average number of related links per document
- **Terminology consistency**: percentage of Glossary terms used correctly (spot-checked quarterly)

---

## Related Documents

- [Review Process](review-process.md) — human review process and approvals
- [Documentation Standards](documentation-standards.md) — writing standards that reviewers check
- [Metadata Standard](metadata-standard.md) — frontmatter rules for automated validation
- [Cross-Reference Policy](cross-reference-policy.md) — link format rules for automated check
