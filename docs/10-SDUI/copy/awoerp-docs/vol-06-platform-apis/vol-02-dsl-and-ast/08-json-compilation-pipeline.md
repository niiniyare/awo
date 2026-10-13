# Chapter 08 — JSON Compilation Pipeline

> **Volume:** II — DSL & AST
> **Phase:** 1 (Foundation)
> **Audience:** Platform Engineers, Backend Engineers
> **Prerequisites:** Chapters 01–07

---

## Table of Contents

- [8.1 Pipeline Overview and Stages](#81-pipeline-overview-and-stages)
- [8.2 Stage 1 — Context Resolution](#82-stage-1--context-resolution)
- [8.3 Stage 2 — AST Construction](#83-stage-2--ast-construction)
- [8.4 Stage 3 — AST Transformation](#84-stage-3--ast-transformation)
- [8.5 Stage 4 — Validation](#85-stage-4--validation)
- [8.6 Stage 5 — Serialization](#86-stage-5--serialization)
- [8.7 Stage 6 — Transport](#87-stage-6--transport)
- [8.8 Pipeline Error Handling](#88-pipeline-error-handling)
- [8.9 Pipeline Observability](#89-pipeline-observability)
- [8.10 Pipeline Extension Points](#810-pipeline-extension-points)

---

## 8.1 Pipeline Overview and Stages

The JSON Compilation Pipeline is the ordered sequence of operations that transforms a client's request for a UI surface into a validated, serialized JSON definition ready for delivery. It is the operational heart of the SDUI platform — every screen the user sees passes through this pipeline.

```
                    Client Request
                         │
                         ▼
            ┌────────────────────────┐
            │  Stage 1               │
            │  CONTEXT RESOLUTION    │  ~2–5ms
            │                        │
            │  Tenant · User · Flags │
            │  Locale · Capabilities │
            │  Permissions (batch)   │
            └────────────┬───────────┘
                         │ CompilationContext
                         ▼
            ┌────────────────────────┐
            │  Stage 2               │
            │  AST CONSTRUCTION      │  ~5–20ms
            │                        │
            │  Business service      │
            │  emits typed AST       │
            │  Template expansion    │
            └────────────┬───────────┘
                         │ *ast.Surface (raw)
                         ▼
            ┌────────────────────────┐
            │  Stage 3               │
            │  AST TRANSFORMATION    │  ~3–10ms
            │                        │
            │  Permission pruning    │
            │  Flag resolution       │
            │  Tenant overrides      │
            │  Localization          │
            │  Default injection     │
            └────────────┬───────────┘
                         │ *ast.Surface (transformed)
                         ▼
            ┌────────────────────────┐
            │  Stage 4               │
            │  VALIDATION            │  ~1–3ms
            │                        │
            │  Schema validation     │
            │  Reference integrity   │
            │  Security assertions   │
            └────────────┬───────────┘
                         │ *ast.Surface (validated)
                         ▼
            ┌────────────────────────┐
            │  Stage 5               │
            │  SERIALIZATION         │  ~1–3ms
            │                        │
            │  JSON marshaling       │
            │  Compression           │
            │  Canonical form / ETag │
            │  Cache write           │
            └────────────┬───────────┘
                         │ []byte (compressed JSON)
                         ▼
            ┌────────────────────────┐
            │  Stage 6               │
            │  TRANSPORT             │  ~1–2ms
            │                        │
            │  HTTP response wrap    │
            │  Cache headers         │
            │  Audit log write       │
            └────────────┬───────────┘
                         │
                    Client Response
```

**Total pipeline latency (cold, cache miss):** ~15–45ms (p50), ~80ms (p95), <150ms (p99)
**Total pipeline latency (warm, cache hit):** ~2–5ms (p50), ~10ms (p95)

The pipeline is implemented in the `uicompile` Go package within the UI Compilation Service. Each stage is an independently tested function. The pipeline coordinator (`Pipeline.Execute()`) calls stages in order, passing the output of each stage as the input to the next, and handling errors consistently.

---

## 8.2 Stage 1 — Context Resolution

Context resolution is the first stage. It collects all information needed to personalize the compilation for the requesting user and tenant. The output is a `CompilationContext` struct that is threaded through all subsequent stages.

Context resolution is also the stage where the **cache lookup** occurs. If a cached definition exists for the resolved context, the pipeline short-circuits and returns the cached definition immediately — bypassing all subsequent stages.

### 8.2.1 Tenant Resolution

The tenant is identified from the incoming request using one of three mechanisms (in priority order):

1. **`X-Tenant-ID` request header** (set by the API gateway from the authenticated JWT's `tenant_id` claim)
2. **Subdomain extraction** (for web clients accessing `acme.awoerp.com`, the tenant slug is `acme`)
3. **JWT claim** (the `tenant_id` field in the validated JWT)

Once the tenant ID is resolved, the tenant's configuration record is loaded from cache (Redis) or the database (PostgreSQL on cache miss). The tenant configuration includes:
- Tenant's active feature set (which ERP modules are enabled)
- Tenant's theme configuration (brand colors, logo)
- Tenant's label overrides (terminology customizations)
- Tenant's form customization rules (field additions, removals, reordering)
- Tenant's feature flag overrides (flags enabled or disabled specifically for this tenant)

```go
type TenantConfig struct {
    TenantID        string
    DisplayName     string
    ActiveModules   []ModuleID
    ThemeTokens     map[string]string
    LabelOverrides  map[i18n.Key]string
    FormOverrides   []OverrideOperation
    FlagOverrides   map[string]FlagValue
    CustomComponents []ComponentRegistration
    Locale          LanguageTag
}
```

### 8.2.2 User Identity & Role Resolution

The user identity is extracted from the validated JWT. The compilation context requires:
- `UserID`: The user's stable unique identifier
- `Roles`: The user's current role assignments within the tenant
- `UserAttributes`: Attributes used in feature flag targeting (department, seniority, location)
- `SessionContext`: The current session metadata (device type, IP region, session age)

Role assignments are loaded from the authorization system (OpenFGA). Roles determine which permission checks will pass in the permission pruning pass, and they are used as a component of the cache key.

### 8.2.3 Feature Flag Resolution

All feature flags relevant to the requested surface are evaluated upfront, before AST construction, so that business services can query flag values during AST building.

```go
flags, err := pipeline.flagResolver.ResolveAll(ctx, FlagContext{
    TenantID:        tenantConfig.TenantID,
    UserID:          userIdentity.UserID,
    UserAttributes:  userIdentity.UserAttributes,
    TenantOverrides: tenantConfig.FlagOverrides,
    Environment:     pipeline.environment,
})
```

The resolved flag map is immutable for the duration of the compilation request. Flag values do not change mid-pipeline.

### 8.2.4 Locale Resolution

The compilation locale is resolved from the following sources, in priority order:

1. User's saved locale preference (from the user preferences store)
2. `Accept-Language` request header (parsed with quality weights)
3. Tenant's configured default locale
4. Platform default locale (`en-US`)

The resolved locale is used both for localization injection (resolving translation keys to strings) and for number/date format configuration (which locale-sensitive formatting the rendering engine should apply).

### 8.2.5 Device / Client Context Resolution

Client context informs compilation decisions about which component variants to use.

```go
type ClientContext struct {
    Platform        Platform        // ios, android, web
    ClientVersion   semver.Version  // "3.2.1"
    Capabilities    CapabilitySet   // {"charts", "native-date-picker", "offline-queue"}
    ScreenCategory  ScreenCategory  // mobile, tablet, desktop (inferred from platform + UA)
    MaxPayloadBytes int             // client-declared payload budget; 0 = no limit
}
```

The `Capabilities` set is used in Stage 3 (transformation) to select appropriate component variants. For example, a client that declares `native-date-picker` capability receives `DateField` nodes with `input_hint: "native_date_picker"`. A client without this capability receives standard date fields.

### 8.2.6 Cache Lookup

Once the full `CompilationContext` is assembled, the cache key is computed and a cache lookup is performed:

```go
cacheKey := computeCacheKey(CacheKeyInput{
    SurfaceID:     request.SurfaceID,
    TenantID:      tenantConfig.TenantID,
    RoleSetHash:   hashRoleSet(userIdentity.Roles),
    FlagSetHash:   hashFlagMap(flags),
    Locale:        locale.String(),
    Platform:      clientCtx.Platform,
    ClientVersion: clientCtx.ClientVersion.String(),
    CapabilityHash: hashCapabilitySet(clientCtx.Capabilities),
})

if cached, err := pipeline.cache.Get(ctx, cacheKey); err == nil {
    // Cache hit — return immediately
    return &CompilationResult{
        Definition: cached.Definition,
        ETag:       cached.ETag,
        CacheHit:   true,
    }, nil
}
```

Cache misses proceed to Stage 2. Cache hits emit a `cache_hit` metric and return in ~2ms.

---

## 8.3 Stage 2 — AST Construction

AST construction is where the business service author's intentions become a concrete UI tree.

### 8.3.1 Business Service Emits Raw AST

The compilation service dispatches the construction request to the appropriate business service handler, identified by the surface ID:

```go
// Surface ID routing table — populated by business service registration
type SurfaceRegistry map[SurfaceID]SurfaceHandler

type SurfaceHandler interface {
    BuildAST(ctx context.Context, compCtx *CompilationContext) (*ast.Surface, error)
}

// Dispatch
handler, ok := pipeline.surfaceRegistry[request.SurfaceID]
if !ok {
    return nil, ErrSurfaceNotFound{SurfaceID: request.SurfaceID}
}

rawAST, err := handler.BuildAST(ctx, compilationContext)
if err != nil {
    return nil, fmt.Errorf("AST construction failed for surface %s: %w", request.SurfaceID, err)
}
```

Business service handlers are registered at application startup. Each handler is a method on a business service struct — it has access to the service's domain repositories, configuration, and business rules.

The handler receives the `CompilationContext` as an argument. This gives the handler access to:
- `compilationContext.Flags` — to query flag values and conditionally emit different AST nodes
- `compilationContext.TenantConfig` — to access tenant-specific configuration that should influence the base AST (distinct from the overlay-based overrides applied in Stage 3)
- `compilationContext.ClientContext` — to adapt the AST for the specific client platform

> 📘 **Note:** The handler should not apply permission logic itself. Permissions are applied by the transformation pipeline in Stage 3. The handler emits the complete AST — all fields, all actions — and trusts the pipeline to prune unauthorized elements. This separation ensures that permission logic is centralized, consistent, and audited.

### 8.3.2 Slot Composition

After the primary handler returns the raw AST, the pipeline executes slot composition. Some surfaces accept pluggable slot content from other services — for example, a purchase order detail view may have a "Related Documents" slot that can be populated by the document management service.

```go
// Slot composition — other services contribute content to named slots
for _, contributor := range pipeline.slotContributors[request.SurfaceID] {
    slotContent, err := contributor.BuildSlotContent(ctx, compilationContext, rawAST)
    if err != nil {
        // Non-fatal: log the error, use the fallback slot content
        pipeline.logger.Warn("slot contributor failed", ...)
        continue
    }
    rawAST.SetSlot(contributor.SlotName(), slotContent)
}
```

Slot composition allows features to contribute to surfaces they do not own, without the owning surface needing to know about all possible contributors at compile time.

### 8.3.3 Template Instantiation

The AST builder library supports template references — `ast.NewTemplateRef("erp.approval_action_bar")` — that defer the expansion of a reusable component pattern. Template instantiation is the pass that replaces these references with their full AST subtrees.

```go
func instantiateTemplates(surface *ast.Surface, registry *ComponentRegistry) error {
    return ast.WalkNodes(surface, func(node *ast.Node) error {
        if node.Type != ast.TemplateRefType {
            return nil
        }
        template, err := registry.GetTemplate(node.TemplateID)
        if err != nil {
            return fmt.Errorf("unknown template %s: %w", node.TemplateID, err)
        }
        expanded := template.Instantiate(node.Props)
        *node = *expanded // replace the ref node with the expanded subtree
        return nil
    })
}
```

Templates are themselves versioned ASTs registered in the component registry. They can reference the props passed to them using a special `$prop:` binding syntax within the template definition.

---

## 8.4 Stage 3 — AST Transformation

Stage 3 applies the ordered sequence of transformation passes described in Chapter 07, Section 7.6. The passes execute in a fixed order:

```
1. Permission Pruning
2. Feature Flag Resolution
3. Tenant Override Application
4. Localization Injection
5. Default Value Injection
6. Client Capability Adaptation
```

The order is significant. Permission pruning runs before tenant overrides to ensure that a tenant override cannot introduce an element the user is not authorized to see. Flag resolution runs before tenant overrides to ensure that flag-conditional nodes are resolved before override operations target them by ID.

### Client Capability Adaptation (Stage 3, Pass 6)

This pass is unique to Stage 3 — it does not correspond to a transformation pass described in Chapter 07 because it is a transport-level concern. It adapts the transformed AST to the specific capabilities of the requesting client.

For example, if the client declares support for `native-date-picker`:

```go
func adaptForCapabilities(surface *ast.Surface, caps CapabilitySet) error {
    return ast.WalkNodes(surface, func(node *ast.Node) error {
        if node.Type == "field.date" && caps.Has(CapNativeDatePicker) {
            node.Props["input_hint"] = "native_date_picker"
        }
        if node.Type == "widget.chart" && !caps.Has(CapCharts) {
            // Replace chart with a tabular fallback
            *node = *buildChartFallbackTable(node)
        }
        return nil
    })
}
```

This pass ensures that the same logical surface definition is adapted to each client's actual rendering capabilities — without requiring the business service to be aware of client capability variations.

---

## 8.5 Stage 4 — Validation

The validation stage re-runs a subset of the AST validations after transformation to catch any issues introduced by the transformation passes.

### 8.5.1 Schema Validation (Post-Transformation)

The full structural and type validation runs against the transformed AST. This catches:
- Tenant override operations that introduced malformed props
- Template instantiation that produced schema-invalid subtrees
- Localization injection that produced strings exceeding field maximum lengths

### 8.5.2 Business Rule Validation

A set of business-level validation rules that are not expressible in JSON Schema:

- **Required field completeness:** If a field is `required: true` and has no `default_value` and is not pre-populated by a data binding, a warning is emitted (the form can be rendered but may not be submittable until the user provides a value — confirm this is intentional)
- **Action endpoint reachability:** Actions that reference absolute endpoint URLs are checked against the allowed endpoint registry (prevents injection of arbitrary endpoints via tenant overrides)
- **Data source loop detection:** Computed data sources that reference each other circularly are detected and rejected

### 8.5.3 Security Assertion Validation

Final security checks before serialization:
- No permission rule IDs or role names are present in any prop value (they should have been replaced by their resolved boolean outcomes during permission pruning)
- No internal endpoint URLs that should not be exposed to clients appear in action definitions
- No PII fields (identified by their schema annotations) appear in the definition without appropriate masking

---

## 8.6 Stage 5 — Serialization

### 8.6.1 JSON Marshaling

The validated `*ast.Surface` is marshaled to JSON using a custom marshaler that:
- Uses `snake_case` field names (not the Go struct field names, which follow Go conventions)
- Omits fields with nil pointer values and zero-value primitives where the schema declares them optional
- Includes the schema version and compilation metadata in the root envelope
- Serializes binding expressions to their `{ "$expr": "..." }` form

```go
type SurfaceEnvelope struct {
    SchemaVersion string    `json:"schema_version"`
    SurfaceID     string    `json:"surface_id"`
    CompiledAt    time.Time `json:"compiled_at"`
    Root          *ast.Node `json:"root"`
}
```

### 8.6.2 Compression

The serialized JSON is compressed using gzip (default) or brotli (for clients that declare Brotli support via `Accept-Encoding: br`).

Compression benchmarks for typical AwoERP surfaces:

| Surface Type | Uncompressed | Gzip | Brotli | Gzip Ratio |
|--------------|-------------|------|--------|------------|
| Simple form (10 fields) | 8 KB | 2.1 KB | 1.7 KB | 74% |
| Complex form (40 fields) | 35 KB | 7.8 KB | 6.1 KB | 78% |
| Dashboard (8 widgets) | 22 KB | 5.4 KB | 4.2 KB | 75% |
| Data table (20 columns) | 18 KB | 4.1 KB | 3.2 KB | 77% |
| PO detail view (full) | 61 KB | 13.2 KB | 10.4 KB | 78% |

### 8.6.3 Payload Size Budgets

Payload size budgets are enforced at serialization time. If the compressed payload exceeds the budget, the compilation pipeline returns a `PAYLOAD_TOO_LARGE` error with the actual and maximum sizes.

| Client Type | Default Budget | Override Mechanism |
|-------------|---------------|-------------------|
| Web | 200 KB (uncompressed) | `X-Max-Payload-KB` request header |
| Mobile | 100 KB (uncompressed) | `X-Max-Payload-KB` request header |
| Low-capability (terminal) | 30 KB (uncompressed) | `X-Max-Payload-KB` request header |

Surfaces that approach the budget trigger a lint warning in the compilation log, advising the surface author to consider splitting the surface into sub-surfaces with lazy-loaded sections.

### 8.6.4 Field Stripping / Projection

Clients can request that specific fields be excluded from the serialized output — for example, a client that handles localization client-side may request that translation strings be excluded from string props (receiving translation keys instead). The `X-Projection` request header specifies a comma-separated list of projection rules.

This is an advanced feature and is disabled by default. Most clients receive the full definition.

### 8.6.5 Cache Write

After serialization, the compressed definition is written to the Redis cache:

```go
cacheEntry := CacheEntry{
    Definition:  compressedJSON,
    ETag:        etag,
    CompiledAt:  time.Now(),
    TTL:         pipeline.cacheConfig.DefaultTTL, // default: 5 minutes
}
pipeline.cache.Set(ctx, cacheKey, cacheEntry, cacheEntry.TTL)
```

The TTL is surface-specific. Surfaces that depend heavily on rapidly changing data (real-time dashboards) have shorter TTLs (30 seconds). Surfaces that are structurally stable (configuration screens, reference data forms) have longer TTLs (30 minutes).

---

## 8.7 Stage 6 — Transport

### 8.7.1 REST Response Wrapping

The compiled definition is wrapped in an HTTP response with appropriate headers:

```
HTTP/2 200 OK
Content-Type: application/json
Content-Encoding: gzip
ETag: "sha256:a3f9b2c4..."
Cache-Control: private, max-age=300
Vary: Accept-Encoding, Accept-Language, X-Client-Capabilities
X-Compilation-Time-Ms: 23
X-Cache: MISS
X-Schema-Version: 2.1.0
X-Surface-ID: procurement.purchase-order.create
```

The `Cache-Control: private` directive ensures that intermediate proxies do not cache the definition — definitions are personalized and must only be cached client-side or in the server-side Redis cache.

The `Vary` header lists all request headers that affect the response content. This is critical for correct caching behavior: two requests with the same URL but different `Accept-Language` values may receive different definitions.

### 8.7.2 gRPC Streaming of UI Definitions

For clients using the gRPC transport (primarily for real-time update scenarios), the `UIService` streams definition updates using server-side streaming:

```proto
service UIService {
  // Unary: initial definition fetch
  rpc GetSurface(GetSurfaceRequest) returns (SurfaceDefinition);

  // Server streaming: subscribe to real-time definition updates
  rpc StreamSurface(StreamSurfaceRequest) returns (stream SurfaceUpdate);
}

message SurfaceUpdate {
  oneof update_type {
    SurfaceDefinition full_definition = 1;   // full replacement
    SurfacePatch      patch           = 2;   // incremental patch
  }
  string etag = 3;
  int64  compiled_at_unix = 4;
}
```

The streaming endpoint keeps a connection open and pushes updates when the surface definition changes (driven by events from the event bus). The initial response is always a full definition; subsequent responses may be patches.

### 8.7.3 Cache Header Strategy

Clients use HTTP caching to avoid re-fetching unchanged definitions. The `ETag` header enables conditional requests:

```
// Subsequent request — client sends the previously received ETag
GET /api/ui/surfaces/procurement.purchase-order.create
If-None-Match: "sha256:a3f9b2c4..."

// If definition hasn't changed
HTTP/2 304 Not Modified
ETag: "sha256:a3f9b2c4..."
Cache-Control: private, max-age=300
```

A `304 Not Modified` response has an empty body — the client uses its locally cached definition. This is the most efficient possible response for surfaces that haven't changed.

---

## 8.8 Pipeline Error Handling

The compilation pipeline can encounter errors at any stage. The error handling strategy differs by error category:

### Recoverable Errors (Warnings)

Recoverable errors allow the pipeline to proceed with graceful degradation. They are collected and reported in the response headers (for development) and the audit log (always).

| Error | Recovery Strategy |
|-------|------------------|
| Unknown component type from business service | Replace with fallback node; emit warning |
| Missing translation key | Use fallback locale string or key itself; emit warning |
| Tenant override targets non-existent node ID | Skip the override operation; emit warning |
| Slot contributor fails | Use slot's fallback content; emit warning |
| Payload exceeds soft budget | Emit warning; continue (hard budget enforcement is an error) |

### Non-Recoverable Errors (Fatal)

Non-recoverable errors abort the pipeline. The client receives a structured error response.

| Error | HTTP Status | Client Behavior |
|-------|------------|-----------------|
| Surface ID not found | `404 Not Found` | Display "Page not found" |
| Business service AST construction failure | `503 Service Unavailable` | Display error with retry option |
| Schema validation failure (post-transformation) | `500 Internal Server Error` | Display error; trigger alert |
| Payload exceeds hard budget | `413 Payload Too Large` | Display error; trigger alert |
| Authorization resolver unavailable | `503 Service Unavailable` | Display error with retry option |

Fatal errors are always logged at `ERROR` level with the full error context (surface ID, tenant ID, stage name, error message, stack trace). They trigger an alert in the observability pipeline.

### Pipeline Error Response Format

```json
{
  "error": {
    "code": "COMPILATION_FAILED",
    "stage": "ast_construction",
    "message": "Business service returned an error for surface procurement.purchase-order.create",
    "request_id": "req-7f3a2b9c",
    "retry_after": 5,
    "support_reference": "err-2026-06-01-7f3a2b9c"
  }
}
```

---

## 8.9 Pipeline Observability

Every pipeline execution emits structured telemetry, regardless of whether it succeeds or fails.

### 8.9.1 Span Tracing per Stage

The pipeline creates a parent span for the overall compilation and a child span for each stage:

```
compilation [23ms]
├── context_resolution [4ms]
│   ├── tenant_config_load [1ms] (cache hit)
│   ├── flag_resolution [2ms]
│   └── auth_batch_load [1ms]
├── ast_construction [9ms]
│   ├── handler_dispatch [8ms]
│   └── template_instantiation [1ms]
├── ast_transformation [6ms]
│   ├── permission_pruning [4ms]
│   ├── flag_resolution [0ms]
│   ├── tenant_overrides [1ms]
│   └── localization [1ms]
├── validation [2ms]
└── serialization [2ms]
    ├── json_marshal [1ms]
    └── gzip_compress [1ms]
```

All spans include:
- `surface_id`: The surface being compiled
- `tenant_id`: The tenant context
- `cache_hit`: Whether the request was served from cache
- `error`: Error message if the stage failed

### 8.9.2 Stage Latency Metrics

The following metrics are emitted as histograms with `surface_id`, `tenant_id`, `platform`, and `cache_hit` labels:

| Metric | Description |
|--------|-------------|
| `ui_compilation_duration_ms` | End-to-end pipeline latency |
| `ui_compilation_stage_duration_ms` | Per-stage latency (with `stage` label) |
| `ui_compilation_payload_bytes` | Uncompressed payload size |
| `ui_compilation_compressed_bytes` | Compressed payload size |
| `ui_compilation_node_count` | Number of nodes in the compiled AST |
| `ui_compilation_pruned_nodes_total` | Number of nodes removed by permission pruning |

### 8.9.3 Payload Size Metrics

Payload size metrics, tracked per surface and over time, are the primary signal for detecting definition bloat:

```promql
# Alert: surface payload exceeds 50KB uncompressed (p95)
histogram_quantile(0.95,
  rate(ui_compilation_payload_bytes_bucket[5m])
) > 51200
```

---

## 8.10 Pipeline Extension Points

The pipeline provides formal extension points for platform contributors and plugin authors.

### Pre-Construction Hook

Called after context resolution, before AST construction. Allows extensions to inject additional context or modify the compilation context.

```go
type PreConstructionHook interface {
    Execute(ctx context.Context, compCtx *CompilationContext) (*CompilationContext, error)
}
```

Use cases: injecting additional user attributes from an external identity provider; loading extension-specific tenant configuration.

### Post-Construction Hook

Called after AST construction and template instantiation, before the transformation passes. Allows extensions to modify the raw AST before it enters the transformation pipeline.

```go
type PostConstructionHook interface {
    Execute(ctx context.Context, surface *ast.Surface, compCtx *CompilationContext) (*ast.Surface, error)
}
```

Use cases: injecting extension-contributed slot content; augmenting AST nodes with extension-specific metadata.

### Transformation Pass Hook

Extensions can register additional transformation passes that execute after all built-in passes. Extension passes follow the same interface as built-in passes.

```go
type TransformationPass interface {
    Name() string
    Execute(ctx context.Context, surface *ast.Surface, compCtx *CompilationContext) (*ast.Surface, error)
}

// Register an extension transformation pass
pipeline.RegisterTransformationPass(myExtensionPass{}, AfterLocalization)
```

### Post-Validation Hook

Called after validation, before serialization. Allows extensions to perform additional validation checks specific to their domain.

```go
type PostValidationHook interface {
    Execute(ctx context.Context, surface *ast.Surface, compCtx *CompilationContext) ([]ValidationWarning, error)
}
```

### Serialization Hook

Called after JSON marshaling, allowing extensions to modify the serialized bytes (for example, to inject extension-specific envelope fields).

```go
type SerializationHook interface {
    Execute(ctx context.Context, jsonBytes []byte, compCtx *CompilationContext) ([]byte, error)
}
```

> ⚠️ **Warning:** Extension hooks execute within the compilation pipeline's latency budget. Hook implementations must be fast (< 5ms for most hooks) and must not make synchronous network calls unless absolutely necessary. Slow hooks degrade the p95 and p99 latency of all surface compilations. The pipeline enforces a per-hook timeout (default: 10ms); hooks that exceed this timeout are terminated and their output is discarded (the pre-hook output is used instead).

---

*End of Chapter 08*

**Previous:** [Chapter 07 — AST Design](./07-ast-design.md)
**Next:** [Chapter 09 — Component System](../vol-03-component-system/09-component-system.md)

---

## Phase 1 Complete

Chapters 01–08 constitute the complete Phase 1 documentation foundation. Any engineer who has read all eight chapters can:

- Articulate the platform's architectural rationale and design decisions
- Understand the full data flow from a business service to a rendered screen
- Read and understand an AST builder code example in any AwoERP business service
- Understand why a compiled UI definition looks the way it does
- Begin contributing to the platform as a backend engineer (with Chapter 07 as the primary reference) or a rendering engine engineer (with Chapters 05–06 as the primary references)

**Phase 2 begins with:** [Chapter 09 — Component System](../vol-03-component-system/09-component-system.md)
