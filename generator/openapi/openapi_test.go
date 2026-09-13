package openapi_test

import (
	"strings"
	"testing"

	"awo.so/awo/compiler"
	"awo.so/awo/def"
	"awo.so/awo/generator/openapi"
	"awo.so/awo/registry"
)

// ── test schema helpers ───────────────────────────────────────────────────────

func testEntity() def.EntityDefinition {
	return &def.SystemDefinition{
		Name:        "invoice",
		Module:      "finance",
		Label:       "Invoice",
		LabelPlural: "Invoices",
		Description: "A customer invoice.",
		Fields: []def.FieldDef{
			{Name: "number", Type: def.FieldTypeData, Label: "Number", Required: true},
			{Name: "status", Type: def.FieldTypeSelect, Label: "Status",
				Options: []string{"draft", "submitted", "paid"}, Required: true},
			{Name: "amount", Type: def.FieldTypeCurrency, Label: "Amount", Required: true},
			{Name: "notes", Type: def.FieldTypeSmallText, Label: "Notes"},
			{Name: "secret_key", Type: def.FieldTypeData, Label: "Secret Key", Sensitive: true},
		},
		Permissions: def.PermissionSet{
			Create: []string{"finance.invoice.create"},
			Read:   []string{"finance.invoice.read"},
			Write:  []string{"finance.invoice.update"},
			Delete: []string{"finance.invoice.delete"},
		},
		Actions: []def.ActionDef{
			{
				Name:        "submit",
				Label:       "Submit",
				HandlerFunc: func(_ *def.ActionContext) (*def.ActionResult, error) { return nil, nil },
			},
		},
	}
}

func buildSchema(t *testing.T, entities ...def.EntityDefinition) *compiler.CompiledSchema {
	t.Helper()
	reg, err := registry.BuildFrom(entities)
	if err != nil {
		t.Fatalf("BuildFrom: %v", err)
	}
	schema, err := compiler.Compile(reg)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	return schema
}

func generate(t *testing.T, entities ...def.EntityDefinition) *openapi.Spec {
	t.Helper()
	schema := buildSchema(t, entities...)
	spec, err := openapi.Generate(schema, openapi.Options{Title: "Test API", Version: "1.0.0"})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	return spec
}

// ── spec structure tests ──────────────────────────────────────────────────────

func TestGenerate_OpenAPIVersion(t *testing.T) {
	spec := generate(t, testEntity())
	if spec.OpenAPI != "3.0.3" {
		t.Errorf("OpenAPI version: got %q, want %q", spec.OpenAPI, "3.0.3")
	}
}

func TestGenerate_InfoTitle(t *testing.T) {
	spec := generate(t, testEntity())
	if spec.Info.Title != "Test API" {
		t.Errorf("Info.Title: got %q, want %q", spec.Info.Title, "Test API")
	}
}

func TestGenerate_DefaultTitle(t *testing.T) {
	schema := buildSchema(t, testEntity())
	spec, _ := openapi.Generate(schema, openapi.Options{})
	if spec.Info.Title != "AwoERP API" {
		t.Errorf("default title: got %q", spec.Info.Title)
	}
}

func TestGenerate_BearerAuthScheme(t *testing.T) {
	spec := generate(t, testEntity())
	scheme, ok := spec.Components.SecuritySchemes["BearerAuth"]
	if !ok {
		t.Fatal("BearerAuth security scheme missing")
	}
	if scheme.Type != "http" || scheme.Scheme != "bearer" {
		t.Errorf("BearerAuth: type=%q scheme=%q", scheme.Type, scheme.Scheme)
	}
}

// ── paths tests ───────────────────────────────────────────────────────────────

func TestGenerate_EntityPresentInPaths(t *testing.T) {
	spec := generate(t, testEntity())
	found := false
	for path := range spec.Paths {
		if strings.Contains(path, "invoices") {
			found = true
			break
		}
	}
	if !found {
		t.Error("finance_invoice routes not found in paths")
	}
}

func TestGenerate_CRUDPathsGenerated(t *testing.T) {
	spec := generate(t, testEntity())

	// Find collection and item paths.
	var collectionPath, itemPath string
	for path := range spec.Paths {
		if strings.Contains(path, "invoices") {
			if strings.HasSuffix(path, "{id}") {
				itemPath = path
			} else if !strings.Contains(path, "{id}") {
				collectionPath = path
			}
		}
	}

	if collectionPath == "" {
		t.Error("collection path (no {id}) not found")
	}
	if itemPath == "" {
		t.Error("item path ({id}) not found")
	}

	col := spec.Paths[collectionPath]
	if col.Get == nil {
		t.Error("collection GET (list) missing")
	}
	if col.Post == nil {
		t.Error("collection POST (create) missing")
	}

	item := spec.Paths[itemPath]
	if item.Get == nil {
		t.Error("item GET missing")
	}
	if item.Patch == nil {
		t.Error("item PATCH missing")
	}
	if item.Delete == nil {
		t.Error("item DELETE missing")
	}
}

func TestGenerate_ActionPathGenerated(t *testing.T) {
	spec := generate(t, testEntity())
	found := false
	for path, item := range spec.Paths {
		if strings.Contains(path, "submit") && item.Post != nil {
			found = true
			break
		}
	}
	if !found {
		t.Error("submit action path not found")
	}
}

func TestGenerate_FiberPathConvertedToOpenAPI(t *testing.T) {
	spec := generate(t, testEntity())
	for path := range spec.Paths {
		if strings.Contains(path, ":") {
			t.Errorf("Fiber path syntax leaked into OpenAPI: %q", path)
		}
	}
}

// ── schema component tests ────────────────────────────────────────────────────

func TestGenerate_EntitySchemaInComponents(t *testing.T) {
	spec := generate(t, testEntity())
	if _, ok := spec.Components.Schemas["finance_invoice"]; !ok {
		t.Error("finance_invoice schema missing from components")
	}
	if _, ok := spec.Components.Schemas["finance_invoiceInput"]; !ok {
		t.Error("finance_invoiceInput schema missing from components")
	}
}

func TestGenerate_SensitiveFieldExcludedFromResponseSchema(t *testing.T) {
	spec := generate(t, testEntity())
	schema := spec.Components.Schemas["finance_invoice"]
	if schema == nil {
		t.Fatal("finance_invoice schema missing")
	}
	if _, ok := schema.Properties["secret_key"]; ok {
		t.Error("sensitive field secret_key must NOT appear in response schema")
	}
}

func TestGenerate_SensitiveFieldIncludedInInputSchema(t *testing.T) {
	spec := generate(t, testEntity())
	schema := spec.Components.Schemas["finance_invoiceInput"]
	if schema == nil {
		t.Fatal("finance_invoiceInput schema missing")
	}
	if _, ok := schema.Properties["secret_key"]; !ok {
		t.Error("sensitive field secret_key MUST appear in input schema (write-only)")
	}
}

func TestGenerate_RequiredFieldsMarkedInInputSchema(t *testing.T) {
	spec := generate(t, testEntity())
	schema := spec.Components.Schemas["finance_invoiceInput"]
	if schema == nil {
		t.Fatal("finance_invoiceInput schema missing")
	}
	requiredSet := make(map[string]bool, len(schema.Required))
	for _, r := range schema.Required {
		requiredSet[r] = true
	}
	for _, name := range []string{"number", "status", "amount"} {
		if !requiredSet[name] {
			t.Errorf("required field %q not marked as required in input schema", name)
		}
	}
}

func TestGenerate_SelectFieldHasEnum(t *testing.T) {
	spec := generate(t, testEntity())
	schema := spec.Components.Schemas["finance_invoice"]
	if schema == nil {
		t.Fatal("finance_invoice schema missing")
	}
	statusSchema, ok := schema.Properties["status"]
	if !ok {
		t.Fatal("status field missing from schema")
	}
	if len(statusSchema.Enum) != 3 {
		t.Errorf("status enum: got %d values, want 3", len(statusSchema.Enum))
	}
}

func TestGenerate_CurrencyFieldIsStringDecimal(t *testing.T) {
	spec := generate(t, testEntity())
	schema := spec.Components.Schemas["finance_invoice"]
	if schema == nil {
		t.Fatal("finance_invoice schema missing")
	}
	amountSchema, ok := schema.Properties["amount"]
	if !ok {
		t.Fatal("amount field missing from schema")
	}
	if amountSchema.Type != "string" || amountSchema.Format != "decimal" {
		t.Errorf("currency field: type=%q format=%q, want string/decimal", amountSchema.Type, amountSchema.Format)
	}
}

func TestGenerate_MultipleEntitiesAllInPaths(t *testing.T) {
	e2 := &def.SystemDefinition{
		Name:        "payment",
		Module:      "finance",
		Label:       "Payment",
		LabelPlural: "Payments",
		Fields:      []def.FieldDef{{Name: "amount", Type: def.FieldTypeCurrency, Required: true}},
		Permissions: def.PermissionSet{Read: []string{"finance.payment.read"}},
	}
	spec := generate(t, testEntity(), e2)

	invoiceFound, paymentFound := false, false
	for path := range spec.Paths {
		if strings.Contains(path, "invoices") {
			invoiceFound = true
		}
		if strings.Contains(path, "payments") {
			paymentFound = true
		}
	}
	if !invoiceFound {
		t.Error("invoice paths missing")
	}
	if !paymentFound {
		t.Error("payment paths missing")
	}
}
