> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Metadata Standard"
id: gov-metadata
status: accepted
category: SPEC
stability: STABLE
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[Documentation Standards](documentation-standards.md)"
  - "[Document Types](document-types.md)"
  - "[Stability Model](stability-model.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Metadata Standard

**Status: Accepted | Stability: Stable**

This document specifies the required YAML frontmatter fields for all documents in the Awo documentation set, their valid values, and validation rules.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. Required Fields

Every document MUST include all of the following frontmatter fields:

```yaml
---
title: "Document Title"
id: unique-doc-id
status: accepted
category: SPEC
stability: STABLE
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[Title](relative-path.md)"
---
```

---

## 2. Field Specifications

### `title`

**Type:** String
**Required:** Yes

The human-readable title of the document. MUST match the `# Heading` in the document body. MUST be quoted.

```yaml
title: "EntityDefinition"
```

### `id`

**Type:** String
**Required:** Yes

A unique identifier for the document across the entire documentation set. Used for programmatic reference and link validation.

Format: `{section-prefix}-{NNN}` or `{section-prefix}-{slug}`.

```yaml
id: kern-001    # Section 03, first document
id: gov-metadata  # Governance document
```

Valid section prefixes: `das` (documentation architecture), `dss` (documentation standards), `gov` (governance), `intro`, `arch`, `kern`, `dom`, `pers`, `ten`, `iam`, `sdui`, `wf`, `mod`, `api`, `cfg`, `obs`, `ops`, `sec`, `mdg`, `adr`.

### `status`

**Type:** Enum
**Required:** Yes
**Valid values:** `proposed | accepted | rejected | deprecated`

The lifecycle state of the document.

| Value | Meaning |
|---|---|
| `proposed` | Under review; not yet authoritative |
| `accepted` | Authoritative; the current approved version |
| `rejected` | Was proposed but rejected; retained for history |
| `deprecated` | Will be removed; see replacement |

### `category`

**Type:** Enum
**Required:** Yes
**Valid values:** `SPEC | GUIDE | ADR | TEMPLATE | OVERVIEW`

See [Document Types](document-types.md) for definitions.

### `stability`

**Type:** Enum
**Required:** Yes
**Valid values:** `EXPERIMENTAL | STABLE | FROZEN | DEPRECATED`

See [Stability Model](stability-model.md) for definitions.

### `audience`

**Type:** List of strings
**Required:** Yes
**Valid values:** `module-authors | framework-authors | operators`

Who the document is written for. Used to filter documentation views.

```yaml
audience: [module-authors, framework-authors]
audience: [operators]
```

### `since`

**Type:** String (version)
**Required:** Yes

The framework version in which this document was introduced.

```yaml
since: "1.0"
since: "1.3"
```

### `normative-level`

**Type:** Enum
**Required:** Yes
**Valid values:** `normative | informative`

Whether the document establishes requirements (normative) or provides guidance (informative). See [Document Types](document-types.md).

### `related`

**Type:** List of markdown links
**Required:** Yes (may be empty list `[]` for standalone documents)

Documents that are related to this one, using markdown link format:

```yaml
related:
  - "[EntityDefinition](../03-kernel/entity-def.md)"
  - "[Architecture Laws](../02-architecture/laws.md)"
```

MUST NOT use the `path:` / `title:` YAML style.

---

## 3. Optional Fields

### `supersedes`

For ADRs that supersede an earlier ADR:

```yaml
supersedes: "ADR-003"
```

### `superseded-by`

For ADRs that have been superseded:

```yaml
superseded-by: "ADR-009"
```

---

## 4. Validation

The following validation rules apply to all frontmatter:

1. All required fields present
2. `id` is unique across all documents in the set
3. `status` is a valid enum value
4. `category` is a valid enum value
5. `stability` is a valid enum value
6. `normative-level` is a valid enum value
7. `related:` entries use markdown link format (not `path:` YAML)
8. `since` matches a valid framework version string

A document failing validation MUST NOT be merged.

---

## Related Documents

- [Documentation Standards](documentation-standards.md) — overall writing rules
- [Document Types](document-types.md) — valid category values
- [Stability Model](stability-model.md) — valid stability values
- [Cross-Reference Policy](cross-reference-policy.md) — `related:` format rules
