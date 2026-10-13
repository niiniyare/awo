# Chapter 02 — Design Philosophy

> **Volume:** I — Vision & Philosophy
> **Phase:** 1 (Foundation)
> **Audience:** All Engineers, Product Owners, Solution Architects
> **Prerequisites:** Chapter 01 — Introduction

---

## Table of Contents

- [2.1 The Server-Driven UI Manifesto](#21-the-server-driven-ui-manifesto)
- [2.2 Inspirations and Prior Art](#22-inspirations-and-prior-art)
- [2.3 Core Design Tenets](#23-core-design-tenets)
- [2.4 Design Trade-offs Accepted](#24-design-trade-offs-accepted)
- [2.5 Non-Goals of This Platform](#25-non-goals-of-this-platform)
- [2.6 Philosophy vs. Implementation — Where Rules Can Flex](#26-philosophy-vs-implementation--where-rules-can-flex)

---

## 2.1 The Server-Driven UI Manifesto

Every software architecture embodies a theory about where intelligence should live. In a traditional frontend architecture, intelligence is distributed: the backend manages data and enforces business rules; the frontend independently manages layout, navigation, visibility, and the mapping from backend data to UI state. Both sides hold partial copies of the truth about what the user should see and do. Neither side has the complete picture alone.

AwoERP is built on a different theory: **the backend should be the complete and sole authority on what the user sees, and the frontend should be an exceptionally capable executor of backend intent.**

This is not a claim that frontend engineering is unimportant. Quite the opposite. Building a rendering engine that faithfully materializes a rich, typed, behavioral specification — across web, iOS, and Android, with pixel-appropriate native interactions, offline support, and accessibility — is substantial and sophisticated engineering work. The claim is that this engineering effort should be directed at **execution excellence**, not at re-encoding decisions that the backend already owns.

The Server-Driven UI manifesto, as it applies to AwoERP, can be stated in five sentences:

1. The backend knows the user, the tenant, the permissions, the workflow state, and the business rules. The backend should decide what the user sees.
2. The frontend knows the device, the platform conventions, the accessibility requirements, and the rendering capabilities. The frontend should decide how to render what it is told.
3. The boundary between "what" and "how" should be explicit, versioned, and enforced by schema.
4. Anything that crosses that boundary in the wrong direction — business logic in the frontend, rendering decisions in the backend — is a defect.
5. The schema is the contract. Changing the schema is a breaking change. Breaking changes require explicit versioning and migration.

---

## 2.2 Inspirations and Prior Art

AwoERP's SDUI approach draws from several prior systems. Understanding their contributions and limitations clarifies the design choices made here.

### 2.2.1 AMIS (Baidu Low-Code Framework)

AMIS is an open-source, JSON-driven UI framework developed by Baidu. It allows developers to describe complex UI — forms, tables, charts, dashboards — as JSON configurations rather than as component code. AMIS demonstrated that a sufficiently rich JSON vocabulary can express enterprise-grade UI complexity without sacrificing flexibility.

**What AwoERP inherits:** The concept that a typed, hierarchical JSON schema can express complete UI definitions, including nested components, data bindings, conditional logic, and actions.

**What AwoERP does differently:** AMIS is primarily a frontend framework — the JSON is authored by frontend developers and delivered statically. AwoERP generates its JSON definitions dynamically at the backend, incorporating authorization, tenant context, and workflow state at compile time. The JSON is not a static template; it is a computed artifact specific to the requesting user and context.

### 2.2.2 Retool's Declarative Component Model

Retool introduced the concept of a drag-and-drop tool-builder backed by a declarative component model, where component properties can reference data sources, transformation expressions, and event handlers using a consistent binding syntax. Retool demonstrated that business users and developers could collaborate on UI construction using a shared, expressive language.

**What AwoERP inherits:** The binding expression model — the idea that component properties can reference data sources, state variables, and computed expressions rather than only static values. The action system design also draws from Retool's event-action chaining model.

**What AwoERP does differently:** Retool's UI definitions are constructed interactively by developers or power users at runtime, stored in a Retool-managed database, and served as-is to all users. AwoERP generates UI definitions programmatically from Go services, applies per-user authorization and tenant context at compile time, and serves different definitions to different users from the same logical surface. The generation is code, not configuration.

### 2.2.3 Appsmith's Event-Action System

Appsmith introduced a mature event-action system in which user interactions trigger named actions — API calls, navigation events, state mutations, notifications — that can be chained and conditioned. This system made complex, stateful UI behavior expressible in a declarative configuration without custom JavaScript.

**What AwoERP inherits:** The action system architecture (Chapter 20) is directly inspired by Appsmith's event-action model. Named action types, action chaining, conditional execution, and error branches all trace to this lineage.

**What AwoERP does differently:** Appsmith actions are defined by frontend developers in the Appsmith editor and executed in the browser. AwoERP action definitions are authored in the backend as part of the UI AST and are permission-checked and validated before they reach the client. The client executes actions it is given; it does not define them.

### 2.2.4 PowerApps Formula-Driven Bindings

Microsoft PowerApps introduced a formula language (inspired by Excel) for expressing reactive UI behavior — computed fields, conditional visibility, dynamic validation — in terms that business analysts could author without programming. This demonstrated that binding expressions could be powerful enough for real business requirements without requiring a full programming language.

**What AwoERP inherits:** The concept of expression bindings — short, safe expressions attached to component properties that compute values from data sources and state. These are not arbitrary code; they are a constrained expression language designed for UI binding contexts.

**What AwoERP does differently:** PowerApps formulas are authored in a visual designer by non-developers. AwoERP binding expressions are authored in Go by backend engineers, as typed constructs in the AST builder API. The expressiveness is comparable; the authoring model and the runtime are different.

### 2.2.5 What AwoERP Does Differently

The common thread across AMIS, Retool, Appsmith, and PowerApps is that each of them treats the UI definition as something **authored separately from the business logic**. The UI definition is a layer that sits atop the business services and must be independently maintained.

AwoERP collapses this separation. **The business service is the UI author.** A Go service that handles purchase order creation does not merely process the form submission — it also defines the form. The form definition is an expression of the business rules encoded in that service: which fields are required by the domain, which depend on vendor type, which require manager approval. The form and the business logic that validates it are written by the same engineer, in the same codebase, versioned together. There is no separate UI configuration layer maintained by a different team with potentially different knowledge.

---

## 2.3 Core Design Tenets

These tenets are the principles from which specific design decisions are derived. When a future architectural question arises — whether it concerns a new component type, a schema change, an extension mechanism, or a performance optimization — these tenets provide the basis for a principled answer.

### 2.3.1 Tenet 1 — UI as Data, Not Code

The UI definition is data. It is a value that can be validated, versioned, cached, diffed, tested, and transported. It is not executable code that runs in the frontend. It is a structured description of intent.

This distinction has practical consequences. Data can be validated against a schema before it leaves the backend — ensuring that the frontend never receives a malformed definition. Data can be diffed — enabling incremental updates that send only changes rather than full page definitions. Data can be cached — because it is deterministic for a given user/tenant/context combination. Data can be tested with snapshot assertions — because the output of the compilation pipeline is a value that can be compared precisely.

**Implication:** The backend must never produce UI definitions that depend on client-side code evaluation to be complete. Expressions in bindings are evaluated by the rendering engine using a defined, constrained expression language — not by executing arbitrary JavaScript or Go code delivered in the payload.

### 2.3.2 Tenet 2 — The Backend Is the Source of Truth

All decisions about what the user should see, what they are permitted to do, and how business data maps to UI state are made by the backend. The frontend's role is to execute these decisions faithfully.

This tenet has a corollary: **the frontend must never make a decision that the backend should have made.** If the frontend is checking a user's role to decide whether to show a button — rather than rendering what the backend told it to render — the frontend is overstepping its role. Conversely, if the backend is deciding which CSS class to apply based on platform-specific conventions — rather than expressing a semantic intent that the frontend maps to platform-appropriate styling — the backend is overstepping its role.

The boundary is between semantic intent ("this action requires manager approval") and platform execution ("this button should be disabled and have a lock icon"). Semantic intent belongs to the backend; platform execution belongs to the frontend.

### 2.3.3 Tenet 3 — Clients Are Rendering Engines, Not Decision Makers

Rendering engines are built to be excellent at rendering. They understand their platform's conventions, performance characteristics, accessibility requirements, and input models. They use this knowledge to materialize backend intent in a way that is native, performant, and appropriate.

They do not understand business rules. They do not know whether a purchase order over a certain amount requires dual approval. They do not know whether a specific tenant has disabled the vendor credit check. They do not know whether the current user's role grants edit access to GL codes. All of these are business decisions, and they arrive pre-made in the UI definition.

A rendering engine that discovers it has received a component type it does not recognize should render a graceful fallback — not attempt to infer the component's behavior from its properties. A rendering engine that receives an action it cannot execute locally should report the error — not silently skip the action. Faithful execution includes faithful error reporting.

### 2.3.4 Tenet 4 — Components Speak Business, Not HTML

The component vocabulary of the platform is defined at the level of business concepts, not HTML primitives. A backend engineer expressing a purchase order form writes `PurchaseOrderHeader`, `VendorSelector`, and `LineItemGrid` — not `div`, `form`, `input[type=text]`, and `table`. The business vocabulary is mapped to platform primitives by the rendering engine, which has the knowledge to make that mapping correctly for its environment.

This tenet enables the component library to evolve independently of the rendering engines. When a new native iOS control becomes available that better serves the `DatePicker` component, the iOS rendering engine can adopt it without any change to the backend. The backend still emits `DatePicker`; the rendering engine decides what native control that maps to.

It also means that the component library reflects the ERP domain. Components are named and parameterized in the language of business: `ApprovalStatus`, `CurrencyAmount`, `GLAccountSelector`, `AuditTrail`. This vocabulary is shared between backend engineers who author UI definitions and product stakeholders who review and specify them.

### 2.3.5 Tenet 5 — Permissions Are First-Class Citizens

Authorization is not a filter applied after the UI is built. It is a dimension of the UI definition itself, evaluated during compilation and enforced at the source.

The UI compilation pipeline integrates with the authorization system at multiple levels: page access, section visibility, field read/write access, action executability, and row-level data visibility. The compiled UI definition delivered to the client already reflects the user's authorization context. Elements the user cannot see or interact with are absent — not hidden, not disabled by client-side code, but **absent from the payload**.

This means the client cannot circumvent authorization by manipulating the DOM, by calling an action endpoint directly, or by finding hidden form fields. Authorization is enforced at the API level (as always) and also at the UI definition level (so the client never even knows about elements it cannot use).

### 2.3.6 Tenet 6 — Extensibility Without Forking

The platform must be extensible — able to accommodate new component types, custom actions, tenant-specific behaviors, and third-party integrations — **without requiring modifications to the core platform codebase**.

Every extension point is defined explicitly: the component registry, the action type registry, the data source adapter interface, the pipeline transformation hook interface. Third-party components are registered, not merged. Tenant customizations are layered on top of base definitions, not embedded within them. The core platform evolves on its own roadmap; extensions evolve on theirs.

This tenet prevents the gradual accumulation of tenant-specific logic in the core platform — a pattern that eventually makes the platform unmaintainable and upgrade-resistant.

### 2.3.7 Tenet 7 — Observability Built In

Every stage of the compilation pipeline, every action execution, every data source fetch, and every client rendering event is designed to be observable from the start. Observability is not instrumented after the fact; it is specified as part of the platform's operational contract.

The platform emits structured trace spans per pipeline stage, metrics on payload sizes and compilation latency, and client-side telemetry on render time and interaction events. This data is not optional — it is the mechanism by which the platform team detects regressions, tunes performance, and understands how the UI platform is actually used in production.

---

## 2.4 Design Trade-offs Accepted

Every architectural decision involves trade-offs. The following are trade-offs that AwoERP explicitly accepts as the correct choice given its goals. They are documented here to prevent future decisions from inadvertently reversing them under pressure.

### 2.4.1 Accepting Backend Coupling for Consistency

In a pure microservices architecture, services are loosely coupled: they communicate through well-defined APIs and make no assumptions about each other's internals. The AwoERP UI platform introduces a form of coupling: business services must use the UI AST builder library and understand the component vocabulary in order to emit UI definitions.

This is an accepted trade-off. The alternative — a separate UI service that knows nothing about business rules and composes UI from generic data — would reproduce the problem that SDUI solves: the UI and the business logic would be maintained separately and would diverge.

The coupling is managed by ensuring that the AST builder library has a stable, versioned API; that the component vocabulary is defined at the business concept level (not the HTML level); and that business services do not need to know how their UI definitions will be rendered.

### 2.4.2 Accepting JSON Payload Overhead for Flexibility

A UI definition JSON payload is larger than a raw API response containing only business data. A form definition includes not just the current field values but the entire structural description of the form — field types, labels, validation rules, layout configuration, action definitions. For a complex ERP form, this payload may be 5–50KB.

This is an accepted trade-off. The payload overhead is mitigated by caching (UI definitions rarely change between requests for the same user/context), compression (gzip/brotli reduce JSON payloads by 70–85%), and incremental updates (when only a portion of the definition changes, only the diff is transmitted). The flexibility gained — the ability to change any aspect of the UI without a client release, the ability to serve different definitions to different users, the ability to test UI changes by examining the compiled JSON output — substantially outweighs the payload overhead.

### 2.4.3 Accepting Rendering Engine Complexity for Platform Control

Building a rendering engine that faithfully materializes an arbitrarily complex UI definition is more complex than building a set of screens with hard-coded structure. The rendering engine must handle unknown component types gracefully, manage the lifecycle of dynamically composed component trees, execute action chains with error handling, and maintain reactive state bindings.

This complexity is accepted because the alternative — allowing any complexity to live in the rendering engine in the form of business logic — would be more complex overall, distributed across three platforms, and impossible to test centrally. The rendering engine complexity is **one-time, centralized, and testable**. The business logic complexity, if allowed to live in the frontend, would be **recurring, distributed, and impossible to keep consistent**.

---

## 2.5 Non-Goals of This Platform

Being explicit about what this platform does not attempt to do is as important as being explicit about what it does. The following are deliberately outside scope.

**This platform is not a no-code builder for non-developers.** The UI definition is authored in Go, using the AST builder library. It is not intended to be configured through a visual drag-and-drop interface by business analysts without engineering involvement (though a visual tooling layer could be built on top of the platform in the future — see Chapter 51, Roadmap).

**This platform is not a general-purpose frontend framework.** It is not a replacement for React, Vue, SwiftUI, or Jetpack Compose. The rendering engines use those frameworks internally. The platform does not compete with them; it uses them.

**This platform does not own the design system.** The visual design tokens, typography, color palette, and spacing system are defined by AwoERP's design system. The UI platform consumes design tokens; it does not define them.

**This platform does not replace API design.** The UI platform defines how users interact with the system. The underlying API endpoints that handle form submissions, action executions, and data fetches are the responsibility of the respective business service teams. The UI platform specifies the interface; the business services implement the behavior.

**This platform does not handle authentication.** Authentication (who is the user?) is handled by AwoERP's identity infrastructure. The UI platform consumes the authenticated user identity; it does not implement login flows, token management, or session handling.

**This platform is not a reporting engine.** Complex analytical reports — multi-dimensional pivot tables, custom SQL queries, export pipelines — are handled by AwoERP's reporting subsystem. The UI platform can display report outputs through its chart and table components, but it does not generate reports.

---

## 2.6 Philosophy vs. Implementation — Where Rules Can Flex

The tenets in Section 2.3 are architectural principles, not absolute laws. There are situations where pragmatic implementation choices deviate slightly from the purest expression of a tenet. This section documents the known, accepted deviations — so they are recognized as deliberate exceptions rather than violations, and so they do not expand beyond their intended scope.

**Exception 1: Client-side validation as UX enhancement, not enforcement.**
Tenet 2 states that all decisions are made by the backend. Yet rendering engines implement validation rules client-side — showing an error message before the form is submitted, rather than requiring a round-trip. This is a UX enhancement, not a deviation from backend authority. The backend always re-validates on submission. The client-side validation is a copy of the backend's validation rules, delivered in the UI definition, executed locally for user experience, and never trusted as the enforcement point.

**Exception 2: Client-side navigation state as transient UI state.**
Tenet 2 implies that all state is server-owned. In practice, the current navigation position (which screen is active, which tab is selected, which modal is open) is maintained as transient client state. The backend defines the navigation structure — available routes, guard conditions, menu items — but the client owns the current position within that structure. This is appropriate because navigation state is ephemeral and device-specific; it is not business state that needs to be persisted or synchronized.

**Exception 3: Platform-specific rendering optimizations.**
Tenet 3 says the client renders what it is told. Rendering engines may apply platform-specific optimizations — list virtualization, image lazy loading, component memoization — that result in components not being rendered until they enter the viewport. This is faithful execution: the logical definition is complete; the physical rendering is optimized for performance. The optimization is invisible to the user and does not change the semantic content of what is displayed.

**Exception 4: Offline action queuing as client-side business behavior.**
When the device is offline, the rendering engine queues actions and replays them when connectivity is restored. This involves the client making decisions about ordering, deduplication, and conflict detection — decisions that would normally be server-side. This is accepted as a necessary accommodation to the physical reality of mobile connectivity. The server still validates and enforces all rules when the queued actions are replayed; the client-side queueing is a delivery mechanism, not an authorization bypass.

---

*End of Chapter 02*

**Previous:** [Chapter 01 — Introduction](./01-introduction.md)
**Next:** [Chapter 03 — Architectural Principles](./03-architectural-principles.md)
