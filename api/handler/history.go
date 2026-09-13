package handler

import (
	"log/slog"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"awo.so/awo/api/response"
	"awo.so/awo/audit"
	"awo.so/awo/compiler"
	"awo.so/awo/runtime"
)

const (
	historyDefaultLimit = 100
	historyMaxLimit     = 500
)

// HistoryHandler serves GET /api/v1/{module}/{resource}/:id/history for
// entities whose EntitySchema.AllowAudit is true.
//
// The route is registered per-entity by the router when AllowAudit: true.
// Entities with AllowAudit: false never have this route registered; an attempt
// to construct a HistoryHandler for such an entity should use the
// AllowAudit field guard in the router before calling NewHistoryHandler.
type HistoryHandler struct {
	schema  *compiler.EntitySchema
	queryer audit.Queryer
}

// NewHistoryHandler creates a HistoryHandler for schema using queryer.
// If queryer is nil, audit.NoopQueryer is used (returns empty history).
func NewHistoryHandler(schema *compiler.EntitySchema, queryer audit.Queryer) *HistoryHandler {
	if queryer == nil {
		queryer = audit.NoopQueryer{}
	}
	return &HistoryHandler{schema: schema, queryer: queryer}
}

// Handle processes GET /api/v1/{module}/{resource}/:id/history.
//
// Path parameter :id must be a valid UUID.
// Optional query parameter ?limit=N (default 100, max 500).
//
// Returns 200 with a JSON array of HistoryEntry values ordered
// chronologically (oldest first).
//
// Returns 404 when the entity does not support audit history
// (AllowAudit: false). In normal operation the router will never register
// this route for such entities, but the guard is present for safety.
//
// Returns 400 for an invalid :id or an out-of-range ?limit.
func (h *HistoryHandler) Handle(c *fiber.Ctx) error {
	// Guard: entity must have audit enabled.
	if !h.schema.AllowAudit {
		return c.Status(fiber.StatusNotFound).JSON(response.Wrap(&runtime.BusinessError{
			Code:    "history.not_available",
			Message: "Audit history is not available for " + h.schema.QualifiedName,
			Status:  fiber.StatusNotFound,
		}))
	}

	// Parse :id.
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.Wrap(&runtime.ValidationError{
			Fields: map[string]string{"id": "must be a valid UUID"},
		}))
	}

	// Parse optional ?limit query parameter.
	limit := historyDefaultLimit
	if raw := c.Query("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			return c.Status(fiber.StatusBadRequest).JSON(response.Wrap(&runtime.ValidationError{
				Fields: map[string]string{"limit": "must be a non-negative integer"},
			}))
		}
		if n > historyMaxLimit {
			n = historyMaxLimit
		}
		limit = n
	}

	entries, err := h.queryer.History(c.UserContext(), h.schema.QualifiedName, id, limit)
	if err != nil {
		slog.Error("history query failed",
			"entity", h.schema.QualifiedName,
			"record_id", id,
			"err", err,
		)
		return c.Status(fiber.StatusInternalServerError).JSON(response.Wrap(&runtime.BusinessError{
			Code:    "history.query_failed",
			Message: "Failed to retrieve history",
			Status:  fiber.StatusInternalServerError,
		}))
	}

	// Return empty array (not null) when no entries found.
	if entries == nil {
		entries = []audit.HistoryEntry{}
	}
	return c.JSON(response.Success{Data: entries})
}
