// Package handler provides auto-generated CRUD handlers derived from
// CompiledSchema. Module authors never write custom CRUD handlers; the
// framework generates them from EntityDefinition declarations.
package handler

import (
	"encoding/json"
	"log/slog"

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
	schema *compiler.EntitySchema
	svc    *service.EntityService
}

// NewEntityHandler creates a handler for the given entity schema.
func NewEntityHandler(schema *compiler.EntitySchema, svc *service.EntityService) *EntityHandler {
	return &EntityHandler{schema: schema, svc: svc}
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

	actx := &def.ActionContext{
		Ctx:      c.UserContext(),
		RecordID: id,
		Actor:    actorFromContext(c),
		Body:     c.Body(),
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
