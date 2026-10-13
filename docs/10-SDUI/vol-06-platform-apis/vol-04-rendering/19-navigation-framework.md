# Chapter 19 — Navigation Framework

> **Volume:** IV — Rendering Architecture
> **Phase:** 2
> **Audience:** Platform Engineers, Mobile Engineers, Web Engineers
> **Prerequisites:** Chapters 01–18

---

## Table of Contents

- [19.1 Navigation Philosophy](#191-navigation-philosophy)
- [19.2 Navigation Node Types](#192-navigation-node-types)
- [19.3 Route System](#193-route-system)
- [19.4 Navigation Action](#194-navigation-action)
- [19.5 Deep Linking](#195-deep-linking)
- [19.6 Permission-Gated Navigation](#196-permission-gated-navigation)
- [19.7 Navigation History and Back Stack](#197-navigation-history-and-back-stack)
- [19.8 Navigation Patterns by Platform](#198-navigation-patterns-by-platform)
- [19.9 Navigation Transitions](#199-navigation-transitions)
- [19.10 Navigation Telemetry](#1910-navigation-telemetry)

---

## 19.1 Navigation Philosophy

Navigation in AwoERP is **declared by the backend** and **executed by the client**. This principle has two practical consequences:

**The backend determines what navigation is available.** The navigation chrome (sidebar, tab bar, top bar) is itself a surface definition. The routes available to a user — and the navigation items that point to them — are computed server-side from the user's roles, tenant configuration, and feature flags. A user who cannot access the Payroll module does not see a Payroll navigation item; the navigation item is not in their surface definition at all.

**The client implements navigation using its platform conventions.** `action.navigate` on iOS produces a `NavigationStack` push. The same action on web produces a history API push. On Android it triggers a `NavController.navigate()`. The navigation semantics (push a new screen, replace the current screen, present a modal) are declared in the action definition; the platform-specific implementation is the rendering engine's responsibility.

---

## 19.2 Navigation Node Types

### 19.2.1 nav.top_bar

The top navigation bar, used primarily on web and Android.

```json
{
  "type": "nav.top_bar",
  "id": "main_top_bar",
  "props": {
    "title": "$state.current_surface_title",
    "show_back_button": "$expr{ $route.can_go_back }",
    "actions": [
      {
        "id": "notifications_action",
        "icon": "bell",
        "label": "Notifications",
        "badge_count": "$data.notifications.unread_count",
        "action_id": "open_notifications"
      },
      {
        "id": "user_menu_action",
        "icon": "user_circle",
        "label": "$user.display_name",
        "action_id": "open_user_menu"
      }
    ]
  }
}
```

| Prop | Type | Description |
|------|------|-------------|
| `title` | `string \| Binding` | Bar title; typically bound to the current surface's title |
| `show_back_button` | `boolean \| Binding` | Whether to show the platform back button |
| `back_action_id` | `string` | Custom action to execute on back; defaults to native back |
| `actions` | `TopBarAction[]` | Right-side action items (icons, menus) |
| `elevation` | `"flat" \| "raised"` | Visual elevation; `"flat"` for scrolling content, `"raised"` for fixed |

### 19.2.2 nav.sidebar

The sidebar navigation container, used on web (always) and iPad (large screen mode).

```json
{
  "type": "nav.sidebar",
  "id": "main_sidebar",
  "props": {
    "logo": { "type": "ui.image", "props": { "src": "$tenant.logo_url" } },
    "groups": [
      {
        "id": "core_group",
        "label": "Core",
        "items": [
          {
            "id": "dashboard_item",
            "label": "Dashboard",
            "icon": "home",
            "route": { "surface": "dashboard_main" },
            "badge": null
          },
          {
            "id": "invoices_item",
            "label": "Invoices",
            "icon": "document_text",
            "route": { "surface": "invoices_list" },
            "badge": "$data.invoice_badges.overdue_count"
          }
        ]
      }
    ],
    "footer_items": [
      {
        "id": "settings_item",
        "label": "Settings",
        "icon": "cog",
        "route": { "surface": "settings_main" }
      }
    ]
  }
}
```

| Prop | Type | Description |
|------|------|-------------|
| `logo` | `UINode` | Logo node rendered at the top of the sidebar |
| `groups` | `NavGroup[]` | Grouped navigation items |
| `footer_items` | `NavItem[]` | Items rendered at the bottom of the sidebar (settings, help) |
| `collapsed` | `boolean` | Initial collapsed state; user preference persists this |
| `collapsible` | `boolean` | Whether the user can collapse the sidebar |

### 19.2.3 nav.tab_bar

The bottom tab bar, used on iOS (always) and Android (always). On web, `nav.tab_bar` is rendered as a `nav.sidebar` — the same definition works across platforms, with the rendering engine selecting the appropriate chrome.

```json
{
  "type": "nav.tab_bar",
  "id": "main_tab_bar",
  "props": {
    "tabs": [
      {
        "id": "home_tab",
        "label": "Home",
        "icon": "home",
        "route": { "surface": "dashboard_main" }
      },
      {
        "id": "approvals_tab",
        "label": "Approvals",
        "icon": "check_circle",
        "route": { "surface": "approvals_queue" },
        "badge_count": "$data.pending_approvals.count"
      },
      {
        "id": "more_tab",
        "label": "More",
        "icon": "ellipsis_horizontal",
        "action_id": "open_more_menu"
      }
    ]
  }
}
```

**Platform rendering:**
- **iOS:** `TabView` with `.tabItem` + system tab bar at screen bottom
- **Android:** `NavigationBar` (Material 3) at screen bottom with `NavigationBarItem`
- **Web:** Renders as `nav.sidebar` equivalent (the same `NavItem` data, different chrome)

### 19.2.4 nav.breadcrumb

Breadcrumb navigation used in web hierarchical contexts.

```json
{
  "type": "nav.breadcrumb",
  "id": "invoice_breadcrumb",
  "props": {
    "items": [
      { "label": "Invoices", "route": { "surface": "invoices_list" } },
      { "label": "$data.invoice.invoice_number" }
    ]
  }
}
```

### 19.2.5 nav.context_menu

A context menu triggered by long-press (mobile) or right-click (web).

```json
{
  "type": "nav.context_menu",
  "id": "row_context_menu",
  "props": {
    "trigger_component_id": "row_$row.id",
    "items": [
      { "label": "View Details", "action_id": "view_invoice", "icon": "eye" },
      { "label": "Edit", "action_id": "edit_invoice", "icon": "pencil",
        "$if": "$user.can('invoice:update')" },
      { "label": "Delete", "action_id": "delete_invoice", "icon": "trash",
        "$if": "$user.can('invoice:delete')", "destructive": true }
    ]
  }
}
```

---

## 19.3 Route System

### 19.3.1 Route Definition

A route identifies a surface by its `surface_id` and an optional set of parameters. Routes are not URL paths — they are semantic identifiers. The rendering engine maps route identifiers to platform-appropriate URL paths or navigation stack entries.

```typescript
interface RouteSpec {
  surface: string;           // Surface ID (e.g. "invoice_detail")
  params?: Record<string, string | number | boolean>;  // Route parameters
  replace?: boolean;         // Replace current history entry instead of push
  transition?: NavigationTransition;  // Optional transition override
}
```

**Example:**
```json
{
  "surface": "invoice_detail",
  "params": {
    "invoice_id": "$row.id",
    "mode": "view"
  }
}
```

### 19.3.2 Route Parameter Binding

Route parameters are available in the target surface as `$route.params.param_name` bindings. They can be bound to state variable initial values, data source request parameters, or directly in expression bindings.

```json
{
  "type": "data.source",
  "props": {
    "id": "invoice",
    "endpoint": "/api/invoices/$route.params.invoice_id",
    "method": "GET"
  }
}
```

### 19.3.3 Web URL Mapping

On web, the route system maps `RouteSpec` objects to browser URL paths. The mapping is configured in the shell's route table, which is itself generated from the platform's surface registry:

```
RouteSpec                          → URL path
----------------------------------------------
{ surface: "dashboard_main" }      → /dashboard
{ surface: "invoices_list" }       → /invoices
{ surface: "invoice_detail",
  params: { invoice_id: "123" } }  → /invoices/123
{ surface: "invoice_detail",
  params: { invoice_id: "123",
            mode: "edit" } }       → /invoices/123?mode=edit
```

The URL format is defined by the platform team and is stable across releases. Surface IDs are an internal detail; the URL path is the external-facing identifier.

### 19.3.4 Deep Link URL Scheme

Mobile deep links use the scheme:
```
awoerp://open?surface=<surface_id>&<param_key>=<param_value>...
```

Example:
```
awoerp://open?surface=invoice_detail&invoice_id=INV-2024-0042
```

Deep links are handled by the application shell before the app's main navigation is initialized.

---

## 19.4 Navigation Action

The `action.navigate` action type executes navigation. It is the bridge between user interactions (button taps, row taps, link clicks) and the navigation system.

```json
{
  "type": "action.navigate",
  "id": "view_invoice_action",
  "props": {
    "route": {
      "surface": "invoice_detail",
      "params": {
        "invoice_id": "$row.id"
      }
    },
    "presentation": "push"
  }
}
```

### 19.4.1 Presentation Modes

| `presentation` | Web | iOS | Android |
|---------------|-----|-----|---------|
| `"push"` (default) | History push | `NavigationStack` push | `NavController.navigate()` |
| `"replace"` | History replace | `navigationPath` replace | `NavOptions(launchSingleTop=true)` |
| `"modal"` | `<dialog>` overlay | `.sheet()` / `.fullScreenCover()` | `ModalBottomSheet` / `Dialog` |
| `"drawer"` | Right panel overlay | Custom drawer | `ModalNavigationDrawer` |
| `"tab"` | N/A | Tab switch | `NavController` tab switch |

### 19.4.2 Navigation Action with Permission Check

If the target route is permission-protected, the navigation action performs a client-side permission check before executing. This is an optimistic check — the server re-validates permissions when the target surface is fetched.

```json
{
  "type": "action.navigate",
  "id": "edit_invoice_action",
  "props": {
    "route": { "surface": "invoice_edit", "params": { "invoice_id": "$row.id" } },
    "require_permission": "invoice:update",
    "permission_denied_action_id": "show_access_denied_toast"
  }
}
```

---

## 19.5 Deep Linking

### 19.5.1 iOS Universal Links

AwoERP on iOS supports Universal Links, which route web URLs directly to the app when it is installed. The Apple App Site Association (AASA) file is served by the AwoERP backend at `/.well-known/apple-app-site-association`.

```json
{
  "applinks": {
    "apps": [],
    "details": [
      {
        "appID": "TEAMID.com.awoerp.mobile",
        "paths": ["/invoices/*", "/dashboard", "/approvals/*"]
      }
    ]
  }
}
```

### 19.5.2 Android App Links

AwoERP on Android supports Android App Links via the Digital Asset Links JSON file served at `/.well-known/assetlinks.json`.

### 19.5.3 Deep Link Routing Logic

When the app opens from a deep link:
1. The application shell intercepts the URL before any surface is loaded
2. The shell parses the URL into a `RouteSpec`
3. If the user is not authenticated: store the pending route, complete authentication, then navigate to the route
4. If the user is authenticated: navigate to the route immediately (replacing the launch screen)

```typescript
// Deep link handler (iOS AppDelegate / Android intent handler equivalent)
async function handleDeepLink(url: URL): Promise<void> {
  const route = parseDeepLink(url);
  if (!route) return; // unrecognized URL

  if (!authStore.isAuthenticated) {
    authStore.setPendingRoute(route);
    return; // auth flow will navigate to route after login
  }

  await navigationStore.navigate(route, { replace: true });
}
```

---

## 19.6 Permission-Gated Navigation

Navigation items and routes are permission-gated at two levels:

**Definition level:** The backend's compilation pipeline evaluates the user's permissions and feature flags when building the navigation surface definition. Navigation items the user cannot access are excluded from the definition entirely — they are not rendered as disabled items but are absent from the response.

**Runtime level:** When a user navigates to a route directly (via deep link, browser back button, or a URL typed manually), the target surface compilation re-evaluates permissions. If the user lacks permission, the backend returns a `403 Forbidden` response. The rendering engine displays the surface error state with a "You don't have permission to view this page" message.

This two-level approach ensures:
- The navigation chrome shows only accessible items (UX clarity)
- Direct URL access is also protected (security correctness)

---

## 19.7 Navigation History and Back Stack

### 19.7.1 Back Button Behavior

The platform's standard back behavior applies: the user returns to the previous surface. Back navigation does not re-fetch the previous surface's definition from the server — the rendering engine restores the cached definition and the previous state.

When the surface declares `on_back_action_id`, executing that action replaces the default back behavior. This is used for surfaces that have unsaved changes and need to show a confirmation dialog before leaving.

```json
{
  "type": "form.container",
  "props": {
    "on_back_action_id": "confirm_discard_changes"
  }
}
```

### 19.7.2 History Stack on Web

On web, the rendering engine uses the browser History API (`pushState`, `replaceState`, `popstate`). The history stack contains `RouteSpec` entries. The browser's standard back/forward navigation works as expected.

The shell installs a `beforeunload` handler that checks for unsaved form state and shows a browser-native confirmation dialog before the user navigates away.

### 19.7.3 Navigation Stack Reset

`action.navigate` with `presentation: "replace"` and `reset_to_root: true` clears the entire navigation stack and starts fresh at the specified route. This is used for flows that should not be back-navigatable (e.g., after completing an onboarding flow).

---

## 19.8 Navigation Patterns by Platform

### 19.8.1 Web Navigation Patterns

**Sidebar + main content area:** The primary web navigation pattern. The sidebar (`nav.sidebar`) is always visible on `md+` breakpoints. On `sm` and below, the sidebar collapses to a hamburger menu that opens as a drawer.

**Tabbed content within a surface:** `layout.tabs` provides within-surface tab navigation (e.g., an invoice detail surface with tabs for Details, Line Items, Payments, History). This is distinct from `nav.tab_bar`, which is application-level navigation.

**Modal surfaces:** Surfaces declared with `presentation: "modal"` open as an overlay dialog. They are used for creation flows, confirmation flows, and record pickers.

**Split-pane layout (master-detail):** A `layout.split` at the surface level shows a list on the left and a detail panel on the right. Selecting a row in the list updates the detail panel without full navigation. This is implemented as a single surface definition with a sub-surface binding in the detail slot.

### 19.8.2 iOS Navigation Patterns

**Navigation stack (push/pop):** The primary iOS pattern. `NavigationStack` with a path bound to the surface navigation state. Each push corresponds to a `NavigationLink` destination.

**Bottom tab bar:** `nav.tab_bar` maps to `TabView` with `.tabItem`. Tab switches do not push to the navigation stack.

**Sheets:** `presentation: "modal"` maps to `.sheet()`. Full-screen: `.fullScreenCover()`.

**iPad split view:** On iPad, `NavigationSplitView` provides sidebar + detail. The sidebar collapses on portrait orientation.

### 19.8.3 Android Navigation Patterns

**NavController + NavGraph:** All navigation uses Jetpack Navigation's `NavController`. Each surface corresponds to a `NavDestination` in the graph. Route parameters are passed as navigation arguments.

**Bottom navigation bar:** `nav.tab_bar` maps to `NavigationBar` at the bottom of the `Scaffold`.

**Bottom sheets / dialogs:** `presentation: "modal"` maps to `ModalBottomSheet` (for partial-screen content) or `AlertDialog` (for confirmation flows).

**Large screens:** On tablets, the navigation graph uses an adaptive layout that shows a `PermanentNavigationDrawer` on large screens and `ModalNavigationDrawer` on phones.

---

## 19.9 Navigation Transitions

Navigation transitions are declared in the route spec or as platform defaults.

| `transition` | Web | iOS | Android |
|-------------|-----|-----|---------|
| `"slide_left"` (default push) | CSS slide-in from right | `NavigationStack` default | `slide_in_right` + `slide_out_left` |
| `"slide_up"` (modal push) | Slide up from bottom | `.sheet()` default | `slide_in_bottom` |
| `"fade"` | CSS fade | Custom `.transition(.opacity)` | `fade_in` + `fade_out` |
| `"none"` | Instant | `.animation(nil)` | No animation |

Transitions are a presentation detail only — they do not affect navigation semantics, back button behavior, or the navigation stack state.

---

## 19.10 Navigation Telemetry

The navigation framework emits telemetry events for route transitions:

| Event | Properties | Purpose |
|-------|-----------|---------|
| `navigation_push` | from_surface, to_surface, presentation, params_keys | Navigation flow analysis |
| `navigation_pop` | from_surface, to_surface | Back navigation patterns |
| `navigation_tab_switch` | from_tab, to_tab | Tab usage patterns |
| `deep_link_opened` | url_pattern, authenticated | Deep link effectiveness |
| `navigation_permission_denied` | surface_id, user_role | Access control gaps |
| `navigation_error` | surface_id, error_code | Definition fetch failures |

Time-to-interactive for navigation pushes is measured from the moment `action.navigate` executes to the moment the target surface's first contentful paint completes. This is reported as `navigation_tti_ms` on the `navigation_push` event.

---

*End of Chapter 19*

**Previous:** [Chapter 18 — Web Rendering Architecture](./18-web-rendering-architecture.md)
**Next:** [Chapter 20 — Action System](./20-action-system.md)
