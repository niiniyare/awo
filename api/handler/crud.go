// Package handler provides auto-generated CRUD handlers derived from
// CompiledSchema. Module authors never write custom CRUD handlers; the
// framework generates them from EntityDefinition declarations.
package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"awo.so/awo/api/filterparse"
	"awo.so/awo/api/response"
	"awo.so/awo/api/service"
	"awo.so/awo/auth"
	"awo.so/awo/compiler"
	"awo.so/awo/def"
	"awo.so/awo/driver"
	"awo.so/awo/runtime"
)

// EntityHandler handles CRUD + action operations for a single entity type.
type EntityHandler struct {
	schema  *compiler.EntitySchema
	svc     *service.EntityService
	actions *service.ActionContextFactory
}

// NewEntityHandler creates a handler for the given entity schema. actions is
// the shared, router-wide factory that constructs the canonical
// runtime.ActionContext for every action invocation (Phase 2 Step 4) — the
// same factory instance is passed to every entity's handler so any action
// can reach any other registered entity via Repo(entityName).
func NewEntityHandler(schema *compiler.EntitySchema, svc *service.EntityService, actions *service.ActionContextFactory) *EntityHandler {
	return &EntityHandler{schema: schema, svc: svc, actions: actions}
}

// List handles GET /api/v1/entities/:entity
func (h *EntityHandler) List(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	pageSize := c.QueryInt("page_size", 20)

	f, err := filterparse.FromQuery(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.Wrap(&runtime.ValidationError{
			Fields: map[string]string{"filter": err.Error()},
		}))
	}

	// AMIS sends orderBy=field&orderDir=asc|desc when the user clicks a column header.
	queryOpts := []driver.QueryOption{driver.WithPage(page, pageSize)}
	if orderBy := c.Query("orderBy"); orderBy != "" {
		queryOpts = append(queryOpts, driver.WithSort(orderBy, c.Query("orderDir") != "desc"))
	}

	records, info, err := h.svc.Query(
		c.UserContext(),
		f,
		queryOpts...,
	)
	if err != nil {
		return h.handleError(c, err)
	}

	data := make([]map[string]any, len(records))
	for i, rec := range records {
		data[i] = recordToMap(rec)
	}
	return c.JSON(response.Success{
		Data: data,
		Meta: &response.Meta{
			Total:    info.Total,
			Page:     info.Page,
			PageSize: info.PageSize,
			HasMore:  info.HasNextPage,
		},
	})
}

// Get handles GET /api/v1/entities/:entity/:id
func (h *EntityHandler) Get(c *fiber.Ctx) error {
	id, err := parseUUID(c, "id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.Wrap(&runtime.ValidationError{
			Fields: map[string]string{"id": "must be a valid UUID"},
		}))
	}
	rec, err := h.svc.Get(c.UserContext(), id)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(response.Success{Data: recordToMap(rec)})
}

// Create handles POST /api/v1/entities/:entity
func (h *EntityHandler) Create(c *fiber.Ctx) error {
	var body map[string]any
	if err := json.Unmarshal(c.Body(), &body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.Wrap(&runtime.ValidationError{
			Fields: map[string]string{"_body": "invalid JSON"},
		}))
	}
	created, err := h.svc.Create(c.UserContext(), body, actorFromContext(c))
	if err != nil {
		return h.handleError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(response.Success{Data: recordToMap(created)})
}

// Update handles PATCH /api/v1/entities/:entity/:id
func (h *EntityHandler) Update(c *fiber.Ctx) error {
	id, err := parseUUID(c, "id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.Wrap(&runtime.ValidationError{
			Fields: map[string]string{"id": "must be a valid UUID"},
		}))
	}
	var body map[string]any
	if err := json.Unmarshal(c.Body(), &body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.Wrap(&runtime.ValidationError{
			Fields: map[string]string{"_body": "invalid JSON"},
		}))
	}
	updated, err := h.svc.Update(c.UserContext(), id, body, actorFromContext(c))
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(response.Success{Data: recordToMap(updated)})
}

// Delete handles DELETE /api/v1/entities/:entity/:id
func (h *EntityHandler) Delete(c *fiber.Ctx) error {
	id, err := parseUUID(c, "id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.Wrap(&runtime.ValidationError{
			Fields: map[string]string{"id": "must be a valid UUID"},
		}))
	}
	if err := h.svc.Delete(c.UserContext(), id, actorFromContext(c)); err != nil {
		return h.handleError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// maxBatchDeleteIDs bounds a single bulk-delete request. Not a backend
// capacity limit — a sanity bound against an accidental or malicious
// request listing an unbounded number of IDs in one call.
const maxBatchDeleteIDs = 200

// DeleteBatch handles DELETE /api/v1/entities/:entity — the ids are always
// in the JSON body, never a query string or path segment, so there is no
// ambiguity with the single-record DELETE /:id route.
//
// The body's "ids" field accepts either a real JSON array of UUID strings
// (`{"ids": ["<uuid>", ...]}`) or a single comma-joined string
// (`{"ids": "<uuid>,<uuid>"}`) — see batchIDs' UnmarshalJSON.
//
// Confirmed against the real amis@6.7.0 SDK source (Table.renderBulkActions:
// `ids: p.map(e => e[primaryField]).filter(Boolean).join(",")`): the
// "${ids}" binding this file's own renderBulkActions (sdui/amis/renderer.go)
// uses always produces a comma-joined string, never a raw array — so the
// comma-string branch is the one every real amis bulk-delete request
// actually takes. The array branch is kept as defensive handling for any
// other caller of this generic endpoint (a non-amis API client sending a
// literal JSON array is a perfectly reasonable request), not dead code.
//
// Deliberately composed from the exact same per-record path as the
// single-record Delete above — EntityService.Delete, unchanged — run once
// per ID inside one outer transaction. This is not a new authorization,
// tenant/org-scope, lifecycle, or audit mechanism: every one of those is
// already enforced inside EntityService.Delete (Get is RLS-filtered by
// tenant/org; RunBeforeDelete enforces any lifecycle-restriction hook;
// RunAuditRecord writes the same audit trail a single delete would). The
// route itself is registered behind the same authz.RequirePermission
// middleware as every other mutation on this entity (api/router/router.go).
//
// All-or-nothing: repo.WithTx's existing InTx() short-circuit
// (contrib/pgx/repo.go) means every per-ID EntityService.Delete call below
// reuses this one outer transaction instead of opening its own — so the
// first ID that fails (not found, cross-tenant, cross-org, unauthorized,
// or lifecycle-restricted — EntityService.Delete returns the same real
// error for all of these as the single-record path already does) rolls
// back every row in the batch, including ones already "deleted" earlier
// in the loop. No partial deletion is possible.
func (h *EntityHandler) DeleteBatch(c *fiber.Ctx) error {
	var body struct {
		IDs batchIDs `json:"ids"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.Wrap(&runtime.ValidationError{
			Fields: map[string]string{"ids": "request body must be {\"ids\": [\"<uuid>\", ...]} or {\"ids\": \"<uuid>,<uuid>\"}"},
		}))
	}
	if len(body.IDs) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(response.Wrap(&runtime.ValidationError{
			Fields: map[string]string{"ids": "at least one id is required"},
		}))
	}
	if len(body.IDs) > maxBatchDeleteIDs {
		return c.Status(fiber.StatusBadRequest).JSON(response.Wrap(&runtime.ValidationError{
			Fields: map[string]string{"ids": "too many ids in one request (max " + strconv.Itoa(maxBatchDeleteIDs) + ")"},
		}))
	}
	seen := make(map[uuid.UUID]bool, len(body.IDs))
	ids := make([]uuid.UUID, 0, len(body.IDs))
	for _, s := range body.IDs {
		id, err := uuid.Parse(s)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(response.Wrap(&runtime.ValidationError{
				Fields: map[string]string{"ids": "not a valid UUID: " + s},
			}))
		}
		// Dedupe: a repeated id (e.g. a stale duplicated frontend selection)
		// would otherwise delete the row once and then fail on its second
		// occurrence with NotFound inside the same transaction — rolling
		// back every row in the batch, including the one that legitimately
		// succeeded first. See this function's own doc comment on the
		// all-or-nothing transaction.
		if seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}

	actor := actorFromContext(c)
	deleted := 0
	err := h.svc.WithTx(c.UserContext(), func(txCtx context.Context) error {
		for _, id := range ids {
			if err := h.svc.Delete(txCtx, id, actor); err != nil {
				return &batchDeleteError{id: id, err: err}
			}
			deleted++
		}
		return nil
	})
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(response.Success{Data: map[string]any{"deleted": deleted}})
}

// batchIDs unmarshals DeleteBatch's "ids" field from either a real JSON
// array of strings or a single comma-joined string — see DeleteBatch's own
// doc comment for why both are accepted. Empty elements from stray
// whitespace/commas (e.g. a trailing comma) are dropped rather than turned
// into a confusing "not a valid UUID: " error for an empty string.
type batchIDs []string

func (b *batchIDs) UnmarshalJSON(data []byte) error {
	var asArray []string
	if err := json.Unmarshal(data, &asArray); err == nil {
		*b = asArray
		return nil
	}
	var asString string
	if err := json.Unmarshal(data, &asString); err != nil {
		return err
	}
	var out batchIDs
	for _, part := range strings.Split(asString, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	*b = out
	return nil
}

// batchDeleteError names which id in a batch failed, without changing the
// underlying error's type — response.HTTPStatus/response.Wrap still see
// through to the real cause (not found, permission, lifecycle, etc.) via
// Unwrap, so the HTTP status and error code for the batch match exactly
// what that same id would have produced through the single-record route.
type batchDeleteError struct {
	id  uuid.UUID
	err error
}

func (e *batchDeleteError) Error() string {
	return "id " + e.id.String() + ": " + e.err.Error()
}

func (e *batchDeleteError) Unwrap() error { return e.err }

// Action handles POST /api/v1/entities/:entity/:id/:action
func (h *EntityHandler) Action(c *fiber.Ctx) error {
	id, err := parseUUID(c, "id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.Wrap(&runtime.ValidationError{
			Fields: map[string]string{"id": "must be a valid UUID"},
		}))
	}
	actionName := c.Params("action")
	actionDef, ok := h.schema.ActionsByName[actionName]
	if !ok {
		return c.Status(fiber.StatusNotFound).JSON(response.Wrap(&runtime.BusinessError{
			Code:    "action.not_found",
			Message: "Action not found: " + actionName,
			Status:  404,
		}))
	}

	actor := actorFromContext(c)
	rt, err := h.actions.New(c.UserContext(), h.schema.QualifiedName, id, actor)
	if err != nil {
		slog.Error("failed to construct action runtime",
			"entity", h.schema.QualifiedName, "action", actionName, "err", err)
		return c.Status(fiber.StatusInternalServerError).JSON(response.Wrap(&runtime.BusinessError{
			Code:    "action.runtime_unavailable",
			Message: "Action runtime could not be constructed",
			Status:  500,
		}))
	}

	actx := &def.ActionContext{
		Ctx:      c.UserContext(),
		RecordID: id,
		Actor:    actor,
		Body:     c.Body(),
		Runtime:  rt,
	}
	result, err := actionDef.HandlerFunc(actx)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(response.Success{Data: result})
}

// -------------------------------------------------------------------------
// Helpers
// -------------------------------------------------------------------------

func (h *EntityHandler) handleError(c *fiber.Ctx, err error) error {
	status := response.HTTPStatus(err)
	if status >= 500 {
		slog.Error("unhandled error",
			"request_id", c.Locals("request_id"),
			"tenant_id", c.Locals("tenant_id"),
			"entity", h.schema.QualifiedName,
			"err", err,
		)
	}
	return c.Status(status).JSON(response.Wrap(err))
}

func recordToMap(rec *def.EntityRecord) map[string]any {
	if rec == nil {
		return nil
	}
	m := map[string]any{
		"id":         rec.ID,
		"tenant_id":  rec.TenantID,
		"created_at": rec.CreatedAt,
		"updated_at": rec.UpdatedAt,
	}
	for k, v := range rec.Data {
		m[k] = v
	}
	for k, v := range rec.CustomFields {
		m[k] = v
	}
	return m
}

func parseUUID(c *fiber.Ctx, param string) (uuid.UUID, error) {
	return uuid.Parse(c.Params(param))
}

func actorFromContext(c *fiber.Ctx) *def.Actor {
	// Use the ViewerContext stored by RequireAuth — it carries the full identity
	// (UserID, ServiceAccountID, TenantID, Roles) rather than just a user_id local.
	viewer, ok := c.Locals("viewer").(auth.ViewerContext)
	if !ok || viewer == nil {
		return nil
	}
	return viewer.Actor()
}
