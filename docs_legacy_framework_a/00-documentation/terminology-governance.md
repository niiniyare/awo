> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Terminology Governance"
id: gov-terminology
status: accepted
category: SPEC
stability: STABLE
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[Documentation Standards](documentation-standards.md)"
  - "[GLOSSARY](../GLOSSARY.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Terminology Governance

**Status: Accepted | Stability: Stable**

This document specifies the rules for using, introducing, and updating terminology in the Awo documentation set.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. The Glossary Is Authoritative

The [GLOSSARY.md](../GLOSSARY.md) contains the canonical definitions for all terms used in Awo documentation and code.

Rules:
- All documentation MUST use terms as defined in the Glossary
- All code (types, functions, variables) SHOULD use Glossary terms where applicable
- No document may use a term differently from its Glossary definition
- Synonyms not in the Glossary MUST NOT be used as alternatives for Glossary terms

---

## 2. Introducing a New Term

Before using a term that does not appear in the Glossary, the author MUST:

1. Check the Glossary — the term may exist under a different name
2. Draft a Glossary entry with: canonical name, category, definition, disambiguation (if similar terms exist)
3. Add the entry to GLOSSARY.md in alphabetical order
4. Submit the Glossary update in the same PR as the first document using the term

New Glossary entries require the same PR review as the document that introduces them.

---

## 3. Glossary Entry Format

```markdown
## {Term Name}

**Category:** {Core Primitives | Architecture | Tenancy | Persistence | ...}

{Definition — one to three sentences. Precise. No ambiguity.}

**See also:** {Related term}, {Related term}

**Disambiguation:** {If similar terms exist, clarify the distinction.}
```

The definition MUST:
- State what the term **is**, not what it **does**
- Be precise enough to distinguish the term from related terms
- Use only other Glossary terms (or common English) — no circular definitions

---

## 4. Updating an Existing Glossary Entry

Definitions may be clarified without process — submit a PR with the clarification.

Definitions MUST NOT be changed in ways that alter the meaning of existing documents that use the term. If the meaning must change:
1. Write a new term with the new definition
2. Mark the old term as deprecated: `**Deprecated:** Use {new term} instead.`
3. Update all documents that used the old term to use the new term

---

## 5. Prohibited Practices

- **Do not use informal synonyms**: "entity type" and "entity name" are distinct terms; using them interchangeably is prohibited
- **Do not invent abbreviations**: all abbreviations MUST be defined in the Glossary (`CompiledSchema`, `EntityRepository`, `PolicyFunc`)
- **Do not use industry jargon without definition**: "saga", "outbox", "RLS" must be defined in the Glossary before use
- **Do not use RFC 2119 terms (MUST/SHALL/SHOULD) outside normative documents**: these words have specific meaning in normative SPECs; using them casually in GUIDEs confuses the obligation level

---

## 6. Term Disambiguation Table

Common confusions and their resolution:

| Confusing Pair | Distinction |
|---|---|
| `EntityDefinition` vs `CompiledEntity` | Definition is the source (authored); CompiledEntity is the runtime artifact |
| `entity type` vs `entity name` | Entity type is the category (`crm_contact`); entity name is the same concept — use `entity name` |
| `Module` (business) vs `Platform Module` | Business module: opt-in per tenant. Platform module: always active |
| `PolicyFunc` vs `PermissionSet` | PolicyFunc filters rows; PermissionSet gates operations |
| `session` vs `token` | Session is the server-side record in Redis; token is the client-held random string |
| `stability` (API) vs `status` (document) | Stability: API change contract. Status: document lifecycle (accepted/proposed) |
| `Hook` vs `Activity` | Hook: synchronous, inside lifecycle, inside TX. Activity: Temporal, async, outside TX |

---

## Related Documents

- [GLOSSARY.md](../GLOSSARY.md) — the Glossary itself
- [Documentation Standards](documentation-standards.md) — overall writing rules
- [Documentation Architecture](documentation-architecture.md) — governance hierarchy
