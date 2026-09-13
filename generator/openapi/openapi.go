// Package openapi generates an OpenAPI 3.0.3 specification from a
// [compiler.CompiledSchema].
//
// One path item is emitted per entity CRUD route and per entity action. One
// component schema is emitted per entity. Sensitive fields are excluded from
// response schemas (but included in request bodies).
//
// Usage:
//
//	spec, err := openapi.Generate(schema, openapi.Options{Title: "AwoERP API", Version: "1.0.0"})
//	if err != nil { ... }
//	data, _ := json.MarshalIndent(spec, "", "  ")
package openapi

import (
	"strings"

	"awo.so/awo/compiler"
	"awo.so/awo/def"
)

// Options configures the generator.
type Options struct {
	// Title is the API info title. Defaults to "AwoERP API".
	Title string
	// Version is the API version string. Defaults to "0.1.0".
	Version string
	// Description is the optional API description.
	Description string
}

// Spec is the root OpenAPI 3.0.3 document.
type Spec struct {
	OpenAPI    string              `json:"openapi"`
	Info       Info                `json:"info"`
	Paths      map[string]PathItem `json:"paths"`
	Components Components          `json:"components"`
	Security   []SecurityReq       `json:"security,omitempty"`
}

// Info is the OpenAPI info object.
type Info struct {
	Title       string `json:"title"`
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
}

// PathItem holds the operations for a single OpenAPI path.
type PathItem struct {
	Get    *Operation `json:"get,omitempty"`
	Post   *Operation `json:"post,omitempty"`
	Patch  *Operation `json:"patch,omitempty"`
	Delete *Operation `json:"delete,omitempty"`
}

// Operation is a single HTTP operation on a path.
type Operation struct {
	Summary     string              `json:"summary,omitempty"`
	Description string              `json:"description,omitempty"`
	OperationID string              `json:"operationId,omitempty"`
	Tags        []string            `json:"tags,omitempty"`
	Parameters  []Parameter         `json:"parameters,omitempty"`
	RequestBody *RequestBody        `json:"requestBody,omitempty"`
	Responses   map[string]Response `json:"responses"`
	Security    []SecurityReq       `json:"security,omitempty"`
}

// Parameter is a path or query parameter.
type Parameter struct {
	Name        string  `json:"name"`
	In          string  `json:"in"`
	Required    bool    `json:"required"`
	Description string  `json:"description,omitempty"`
	Schema      *Schema `json:"schema,omitempty"`
}

// RequestBody describes the request payload.
type RequestBody struct {
	Required bool                   `json:"required"`
	Content  map[string]MediaType   `json:"content"`
}

// MediaType wraps a schema for a content type.
type MediaType struct {
	Schema *Schema `json:"schema"`
}

// Response is a single response object.
type Response struct {
	Description string                 `json:"description"`
	Content     map[string]MediaType   `json:"content,omitempty"`
}

// Schema is an OpenAPI/JSON Schema object.
type Schema struct {
	Type        string             `json:"type,omitempty"`
	Format      string             `json:"format,omitempty"`
	Description string             `json:"description,omitempty"`
	Properties  map[string]*Schema `json:"properties,omitempty"`
	Required    []string           `json:"required,omitempty"`
	Items       *Schema            `json:"items,omitempty"`
	Ref         string             `json:"$ref,omitempty"`
	Enum        []any              `json:"enum,omitempty"`
	ReadOnly    bool               `json:"readOnly,omitempty"`
}

// Components holds reusable schema and security scheme definitions.
type Components struct {
	Schemas         map[string]*Schema        `json:"schemas,omitempty"`
	SecuritySchemes map[string]SecurityScheme `json:"securitySchemes,omitempty"`
}

// SecurityScheme defines an authentication mechanism.
type SecurityScheme struct {
	Type         string `json:"type"`
	Scheme       string `json:"scheme,omitempty"`
	BearerFormat string `json:"bearerFormat,omitempty"`
	Description  string `json:"description,omitempty"`
}

// SecurityReq maps a security scheme name to its required scopes.
type SecurityReq map[string][]string

// Generate produces an OpenAPI 3.0.3 Spec from the compiled schema.
// All routes emitted by the compiler are included. Sensitive fields are
// excluded from response schemas. Required fields are marked in request
// body schemas.
func Generate(schema *compiler.CompiledSchema, opts Options) (*Spec, error) {
	if opts.Title == "" {
		opts.Title = "AwoERP API"
	}
	if opts.Version == "" {
		opts.Version = "0.1.0"
	}

	spec := &Spec{
		OpenAPI: "3.0.3",
		Info: Info{
			Title:       opts.Title,
			Version:     opts.Version,
			Description: opts.Description,
		},
		Paths: make(map[string]PathItem),
		Components: Components{
			Schemas: make(map[string]*Schema),
			SecuritySchemes: map[string]SecurityScheme{
				"BearerAuth": {
					Type:         "http",
					Scheme:       "bearer",
					BearerFormat: "JWT",
					Description:  "Session token obtained from POST /api/v1/iam/auth/login",
				},
			},
		},
		Security: []SecurityReq{{"BearerAuth": {}}},
	}

	// Build one schema component per entity.
	for _, es := range schema.Entities {
		spec.Components.Schemas[es.QualifiedName] = entityResponseSchema(es)
		spec.Components.Schemas[es.QualifiedName+"Input"] = entityInputSchema(es)
	}

	// Build path items from compiled routes.
	for _, route := range schema.Routes {
		es := schema.ByName[route.EntityQualifiedName]
		if es == nil {
			continue
		}
		oaPath := fiberPathToOpenAPI(route.Path)
		item := spec.Paths[oaPath]

		op := buildOperation(route, es)
		switch strings.ToUpper(route.Method) {
		case "GET":
			item.Get = op
		case "POST":
			item.Post = op
		case "PATCH":
			item.Patch = op
		case "DELETE":
			item.Delete = op
		}
		spec.Paths[oaPath] = item
	}

	return spec, nil
}

// fiberPathToOpenAPI converts Fiber route syntax (:id) to OpenAPI syntax ({id}).
func fiberPathToOpenAPI(path string) string {
	parts := strings.Split(path, "/")
	for i, p := range parts {
		if strings.HasPrefix(p, ":") {
			parts[i] = "{" + p[1:] + "}"
		}
	}
	return strings.Join(parts, "/")
}

// buildOperation constructs an Operation for the given route.
func buildOperation(route compiler.RouteDescriptor, es *compiler.EntitySchema) *Operation {
	op := &Operation{
		Tags:    []string{es.OpenAPITag},
		Security: []SecurityReq{{"BearerAuth": {}}},
	}

	switch route.Operation {
	case "list":
		op.Summary = "List " + es.LabelPlural
		op.OperationID = "list" + pascal(es.QualifiedName)
		op.Responses = jsonResponses("200", "List of "+es.LabelPlural, arrayRef(es.QualifiedName))

	case "get":
		op.Summary = "Get " + es.Label
		op.OperationID = "get" + pascal(es.QualifiedName)
		op.Parameters = []Parameter{idParam()}
		op.Responses = jsonResponses("200", es.Label+" record", ref(es.QualifiedName))

	case "create":
		op.Summary = "Create " + es.Label
		op.OperationID = "create" + pascal(es.QualifiedName)
		op.RequestBody = inputBody(es.QualifiedName)
		op.Responses = jsonResponses("201", "Created "+es.Label, ref(es.QualifiedName))

	case "update":
		op.Summary = "Update " + es.Label
		op.OperationID = "update" + pascal(es.QualifiedName)
		op.Parameters = []Parameter{idParam()}
		op.RequestBody = inputBody(es.QualifiedName)
		op.Responses = jsonResponses("200", "Updated "+es.Label, ref(es.QualifiedName))

	case "delete":
		op.Summary = "Delete " + es.Label
		op.OperationID = "delete" + pascal(es.QualifiedName)
		op.Parameters = []Parameter{idParam()}
		op.Responses = map[string]Response{
			"204": {Description: "Deleted"},
			"404": {Description: "Not found"},
		}

	case "action":
		op.Summary = route.ActionName + " " + es.Label
		op.OperationID = route.ActionName + pascal(es.QualifiedName)
		op.Parameters = []Parameter{idParam()}
		op.Responses = jsonResponses("200", "Action result", nil)
	}

	return op
}

// entityResponseSchema builds the schema for GET responses (sensitive excluded).
func entityResponseSchema(es *compiler.EntitySchema) *Schema {
	props := make(map[string]*Schema)
	props["id"] = &Schema{Type: "string", Format: "uuid"}
	props["created_at"] = &Schema{Type: "string", Format: "date-time", ReadOnly: true}
	props["updated_at"] = &Schema{Type: "string", Format: "date-time", ReadOnly: true}

	for _, f := range es.Fields {
		if f.Sensitive {
			continue // excluded from response per security policy
		}
		props[f.Name] = fieldSchema(f)
	}

	return &Schema{
		Type:        "object",
		Description: es.Description,
		Properties:  props,
	}
}

// entityInputSchema builds the schema for POST/PATCH request bodies.
// Includes all non-ReadOnly fields; marks Required fields in required array.
// Sensitive fields ARE included in input (they are write-only).
func entityInputSchema(es *compiler.EntitySchema) *Schema {
	props := make(map[string]*Schema)
	var required []string

	for _, f := range es.Fields {
		if f.ReadOnly {
			continue
		}
		props[f.Name] = fieldSchema(f)
		if f.Required {
			required = append(required, f.Name)
		}
	}

	s := &Schema{
		Type:       "object",
		Properties: props,
	}
	if len(required) > 0 {
		s.Required = required
	}
	return s
}

// fieldSchema maps a FieldDef to an OpenAPI Schema.
func fieldSchema(f def.FieldDef) *Schema {
	s := &Schema{Description: f.Description}
	if f.ReadOnly {
		s.ReadOnly = true
	}

	switch f.Type {
	case def.FieldTypeData, def.FieldTypeSmallText, def.FieldTypeLongText:
		s.Type = "string"
	case def.FieldTypeInt:
		s.Type = "integer"
		s.Format = "int64"
	case def.FieldTypeFloat:
		s.Type = "number"
		s.Format = "double"
	case def.FieldTypeCurrency:
		s.Type = "string"
		s.Format = "decimal"
		s.Description = appendNote(s.Description, "numeric(20,4)")
	case def.FieldTypeBool:
		s.Type = "boolean"
	case def.FieldTypeDate:
		s.Type = "string"
		s.Format = "date"
	case def.FieldTypeDateTime:
		s.Type = "string"
		s.Format = "date-time"
	case def.FieldTypeTime:
		s.Type = "string"
		s.Format = "time"
	case def.FieldTypeSelect:
		s.Type = "string"
		if len(f.Options) > 0 {
			for _, o := range f.Options {
				s.Enum = append(s.Enum, o)
			}
		}
	case def.FieldTypeMultiSelect:
		s.Type = "array"
		s.Items = &Schema{Type: "string"}
	case def.FieldTypeNamingSeries:
		s.Type = "string"
		s.Description = appendNote(s.Description, "auto-generated naming series")
	case def.FieldTypeJSON:
		s.Type = "object"
	case def.FieldTypeLink:
		s.Type = "string"
		s.Format = "uuid"
		if f.LinkTarget != "" {
			s.Description = appendNote(s.Description, "FK → "+f.LinkTarget)
		}
	case def.FieldTypeLinkList:
		s.Type = "array"
		s.Items = &Schema{Type: "string", Format: "uuid"}
	case def.FieldTypeDynamicLink:
		s.Type = "string"
		s.Format = "uuid"
	default:
		s.Type = "string"
	}

	return s
}

// ref returns a $ref schema pointer to a component schema.
func ref(name string) *Schema {
	return &Schema{Ref: "#/components/schemas/" + name}
}

// arrayRef returns an array schema wrapping a $ref.
func arrayRef(name string) *Schema {
	return &Schema{Type: "array", Items: ref(name)}
}

// idParam returns the standard {id} path parameter.
func idParam() Parameter {
	return Parameter{
		Name:     "id",
		In:       "path",
		Required: true,
		Schema:   &Schema{Type: "string", Format: "uuid"},
	}
}

// inputBody wraps a schema reference in a required JSON request body.
func inputBody(entityName string) *RequestBody {
	return &RequestBody{
		Required: true,
		Content: map[string]MediaType{
			"application/json": {Schema: ref(entityName + "Input")},
		},
	}
}

// jsonResponses returns a standard responses map with a 200/201 JSON response
// and a 404 fallback. schema may be nil for action responses.
func jsonResponses(successCode, description string, schema *Schema) map[string]Response {
	r := map[string]Response{
		successCode: {
			Description: description,
		},
		"401": {Description: "Unauthorized"},
		"403": {Description: "Forbidden"},
	}
	if schema != nil {
		r[successCode] = Response{
			Description: description,
			Content: map[string]MediaType{
				"application/json": {Schema: schema},
			},
		}
	}
	return r
}

// pascal converts a snake_case qualified name to PascalCase.
// "finance_invoice" → "FinanceInvoice"
func pascal(s string) string {
	parts := strings.Split(s, "_")
	var b strings.Builder
	for _, p := range parts {
		if len(p) > 0 {
			b.WriteString(strings.ToUpper(p[:1]) + p[1:])
		}
	}
	return b.String()
}

// appendNote appends a parenthetical note to an existing description.
func appendNote(desc, note string) string {
	if desc == "" {
		return note
	}
	return desc + " (" + note + ")"
}
