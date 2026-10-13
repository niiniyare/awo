# Chapter 04 — System Overview

> **Volume:** I — Vision & Philosophy
> **Phase:** 1 (Foundation)
> **Audience:** All
> **Prerequisites:** Chapters 01–03

---

## Table of Contents

- [4.1 High-Level System Diagram](#41-high-level-system-diagram)
- [4.2 Platform Components Inventory](#42-platform-components-inventory)
- [4.3 Request Lifecycle: From Business Event to Rendered UI](#43-request-lifecycle-from-business-event-to-rendered-ui)
- [4.4 Technology Stack Mapping](#44-technology-stack-mapping)
- [4.5 Deployment Topology](#45-deployment-topology)
- [4.6 Scalability Characteristics](#46-scalability-characteristics)
- [4.7 Known Limitations](#47-known-limitations)

---

## 4.1 High-Level System Diagram

The following diagram presents the complete AwoERP UI platform, showing all major components and their relationships.

```
╔══════════════════════════════════════════════════════════════════════════╗
║                        CLIENT TIER                                       ║
║                                                                          ║
║  ┌───────────────────┐   ┌───────────────────┐   ┌───────────────────┐  ║
║  │   Web Browser     │   │   iOS Application  │   │ Android Application│  ║
║  │                   │   │                   │   │                   │  ║
║  │  Web Rendering    │   │ Mobile Rendering  │   │ Mobile Rendering  │  ║
║  │     Engine        │   │  Engine (Swift)   │   │ Engine (Kotlin)   │  ║
║  └────────┬──────────┘   └────────┬──────────┘   └────────┬──────────┘  ║
╚═══════════╪══════════════════════╪══════════════════════╪══════════════╝
            │                      │                      │
            │     HTTPS / gRPC (TLS)                      │
            │                      │                      │
╔═══════════╪══════════════════════╪══════════════════════╪══════════════╗
║           ▼                      ▼                      ▼              ║
║                         API GATEWAY                                     ║
║              (Auth · Rate Limiting · Routing · TLS Termination)        ║
║                                  │                                      ║
║         ┌────────────────────────┼────────────────────────┐            ║
║         ▼                        ▼                        ▼            ║
║  ┌─────────────────┐   ┌──────────────────┐   ┌────────────────────┐  ║
║  │ UI Compilation  │   │  Business Service │   │  Business Service  │  ║
║  │    Service      │   │  (Finance, etc.)  │   │ (Procurement, etc.)│  ║
║  │                 │◄──│                  │   │                    │  ║
║  │ - AST Transform │   │  - Domain Logic   │   │  - Domain Logic    │  ║
║  │ - Permission    │   │  - AST Emission   │   │  - AST Emission    │  ║
║  │   Pruning       │   │  - Data Sources   │   │  - Data Sources    │  ║
║  │ - Flag Resolve  │   └──────────────────┘   └────────────────────┘  ║
║  │ - Localization  │                                                    ║
║  │ - Validation    │   ┌──────────────────────────────────────────┐    ║
║  │ - Serialization │   │           PLATFORM SERVICES              │    ║
║  └────────┬────────┘   │                                          │    ║
║           │            │  ┌──────────────┐  ┌─────────────────┐  │    ║
║           │            │  │  Component   │  │  Feature Flag   │  │    ║
║           │            │  │  Registry    │  │  Service        │  │    ║
║           │            │  └──────────────┘  └─────────────────┘  │    ║
║           │            │  ┌──────────────┐  ┌─────────────────┐  │    ║
║           │            │  │  OpenFGA /   │  │  Localization   │  │    ║
║           │            │  │  Casbin      │  │  Service        │  │    ║
║           │            │  └──────────────┘  └─────────────────┘  │    ║
║           │            │  ┌──────────────┐  ┌─────────────────┐  │    ║
║           │            │  │  Temporal    │  │  Event Bus      │  │    ║
║           │            │  │  (Workflows) │  │  (NATS/Kafka)   │  │    ║
║           │            │  └──────────────┘  └─────────────────┘  │    ║
║           │            └──────────────────────────────────────────┘    ║
║           │                                                             ║
║           │            ┌──────────────────────────────────────────┐    ║
║           │            │           DATA TIER                      │    ║
║           │            │                                          │    ║
║           └───────────►│  ┌──────────────┐  ┌─────────────────┐  │    ║
║                        │  │  Redis       │  │  PostgreSQL     │  │    ║
║                        │  │  (UI Def     │  │  (Tenant Config │  │    ║
║                        │  │   Cache)     │  │   UI Customiz.) │  │    ║
║                        │  └──────────────┘  └─────────────────┘  │    ║
║                        └──────────────────────────────────────────┘    ║
╚═════════════════════════════════════════════════════════════════════════╝
```

---

## 4.2 Platform Components Inventory

### 4.2.1 UI Compilation Service

The UI Compilation Service is the central hub of the platform. It is a stateless Go service responsible for accepting raw AST input from business services and producing compiled, validated, serialized UI definitions for transport to clients.

**Responsibilities:**
- Accept `CompileUIDefinition` requests containing a surface identifier and compilation context
- Delegate AST construction to the appropriate business service
- Execute the transformation pipeline (six stages detailed in Chapter 08)
- Write compiled definitions to the Redis cache
- Return the serialized JSON definition to the caller
- Emit compilation audit log entries

**Interface:** gRPC (`UICompilationService`) with REST facade for browser clients.

**Scaling:** Horizontally scalable; stateless. All state lives in Redis and PostgreSQL.

**Dependencies:** Component Registry, OpenFGA/Casbin, Feature Flag Service, Localization Service, Redis, PostgreSQL.

### 4.2.2 Component Registry

The Component Registry is the catalog of all known component types — their schemas, versioning history, capability requirements, and fallback behaviors.

**Responsibilities:**
- Serve component schema lookups during AST validation
- Track component type versions and compatibility matrix
- Register custom components from third-party plugins
- Report component deprecation warnings during compilation

**Interface:** Internal gRPC service; not directly exposed to external clients.

**Storage:** PostgreSQL for persistent schema records; Redis for hot schema cache.

### 4.2.3 Schema Validation Service

The Schema Validation Service validates compiled ASTs against component schemas before serialization. It is invoked as a pipeline stage within the UI Compilation Service.

**Responsibilities:**
- Validate each AST node's props against the component's JSON Schema definition
- Validate required slots are populated
- Validate binding expressions are syntactically correct
- Validate action references resolve to known action types
- Report validation errors with precise node path and field context

**Interface:** Internal library (not a separate network service in the initial architecture); extracted to a standalone service if validation latency becomes a bottleneck.

### 4.2.4 Feature Flag Resolver

The Feature Flag Resolver evaluates feature flag conditions for a given user/tenant/environment context and returns a resolved flag map.

**Responsibilities:**
- Integrate with the platform's feature flag provider (e.g., Unleash, LaunchDarkly, or a custom implementation)
- Evaluate flag targeting rules for the current context (tenant ID, user ID, user segments, environment)
- Cache flag resolution results for the duration of the compilation request
- Provide a Go interface for use within the compilation pipeline

**Interface:** Go interface (`FlagResolver`) within the compilation pipeline; external HTTP/gRPC to the flag provider.

### 4.2.5 Authorization Resolver

The Authorization Resolver evaluates permission checks for the current user against the OpenFGA (or Casbin) policy store, and provides results to the permission pruning transformation pass.

**Responsibilities:**
- Batch authorization check requests to minimize OpenFGA round-trips
- Cache authorization decisions for the duration of the compilation request (with TTL for longer-lived contexts)
- Provide results to the permission pruning pass as a resolved permission map
- Write authorization decisions to the audit log

**Interface:** Go interface (`AuthorizationResolver`) within the compilation pipeline; gRPC to OpenFGA.

### 4.2.6 Localization Service

The Localization Service resolves translation keys to locale-appropriate strings for the current user's locale.

**Responsibilities:**
- Accept a locale identifier and a set of translation keys
- Return resolved translation strings from the translation catalog
- Apply pluralization rules
- Fall back to the configured fallback locale chain when a key is missing in the target locale
- Support tenant-specific translation overrides

**Interface:** Go interface (`LocalizationResolver`) within the compilation pipeline; HTTP to the translation catalog service (or in-memory for bundled translations).

### 4.2.7 Web Rendering Engine

The Web Rendering Engine is a TypeScript/JavaScript library that runs in the browser. It deserializes UI definitions and renders them as interactive web interfaces.

**Responsibilities:**
- Parse and validate received JSON definitions
- Resolve component types against the web component registry
- Build and manage a reactive component tree
- Execute action chains on user interaction
- Manage data source subscriptions and state
- Handle unknown components and actions with graceful fallbacks
- Emit telemetry events

**Technology:** TypeScript; built on top of a modern frontend framework (React or Vue — specified in Chapter 18).

### 4.2.8 Mobile Rendering Engine

The Mobile Rendering Engine runs on iOS (Swift) and Android (Kotlin). It deserializes UI definitions and renders them as native mobile interfaces.

**Responsibilities:** Same as the Web Rendering Engine, adapted for mobile platform conventions, input models, and performance characteristics.

**Technology:** Swift + SwiftUI (iOS); Kotlin + Jetpack Compose (Android). See Chapter 17 for details.

---

## 4.3 Request Lifecycle: From Business Event to Rendered UI

The following walkthrough traces a complete, concrete request: a user opening the "Create Purchase Order" screen on their iPhone.

### 4.3.1 Step 1 — Client Requests a UI Surface

The iOS app, following a user tap on "New Purchase Order," dispatches a request:

```
GET /api/ui/surfaces/procurement.purchase-order.create
Authorization: Bearer <jwt>
Accept: application/json
X-Tenant-ID: tenant_acme_corp
X-Client-Version: 3.2.1
X-Client-Capabilities: charts,inline-edit,native-date-picker,offline-queue
X-Client-Platform: ios
Accept-Language: en-US
```

The request is received by the API gateway, which:
- Validates the JWT and extracts the user identity and tenant
- Routes the request to the UI Compilation Service

### 4.3.2 Step 2 — Backend Resolves Context

The UI Compilation Service builds the compilation context object:

```go
ctx := CompilationContext{
    Surface:      "procurement.purchase-order.create",
    TenantID:     "tenant_acme_corp",
    UserID:       "user_jane_smith",
    UserRoles:    []string{"procurement_officer", "budget_approver"},
    Locale:       "en-US",
    Platform:     PlatformIOS,
    ClientVersion: semver.MustParse("3.2.1"),
    Capabilities: CapabilitySet{Charts, InlineEdit, NativeDatePicker, OfflineQueue},
    RequestedAt:  time.Now(),
}
```

Concurrently, the compilation service:
- Calls the Feature Flag Resolver to evaluate all relevant flags for this tenant/user
- Calls the Authorization Resolver to batch-load all permission checks needed for this surface
- Retrieves the tenant configuration record from PostgreSQL (or Redis cache)

### 4.3.3 Step 3 — Business Service Emits UI AST

The compilation service calls the Procurement business service's `BuildPurchaseOrderCreateAST` handler. The business service uses the AST builder library to construct the surface definition:

```go
func (s *ProcurementService) BuildPurchaseOrderCreateAST(
    ctx context.Context,
    compCtx *compilation.Context,
) (*ast.Surface, error) {

    surface := ast.NewSurface("procurement.purchase-order.create").
        WithTitle(i18n.Key("po.create.title")).
        WithLayout(ast.NewSingleColumnLayout()).
        AddChild(
            ast.NewForm("po-create-form").
                AddSection(
                    ast.NewFormSection("header").
                        WithTitle(i18n.Key("po.section.header")).
                        AddField(ast.NewVendorSelector("vendor_id").
                            WithLabel(i18n.Key("po.field.vendor")).
                            WithRequired(true).
                            WithPermission("procurement:vendor:select"),
                        ).
                        AddField(ast.NewDateField("required_date").
                            WithLabel(i18n.Key("po.field.required_date")).
                            WithMinDate(ast.ExprNow()).
                            WithRequired(true),
                        ).
                        AddField(ast.NewCurrencyAmountField("estimated_total").
                            WithLabel(i18n.Key("po.field.estimated_total")).
                            WithReadOnly(true).
                            WithBinding(ast.ComputedBinding("sum(line_items.amount)")),
                        ),
                ).
                AddSection(
                    ast.NewLineItemSection("line_items").
                        WithPermission("procurement:line_items:write"),
                ).
                WithActions(
                    ast.NewSubmitAction("submit_po").
                        WithLabel(i18n.Key("po.action.submit")).
                        WithEndpoint("POST", "/api/procurement/purchase-orders").
                        WithPermission("procurement:po:create"),
                    ast.NewNavigateAction("cancel").
                        WithLabel(i18n.Key("common.action.cancel")).
                        WithTarget("procurement.purchase-orders.list"),
                ),
        )

    return surface, nil
}
```

### 4.3.4 Step 4 — Compiler Validates and Serializes

The compilation service runs the AST through six transformation and validation stages (detailed in Chapter 08):

1. **Permission pruning:** The `estimated_total` field has no special permission, so it remains. The `line_items` section requires `procurement:line_items:write` — Jane has this permission, so the section remains. The `submit_po` action requires `procurement:po:create` — Jane has this, so the action remains.

2. **Feature flag resolution:** If a flag `procurement.enhanced-vendor-search` is active for ACME Corp, the `VendorSelector` node is replaced with an enhanced variant that includes vendor scoring data.

3. **Tenant override application:** ACME Corp has configured a custom label for "vendor" — "supplier" — in their localization overrides. The `i18n.Key("po.field.vendor")` now resolves to "Supplier" for this tenant.

4. **Localization injection:** All `i18n.Key(...)` references are replaced with their resolved English strings.

5. **Schema validation:** Each node is validated against its component schema. The `VendorSelector` with `required=true` and `permission` is valid. All references resolve.

6. **Serialization:** The validated AST is serialized to JSON, compressed with gzip, and written to Redis with a cache key derived from: `surface + tenant + user_roles + flag_hash + locale + client_version`.

### 4.3.5 Step 5 — JSON Delivered to Client

The API gateway returns the compiled definition:

```
HTTP/2 200 OK
Content-Type: application/json
Content-Encoding: gzip
ETag: "sha256:a3f9b2..."
Cache-Control: private, max-age=300
X-Compilation-Time-Ms: 23
X-Cache: MISS
```

```json
{
  "schema_version": "2.1.0",
  "surface_id": "procurement.purchase-order.create",
  "title": "Create Purchase Order",
  "compiled_at": "2026-06-01T09:15:32Z",
  "root": {
    "id": "root-layout",
    "type": "layout.single_column",
    "children": [
      {
        "id": "po-create-form",
        "type": "form.container",
        "props": {
          "submit_action": "submit_po"
        },
        "sections": [
          {
            "id": "header",
            "type": "form.section",
            "props": { "title": "Order Details" },
            "fields": [
              {
                "id": "vendor_id",
                "type": "field.vendor_selector",
                "props": {
                  "label": "Supplier",
                  "required": true,
                  "variant": "enhanced"
                }
              },
              {
                "id": "required_date",
                "type": "field.date",
                "props": {
                  "label": "Required Date",
                  "required": true,
                  "min_date": { "$expr": "now()" },
                  "input_hint": "native_date_picker"
                }
              },
              {
                "id": "estimated_total",
                "type": "field.currency_amount",
                "props": {
                  "label": "Estimated Total",
                  "read_only": true,
                  "value": { "$expr": "sum(line_items.amount)" }
                }
              }
            ]
          },
          {
            "id": "line_items",
            "type": "form.line_item_section",
            "props": {}
          }
        ],
        "actions": [
          {
            "id": "submit_po",
            "type": "action.http_request",
            "props": {
              "label": "Submit Purchase Order",
              "method": "POST",
              "endpoint": "/api/procurement/purchase-orders",
              "variant": "primary"
            }
          },
          {
            "id": "cancel",
            "type": "action.navigate",
            "props": {
              "label": "Cancel",
              "target": "procurement.purchase-orders.list",
              "variant": "secondary"
            }
          }
        ]
      }
    ]
  }
}
```

### 4.3.6 Step 6 — Client Renders Definitively

The iOS rendering engine:
1. Deserializes the JSON and validates against the client-side schema copy
2. Resolves each `type` string against the component registry (e.g., `field.vendor_selector` → `VendorSelectorView`)
3. Builds the SwiftUI view tree from the resolved components and their props
4. Binds data sources — subscribes to the form state store, connects the `estimated_total` computed expression
5. Recognizes `input_hint: native_date_picker` on the date field and renders a native `DatePicker` view
6. Renders the screen — Jane sees the "Create Purchase Order" form with her tenant's label ("Supplier") and the enhanced vendor search (because the flag was active)

### 4.3.7 Step 7 — Client Submits Actions / Events

When Jane taps "Submit Purchase Order":
1. The rendering engine evaluates the action's pre-conditions (form validation)
2. Serializes the form data
3. Executes the `action.http_request` with the defined endpoint and method
4. Receives the business service response
5. Follows the action's success branch: navigates to the purchase order detail view

---

## 4.4 Technology Stack Mapping

### 4.4.1 Go + Goa for Service Contracts

AwoERP's backend is implemented in Go. The Goa framework provides:
- A DSL for defining REST and gRPC service contracts
- Code generation for server handlers, client stubs, and OpenAPI documentation
- Transport-agnostic service design (the service logic is separated from the HTTP/gRPC transport)

The UI Compilation Service and all business services are Go services defined with Goa. The AST builder library is a Go library imported by business services.

### 4.4.2 PostgreSQL for Persistent UI Definitions

PostgreSQL stores:
- Tenant configuration records (theme overrides, label customizations, feature configurations)
- Custom UI definition overrides (tenant-specific form layouts, field additions)
- Component registry schema records
- User preference data (dashboard layouts, column visibility settings)
- Compilation audit logs (for compliance and debugging)

All PostgreSQL access is through the `pgx` driver with connection pooling via `pgxpool`. Migrations are managed with a schema migration tool (Atlas or golang-migrate).

### 4.4.3 Redis for UI Definition Caching

Redis stores:
- Compiled UI definition cache (key: composite of surface ID + tenant + role hash + flag hash + locale + client version)
- Authorization decision cache (key: user ID + resource + action, TTL: 60 seconds)
- Feature flag resolution cache (key: tenant ID + user ID, TTL: 30 seconds)
- Tenant configuration cache (key: tenant ID, TTL: 5 minutes, invalidated on config change)

Redis is the primary performance lever for the compilation pipeline. A cache hit reduces compilation latency from ~20–50ms to ~2–5ms.

### 4.4.4 Temporal for Long-Running Workflow UI

Temporal orchestrates multi-step business workflows (purchase order approval chains, multi-stage invoice processing, payroll runs). The UI platform integrates with Temporal by:
- Subscribing to Temporal workflow state via the Temporal Go SDK
- Mapping workflow state to UI definition inputs (the current workflow step drives which actions are available)
- Sending Temporal signals when the user executes workflow actions (approve, reject, delegate)
- Displaying workflow progress using the `WorkflowProgressStepper` component

See Chapter 15 for the complete workflow UI component documentation.

### 4.4.5 OpenFGA/Casbin for Authorization-Aware UI

The authorization system uses OpenFGA for relationship-based access control (ReBAC). Each object in AwoERP (purchase order, GL account, cost center, employee record) has defined relationships (owner, approver, viewer) and each user has defined relationships to specific objects.

During UI compilation, the authorization resolver evaluates checks such as:
- Can `user:jane` read `field:gl_code` on `object:purchase-order:po-12345`?
- Can `user:jane` execute `action:approve` on `object:purchase-order:po-12345`?

These checks are batched and cached. The results drive the permission pruning pass. See Chapter 27 for the complete authorization integration documentation.

Casbin is used as a fallback for role-based (RBAC) checks on surfaces where ReBAC is unnecessary — for example, access to administrative configuration screens.

### 4.4.6 Feature Flags for Tenant Variations

Feature flags control which variant of a UI surface a given user or tenant receives. The flag system supports:
- Boolean flags (feature on/off)
- Multi-variant flags (A/B/C variants)
- Percentage rollout flags (gradual rollout to a percentage of users)
- Tenant-targeted flags (enabled only for specific tenants)

See Chapter 29 for the complete feature flag architecture documentation.

---

## 4.5 Deployment Topology

AwoERP is deployed on Kubernetes. The UI platform components are deployed as follows:

| Component | Deployment Type | Replicas | Notes |
|-----------|----------------|----------|-------|
| UI Compilation Service | Deployment | 3–10 (HPA) | Stateless; scales on CPU and request queue depth |
| Component Registry | Deployment | 2 | Low traffic; schema reads cached in Redis |
| API Gateway | Deployment | 3+ | Handles TLS termination, auth, routing |
| Redis | StatefulSet | 3 (cluster mode) | Persistence enabled; AOF + RDB snapshots |
| PostgreSQL | StatefulSet | 1 primary + 2 replicas | Primary for writes; replicas for reads |

Business services are deployed independently by their respective service teams. Each business service is a separate Kubernetes Deployment with its own HPA configuration.

The rendering engines are deployed differently:
- **Web:** Served as a static bundle from a CDN; updated with each release
- **iOS:** Distributed via the App Store; updated through user-initiated or force-prompted upgrades
- **Android:** Distributed via the Google Play Store; same update model as iOS

---

## 4.6 Scalability Characteristics

**UI Compilation Service** scales horizontally. The primary scaling constraint is the latency budget for a compilation request. Cold compilations (cache miss) take 20–50ms including authorization checks and transformation passes. Warm compilations (cache hit) take 2–5ms. The target p99 latency is <100ms end-to-end from API gateway to serialized response.

**Cache efficiency** is the primary performance lever. The cache hit rate depends on the diversity of compilation contexts. For a tenant with 100 users who share the same role set, the cache hit rate after the first request per surface approaches 100%. For a tenant with highly individualized permission configurations, the hit rate will be lower. Cache hit rate is monitored and reported; a rate below 70% triggers investigation.

**Authorization resolution** is the most latency-sensitive operation in the cold compilation path. Batch authorization checks against OpenFGA are used to minimize round-trips. Authorization results are cached within the compilation request and, for longer-lived contexts (e.g., read-heavy dashboards), in Redis with a 60-second TTL.

**Component Registry** is read-heavy and nearly static. The schema definitions change only during component library releases. The registry is cached aggressively in Redis and in-process memory.

---

## 4.7 Known Limitations

The following are known limitations of the current platform design. They are documented here so they can be tracked, prioritized, and addressed in the platform roadmap (Chapter 51).

**Real-time collaborative editing is not supported.** The UI definition model assumes a single user owns the form state at any point. Concurrent editing by multiple users of the same document is not currently modeled.

**Highly dynamic forms with large conditional trees may produce large payloads.** A form with 200 fields and complex conditional logic may produce a definition exceeding 100KB even after compression. Payload size budgets and incremental update mechanisms (Chapter 32) mitigate this, but very large form surfaces may require architectural accommodation.

**The mobile rendering engine cannot execute arbitrary expressions.** The binding expression language is intentionally constrained. Complex computed fields that require full business logic must be computed on the backend and delivered as static values, rather than as expressions evaluated on the client.

**Third-party custom components require platform approval.** Plugin components must be reviewed and registered by the platform team before they can be referenced in UI definitions (Chapter 31, Chapter 45). There is no mechanism for tenants to deploy components without this review process.

**Schema versioning requires coordinated client releases.** Breaking schema changes require all active clients to be updated before the old schema version is retired. The multi-version support window (Chapter 43) determines how long legacy schema versions remain supported.

---

*End of Chapter 04*

**Previous:** [Chapter 03 — Architectural Principles](./03-architectural-principles.md)
**Next:** [Chapter 05 — Server-Driven UI Fundamentals](../vol-02-dsl-and-ast/05-sdui-fundamentals.md)
