// Package showcase provides the developer showcase HTTP API.
//
// These endpoints are diagnostic and require no authentication. They expose
// framework metadata — entity counts, route counts, registered renderers —
// to support the developer experience portal at /showcase.
//
// Do not mount these routes in production behind sensitive data.
package showcase

import (
	"sort"

	"github.com/gofiber/fiber/v2"

	"awo.so/awo/compiler"
	sdui_engine "awo.so/awo/sdui/engine"
)

// Handler provides showcase diagnostic endpoints.
// Safe for concurrent use; holds no mutable state.
type Handler struct {
	schema *compiler.CompiledSchema
	engine *sdui_engine.Engine
}

// New constructs a Handler. Both schema and engine are required.
func New(schema *compiler.CompiledSchema, engine *sdui_engine.Engine) *Handler {
	return &Handler{schema: schema, engine: engine}
}

// Register mounts showcase diagnostic endpoints on app (no auth required).
//
//	GET /api/v1/showcase/info       → framework summary
//	GET /api/v1/showcase/entities   → entity list
//	GET /api/v1/showcase/renderers  → registered renderer IDs
func (h *Handler) Register(app *fiber.App) {
	g := app.Group("/api/v1/showcase")
	g.Get("/info", h.info)
	g.Get("/entities", h.entities)
	g.Get("/renderers", h.renderers)
}

// InfoResponse is the payload returned by GET /api/v1/showcase/info.
type InfoResponse struct {
	Version     string   `json:"version"`
	EntityCount int      `json:"entityCount"`
	RouteCount  int      `json:"routeCount"`
	Modules     []string `json:"modules"`
	RendererIDs []string `json:"rendererIds"`
}

func (h *Handler) info(c *fiber.Ctx) error {
	modules := make(map[string]struct{})
	for _, es := range h.schema.Entities {
		modules[es.Module] = struct{}{}
	}
	mods := make([]string, 0, len(modules))
	for m := range modules {
		mods = append(mods, m)
	}
	sort.Strings(mods)

	rmap := h.engine.Renderers()
	renderers := make([]string, 0, len(rmap))
	for id := range rmap {
		renderers = append(renderers, id)
	}
	sort.Strings(renderers)

	return c.JSON(InfoResponse{
		Version:     "0.1.0-dev",
		EntityCount: len(h.schema.Entities),
		RouteCount:  len(h.schema.Routes),
		Modules:     mods,
		RendererIDs: renderers,
	})
}

// EntitySummary is a single row in the entities list response.
type EntitySummary struct {
	QualifiedName string `json:"qualifiedName"`
	Module        string `json:"module"`
	Label         string `json:"label"`
	LabelPlural   string `json:"labelPlural"`
	Icon          string `json:"icon,omitempty"`
	FieldCount    int    `json:"fieldCount"`
	RoutePrefix   string `json:"routePrefix"`
}

func (h *Handler) entities(c *fiber.Ctx) error {
	out := make([]EntitySummary, 0, len(h.schema.Entities))
	for _, es := range h.schema.Entities {
		out = append(out, EntitySummary{
			QualifiedName: es.QualifiedName,
			Module:        es.Module,
			Label:         es.Label,
			LabelPlural:   es.LabelPlural,
			Icon:          es.Icon,
			FieldCount:    len(es.Fields),
			RoutePrefix:   es.RoutePrefix,
		})
	}
	return c.JSON(out)
}

func (h *Handler) renderers(c *fiber.Ctx) error {
	rmap := h.engine.Renderers()
	ids := make([]string, 0, len(rmap))
	for id := range rmap {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return c.JSON(map[string]any{"renderers": ids})
}
