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
id: doc-003
status: accepted
category: GUIDE
stability: STABLE
audience: [contributors, module-authors, framework-authors]
since: "1.0"
normative-level: informative
related:
  - "[Documentation Standards](standards.md)"
  - "[ADR Template](adr-template.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Contributing to Awo Documentation

**DOC-003 | Status: Accepted | Stability: Stable**

How to add, update, and review documentation for the Awo Framework.

---

## 1. When to Write Documentation

Write documentation when:
- A new feature changes a behavior described in existing docs
- A new concept is introduced that isn't explained anywhere
- An ADR is made (always document the decision)
- You add a new module with patterns that other module authors will need
- A troubleshooting scenario is discovered (add to `troubleshooting.md`)

Do NOT write documentation when:
- The behavior is obvious from the code
- The behavior is already documented (link instead)
- The change is a bug fix with no behavioral difference for module authors

---

## 2. Document Structure

Every document MUST start with a YAML frontmatter block:

```yaml
---
title: "Document Title"
id: section-NNN
status: accepted         # draft | proposed | accepted | deprecated
category: SPEC           # SPEC | GUIDE | REFERENCE | ADR | LAW
stability: STABLE        # EXPERIMENTAL | STABLE | FROZEN | DEPRECATED
audience: [module-authors, framework-authors, operators]
since: "1.0"
normative-level: normative  # normative | informative
related:
  - "[Title](relative-path.md)"
---
```

**Status values**:
- `draft` — work in progress, not for reference
- `proposed` — complete but not yet reviewed
- `accepted` — reviewed and binding
- `deprecated` — superseded, preserved for history

**Stability values**:
- `EXPERIMENTAL` — may change without notice
- `STABLE` — backwards-compatible changes only
- `FROZEN` — no changes except errata (critical typos/errors)
- `DEPRECATED` — scheduled for removal

---

## 3. ID Assignment

Document IDs follow the pattern `{section-prefix}-{NNN}`:

| Section | Prefix |
|---|---|
| 00 — Documentation | `doc` |
| 01 — Introduction | `intro` |
| 02 — Architecture | `arch` |
| 03 — Kernel | `kern` |
| 04 — Domain | `dom` |
| 05 — Persistence | `pers` |
| 06 — Tenancy | `ten` |
| 07 — IAM | `iam` |
| 08 — SDUI | `sdui` |
| 09 — Workflow | `wf` |
| 10 — Modules | `mod` |
| 11 — API | `api` |
| 12 — Configuration | `cfg` |
| 13 — Observability | `obs` |
| 14 — Operations | `ops` |
| 15 — Security | `sec` |
| 16 — Module Dev Guide | `mdg` |
| 17 — ADRs | `adr` |

Assign the next available number within the section.

---

## 4. Normative Language

Use RFC 2119 keywords correctly:

- `MUST` / `MUST NOT` — absolute requirement / prohibition
- `SHOULD` / `SHOULD NOT` — strong recommendation with rare exceptions
- `MAY` — optional

Include this sentence in normative (`normative-level: normative`) documents:

> The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 5. Code Examples

All code examples must be correct Go (no pseudocode). Use real package names:

```go
import (
    "awo.so/awo/def"
    "awo.so/awo/filter"
    "awo.so/awo/session"
)
```

Never use `TODO:` in documentation code examples as placeholder business logic — only use `TODO:` in actual source code files as per the coding convention.

---

## 6. Cross-References

Use relative Markdown links:

```markdown
See [Filter DSL](../05-persistence/filter-dsl.md) for predicate syntax.
```

Do NOT use:
- Absolute paths
- `path:/title:` YAML format
- Bare file names without `.md` extension

In `related:` frontmatter, always use the markdown link format:

```yaml
related:
  - "[Filter DSL](../05-persistence/filter-dsl.md)"  # CORRECT
  - "path:../05-persistence/filter-dsl.md"           # WRONG
```

---

## 7. Updating READMEs

When adding a document to a section:

1. Create the document file
2. Add an entry to the section's `README.md` Contents table
3. Assign the correct document ID

---

## 8. ADR Process

For architecture decisions:

1. Create the ADR file using the [ADR Template](adr-template.md)
2. Propose as a PR (status: `proposed`)
3. Architecture review: at least one framework author must approve
4. On approval, change status to `accepted`
5. Add to `17-adr/README.md` index
6. ADRs are never modified after acceptance — write a new ADR to supersede

---

## 9. Deprecating Documents

When a document is superseded:

1. Change `status: deprecated` and `stability: DEPRECATED`
2. Add a note at the top: `> **Deprecated**: Superseded by [New Document](new-doc.md).`
3. Do NOT delete the file — ADRs are immutable, other docs should be preserved for at least one major version
4. Update all links in other documents to point to the new document

---

## 10. Review Checklist

Before merging documentation:

- [ ] Frontmatter is complete and valid
- [ ] Document ID assigned and added to section README
- [ ] All code examples are correct Go (compiles / is syntactically valid)
- [ ] Cross-references use relative Markdown links
- [ ] Normative documents use RFC 2119 language correctly
- [ ] Sensitive field names are not used in example data (use "example@example.com", not real KRA PINs)
- [ ] No hard-coded secrets or real credentials in examples

---

## Related Documents

- [Documentation Standards](standards.md) — style guide and conventions
- [ADR Template](adr-template.md) — template for architecture decisions
- [Glossary](../GLOSSARY.md) — terms to use consistently throughout docs
