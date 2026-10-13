# Chapter 03 — Architectural Principles

> **Volume:** I — Vision & Philosophy
> **Phase:** 1 (Foundation)
> **Audience:** Platform Engineers, Solution Architects, Senior Backend/Frontend Engineers
> **Prerequisites:** Chapter 02 — Design Philosophy

---

## Table of Contents

- [3.1 Separation of Concerns: Business vs. Presentation](#31-separation-of-concerns-business-vs-presentation)
- [3.2 The AST-First Principle](#32-the-ast-first-principle)
- [3.3 Schema-Driven Everything](#33-schema-driven-everything)
- [3.4 Progressive Enhancement](#34-progressive-enhancement)
- [3.5 Backward Compatibility as a Contract](#35-backward-compatibility-as-a-contract)
- [3.6 Fail-Safe Degradation](#36-fail-safe-degradation)
- [3.7 Least-Privilege UI Rendering](#37-least-privilege-ui-rendering)
- [3.8 Composability Over Configuration](#38-composability-over-configuration)
- [3.9 Immutability of Compiled Definitions](#39-immutability-of-compiled-definitions)
- [3.10 Audit-Ability of UI Decisions](#310-audit-ability-of-ui-decisions)
- [3.11 Layered Architecture Overview](#311-layered-architecture-overview)
- [3.12 Cross-Cutting Concerns](#312-cross-cutting-concerns)

---

## 3.1 Separation of Concerns: Business vs. Presentation

The most fundamental architectural principle is a clear, enforced boundary between **business concerns** and **presentation concerns**.

**Business concerns** are questions that require domain knowledge to answer:
- Is this purchase order above the approval threshold for this tenant?
- Does this user have write access to GL codes in cost center 400?
- Is the vendor's credit limit sufficient for this order value?
- Which approval stages does this workflow require given the order amount and department?

**Presentation concerns** are questions that require platform knowledge to answer:
- How wide should this column be on a 375-point-width mobile screen?
- Should this input use a native iOS date picker or a custom calendar wheel?
- How many rows should the virtual list render before windowing kicks in?
- What is the correct tab order for keyboard navigation through this form?

Business concerns are answered in Go, in the service layer, using the business domain model. They are encoded into the UI definition as semantic data: field visibility flags, required markers, validation rules, action permissions. Presentation concerns are answered in the rendering engine, using platform-specific knowledge. The rendering engine reads semantic data and translates it into platform-appropriate presentation.

The separation fails when:
- A rendering engine contains a condition like `if userRole == 'manager' { show(approveButton) }`. This is a business concern (authorization) being answered in the presentation layer.
- A backend service specifies `textColor: '#FF0000'` to communicate "this field has an error." This is a presentation concern (color choice) being encoded in the semantic layer. The correct expression is `state: 'error'`, and the rendering engine decides what error state looks like.

Maintaining this separation requires discipline. The UI DSL must be rich enough to express business intent without encoding presentation decisions. The component library must map business-semantic props to platform-appropriate presentation. The architectural review process must catch boundary violations before they accumulate.

---

## 3.2 The AST-First Principle

Every UI definition in AwoERP originates as an **Abstract Syntax Tree (AST)**: a typed, in-memory, structured object graph built using the Go AST builder library. The JSON wire format is a serialization of this AST, not the primary representation.

This principle has several consequences:

**Validation at construction time.** When a backend engineer calls `NewPurchaseOrderHeader().WithVendor(vendorID).WithRequiredApprovalLevel(2)`, the builder validates the construction immediately. If `approvalLevel` must be between 1 and 5, the builder rejects out-of-range values before the AST is complete. Type errors are caught at compilation time (Go's type system enforces the node type hierarchy). Structural errors — a required slot left empty, a reference to an undefined data source — are caught by the AST validator before the pipeline proceeds. The frontend never receives a structurally invalid definition.

**Transformation as a first-class operation.** The AST can be traversed, transformed, and pruned programmatically. Permission pruning (removing unauthorized nodes), feature flag resolution (substituting variant subtrees), and localization injection (replacing translation keys with locale strings) are all transformations on the AST. These operations are well-defined, testable, and composable — because they operate on a structured tree, not on a partially-constructed JSON string.

**Multiple serialization targets.** Because the AST is the canonical form, it can be serialized to multiple formats: compact JSON for mobile clients, verbose JSON with debug metadata for development tooling, Protocol Buffer binary for gRPC delivery, or a canonical form for cache keying. The AST is serialization-format-agnostic.

**Testability.** The output of a UI compilation is an AST value. Tests can assert on specific nodes within that tree — "this form should contain a VendorSelector field with required=true when the order type is external" — without parsing JSON. The AST builder library includes test helpers for constructing expected subtrees and asserting their presence within a compiled result.

---

## 3.3 Schema-Driven Everything

Every data structure that crosses the boundary between backend and client — UI definitions, action payloads, event schemas, error envelopes — is defined by a formal schema. Schemas are the contracts that enable independent evolution of the components on each side of the boundary.

**JSON Schema** defines the structure of compiled UI definitions. Every component type has a corresponding JSON Schema definition for its property schema, required fields, and allowed nested types. The compilation pipeline validates against these schemas before serialization; the rendering engines validate against them upon receipt.

**Protocol Buffer definitions** (in `ui.proto`) define the gRPC API contract for UI services. These are the authoritative definitions from which Go server stubs, Go client stubs, TypeScript client types, and Swift/Kotlin client types are generated.

**OpenAPI / Goa DSL** defines the REST API surface. The Goa DSL is the source of truth; REST documentation, server handlers, and client SDKs are generated from it.

The discipline of schema-driven development prevents two classes of failure:
- **Silent incompatibility:** A backend changes a field name; the frontend continues to send the old name. Without schema validation on both sides, this failure is silent. With schema validation, it is detected immediately in integration tests.
- **Undocumented extensions:** An engineer adds a property to a component definition without updating the schema. The property works in development but is invisible to other teams and absent from the generated SDK types. With schema enforcement, the schema must be updated before the property is usable.

---

## 3.4 Progressive Enhancement

The platform is designed so that a minimal rendering engine can handle a UI definition correctly, and additional capabilities can be added incrementally without breaking core functionality.

A rendering engine must implement a **required capability set**: the core component types, the standard layout system, the basic action types (HTTP request, navigate, show notification), and the fundamental event handling. A rendering engine that implements only the required set is a valid, functional client.

Beyond the required set, the rendering engine may implement **optional capability sets**: chart rendering, inline editing, drag-and-drop, advanced gesture recognition, custom animation, offline action queuing. When a client declares its capabilities in the request (via the `X-Client-Capabilities` header), the backend uses this declaration to decide whether to include capability-dependent components in the definition, or to substitute simpler equivalents.

For example, a low-capability embedded client (perhaps a terminal in a warehouse) may not support chart rendering. When the backend compiles a dashboard definition for this client, chart widgets are replaced with tabular data equivalents. The same backend logic produces the same semantic content in a format the client can handle.

This progressive enhancement model ensures that the UI platform can serve clients with widely varying capabilities — from full-featured mobile apps to embedded terminals — without maintaining separate definition systems for each.

---

## 3.5 Backward Compatibility as a Contract

The schema is a contract. Rendering engines are built against a specific schema version. When the schema changes, existing rendering engines may not be aware of the changes. Backward compatibility rules determine whether those engines continue to function correctly.

**Backward-compatible changes** (safe to make without client updates):
- Adding an optional field to a component's property schema (clients ignore unknown optional fields)
- Adding a new component type (clients render an unknown-component fallback)
- Adding a new action type (clients execute unknown-action fallbacks — typically no-op or error display)
- Adding a new optional field to the request envelope

**Breaking changes** (require coordinated client update or versioned API endpoint):
- Removing a field from a component's property schema
- Renaming a field
- Changing the type of a field (e.g., from string to object)
- Removing a component type
- Changing the semantics of an existing field without changing its name or type (the most dangerous, because it is not detectable by schema validation alone)

Breaking changes must be introduced through the versioning process described in Chapter 43. This process includes a deprecation period during which both the old and new versions are supported, explicit client version negotiation, and a migration guide for rendering engine implementors.

The practical implication: **every schema change must be classified as backward-compatible or breaking before it is made.** This classification must be reviewed by the platform governance team (Chapter 45). It may not be inferred by the engineer making the change alone.

---

## 3.6 Fail-Safe Degradation

When a rendering engine encounters something it cannot handle — an unknown component type, an action type it does not support, a data binding expression it cannot evaluate — it must fail safely. "Failing safely" means:

1. **Render a graceful fallback** in place of the unsupported element. The fallback should indicate to the user that this section is unavailable, rather than crashing or showing an empty space with no explanation.
2. **Continue rendering the rest of the definition.** An unsupported component in one section of a form should not prevent the rest of the form from being displayed and used.
3. **Report the failure to the telemetry system.** Unknown component types, failed expressions, and unsupported action types must be logged with enough context (component type name, surface ID, client version) to diagnose the incompatibility.
4. **Never execute an action whose side effects are unclear.** If an action type is unknown, the safe failure is to take no action and display an error. It is never acceptable to attempt to guess the intended behavior.

The backend assists fail-safe degradation by including a `fallback` property on nodes that may not be supported by all clients. When a node type is unrecognized, the rendering engine substitutes the node's fallback subtree. For a chart widget, the fallback might be a simple text display of the underlying data. For a custom ERP component, the fallback might be a generic data table.

---

## 3.7 Least-Privilege UI Rendering

The UI definition delivered to a client should contain the **minimum information necessary to render the authorized view** for that user in that context. Nothing more.

This principle extends beyond permission pruning (removing unauthorized elements) to include:

**No speculative data.** The UI definition does not include data that might be useful in edge cases but is not required for the current view. Every piece of information in the payload was put there intentionally for the current user's current context.

**No authorization metadata in the payload.** The UI definition does not include role names, permission rule IDs, or authorization check results. It includes only the resolved outcome: this field is visible, this action is available. The mechanism by which that determination was made is not exposed to the client.

**No structural hints about what was removed.** A permission-pruned UI definition does not include empty slots, placeholder nodes, or comments indicating that elements were removed. The pruned definition looks exactly as if the removed elements never existed. This prevents clients from inferring the existence of features or data they are not authorized to access.

**Sensitive data masking.** Certain fields in UI definitions may reference sensitive data (for example, a display field showing a partially masked account number). The masking is applied at the compilation stage — the client receives only the masked value, never the original.

---

## 3.8 Composability Over Configuration

The UI DSL is designed for composition rather than configuration. This distinction is subtle but important.

**A configuration-based system** provides a fixed set of top-level settings that modify predefined templates. To support a new use case, a new configuration key must be added to the template. The template grows in complexity as more use cases are accommodated. Eventually, the configuration space becomes too large to reason about, and undocumented interactions between configuration keys create unpredictable behavior.

**A composition-based system** provides a set of primitives that can be combined in principled ways to produce any supported UI structure. The primitive set is stable; the combinations are limitless. New use cases are supported by composing existing primitives, not by adding new configuration keys.

AwoERP's DSL is composition-based. A form is composed of sections, which are composed of field groups, which contain fields. A dashboard is composed of widget containers, each containing a widget component. The composition is expressed as an AST — a tree of typed nodes with defined parent-child relationships. The depth and breadth of the tree are not fixed; any supported nesting is valid.

This approach enables backend engineers to express novel UI structures without waiting for the platform team to add a configuration option. If the primitives support the required structure, it can be expressed immediately. If the primitives do not support the required structure, that is the signal that a new primitive (or a new slot type on an existing component) is needed — and that request goes through the governance process.

---

## 3.9 Immutability of Compiled Definitions

Once a UI definition has been compiled and serialized, it is **immutable for its recipient**. The rendering engine receives a definition, renders it, and owns the rendered state. The definition does not change under the rendering engine's feet.

When the backend needs to update a UI definition — because a workflow step advanced, because a data source was refreshed, because a tenant configuration changed — it does not mutate the existing definition in place. It compiles a new definition (or a diff patch) and delivers it to the client through the event system. The client replaces or patches its current definition at a defined moment in the rendering lifecycle.

This immutability enables several important properties:
- **Deterministic rendering.** Given the same definition, a rendering engine should always produce the same visual output. There is no ambient mutable state that the definition depends on.
- **Time-travel debugging.** Stored UI definition snapshots can be replayed to reproduce exactly what a user saw at a specific moment — invaluable for debugging form submission errors or approval workflow state issues.
- **Safe caching.** An immutable definition with a known cache key can be cached aggressively. The cache is invalidated when the definition changes, not on every request.

---

## 3.10 Audit-Ability of UI Decisions

Every non-trivial decision made during UI compilation — which permission was evaluated, which feature flag was resolved, which tenant override was applied, which validation rule was injected — is recorded in a structured audit log.

This audit-ability serves multiple purposes:

**Debugging.** When a user reports "I can't see the Approve button on the purchase order form," the audit log for their most recent UI definition compilation shows exactly which permission check resulted in the action node being pruned. The support engineer does not need to reproduce the environment — the decision is recorded.

**Compliance.** Enterprise customers subject to audit requirements may need to demonstrate that their authorization controls function as documented. The UI compilation audit log provides evidence that field-level authorization rules were evaluated and applied correctly for specific users on specific dates.

**Regression detection.** When a change to the authorization rules or tenant configuration causes an unexpected change in the compiled UI definition for a specific user, the difference between the old and new audit logs identifies exactly which decision changed and why.

The audit log is append-only and immutable. It records the input context (user ID, tenant ID, surface ID, flag values, timestamp), the decision made (node ID, decision type, outcome, reason), and the output reference (compiled definition cache key). It does not record the full compiled definition — that is stored separately in the definition snapshot store.

---

## 3.11 Layered Architecture Overview

The AwoERP UI platform is organized into five layers. Each layer has a defined responsibility, a defined interface to the layers above and below it, and a defined set of components that live within it. No layer communicates with a non-adjacent layer except through the interface of the intervening layer.

```
┌─────────────────────────────────────────────────────────────────┐
│  Layer 5 — Client Interaction & State                           │
│  User input handling, local state, optimistic updates,          │
│  action queue, telemetry emission                               │
├─────────────────────────────────────────────────────────────────┤
│  Layer 4 — Client Rendering Engine                              │
│  JSON deserialization, component resolution, tree rendering,    │
│  event binding, action dispatch, offline support                │
├─────────────────────────────────────────────────────────────────┤
│  Layer 3 — Transport (JSON over REST / gRPC)                    │
│  HTTP/2 delivery, gRPC streaming, compression, caching,         │
│  ETag / If-None-Match, capability negotiation                   │
├─────────────────────────────────────────────────────────────────┤
│  Layer 2 — UI Compilation Layer                                 │
│  Context resolution, AST construction, transformation passes,   │
│  schema validation, serialization, audit logging                │
├─────────────────────────────────────────────────────────────────┤
│  Layer 1 — Business Services                                    │
│  Domain logic, AST emission, data sources, Temporal workflows,  │
│  OpenFGA authorization, feature flag evaluation                 │
└─────────────────────────────────────────────────────────────────┘
```

### 3.11.1 Layer 1 — Business Services

Business services are Go services implemented using the Goa framework. They own domain logic and are responsible for:
- Processing business transactions (creating orders, posting journal entries, processing payroll)
- Emitting UI AST nodes that describe the UI associated with their domain objects
- Defining data source endpoints that the UI definition references
- Interacting with Temporal for workflow orchestration
- Enforcing business rules on form submissions and action executions

Business services do not know about JSON serialization, HTTP response formatting, or client capability negotiation. They use the AST builder library to compose typed node trees and hand them to the compilation layer.

### 3.11.2 Layer 2 — UI Compilation Layer

The compilation layer accepts a raw AST from a business service and produces a compiled, validated, serialized UI definition ready for transport. Its responsibilities include:
- Resolving the request context (tenant, user, flags, locale, client capabilities)
- Applying transformation passes (permission pruning, flag resolution, localization, tenant overrides)
- Validating the transformed AST against component schemas
- Serializing to JSON (or binary proto for gRPC)
- Computing the cache key and writing to the definition cache
- Writing the compilation audit log entry

The compilation layer is a platform service — it is not owned by individual business service teams. It has no knowledge of specific business domains; it processes any well-formed AST that conforms to the component registry.

### 3.11.3 Layer 3 — Transport (JSON over REST/gRPC)

The transport layer handles the mechanics of delivering compiled definitions to clients:
- REST endpoints for synchronous UI definition requests
- gRPC streaming for real-time definition updates
- Cache header negotiation (ETag, If-None-Match, Cache-Control)
- Response compression
- Client capability negotiation (parsing the `X-Client-Capabilities` header)
- Rate limiting and circuit breaking

The transport layer does not interpret the content of UI definitions. It treats them as opaque payloads with known cache keys and content types.

### 3.11.4 Layer 4 — Client Rendering Engine

The rendering engine runs on the client device (browser or mobile). It is responsible for:
- Deserializing the JSON definition and validating it against the client-side schema copy
- Resolving component types against the client component registry
- Building and rendering the component tree
- Binding data sources and subscribing to real-time updates
- Binding event handlers and routing events to action chains
- Managing the rendering lifecycle (mount, update, unmount)
- Handling unknown components and actions with graceful fallbacks

The rendering engine is a platform component, shared across all surfaces on a given platform. It is not customized per ERP module or per tenant.

### 3.11.5 Layer 5 — Client Interaction & State

The interaction and state layer handles everything that happens after the initial render:
- Capturing user input events
- Maintaining local form state (controlled input values, dirty tracking)
- Executing action chains (dispatching HTTP requests, navigating, updating state variables)
- Managing the optimistic update lifecycle
- Queuing actions when offline
- Emitting telemetry events to the observability pipeline
- Handling session expiry and token refresh

---

## 3.12 Cross-Cutting Concerns

Four concerns cut across all layers and must be addressed consistently at each layer. They are not the responsibility of any single layer — each layer has its part.

### 3.12.1 Security

| Layer | Security Responsibility |
|-------|------------------------|
| Layer 1 | Emit only AST nodes appropriate for the domain; never include data the service is not authorized to expose |
| Layer 2 | Apply permission pruning; validate authorization against OpenFGA; mask sensitive fields |
| Layer 3 | Enforce TLS; validate JWT on every request; apply rate limits |
| Layer 4 | Validate received definitions against schema; never execute unknown action types |
| Layer 5 | Never store sensitive data in persistent local storage; always include auth tokens in action requests |

### 3.12.2 Observability

| Layer | Observability Responsibility |
|-------|------------------------------|
| Layer 1 | Emit span for AST construction; record construction errors |
| Layer 2 | Emit spans per transformation pass; record payload size and validation errors |
| Layer 3 | Emit HTTP request/response metrics; record cache hits/misses |
| Layer 4 | Emit render time metrics; record unknown component/action fallbacks |
| Layer 5 | Emit user interaction events; record action execution outcomes |

### 3.12.3 Tenancy

| Layer | Tenancy Responsibility |
|-------|------------------------|
| Layer 1 | Tenant-aware domain logic; tenant-specific data sources |
| Layer 2 | Apply tenant configuration overrides in transformation pass |
| Layer 3 | Tenant-aware cache key partitioning |
| Layer 4 | Apply tenant theme tokens; render tenant-registered custom components |
| Layer 5 | Include tenant context in all outbound API requests |

### 3.12.4 Localization

| Layer | Localization Responsibility |
|-------|------------------------------|
| Layer 1 | Use translation keys in AST string fields, not literal strings |
| Layer 2 | Apply localization injection pass — substitute translation keys with locale strings |
| Layer 3 | Include `Content-Language` header in responses |
| Layer 4 | Format numbers, dates, and currencies according to locale |
| Layer 5 | Send `Accept-Language` header; apply locale-appropriate input patterns |

---

*End of Chapter 03*

**Previous:** [Chapter 02 — Design Philosophy](./02-design-philosophy.md)
**Next:** [Chapter 04 — System Overview](./04-system-overview.md)
