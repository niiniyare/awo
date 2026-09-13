// Package meta provides the Metadata API endpoints.
//
// These endpoints expose the compiled entity schema as structured JSON,
// enabling external UI builders, CLI tooling, and developer portals to
// introspect the system without parsing Go source code.
//
// # Endpoints
//
//	GET /api/v1/meta/entities          → list of all entity summaries
//	GET /api/v1/meta/entities/{name}   → full EntitySchema as JSON
//	GET /api/v1/meta/permissions       → all declared permission identifiers
//
// No authentication is required. Entity metadata (field names, types,
// labels) is not considered sensitive. Do not add auth middleware to these
// routes — they must be reachable by unauthenticated CLI tools.
package meta

import (
	"github.com/gofiber/fiber/v2"

	"awo.so/awo/compiler"
)

// Handler serves entity metadata from a compiled schema.
// Safe for concurrent use; holds no mutable state.
type Handler struct {
	schema *compiler.CompiledSchema
}

// New constructs a Handler for the given compiled schema.
func New(schema *compiler.CompiledSchema) *Handler {
	return &Handler{schema: schema}
}

// Register mounts the metadata endpoints on the given Fiber router.
// Intended to be called with an unauthenticated group.
func (h *Handler) Register(r fiber.Router) {
	r.Get("/entities", h.listEntities)
	r.Get("/entities/:name", h.getEntity)
	r.Get("/permissions", h.listPermissions)
}

// entitySummary is the response shape for list entities.
type entitySummary struct {
	Name        string `json:"name"`
	Module      string `json:"module"`
	Label       string `json:"label"`
	LabelPlural string `json:"label_plural"`
	IsSystem    bool   `json:"is_system"`
	Scope       string `json:"scope"`
	TableName   string `json:"table_name,omitempty"`
	RoutePrefix string `json:"route_prefix"`
	FieldCount  int    `json:"field_count"`
	EdgeCount   int    `json:"edge_count"`
	ActionCount int    `json:"action_count"`
}

// fieldSummary describes a single field for the entity detail response.
type fieldSummary struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Type        string `json:"type"`
	Required    bool   `json:"required,omitempty"`
	Unique      bool   `json:"unique,omitempty"`
	Immutable   bool   `json:"immutable,omitempty"`
	Sensitive   bool   `json:"sensitive,omitempty"`
	Searchable  bool   `json:"searchable,omitempty"`
	ReadOnly    bool   `json:"read_only,omitempty"`
	Hidden      bool   `json:"hidden,omitempty"`
	MaxLen      int    `json:"max_len,omitempty"`
	LinkTarget  string `json:"link_target,omitempty"`
	Options     []string `json:"options,omitempty"`
	Description string `json:"description,omitempty"`
}

// edgeSummary describes a single edge for the entity detail response.
type edgeSummary struct {
	Name       string `json:"name"`
	Label      string `json:"label"`
	Target     string `json:"target"`
	Type       string `json:"type"`
	ForeignKey string `json:"foreign_key,omitempty"`
}

// actionSummary describes a single action for the entity detail response.
type actionSummary struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Method      string `json:"method"`
	Permissions []string `json:"permissions,omitempty"`
}

// entityDetail is the full entity detail response.
type entityDetail struct {
	Name        string        `json:"name"`
	Module      string        `json:"module"`
	Label       string        `json:"label"`
	LabelPlural string        `json:"label_plural"`
	Description string        `json:"description,omitempty"`
	IsSystem    bool          `json:"is_system"`
	Scope       string        `json:"scope"`
	AllowAudit  bool          `json:"allow_audit"`
	TableName   string        `json:"table_name,omitempty"`
	RoutePrefix string        `json:"route_prefix"`
	Icon        string        `json:"icon,omitempty"`
	Fields      []fieldSummary `json:"fields"`
	Edges       []edgeSummary  `json:"edges"`
	Actions     []actionSummary `json:"actions"`
	Permissions permissionDetail `json:"permissions"`
}

// permissionDetail maps CRUD operations to permission identifiers.
type permissionDetail struct {
	Create  []string            `json:"create,omitempty"`
	Read    []string            `json:"read,omitempty"`
	Write   []string            `json:"write,omitempty"`
	Delete  []string            `json:"delete,omitempty"`
	Actions map[string][]string `json:"actions,omitempty"`
}

// listEntities handles GET /api/v1/meta/entities.
// Returns a summary of every compiled entity.
func (h *Handler) listEntities(c *fiber.Ctx) error {
	items := make([]entitySummary, 0, len(h.schema.Entities))
	for _, es := range h.schema.Entities {
		items = append(items, entitySummary{
			Name:        es.QualifiedName,
			Module:      es.Module,
			Label:       es.Label,
			LabelPlural: es.LabelPlural,
			IsSystem:    es.IsSystem,
			Scope:       string(es.Scope),
			TableName:   es.TableName,
			RoutePrefix: es.RoutePrefix,
			FieldCount:  len(es.Fields),
			EdgeCount:   len(es.Edges),
			ActionCount: len(es.Actions),
		})
	}
	return c.JSON(items)
}

// getEntity handles GET /api/v1/meta/entities/:name.
// Returns full schema details for a single entity.
func (h *Handler) getEntity(c *fiber.Ctx) error {
	name := c.Params("name")
	es, ok := h.schema.ByName[name]
	if !ok {
		return fiber.NewError(fiber.StatusNotFound, "entity not found: "+name)
	}

	fields := make([]fieldSummary, 0, len(es.Fields))
	for _, f := range es.Fields {
		fs := fieldSummary{
			Name:        f.Name,
			Label:       f.Label,
			Type:        string(f.Type),
			Required:    f.Required,
			Unique:      f.Unique,
			Immutable:   f.Immutable,
			Sensitive:   f.Sensitive,
			Searchable:  f.Searchable,
			ReadOnly:    f.ReadOnly,
			Hidden:      f.Hidden,
			MaxLen:      f.MaxLen,
			LinkTarget:  f.LinkTarget,
			Options:     f.Options,
			Description: f.Description,
		}
		fields = append(fields, fs)
	}

	edges := make([]edgeSummary, 0, len(es.Edges))
	for _, e := range es.Edges {
		edges = append(edges, edgeSummary{
			Name:       e.Name,
			Label:      e.Label,
			Target:     e.Target,
			Type:       string(e.Type),
			ForeignKey: e.ForeignKey,
		})
	}

	actions := make([]actionSummary, 0, len(es.Actions))
	for _, a := range es.Actions {
		var perms []string
		if p, ok := es.Permissions.Actions[a.Name]; ok {
			perms = p
		}
		actions = append(actions, actionSummary{
			Name:        a.Name,
			Label:       a.Label,
			Method:      string(a.Method),
			Permissions: perms,
		})
	}

	detail := entityDetail{
		Name:        es.QualifiedName,
		Module:      es.Module,
		Label:       es.Label,
		LabelPlural: es.LabelPlural,
		Description: es.Description,
		IsSystem:    es.IsSystem,
		Scope:       string(es.Scope),
		AllowAudit:  es.AllowAudit,
		TableName:   es.TableName,
		RoutePrefix: es.RoutePrefix,
		Icon:        es.Icon,
		Fields:      fields,
		Edges:       edges,
		Actions:     actions,
		Permissions: permissionDetail{
			Create:  es.Permissions.Create,
			Read:    es.Permissions.Read,
			Write:   es.Permissions.Write,
			Delete:  es.Permissions.Delete,
			Actions: es.Permissions.Actions,
		},
	}
	return c.JSON(detail)
}

// permissionEntry is a single permission identifier entry in the list response.
type permissionEntry struct {
	Permission string `json:"permission"`
	Entity     string `json:"entity"`
	Action     string `json:"action"`
}

// listPermissions handles GET /api/v1/meta/permissions.
// Returns all CapabilityGrants (permission identifier → entity + action).
func (h *Handler) listPermissions(c *fiber.Ctx) error {
	items := make([]permissionEntry, 0, len(h.schema.CapabilityGrants))
	for _, g := range h.schema.CapabilityGrants {
		items = append(items, permissionEntry{
			Permission: g.Permission,
			Entity:     g.Entity,
			Action:     g.Action,
		})
	}
	return c.JSON(items)
}
