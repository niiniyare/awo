package openapi_test

import (
	"strings"
	"testing"

	"awo.so/awo/api/openapi"
	"awo.so/awo/compiler"
	"awo.so/awo/def"
	"awo.so/awo/registry"
)

func buildSchema(t *testing.T, defs ...def.EntityDefinition) *compiler.CompiledSchema {
	t.Helper()
	reg, err := registry.BuildFrom(defs)
	if err != nil {
		t.Fatalf("registry.BuildFrom: %v", err)
	}
	cs, err := compiler.Compile(reg)
	if err != nil {
		t.Fatalf("compiler.Compile: %v", err)
	}
	return cs
}

func entity(name, module string, fields ...def.FieldDef) *def.SystemDefinition {
	return &def.SystemDefinition{Name: name, Module: module, Fields: fields}
}

func TestGenerate_TopLevelKeys(t *testing.T) {
	cs := buildSchema(t, entity("widget", "test"))
	doc := openapi.Generate(cs, "https://api.example.com")
	for _, key := range []string{"openapi", "info", "servers", "paths", "components", "security"} {
		if _, ok := doc[key]; !ok {
			t.Errorf("missing top-level key %q", key)
		}
	}
}

func TestGenerate_OpenAPIVersion(t *testing.T) {
	cs := buildSchema(t, entity("widget", "test"))
	doc := openapi.Generate(cs, "")
	if doc["openapi"] != "3.1.0" {
		t.Errorf("expected openapi 3.1.0, got %v", doc["openapi"])
	}
}

func TestGenerate_ServerURL(t *testing.T) {
	cs := buildSchema(t, entity("widget", "test"))
	doc := openapi.Generate(cs, "https://erp.example.com")
	servers, ok := doc["servers"].([]map[string]any)
	if !ok || len(servers) == 0 {
		t.Fatal("servers list missing or empty")
	}
	if servers[0]["url"] != "https://erp.example.com" {
		t.Errorf("server url mismatch: %v", servers[0]["url"])
	}
}

func TestGenerate_BearerAuth(t *testing.T) {
	cs := buildSchema(t, entity("widget", "test"))
	doc := openapi.Generate(cs, "")
	comps, ok := doc["components"].(map[string]any)
	if !ok {
		t.Fatal("components missing")
	}
	schemes, ok := comps["securitySchemes"].(map[string]any)
	if !ok {
		t.Fatal("securitySchemes missing")
	}
	if _, ok := schemes["bearerAuth"]; !ok {
		t.Error("bearerAuth scheme missing")
	}
}

func TestGenerate_EntitySchema_InComponents(t *testing.T) {
	cs := buildSchema(t, entity("invoice", "finance"))
	doc := openapi.Generate(cs, "")
	comps := doc["components"].(map[string]any)
	schemas := comps["schemas"].(map[string]any)
	if _, ok := schemas["FinanceInvoice"]; !ok {
		t.Error("expected FinanceInvoice schema in components")
	}
}

func TestGenerate_EntityPaths_CollectionAndRecord(t *testing.T) {
	cs := buildSchema(t, entity("invoice", "finance"))
	doc := openapi.Generate(cs, "")
	paths := doc["paths"].(map[string]any)

	// At minimum: collection path and /{id} path must exist.
	var hasCollection, hasRecord bool
	for p := range paths {
		if strings.Contains(p, "invoice") && !strings.Contains(p, "{id}") {
			hasCollection = true
		}
		if strings.Contains(p, "invoice") && strings.Contains(p, "{id}") {
			hasRecord = true
		}
	}
	if !hasCollection {
		t.Error("no collection path for finance_invoice")
	}
	if !hasRecord {
		t.Error("no /{id} path for finance_invoice")
	}
}

func TestGenerate_EntityPaths_CRUDVerbs(t *testing.T) {
	cs := buildSchema(t, entity("invoice", "finance"))
	doc := openapi.Generate(cs, "")
	paths := doc["paths"].(map[string]any)

	// Find collection path (has GET+POST).
	for p, v := range paths {
		if !strings.Contains(p, "invoice") || strings.Contains(p, "{id}") {
			continue
		}
		ops := v.(map[string]any)
		for _, verb := range []string{"get", "post"} {
			if _, ok := ops[verb]; !ok {
				t.Errorf("collection path %q missing verb %q", p, verb)
			}
		}
	}

	// Find /{id} path (has GET+PATCH+DELETE).
	for p, v := range paths {
		if !strings.Contains(p, "invoice") || !strings.Contains(p, "{id}") || strings.Contains(p, "/") {
			continue
		}
		ops := v.(map[string]any)
		for _, verb := range []string{"get", "patch", "delete"} {
			if _, ok := ops[verb]; !ok {
				t.Errorf("record path %q missing verb %q", p, verb)
			}
		}
		break
	}
}

func TestGenerate_ActionPaths(t *testing.T) {
	d := &def.SystemDefinition{
		Name:   "invoice",
		Module: "finance",
		Actions: []def.ActionDef{
			{Name: "submit", Label: "Submit Invoice", HandlerFunc: func(_ *def.ActionContext) (*def.ActionResult, error) { return nil, nil }},
		},
	}
	cs := buildSchema(t, d)
	doc := openapi.Generate(cs, "")
	paths := doc["paths"].(map[string]any)

	var found bool
	for p := range paths {
		if strings.Contains(p, "submit") {
			found = true
			break
		}
	}
	if !found {
		t.Error("action path for 'submit' not found in paths")
	}
}

func TestGenerate_SensitiveFields_Excluded(t *testing.T) {
	d := entity("user", "iam",
		def.FieldDef{Name: "email", Type: def.FieldTypeData},
		def.FieldDef{Name: "password_hash", Type: def.FieldTypeData, Sensitive: true},
	)
	cs := buildSchema(t, d)
	doc := openapi.Generate(cs, "")
	comps := doc["components"].(map[string]any)
	schemas := comps["schemas"].(map[string]any)
	iamUser := schemas["IamUser"].(map[string]any)
	props := iamUser["properties"].(map[string]any)
	if _, ok := props["password_hash"]; ok {
		t.Error("sensitive field password_hash should be excluded from OpenAPI schema")
	}
	if _, ok := props["email"]; !ok {
		t.Error("non-sensitive field email should be present")
	}
}

func TestGenerate_RequiredFields_Marked(t *testing.T) {
	d := entity("item", "test",
		def.FieldDef{Name: "name", Type: def.FieldTypeData, Required: true},
		def.FieldDef{Name: "note", Type: def.FieldTypeData},
	)
	cs := buildSchema(t, d)
	doc := openapi.Generate(cs, "")
	comps := doc["components"].(map[string]any)
	schemas := comps["schemas"].(map[string]any)
	testItem := schemas["TestItem"].(map[string]any)
	req, ok := testItem["required"]
	if !ok {
		t.Fatal("required array missing from schema")
	}
	reqs := req.([]string)
	found := false
	for _, r := range reqs {
		if r == "name" {
			found = true
		}
	}
	if !found {
		t.Error("required field 'name' not in required array")
	}
}

func TestGenerate_FieldSchema_BoolType(t *testing.T) {
	d := entity("item", "test", def.FieldDef{Name: "active", Type: def.FieldTypeBool})
	cs := buildSchema(t, d)
	doc := openapi.Generate(cs, "")
	props := entityProps(t, doc, "TestItem")
	active, ok := props["active"].(map[string]any)
	if !ok {
		t.Fatal("active property missing")
	}
	if active["type"] != "boolean" {
		t.Errorf("bool field should have type=boolean, got %v", active["type"])
	}
}

func TestGenerate_FieldSchema_IntType(t *testing.T) {
	d := entity("item", "test", def.FieldDef{Name: "qty", Type: def.FieldTypeInt})
	cs := buildSchema(t, d)
	doc := openapi.Generate(cs, "")
	props := entityProps(t, doc, "TestItem")
	qty := props["qty"].(map[string]any)
	if qty["type"] != "integer" {
		t.Errorf("int field should have type=integer, got %v", qty["type"])
	}
}

func TestGenerate_FieldSchema_SelectWithEnum(t *testing.T) {
	d := entity("item", "test", def.FieldDef{
		Name: "status", Type: def.FieldTypeSelect, Options: []string{"draft", "active"},
	})
	cs := buildSchema(t, d)
	doc := openapi.Generate(cs, "")
	props := entityProps(t, doc, "TestItem")
	status := props["status"].(map[string]any)
	if _, ok := status["enum"]; !ok {
		t.Error("select field with options should have enum")
	}
}

func TestGenerate_FieldSchema_LinkType(t *testing.T) {
	target := entity("category", "test")
	source := &def.SystemDefinition{
		Name: "item", Module: "test",
		Fields: []def.FieldDef{
			{Name: "category_id", Type: def.FieldTypeLink, LinkTarget: "test_category"},
		},
	}
	cs := buildSchema(t, target, source)
	doc := openapi.Generate(cs, "")
	props := entityProps(t, doc, "TestItem")
	catField := props["category_id"].(map[string]any)
	if catField["format"] != "uuid" {
		t.Error("link field should have format=uuid")
	}
	desc, _ := catField["description"].(string)
	if !strings.Contains(desc, "test_category") {
		t.Errorf("link field description should reference target, got %q", desc)
	}
}

func TestGenerate_MultipleEntities_AllInPaths(t *testing.T) {
	cs := buildSchema(t,
		entity("invoice", "finance"),
		entity("payment", "finance"),
	)
	doc := openapi.Generate(cs, "")
	paths := doc["paths"].(map[string]any)
	var hasInvoice, hasPayment bool
	for p := range paths {
		if strings.Contains(p, "invoice") {
			hasInvoice = true
		}
		if strings.Contains(p, "payment") {
			hasPayment = true
		}
	}
	if !hasInvoice {
		t.Error("invoice paths missing")
	}
	if !hasPayment {
		t.Error("payment paths missing")
	}
}

// entityProps extracts properties map from a named component schema.
func entityProps(t *testing.T, doc map[string]any, schemaName string) map[string]any {
	t.Helper()
	comps := doc["components"].(map[string]any)
	schemas := comps["schemas"].(map[string]any)
	s, ok := schemas[schemaName].(map[string]any)
	if !ok {
		t.Fatalf("schema %q not found in components", schemaName)
	}
	props, ok := s["properties"].(map[string]any)
	if !ok {
		t.Fatalf("schema %q has no properties", schemaName)
	}
	return props
}
