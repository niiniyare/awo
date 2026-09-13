> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "RFCs — Overview"
id: gov-rfcs-readme
status: accepted
category: OVERVIEW
stability: STABLE
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: informative
related:
  - "[RFC Template](../rfc-template.md)"
  - "[Review Process](../review-process.md)"
  - "[Documentation Architecture](../documentation-architecture.md)"
---

# RFCs

**Section: 00-documentation/rfcs/**

This directory contains Request for Comments (RFC) documents. RFCs propose significant changes — to FROZEN documents, framework architecture, or the module system — and open them for architecture team discussion before a decision is made.

---

## What Belongs Here

- Proposals to change FROZEN documents
- Proposals for new framework primitives (new FieldType, new QueryOption, new Hook stage)
- Proposals that change the module developer experience
- Proposals that require coordination across multiple teams

---

## What Does Not Belong Here

- Bug fixes → PR directly
- Clarifications to STABLE documents → PR directly
- New EXPERIMENTAL features → PR directly
- ADRs (recording past decisions) → see `awo/docs/17-adr/`

---

## RFC Index

No RFCs open at this time. The first RFC will be RFC-001.

---

## RFC Lifecycle

See [RFC Template](../rfc-template.md) for the complete RFC process and status definitions.

1. Draft RFC using the template
2. Place in this directory as `rfc-NNN-{title-slug}.md`
3. Open for comment (7-day minimum)
4. Architecture team votes
5. Accepted RFCs → result in ADR + implementation
6. Rejected RFCs → retained here as history

---

## How to Submit an RFC

1. Copy [RFC Template](../rfc-template.md)
2. Fill in all sections
3. Number it as `RFC-{next available number}`
4. Open as a PR targeting the `main` branch
5. Ping the architecture team in the PR description
