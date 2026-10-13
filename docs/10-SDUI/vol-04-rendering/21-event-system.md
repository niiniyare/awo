# Chapter 21 — Event System

> **Volume:** IV — Rendering Architecture
> **Phase:** 2
> **Audience:** Platform Engineers, Backend Engineers, Mobile Engineers, Web Engineers
> **Prerequisites:** Chapters 01–20

---

## Table of Contents

- [21.1 Event System Overview](#211-event-system-overview)
- [21.2 Component Events](#212-component-events)
- [21.3 Surface-Level Event Bus](#213-surface-level-event-bus)
- [21.4 Real-Time Updates: SSE](#214-real-time-updates-sse)
- [21.5 Real-Time Updates: WebSocket](#215-real-time-updates-websocket)
- [21.6 Real-Time Updates: gRPC Streaming](#216-real-time-updates-grpc-streaming)
- [21.7 Platform Transport Selection](#217-platform-transport-selection)
- [21.8 Patch Application](#218-patch-application)
- [21.9 Subscription Lifecycle](#219-subscription-lifecycle)
- [21.10 Event Security](#2110-event-security)
- [21.11 Event Telemetry](#2111-event-telemetry)

---

## 21.1 Event System Overview

The AwoERP event system handles two distinct problems:

**Intra-surface communication:** Components within the same surface need to communicate. A button click in the toolbar should refresh a data table. A row selection in one panel should update a details panel in another. These interactions are declared as component events bound to action IDs.

**Real-time server-to-client updates:** Backend changes need to reach the client without a full page reload. An approval workflow completing, a purchase order being updated by a colleague, a stock level dropping below threshold — these are server-originated events that the client's surface should reflect. The real-time update system delivers these changes as patch documents applied to the live view.

These two problems have different solutions and different scopes, but they share the same action execution path on the client side: a real-time update that triggers a UI refresh ultimately calls `action.invalidate_data_source` or applies a UI patch through the same view reconciler used by synchronous state updates.

---

## 21.2 Component Events

Every component type defines a set of events it can emit. These events are declared in the component's `events` schema (Chapter 9, Section 9.3.5). Component authors declare event handlers in their surface definition by binding event names to action IDs.

### 21.2.1 Standard Component Events

| Component | Event | Emitted When |
|-----------|-------|-------------|
| `action.button` | `on_click` | Button is tapped/clicked |
| `field.*` | `on_change` | Field value changes |
| `field.*` | `on_blur` | Field loses focus |
| `field.*` | `on_focus` | Field gains focus |
| `form.container` | `on_submit` | Form is submitted |
| `form.container` | `on_validation_error` | Submission blocked by validation errors |
| `ui.data_table` | `on_row_click` | A row is tapped/clicked |
| `ui.data_table` | `on_selection_change` | Row selection changes |
| `ui.data_table` | `on_sort_change` | Column sort changes |
| `ui.data_table` | `on_filter_change` | Column filter changes |
| `layout.tabs` | `on_tab_change` | Active tab changes |
| `dashboard.widget` | `on_refresh` | Widget refresh triggered |
| `workflow.approval_action_bar` | `on_action` | An approval action is submitted |

### 21.2.2 Binding Events to Actions

Events are bound to action IDs in the component's `events` prop:

```json
{
  "type": "ui.data_table",
  "id": "invoices_table",
  "props": {
    "data_source_id": "invoices_list",
    "events": {
      "on_row_click": "open_invoice_detail",
      "on_selection_change": "update_bulk_action_bar"
    }
  }
}
```

When the component emits an event, the rendering engine:
1. Looks up the action ID in the surface's `actions` map
2. Populates the event context into the appropriate binding scope (e.g., `$row` for row events)
3. Executes the action chain

### 21.2.3 Event Payload Scope

Each event type defines what data it provides to the action chain:

| Event | Scope populated |
|-------|----------------|
| `on_row_click` | `$row` — the clicked row's data |
| `on_selection_change` | `$selection.ids`, `$selection.rows`, `$selection.count` |
| `on_tab_change` | `$event.tab_id`, `$event.previous_tab_id` |
| `on_change` (field) | `$event.value`, `$event.previous_value`, `$event.field_id` |
| `on_filter_change` | `$event.filters` — current active filter set |

---

## 21.3 Surface-Level Event Bus

The surface-level event bus enables decoupled communication between components that are not in a parent-child relationship. A component emits a named event on the bus; any other component on the same surface (or the surface root) can subscribe to it.

### 21.3.1 Emitting Bus Events

An action step can emit a named event on the surface bus:

```json
{
  "type": "action.emit_event",
  "props": {
    "event_name": "vendor_selected",
    "payload": {
      "vendor_id": "$row.id",
      "vendor_name": "$row.name"
    }
  }
}
```

### 21.3.2 Subscribing to Bus Events

Components and the surface root can declare subscriptions:

```json
{
  "surface": {
    "event_subscriptions": [
      {
        "event_name": "vendor_selected",
        "action_id": "load_vendor_details"
      },
      {
        "event_name": "vendor_selected",
        "action_id": "update_po_vendor_field"
      }
    ]
  }
}
```

Multiple subscribers can listen to the same event. Subscriptions execute in declaration order.

### 21.3.3 Event Payload in Subscribed Actions

When a subscribed action chain executes, the event payload is available in the `$event` scope:

```json
{
  "id": "load_vendor_details",
  "chain": [
    {
      "type": "action.set_state",
      "props": {
        "assignments": [
          { "var": "selected_vendor_id", "value": "$event.vendor_id" }
        ]
      }
    },
    {
      "type": "action.invalidate_data_source",
      "props": { "source_ids": ["vendor_details"] }
    }
  ]
}
```

### 21.3.4 Cross-Surface Events

Events cannot cross surface boundaries through the bus — the event bus is scoped to a single surface instance. Communication between surfaces is achieved through route parameters (when navigating to a new surface) or modal result payloads (when opening/closing a modal surface).

---

## 21.4 Real-Time Updates: SSE

Server-Sent Events (SSE) is the primary real-time transport for web clients. SSE provides a persistent HTTP connection over which the server can push events to the client. It uses the standard `EventSource` API in the browser.

### 21.4.1 SSE Connection Lifecycle

```
Client                          Server
  │                                │
  │──── GET /api/stream/surfaces/{surface_id} ────►│
  │     Headers:                                   │
  │     Accept: text/event-stream                  │
  │     Authorization: Bearer {token}              │
  │     X-Tenant-ID: {tenant_id}                   │
  │     Last-Event-ID: {last_event_id}             │
  │                                                │
  │◄──── HTTP 200, text/event-stream ──────────────│
  │◄──── event: patch, id: 1234, data: {...} ──────│
  │◄──── event: data_refresh, id: 1235, data: {..}─│
  │◄──── : keep-alive comment (every 30s) ─────────│
  │                                                │
  │  [network interruption]                        │
  │                                                │
  │──── GET /api/stream/surfaces/{surface_id} ────►│
  │     Last-Event-ID: 1235                        │
  │◄──── (resumes from event 1236) ────────────────│
```

**Automatic reconnect:** The `EventSource` API handles reconnection automatically. The `Last-Event-ID` header ensures the server resumes from where it left off. Events with IDs allow the server to deliver any missed events after a reconnect.

### 21.4.2 SSE Event Types

| Event name | When emitted | Client action |
|-----------|-------------|---------------|
| `patch` | A UI patch document is available | Apply patch to view tree (Section 21.8) |
| `data_refresh` | A data source has new data | Invalidate specified data sources |
| `state_update` | A server-side state variable has changed | Update specified state variables |
| `toast` | Server wants to show a toast notification | Show toast |
| `redirect` | Server requires the client to navigate | Execute `action.navigate` |
| `session_expired` | Session has expired | Show session expiry UI |
| `permission_changed` | User's permissions have changed | Reload the surface definition |

### 21.4.3 Backend SSE Infrastructure

The backend SSE endpoint is a long-lived HTTP handler in Go. It uses Redis Pub/Sub to receive events for the connected surface from the relevant backend services (workflow engine, API handlers, scheduled jobs).

```go
// Backend SSE handler (simplified)
func (h *SSEHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    surfaceID := chi.URLParam(r, "surfaceID")
    tenantID := middleware.TenantFromCtx(r.Context())
    userID := middleware.UserFromCtx(r.Context())

    // Validate subscription permission
    if err := h.authz.CanSubscribe(r.Context(), userID, surfaceID); err != nil {
        http.Error(w, "Forbidden", http.StatusForbidden)
        return
    }

    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

    flusher := w.(http.Flusher)
    channel := fmt.Sprintf("surface:%s:tenant:%s:user:%s", surfaceID, tenantID, userID)
    sub := h.redis.Subscribe(r.Context(), channel)
    defer sub.Close()

    keepAlive := time.NewTicker(30 * time.Second)
    defer keepAlive.Stop()

    for {
        select {
        case msg := <-sub.Channel():
            fmt.Fprintf(w, "event: %s\ndata: %s\nid: %s\n\n",
                msg.EventType, msg.Data, msg.EventID)
            flusher.Flush()
        case <-keepAlive.C:
            fmt.Fprintf(w, ": keep-alive\n\n")
            flusher.Flush()
        case <-r.Context().Done():
            return
        }
    }
}
```

### 21.4.4 SSE Scale

SSE connections are stateless from the perspective of the application logic — each connection is an HTTP request served by any backend pod. The event delivery goes through Redis Pub/Sub, so any backend pod can publish to any connected client. The number of simultaneous SSE connections is bounded by the reverse proxy (nginx) and the pod's goroutine capacity. A conservative estimate is 10,000 connections per backend pod.

---

## 21.5 Real-Time Updates: WebSocket

WebSockets are used for bidirectional, low-latency communication scenarios — primarily the real-time collaboration features (concurrent editing indicators, live cursor positions) and the terminal/CLI surfaces (Chapter 50). For the standard real-time update use case (server pushes data changes to the client), SSE is preferred.

### 21.5.1 When to Use WebSocket

| Scenario | Transport | Reason |
|----------|-----------|--------|
| Server pushing data refreshes | SSE | One-directional; simpler; auto-reconnect |
| UI patch delivery | SSE | One-directional; auto-reconnect |
| Collaborative editing presence | WebSocket | Bidirectional (client sends cursor position) |
| Live terminal output | WebSocket | Bidirectional (client sends keystrokes) |
| High-frequency chart data | WebSocket | Faster than SSE for sub-second updates |

### 21.5.2 WebSocket Message Format

```typescript
interface WebSocketMessage {
  type: 'patch' | 'data_refresh' | 'presence' | 'command' | 'ack';
  id: string;        // Message ID for deduplication
  payload: unknown;  // Type-specific payload
  ts: number;        // Server timestamp (ms since epoch)
}
```

### 21.5.3 WebSocket Connection Management

The WebSocket connection is managed by the `RealtimeManager` in the rendering engine. It handles:
- Connection establishment with exponential backoff reconnect
- Message deduplication by `id`
- Heartbeat (`ping`/`pong`) to detect silent disconnections
- Graceful degradation to SSE if WebSocket is unavailable

---

## 21.6 Real-Time Updates: gRPC Streaming

The mobile rendering engines use gRPC server streaming for real-time updates instead of SSE. gRPC provides a more efficient binary protocol and better integration with the existing gRPC API surface.

### 21.6.1 gRPC Streaming Endpoint

```protobuf
service SurfaceService {
  // Subscribe to real-time updates for a surface
  rpc SubscribeSurface(SurfaceSubscribeRequest)
      returns (stream SurfaceEvent);
}

message SurfaceSubscribeRequest {
  string surface_id = 1;
  string last_event_id = 2;  // Resume from this event ID
}

message SurfaceEvent {
  string id = 1;
  string type = 2;       // "patch" | "data_refresh" | "toast" | etc.
  string payload = 3;    // JSON payload
  int64 timestamp = 4;   // Server timestamp
}
```

### 21.6.2 Mobile gRPC Stream Client

```swift
// iOS — gRPC streaming client
class SurfaceStreamClient: ObservableObject {
    private var call: GRPCAsyncServerStreamingCall<
        SurfaceSubscribeRequest, SurfaceEvent
    >?

    func subscribe(surfaceID: String, lastEventID: String?) async {
        let request = SurfaceSubscribeRequest.with {
            $0.surfaceID = surfaceID
            $0.lastEventID = lastEventID ?? ""
        }

        call = client.makeSubscribeSurfaceCall(request)

        do {
            for try await event in call!.responseStream {
                await handleEvent(event)
            }
        } catch {
            // Reconnect with exponential backoff
            await reconnect(surfaceID: surfaceID, delay: nextBackoff())
        }
    }

    private func handleEvent(_ event: SurfaceEvent) async {
        switch event.type {
        case "patch":
            await viewReconciler.applyPatch(decode(event.payload))
        case "data_refresh":
            await dataSourceManager.invalidate(decode(event.payload).sourceIDs)
        case "toast":
            await toastManager.show(decode(event.payload))
        default:
            telemetry.record(.unknownEventType(event.type))
        }
    }
}
```

---

## 21.7 Platform Transport Selection

| Platform | Primary Transport | Fallback |
|----------|------------------|---------|
| Web (modern browser) | SSE | Long-polling |
| Web (collaborative surfaces) | WebSocket | SSE |
| iOS | gRPC streaming | SSE (via URLSession) |
| Android | gRPC streaming | SSE (via OkHttp) |

The rendering engine auto-selects the transport based on the platform and the surface's real-time requirements (declared in the surface definition's `realtime` block):

```json
{
  "surface": {
    "realtime": {
      "enabled": true,
      "subscriptions": [
        { "event_type": "data_refresh", "source_ids": ["invoices_list", "invoice_summary"] },
        { "event_type": "patch", "target_node_ids": ["approval_status_badge"] }
      ]
    }
  }
}
```

Surfaces that do not declare a `realtime` block do not establish any streaming connection — their data is refreshed only by explicit `action.invalidate_data_source` calls or manual refresh.

---

## 21.8 Patch Application

When the rendering engine receives a `patch` event, it applies the patch document to the current view tree using the view reconciler (Chapter 17, Section 17.3.5 and Chapter 18, Section 18.3.1).

### 21.8.1 Patch Document Format

```typescript
interface PatchDocument {
  version: number;         // The version this patch applies to
  base_version: number;    // The version this patch is relative to
  operations: PatchOperation[];
}

type PatchOperation =
  | { op: 'set_props'; node_id: string; props: Record<string, unknown> }
  | { op: 'insert'; parent_id: string; index: number; node: NodeDefinition }
  | { op: 'remove'; node_id: string }
  | { op: 'replace'; node_id: string; node: NodeDefinition }
  | { op: 'move'; node_id: string; parent_id: string; index: number };
```

### 21.8.2 Version Tracking

Each surface definition has a `version` number. Patches carry `version` (the version the patch produces) and `base_version` (the version the patch applies to). The rendering engine tracks its current version and rejects patches whose `base_version` does not match.

When a patch is rejected due to version mismatch (the client is too far behind), the engine performs a full surface reload instead.

### 21.8.3 Patch Application Algorithm

```
1. Check base_version matches current view tree version
   - If mismatch: queue patch, trigger full reload
   - If match: proceed

2. For each operation in order:
   a. SET_PROPS: Find node by id; merge new props; trigger reactive update
   b. INSERT: Find parent by id; create new node; insert at index; mount component
   c. REMOVE: Find node by id; unmount component; release resources
   d. REPLACE: Find node by id; unmount; replace with new node definition; mount
   e. MOVE: Find node; detach from current parent; attach to new parent at index

3. Update current view tree version to patch.version

4. Emit telemetry: patch_applied { operation_count, version }
```

---

## 21.9 Subscription Lifecycle

### 21.9.1 Subscription Establishment

The rendering engine establishes the real-time subscription after the surface definition is loaded and the initial data sources have completed their first fetch. This ordering ensures:
- Patches arrive for a fully-initialized surface (not a partially-loaded one)
- The `base_version` for the first patch matches the loaded definition's version

### 21.9.2 Subscription Teardown

The subscription is torn down when:
- The surface is unmounted (user navigates away)
- The surface is replaced (full reload)
- The session expires

The rendering engine sends a graceful close on teardown. The server closes its side of the connection and releases the Redis subscription.

### 21.9.3 Reconnection Strategy

All real-time transports implement exponential backoff reconnection:

```
Attempt 1: 1s delay
Attempt 2: 2s delay
Attempt 3: 4s delay
Attempt 4: 8s delay
Attempt 5: 16s delay
Attempt 6+: 30s delay (cap)
```

During reconnection, the surface continues to function — it just does not receive real-time updates. A reconnecting indicator is shown in the shell status area. After reconnect, the engine includes the `Last-Event-ID` (SSE) or `last_event_id` (gRPC) from the last successfully processed event, allowing the server to deliver any events missed during the disconnection.

### 21.9.4 Missed Event Recovery

If more events were missed than the server's event buffer retains (configurable; default: 1,000 events), the server sends a `full_reload_required` event instead of the missed events. The rendering engine performs a full surface reload in response.

---

## 21.10 Event Security

### 21.10.1 Authentication

All real-time connections carry the user's authentication token:
- SSE: `Authorization: Bearer {token}` header
- WebSocket: `Authorization: Bearer {token}` header in the handshake request
- gRPC: `authorization: Bearer {token}` metadata in the `SubscribeSurface` call

The backend validates the token on connection establishment and on each reconnect.

### 21.10.2 Authorization Scoping

The server only publishes events to a connection that the connected user is authorized to receive. Before publishing a patch or data refresh event, the backend checks that the user retains the permission to view the affected surface and data. Events are never broadcast indiscriminately — they are routed to specific `surface:tenant:user` channels.

### 21.10.3 Tenant Isolation

Real-time channels are namespaced by tenant ID. A tenant's events cannot be delivered to another tenant's clients. The channel naming convention is:

```
surface:{surface_id}:tenant:{tenant_id}:user:{user_id}
```

There are no cross-tenant channels. Events that affect multiple tenants (e.g., a platform-wide maintenance notice) are published to each tenant's channel individually.

### 21.10.4 Permission Change Handling

When a user's permissions change while they have an active real-time subscription (e.g., they are removed from a role mid-session), the server publishes a `permission_changed` event to their active connections. The rendering engine reloads the surface definition on receiving this event. If the user no longer has permission to view the surface, the backend returns 403 on the reload request, and the rendering engine displays the access denied state.

---

## 21.11 Event Telemetry

| Event | Properties | Purpose |
|-------|-----------|---------|
| `stream_connected` | surface_id, transport, latency_ms | Connection establishment |
| `stream_reconnected` | surface_id, transport, attempt_number, downtime_ms | Reconnect success |
| `stream_disconnected` | surface_id, transport, reason | Disconnect analysis |
| `event_received` | surface_id, event_type, event_id | Event delivery rate |
| `patch_applied` | surface_id, operation_count, version, apply_time_ms | Patch performance |
| `patch_rejected` | surface_id, expected_version, received_version | Version mismatch rate |
| `full_reload_triggered` | surface_id, trigger_reason | Recovery frequency |
| `missed_events` | surface_id, count, transport | Event buffer sufficiency |
| `unknown_event_type` | surface_id, event_type | Protocol version gaps |

Event throughput (events received per minute per surface) is tracked as a time-series metric to detect anomalous backend activity (event storms) and to right-size Redis Pub/Sub buffer configuration.

---

*End of Chapter 21*

**Previous:** [Chapter 20 — Action System](./20-action-system.md)
**Next:** [Chapter 22 — Client State Management](../vol-05-state-and-validation/22-client-state-management.md)

---

## Volume IV Summary

Volume IV has covered the complete rendering layer of the AwoERP SDUI platform across five chapters:

| Chapter | Topic | Key Coverage |
|---------|-------|-------------|
| 17 | Mobile Rendering Architecture | iOS SwiftUI + Android Compose engines, component registry, native mapping, form keyboard handling, offline caching |
| 18 | Web Rendering Architecture | React 18 + TypeScript engine, Jotai reactive bindings, CSS Grid/Flex layout, react-hook-form integration, shell integration |
| 19 | Navigation Framework | Nav node types (top_bar, sidebar, tab_bar, breadcrumb), route system, deep linking, permission-gated navigation, platform-specific patterns |
| 20 | Action System | All 15 action types with full schemas, chain execution, optimistic updates, error handling ($error scope), offline action queue |
| 21 | Event System | Component events, surface bus, SSE/WebSocket/gRPC streaming transports, patch application algorithm, subscription lifecycle, event security |

The rendering layer is the rendering engine's public contract with the backend: it faithfully executes what the backend specifies, using the best available native controls on each platform, and reports back through telemetry so the platform team can observe usage and detect issues.
