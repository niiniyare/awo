# Chapter 01 — Introduction to AwoERP UI Platform

> **Volume:** I — Vision & Philosophy
> **Phase:** 1 (Foundation)
> **Audience:** All Engineers, Product Owners, Solution Architects, ERP Implementers, Third-Party Integrators
> **Status:** Approved for Development

---

## Table of Contents

- [1.1 What Is AwoERP?](#11-what-is-awoerp)
- [1.2 The Problem This Platform Solves](#12-the-problem-this-platform-solves)
- [1.3 Platform Mission Statement](#13-platform-mission-statement)
- [1.4 Key Value Propositions](#14-key-value-propositions)
- [1.5 Who Should Read This Documentation](#15-who-should-read-this-documentation)
- [1.6 How to Use This Documentation](#16-how-to-use-this-documentation)
- [1.7 Relationship to Other AwoERP Documentation](#17-relationship-to-other-awoerp-documentation)
- [1.8 Glossary Reference](#18-glossary-reference)

---

## 1.1 What Is AwoERP?

AwoERP is a **multi-tenant Enterprise Resource Planning (ERP) platform** designed for organizations that operate across diverse business functions — finance, procurement, inventory, human resources, and sales — within a unified, cloud-native system.

At its technical core, AwoERP is distinguished not by what it does as a business application, but by **how it builds and delivers its user interface**. Rather than maintaining separate frontend codebases for web, iOS, and Android, AwoERP uses a **Server-Driven UI (SDUI)** architecture in which the backend is the single, authoritative source that defines what users see, how they interact with the application, and what they are permitted to do.

The user interface is not written by frontend engineers as static component trees or hard-coded screens. Instead, the backend composes the UI as a structured, typed data payload — a JSON document that describes the complete visual and behavioral contract of every screen, form, dashboard, and workflow. Mobile and web applications receive this payload and act as **rendering engines**: sophisticated, capable clients whose job is to faithfully materialize whatever the backend describes.

This architecture is not novel in isolation. It draws from a lineage of industry approaches — Meta's SDUI for Facebook's news feed, Airbnb's Ghost Platform, Shopify's Polaris, and enterprise low-code platforms such as AMIS, Retool, Appsmith, and Microsoft PowerApps. What AwoERP contributes is the application of these principles **at the domain depth required by enterprise ERP**: multi-currency ledgers, approval workflows, fine-grained authorization, multi-tenant customization, and offline-capable field operations — all driven from a single, coherent, server-owned UI description.

---

## 1.2 The Problem This Platform Solves

### 1.2.1 Traditional ERP UI Fragmentation

Enterprise software has historically suffered from a structural mismatch between its business complexity and its frontend delivery model. Most ERP systems — both legacy monoliths and modern SaaS platforms — maintain distinct frontend codebases for each platform they support.

The consequences are predictable and severe:

**Drift between platforms.** A form field added to the web application is not automatically available on mobile. A validation rule tightened after a compliance audit must be re-implemented in three separate codebases. A layout change approved by the design team must be coordinated across web, iOS, and Android sprint cycles. Over months and years, the platforms diverge. Users switching between web and mobile encounter different field arrangements, different error messages, and occasionally different business behaviors.

**Duplicate business logic.** Authorization rules, validation logic, and workflow state machines are implemented once in the backend — and then re-implemented, imperfectly, in each frontend. The frontend implementations are approximations. They exist for responsiveness and user experience, but they are ultimately redundant copies of truth that lives elsewhere. Every copy is a liability: it can contradict the source, it can be forgotten when the source changes, and it must be maintained indefinitely.

**Release coupling.** When a business rule changes — a new mandatory approval step, a restructured GL account hierarchy, a regulatory reporting requirement — the backend can be updated in a single deployment. But that change cannot reach users until each frontend application is updated and re-released. On mobile, this includes the additional friction of app store review cycles. The business is blocked by the tooling.

**Tenant customization through code.** When enterprise customers require customizations — a field renamed for their industry terminology, a section hidden because their workflow does not use it, a custom field added to capture data specific to their operations — the only mechanism available in most ERP systems is a code fork, a plugin with privileged access to internals, or an expensive professional services engagement. None of these scale to hundreds of tenants with unique requirements.

### 1.2.2 The Multi-Platform Maintenance Burden

Building and maintaining UI across three platforms (web, iOS, Android) with dedicated engineering teams is expensive. More importantly, it introduces **structural latency** between business intent and user experience. Every new ERP module, every workflow enhancement, every regulatory adaptation requires parallel frontend work before it is visible to users.

The industry response to this problem — cross-platform frameworks such as React Native and Flutter — reduces the platform count but does not address the root cause. The business logic that determines what a user should see and what they are permitted to do still lives in the backend. The frontend still re-implements it, now in a shared language rather than three separate ones.

AwoERP's response is more fundamental: **move the UI description itself to the backend**, and reduce the frontend to the minimum viable intelligence required to render what it is told.

### 1.2.3 Tenant Customization Without Code Forks

A multi-tenant ERP platform serves customers with genuinely different requirements. A manufacturing company and a logistics firm both need purchase order management, but their form layouts, mandatory fields, approval hierarchies, and integration touch points differ substantially. A regional subsidiary of a global enterprise operates in a different language, a different currency, and under different regulatory constraints than its parent.

Serving this diversity through code forks creates an unmaintainable combinatorial explosion. Serving it through rigid configuration tables limits customization to whatever the platform designers anticipated. Neither approach is adequate.

AwoERP addresses tenant customization through the UI platform itself. Because the backend composes and compiles UI definitions dynamically — resolving tenant context, user permissions, and feature flags at request time — **different tenants receive different UI experiences from the same underlying business logic**, without any code duplication. The customization layer is a first-class architectural concern, not an afterthought bolted onto a single-tenant design.

---

## 1.3 Platform Mission Statement

> **AwoERP's UI platform exists to ensure that every user, on every device, in every tenant, always experiences a UI that is consistent with the business rules, permissions, and configurations that govern their context — with no frontend code required to enforce or maintain that consistency.**

This mission has three practical implications:

**For engineers:** The backend is where UI intent is expressed. Frontend engineering is the craft of building capable, faithful rendering engines — not the craft of re-encoding business decisions that already exist elsewhere.

**For product and implementation teams:** Adding a new screen, modifying a workflow form, or customizing a tenant's experience is a backend and configuration concern. It does not require a frontend sprint, a mobile release, or a cross-team coordination cycle.

**For enterprise customers:** Customizations are first-class, version-controlled, and isolated to their tenant. Their instance of AwoERP can diverge from the platform defaults in precisely the ways they need, without affecting other tenants and without preventing them from receiving platform upgrades.

---

## 1.4 Key Value Propositions

### 1.4.1 One Backend, Many Surfaces

A single backend UI definition drives all client surfaces — web browser, iOS application, Android application, and any future surface (voice assistant, industrial terminal, embedded widget, AR overlay). The business logic, permissions, and UI intent are expressed once. Each surface renders that intent using its native capabilities.

This is not a compromise model where platforms are forced to share a lowest-common-denominator interface. The UI definition system is expressive enough that mobile clients can render native date pickers, sheet navigators, and haptic interactions — while web clients render the same underlying definition using browser-native controls, CSS grid layouts, and keyboard navigation patterns. The **definition is shared; the rendering is platform-native**.

### 1.4.2 Business Logic Drives UI, Not the Inverse

In conventional frontend architectures, business logic and UI logic are entangled. A component knows not only how to render itself but also when it should be visible, what values it should accept, and what should happen when the user interacts with it. This entanglement makes the frontend difficult to test, difficult to change, and prone to drift from backend truth.

In AwoERP, the backend expresses UI intent — field visibility, validation rules, action permissions, workflow states — as data. The frontend receives that data and renders it. Business logic does not leak into rendering engines. Rendering engines do not re-implement business logic. The separation is structural, not aspirational.

### 1.4.3 ERP-Native Component Vocabulary

General-purpose UI frameworks offer components appropriate for general-purpose applications: buttons, text inputs, modal dialogs, data tables. ERP applications require a richer vocabulary: multi-currency amount inputs, GL account selectors, approval action bars, line-item grids with tax breakdowns, audit trails, document attachment viewers, and workflow progress indicators.

AwoERP's component system is designed for the ERP domain. The component library is not a generic component system adapted for ERP — it is an ERP component system, built from the vocabulary of finance, procurement, inventory, HR, and sales. This means that a backend engineer composing a purchase order form uses `PurchaseOrderHeader`, `LineItemGrid`, `VendorCard`, and `ApprovalActionBar` — not `div`, `input`, `table`, and `button`.

### 1.4.4 Permission-Aware UI at the Source

In most systems, authorization is enforced at the API layer. The frontend independently decides which elements to show or hide, based on the user's role or permissions — a decision that is made in frontend code, duplicated from the backend's authorization model, and always at risk of divergence.

In AwoERP, the UI compilation pipeline integrates with the platform's authorization system (OpenFGA / Casbin) before the UI definition leaves the backend. Fields the user is not permitted to see are removed from the payload. Actions the user is not permitted to execute are absent from the definition. Navigation items the user cannot access are not included in the menu structure.

The frontend never receives a UI definition containing elements it should hide. **The hiding happens at the source.** The frontend renders exactly what it receives, and what it receives is already correct for the user's authorization context.

---

## 1.5 Who Should Read This Documentation

This documentation is written for multiple audiences, each with a different scope of interest. The following guide indicates which volumes and chapters are most relevant to each role.

### 1.5.1 Platform Engineers

Platform engineers are responsible for the UI compilation service, the AST framework, the JSON pipeline, and the component registry. They are the primary authors and maintainers of the platform itself.

**Essential reading:** All volumes. Platform engineers must understand the full system from vision through reference architecture.

**Start with:** Chapters 01–08 (the complete Phase 1 foundation), then proceed to Volume VI (Platform & API).

### 1.5.2 Backend (Go/Goa) Engineers

Backend engineers implement business services that emit UI AST nodes. They do not build rendering engines, but they must understand the component vocabulary, the AST construction API, and the action system well enough to express correct UI intent from their services.

**Essential reading:** Chapters 01–08 (foundation), Chapter 09 (component system), Chapters 11–16 (component library), Chapter 20 (action system), Chapter 23 (validation framework), Chapter 27 (authorization integration).

**Start with:** Chapter 01, then jump to Chapter 07 (AST Design) and Chapter 09 (Component System).

### 1.5.3 Mobile Engineers

Mobile engineers build the iOS and Android rendering engines. They receive JSON definitions and must faithfully materialize them using native platform capabilities.

**Essential reading:** Chapters 01–05 (conceptual foundation), Chapter 09 (component system), Chapter 17 (mobile rendering architecture), Chapters 19–21 (navigation, actions, events), Chapter 22 (state management), Chapter 33 (offline support), Chapter 37 (accessibility), Chapter 50 (SDK).

**Start with:** Chapter 01, then Chapter 05 (SDUI Fundamentals), then Chapter 17 (Mobile Rendering Architecture).

### 1.5.4 Web Engineers

Web engineers build the browser-based rendering engine. Like mobile engineers, they receive JSON definitions and render them using web platform capabilities.

**Essential reading:** Chapters 01–05 (conceptual foundation), Chapter 09 (component system), Chapter 18 (web rendering architecture), Chapters 19–21 (navigation, actions, events), Chapter 22 (state management), Chapter 33 (offline support), Chapter 37 (accessibility), Chapter 50 (SDK).

**Start with:** Chapter 01, then Chapter 05 (SDUI Fundamentals), then Chapter 18 (Web Rendering Architecture).

### 1.5.5 UI Framework Developers

UI framework developers design and implement the component library — the mapping between DSL component types and their native implementations on each platform. They sit at the intersection of platform engineering and rendering engineering.

**Essential reading:** Chapters 01–09 (full foundation and component system), Chapter 10 (layout), Chapter 17 (mobile rendering), Chapter 18 (web rendering), Chapter 37 (accessibility), Chapter 41 (testing), Chapter 45 (governance).

**Start with:** Chapter 02 (Design Philosophy) and Chapter 06 (UI DSL Architecture).

### 1.5.6 Product Owners and Solution Architects

Product owners and solution architects need to understand the platform's capabilities and constraints to design features, prioritize customization requirements, and evaluate feasibility without diving into implementation details.

**Essential reading:** Chapters 01–05 (foundation and philosophy), Chapter 28 (multi-tenant architecture), Chapter 29 (feature flags), Chapter 30 (customization framework), Chapter 46 (best practices), Chapter 47 (anti-patterns), Chapter 48 (reference architecture), Chapter 51 (roadmap).

**Start with:** Chapters 01–03. The philosophy chapters are sufficient to engage productively in platform design discussions.

### 1.5.7 ERP Implementers

ERP implementers deploy and configure AwoERP for specific enterprise customers. They need to understand the customization system, the tenant configuration model, and the available component vocabulary without necessarily understanding the compiler internals.

**Essential reading:** Chapter 01 (introduction), Chapter 04 (system overview), Chapter 28 (multi-tenant architecture), Chapter 30 (customization framework), Chapters 09–16 (component library), Chapter 46 (best practices), Chapter 49 (end-to-end examples).

**Start with:** Chapter 04 (System Overview) and Chapter 30 (Customization Framework).

### 1.5.8 Third-Party Integrators

Third-party integrators build plugins, custom components, or external systems that interact with the AwoERP UI platform via its APIs and SDKs.

**Essential reading:** Chapter 01 (introduction), Chapter 04 (system overview), Chapter 25 (API contracts), Chapter 31 (extension and plugin architecture), Chapter 43 (versioning strategy), Chapter 50 (SDK development).

**Start with:** Chapter 25 (API Contracts) and Chapter 31 (Extension and Plugin Architecture).

---

## 1.6 How to Use This Documentation

This documentation is organized into ten volumes, each representing a coherent layer of the platform. Within each volume, chapters are ordered from foundational concepts to implementation detail. Within each chapter, sections are ordered from "why" to "what" to "how."

**Reading strategies by goal:**

| Goal | Recommended Path |
|------|-----------------|
| Understand the platform vision in 30 minutes | Chapters 01, 02, 04 (introduction sections only) |
| Onboard as a new backend engineer | Chapters 01–08 in order, then Ch. 09, 11, 20 |
| Onboard as a new mobile/web engineer | Chapters 01–05, then Ch. 17 or 18, then Ch. 19–22 |
| Design a new ERP module's UI | Chapters 07, 09, 11–16, 20, 23 |
| Configure a new tenant | Chapters 28, 29, 30, 19 (navigation section) |
| Build a custom component plugin | Chapters 09, 31, 50 |
| Debug a UI rendering issue | Chapters 08, 17 or 18, 22, 38 |
| Prepare for a schema migration | Chapters 43, 44, 25 |

**Conventions used throughout this documentation:**

> 📘 **Note** — Additional context or clarification that is helpful but not critical.

> ⚠️ **Warning** — A decision or pattern with known negative consequences if applied incorrectly.

> 🚫 **Anti-Pattern** — A specific approach that violates platform principles and must be avoided.

> ✅ **Best Practice** — A recommended approach with explanation of why it is preferred.

> 🔗 **Cross-Reference** — A pointer to a related section in another chapter.

Code blocks are annotated with the language and context:

```go
// Go — AST Builder (backend service)
```

```typescript
// TypeScript — Rendering Engine (web client)
```

```json
// JSON — Compiled UI Definition (wire format)
```

```proto
// Protocol Buffers — gRPC service definition
```

---

## 1.7 Relationship to Other AwoERP Documentation

This documentation covers exclusively the **UI platform**: the server-driven UI architecture, the DSL and AST framework, the compilation pipeline, and the rendering engines. It does not document:

- **AwoERP Business Domain Documentation** — The functional specifications for finance, procurement, inventory, HR, and sales modules. Those documents describe what the business services do, not how they express UI.

- **AwoERP Infrastructure Documentation** — The deployment, networking, Kubernetes configuration, and operational runbooks for running AwoERP in production.

- **AwoERP Data Model Documentation** — The PostgreSQL schema, migration history, and data access patterns for AwoERP's business data.

- **AwoERP Integration Documentation** — The specifications for external system integrations (ERP connectors, payment gateways, tax services, identity providers).

- **AwoERP API Reference** — The auto-generated Goa API reference for all service endpoints. This documentation describes the API contract at a design and architecture level; the generated reference provides exhaustive endpoint-by-endpoint detail.

When this documentation refers to concepts from those other bodies of work, it provides a brief explanation sufficient for context and a cross-reference pointer. It does not reproduce content that is maintained elsewhere.

---

## 1.8 Glossary Reference

The following terms appear throughout this documentation with specific, precise meanings. Their full definitions are in the platform Glossary (`GLOSSARY.md`). This section provides working definitions sufficient to begin reading.

| Term | Working Definition |
|------|-------------------|
| **SDUI** | Server-Driven UI. An architectural pattern in which the backend defines what the client renders, rather than the client encoding that logic itself. |
| **UI Definition** | A JSON document produced by the backend that describes the complete visual and behavioral specification of a screen or surface. |
| **AST** | Abstract Syntax Tree. The in-memory, typed, structured representation of a UI definition, used internally by the backend before serialization to JSON. |
| **DSL** | Domain-Specific Language. The vocabulary of types, expressions, and constructs available for composing UI definitions within the backend. |
| **Surface** | A named, addressable unit of UI that a client can request — for example, a page, a modal, a dashboard, or a form. |
| **Component** | A named, typed, reusable UI element with a defined property schema, slot structure, and behavioral contract. |
| **Node** | A single element in the AST. Every component instance, layout container, data binding, and action reference is a node. |
| **Slot** | A named insertion point within a component where child nodes can be placed. |
| **Binding** | An expression that connects a component property to a data source, a state variable, or a computed value. |
| **Action** | A named, typed operation that the client executes in response to a user interaction or system event. |
| **Event** | A signal emitted by the client (or server) indicating that something has occurred, to which action chains can be bound. |
| **Compilation Pipeline** | The sequence of stages — context resolution, AST construction, transformation, validation, and serialization — that converts business intent into a JSON UI definition. |
| **Tenant** | A distinct organizational customer of AwoERP, with isolated data, configuration, permissions, and potentially customized UI definitions. |
| **Feature Flag** | A runtime boolean or multi-variant condition that controls which variant of a UI definition a given user or tenant receives. |
| **Rendering Engine** | A client-side runtime (web or mobile) responsible for deserializing a UI definition and materializing it as a visible, interactive interface. |
| **Permission Pruning** | The compilation pipeline pass in which AST nodes that the current user is not authorized to see or interact with are removed before serialization. |
| **OpenFGA** | Open Fine-Grained Authorization. The authorization system used by AwoERP to evaluate relationship-based access control (ReBAC) decisions. |
| **Temporal** | The workflow orchestration engine used by AwoERP for long-running, multi-step business processes. Workflow state drives corresponding UI state in the SDUI platform. |

---

*End of Chapter 01*

**Next:** [Chapter 02 — Design Philosophy](./02-design-philosophy.md)
