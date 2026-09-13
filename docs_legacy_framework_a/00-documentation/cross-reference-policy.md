> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Cross-Reference Policy"
id: gov-cross-reference
status: accepted
category: SPEC
stability: STABLE
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[Documentation Standards](documentation-standards.md)"
  - "[Documentation Architecture](documentation-architecture.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Cross-Reference Policy

**Status: Accepted | Stability: Stable**

This document specifies how documents cross-reference each other, the required formats, and validation rules.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. Frontmatter Related Links

Every document MUST declare its related documents in the frontmatter `related:` field using this format:

```yaml
related:
  - "[Document Title](relative-path.md)"
  - "[Section Title](../section/document.md)"
```

Rules:
- Format MUST be markdown link: `"[Title](path)"` — not YAML `path:` / `title:` style
- Path MUST be relative from the current document's location
- Title MUST match the `title:` frontmatter field of the linked document
- Related links MUST be reciprocal — if A links to B, B SHOULD link to A

---

## 2. Inline Cross-References

Within document body, cross-references MUST use markdown links:

```markdown
See [Architecture Laws](../02-architecture/laws.md) for the 20 binding constraints.

See [LAW-006](../02-architecture/laws.md#law-006) for the outbox requirement specifically.
```

Rules:
- Inline links MUST use the linked document's title or the specific section header
- Inline links MUST use relative paths
- Law references MUST include the LAW-NNN identifier
- Invariant references MUST include the INV-NNN identifier

---

## 3. Law and Invariant References

References to Architecture Laws and Invariants MUST include the identifier:

```markdown
# Correct
See [LAW-006](../02-architecture/laws.md#law-006) for the outbox + transaction atomicity requirement.
This behavior is mandated by [INV-001](../02-architecture/invariants.md#inv-001).

# Incorrect
See the Architecture Laws document.
```

Every normative document (SPEC) MUST list in its "Related Documents" section the Laws and Invariants it enforces or implements.

---

## 4. Glossary References

Glossary terms mentioned for the first time in a document SHOULD be linked:

```markdown
The [CompiledSchema](../GLOSSARY.md#compiledschema) is produced by [Registry.Compile()].
```

Glossary links use `../GLOSSARY.md#{term-anchor}` where the anchor is the lowercase term with spaces replaced by hyphens.

Repeated mentions of the same Glossary term in the same document do not need to be linked — link only the first occurrence per section.

---

## 5. Prohibited Reference Patterns

| Prohibited | Use Instead |
|---|---|
| `See the laws document` | `See [Architecture Laws](../02-architecture/laws.md)` |
| `See LAW-006` (no link) | `See [LAW-006](../02-architecture/laws.md#law-006)` |
| `[title](absolute-path/from/root)` | Relative path from current document |
| `related: - path: ../doc.md\n  title: Doc` | `related: - "[Doc](../doc.md)"` |

---

## 6. Broken Link Prevention

All document cross-references are relative paths. When a document is moved:
1. Update all `related:` frontmatter entries that point to the moved document
2. Update all inline links within documents that reference it
3. Add a redirect entry (if the documentation portal supports redirects)

Breaking links is treated as a documentation bug — the same severity as a broken code reference.

---

## Related Documents

- [Documentation Standards](documentation-standards.md) — overall writing rules
- [Documentation Architecture](documentation-architecture.md) — document hierarchy
- [Glossary](../GLOSSARY.md) — canonical term definitions
