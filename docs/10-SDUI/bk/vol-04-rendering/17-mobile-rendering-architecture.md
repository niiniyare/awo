# Chapter 17 — Mobile Rendering Architecture

> **Volume:** IV — Rendering Architecture
> **Phase:** 2
> **Audience:** Mobile Engineers (iOS/Android), Platform Engineers
> **Prerequisites:** Chapters 01–16

---

## Table of Contents

- [17.1 Mobile Rendering Philosophy](#171-mobile-rendering-philosophy)
- [17.2 Supported Mobile Platforms](#172-supported-mobile-platforms)
- [17.3 Mobile Rendering Engine Architecture](#173-mobile-rendering-engine-architecture)
- [17.4 Component-to-Native Mapping](#174-component-to-native-mapping)
- [17.5 Layout Rendering on Mobile](#175-layout-rendering-on-mobile)
- [17.6 Form Rendering on Mobile](#176-form-rendering-on-mobile)
- [17.7 Navigation Rendering on Mobile](#177-navigation-rendering-on-mobile)
- [17.8 Mobile State Management](#178-mobile-state-management)
- [17.9 Mobile Performance](#179-mobile-performance)
- [17.10 Mobile Offline Support](#1710-mobile-offline-support)
- [17.11 Mobile Accessibility](#1711-mobile-accessibility)
- [17.12 Mobile Telemetry](#1712-mobile-telemetry)
- [17.13 Testing Mobile Rendering](#1713-testing-mobile-rendering)

---

## 17.1 Mobile Rendering Philosophy

The mobile rendering engine is a **faithful, native-first interpreter** of UI definitions. Its two governing commitments are:

**Faithfulness to the definition.** The engine renders exactly what the backend specifies — the component hierarchy, the property values, the event bindings, the action definitions. It does not add components, reorder children, or substitute its own judgment for the backend's intent. When the engine encounters a component it does not recognize, it renders the declared fallback — not a guess at what the component might mean.

**Native-first realization.** Within the constraints of fidelity, the engine uses the best available native control for each semantic component type. A `field.date` on iOS is a native `DatePicker` when `input_hint: "native_date_picker"` is declared. A `nav.tab_bar` on iOS uses `UITabBarController` conventions — bottom-positioned, icon+label style. A `ui.modal` on iOS is a `UISheetPresentationController` using the platform's standard sheet appearance. The user should not be able to distinguish an AwoERP screen from a natively-built iOS or Android screen.

These two commitments are in harmony, not tension. Fidelity concerns *what* is rendered; native-first concerns *how* it is rendered.

---

## 17.2 Supported Mobile Platforms

### 17.2.1 iOS (Swift / SwiftUI)

**Minimum deployment target:** iOS 16.0

**Primary framework:** SwiftUI, with UIKit bridges for components where SwiftUI does not yet provide adequate native controls (complex collection views, certain keyboard management scenarios).

**Swift Package structure:**
```
AwoERPMobile/
├── AwoERPCore/           # Platform-agnostic: schema types, expression evaluator, state machine
├── AwoERPiOS/            # iOS rendering engine: component registry, SwiftUI views
└── AwoERPApp/            # iOS application shell: navigation, app lifecycle, deep links
```

### 17.2.2 Android (Kotlin / Jetpack Compose)

**Minimum SDK:** API 26 (Android 8.0)

**Primary framework:** Jetpack Compose, with Android View system bridges for legacy components or platform integrations that Compose does not yet support natively.

**Gradle module structure:**
```
AwoERPMobile/
├── core/                 # Platform-agnostic: schema types, expression evaluator, state machine
├── android-engine/       # Android rendering engine: component registry, Composables
└── app/                  # Android application shell: navigation, activity, deep links
```

### 17.2.3 Cross-Platform Strategy

AwoERP does not use a cross-platform framework (React Native, Flutter) for the mobile rendering engines. The two native implementations share a **platform-agnostic core** — JSON schema types, expression evaluator, state machine, data source adapter interfaces — through a language-appropriate mechanism:

- The core logic is implemented in **Go** and compiled to a shared library via `gomobile`, consumed by both iOS and Android; or
- Duplicated Go implementations in Swift and Kotlin, maintaining behavioral parity through shared contract tests.

The rendering engine — the layer that maps component types to native views — is written natively on each platform. This is intentional: rendering engine quality depends on deep platform expertise, best expressed in each platform's native language and frameworks.

---

## 17.3 Mobile Rendering Engine Architecture

The mobile rendering engine has six internal subsystems that collaborate to turn a JSON definition into an interactive native screen.

```
JSON Bytes (from network / cache)
        │
        ▼
┌─────────────────────────────────────┐
│  1. JSON Parser & Schema Validator  │
│     Deserializes JSON, validates    │
│     against bundled schema, reports │
│     unknown types for telemetry     │
└────────────────┬────────────────────┘
                 │ Typed SurfaceDefinition
                 ▼
┌─────────────────────────────────────┐
│  2. Component Resolver              │
│     Maps each node's `type` string  │
│     to the registered native view   │
│     factory, or to the fallback     │
│     component if unrecognized       │
└────────────────┬────────────────────┘
                 │ Resolved ComponentTree
                 ▼
┌─────────────────────────────────────┐
│  3. Rendering Tree Builder          │
│     Constructs the native view tree │
│     from resolved components and    │
│     their props. Handles slots,     │
│     children, layout composition    │
└────────────────┬────────────────────┘
                 │ Native View Hierarchy
                 ▼
┌─────────────────────────────────────┐
│  4. Data Source Manager             │
│     Loads declared data sources,    │
│     resolves $data bindings,        │
│     manages refresh lifecycle,      │
│     provides reactive data streams  │
└────────────────┬────────────────────┘
                 │ Reactive Data Streams
                 ▼
┌─────────────────────────────────────┐
│  5. State & Expression Engine       │
│     Manages state variables,        │
│     evaluates $expr and $if         │
│     bindings reactively, propagates │
│     state changes to bound views    │
└────────────────┬────────────────────┘
                 │ Reactive State
                 ▼
┌─────────────────────────────────────┐
│  6. Action Dispatcher               │
│     Handles user interaction        │
│     events, executes action chains, │
│     manages optimistic updates,     │
│     queues offline actions          │
└─────────────────────────────────────┘
```

### 17.3.1 JSON Parser and Schema Validator

The parser validates received JSON bytes against the bundled schema (the same JSON Schema definitions used by the backend compiler, bundled as a static app resource).

Validation produces one of three outcomes:

- **Full validation pass:** The definition is well-formed. Proceed to component resolution.
- **Partial validation with warnings:** Some nodes have unrecognized props or types. Record warnings to the telemetry buffer; proceed with graceful fallbacks for unrecognized elements.
- **Fatal validation failure:** The definition is structurally invalid (missing required envelope fields, malformed root node). Log a fatal error, emit a telemetry event, and show the surface error state.

The deserializer produces a typed `SurfaceDefinition` struct — not a raw dictionary. All known component types have concrete Swift/Kotlin structs. Unknown types are deserialized into an `UnknownNode` struct carrying the raw props map, the type string, and the declared fallback node.

### 17.3.2 Component Resolver

The component registry maps type identifier strings to native view factories.

**iOS (Swift):**
```swift
protocol ComponentFactory {
    func makeView(node: ResolvedNode, context: RenderContext) -> AnyView
}

class ComponentRegistry {
    private var factories: [String: any ComponentFactory] = [:]

    func register(_ typeID: String, factory: some ComponentFactory) {
        factories[typeID] = factory
    }

    func resolve(_ typeID: String) -> (any ComponentFactory)? {
        return factories[typeID]
    }
}
```

**Android (Kotlin):**
```kotlin
typealias ComponentFactory = @Composable (node: ResolvedNode, context: RenderContext) -> Unit

class ComponentRegistry {
    private val factories = mutableMapOf<String, ComponentFactory>()

    fun register(typeID: String, factory: ComponentFactory) {
        factories[typeID] = factory
    }

    fun resolve(typeID: String): ComponentFactory? = factories[typeID]
}
```

The registry is populated at application startup with all built-in component factories. Plugin components register their factories through the same mechanism during app initialization.

### 17.3.3 Rendering Tree Builder

The tree builder performs a depth-first traversal of the resolved `ComponentTree`, calling each node's factory to produce its native view, then composing the views according to parent-child and slot relationships.

The builder handles three special cases:

**Unknown nodes:** When `resolve(typeID)` returns nil, the builder uses the node's `fallback` subtree. If no fallback is declared, it instantiates the `UnknownComponentView` — a neutral, non-interactive placeholder that records the unknown type in the telemetry buffer.

**Control flow nodes (`flow.*`):** These are not rendered as views. The builder evaluates their conditions using the expression engine and produces the appropriate child subtree.

**Data nodes (`data.*`):** These register data sources with the Data Source Manager and produce no visible views.

### 17.3.4 Action Dispatcher

The action dispatcher bridges user interactions (taps, form submissions) and their effects. It receives `ActionEvent` objects from components and executes the corresponding action chain.

```swift
// iOS Action Dispatcher
class ActionDispatcher: ObservableObject {
    func dispatch(_ event: ActionEvent, context: ActionContext) async {
        guard let actionDef = context.surface.findAction(event.actionID) else {
            telemetry.record(.unknownActionType(event.actionID))
            return
        }
        await executeChain(actionDef.chain, context: context)
    }

    private func executeChain(_ chain: [ActionNode], context: ActionContext) async {
        for action in chain {
            let result = await execute(action, context: context)
            if case .failure(let error) = result {
                await executeChain(action.errorChain, context: context.withError(error))
                return
            }
        }
    }
}
```

### 17.3.5 View Reconciler

When the surface receives a patch update (from a real-time subscription or a server diff), the view reconciler applies the patch to the current view tree. It matches nodes by their `id` fields and applies patch operations:

| Operation | Behaviour |
|-----------|-----------|
| `SET_PROPS` | Update the matched node's props, triggering reactive updates to bound views |
| `INSERT` | Add a new node at the specified position |
| `REMOVE` | Remove the node and its subtree, releasing resources |
| `REPLACE` | Replace the node's entire subtree |

---

## 17.4 Component-to-Native Mapping

### 17.4.1 Mapping Table

| DSL Type | iOS (SwiftUI) | Android (Compose) | Notes |
|----------|--------------|-------------------|-------|
| `layout.stack` vertical | `VStack` | `Column` | |
| `layout.stack` horizontal | `HStack` | `Row` | |
| `layout.grid` | `LazyVGrid` | `LazyVerticalGrid` | |
| `layout.scroll` | `ScrollView` | `LazyColumn` / `LazyRow` | |
| `layout.tabs` | `TabView` | `TabRow` + `HorizontalPager` | |
| `ui.text` | `Text` | `Text` | |
| `ui.heading` | `Text` (styled) | `Text` (styled) | |
| `ui.badge` | `Label` (custom) | `Badge` (Material 3) | |
| `ui.avatar` | `AsyncImage` (circular) | Coil `AsyncImage` (circular) | |
| `ui.spinner` | `ProgressView` | `CircularProgressIndicator` | |
| `ui.progress_bar` | `ProgressView` (linear) | `LinearProgressIndicator` | |
| `action.button` | `Button` | `Button` / `OutlinedButton` | Variant maps to style |
| `action.icon_button` | `Button` (icon only) | `IconButton` | |
| `ui.card` | `.background` + `.cornerRadius` | `Card` (Material 3) | |
| `ui.modal` | `.sheet` / `.fullScreenCover` | `ModalBottomSheet` / `Dialog` | |
| `ui.drawer` | Custom side panel | `ModalNavigationDrawer` | |
| `field.text` | `TextField` / `TextEditor` | `OutlinedTextField` | |
| `field.date` | `DatePicker` | `DatePickerDialog` | When `native_date_picker` hint |
| `field.select` | `Picker` (menu style) | `ExposedDropdownMenuBox` | |
| `field.toggle` | `Toggle` | `Switch` | |
| `field.file_upload` | `PhotosPicker` / `DocumentPicker` | `ActivityResultContracts` | |
| `nav.tab_bar` | `TabView` (bottom) | `NavigationBar` (bottom) | |
| `nav.sidebar` | `NavigationSplitView` | `PermanentNavigationDrawer` | iPad / large screen |
| `ui.data_table` | `List` + custom cells | `LazyColumn` + custom items | Card layout on mobile |

### 17.4.2 Unsupported Component Fallback Strategy

When a component type is not in the registry, the following fallback hierarchy applies:

1. **Node-declared fallback:** Use the `fallback` subtree from the node's definition (if present)
2. **Registry-declared fallback:** Look up the type in the bundled fallback map (populated from the component registry's `fallback` declarations at build time)
3. **Generic unknown component placeholder:** Render `UnknownComponentView` — a light grey rounded rectangle with an information icon

All three outcomes record a telemetry event with the unknown type ID and fallback used, enabling the platform team to detect version skew between backend and rendering engine.

### 17.4.3 Plugin Component Bridges

Plugin and tenant-specific components that require native implementation register their factories before surface rendering begins. The Plugin SDK provides the `PluginComponentFactory` protocol/interface:

```swift
// iOS Plugin Registration
struct MyCustomScorecard: PluginComponentFactory {
    static let typeID = "plugin.acme_corp.vendor_scorecard"

    func makeView(node: ResolvedNode, context: RenderContext) -> AnyView {
        let props = try! node.decodeProps(as: VendorScorecardProps.self)
        return AnyView(VendorScorecardView(
            vendorID: props.vendorID,
            showHistory: props.showHistory
        ))
    }
}

// Registration in App initializer
ComponentRegistry.shared.register(
    MyCustomScorecard.typeID,
    factory: MyCustomScorecard()
)
```

---

## 17.5 Layout Rendering on Mobile

### 17.5.1 Stack Layout

`layout.stack` with `direction: "vertical"` maps to `VStack` (iOS) or `Column` (Android). The `gap` token converts to the platform's spacing value.

```swift
// iOS — layout.stack renderer
struct StackLayoutView: View {
    let node: StackLayoutNode
    let children: [ResolvedNode]

    var body: some View {
        Group {
            if node.direction == .vertical {
                VStack(alignment: node.swiftUIAlignment,
                       spacing: DesignTokens.spacing(node.gap)) {
                    ForEach(children, id: \.id) { child in
                        ComponentRenderer(node: child)
                    }
                }
            } else {
                HStack(alignment: node.swiftUIVerticalAlignment,
                       spacing: DesignTokens.spacing(node.gap)) {
                    ForEach(children, id: \.id) { child in
                        ComponentRenderer(node: child)
                    }
                }
            }
        }
    }
}
```

### 17.5.2 Grid on Mobile

`layout.grid` uses `LazyVGrid` (iOS) or `LazyVerticalGrid` (Android) with `GridItem` definitions derived from the column template. On the `xs` breakpoint, all columns automatically collapse to a single column (see Chapter 10, Section 10.7.1).

```swift
// iOS — layout.grid renderer
struct GridLayoutView: View {
    let node: GridLayoutNode
    let children: [ResolvedNode]
    @Environment(\.horizontalSizeClass) var sizeClass

    var columns: [GridItem] {
        let count = sizeClass == .compact ? 1 : node.effectiveColumnCount
        return Array(
            repeating: GridItem(.flexible(), spacing: DesignTokens.spacing(node.colGap)),
            count: count
        )
    }

    var body: some View {
        LazyVGrid(columns: columns,
                  spacing: DesignTokens.spacing(node.rowGap)) {
            ForEach(children, id: \.id) { child in
                ComponentRenderer(node: child)
                    .gridCellColumns(
                        child.gridPlacement?.effectiveColSpan(sizeClass: sizeClass) ?? 1
                    )
            }
        }
    }
}
```

### 17.5.3 Safe Area Handling

The `layout.page` renderer automatically applies safe area insets. Platform navigation bars are handled by the navigation component (Section 17.7) and do not require explicit handling in surface layouts.

```swift
// iOS — layout.page applies safe area insets
struct PageLayoutView: View {
    let node: PageLayoutNode
    let contentNode: ResolvedNode

    var body: some View {
        ScrollView {
            ComponentRenderer(node: contentNode)
                .padding(.horizontal, DesignTokens.spacing(.md))
        }
        .safeAreaInset(edge: .bottom) {
            if let fab = node.slots["fab"] {
                ComponentRenderer(node: fab)
                    .padding(.bottom, DesignTokens.spacing(.md))
            }
        }
    }
}
```

---

## 17.6 Form Rendering on Mobile

### 17.6.1 Keyboard Avoidance

The form container renderer wraps the form in a scroll view with interactive keyboard avoidance. When the keyboard appears, the scroll view adjusts to keep the focused field visible.

```swift
// iOS — form container with keyboard avoidance
struct FormContainerView: View {
    @StateObject private var formState: FormStateModel
    @FocusState private var focusedField: String?

    var body: some View {
        ScrollViewReader { proxy in
            ScrollView {
                VStack(spacing: DesignTokens.spacing(.md)) {
                    ForEach(node.sections) { section in
                        FormSectionView(section: section, formState: formState)
                    }
                    FormActionBarView(actions: node.actions, formState: formState)
                }
                .padding()
            }
            .scrollDismissesKeyboard(.interactively)
            .onChange(of: focusedField) { fieldID in
                if let id = fieldID {
                    withAnimation { proxy.scrollTo(id, anchor: .center) }
                }
            }
        }
    }
}
```

### 17.6.2 Native Input Pickers

When `field.date` receives `input_hint: "native_date_picker"`, iOS uses `DatePicker` with `.compact` or `.wheel` display mode. Android uses `DatePickerDialog` via `DatePickerState`. When no hint is specified, the engine uses its default date input control — the hint is an optimization, not a requirement.

### 17.6.3 File and Camera Inputs

`field.file_upload` on iOS uses `PhotosPicker` for images and `UIDocumentPickerViewController` for documents. On Android, it uses `ActivityResultContracts.GetContent` and `ActivityResultContracts.TakePicture`.

Both platforms handle the permission request lifecycle automatically — the rendering engine requests the appropriate system permission (`NSPhotoLibraryUsageDescription` / `READ_MEDIA_IMAGES`) when the user initiates a file upload.

---

## 17.7 Navigation Rendering on Mobile

Navigation is covered in depth in Chapter 19. This section documents mobile-specific implementation details.

On iOS, `nav.tab_bar` maps to a `TabView` with `.tabItem` modifiers positioned at the bottom of the screen per iOS HIG conventions. `nav.sidebar` on iPad maps to `NavigationSplitView`.

On Android, `nav.tab_bar` maps to a `NavigationBar` (Material 3) at the bottom. `nav.sidebar` maps to `PermanentNavigationDrawer` on large screens and `ModalNavigationDrawer` on phones.

Surface-to-surface navigation uses the platform's native navigation stack — `NavigationStack` on iOS 16+ and `NavHost` + `NavController` on Android. The action dispatcher translates `action.navigate` into native navigation calls, passing route parameters as navigation arguments.

---

## 17.8 Mobile State Management

The mobile rendering engine maintains three categories of state:

**Surface-scoped state** — exists for the lifetime of a surface instance:
- State variable values (declared in the surface definition's `state` block)
- Form state (field values, dirty flags, touched flags, error messages)
- Data source loading state (loading / loaded / error, cached data)
- Navigation state (active modal, active drawer)

On iOS, surface-scoped state is managed by `@StateObject` view models tied to the surface view's lifecycle. On Android, it is managed by `ViewModel` instances scoped to the Compose navigation back stack entry.

**Session-scoped state** — persists across surface navigations within a session:
- Authentication token
- Tenant and user context
- Global notification state
- Offline action queue (in-memory)

**Persistent state** — survives app termination:
- User preferences (dashboard layouts, column visibility)
- Offline action queue (serialized to disk for retry across sessions)
- Cached UI definitions and data (with TTL)

---

## 17.9 Mobile Performance

### 17.9.1 Lazy Loading of Nested Components

Heavy components (charts, maps, PDF viewers) are marked with `loading_strategy: "lazy"` in the surface definition. Lazy components are not rendered until they scroll into the device viewport. The rendering engine uses platform intersection observers (`GeometryReader` on iOS, `LazyColumn` item visibility on Android) to trigger rendering at the appropriate moment.

### 17.9.2 Image Caching

All `ui.image` and `ui.avatar` components load images via an image caching layer:
- **iOS:** `AsyncImage` with a custom `URLCache` configuration; disk cache: 100 MB; memory cache: 50 MB
- **Android:** Coil image loading library with disk and memory cache configuration

Images are cached by URL. Cache invalidation uses standard HTTP `Cache-Control` and `ETag` headers.

### 17.9.3 List Virtualization

`ui.data_table` in card layout mode and `layout.scroll` with many children use lazy rendering:
- **iOS:** `LazyVStack` or `List` with identifier-based row reuse
- **Android:** `LazyColumn` with `items` keyed by the row's `row_key`

The engine pre-renders a buffer of items above and below the visible area (typically 3× the viewport height). Items outside the buffer are deallocated.

### 17.9.4 Surface Definition Caching

Surface definitions are cached to encrypted disk storage after each successful fetch (see Chapter 33). On subsequent visits, the cached definition is displayed immediately while a background refresh is attempted. The cache key matches the server's cache key formula:

```
surface_id + tenant_id + role_set_hash + flag_set_hash + locale + "mobile" + app_version
```

---

## 17.10 Mobile Offline Support

Offline support is covered in full in Chapter 33. The mobile rendering engine's role:

- **UI definition caching:** Surface definitions cached to encrypted disk after each successful fetch; served immediately on next open while a background refresh runs
- **Data caching:** Data source responses cached per `source_id + params_hash`; served offline until refreshed
- **Action queuing:** When `action.http_request` executes without network connectivity, the action is serialized to the offline queue rather than failing; the queue is persisted to disk and replayed in order when connectivity is restored

---

## 17.11 Mobile Accessibility

The mobile rendering engine populates accessibility attributes from component props:

| Component Prop | iOS Accessibility | Android Accessibility |
|---------------|-------------------|----------------------|
| `label` (interactive elements) | `accessibilityLabel` | `contentDescription` |
| `ui.text` content | Readable by VoiceOver | Readable by TalkBack |
| `action.button` label | `accessibilityLabel` | `contentDescription` |
| `disabled: true` | `.notEnabled` trait | `enabled = false` |
| `required: true` on fields | Appended: "(required)" | Appended to `contentDescription` |
| `aria.role` prop | `accessibilityAddTraits` | Role-based `semantics {}` |

Focus management follows platform conventions:
- **iOS:** VoiceOver natural reading order (top-to-bottom, left-to-right within each container)
- **Android:** TalkBack uses the same traversal order; `focusGroup` directs focus for complex layouts

All color choices use design tokens that maintain WCAG AA contrast ratios in both light and dark mode.

---

## 17.12 Mobile Telemetry

The mobile rendering engine emits structured telemetry events to a local buffer, flushed to the analytics pipeline every 30 seconds or on app background.

| Event | Key Properties | Purpose |
|-------|---------------|---------|
| `surface_view` | surface_id, render_time_ms, cache_hit | Surface open performance |
| `surface_render_error` | surface_id, error_code, node_id | Rendering failures |
| `unknown_component` | type_id, fallback_used, surface_id | Version skew detection |
| `action_executed` | action_id, action_type, duration_ms | Action performance |
| `action_failed` | action_id, error_code | Action failure rate |
| `offline_action_queued` | action_id, action_type | Offline usage |
| `data_source_load` | source_id, duration_ms, row_count, cache_hit | Data fetch performance |
| `form_submitted` | surface_id, field_count, validation_errors | Form funnel |
| `validation_error` | field_id, rule_type, surface_id | Common validation failures |

All events include: `tenant_id`, `user_id` (hashed), `platform`, `app_version`, `schema_version`, `session_id`, `timestamp`.

---

## 17.13 Testing Mobile Rendering

### 17.13.1 Unit Testing Component Factories

Each component factory is unit-tested in isolation, providing a mock `ResolvedNode` with known props and asserting the resulting native view has expected accessibility properties and state.

```swift
// iOS — Unit test for ButtonFactory
func testButtonFactoryPrimaryVariant() throws {
    let node = MockNode.button(label: "Submit", variant: .primary, disabled: false)
    let sut = ButtonFactory()
    let view = sut.makeView(node: node, context: .mock)
    // Assert via ViewInspector or snapshot test
    XCTAssertFalse(view.isDisabled())
}
```

### 17.13.2 Snapshot Testing

Snapshot tests capture the rendered appearance of each component in all significant states (enabled, disabled, error, loading) across light and dark mode. Snapshots are committed to the repository. CI fails on any unintended pixel deviation.

### 17.13.3 Integration Testing with Mock Definitions

Integration tests provide complete surface definition JSON to the rendering engine and assert on the resulting view hierarchy:

- **iOS:** `XCTest` + `ViewInspector` for view tree assertions; `XCUITest` for end-to-end flows
- **Android:** Compose Testing API for composable assertions; `Espresso` for end-to-end flows

### 17.13.4 Contract Testing

The rendering engine's JSON schema validation is tested against a shared set of golden definition files that are also tested by the backend compiler. This ensures that definitions the backend produces are accepted by the rendering engine without modification — the contract test suite is the authoritative integration boundary between the two teams.

---

*End of Chapter 17*

**Previous:** [Chapter 16 — ERP-Specific Components](../vol-03-component-system/13-16-dashboard-charts-workflow-erp.md)
**Next:** [Chapter 18 — Web Rendering Architecture](./18-web-rendering-architecture.md)
