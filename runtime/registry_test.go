package runtime

import (
	"testing"

	"awo.so/awo/compiler"
	"awo.so/awo/def"
	"awo.so/awo/registry"
)

// buildRuntimeRegistry builds a RuntimeRegistry from a minimal entity definition.
func buildRuntimeRegistry(t *testing.T, defs ...def.EntityDefinition) *RuntimeRegistry {
	t.Helper()
	reg, err := registry.BuildFrom(defs)
	if err != nil {
		t.Fatalf("BuildFrom: %v", err)
	}
	schema, err := compiler.Compile(reg)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	return NewRuntimeRegistry(schema)
}

// minimalDef returns a minimal entity definition for runtime tests.
func minimalDef() def.EntityDefinition {
	return &def.SystemDefinition{
		Name:        "widget",
		Module:      "test",
		Label:       "Widget",
		LabelPlural: "Widgets",
		Fields: []def.FieldDef{
			{Name: "name", Type: def.FieldTypeData, Required: true},
		},
		Permissions: def.PermissionSet{
			Create: []string{"test.widget.create"},
			Read:   []string{"test.widget.read"},
		},
	}
}

func TestNewRuntimeRegistry_FindEntity(t *testing.T) {
	r := buildRuntimeRegistry(t, minimalDef())

	es, err := r.FindEntity("test_widget")
	if err != nil {
		t.Fatalf("FindEntity: %v", err)
	}
	if es.QualifiedName != "test_widget" {
		t.Errorf("expected test_widget, got %s", es.QualifiedName)
	}
}

func TestNewRuntimeRegistry_FindEntity_NotFound(t *testing.T) {
	r := buildRuntimeRegistry(t, minimalDef())

	_, err := r.FindEntity("nonexistent_entity")
	if err == nil {
		t.Error("expected error for missing entity, got nil")
	}
}

func TestNewRuntimeRegistry_FindTable(t *testing.T) {
	r := buildRuntimeRegistry(t, minimalDef())

	es, err := r.FindTable("test_widget")
	if err != nil {
		t.Fatalf("FindTable: %v", err)
	}
	if es.QualifiedName != "test_widget" {
		t.Errorf("unexpected entity: %s", es.QualifiedName)
	}
}

func TestNewRuntimeRegistry_FindTable_NotFound(t *testing.T) {
	r := buildRuntimeRegistry(t, minimalDef())

	_, err := r.FindTable("nonexistent_table")
	if err == nil {
		t.Error("expected error for missing table, got nil")
	}
}

func TestNewRuntimeRegistry_FindPermission(t *testing.T) {
	r := buildRuntimeRegistry(t, minimalDef())

	es, err := r.FindPermission("test_widget")
	if err != nil {
		t.Fatalf("FindPermission: %v", err)
	}
	if es.QualifiedName != "test_widget" {
		t.Errorf("unexpected entity: %s", es.QualifiedName)
	}
}

func TestNewRuntimeRegistry_FindPermission_NotFound(t *testing.T) {
	r := buildRuntimeRegistry(t, minimalDef())

	_, err := r.FindPermission("nonexistent_permission")
	if err == nil {
		t.Error("expected error for missing permission namespace, got nil")
	}
}

func TestNewRuntimeRegistry_FindWorkflow(t *testing.T) {
	r := buildRuntimeRegistry(t, minimalDef())

	es, err := r.FindWorkflow("test.widget")
	if err != nil {
		t.Fatalf("FindWorkflow: %v", err)
	}
	if es.QualifiedName != "test_widget" {
		t.Errorf("unexpected entity: %s", es.QualifiedName)
	}
}

func TestNewRuntimeRegistry_FindWorkflow_NotFound(t *testing.T) {
	r := buildRuntimeRegistry(t, minimalDef())

	_, err := r.FindWorkflow("nonexistent.namespace")
	if err == nil {
		t.Error("expected error for missing workflow namespace, got nil")
	}
}

func TestNewRuntimeRegistry_FindEvent(t *testing.T) {
	r := buildRuntimeRegistry(t, minimalDef())

	es, err := r.FindEvent("test.widget")
	if err != nil {
		t.Fatalf("FindEvent: %v", err)
	}
	if es.QualifiedName != "test_widget" {
		t.Errorf("unexpected entity: %s", es.QualifiedName)
	}
}

func TestNewRuntimeRegistry_FindEvent_NotFound(t *testing.T) {
	r := buildRuntimeRegistry(t, minimalDef())

	_, err := r.FindEvent("nonexistent.event")
	if err == nil {
		t.Error("expected error for missing event namespace, got nil")
	}
}

func TestNewRuntimeRegistry_FindCache(t *testing.T) {
	r := buildRuntimeRegistry(t, minimalDef())

	es, err := r.FindCache("test:widget")
	if err != nil {
		t.Fatalf("FindCache: %v", err)
	}
	if es.QualifiedName != "test_widget" {
		t.Errorf("unexpected entity: %s", es.QualifiedName)
	}
}

func TestNewRuntimeRegistry_FindCache_NotFound(t *testing.T) {
	r := buildRuntimeRegistry(t, minimalDef())

	_, err := r.FindCache("nonexistent:cache")
	if err == nil {
		t.Error("expected error for missing cache namespace, got nil")
	}
}

func TestNewRuntimeRegistry_FindMetric(t *testing.T) {
	r := buildRuntimeRegistry(t, minimalDef())

	es, err := r.FindMetric("test_widget")
	if err != nil {
		t.Fatalf("FindMetric: %v", err)
	}
	if es.QualifiedName != "test_widget" {
		t.Errorf("unexpected entity: %s", es.QualifiedName)
	}
}

func TestNewRuntimeRegistry_FindMetric_NotFound(t *testing.T) {
	r := buildRuntimeRegistry(t, minimalDef())

	_, err := r.FindMetric("nonexistent_metric")
	if err == nil {
		t.Error("expected error for missing metric namespace, got nil")
	}
}

func TestNewRuntimeRegistry_FindRoute(t *testing.T) {
	r := buildRuntimeRegistry(t, minimalDef())

	// Find any route from the compiled schema.
	if len(r.Routes()) == 0 {
		t.Fatal("no routes compiled")
	}
	firstRoute := r.Routes()[0]
	rd, err := r.FindRoute(firstRoute.Path)
	if err != nil {
		t.Fatalf("FindRoute(%q): %v", firstRoute.Path, err)
	}
	if rd.Path != firstRoute.Path {
		t.Errorf("expected path %q, got %q", firstRoute.Path, rd.Path)
	}
}

func TestNewRuntimeRegistry_FindRoute_NotFound(t *testing.T) {
	r := buildRuntimeRegistry(t, minimalDef())

	_, err := r.FindRoute("/api/v1/nonexistent/path")
	if err == nil {
		t.Error("expected error for missing route, got nil")
	}
}

func TestNewRuntimeRegistry_Entities(t *testing.T) {
	r := buildRuntimeRegistry(t, minimalDef())

	entities := r.Entities()
	if len(entities) == 0 {
		t.Error("expected at least one entity")
	}
	found := false
	for _, es := range entities {
		if es.QualifiedName == "test_widget" {
			found = true
		}
	}
	if !found {
		t.Error("test_widget not in Entities() slice")
	}
}

func TestNewRuntimeRegistry_Routes(t *testing.T) {
	r := buildRuntimeRegistry(t, minimalDef())

	routes := r.Routes()
	if len(routes) < 5 {
		t.Errorf("expected ≥5 routes for one entity, got %d", len(routes))
	}
}

func TestNewRuntimeRegistry_Schema(t *testing.T) {
	r := buildRuntimeRegistry(t, minimalDef())

	schema := r.Schema()
	if schema == nil {
		t.Error("Schema() returned nil")
	}
	if len(schema.Entities) == 0 {
		t.Error("Schema().Entities is empty")
	}
}

func TestNewRuntimeRegistry_MultiEntity(t *testing.T) {
	a := &def.SystemDefinition{
		Name: "alpha", Module: "test", Label: "Alpha", LabelPlural: "Alphas",
	}
	b := &def.SystemDefinition{
		Name: "beta", Module: "test", Label: "Beta", LabelPlural: "Betas",
	}
	r := buildRuntimeRegistry(t, a, b)

	if _, err := r.FindEntity("test_alpha"); err != nil {
		t.Errorf("FindEntity(test_alpha): %v", err)
	}
	if _, err := r.FindEntity("test_beta"); err != nil {
		t.Errorf("FindEntity(test_beta): %v", err)
	}
	if len(r.Entities()) != 2 {
		t.Errorf("expected 2 entities, got %d", len(r.Entities()))
	}
}
