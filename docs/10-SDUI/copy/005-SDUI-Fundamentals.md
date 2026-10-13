# Chapter 05 — Server-Driven UI Fundamentals

> **Volume:** II — DSL & AST
> **Phase:** 1 (Foundation)
> **Audience:** All Engineers
> **Prerequisites:** Chapters 01–04

---

## Table of Contents

- [5.1 What Is Server-Driven UI (SDUI)?](#51-what-is-server-driven-ui-sdui)
- [5.2 SDUI Spectrum: Thin Templates to Full AST](#52-sdui-spectrum-thin-templates-to-full-ast)
- [5.3 Why AwoERP Chose Full AST Over Thin Templates](#53-why-awoerp-chose-full-ast-over-thin-templates)
- [5.4 The Role of JSON in SDUI](#54-the-role-of-json-in-sdui)
- [5.5 SDUI vs. Traditional Frontend Rendering](#55-sdui-vs-traditional-frontend-rendering)
- [5.6 Client Contract Guarantees](#56-client-contract-guarantees)
- [5.7 Schema Versioning and Client Compatibility](#57-schema-versioning-and-client-compatibility)
- [5.8 SDUI Security Boundaries](#58-sdui-security-boundaries)
- [5.9 SDUI Debugging Mental Model](#59-sdui-debugging-mental-model)

---

## 5.1 What Is Server-Driven UI (SDUI)?

Server-Driven UI is an architectural pattern in which the **server controls the structure, content, and behavior of the user interface**, rather than those decisions being encoded statically into client application code.

In a conventional client application — whether web, iOS, or Android — the application code contains the definitions of screens: which components appear, how they are arranged, what data they display, and what happens when the user interacts with them. The server provides data; the client interprets it and decides what to show. Screen definitions live in the client codebase. Changing a screen requires changing client code, rebuilding the application, and releasing it to users.

In a Server-Driven UI system, the server provides not just data but **the specification of the interface itself**. The client receives a structured description — typically JSON — that tells it what to render and how to behave. The client's code knows how to render components and handle interactions, but it does not know in advance which components will appear on any given screen, in what arrangement, with what properties. All of that is determined at runtime by what the server sends.

This is the core transfer of authority: from client-encoded screen definitions to server-delivered interface specifications.

### The Canonical SDUI Exchange

Every SDUI interaction follows the same fundamental pattern:

```
Client                          Server
  │                               │
  │── GET /ui/surfaces/foo ───────►│
  │   (with context: user,         │
  │    tenant, capabilities)       │  1. Resolve context
  │                                │  2. Build UI spec
  │                                │  3. Apply permissions
  │                                │  4. Serialize
  │◄─ 200 { "root": { ... } } ────│
  │                                │
  │  [Render from spec]            │
  │  [User interacts]              │
  │                                │
  │── POST /api/actions/submit ───►│
  │   (form data + action ID)      │  5. Execute business logic
  │◄─ 200 { result } ─────────────│
  │                                │
  │  [Render result / navigate]    │
```

Steps 1–4 happen every time the user navigates to a surface. The client never assumes it knows what a surface looks like — it always asks. Steps 5 onward are standard API interactions; the action endpoint is defined in the UI spec but executed by the business service.

---

## 5.2 SDUI Spectrum: Thin Templates to Full AST

SDUI is not a single technique — it is a spectrum of approaches that vary in how much UI authority the server exercises. Understanding where AwoERP sits on this spectrum, and why, requires a brief tour of the options.

### Level 1 — Feature Flags and Configuration

The most minimal form of SDUI: the server tells the client which features to show or hide via boolean flags. The client still has all screen definitions hard-coded; it just knows which to activate.

```json
{ "show_advanced_analytics": true, "enable_bulk_import": false }
```

**Authority transferred:** Visibility of predefined features.
**Limitations:** The client still hard-codes all component arrangements. New screens require client releases.

### Level 2 — Data-Driven Content

The server provides structured data that the client uses to populate predefined screen templates. A navigation menu defined by server data is a common example: the client knows how to render a menu, the server tells it what items appear.

```json
{
  "nav_items": [
    { "label": "Dashboard", "icon": "home", "route": "/dashboard" },
    { "label": "Purchase Orders", "icon": "cart", "route": "/procurement/po" }
  ]
}
```

**Authority transferred:** Content within predefined structures.
**Limitations:** The structure itself (how many levels, what layout, what interaction model) is still client-defined.

### Level 3 — Component-Level Specification

The server specifies which named components should appear and provides their properties. The client has a registry of components and renders whichever ones the server requests, with the provided props.

```json
{
  "components": [
    { "type": "KPICard", "props": { "label": "Open POs", "value": 42 } },
    { "type": "RecentActivityFeed", "props": { "limit": 10 } }
  ]
}
```

**Authority transferred:** Component selection and configuration within a surface.
**Limitations:** Layout, nesting, conditional logic, actions, and event handling are still client-defined or must be handled by special-purpose props.

### Level 4 — Full AST (AwoERP's Approach)

The server specifies the complete interface as a typed tree: every component, its full property set, its children, its layout, its data bindings, its event handlers, its actions, and its conditional rendering rules. The client is a pure rendering engine — it materializes whatever tree it receives.

```json
{
  "root": {
    "type": "layout.grid",
    "props": { "columns": 2, "gap": "md" },
    "children": [
      {
        "type": "widget.kpi_card",
        "props": { "label": "Open POs", "value_source": "datasource:open_po_count" },
        "events": { "on_click": [{ "type": "action.navigate", "target": "procurement.po.list" }] }
      },
      {
        "type": "widget.activity_feed",
        "props": { "limit": 10, "filter": "procurement" }
      }
    ]
  }
}
```

**Authority transferred:** Complete UI structure, behavior, data binding, and interactivity.
**Limitations:** The client must be pre-built with implementations of all component types it may encounter. Entirely novel interaction models require client updates.

---

## 5.3 Why AwoERP Chose Full AST Over Thin Templates

The choice of Full AST (Level 4) over simpler approaches reflects the specific demands of enterprise ERP software. Each simpler level fails to meet at least one critical ERP requirement.

**ERP requirement: Field-level authorization.** A purchase order form has 30 fields. Different user roles can view and edit different subsets of those fields. The specific set of visible/editable fields for a given user may be determined by a combination of their role, the document's current workflow state, and fine-grained permission rules on specific data objects.

Levels 1–3 cannot express this. At Level 3, the server can tell the client which component types to render — but it cannot fully specify which fields within a form component are present, in what arrangement, with what validation rules, for this specific user. The form component itself would need to contain all the logic for determining field sets — reproducing the authorization logic in client code.

At Level 4, the form is fully specified in the server payload. The authorization system runs during compilation. The payload delivered to the client contains exactly the fields that user is authorized to see, arranged exactly as their role configuration specifies, with exactly the validation rules that apply. The form component in the client is a renderer; it renders what it receives.

**ERP requirement: Tenant customization without client releases.** ACME Corp has renamed "Vendor" to "Supplier" throughout their instance. They have added a custom field for their internal cost allocation code to the purchase order form. They have removed the "Preferred Vendor" toggle because their procurement policy prohibits it.

At Levels 1–3, these customizations require either client code changes (for structural modifications) or elaborate configuration systems that must anticipate every possible customization. At Level 4, customization is expressed as modifications to the AST during the tenant override transformation pass. Any structurally valid modification — adding fields, removing fields, changing labels, reordering sections — is expressible without client code.

**ERP requirement: Workflow-driven UI.** A purchase order approval workflow has five stages. At each stage, different actions are available, different fields become editable or read-only, and different status information is displayed. The workflow may advance asynchronously, driven by approver actions in other sessions.

At Levels 1–3, the client must know about all five workflow stages and implement the UI state for each. When a new workflow stage is added (a compliance review step added after the legal team's request), the client must be updated. At Level 4, each workflow stage produces a different compiled UI definition. Adding a new stage is a backend change; the client renders whatever definition it receives.

---

## 5.4 The Role of JSON in SDUI

JSON is the wire format of the SDUI exchange. It serves as the language in which the backend communicates UI intent to the client. Understanding what JSON is and is not in this context prevents several common misunderstandings.

### JSON Is a Serialization of the AST, Not the AST Itself

The backend works with a typed Go AST — a tree of strongly-typed Go structs with compile-time type safety. JSON is what that AST becomes when serialized for transport. A rendering engine works with a deserialized, validated representation of the received JSON — not with the raw JSON string.

Engineers working on business services write Go code using the AST builder API. They do not write JSON. Engineers working on rendering engines write component implementations in TypeScript, Swift, or Kotlin. They do not parse JSON directly — they implement components that receive typed props objects that the rendering engine has already deserialized from JSON.

### JSON Is Validated in Both Directions

The JSON payload is validated by the backend compilation service before it is sent (against the component schemas) and by the rendering engine upon receipt (against the client-side copy of the schemas). This bidirectional validation catches schema drift — the situation where the backend and client have diverging understandings of a component's property structure.

The client-side schema copy is bundled with the rendering engine and updated as part of client releases. When a rendering engine receives a definition referencing a schema version newer than its bundled schemas, it logs a warning and applies graceful fallbacks for any properties it does not recognize.

### JSON Is the Caching Unit

The compiled JSON definition — after compression — is the unit stored in Redis and the unit sent with ETag headers for client-side HTTP caching. The backend does not cache the AST (which is a large in-memory object graph); it caches the serialized JSON string. Redis stores strings; the compilation service stores and retrieves the gzip-compressed JSON bytes directly.

### JSON Is Not Executable

The JSON payload does not contain executable code. Action definitions are typed references (e.g., `"type": "action.http_request"`) with typed parameters — not JavaScript functions, not Go closures, not SQL strings. The rendering engine executes actions by dispatching to its own, pre-compiled action handler for the specified action type. There is no eval, no code injection, no script execution from the payload.

Binding expressions — the `{ "$expr": "sum(line_items.amount)" }` style values in component props — are evaluated by a sandboxed, constrained expression evaluator within the rendering engine. The expression language is intentionally limited: it can reference data source values, perform arithmetic, apply string transformations, and evaluate conditions. It cannot call arbitrary APIs, access the file system, or execute platform-level code.

---

## 5.5 SDUI vs. Traditional Frontend Rendering

### 5.5.1 Comparison: React/Vue Component Trees vs. SDUI JSON

In a traditional React application, a purchase order form is a React component:

```jsx
// Traditional React — screen definition in client code
function PurchaseOrderCreateForm() {
  const { user } = useAuth();
  const { tenantConfig } = useTenantConfig();

  return (
    <Form onSubmit={handleSubmit}>
      <FormSection title={tenantConfig.vendorLabel || "Vendor Details"}>
        {user.permissions.includes("procurement:vendor:select") && (
          <VendorSelector
            name="vendor_id"
            label={tenantConfig.vendorLabel || "Vendor"}
            required
          />
        )}
        <DateField
          name="required_date"
          label="Required Date"
          minDate={new Date()}
          required
        />
        {/* ... 25 more fields with their own permission checks ... */}
      </FormSection>

      {user.permissions.includes("procurement:line_items:write") && (
        <LineItemSection name="line_items" />
      )}

      <FormActions>
        {user.permissions.includes("procurement:po:create") && (
          <SubmitButton label="Submit Purchase Order" />
        )}
        <CancelButton target="/procurement/po" />
      </FormActions>
    </Form>
  );
}
```

This code:
- Re-implements authorization logic that already exists in the backend
- Hard-codes tenant configuration fallbacks that may diverge from the backend's tenant config
- Must be updated and released when new fields, new permission rules, or new tenant customizations are required
- Is duplicated (with variations) across web, iOS, and Android

In the AwoERP SDUI model, none of this code exists in the client. The client contains a `FormContainer` renderer, a `VendorSelectorRenderer`, a `DateFieldRenderer`, and a `LineItemSectionRenderer`. It does not contain the purchase order form's specific arrangement of those renderers. That arrangement is specified by the backend and delivered at runtime.

### 5.5.2 Comparison: Native Mobile vs. SDUI JSON

A traditional iOS implementation of the same screen is a SwiftUI view:

```swift
// Traditional SwiftUI — screen definition in client code
struct PurchaseOrderCreateView: View {
    @EnvironmentObject var auth: AuthContext
    @EnvironmentObject var tenantConfig: TenantConfiguration

    var body: some View {
        Form {
            Section(tenantConfig.vendorSectionTitle) {
                if auth.hasPermission("procurement:vendor:select") {
                    VendorSelectorView(required: true)
                }
                DatePickerField(
                    label: "Required Date",
                    minimumDate: Date()
                )
                // ... more fields
            }
            // ... more sections
        }
        .navigationTitle("Create Purchase Order")
    }
}
```

The same problems apply. The iOS app must be released to App Store review before any structural change reaches iOS users. If the Android team makes a change to the form (a new field added, a section reordered), the iOS team must make a corresponding change — discovered through manual coordination, not enforced by the platform.

In AwoERP's SDUI model, the iOS app contains a rendering engine. The same JSON definition that drives the web form drives the iOS form. A new field added in the backend's AST builder appears on web, iOS, and Android simultaneously — without any client code changes.

---

## 5.6 Client Contract Guarantees

The SDUI contract between backend and client is asymmetric: the backend makes strong guarantees about what it will deliver; the client makes strong guarantees about what it will render. Neither side can fulfill its obligations without understanding the other's guarantees.

### 5.6.1 What the Client Must Render

The client rendering engine makes the following commitments to the backend:

**Structural fidelity.** The client renders the component hierarchy exactly as specified. It does not reorder siblings, collapse sections, or merge nodes based on its own judgment. If the backend specifies a three-section form, the client renders three sections in the specified order.

**Property completeness.** The client applies all specified props to the component. It does not silently ignore props it does not understand — it logs them as warnings and applies as many as it can. A warning in the telemetry log for an unrecognized prop is a signal that the rendering engine needs to be updated, not that the prop is optional.

**Action execution.** When the user triggers an action (by tapping a button, submitting a form), the client dispatches the action exactly as specified. It does not substitute a different action, skip an action because it seems redundant, or modify the action payload based on local state not referenced in the action definition.

**Event routing.** The client routes component events to the handlers specified in the definition. It does not suppress events, reroute them, or add default handlers that override the specified handlers.

### 5.6.2 What the Client May Enhance

Within the constraints of the specification, the rendering engine has discretion to apply platform-appropriate enhancements:

**Native input affordances.** A `field.date` with `input_hint: "native_date_picker"` may be rendered using the platform's native date picker control rather than a custom calendar widget. The semantic contract (a date field, required, with a minimum date) is preserved; the visual affordance is platform-native.

**Layout adaptation.** The rendering engine may adapt the specified layout to the actual screen dimensions. A two-column grid may be collapsed to a single column on a narrow viewport, with columns rendered sequentially.

**Performance optimizations.** The rendering engine may apply virtualization, memoization, lazy loading, and other performance techniques that do not alter the visible content or behavior of the rendered surface.

**Accessibility enhancements.** The rendering engine may add platform-appropriate accessibility attributes (ARIA labels on web, accessibility identifiers on iOS, content descriptions on Android) based on the semantic information in the component props.

### 5.6.3 What the Client Must Not Override

**Authorization decisions.** If the backend has omitted an element from the definition (because the user is not authorized), the client must not add it back. The client does not have a local copy of the authorization rules and must not attempt to reconstruct or second-guess the backend's authorization decisions.

**Validation rules.** The client may execute validation rules locally for UX purposes, but it must not relax them. A field marked `required: true` in the definition is required — the client may not submit the form with that field empty regardless of any local state or user preference.

**Action endpoints.** The client executes actions at the endpoints specified in the definition. It must not redirect an action to a different endpoint, merge multiple actions into one, or split one action into several.

---

## 5.7 Schema Versioning and Client Compatibility

The compiled UI definition includes a `schema_version` field that declares which version of the component schema the definition was compiled against. Rendering engines declare their supported schema versions in the request's `X-Client-Capabilities` header.

### Version Negotiation

```
Request:
  X-Client-Capabilities: schema_version=2.1.0,charts,native-date-picker

Response:
  X-Schema-Version: 2.1.0
  X-Compiled-For-Capabilities: schema_version=2.1.0,charts,native-date-picker
```

When the backend receives a request from a client that supports an older schema version, it compiles the definition using the older schema — omitting fields added in newer versions, substituting components added in newer versions with backward-compatible equivalents.

When a client receives a definition with a schema version newer than its bundled schemas, it processes the definition as best it can — rendering components it recognizes, applying fallbacks for those it does not — and logs a warning for the telemetry pipeline. This graceful handling ensures that a backend schema update does not immediately break older clients; it only means those clients cannot render the newest components until they update.

### Compatibility Matrix

The platform maintains a compatibility matrix that documents which schema versions each released rendering engine version supports. This matrix is used to:
- Determine when a schema version can be retired (no active client versions still depend on it)
- Alert the team when a rendering engine version is approaching end-of-life for schema support
- Plan coordinated rollouts of breaking schema changes

---

## 5.8 SDUI Security Boundaries

The SDUI architecture creates specific security boundaries that differ from traditional frontend security considerations.

### The Backend Is the Enforcement Point

Because the UI definition is generated by the backend and already reflects the user's authorization context, the temptation exists to treat the frontend as a "trusted" client. This temptation must be resisted.

The UI definition tells the client what the user is authorized to do from the UI's perspective. It does not authorize the corresponding API calls. Every action endpoint enforces its own authorization independently. A malicious client that constructs a direct HTTP request to an action endpoint — bypassing the UI entirely — will be rejected by the backend's API-level authorization.

**The security guarantee of SDUI is UI consistency, not additional API security.** The backend authorization system secures the API. The SDUI compilation pipeline secures the UI definition. Both layers of enforcement are necessary; neither replaces the other.

### No Secrets in the UI Definition

The compiled UI definition must never contain information that the user is not authorized to possess, even in a non-rendered form. A field that is hidden because of permissions must be absent from the payload — not present with a `"visible": false` prop. The hidden field might contain data (a salary, a confidential note, an internal pricing adjustment) that an attacker could extract from the payload even if it is not visually rendered.

Specifically:
- No field values for fields that failed the authorization check
- No action endpoint URLs for actions the user cannot execute (endpoint URLs may reveal internal service architecture)
- No permission rule IDs or role names that reveal the authorization model structure
- No tenant configuration details beyond what the current user's context requires

### The Expression Language Is Sandboxed

Binding expressions in the UI definition are evaluated in a sandboxed environment. The expression evaluator:
- Accepts only a defined set of functions and operators
- Can only reference data sources declared in the definition and state variables in the current scope
- Cannot access the file system, network, or platform APIs
- Cannot reference or modify variables outside the current component's scope
- Has a maximum evaluation depth to prevent recursive expression attacks
- Times out after a maximum evaluation duration (default: 100ms)

Any expression that exceeds these constraints is rejected during compilation (syntactic constraints checked by the expression parser) or at evaluation time (runtime constraints enforced by the evaluator).

---

## 5.9 SDUI Debugging Mental Model

Debugging issues in a SDUI system requires a different mental model than debugging traditional frontends. The key shift: **when something looks wrong in the UI, the first question is not "what is the frontend doing wrong?" — it is "what did the backend send?"**

### The Debugging Decision Tree

```
User reports: "I can't see the Approve button."
      │
      ▼
Step 1: Fetch the compiled UI definition for that user/surface
      │
      ├─ Approve button IS in the definition?
      │         │
      │         ▼
      │   Step 2: Is the rendering engine rendering it?
      │         ├─ Yes → Check local state / action binding
      │         └─ No  → Rendering engine bug
      │
      └─ Approve button is NOT in the definition?
                │
                ▼
          Step 3: Check the compilation audit log
                │
                ├─ Permission pruning removed it?
                │         │
                │         ▼
                │   Check OpenFGA: does the user have the permission?
                │         ├─ Yes → Authorization resolver bug
                │         └─ No  → Expected behavior (user lacks permission)
                │
                └─ Business service never emitted it?
                          │
                          ▼
                    Check business service logic
                    (workflow state? flag condition?)
```

### Developer Tooling Support

The compilation service exposes a debug endpoint (available in non-production environments and to users with the `platform:debug` permission in production) that returns the full compilation trace alongside the compiled definition:

```json
{
  "definition": { "...": "..." },
  "compilation_trace": {
    "surface": "procurement.purchase-order.detail",
    "context": { "user_id": "...", "tenant_id": "...", "flags": {} },
    "stages": [
      {
        "stage": "permission_pruning",
        "duration_ms": 8,
        "pruned_nodes": [
          {
            "node_id": "action-approve",
            "reason": "permission_denied",
            "permission": "procurement:po:approve",
            "check_result": "denied"
          }
        ]
      },
      {
        "stage": "flag_resolution",
        "duration_ms": 2,
        "resolved_flags": { "procurement.enhanced-vendor-search": true }
      }
    ]
  }
}
```

This trace eliminates the guesswork from authorization-related UI debugging. The audit record shows exactly why each node was or was not included.

### Client-Side Debug Mode

Rendering engines support a debug overlay mode (activated via a developer settings screen or a URL parameter in development) that displays:
- The component type name for each rendered element
- The props received from the definition
- Any fallbacks that were applied for unrecognized types
- The data source binding status (loading / loaded / error)
- Action execution logs

This overlay allows frontend engineers to verify that the rendering engine is correctly materializing the received definition, independent of any questions about what the backend sent.

---

*End of Chapter 05*

**Previous:** [Chapter 04 — System Overview](../vol-01-vision/04-system-overview.md)
**Next:** [Chapter 06 — UI DSL Architecture](./06-ui-dsl-architecture.md)
