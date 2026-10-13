# Chapter 51 — Roadmap

> **Volume:** X — Reference
> **Phase:** 4
> **Audience:** All Engineers, Engineering Leadership
> **Prerequisites:** All previous chapters

---

## Table of Contents

- [51.1 Architectural Principles, Revisited](#511-architectural-principles-revisited)
- [51.2 Near-Term (0–6 Months)](#512-near-term-06-months)
- [51.3 Medium-Term (6–18 Months)](#513-medium-term-618-months)
- [51.4 Long-Term (18+ Months)](#514-long-term-18-months)
- [51.5 Technical Debt Backlog](#515-technical-debt-backlog)
- [51.6 Explicitly Out of Scope](#516-explicitly-out-of-scope)
- [51.7 Decision Log](#517-decision-log)

---

## 51.1 Architectural Principles, Revisited

Before listing what comes next, it is worth restating the principles that have governed every architectural decision in this documentation. The roadmap below is constrained by these — items that conflict with them require revisiting the principles, not just the items.

**1. Tenant isolation is non-negotiable.**
No feature, optimization, or deadline justifies weakening RLS enforcement or the principle that tenant data never crosses tenant boundaries. Every future feature is reviewed for isolation implications before being designed.

**2. Clients are rendering engines.**
The SDUI architecture is the foundation of the platform's ability to serve web, iOS, Android, and terminal clients from a single surface definition. Features that require client-specific business logic undermine this foundation. The answer to "can we add this logic to the app?" is "add it to the surface compiler."

**3. Async-first for operations that take time.**
Every operation with meaningful duration is a Temporal workflow. The HTTP layer is for immediate responses. This distinction is firm — not a performance threshold to be moved later.

**4. Observability is built-in, not bolted on.**
Every new service, every new endpoint, every new workflow includes metrics, traces, and logs from the first day it ships. Observability is not added after the fact.

**5. The platform is a product, not an internal tool.**
External developers (plugin authors, tenant integrators) are first-class users of the SDK and the platform. Breaking changes to the plugin contract are treated with the same weight as breaking changes to the public API.

---

## 51.2 Near-Term (0–6 Months)

### 51.2.1 Plugin Marketplace (Alpha)

**What:** A curated set of first-party and approved third-party plugins available within the platform admin surface. Tenants can install plugins without contacting AwoERP support.

**Why:** The plugin SDK (Chapter 50) is complete. The marketplace is the distribution mechanism that makes it useful. Pilot partners have plugins ready to publish.

**Architecture notes:**
- Plugin packages are signed with the plugin author's private key; the platform verifies the signature on install
- Plugin actions run in a sandboxed Go goroutine with explicit resource limits (CPU, memory, time)
- Plugin database tables are prefixed with `plugin_{plugin_id}_` and covered by the same RLS policies as core tables

**Non-goals for alpha:** Plugin revenue sharing, self-service plugin submission, community plugins.

---

### 51.2.2 SDUI Formula Fields

**What:** Surface definitions can declare computed fields using a constrained formula syntax, evaluated by the surface compiler at render time, rather than requiring a new backend field for every derived value.

**Why:** Today, displaying `subtotal + tax = total` on an invoice requires a dedicated `total` field on the invoice model. A formula field (`$row.subtotal + $row.tax`) computes it at compile time. This unblocks a large class of tenant customization requests.

**Architecture notes:**
- Formula syntax is a strict subset of arithmetic and string operations — no loops, no external calls
- Formulas are parsed and validated at surface publication time (the linter rejects invalid formulas before they reach production)
- Formula evaluation is deterministic: same inputs, same output, always

---

### 51.2.3 Mobile Biometric Authentication

**What:** iOS and Android clients support Face ID / fingerprint authentication as an alternative to PIN for unlocking the app after backgrounding.

**Why:** Enterprise users on mobile require device-level authentication. The session JWT refresh (Chapter 12) already handles session continuation; biometric auth gates the local app unlock, not the server session.

**Architecture notes:** The biometric prompt result does not change the JWT. It unlocks a locally-encrypted keychain entry that holds the session token. The server is not involved in the biometric flow.

---

### 51.2.4 Audit Log Export

**What:** Tenant admins can export their audit log as CSV or JSON via the platform admin surface or the API.

**Why:** Compliance teams need audit logs in tools outside of AwoERP (SIEM, compliance platforms). The audit log is append-only in PostgreSQL (Chapter 40); the export is a read-only operation.

**Architecture notes:** Exports are Temporal workflows (large result sets). Files are stored in object storage with a 48-hour expiry pre-signed URL. The export endpoint returns a job ID; the client polls for completion.

---

## 51.3 Medium-Term (6–18 Months)

### 51.3.1 Terminal Client (TUI)

**What:** A full-featured terminal client built on the same SDUI surface definitions as the web and mobile clients. Aimed at developers and power users who prefer keyboard-driven workflows.

**Why:** The SDUI architecture makes this possible without duplicating business logic. A terminal rendering engine that understands the same surface definition JSON is a client implementation, not a platform feature. AwoERP's target market includes logistics companies whose warehouse staff use terminal interfaces.

**Architecture notes:**
- Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) (Go TUI framework)
- Renders SDUI components as terminal widgets: data tables as bordered tables, forms as interactive prompts
- SSE patch protocol is identical to web/mobile (the rendering layer is different, the protocol is the same)
- Accessibility: terminal clients inherit VoiceOver/screen reader support from the terminal emulator

---

### 51.3.2 Multi-Currency Accounting

**What:** Full double-entry accounting with multi-currency support, including foreign exchange gain/loss tracking, functional vs. presentation currency, and revaluation workflows.

**Why:** Current invoicing handles foreign currencies for display purposes. Full accounting (balance sheet impact, FX gain/loss, currency revaluation) is the next capability required by larger tenants with cross-border operations.

**Architecture notes:**
- All monetary values stored in the functional currency (tenant-configured) in the database; display currency computed at render time
- FX rates stored in a `exchange_rates` table with daily snapshots; historical rates preserved for revaluation
- Revaluation is a Temporal workflow: iterates open foreign-currency balances, computes FX gain/loss, creates journal entries
- Breaking change: `amount` in the API gains a `functional_currency_amount` field alongside the existing `amount` (presentation currency); existing clients continue to work

---

### 51.3.3 AI-Assisted Data Entry

**What:** Invoice data entry surfaces include optional AI-assisted field completion: extract vendor name, line items, and amounts from a photo of a paper invoice, and pre-fill the invoice form.

**Why:** Manual data entry is the highest-friction part of ERP workflows in the markets AwoERP serves. Reducing it increases platform retention.

**Architecture notes:**
- OCR and extraction run as a Temporal workflow (document → OCR → extraction model → structured output)
- Extracted data is a suggestion, not an automatic commit. The surface pre-fills the form; the user reviews and saves.
- PII handling: invoice images are stored in tenant-scoped object storage; extraction results are stored in the tenant's database; the extraction model is called via the platform's AI gateway, not directly by the client
- Tenant opt-in: disabled by default, enabled per-tenant in Settings → AI Features

---

### 51.3.4 SDUI Component Library Expansion

**What:** Extend the SDUI component library with: Gantt chart (`data.gantt`), rich text display (`display.rich_text`), map view (`display.map`), and photo upload (`input.photo`).

**Why:** Current component library covers standard ERP screens. Logistics and field operations use cases require map views (delivery tracking) and photo uploads (proof of delivery).

**Architecture notes:**
- `display.map` uses a configurable map tile provider (tenant-provided API key); the surface compiler emits the provider and coordinates; the rendering engine handles the map rendering
- `input.photo` stores photos in object storage; the upload action returns an object key, not the raw bytes; the surface binding references the object key
- All new components are added to the SDUI JSON schema and validated by the compiler before publication

---

## 51.4 Long-Term (18+ Months)

### 51.4.1 Multi-Region Deployment

**What:** Deploy AwoERP in multiple geographic regions (Africa-west, Africa-east, EU) with data residency controls that ensure tenant data stays in the tenant's selected region.

**Why:** Data residency is a regulatory requirement for some enterprise tenants. Latency to AwoERP from West Africa is currently 200ms+; a regional deployment would reduce it to <50ms.

**Architecture notes:**
- Each region is an independent deployment (no cross-region database replication for tenant data)
- The global control plane (tenant provisioning, billing, platform admin) operates from a primary region with async replication
- Tenant selection of region is irrevocable after onboarding (data residency guarantee)
- Plugin marketplace catalog is global; plugin execution is regional

---

### 51.4.2 Third-Party ERP Migration Toolkit

**What:** A set of migration tools and surface definitions for importing data from common African ERP and accounting systems (QuickBooks, Sage, Navision) into AwoERP.

**Why:** Tenant acquisition requires a credible migration story. Currently, migrations are manual. A toolkit reduces friction and accelerates onboarding.

**Architecture notes:** Migration jobs are Temporal workflows. Source system connectors are plugins (using the plugin SDK). The migration surface guides tenants through data mapping, validation, and import.

---

### 51.4.3 AI Workflow Automation

**What:** Tenants can define natural-language rules that create Temporal workflows: "When an invoice over KES 500,000 is submitted, route to the CFO for approval."

**Why:** Custom approval routing is currently only achievable through the plugin SDK (requires a developer). An AI-assisted workflow builder would make it accessible to non-technical tenant admins.

**Architecture notes:**
- Rule definition surface: a form-based interface where admins describe the trigger, condition, and action
- Rule compilation: converts the form output to a Temporal workflow definition (not arbitrary code execution — a restricted set of workflow templates with configurable parameters)
- The AI layer is used for the natural-language-to-form translation step; the form output, not the AI output, drives workflow creation

---

## 51.5 Technical Debt Backlog

The following known technical debt items are tracked and will be addressed:

| Item | Priority | Notes |
|------|---------|-------|
| Redis cache key namespace collision risk | Medium | Key format needs a version prefix to prevent collisions during schema changes |
| Temporal worker pool sizing | Medium | Currently static; should autoscale on queue depth (blocker: needs metrics integration) |
| Surface compiler error messages | Low | Error messages from the JSON Schema validator are not yet human-readable |
| Integration test parallelism | Low | Integration tests run serially; can be parallelized with per-test database schemas |
| golangci-lint `wrapcheck` false positives | Low | Some legitimate unwrapped errors trigger lint warnings; suppress list needs cleanup |

---

## 51.6 Explicitly Out of Scope

The following capabilities are not planned and will not be added without a deliberate architectural decision:

**Multi-database tenancy (one database per tenant).** The current shared-database-RLS architecture is the right trade-off at current scale. Per-tenant databases would add operational complexity (hundreds of database connections, per-tenant migration management) without meaningful isolation benefits beyond what RLS provides.

**Client-side business logic.** SDUI surfaces do not evaluate business rules. Adding if/else, loop, or function call semantics to binding expressions would be a significant architectural regression. Complex computations belong in the surface compiler or in backend computed fields.

**Arbitrary plugin code execution.** Plugins run Go handler functions with explicit resource limits and a restricted set of platform APIs. General-purpose code execution (shell access, arbitrary HTTP calls, database DDL) will not be added to the plugin SDK.

**Managed hosting of tenants' external systems.** AwoERP integrates with external systems via webhooks and the SDK. It does not host, manage, or maintain external systems on behalf of tenants.

---

## 51.7 Decision Log

Key architectural decisions and the reasoning behind them:

| Decision | Date | Rationale |
|----------|------|-----------|
| PostgreSQL RLS over application-layer tenant filtering | Phase 1 | Enforcement at the database layer cannot be bypassed by application code bugs |
| SDUI over traditional client-specific APIs | Phase 1 | Single surface definition serves all clients; product logic centralized in compiler |
| Temporal over ad-hoc async workers | Phase 1 | Workflow durability, observability, and retry semantics are built-in |
| OpenFGA over role-based access control | Phase 2 | Relationship-based model supports complex permission structures (delegation, hierarchy) that RBAC cannot express cleanly |
| Shared-database multi-tenancy over per-tenant databases | Phase 1 | Operational simplicity; RLS provides equivalent isolation at current scale |
| Go as primary backend language | Phase 1 | Strong concurrency primitives, fast compilation, explicit error handling |
| No microservices | Phase 1 | Monorepo with service boundaries enforced by Go packages; decompose when operationally necessary, not speculatively |
| Semantic versioning with 180-day deprecation window | Phase 2 | External developer trust requires predictable and generous deprecation windows |

---

*End of Chapter 51*

---

*End of Volume X — Reference*

---

# AwoERP Architecture Documentation — Complete

This concludes the AwoERP platform architecture documentation spanning:

| Volume | Chapters | Topic |
|--------|---------|-------|
| I | 1–5 | Foundations |
| II | 6–10 | Core Platform |
| III | 11–16 | Security & Authorization |
| IV | 17–21 | SDUI Architecture |
| V | 22–26 | Integration & API |
| VI | 27–31 | Data & Storage |
| VII | 32–35 | Performance |
| VIII | 36–40 | Quality |
| IX | 41–45 | Operations & Governance |
| X | 46–51 | Reference |

**Previous:** [Chapter 50 — SDK](./50-sdk.md)

---

*AwoERP Platform Architecture — v1.0*
