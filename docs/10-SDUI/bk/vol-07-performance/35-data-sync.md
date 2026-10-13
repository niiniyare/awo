# Chapter 35 — Data Sync

> **Volume:** VII — Performance, Offline & Sync
> **Phase:** 4
> **Audience:** Backend Engineers, Mobile Engineers, Web Engineers
> **Prerequisites:** Chapters 21, 33, 34

---

## Table of Contents

- [35.1 Sync Architecture Overview](#351-sync-architecture-overview)
- [35.2 Server-to-Client Sync (SSE Patch Protocol)](#352-server-to-client-sync-sse-patch-protocol)
- [35.3 Client-to-Server Sync (Mutation Replay)](#353-client-to-server-sync-mutation-replay)
- [35.4 Background Sync (Mobile)](#354-background-sync-mobile)
- [35.5 Selective Sync](#355-selective-sync)
- [35.6 Sync State Machine](#356-sync-state-machine)
- [35.7 Sync Conflict Handling](#357-sync-conflict-handling)
- [35.8 Sync Telemetry](#358-sync-telemetry)

---

## 35.1 Sync Architecture Overview

Data sync in AwoERP is directional:

- **Server → Client:** The SSE patch protocol pushes incremental updates to active sessions. When a record changes on the server (due to another user's action, a workflow completing, or a background job), the client's rendered data is updated without polling.

- **Client → Server:** Mutations travel via HTTP requests (online) or via the offline action queue (offline). The server is always the authoritative data store.

There is no peer-to-peer sync. There is no client-side database that diverges from the server and must be reconciled — the client's local data store is a cache of server data, not a replica with its own write authority.

```
Server (PostgreSQL)
    │
    ├─ SSE stream → patch documents → Client renders update
    │
    └─ HTTP mutations ← Client action execution
         │
         └─ Offline queue replay (on reconnect)
```

---

## 35.2 Server-to-Client Sync (SSE Patch Protocol)

### 35.2.1 Patch Generation

When a resource mutation occurs on the server, the mutation handler emits a patch event to the relevant SSE channels:

```go
func (s *InvoiceService) ApproveInvoice(ctx context.Context, invoiceID string) error {
    // 1. Perform the mutation
    updated, err := s.store.ApproveInvoice(ctx, invoiceID)
    if err != nil {
        return err
    }

    // 2. Determine which surfaces display this invoice
    affectedSurfaces, err := s.surfaceIndex.SurfacesForResource("invoice", invoiceID)
    if err != nil {
        slog.WarnContext(ctx, "could not resolve affected surfaces", "error", err)
        // Non-fatal: SSE patch is best-effort; client will catch up on next fetch
    }

    // 3. Emit patch to each surface's SSE channel
    for _, surfaceID := range affectedSurfaces {
        patch := buildInvoicePatch(updated)
        channel := fmt.Sprintf("surface:%s:tenant:%s", surfaceID, updated.TenantID)
        s.pubsub.Publish(ctx, channel, SSEEvent{
            Type:    "patch",
            Payload: patch,
        })
    }

    return nil
}
```

### 35.2.2 Patch Document Structure

A patch document describes the minimal change to update the client's rendered state:

```json
{
  "version": 1248,
  "base_version": 1247,
  "operations": [
    {
      "op": "set_props",
      "path": "$.data_sources.invoices_pending.rows[id=inv-001]",
      "props": {
        "status": "approved",
        "approved_by": "Aisha Mohamed",
        "approved_at": "2024-03-15T14:32:00Z"
      }
    },
    {
      "op": "remove",
      "path": "$.data_sources.approval_queue.rows[id=inv-001]"
    }
  ]
}
```

The patch path uses a JSONPath-like selector. Supported selectors:
- `$.data_sources.{source_id}.rows[id={row_id}]` — specific row by ID
- `$.data_sources.{source_id}` — entire source (triggers refetch)
- `$.state.{var_name}` — surface state variable

### 35.2.3 Patch Application (Client)

**Web (TypeScript):**
```typescript
function applyPatch(store: SurfaceStore, patch: PatchDocument): boolean {
    if (patch.base_version !== store.version) {
        // Version mismatch — patch cannot be applied safely
        // Trigger full reload
        return false;
    }

    for (const op of patch.operations) {
        switch (op.op) {
            case 'set_props':
                applySetProps(store, op.path, op.props);
                break;
            case 'insert':
                applyInsert(store, op.path, op.value, op.index);
                break;
            case 'remove':
                applyRemove(store, op.path);
                break;
            case 'replace':
                applyReplace(store, op.path, op.value);
                break;
            case 'move':
                applyMove(store, op.from, op.to);
                break;
        }
    }

    store.version = patch.version;
    return true;
}
```

If `applyPatch` returns false (version mismatch), the rendering engine performs a full data source refetch rather than leaving the UI in an inconsistent state.

### 35.2.4 Surface Index

The surface index maps resource types and IDs to the surfaces that display them. It is maintained as an in-memory index (rebuilt on startup from the surface definition store) and updated as surface definitions are published:

```go
type SurfaceIndex struct {
    // resourceType -> resourceID -> []surfaceID
    index map[string]map[string][]string
    mu    sync.RWMutex
}

func (si *SurfaceIndex) SurfacesForResource(resourceType, resourceID string) ([]string, error) {
    si.mu.RLock()
    defer si.mu.RUnlock()

    byType, ok := si.index[resourceType]
    if !ok { return nil, nil }

    // Wildcard match (surface shows all records of this type)
    wildcards := byType["*"]
    // Specific match (surface shows only this record)
    specific := byType[resourceID]

    return append(wildcards, specific...), nil
}
```

---

## 35.3 Client-to-Server Sync (Mutation Replay)

This is the offline action queue described in Chapter 33. From a sync perspective, the key properties are:

**Idempotency.** Every queued mutation carries an `X-Idempotency-Key`. The server stores the response for 24 hours; duplicate replays return the original response.

**Ordering.** Actions within a surface session are replayed in FIFO order. Cross-session ordering is not guaranteed — if a user on mobile and a user on web modify the same record offline, the optimistic locking check (Chapter 33.6) detects the conflict.

**Retry policy.** Retryable failures (5xx, network timeout) are retried with exponential backoff: 1s, 2s, 4s, 8s, 16s, 30s cap. Non-retryable failures (400, 403, 422) are dequeued immediately and the user is notified.

---

## 35.4 Background Sync (Mobile)

### 35.4.1 iOS Background App Refresh

AwoERP registers for iOS Background App Refresh to sync the offline queue and refresh critical data even when the app is not in the foreground:

```swift
// AppDelegate
func application(_ application: UIApplication,
                 performFetchWithCompletionHandler completionHandler: @escaping (UIBackgroundFetchResult) -> Void) {
    Task {
        do {
            let synced = try await SyncService.shared.performBackgroundSync()
            completionHandler(synced ? .newData : .noData)
        } catch {
            completionHandler(.failed)
        }
    }
}

// Background sync: replay queue + refresh pinned data sources
class SyncService {
    func performBackgroundSync() async throws -> Bool {
        var changed = false

        // Replay offline queue
        let replayed = try await queueExecutor.replayQueue(for: currentTenantID)
        if replayed > 0 { changed = true }

        // Refresh pinned surfaces' data sources
        for surfaceID in pinnedSurfaces {
            try await dataSourceManager.refreshAll(for: surfaceID)
            changed = true
        }

        return changed
    }
}
```

Background refresh is limited to 30 seconds of execution time by iOS. The sync service prioritizes the offline queue (mutations first) over data refreshes.

### 35.4.2 Android WorkManager

Android uses WorkManager for background sync:

```kotlin
class BackgroundSyncWorker(context: Context, params: WorkerParameters) :
    CoroutineWorker(context, params) {

    override suspend fun doWork(): Result {
        return try {
            val synced = SyncService.get(applicationContext).performSync()
            if (synced) Result.success() else Result.success()
        } catch (e: Exception) {
            if (runAttemptCount < 3) Result.retry() else Result.failure()
        }
    }
}

// Scheduled via WorkManager constraints
val syncWork = PeriodicWorkRequestBuilder<BackgroundSyncWorker>(15, TimeUnit.MINUTES)
    .setConstraints(
        Constraints.Builder()
            .setRequiredNetworkType(NetworkType.CONNECTED)
            .build()
    )
    .setBackoffCriteria(BackoffPolicy.EXPONENTIAL, 1, TimeUnit.MINUTES)
    .build()

WorkManager.getInstance(context).enqueueUniquePeriodicWork(
    "background_sync",
    ExistingPeriodicWorkPolicy.KEEP,
    syncWork
)
```

---

## 35.5 Selective Sync

Not all data is relevant to every mobile user. Selective sync limits what is pre-fetched and persisted offline to the data that matters for the specific user's role and workflow:

```json
{
  "surface_id": "invoice_list",
  "offline": {
    "enabled": true,
    "tier": "read_write",
    "sync_scope": {
      "invoices_pending": {
        "filter": { "assigned_to": "$user.id" },
        "max_rows": 200,
        "priority": "high"
      },
      "vendors_list": {
        "max_rows": 500,
        "priority": "medium"
      }
    }
  }
}
```

`high` priority sources are synced on every background sync cycle. `medium` priority sources are synced once daily or when the user explicitly opens the relevant surface online.

---

## 35.6 Sync State Machine

Each surface's sync state is a state machine with the following states:

```
idle
  │
  ├─ [connectivity lost] → offline_reading
  │                          │
  │                          └─ [user makes change] → offline_writing
  │                                                      │
  ├─ [connectivity restored] ←───────────────────────────┘
  │
  ↓
syncing
  │
  ├─ [all queue items replayed] → idle
  │
  └─ [conflict detected] → conflict_resolution
                              │
                              └─ [user resolves] → syncing
```

The sync state is surfaced to the user via the offline indicator (Chapter 33.9) and drives which actions are available in the action bar.

---

## 35.7 Sync Conflict Handling

See Chapter 33.6 for the full conflict detection and resolution protocol. From the sync perspective:

1. A 409 response during queue replay pauses the affected item
2. The user is notified and presented with the conflict resolution surface
3. The user's decision produces either a force-update request or a discard
4. The queue item is resolved (removed or replaced) and sync continues

Multiple conflicts are resolved sequentially, one at a time.

---

## 35.8 Sync Telemetry

| Event | Properties |
|-------|-----------|
| `sync.queue_replay_started` | tenant_id, queue_depth |
| `sync.queue_item_replayed` | action_id, duration_ms, attempt_count |
| `sync.queue_item_failed` | action_id, error_code, is_retryable |
| `sync.conflict_detected` | surface_id, resource_type, resource_id |
| `sync.conflict_resolved` | resolution: keep_mine / keep_server / merged |
| `sync.background_sync_completed` | synced_count, duration_ms, platform |
| `sync.sse_reconnected` | missed_events, required_full_reload |
| `sync.patch_applied` | surface_id, operation_count, duration_ms |
| `sync.patch_rejected_version_mismatch` | surface_id, expected_version, received_version |

These events feed the sync health dashboard (Chapter 39) which tracks queue depth trends, conflict rates, and background sync success rates per tenant.

---

*End of Chapter 35*

---

*End of Volume VII — Performance, Offline & Sync*

**Previous:** [Chapter 34 — Caching Strategy](./34-caching-strategy.md)
**Next:** [Chapter 36 — Internationalization](../vol-08-quality/36-internationalization.md)
