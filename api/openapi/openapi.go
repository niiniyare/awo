// Package openapi generates OpenAPI 3.1 schemas from a CompiledSchema.
//
// The generated spec is served at GET /api/openapi.json. It covers all
// auto-generated CRUD and action routes. Custom routes added by module authors
// outside the framework are not included automatically.
//
// Output format: OpenAPI 3.1.0 JSON (map[string]any for flexibility;
// marshal with encoding/json for wire format).
package openapi

import (
	"fmt"

	"awo.so/awo/compiler"
	"awo.so/awo/def"
)

// Generate produces an OpenAPI 3.1 document from the compiled schema.
func Generate(schema *compiler.CompiledSchema, serverURL string) map[string]any {
	paths := map[string]any{}
	components := map[string]any{
		"schemas":         map[string]any{},
		"securitySchemes": bearerAuth(),
	}

	for _, es := range schema.Entities {
		entityPaths(es, paths, components["schemas"].(map[string]any))
	}

	return map[string]any{
		"openapi": "3.1.0",
		"info": map[string]any{
			"title":   "Awo Framework API",
			"version": "1.0.0",
		},
		"servers": []map[string]any{
			{"url": serverURL},
		},
		"paths":      paths,
		"components": components,
		"security":   []map[string]any{{"bearerAuth": []string{}}},
	}
}

func entityPaths(es *compiler.EntitySchema, paths, schemas map[string]any) {
	name := es.TableName
	schemaName := toPascalCase(name)
	// Use the entity's canonical RoutePrefix (e.g. /api/v1/finance/invoices)
	// rather than the generic /api/v1/entities/{table} pattern, which does not
	// match any registered Fiber route.
	base := es.RoutePrefix

	// Register schema component.
	schemas[schemaName] = entitySchema(es)

	ref := map[string]any{"$ref": "#/components/schemas/" + schemaName}
	listRef := map[string]any{
		"type":  "array",
		"items": ref,
	}

	// Collection routes.
	paths[base] = map[string]any{
		"get": map[string]any{
			"summary":     "List " + schemaName,
			"operationId": "list_" + name,
			"tags":        []string{schemaName},
			"parameters":  paginationParams(),
			"responses": map[string]any{
				"200": jsonResponse("Paginated list", listRef),
				"401": errorResponse("Unauthorized"),
				"403": errorResponse("Forbidden"),
			},
		},
		"post": map[string]any{
			"summary":     "Create " + schemaName,
			"operationId": "create_" + name,
			"tags":        []string{schemaName},
			"requestBody": jsonBody("Create payload", ref),
			"responses": map[string]any{
				"201": jsonResponse("Created", ref),
				"422": errorResponse("Validation error"),
			},
		},
	}

	// Single-record routes.
	paths[base+"/{id}"] = map[string]any{
		"get": map[string]any{
			"summary":     "Get " + schemaName,
			"operationId": "get_" + name,
			"tags":        []string{schemaName},
			"parameters":  idParam(),
			"responses": map[string]any{
				"200": jsonResponse("Record", ref),
				"404": errorResponse("Not found"),
			},
		},
		"patch": map[string]any{
			"summary":     "Update " + schemaName,
			"operationId": "update_" + name,
			"tags":        []string{schemaName},
			"parameters":  idParam(),
			"requestBody": jsonBody("Partial update", ref),
			"responses": map[string]any{
				"200": jsonResponse("Updated record", ref),
				"422": errorResponse("Validation error"),
			},
		},
		"delete": map[string]any{
			"summary":     "Delete " + schemaName,
			"operationId": "delete_" + name,
			"tags":        []string{schemaName},
			"parameters":  idParam(),
			"responses": map[string]any{
				"204": map[string]any{"description": "Deleted"},
				"404": errorResponse("Not found"),
			},
		},
	}

	// Action routes.
	for actionName, action := range es.ActionsByName {
		opID := fmt.Sprintf("action_%s_%s", name, actionName)
		paths[fmt.Sprintf("%s/{id}/%s", base, actionName)] = map[string]any{
			"post": map[string]any{
				"summary":     action.Label,
				"operationId": opID,
				"tags":        []string{schemaName},
				"parameters":  idParam(),
				"responses": map[string]any{
					"200": jsonResponse("Action result", map[string]any{"type": "object"}),
				},
			},
		}
	}
}

func entitySchema(es *compiler.EntitySchema) map[string]any {
	props := map[string]any{
		"id":         map[string]any{"type": "string", "format": "uuid"},
		"tenant_id":  map[string]any{"type": "string", "format": "uuid"},
		"created_at": map[string]any{"type": "string", "format": "date-time"},
		"updated_at": map[string]any{"type": "string", "format": "date-time"},
	}
	required := []string{}

	for _, f := range es.Fields {
		if f.Sensitive {
			continue // never expose sensitive fields in API docs
		}
		prop := fieldSchema(f)
		props[f.Name] = prop
		if f.Required {
			required = append(required, f.Name)
		}
	}

	s := map[string]any{
		"type":       "object",
		"properties": props,
	}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

func fieldSchema(f def.FieldDef) map[string]any {
	switch f.Type {
	case def.FieldTypeBool:
		return map[string]any{"type": "boolean"}
	case def.FieldTypeInt:
		return map[string]any{"type": "integer", "format": "int64"}
	case def.FieldTypeFloat:
		return map[string]any{"type": "number", "format": "double"}
	case def.FieldTypeCurrency:
		return map[string]any{"type": "string", "description": "Decimal string (numeric(20,4))"}
	case def.FieldTypeDate:
		return map[string]any{"type": "string", "format": "date"}
	case def.FieldTypeDateTime:
		return map[string]any{"type": "string", "format": "date-time"}
	case def.FieldTypeSelect:
		schema := map[string]any{"type": "string"}
		if len(f.Options) > 0 {
			enums := make([]any, len(f.Options))
			for i, o := range f.Options {
				enums[i] = o
			}
			schema["enum"] = enums
		}
		return schema
	case def.FieldTypeLink:
		return map[string]any{"type": "string", "format": "uuid", "description": "FK → " + f.LinkTarget}
	case def.FieldTypeJSON:
		return map[string]any{"type": "object"}
	default:
		s := map[string]any{"type": "string"}
		if f.MaxLen > 0 {
			s["maxLength"] = f.MaxLen
		}
		return s
	}
}

func paginationParams() []map[string]any {
	return []map[string]any{
		{"name": "page", "in": "query", "schema": map[string]any{"type": "integer", "default": 1}},
		{"name": "page_size", "in": "query", "schema": map[string]any{"type": "integer", "default": 20}},
	}
}

func idParam() []map[string]any {
	return []map[string]any{
		{
			"name":     "id",
			"in":       "path",
			"required": true,
			"schema":   map[string]any{"type": "string", "format": "uuid"},
		},
	}
}

func jsonResponse(desc string, schema map[string]any) map[string]any {
	return map[string]any{
		"description": desc,
		"content": map[string]any{
			"application/json": map[string]any{
				"schema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"data": schema,
					},
				},
			},
		},
	}
}

func jsonBody(desc string, schema map[string]any) map[string]any {
	return map[string]any{
		"description": desc,
		"required":    true,
		"content": map[string]any{
			"application/json": map[string]any{
				"schema": schema,
			},
		},
	}
}

func errorResponse(desc string) map[string]any {
	return map[string]any{
		"description": desc,
		"content": map[string]any{
			"application/json": map[string]any{
				"schema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"error": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"code":    map[string]any{"type": "string"},
								"message": map[string]any{"type": "string"},
							},
						},
					},
				},
			},
		},
	}
}

func bearerAuth() map[string]any {
	return map[string]any{
		"bearerAuth": map[string]any{
			"type":         "http",
			"scheme":       "bearer",
			"bearerFormat": "token",
		},
	}
}

// toPascalCase converts snake_case to PascalCase (e.g. finance_invoice → FinanceInvoice).
func toPascalCase(s string) string {
	parts := splitUnderscore(s)
	result := ""
	for _, p := range parts {
		if len(p) > 0 {
			result += string(p[0]-32) + p[1:] // uppercase first char
		}
	}
	return result
}

func splitUnderscore(s string) []string {
	var parts []string
	start := 0
	for i, c := range s {
		if c == '_' {
			if i > start {
				parts = append(parts, s[start:i])
			}
			start = i + 1
		}
	}
	if start < len(s) {
		parts = append(parts, s[start:])
	}
	return parts
}
