> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "ADR-003: amis for Server-Driven UI"
id: adr-003
status: accepted
category: ADR
stability: FROZEN
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: informative
related:
  - "[Page Schema](../08-sdui/page-schema.md)"
  - "[Page Builders](../08-sdui/page-builders.md)"
  - "[amis Integration](../08-sdui/amis-integration.md)"
---

# ADR-003: amis for Server-Driven UI

**Status:** Accepted
**Date:** 2024-01-20
**Authors:** Framework Team

---

## Context

Awo must present UI for hundreds of entity types across many business modules. Building React components for each entity type is not scalable. The team evaluated server-driven UI approaches.

Requirements:
- Standard ERP views (lists, forms, detail views) without writing frontend code
- Permission-gated UI elements (absent, not disabled)
- Dynamic layout (tabbed views, embedded sub-tables)
- No custom build step for module authors adding new entities

---

## Options Considered

### Option A: Custom React Component Library

Build a React component library that module authors use to compose views.

Cons:
- Requires frontend developers for every new entity
- Build step required (TypeScript, bundling, testing)
- Permission gating must be replicated in both frontend and backend
- Schema evolution requires redeploy of frontend

### Option B: OpenAPI-Driven UI Generation (e.g., react-jsonschema-form)

Generate forms from OpenAPI schema definitions.

Pros: Automatic form generation.

Cons:
- Limited to simple forms — no tabbed layouts, embedded sub-tables, charts
- Poor developer experience for complex views
- Still requires frontend build step

### Option C: amis (Baidu Open Source)

amis is a JSON-schema-driven React renderer. The server generates JSON schemas; the browser renders them using the pinned amis SDK. No custom JavaScript required per entity.

Pros:
- Complete ERP UI vocabulary: CRUD tables, forms, tabs, charts, action buttons
- Server controls all UI logic — permission gating is a server-side schema decision
- No frontend build step — module authors write Go
- Feature-flag-controlled UI elements (server generates different schema per flag state)
- Active use in production at Baidu (large-scale battle-tested)

Cons:
- External dependency (Baidu open source — not an enterprise vendor)
- SDK must be pinned and audited for updates (no auto-update)
- Limited mobile support (desktop-first renderer)
- No real-time/streaming UI support
- Dark mode requires CSS custom property override (no built-in dark CSS for cxd theme)

---

## Decision

**Use amis as the SDUI renderer, with the SDK pinned in `web/sdk/`.**

The productivity gain (module authors write only Go to get full ERP UI) outweighs the operational constraints (pinned SDK, desktop-first). The 90% of ERP use cases (CRUD tables, forms, approval flows) are well-served by amis. The 10% that are not (real-time dashboards, pixel-perfect branded portals) are explicitly in scope as non-goals.

---

## Consequences

**Positive:**
- Module authors add entities without any frontend work
- Permission-gated UI is correct by construction — schema generation runs RBAC checks server-side
- No JavaScript deployment step for new features

**Negative:**
- amis SDK pinned — update requires full compatibility audit
- Mobile-limited — basic mobile layout only
- Dark mode requires CSS custom property override — not obvious to developers

**Architecture Laws generated:**
- Requirement to never update amis SDK without full compatibility audit (CLAUDE.md critical rule)

---

## Revisit Trigger

If amis is abandoned by Baidu or critical security vulnerabilities are not patched within 30 days, evaluate replacement. The PageBuilderFunc interface and PageSchema type are implementation-agnostic — the renderer can be swapped without changing module code.
