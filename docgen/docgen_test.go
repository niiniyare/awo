package docgen_test

import (
	"strings"
	"testing"

	"awo.so/awo/compiler"
	"awo.so/awo/def"
	"awo.so/awo/registry"

	. "awo.so/awo/docgen"
)

// buildSchema compiles defs into a CompiledSchema.
func buildSchema(t *testing.T, defs []def.EntityDefinition) *compiler.CompiledSchema {
	t.Helper()
	reg, err := registry.BuildFrom(defs)
	if err != nil {
		t.Fatalf("registry.BuildFrom: %v", err)
	}
	schema, err := compiler.Compile(reg)
	if err != nil {
		t.Fatalf("compiler.Compile: %v", err)
	}
	return schema
}

func testEntity() def.EntityDefinition {
	return &def.SystemDefinition{
		Name:        "doc_widget",
		Module:      "doc",
		Label:       "Widget",
		LabelPlural: "Widgets",
		Description: "A test widget for documentation.",
		Fields: []def.FieldDef{
			{Name: "name", Type: def.FieldTypeData, Required: true, Label: "Name"},
			{Name: "weight", Type: def.FieldTypeCurrency, Label: "Weight"},
			{Name: "kind", Type: def.FieldTypeSelect, Options: []string{"small", "large"}, Label: "Kind"},
		},
		Actions: []def.ActionDef{
			{
				Name:        "activate",
				Label:       "Activate",
				Description: "Activate the widget.",
				HandlerFunc: func(_ *def.ActionContext) (*def.ActionResult, error) {
					return &def.ActionResult{Message: "activated"}, nil
				},
			},
		},
		Permissions: def.PermissionSet{
			Create: []string{"doc.widget.create"},
			Read:   []string{"doc.widget.read"},
		},
	}
}

func TestGenerate_ProducesOneFilePerEntity(t *testing.T) {
	schema := buildSchema(t, []def.EntityDefinition{testEntity()})
	plan, err := Generate(schema)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(plan.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(plan.Files))
	}
}

func TestGenerate_FileNameIsQualifiedEntityName(t *testing.T) {
	schema := buildSchema(t, []def.EntityDefinition{testEntity()})
	plan, err := Generate(schema)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if plan.Files[0].Name != "doc_widget" {
		t.Errorf("expected file name 'doc_widget', got %q", plan.Files[0].Name)
	}
}

func TestGenerate_ContentContainsTitle(t *testing.T) {
	schema := buildSchema(t, []def.EntityDefinition{testEntity()})
	plan, err := Generate(schema)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	content := plan.Files[0].Content
	if !strings.Contains(content, "Widget") {
		t.Errorf("expected entity label 'Widget' in content")
	}
	if !strings.Contains(content, "doc_widget") {
		t.Errorf("expected qualified name 'doc_widget' in content")
	}
}

func TestGenerate_ContentContainsFieldNames(t *testing.T) {
	schema := buildSchema(t, []def.EntityDefinition{testEntity()})
	plan, err := Generate(schema)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	content := plan.Files[0].Content
	for _, field := range []string{"name", "weight", "kind"} {
		if !strings.Contains(content, field) {
			t.Errorf("expected field %q in doc content", field)
		}
	}
}

func TestGenerate_ContentContainsActions(t *testing.T) {
	schema := buildSchema(t, []def.EntityDefinition{testEntity()})
	plan, err := Generate(schema)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	content := plan.Files[0].Content
	if !strings.Contains(content, "activate") {
		t.Errorf("expected action 'activate' in doc content")
	}
}

func TestGenerate_ContentContainsPermissions(t *testing.T) {
	schema := buildSchema(t, []def.EntityDefinition{testEntity()})
	plan, err := Generate(schema)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	content := plan.Files[0].Content
	if !strings.Contains(content, "doc.widget.read") {
		t.Errorf("expected permission identifier in doc content")
	}
}

func TestGenerate_MultipleEntities_AllDocumented(t *testing.T) {
	e2 := &def.SystemDefinition{
		Name: "doc_gadget", Module: "doc", Label: "Gadget", LabelPlural: "Gadgets",
		Fields: []def.FieldDef{
			{Name: "code", Type: def.FieldTypeData, Required: true},
		},
		Permissions: def.PermissionSet{Read: []string{"doc.gadget.read"}},
	}
	schema := buildSchema(t, []def.EntityDefinition{testEntity(), e2})
	plan, err := Generate(schema)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(plan.Files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(plan.Files))
	}
	names := map[string]bool{}
	for _, f := range plan.Files {
		names[f.Name] = true
	}
	if !names["doc_widget"] || !names["doc_gadget"] {
		t.Errorf("missing expected file names, got: %v", names)
	}
}

func TestGenerate_NoActions_StillGenerates(t *testing.T) {
	// Entity with no actions — docgen should still produce output.
	d := &def.SystemDefinition{
		Name: "doc_bare", Module: "doc", Label: "Bare", LabelPlural: "Bares",
		Fields:      []def.FieldDef{{Name: "code", Type: def.FieldTypeData, Required: true}},
		Permissions: def.PermissionSet{Read: []string{"doc.bare.read"}},
	}
	schema := buildSchema(t, []def.EntityDefinition{d})
	plan, err := Generate(schema)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(plan.Files) != 1 {
		t.Errorf("expected 1 file, got %d", len(plan.Files))
	}
	if !strings.Contains(plan.Files[0].Content, "code") {
		t.Errorf("expected field 'code' in doc content")
	}
}

func TestGenerate_ContentIsMarkdown(t *testing.T) {
	schema := buildSchema(t, []def.EntityDefinition{testEntity()})
	plan, err := Generate(schema)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	content := plan.Files[0].Content
	// Markdown heading starts with #.
	if !strings.HasPrefix(content, "#") {
		t.Errorf("expected Markdown content starting with '#', got: %q", content[:min(40, len(content))])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
