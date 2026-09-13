// Package introspect provides runtime self-inspection of the compiled schema.
// It is intended for developer tooling and non-production debugging endpoints.
// Never expose introspect data to untrusted clients in production.
package introspect

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2"

	"awo.so/awo/compiler"
)

// SchemaInfo is the JSON-serializable representation of the compiled schema.
type SchemaInfo struct {
	Fingerprint string       `json:"fingerprint"`
	EntityCount int          `json:"entity_count"`
	RouteCount  int          `json:"route_count"`
	Entities    []EntityInfo `json:"entities"`
}

// EntityInfo describes a single entity within the compiled schema.
type EntityInfo struct {
	Name        string      `json:"name"`
	Module      string      `json:"module"`
	Label       string      `json:"label"`
	IsSystem    bool        `json:"is_system"`
	TableName   string      `json:"table_name"`
	FieldCount  int         `json:"field_count"`
	Fields      []FieldInfo `json:"fields"`
	EdgeCount   int         `json:"edge_count"`
	ActionCount int         `json:"action_count"`
	Routes      []string    `json:"routes"`
}

// FieldInfo describes a single field on an entity.
type FieldInfo struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Required   bool   `json:"required,omitempty"`
	Immutable  bool   `json:"immutable,omitempty"`
	Sensitive  bool   `json:"sensitive,omitempty"`
	Searchable bool   `json:"searchable,omitempty"`
}

// Inspect builds a SchemaInfo from a compiled schema.
func Inspect(s *compiler.CompiledSchema) SchemaInfo {
	info := SchemaInfo{
		Fingerprint: compiler.Fingerprint(s),
		EntityCount: len(s.Entities),
		RouteCount:  len(s.Routes),
	}

	// Build per-entity route index.
	routesByEntity := make(map[string][]string)
	for _, r := range s.Routes {
		routesByEntity[r.EntityQualifiedName] = append(routesByEntity[r.EntityQualifiedName], r.Method+" "+r.Path)
	}

	for _, es := range s.Entities {
		var fieldInfos []FieldInfo
		for _, f := range es.Fields {
			fieldInfos = append(fieldInfos, FieldInfo{
				Name:       f.Name,
				Type:       string(f.Type),
				Required:   f.Required,
				Immutable:  f.Immutable,
				Sensitive:  f.Sensitive,
				Searchable: f.Searchable,
			})
		}

		info.Entities = append(info.Entities, EntityInfo{
			Name:        es.QualifiedName,
			Module:      es.Module,
			Label:       es.Label,
			IsSystem:    es.IsSystem,
			TableName:   es.TableName,
			FieldCount:  len(es.Fields),
			Fields:      fieldInfos,
			EdgeCount:   len(es.Edges),
			ActionCount: len(es.Actions),
			Routes:      routesByEntity[es.QualifiedName],
		})
	}

	return info
}

// Handler returns a Fiber handler that serves the SchemaInfo as JSON.
// Only wire this into non-production environments or behind platform-admin auth.
func Handler(s *compiler.CompiledSchema) fiber.Handler {
	// Compute once at startup — schema is immutable after Compile.
	info := Inspect(s)
	data, _ := json.Marshal(info)

	return func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSONCharsetUTF8)
		return c.Send(data)
	}
}
