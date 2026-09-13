> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Style Guide"
id: gov-style
status: accepted
category: SPEC
stability: STABLE
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[Documentation Standards](documentation-standards.md)"
  - "[Terminology Governance](terminology-governance.md)"
  - "[Diagram Standards](diagram-standards.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Style Guide

**Status: Accepted | Stability: Stable**

This document specifies prose style, code example conventions, heading structure, and formatting rules for all Awo documentation.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. Prose Style

### Voice and Person

- Use **second person** in guides ("You declare the entity...") — direct and action-oriented
- Use **third person** in specs ("The framework generates...") — authoritative and precise
- Use **present tense** — "The registry validates..." not "The registry will validate..."
- Use **active voice** — "The compiler rejects..." not "The compiled entity is rejected by..."

### Sentence Length

- Maximum one idea per sentence
- Long sentences SHOULD be split at conjunctions ("and", "but", "while")
- Technical terms are precise — do not substitute synonyms for the sake of variety

### Paragraph Length

- Maximum 5 sentences per paragraph
- Each paragraph addresses one concept
- Blank line between paragraphs (standard Markdown)

---

## 2. Heading Structure

Headings MUST follow the hierarchy:
- `#` — Document title (one per document)
- `##` — Major section (numbered: `## 1. Topic`)
- `###` — Subsection (`### 1.1 Sub-topic` or unnumbered)
- `####` — Rarely used; avoid more than 3 heading levels

Section numbering is REQUIRED for normative sections in SPEC documents (1, 2, 3...). GUIDEs may omit section numbers.

Do not use heading-level emphasis for non-heading text.

---

## 3. Code Examples

### Required: Filename Comment

Every Go code block MUST include a comment indicating the file location:

```go
// internal/core/crm/def.go
package crm

var ContactDefinition = def.EntityDefinition{...}
```

### Required: Runnable Examples

Code examples MUST be syntactically correct Go (or clearly marked as pseudocode). Pseudocode blocks use no language identifier:

```
// pseudocode — not valid Go
registry → compile → validate → seal
```

### Required: Context

Code examples MUST include enough surrounding context to be understood in isolation. Avoid `...` in critical positions:

```go
// Incorrect: ... obscures what comes before Actions
var InvoiceDefinition = def.EntityDefinition{
    // ...
    Actions: []def.ActionDef{...},
}

// Correct: state what fields are omitted
var InvoiceDefinition = def.EntityDefinition{
    Name:   "finance_invoice",
    // Fields, Edges, Hooks, Permissions declared above
    Actions: []def.ActionDef{
        {Name: "submit", HandlerFunc: SubmitInvoiceAction},
    },
}
```

### SQL Examples

SQL statements MUST use uppercase keywords:

```sql
-- Correct
CREATE TABLE finance_invoice (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid()
);

-- Incorrect
create table finance_invoice (
    id uuid primary key default gen_random_uuid()
);
```

### JSON Examples

JSON examples MUST be formatted with 2-space indentation. Long arrays may be abbreviated with `...`:

```json
{
  "type": "crud",
  "columns": [
    {"field": "number", "label": "Invoice #"},
    {"field": "status", "label": "Status"}
  ]
}
```

---

## 4. Tables

Tables MUST include a header row. Column widths determined by content — do not pad with spaces.

Table columns MUST NOT have redundant headers. A column titled "Description" under a table titled "Field Descriptions" is redundant; use "Notes" instead.

---

## 5. Lists

- Unordered lists: use `-` not `*`
- Ordered lists: use `1.`, `2.`, `3.` — not `1)`, `2)`, `3)`
- Nested lists: one level of nesting maximum in documentation

---

## 6. Emphasis

- `**bold**` — for important terms being introduced for the first time in a section
- `` `code` `` — for code symbols, field names, entity names, environment variable names
- `*italic*` — for titles of books, specifications, or documents (rarely needed)
- Do NOT use bold for general emphasis — if something is important enough to emphasize, restructure the sentence

---

## 7. Notes and Warnings

For important non-normative callouts:

```markdown
> **Note:** This behavior changed in version 1.2. See the migration guide.
```

For security-critical information:

```markdown
> **Security:** Never log the value of `JWTSecret`. This field is sensitive.
```

Warnings are reserved for information that, if missed, causes data loss, security vulnerabilities, or system instability.

---

## 8. Numbers

- Spell out numbers one through nine in prose: "three entities", "seven steps"
- Use numerals for 10 and above: "20 fields", "100 records"
- Always use numerals for: versions, port numbers, HTTP status codes, timeouts

---

## Related Documents

- [Documentation Standards](documentation-standards.md) — structural requirements
- [Terminology Governance](terminology-governance.md) — term usage rules
- [Diagram Standards](diagram-standards.md) — diagram formatting rules
