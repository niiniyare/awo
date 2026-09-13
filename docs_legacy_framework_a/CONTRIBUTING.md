> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Contributing to Awo Documentation"
id: docs-contributing
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: informative
related:
  - "[Review Process](00-documentation/review-process.md)"
  - "[Documentation Standards](00-documentation/documentation-standards.md)"
  - "[Style Guide](00-documentation/style-guide.md)"
  - "[Quality Gates](00-documentation/quality-gates.md)"
---

# Contributing to Awo Documentation

This guide explains how to contribute new or updated documentation to the Awo Framework.

---

## Before You Start

1. Read [Documentation Architecture](00-documentation/documentation-architecture.md) — understand how documents are organized
2. Read [Documentation Standards](00-documentation/documentation-standards.md) — understand the writing rules
3. Read [Style Guide](00-documentation/style-guide.md) — understand prose and code conventions
4. Check the [GLOSSARY](GLOSSARY.md) for existing term definitions

---

## Types of Contributions

### Bug Fix (incorrect content)

A bug in documentation is content that is factually incorrect. To fix:
1. Open a PR with the correction
2. In the PR description, explain what was wrong and what the correct behavior is
3. Include a reference to the code or test that demonstrates the correct behavior
4. One maintainer approval required

### Clarification (ambiguous content)

If documented behavior is unclear but not wrong:
1. Open an issue first to confirm the intended behavior
2. Then open a PR with the clarification
3. One maintainer approval required

### New Document (EXPERIMENTAL or STABLE)

To add a new document:
1. Identify the correct section based on [Documentation Architecture](00-documentation/documentation-architecture.md)
2. Use the appropriate template from [Document Types](00-documentation/document-types.md)
3. Fill in all required frontmatter
4. Ensure all cross-references are correct
5. Add entry to the section's README
6. Add any new Glossary terms to GLOSSARY.md
7. Apply the [Review Process](00-documentation/review-process.md) checklist
8. One maintainer approval for EXPERIMENTAL, two for STABLE

### Changing a FROZEN Document

All FROZEN documents require the RFC process (see [RFC Template](00-documentation/rfc-template.md)):
1. Write an RFC describing the proposed change
2. Open RFC as a PR for 7-day comment period
3. Architecture team votes
4. If approved: open document change PR
5. Two maintainer approvals required

---

## Quick Reference: Where Does This Content Go?

| Content | Section |
|---|---|
| What the system must do | 02-architecture (laws/invariants) or appropriate SPEC |
| How a framework primitive works | 03-kernel or 04-domain |
| How persistence works | 05-persistence |
| Multi-tenancy | 06-tenancy |
| IAM and auth | 07-iam |
| UI generation | 08-sdui |
| Async workflows | 09-workflow |
| Module structure | 10-modules |
| API conventions | 11-api |
| Configuration | 12-configuration |
| Logging and metrics | 13-observability |
| Deployment procedures | 14-operations |
| Security guidance | 15-security |
| Tutorial for module authors | 16-module-dev-guide |
| Why a decision was made | 17-adr |

---

## Frontmatter Checklist

Every document must have:

```yaml
---
title: "Your Document Title"
id: section-NNN           # unique ID
status: proposed          # start as proposed
category: SPEC            # or GUIDE, ADR, TEMPLATE, OVERVIEW
stability: EXPERIMENTAL   # start as EXPERIMENTAL
audience: [module-authors]
since: "1.0"
normative-level: normative  # or informative
related:
  - "[Title](relative-path.md)"
---
```

---

## Code Example Rules

- All Go code must be syntactically correct
- Include a `// path/to/file.go` comment at the top of each code block
- Use `// TODO:` for placeholder logic — never silent no-ops
- SQL must use uppercase keywords
- Never include real credentials, even example ones

---

## Getting Help

- Open an issue with the `documentation` label
- Tag `@framework-team` in the issue for architecture questions
- For urgent corrections: tag `@maintainers` directly in the PR
