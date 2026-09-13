> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Documentation Reports — Overview"
id: report-readme
status: accepted
category: OVERVIEW
stability: STABLE
audience: [framework-authors]
since: "1.0"
normative-level: informative
related:
  - "[Documentation Architecture](../documentation-architecture.md)"
  - "[Quality Gates](../quality-gates.md)"
---

# Documentation Reports

**Section: 00-documentation/reports/**

This directory contains quality and coverage reports for the Awo documentation set. Reports are generated at each major version release and after significant documentation additions.

---

## Reports

| Report | Purpose |
|---|---|
| [Documentation Coverage](documentation-coverage.md) | Complete file index, section coverage |
| [Architecture Law Coverage](architecture-law-coverage.md) | Verification that all 20 Laws and 12 Invariants are documented in non-primary locations |
| [Mermaid Diagram Index](mermaid-diagram-index.md) | Index of all diagrams by document and type |

---

## Report Generation Schedule

- **Major release**: All reports regenerated
- **Minor release**: Coverage report updated
- **Patch release**: No report update required unless documentation was added

Reports are living documents — update them when the documentation set changes significantly.
