# Chapter 18 — Web Rendering Architecture

> **Volume:** IV — Rendering Architecture
> **Phase:** 2
> **Audience:** Web Engineers, Platform Engineers
> **Prerequisites:** Chapters 01–16

---

## Table of Contents

- [18.1 Web Rendering Philosophy](#181-web-rendering-philosophy)
- [18.2 Technology Stack](#182-technology-stack)
- [18.3 Web Rendering Engine Architecture](#183-web-rendering-engine-architecture)
- [18.4 Component Registry and Resolution](#184-component-registry-and-resolution)
- [18.5 Reactive Binding System](#185-reactive-binding-system)
- [18.6 Layout Rendering on Web](#186-layout-rendering-on-web)
- [18.7 Form Rendering on Web](#187-form-rendering-on-web)
- [18.8 Web State Management](#188-web-state-management)
- [18.9 Shell Integration](#189-shell-integration)
- [18.10 Progressive Enhancement and Accessibility](#1810-progressive-enhancement-and-accessibility)
- [18.11 Web Performance](#1811-web-performance)
- [18.12 Web Telemetry](#1812-web-telemetry)
- [18.13 Testing Web Rendering](#1813-testing-web-rendering)

---

## 18.1 Web Rendering Philosophy

The web rendering engine interprets JSON UI definitions and renders them as interactive HTML/CSS/JavaScript in the browser. Its governing principle is the same as the mobile engine: **faithfulness to the definition with platform-appropriate realization**. 

On web, "platform-appropriate" means:

- **Standards-first HTML:** Interactive elements use semantic HTML — `<button>`, `<input>`, `<select>`, `<form>` — not `<div>` role-attributed substitutes. Semantic HTML gives correct keyboard behavior, screen reader support, and browser default styles without extra work.
- **CSS-based layout:** Layout props map to CSS Grid and Flexbox properties. Layout is declared, not calculated. The browser's layout engine handles the hard geometry work.
- **Reactive, not imperative:** UI updates are driven by state changes, not manual DOM manipulation. The rendering engine maintains a reactive state graph; component views subscribe to the parts of state they need.
- **ERP-grade reliability:** Enterprise users work in the application all day. The web engine prioritizes correctness and stability over cutting-edge features. Browser support targets the most recent two stable versions of Chrome, Firefox, Safari, and Edge.

---

## 18.2 Technology Stack

| Layer | Choice | Rationale |
|-------|--------|-----------|
| Language | TypeScript (strict) | Type safety across the large component surface; compile-time schema validation |
| Component model | React 18 + Server Components | Mature, large ecosystem; concurrent features for deferred rendering |
| State management | Zustand + Jotai | Zustand for surface-scoped store; Jotai for fine-grained reactive atoms per binding |
| Data fetching | TanStack Query | Cache management, background refresh, optimistic updates |
| Forms | react-hook-form | Performant uncontrolled form with validation integration |
| CSS framework | CSS Modules + design token CSS variables | Encapsulated styles; tokens from the platform token system |
| Build | Vite | Fast HMR; tree-shaking; module federation for plugin components |
| Bundling | Rollup (via Vite) | Optimized chunk splitting per surface category |

### 18.2.1 Why React

React was chosen over alternatives (Vue, Svelte, SolidJS, Angular) for the following reasons:

- **Ecosystem breadth:** The component types needed by an ERP (complex data tables, rich text editors, PDF viewers, chart libraries, barcode scanners) are all available as well-maintained React packages.
- **TypeScript integration:** React's TypeScript types are thorough and widely understood by the engineering talent pool.
- **Concurrent Mode:** React 18's concurrent features enable deferred rendering of heavy components and `<Suspense>`-based lazy loading without bespoke infrastructure.
- **Team familiarity:** The existing web team has React expertise.

This choice is internal to the web rendering engine. The backend is entirely agnostic to the rendering technology on the client.

---

## 18.3 Web Rendering Engine Architecture

```
JSON Bytes (from network / cache / SSE patch)
        │
        ▼
┌─────────────────────────────────────────┐
│  1. SurfaceLoader                       │
│     Fetches surface definition via      │
│     TanStack Query. Manages cache,      │
│     background refresh, ETag headers    │
└──────────────────┬──────────────────────┘
                   │ Validated SurfaceDefinition (TypeScript types)
                   ▼
┌─────────────────────────────────────────┐
│  2. DefinitionParser                    │
│     Deserializes JSON to typed TS       │
│     structs. Validates against bundled  │
│     JSON Schema. Reports unknown types  │
└──────────────────┬──────────────────────┘
                   │ ComponentTree
                   ▼
┌─────────────────────────────────────────┐
│  3. SurfaceStore (Zustand)              │
│     Single source of truth for this     │
│     surface instance: state variables,  │
│     form state, data source state,      │
│     component tree, patch queue         │
└──────────────────┬──────────────────────┘
                   │ Reactive Store
                   ▼
┌─────────────────────────────────────────┐
│  4. ComponentRenderer (React tree)      │
│     Traverses ComponentTree and         │
│     renders each node via its factory.  │
│     Resolves $data / $state / $expr     │
│     bindings via Jotai atoms derived    │
│     from the SurfaceStore              │
└──────────────────┬──────────────────────┘
                   │ React Virtual DOM
                   ▼
┌─────────────────────────────────────────┐
│  5. DataSourceManager (TanStack Query)  │
│     Fetches / caches data source        │
│     responses. Exposes reactive query   │
│     results to the binding system       │
└──────────────────┬──────────────────────┘
                   │ Query Results
                   ▼
┌─────────────────────────────────────────┐
│  6. ActionExecutor                      │
│     Handles user events, executes       │
│     action chains, manages optimistic   │
│     updates, coordinates with           │
│     TanStack Query cache invalidation   │
└─────────────────────────────────────────┘
```

### 18.3.1 SurfaceLoader

The `SurfaceLoader` is a React component that wraps the surface definition fetch in a TanStack Query call. The fetch includes the full cache key as query parameters (tenant, locale, platform=web, client_version, capability_hash). The response is cached in TanStack Query's client cache using the backend's `ETag` header for validation.

```typescript
// SurfaceLoader.tsx
function SurfaceLoader({ surfaceId }: { surfaceId: string }) {
  const { data, isLoading, error } = useSurfaceDefinition(surfaceId);

  if (isLoading) return <SurfaceSkeletonLoader />;
  if (error) return <SurfaceErrorState error={error} />;
  if (!data) return null;

  return (
    <SurfaceStoreProvider definition={data}>
      <ComponentRenderer node={data.root} />
    </SurfaceStoreProvider>
  );
}

function useSurfaceDefinition(surfaceId: string) {
  const { tenant, locale, user } = useSession();
  return useQuery({
    queryKey: ['surface', surfaceId, tenant.id, locale, user.roleSetHash],
    queryFn: () => fetchSurfaceDefinition(surfaceId, { tenant, locale }),
    staleTime: 30_000,       // 30s before background refresh
    gcTime: 5 * 60_000,      // 5min before cache eviction
  });
}
```

### 18.3.2 DefinitionParser

The `DefinitionParser` validates the received JSON against the bundled JSON Schema definitions and deserializes it to strongly-typed TypeScript structs. Unknown component types produce a `UnknownNode` struct with the raw props and the declared fallback subtree.

```typescript
// Parser result type
type ParseResult =
  | { ok: true; definition: SurfaceDefinition }
  | { ok: false; error: ParseError; partial?: SurfaceDefinition };

function parseSurfaceDefinition(json: unknown): ParseResult {
  const validation = validateAgainstSchema(json);
  if (validation.fatal) {
    return { ok: false, error: validation.error };
  }
  const definition = deserialize(json as RawSurfaceJSON);
  if (validation.warnings.length > 0) {
    telemetry.recordParseWarnings(definition.id, validation.warnings);
  }
  return { ok: true, definition };
}
```

### 18.3.3 SurfaceStore

Each surface instance owns a Zustand store that is the single source of truth for that surface's runtime state. The store is created fresh for each surface mount and destroyed on unmount.

```typescript
interface SurfaceStore {
  // State variables declared in the surface definition
  stateVars: Record<string, unknown>;
  setStateVar: (key: string, value: unknown) => void;

  // Form state (managed by react-hook-form; mirrored here for cross-component reads)
  formValues: Record<string, unknown>;
  formErrors: Record<string, FieldError>;
  formDirty: Record<string, boolean>;

  // Data source state
  dataSources: Record<string, DataSourceState>;

  // Patch queue from real-time updates
  pendingPatches: PatchOperation[];
  applyPatch: (patch: PatchOperation) => void;
}
```

---

## 18.4 Component Registry and Resolution

The web component registry maps DSL type strings to React component factories.

```typescript
// Component factory type
type WebComponentFactory<P = BaseNodeProps> = React.ComponentType<{
  node: ResolvedNode<P>;
  context: RenderContext;
}>;

class WebComponentRegistry {
  private factories = new Map<string, WebComponentFactory>();

  register(typeID: string, factory: WebComponentFactory): void {
    this.factories.set(typeID, factory);
  }

  resolve(typeID: string): WebComponentFactory | undefined {
    return this.factories.get(typeID);
  }
}

export const registry = new WebComponentRegistry();
```

### 18.4.1 ComponentRenderer

The `ComponentRenderer` is the central recursive React component. It looks up the node's type in the registry, passes the node and its evaluated props to the factory component, and handles the unknown-type fallback.

```typescript
function ComponentRenderer({ node }: { node: ResolvedNode }) {
  const Factory = registry.resolve(node.type);

  if (!Factory) {
    telemetry.record('unknown_component', { typeID: node.type });
    if (node.fallback) {
      return <ComponentRenderer node={node.fallback} />;
    }
    return <UnknownComponentPlaceholder typeID={node.type} />;
  }

  return (
    <ErrorBoundary fallback={<ComponentErrorState node={node} />}>
      <Suspense fallback={<ComponentSkeletonLoader />}>
        <Factory node={node} context={useSurfaceContext()} />
      </Suspense>
    </ErrorBoundary>
  );
}
```

### 18.4.2 Lazy-Loaded Component Bundles

Built-in component categories are code-split into lazy-loaded bundles. The bundles are loaded on demand when the first component of that category is rendered:

| Bundle | Components | Approx. gzipped size |
|--------|-----------|----------------------|
| `core` | layout.*, ui.text, ui.heading, ui.badge, action.button | 42 KB |
| `forms` | field.*, form.* | 68 KB |
| `tables` | ui.data_table, table columns | 55 KB |
| `charts` | chart.* | 110 KB (includes chart library) |
| `workflow` | workflow.* | 28 KB |
| `erp` | erp.* | 85 KB |
| `dashboard` | dashboard.* | 34 KB |

Plugin bundles are loaded via Module Federation at runtime from their declared remote URLs.

---

## 18.5 Reactive Binding System

The binding system resolves DSL binding expressions to live values and keeps the UI up to date when the underlying state changes.

### 18.5.1 Binding Types

| Binding Syntax | Source | React Hook |
|---------------|--------|------------|
| `$data.source_id.field_name` | Data source query result | `useDataBinding(source_id, field_name)` |
| `$state.var_name` | Surface state variable | `useStateVar(var_name)` |
| `$form.field_name` | Form field current value | `useFormValue(field_name)` |
| `$user.attribute` | Session user context | `useUserAttribute(attribute)` |
| `$tenant.attribute` | Tenant context | `useTenantAttribute(attribute)` |
| `$expr{ expression }` | Evaluated expression | `useExpression(expression)` |

### 18.5.2 Jotai Atoms for Fine-Grained Reactivity

Each binding is implemented as a derived Jotai atom. Components subscribe to only the atoms they need; an update to one atom re-renders only the components that depend on it — not the entire surface tree.

```typescript
// Atom factory for a $data binding
function makeDataBindingAtom(sourceID: string, fieldPath: string) {
  return atom((get) => {
    const sourceState = get(dataSourceAtomFamily(sourceID));
    if (sourceState.status !== 'success') return undefined;
    return getByPath(sourceState.data, fieldPath);
  });
}

// Usage in a component
function TextComponent({ node }: { node: TextNode }) {
  const value = useAtomValue(
    useMemo(() => resolveBinding(node.props.content), [node.props.content])
  );
  return <span className={styles.text}>{value}</span>;
}
```

### 18.5.3 Expression Evaluation

`$expr{ }` bindings use a sandboxed expression evaluator (a subset of JavaScript: arithmetic, comparison, string concat, ternary, array map/filter). The evaluator receives the full binding scope (state, form, data, user, tenant) and returns the computed value. Expressions are compiled to safe evaluator functions at parse time and cached.

```typescript
// Expression evaluator (sandboxed — no access to global scope)
const exprResult = evaluate(
  node.props.visible,   // e.g.: "$expr{ $data.invoice.status === 'approved' }"
  {
    $data: dataSources,
    $state: stateVars,
    $form: formValues,
    $user: session.user,
    $tenant: session.tenant,
  }
);
```

---

## 18.6 Layout Rendering on Web

### 18.6.1 layout.stack → Flexbox

`layout.stack` with `direction: "vertical"` renders as a flex container with `flex-direction: column`. `direction: "horizontal"` renders with `flex-direction: row`. The `gap` token maps to the CSS `gap` property.

```typescript
// Stack layout component
function StackLayout({ node, context }: { node: StackNode; context: RenderContext }) {
  return (
    <div
      className={styles.stack}
      style={{
        flexDirection: node.direction === 'vertical' ? 'column' : 'row',
        gap: `var(--spacing-${node.gap ?? 'md'})`,
        alignItems: node.align ?? 'stretch',
        justifyContent: node.justify ?? 'flex-start',
        flexWrap: node.wrap ? 'wrap' : 'nowrap',
      }}
    >
      {node.children.map((child) => (
        <ComponentRenderer key={child.id} node={child} />
      ))}
    </div>
  );
}
```

### 18.6.2 layout.grid → CSS Grid

`layout.grid` renders as a CSS Grid container. Column templates use `fr` units as-is from the DSL; named grid areas are supported for dashboard and form layouts.

```typescript
function GridLayout({ node }: { node: GridNode }) {
  const style: React.CSSProperties = {
    display: 'grid',
    gridTemplateColumns: node.columns,  // e.g. "1fr 2fr 1fr"
    gridTemplateRows: node.rows ?? 'auto',
    gap: `var(--spacing-${node.rowGap ?? 'md'}) var(--spacing-${node.colGap ?? 'md'})`,
  };

  return (
    <div style={style}>
      {node.children.map((child) => (
        <div
          key={child.id}
          style={{
            gridColumn: child.gridPlacement?.colSpan
              ? `span ${child.gridPlacement.colSpan}`
              : undefined,
            gridRow: child.gridPlacement?.rowSpan
              ? `span ${child.gridPlacement.rowSpan}`
              : undefined,
          }}
        >
          <ComponentRenderer node={child} />
        </div>
      ))}
    </div>
  );
}
```

### 18.6.3 Responsive Breakpoints

The web engine applies responsive styles using CSS custom properties and a `data-breakpoint` attribute on the document root, updated by a `ResizeObserver` on the root container. Components use CSS `@container` queries or read the `data-breakpoint` attribute to apply responsive styles.

Breakpoints match the platform breakpoint system (Chapter 10):

| Breakpoint | Min Width |
|------------|----------|
| `xs` | 0px |
| `sm` | 480px |
| `md` | 768px |
| `lg` | 1024px |
| `xl` | 1280px |

---

## 18.7 Form Rendering on Web

### 18.7.1 react-hook-form Integration

The `form.container` component initializes a `react-hook-form` instance and exposes it via React context. All `field.*` components within the form use `useFormContext()` to register themselves with the form and receive their value/error state.

```typescript
function FormContainer({ node }: { node: FormContainerNode }) {
  const methods = useForm({
    defaultValues: resolveInitialValues(node.initialValues, dataSources),
    mode: 'onBlur',
    reValidateMode: 'onChange',
  });

  // Mirror form values to SurfaceStore for cross-component $form bindings
  const setSurfaceFormValues = useSurfaceStore((s) => s.setFormValues);
  useEffect(() => {
    const subscription = methods.watch((values) => setSurfaceFormValues(values));
    return () => subscription.unsubscribe();
  }, [methods]);

  return (
    <FormProvider {...methods}>
      <form onSubmit={methods.handleSubmit(handleSubmit)} noValidate>
        {node.sections.map((section) => (
          <FormSection key={section.id} node={section} />
        ))}
        <FormActionBar actions={node.actions} />
      </form>
    </FormProvider>
  );
}
```

### 18.7.2 Validation Integration

Client-side validation rules declared in the field's `validation` prop are translated to `react-hook-form` validation rules at form initialization time. Remote validation rules (`rule: "remote_unique"`) are implemented as async validators that call the backend's validation endpoint.

```typescript
function buildValidationRules(field: FieldNode): RegisterOptions {
  const rules: RegisterOptions = {};

  for (const rule of field.validation ?? []) {
    switch (rule.rule) {
      case 'required':
        rules.required = rule.message ?? 'This field is required';
        break;
      case 'min_length':
        rules.minLength = { value: rule.value, message: rule.message };
        break;
      case 'pattern':
        rules.pattern = { value: new RegExp(rule.pattern), message: rule.message };
        break;
      case 'remote_unique':
        rules.validate = async (value) => {
          const result = await validateRemoteUnique(field.id, value);
          return result.valid || (result.message ?? 'Value already exists');
        };
        break;
    }
  }
  return rules;
}
```

### 18.7.3 Autosave

When `form.container` declares `auto_save`, the form engine debounces field changes and submits a partial PATCH request to the auto-save endpoint. The debounce delay defaults to 2 seconds. Auto-save state (saving / saved / error) is displayed in the form action bar.

---

## 18.8 Web State Management

### 18.8.1 State Variable Initialization

State variables declared in the surface definition's `state` block are initialized in the `SurfaceStore` when the surface mounts. Initial values can be literals, bound to a data source field, or a URL query parameter.

```typescript
// State variable initialization
function initializeStateVars(
  declarations: StateVarDeclaration[],
  context: SurfaceContext
): Record<string, unknown> {
  return Object.fromEntries(
    declarations.map((decl) => [
      decl.name,
      resolveInitialValue(decl.initial_value, context),
    ])
  );
}
```

### 18.8.2 Cross-Surface State

State that needs to survive surface navigation (e.g., a filter set carried from a list surface to a detail surface) is passed as route parameters (see Chapter 19). The target surface declares the received parameters as state variable initial values bound to `$route.params.param_name`.

Global application state (authentication, tenant context, notifications) lives in a session-scoped Zustand store that is instantiated once when the application shell loads.

---

## 18.9 Shell Integration

### 18.9.1 Application Shell

The AwoERP web application shell is a thin host application that:
- Handles authentication (OAuth 2.0 + PKCE flow)
- Establishes the session context (tenant, user, locale, feature flags)
- Renders the navigation chrome (`nav.sidebar`, `nav.top_bar`)
- Manages the main content area, routing surface definitions by route

The shell is a small, stable application. All application UI above the authentication layer is defined by surface definitions fetched from the backend.

### 18.9.2 Shell-Surface Contract

The shell provides the surface rendering engine with the following context:

```typescript
interface SurfaceContext {
  tenantId: string;
  userId: string;
  roles: string[];
  featureFlags: Record<string, boolean>;
  locale: string;
  themeTokens: DesignTokenSet;
  platformCapabilities: PlatformCapabilitySet;
  navigate: (route: RouteSpec) => void;
  showToast: (toast: ToastSpec) => void;
  openModal: (modal: ModalSpec) => void;
}
```

### 18.9.3 Theme Token Injection

The shell injects design token CSS custom properties on the `:root` element at startup. Tenant-specific theme overrides (declared in the tenant's theme config) are applied on top of the platform default tokens.

```css
/* Generated by the shell from the active DesignTokenSet */
:root {
  --color-primary-500: #2563EB;
  --color-primary-600: #1D4ED8;
  --spacing-xs: 4px;
  --spacing-sm: 8px;
  --spacing-md: 16px;
  --spacing-lg: 24px;
  --spacing-xl: 32px;
  --spacing-2xl: 48px;
  --font-size-sm: 14px;
  --font-size-base: 16px;
  --border-radius-md: 6px;
  /* ... all tokens ... */
}
```

---

## 18.10 Progressive Enhancement and Accessibility

### 18.10.1 Semantic HTML

The web rendering engine uses semantic HTML for all interactive elements:

| Component | HTML element |
|-----------|-------------|
| `action.button` | `<button type="button">` |
| Form submit | `<button type="submit">` |
| `field.text` (single line) | `<input type="text">` |
| `field.text` (multi-line) | `<textarea>` |
| `field.select` | `<select>` + `<option>` (or custom when searchable) |
| `field.checkbox` | `<input type="checkbox">` |
| `field.radio_group` | `<fieldset>` + `<input type="radio">` |
| `nav.tab_bar` | `<nav>` + `<ul>` + `<li>` + `<a>` or `<button>` |
| `ui.data_table` | `<table>` + `<thead>` + `<tbody>` |
| `ui.modal` | `<dialog>` with `aria-modal="true"` |

### 18.10.2 ARIA Attributes

ARIA attributes are populated from component props:

| Prop | ARIA attribute |
|------|---------------|
| `label` | `aria-label` |
| `aria_describedby` | `aria-describedby` |
| `required: true` | `aria-required="true"` |
| `disabled: true` | `aria-disabled="true"` (+ native `disabled`) |
| `error_message` | `aria-invalid="true"` + `aria-errormessage` |
| `loading: true` | `aria-busy="true"` |

### 18.10.3 Keyboard Navigation

All interactive components support full keyboard operation:
- Tab / Shift+Tab: focus traversal in DOM order
- Enter / Space: activate buttons and links
- Arrow keys: navigate within menus, tabs, radio groups, select lists
- Escape: close modals, drawers, dropdowns

The rendering engine does not set `tabIndex` on non-interactive elements. Focus management for modals uses the `focus-trap` library to restrict Tab focus within the open modal and restore focus to the trigger element on close.

### 18.10.4 Colour and Contrast

All component colour choices use design tokens that meet WCAG 2.1 AA contrast requirements (4.5:1 for normal text, 3:1 for large text and UI components). A token audit tool is included in the platform toolchain (see Chapter 43) that fails CI if any token combination falls below the AA threshold.

---

## 18.11 Web Performance

### 18.11.1 Payload Budgets

The web rendering engine enforces payload budgets defined in Chapter 8:
- **Surface definition JSON:** 200 KB uncompressed (enforced at compilation; Chapter 8 Section 8.3.4)
- **Data source responses:** 2,000 rows max, enforced at compilation
- **Component bundles:** lazy-loaded; no bundle over 150 KB gzipped before splitting

### 18.11.2 Surface Definition Caching

Surface definitions are cached using TanStack Query's cache with a 30-second stale time and 5-minute GC time. The background refetch uses the `ETag` and `If-None-Match` headers to avoid re-downloading unchanged definitions.

For offline support, definitions are additionally stored in `localStorage` with a 24-hour TTL. On mount, the engine serves the cached definition immediately while the network refetch runs in the background.

### 18.11.3 Virtual Scrolling

`ui.data_table` uses TanStack Virtual for row virtualization when the row count exceeds the `virtualize_threshold` (default: 100 rows). Only the rows currently in the viewport plus a configurable overscan buffer are rendered in the DOM.

### 18.11.4 Concurrent Rendering

Heavy components (charts, complex data grids, PDF viewers) are wrapped in `<Suspense>` boundaries. React's concurrent mode defers rendering these components until after the initial paint, keeping the surface interactive even when a heavy component is loading.

---

## 18.12 Web Telemetry

The web rendering engine emits telemetry events to the same analytics pipeline as the mobile engines. Events are batched and sent via a `navigator.sendBeacon()` call to survive page unloads.

| Event | Key Properties | Purpose |
|-------|---------------|---------|
| `surface_view` | surface_id, render_time_ms, cache_hit, ttfb_ms | Surface open performance |
| `surface_render_error` | surface_id, error_code, node_id | Rendering failures |
| `unknown_component` | type_id, fallback_used, surface_id | Version skew detection |
| `action_executed` | action_id, action_type, duration_ms | Action performance |
| `action_failed` | action_id, error_code, http_status | Action failure rate |
| `data_source_load` | source_id, duration_ms, row_count, cache_hit | Data fetch performance |
| `form_submitted` | surface_id, field_count, validation_errors | Form funnel |
| `virtual_scroll_active` | surface_id, row_count | Virtualization usage |
| `component_bundle_load` | bundle_name, duration_ms | Bundle load performance |

Performance metrics additionally report:
- `time_to_interactive` (TTI): time from navigation start to first interactive surface
- `largest_contentful_paint` (LCP): measured via the Web Vitals library
- `cumulative_layout_shift` (CLS): measured via the Web Vitals library

---

## 18.13 Testing Web Rendering

### 18.13.1 Unit Testing Components

Each component factory is unit-tested with React Testing Library. Tests assert on rendered HTML structure, ARIA attributes, and user interaction behavior using `userEvent`.

```typescript
// Unit test for ButtonComponent
describe('ButtonComponent', () => {
  it('renders with correct ARIA attributes when disabled', () => {
    const node = mockButtonNode({ label: 'Submit', disabled: true });
    render(<ButtonComponent node={node} context={mockContext()} />);
    const button = screen.getByRole('button', { name: 'Submit' });
    expect(button).toBeDisabled();
    expect(button).toHaveAttribute('aria-disabled', 'true');
  });
});
```

### 18.13.2 Integration Tests with Mock Definitions

Integration tests mount the full `SurfaceLoader` with a mocked fetch that returns a known surface definition JSON. The test asserts on the final rendered HTML structure and user interaction flows.

```typescript
it('submits form and shows success toast', async () => {
  server.use(
    http.post('/api/invoices', () => HttpResponse.json({ id: '123' }))
  );
  renderSurface('invoice_create_surface');
  await userEvent.type(screen.getByLabelText('Amount'), '1500');
  await userEvent.click(screen.getByRole('button', { name: 'Save Invoice' }));
  await screen.findByText('Invoice saved');
});
```

### 18.13.3 Visual Regression Testing

Storybook is used as the component workbench. Each component has stories for all significant states and variants. Chromatic (or a self-hosted equivalent) captures visual snapshots of each story and fails CI on unexpected visual changes.

### 18.13.4 Accessibility Testing

`axe-core` runs on every component story and every integration test render. Any WCAG 2.1 AA violation causes a test failure. The rule set includes: `aria-required-attr`, `color-contrast`, `keyboard`, `focus-trap`, `label`.

### 18.13.5 Contract Testing

The same golden definition file suite used for mobile contract tests (Chapter 17, Section 17.13.4) is also used by the web engine's parser tests. The suite ensures backend-produced definitions are accepted without modification.

---

*End of Chapter 18*

**Previous:** [Chapter 17 — Mobile Rendering Architecture](./17-mobile-rendering-architecture.md)
**Next:** [Chapter 19 — Navigation Framework](./19-navigation-framework.md)
