> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Route Generation"
id: kern-007
status: accepted
category: SPEC
stability: FROZEN
audience: [framework-authors, module-authors]
since: "1.0"
normative-level: normative
related:
  - "[EntityDefinition](entity-def.md)"
  - "[Entity Registry](registry.md)"
  - "[Actions](../04-domain/actions.md)"
  - "[Entity API Reference](../11-api/entity-api-reference.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Route Generation

**KERN-007 | Status: Accepted | Stability: Frozen**

How the framework automatically generates Fiber HTTP routes from `EntityDefinition` declarations.

---

## 1. Route Generation Trigger

Route generation runs once during startup, after the EntityRegistry is sealed:

```
EntityRegistry.Seal()
    ↓
For each registered EntityDefinition:
    GenerateCRUDRoutes(app, def)
    GenerateActionRoutes(app, def)
    GenerateEdgeRoutes(app, def)  // if edge has dedicated endpoint
```

Route generation is idempotent — running it twice with the same registry produces identical routes. Sealing the registry prevents new routes from being added after startup.

---

## 2. Standard CRUD Routes

For each `EntityDefinition` with `Name: "finance_invoice"`:

| Method | Path | Handler | Permission |
|---|---|---|---|
| `GET` | `/api/v1/entities/finance_invoice` | `listHandler` | `Read` |
| `GET` | `/api/v1/entities/finance_invoice/:id` | `getHandler` | `Read` |
| `POST` | `/api/v1/entities/finance_invoice` | `createHandler` | `Create` |
| `PATCH` | `/api/v1/entities/finance_invoice/:id` | `updateHandler` | `Write` |
| `DELETE` | `/api/v1/entities/finance_invoice/:id` | `deleteHandler` | `Delete` |

Generated route code (framework-internal):

```go
func GenerateCRUDRoutes(app *fiber.App, def *def.EntityDefinition, deps RouteDeps) {
    base := "/api/v1/entities/" + def.Name

    app.Get(base, withPermission(def, "read", deps.makeListHandler(def)))
    app.Get(base+"/:id", withPermission(def, "read", deps.makeGetHandler(def)))
    app.Post(base, withPermission(def, "create", deps.makeCreateHandler(def)))
    app.Patch(base+"/:id", withPermission(def, "write", deps.makeUpdateHandler(def)))
    app.Delete(base+"/:id", withPermission(def, "delete", deps.makeDeleteHandler(def)))
}
```

---

## 3. Action Routes

For each `ActionDef` in `EntityDefinition.Actions`:

| Method | Path | Permission |
|---|---|---|
| `POST` | `/api/v1/entities/{type}/{id}/{action-name}` | From `ActionDef.Permission` |

```go
func GenerateActionRoutes(app *fiber.App, def *def.EntityDefinition, deps RouteDeps) {
    for _, action := range def.Actions {
        action := action  // capture for closure
        path := fmt.Sprintf("/api/v1/entities/%s/:id/%s", def.Name, action.Name)
        app.Post(path, withActionPermission(def, action, deps.makeActionHandler(def, action)))
    }
}
```

---

## 4. Permission Middleware

Each route is wrapped with permission-checking middleware:

```go
func withPermission(def *def.EntityDefinition, action string, next fiber.Handler) fiber.Handler {
    return func(c *fiber.Ctx) error {
        ctx := c.UserContext()
        actor := session.ActorFromContext(ctx)
        tenantID := session.TenantIDFromContext(ctx)

        // Platform admins bypass Casbin
        if actor.IsPlatformAdmin {
            return next(c)
        }

        allowed, err := enforcer.Enforce(
            "user:"+actor.UserID.String(),
            tenantID.String(),
            def.Name,
            action,
        )
        if err != nil {
            return fiber.ErrInternalServerError
        }
        if !allowed {
            return c.Status(403).JSON(fiber.Map{
                "error": fiber.Map{"code": "forbidden"},
            })
        }

        return next(c)
    }
}
```

---

## 5. Custom Handler Override

To use a custom handler for a specific route, include a `CustomRoutes` field in `EntityDefinition`:

```go
var InvoiceDefinition = def.SystemDefinition{
    // ...
    CustomRoutes: []def.CustomRoute{
        {
            Method:  "GET",
            Path:    "/api/v1/entities/finance_invoice/:id/pdf",
            Handler: GenerateInvoicePDFHandler,
            // No auto-generated permission check — handler must check manually
        },
    },
}
```

Custom routes are registered after CRUD routes. They do not get the auto-permission middleware — the handler is responsible for its own permission checks.

---

## 6. Route Listing

At startup, the framework logs all registered routes at DEBUG level:

```
DEBUG registered route GET    /api/v1/entities/finance_invoice
DEBUG registered route GET    /api/v1/entities/finance_invoice/:id
DEBUG registered route POST   /api/v1/entities/finance_invoice
DEBUG registered route PATCH  /api/v1/entities/finance_invoice/:id
DEBUG registered route DELETE /api/v1/entities/finance_invoice/:id
DEBUG registered route POST   /api/v1/entities/finance_invoice/:id/submit
DEBUG registered route POST   /api/v1/entities/finance_invoice/:id/approve
```

Route conflicts (duplicate method+path) cause startup to fail with an explicit error — the registry catches this before Fiber's panics would surface.

---

## 7. SDUI Schema Routes

Each entity also gets SDUI schema endpoints:

```
GET /api/v1/schemas/finance_invoice/list    → Returns amis list schema
GET /api/v1/schemas/finance_invoice/create  → Returns amis create form schema
GET /api/v1/schemas/finance_invoice/edit    → Returns amis edit form schema
GET /api/v1/schemas/finance_invoice/detail  → Returns amis detail view schema
```

These are served from Redis cache (5min TTL, keyed by entity name + version + tenant + permissions hash). The schema is regenerated on cache miss by calling the entity's `PageBuilderSet`.

---

## Related Documents

- [EntityDefinition](entity-def.md) — the source of route generation
- [Entity Registry](registry.md) — sealing the registry triggers route generation
- [Actions](../04-domain/actions.md) — ActionDef that produces action routes
- [Entity API Reference](../11-api/entity-api-reference.md) — complete route contract for API consumers
