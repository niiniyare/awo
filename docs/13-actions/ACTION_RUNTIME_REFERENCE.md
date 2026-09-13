# ActionRuntime Reference

**Classification:** Reference — Tier 1
**Owner:** `13-actions/ACTION_RUNTIME_REFERENCE.md`
**Status:** Frozen at v1.0
**Package:** `awo.so/awo/def`

---

## Purpose

This document is the exhaustive reference for the `ActionRuntime` interface — the execution environment injected into every `ActionHandlerFunc`. All 11 methods are specified here with their contracts, error conditions, and behavioural guarantees.

---

## Interface Declaration

```go
// Package: awo.so/awo/def

type ActionRuntime interface {
    Repo(entityName string) ActionEntityRepo
    Tx(ctx context.Context, fn func(ctx context.Context) error) error
    Publish(ctx context.Context, event ActionEvent) error
    StartWorkflow(ctx context.Context, spec ActionWorkflowSpec) (workflowID string, err error)
    Notify(ctx context.Context, n ActionNotification) error
    InvalidateCache(ctx context.Context, entityName string) error
    Cache() ActionCache
    Clock() time.Time
    Logger() *slog.Logger
    TenantID() uuid.UUID
    Actor() *Actor
}
```

The runtime is constructed per-request, scoped to one tenant, and permission-checked before handler invocation. It MUST NOT be retained beyond the handler's return.

---

## Method Reference

### `Repo(entityName string) ActionEntityRepo`

Returns a tenant-scoped, permission-aware repository for the named entity.

**Parameters:**
- `entityName`: fully-qualified entity name (e.g. `"finance_invoice"`). Local names (e.g. `"invoice"`) are NOT accepted.

**Behaviour:**
- Applies `set_tenant_context()` automatically on every DB call.
- Enforces `PermissionSet` for the current actor on read/write operations.
- Returns the same `ActionEntityRepo` on repeated calls with the same `entityName`.

**Panics:** if `entityName` is not registered in the schema. This is a programming error caught at development time.

---

### `Tx(ctx context.Context, fn func(ctx context.Context) error) error`

Executes `fn` inside a database transaction.

**Behaviour:**
- The `ctx` passed to `fn` carries the open transaction.
- If `fn` returns a non-nil error, the transaction rolls back and `Tx` returns that error.
- `StartWorkflow` calls issued inside `fn` are deferred to after the transaction commits — they do NOT trigger on rollback.
- `Publish` calls inside `fn` write to `event_outbox` within the same TX — they roll back if `fn` fails.
- Nested `Tx` calls use savepoints (PostgreSQL `SAVEPOINT`), not nested transactions.

**Error returns:** the error returned by `fn`, or a database error if the TX commit fails.

---

### `Publish(ctx context.Context, event ActionEvent) error`

Emits a domain event via the event outbox.

**Parameters:**
- `event.Topic`: routing key, e.g. `"finance.invoice.submitted"`. Must follow the `{module}.{entity}.{verb}` convention.
- `event.Payload`: JSON-serialisable value. MUST NOT contain sensitive fields.
- `event.TenantID`: set automatically by the runtime when left zero.

**Behaviour:**
- Writes to `event_outbox` inside the current transaction (if inside `Tx`), or in its own mini-transaction (if called outside `Tx`).
- Returns immediately — delivery is guaranteed by the outbox worker asynchronously.
- Returns error if JSON serialisation of `Payload` fails.

---

### `StartWorkflow(ctx context.Context, spec ActionWorkflowSpec) (workflowID string, err error)`

Enqueues a Temporal workflow via the workflow outbox.

**Parameters:**
- `spec.WorkflowFn`: registered Temporal workflow function name.
- `spec.TaskQueue`: Temporal task queue.
- `spec.WorkflowID`: optional. When empty, the runtime generates `{tenant_id}.{entity}.{record_id}.{action_name}`.
- `spec.Input`: JSON-serialisable workflow input.

**Behaviour:**
- Inside `Tx`: the workflow outbox record is written within the TX and deferred to post-commit dispatch.
- Outside `Tx`: written in a new mini-transaction and dispatched immediately by the outbox worker.
- Returns the workflow ID (generated or provided) on success.
- Returns error if JSON serialisation fails or the outbox write fails.

**Idempotency:** if a workflow with the same ID is already running, Temporal returns the existing run rather than starting a duplicate.

---

### `Notify(ctx context.Context, n ActionNotification) error`

Sends a notification through the configured notification channels.

**Parameters:**
- `n.UserIDs`: target user UUIDs. Must be non-empty.
- `n.Subject`: notification title.
- `n.Body`: notification body (plain text or HTML depending on `Channel`).
- `n.Channel`: `"email"` / `"sms"` / `"in_app"`. Defaults to `"in_app"`.
- `n.TenantID`: set automatically by runtime.

**Behaviour:**
- Sends synchronously for `"in_app"` (Redis pub/sub, best-effort).
- Enqueues asynchronously for `"email"` and `"sms"` via notification outbox.
- Returns error if `UserIDs` is empty, channel is unknown, or Body is empty.

---

### `InvalidateCache(ctx context.Context, entityName string) error`

Clears all cached SDUI schemas and feature flag evaluations for the entity.

**Parameters:**
- `entityName`: fully-qualified entity name.

**Behaviour:**
- Calls `Cache.DeletePrefix("page:{entityName}:")` to remove all view/role variants.
- Calls `Cache.DeletePrefix("eval:")` scoped to the tenant to invalidate feature flag caches.
- Returns immediately. Cache misses on subsequent requests will trigger recomputation.

Call this after actions that change record state visible to SDUI schemas (e.g., a status change that shows/hides form sections or list filters).

---

### `Cache() ActionCache`

Returns the tenant-namespaced cache accessor.

The returned `ActionCache` automatically prepends `{tenant_id}:` to all keys. Do not manually include `tenant_id` in cache keys — it will be double-prefixed.

See [`11-cache/CACHE_SPEC.md`](../11-cache/CACHE_SPEC.md) for the full `ActionCache` interface.

---

### `Clock() time.Time`

Returns the current wall-clock time.

**Why not `time.Now()`:** `Clock()` is mockable in tests. Test implementations return a controlled time. Always use `Clock()` in action handlers instead of `time.Now()` to enable deterministic testing.

---

### `Logger() *slog.Logger`

Returns a structured logger pre-seeded with context fields:

```
tenant_id:   {current tenant UUID}
user_id:     {actor user UUID or nil}
entity_name: {action's target entity name}
action_name: {action name from ActionDef.Name}
request_id:  {X-Request-ID from request context}
```

Use this logger — do not construct a new `slog.Logger` inside action handlers. This ensures all log entries from the action are correlated by `request_id` and `tenant_id`.

---

### `TenantID() uuid.UUID`

Returns the UUID of the tenant this action executes within.

This is the same tenant ID set by `set_tenant_context()` in the middleware. It is always non-zero for tenant-scoped actions.

---

### `Actor() *Actor`

Returns the authenticated principal who invoked the action.

**Behaviour:**
- Never nil for guarded actions (all actions with a non-empty `Permission` in `ActionDef`).
- Returns a system actor (platform admin service account) for platform-initiated actions.

The returned `*Actor` reflects the post-ADR-003 struct: `UserID`, `ServiceAccountID`, `TenantID`, `Roles`. See [`03-auth/ACTOR_MODEL.md`](../03-auth/ACTOR_MODEL.md).

---

## ActionEntityRepo Reference

`ActionRuntime.Repo()` returns `ActionEntityRepo`. Its full interface:

```go
type ActionEntityRepo interface {
    EntityName() string
    Get(ctx context.Context, id uuid.UUID) (*EntityRecord, error)
    Query(ctx context.Context, f ActionFilter, opts ...ActionQueryOpt) ([]*EntityRecord, error)
    Count(ctx context.Context, f ActionFilter) (int64, error)
    Exists(ctx context.Context, f ActionFilter) (bool, error)
    Create(ctx context.Context, data map[string]any) (*EntityRecord, error)
    Update(ctx context.Context, id uuid.UUID, patch map[string]any) (*EntityRecord, error)
    Delete(ctx context.Context, id uuid.UUID) error
}
```

`Create` and `Update` run the full lifecycle pipeline (hooks, validation, audit). They are not raw SQL inserts.

`Query` accepts `*filter.Filter` as the `ActionFilter` argument. Always use `awo/filter` constructors — never implement `ActionFilter` directly.

---

## References

- `awo/def/action_runtime.go` — Interface source
- [`13-actions/ACTION_HANDLER_GUIDE.md`](ACTION_HANDLER_GUIDE.md) — How to write action handlers
- [`13-actions/CUSTOM_ACTIONS_EXAMPLES.md`](CUSTOM_ACTIONS_EXAMPLES.md) — Finance module examples
- [`06-filter/FILTER_DSL_REFERENCE.md`](../06-filter/FILTER_DSL_REFERENCE.md) — Filter constructors
