> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "SDUI — Section Overview"
id: sdui-000-readme
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: informative
related:
  - "[Page Schema](page-schema.md)"
  - "[Page Builders](page-builders.md)"
  - "[amis Integration](amis-integration.md)"
  - "[EntityDefinition](../03-kernel/entity-def.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# SDUI

**Section 08 | Server-Driven UI**

Awo uses Server-Driven UI (SDUI) to generate all standard ERP views without custom JavaScript. The server generates complete JSON schemas; the browser renders them using the pinned amis SDK.

---

## Contents

| Document | ID | Purpose | Stability |
|---|---|---|---|
| [Page Schema](page-schema.md) | SDUI-001 | Page schema structure, caching, permission gating, serving | FROZEN |
| [Page Builders](page-builders.md) | SDUI-002 | PageBuilderSet, custom view construction, SDUI primitives | STABLE |
| [amis Integration](amis-integration.md) | SDUI-003 | amis SDK setup, theming, known limitations, dark mode | STABLE |
| [Dashboard Patterns](dashboard-patterns.md) | SDUI-004 | KPI tiles, charts, embedded tables, feature-flag gating | STABLE |
| [Form Patterns](form-patterns.md) | SDUI-005 | Conditional fields, cascading dropdowns, multi-step forms | STABLE |
| [List Patterns](list-patterns.md) | SDUI-006 | Columns, row actions, bulk actions, cursor pagination | STABLE |
| [Action Buttons](action-buttons.md) | SDUI-007 | Toolbar buttons, row actions, confirmation dialogs, permission gates | STABLE |
| [Detail View Patterns](detail-view-patterns.md) | SDUI-008 | Field groups, tabs, related records, status badges, inline edit | STABLE |
| [Report View Patterns](report-view-patterns.md) | SDUI-009 | Summary cards, charts, date range filters, aggregation tables, export | STABLE |

---

## Prerequisites

- [EntityDefinition §10](../03-kernel/entity-def.md#10-page-builders) — PageBuilderSet declarations
- [Architecture Overview §9](../01-introduction/architecture-overview.md#9-what-is-not-in-the-framework) — SDUI limitations
- [Glossary](../GLOSSARY.md) — SDUI, amis, Page Schema, Page Builder, PageBuilderSet
