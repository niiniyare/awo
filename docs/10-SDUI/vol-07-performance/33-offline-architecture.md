# Chapter 33 — Offline Architecture

> **Volume:** VII — Performance, Offline & Sync
> **Phase:** 4
> **Audience:** Mobile Engineers, Web Engineers, Backend Engineers
> **Prerequisites:** Chapters 17–21, 24, 32

---

## Table of Contents

- [33.1 Offline Philosophy](#331-offline-philosophy)
- [33.2 Offline Capability Tiers](#332-offline-capability-tiers)
- [33.3 Surface Definition Persistence](#333-surface-definition-persistence)
- [33.4 Data Persistence](#334-data-persistence)
- [33.5 Offline Action Queue](#335-offline-action-queue)
- [33.6 Conflict Detection and Resolution](#336-conflict-detection-and-resolution)
- [33.7 Sync Protocol](#337-sync-protocol)
- [33.8 Network State Management](#338-network-state-management)
- [33.9 Offline UI Patterns](#339-offline-ui-patterns)

---

## 33.1 Offline Philosophy

AwoERP targets East African businesses where mobile data connectivity is intermittent, expensive, or throttled. Offline support is not an edge case — it is a first-class requirement for mobile clients.

**The offline contract:**
1. Previously loaded surfaces remain usable without a network connection
2. Data created or modified offline is durably queued and replayed when connectivity returns
3. The user always knows whether they are online or offline, and whether unsynced changes exist
4. Conflicts (offline change + server change to the same record) are surfaced to the user, never silently overwritten

**What offline does not support:**
- Fetching new data not previously cached (requires connectivity)
- Real-time SSE updates (paused during offline; catches up on reconnect)
- Actions that explicitly require server authorization at time of execution (payment processing, approval workflows with live quorum checks)

---

## 33.2 Offline Capability Tiers

AwoERP surfaces declare their offline capability level in their definition:

```json
{
  "surface_id": "invoice_create",
  "offline": {
    "enabled": true,
    "tier": "read_write",
    "queueable_actions": ["action_save_draft", "action_submit_invoice"],
    "required_data_sources": ["vendors_list", "chart_of_accounts"]
  }
}
```

| Tier | Behavior |
|------|----------|
| `none` | Surface requires connectivity; shows offline error if unavailable |
| `read_only` | Cached surface definition and data are usable; mutations blocked with user notification |
| `read_write` | Full offline use; mutations queued and replayed on reconnect |

---

## 33.3 Surface Definition Persistence

### 33.3.1 Persistent Cache (Mobile)

On mobile, surface definitions are persisted to encrypted disk storage after first successful load:

**iOS:**
```swift
class SurfaceDefinitionCache {
    private let fileManager = FileManager.default
    private let cacheDirectory: URL

    init() {
        // NSFileProtectionComplete — encrypted at rest, inaccessible when device locked
        let baseURL = fileManager.urls(for: .documentDirectory, in: .userDomainMask).first!
        cacheDirectory = baseURL.appendingPathComponent("surface_definitions", isDirectory: true)
        try? fileManager.createDirectory(at: cacheDirectory,
                                         withIntermediateDirectories: true,
                                         attributes: [
                                             .protectionKey: FileProtectionType.complete
                                         ])
    }

    func store(definition: SurfaceDefinition, forKey key: String) throws {
        let data = try JSONEncoder().encode(definition)
        let url = cacheDirectory.appendingPathComponent("\(key).json")
        try data.write(to: url, options: .atomic)
    }

    func load(forKey key: String) -> SurfaceDefinition? {
        let url = cacheDirectory.appendingPathComponent("\(key).json")
        guard let data = try? Data(contentsOf: url) else { return nil }
        return try? JSONDecoder().decode(SurfaceDefinition.self, from: data)
    }
}
```

**Android:**
```kotlin
class SurfaceDefinitionCache(context: Context) {
    private val masterKey = MasterKey.Builder(context)
        .setKeyScheme(MasterKey.KeyScheme.AES256_GCM)
        .build()

    private val encryptedFile = EncryptedFile.Builder(
        context,
        File(context.filesDir, "surface_cache"),
        masterKey,
        EncryptedFile.FileEncryptionScheme.AES256_GCM_HKDF_4KB
    ).build()

    fun store(key: String, definition: SurfaceDefinition) {
        val json = Json.encodeToString(definition)
        encryptedFile.openFileOutput().use { it.write(json.toByteArray()) }
    }

    fun load(key: String): SurfaceDefinition? = runCatching {
        val json = encryptedFile.openFileInput().readBytes().toString(Charsets.UTF_8)
        Json.decodeFromString<SurfaceDefinition>(json)
    }.getOrNull()
}
```

### 33.3.2 Cache Validation on Reconnect

When connectivity returns, the mobile client sends a conditional GET with the cached ETag. If the server returns 304, the cached definition is still current. If the server returns 200 with a new definition, the cache is updated and the surface is re-rendered.

---

## 33.4 Data Persistence

### 33.4.1 Offline Data Store (Mobile)

Data fetched for `read_write` tier surfaces is persisted locally in an encrypted SQLite database managed by SQLCipher:

```swift
// iOS: SQLCipher-backed local store
class OfflineDataStore {
    private var db: OpaquePointer?

    func open(key: String) {
        sqlite3_open(dbPath, &db)
        sqlite3_key(db, key, Int32(key.count)) // Encryption key
    }

    // Store fetched data source results
    func storeSourceData(sourceID: String, tenantID: String,
                         data: [AnyDecodable], fetchedAt: Date) throws {
        let json = try JSONEncoder().encode(data)
        let sql = """
            INSERT OR REPLACE INTO source_cache
                (source_id, tenant_id, data, fetched_at)
            VALUES (?, ?, ?, ?)
        """
        // ... prepared statement execution
    }

    // Retrieve for offline use
    func loadSourceData(sourceID: String, tenantID: String) -> [AnyDecodable]? {
        // ... query and decode
    }
}
```

### 33.4.2 Cache Expiry for Data

Persisted data source results carry a `fetched_at` timestamp and a `cache_ttl_seconds` value from the data source declaration. On load:

```swift
func isDataStale(fetchedAt: Date, ttl: Int) -> Bool {
    let age = Date().timeIntervalSince(fetchedAt)
    return age > Double(ttl)
}

func loadDataSource(sourceID: String) -> DataSourceState {
    guard let cached = store.loadSourceData(sourceID: sourceID) else {
        return .empty
    }

    if isDataStale(fetchedAt: cached.fetchedAt, ttl: source.cacheTTLSeconds) && isOnline {
        // Stale but online — refresh in background, serve stale immediately
        refreshInBackground(sourceID: sourceID)
    }

    return .loaded(cached.data)
}
```

---

## 33.5 Offline Action Queue

### 33.5.1 Queue Declaration

Actions marked `offline_queueable: true` in the surface definition (Chapter 20) are eligible for offline queuing:

```json
{
  "id": "action_save_draft",
  "type": "action.http_request",
  "method": "POST",
  "endpoint": "/api/invoices",
  "body": { ... },
  "offline_queueable": true,
  "offline_label": "Save draft invoice"
}
```

### 33.5.2 Queue Storage

The offline action queue is persisted to encrypted local storage. Each queued item includes everything needed to replay the action when connectivity returns:

```swift
struct QueuedAction: Codable {
    let id: String                   // Unique queue item ID
    let tenantID: String
    let userID: String
    let actionID: String
    let method: String
    let endpoint: String
    let headers: [String: String]
    let body: Data?
    let queuedAt: Date
    let attemptCount: Int
    let lastAttemptAt: Date?
    let idempotencyKey: String       // UUID generated at queue time
    let surfaceID: String
    let actionLabel: String          // Human-readable for UI
}
```

The `idempotencyKey` is generated when the action is queued. When the action is replayed, this key is sent as `X-Idempotency-Key`. If the network dropped after the server received the request but before the client got the response, the server returns the original response rather than creating a duplicate.

### 33.5.3 Queue Execution on Reconnect

```swift
class OfflineQueueExecutor {
    func replayQueue(for tenantID: String) async {
        let queue = await queueStore.load(tenantID: tenantID)
        guard !queue.isEmpty else { return }

        // Notify UI that sync is in progress
        await MainActor.run {
            syncState.isSyncing = true
            syncState.pendingCount = queue.count
        }

        for item in queue {
            do {
                let result = try await execute(item)
                await queueStore.remove(item.id)
                await notifySuccess(item, result: result)
            } catch let error as HTTPError where error.isRetryable {
                await queueStore.incrementAttempt(item.id)
                // Retry later with exponential backoff
            } catch {
                // Non-retryable error: remove from queue, notify user
                await queueStore.remove(item.id)
                await notifyFailure(item, error: error)
            }
        }

        await MainActor.run {
            syncState.isSyncing = false
            syncState.pendingCount = 0
        }
    }
}
```

### 33.5.4 Queue Ordering

The queue replays actions in FIFO order. Actions within a surface session are replayed in the order they were queued. If action B depends on the result of action A (e.g., "create invoice" then "add line item to invoice"), the surface definition must declare them as sequential actions with `result_var` chaining — the queue preserves this ordering.

---

## 33.6 Conflict Detection and Resolution

### 33.6.1 Optimistic Locking

Resources that support offline writes include an `updated_at` timestamp. When replaying an offline mutation, the client sends the `updated_at` of the record as it was when the user edited it:

```json
{
  "invoice_number": "INV-2024-001",
  "amount": 150000,
  "_if_unmodified_since": "2024-03-15T08:00:00Z"
}
```

If the server detects that the record was modified after `_if_unmodified_since`, it returns:

```http
HTTP/1.1 409 Conflict
Content-Type: application/json

{
  "error": "conflict",
  "message": "This record was modified while you were offline.",
  "server_version": { ... current record ... },
  "your_changes": { ... the client's changes ... }
}
```

### 33.6.2 Conflict Resolution UI

When a 409 is received during queue replay, the queue executor pauses that item and presents a conflict resolution surface to the user. The surface shows the current server state alongside the user's offline changes and offers three options:

- **Keep mine** — overwrite the server version with the user's changes (force update)
- **Keep server** — discard the user's changes
- **Review diff** — open a field-by-field diff view for manual merge

The conflict resolution surface is a built-in platform surface (`_conflict_resolution`) that is always available, even for offline sessions.

---

## 33.7 Sync Protocol

### 33.7.1 Sync Triggers

Queue replay is triggered by:
1. Network connectivity restoration (monitored via `NWPathMonitor` on iOS, `ConnectivityManager` on Android)
2. App foreground event (in case connectivity was restored while the app was backgrounded)
3. Manual "Sync now" action from the offline indicator

### 33.7.2 SSE Catch-Up

When the SSE connection re-establishes after an offline period, the `Last-Event-ID` header causes the server to replay any events missed during the outage (up to the server's buffer of 1,000 events). Events beyond the buffer trigger a `full_reload_required` event:

```go
// Server replays from last-event-id
func (h *StreamHandler) replayMissedEvents(ctx context.Context,
    lastEventID string, w http.ResponseWriter) error {

    events, err := h.buffer.GetAfter(ctx, lastEventID)
    if err == ErrBufferExceeded {
        // Too many missed events — tell client to reload
        writeSSEEvent(w, SSEEvent{
            Type: "full_reload_required",
            Data: `{"reason": "buffer_exceeded"}`,
        })
        return nil
    }

    for _, event := range events {
        writeSSEEvent(w, event)
    }
    return nil
}
```

---

## 33.8 Network State Management

### 33.8.1 Network Monitor (iOS)

```swift
class NetworkMonitor: ObservableObject {
    private let monitor = NWPathMonitor()
    @Published var isConnected = true
    @Published var connectionType: ConnectionType = .unknown

    func start() {
        monitor.pathUpdateHandler = { [weak self] path in
            DispatchQueue.main.async {
                self?.isConnected = path.status == .satisfied
                self?.connectionType = path.usesInterfaceType(.wifi) ? .wifi :
                                       path.usesInterfaceType(.cellular) ? .cellular : .unknown
            }
        }
        monitor.start(queue: DispatchQueue(label: "NetworkMonitor"))
    }
}
```

### 33.8.2 Network Monitor (Android)

```kotlin
class NetworkMonitor(context: Context) {
    private val connectivityManager =
        context.getSystemService(Context.CONNECTIVITY_SERVICE) as ConnectivityManager

    val isConnected: StateFlow<Boolean> = callbackFlow {
        val callback = object : ConnectivityManager.NetworkCallback() {
            override fun onAvailable(network: Network) { trySend(true) }
            override fun onLost(network: Network) { trySend(false) }
        }
        val request = NetworkRequest.Builder()
            .addCapability(NetworkCapabilities.NET_CAPABILITY_INTERNET)
            .build()
        connectivityManager.registerNetworkCallback(request, callback)
        awaitClose { connectivityManager.unregisterNetworkCallback(callback) }
    }.stateIn(scope, SharingStarted.Eagerly,
              connectivityManager.activeNetwork != null)
}
```

---

## 33.9 Offline UI Patterns

### 33.9.1 Offline Indicator

A persistent banner appears at the top of all surfaces when the device is offline:

```json
{
  "type": "banner",
  "variant": "warning",
  "message": "You are offline. Changes will sync when you reconnect.",
  "show_if": "$expr{ !$session.isOnline }",
  "dismissible": false
}
```

### 33.9.2 Pending Changes Indicator

When the offline action queue is non-empty, a badge on the navigation bar shows the count of pending changes. Tapping it opens the `_pending_changes` surface listing all queued actions with their status.

### 33.9.3 Disabled Actions in Offline State

Actions that are not `offline_queueable` display as disabled with a tooltip when the device is offline:

```json
{
  "type": "button",
  "label": "Approve",
  "on_click_action_id": "action_approve",
  "disabled_if": "$expr{ !$session.isOnline }",
  "disabled_reason": "Approval requires an internet connection"
}
```

### 33.9.4 Sync Status After Reconnect

After a successful queue replay, the surfaces that were affected by replayed actions are automatically refreshed via `action.invalidate_data_source`. A toast summarizes the sync result:

- "3 drafts synced successfully"
- "1 draft synced; 1 conflict needs your review"

---

*End of Chapter 33*

**Previous:** [Chapter 32 — Performance Architecture](./32-performance-architecture.md)
**Next:** [Chapter 34 — Caching Strategy](./34-caching-strategy.md)
