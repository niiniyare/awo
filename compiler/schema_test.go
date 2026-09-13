package compiler

import (
	"testing"

	"awo.so/awo/def"
	"awo.so/awo/registry"
)

// buildSchema compiles a set of entity definitions to a CompiledSchema.
func buildSchema(t *testing.T, defs []def.EntityDefinition) *CompiledSchema {
	t.Helper()
	reg, err := registry.BuildFrom(defs)
	if err != nil {
		t.Fatalf("registry.BuildFrom: %v", err)
	}
	schema, err := Compile(reg)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	return schema
}

// TestBuildEntitySchema_BasicFields verifies field attribute promotion.
func TestBuildEntitySchema_BasicFields(t *testing.T) {
	d := &def.SystemDefinition{
		Name:        "mod_item",
		Module:      "mod",
		Label:       "Item",
		LabelPlural: "Items",
		Fields: []def.FieldDef{
			{Name: "name", Type: def.FieldTypeData, Required: true},
			{Name: "code", Type: def.FieldTypeData, Immutable: true},
			{Name: "secret", Type: def.FieldTypeData, Sensitive: true},
			{Name: "title", Type: def.FieldTypeData, Searchable: true},
			{Name: "status", Type: def.FieldTypeData, Default: func() any { return "draft" }},
		},
	}
	schema := buildSchema(t, []def.EntityDefinition{d})
	es, ok := schema.ByName["mod_item"]
	if !ok {
		t.Fatal("mod_item not in schema")
	}

	if !es.RequiredFields["name"] {
		t.Error("name should be in RequiredFields")
	}
	if !es.ImmutableFields["code"] {
		t.Error("code should be in ImmutableFields")
	}
	if !es.SensitiveFields["secret"] {
		t.Error("secret should be in SensitiveFields")
	}
	if !es.SearchableFields["title"] {
		t.Error("title should be in SearchableFields")
	}
	if es.DefaultValues["status"] == nil {
		t.Error("status should have a DefaultValues entry")
	}
}

// TestBuildEntitySchema_Namespaces verifies all derived namespaces.
func TestBuildEntitySchema_Namespaces(t *testing.T) {
	schema := buildSchema(t, []def.EntityDefinition{simpleDef("mod_item", "mod")})
	es := schema.ByName["mod_item"]
	if es == nil {
		t.Fatal("mod_item missing")
	}

	if es.EventNamespace != "mod.item" {
		t.Errorf("EventNamespace: got %q, want %q", es.EventNamespace, "mod.item")
	}
	if es.WorkflowNamespace != "mod.item" {
		t.Errorf("WorkflowNamespace: got %q, want %q", es.WorkflowNamespace, "mod.item")
	}
	if es.PermissionNamespace != "mod_item" {
		t.Errorf("PermissionNamespace: got %q, want %q", es.PermissionNamespace, "mod_item")
	}
	if es.MetricNamespace != "mod_item" {
		t.Errorf("MetricNamespace: got %q, want %q", es.MetricNamespace, "mod_item")
	}
	if es.CacheNamespace != "mod:item" {
		t.Errorf("CacheNamespace: got %q, want %q", es.CacheNamespace, "mod:item")
	}
	if es.TableName != "mod_item" {
		t.Errorf("TableName: got %q, want %q", es.TableName, "mod_item")
	}
	if es.RoutePrefix != "/api/v1/mod/items" {
		t.Errorf("RoutePrefix: got %q, want %q", es.RoutePrefix, "/api/v1/mod/items")
	}
}

// TestEmitRoutes_StandardCRUD verifies 5 base routes are emitted.
func TestEmitRoutes_StandardCRUD(t *testing.T) {
	schema := buildSchema(t, []def.EntityDefinition{simpleDef("mod_item", "mod")})
	es := schema.ByName["mod_item"]

	routes := emitRoutes(es)
	if len(routes) != 5 {
		t.Errorf("expected 5 base routes, got %d", len(routes))
	}

	ops := map[string]bool{}
	for _, r := range routes {
		ops[r.Operation] = true
	}
	for _, op := range []string{"list", "get", "create", "update", "delete"} {
		if !ops[op] {
			t.Errorf("missing operation %q in routes", op)
		}
	}
}

// TestEmitRoutes_Action_DefaultsToPost verifies action route defaults.
func TestEmitRoutes_Action_DefaultsToPost(t *testing.T) {
	d := &def.SystemDefinition{
		Name: "mod_item", Module: "mod", Label: "Item", LabelPlural: "Items",
		Actions: []def.ActionDef{
			{Name: "submit", HandlerFunc: func(*def.ActionContext) (*def.ActionResult, error) { return nil, nil }},
		},
	}
	schema := buildSchema(t, []def.EntityDefinition{d})
	es := schema.ByName["mod_item"]

	routes := emitRoutes(es)
	// 5 CRUD + 1 action
	if len(routes) != 6 {
		t.Errorf("expected 6 routes (5 CRUD + 1 action), got %d", len(routes))
	}

	var actionRoute *RouteDescriptor
	for i := range routes {
		if routes[i].Operation == "action" {
			actionRoute = &routes[i]
		}
	}
	if actionRoute == nil {
		t.Fatal("action route not emitted")
	}
	if actionRoute.Method != "POST" {
		t.Errorf("default action method should be POST, got %q", actionRoute.Method)
	}
	if actionRoute.RequiredPermission != "write" {
		t.Errorf("default action permission should be 'write', got %q", actionRoute.RequiredPermission)
	}
	if actionRoute.ActionName != "submit" {
		t.Errorf("action name should be 'submit', got %q", actionRoute.ActionName)
	}
}

// TestEmitRoutes_Action_ExplicitMethod verifies explicit method and permission.
func TestEmitRoutes_Action_ExplicitMethod(t *testing.T) {
	d := &def.SystemDefinition{
		Name: "mod_item", Module: "mod", Label: "Item", LabelPlural: "Items",
		Actions: []def.ActionDef{
			{Name: "cancel", Method: def.ActionMethodDelete, Permission: "mod.item.cancel",
				HandlerFunc: func(*def.ActionContext) (*def.ActionResult, error) { return nil, nil }},
		},
	}
	schema := buildSchema(t, []def.EntityDefinition{d})
	es := schema.ByName["mod_item"]

	routes := emitRoutes(es)
	var actionRoute *RouteDescriptor
	for i := range routes {
		if routes[i].Operation == "action" {
			actionRoute = &routes[i]
		}
	}
	if actionRoute == nil {
		t.Fatal("action route not found")
	}
	if actionRoute.Method != "DELETE" {
		t.Errorf("expected DELETE, got %q", actionRoute.Method)
	}
	if actionRoute.RequiredPermission != "mod.item.cancel" {
		t.Errorf("expected explicit permission, got %q", actionRoute.RequiredPermission)
	}
}

// TestBuildLookup_LabelFieldPriority verifies label field heuristic order.
func TestBuildLookup_LabelFieldPriority(t *testing.T) {
	// Target has both "code" and "title"; "name" takes priority per priority list.
	target := &EntitySchema{
		QualifiedName: "mod_currency",
		Label:         "Currency",
		RoutePrefix:   "/api/v1/mod/currencies",
		FieldsByName: map[string]def.FieldDef{
			"code":  {Name: "code", Type: def.FieldTypeData},
			"title": {Name: "title", Type: def.FieldTypeData},
		},
		SearchableFields: map[string]bool{},
	}
	lookup := buildLookup(target, false)
	if lookup.LabelField != "code" {
		t.Errorf("expected LabelField=code (before title), got %q", lookup.LabelField)
	}
}

// TestBuildLookup_FallbackToSearchable verifies Searchable field fallback.
func TestBuildLookup_FallbackToSearchable(t *testing.T) {
	target := &EntitySchema{
		QualifiedName: "mod_tag",
		Label:         "Tag",
		RoutePrefix:   "/api/v1/mod/tags",
		FieldsByName: map[string]def.FieldDef{
			"slug": {Name: "slug", Type: def.FieldTypeData, Searchable: true},
		},
		SearchableFields: map[string]bool{"slug": true},
	}
	lookup := buildLookup(target, false)
	// None of name/code/title/label/full_name/account_name → falls back to Searchable.
	if lookup.LabelField != "slug" {
		t.Errorf("expected LabelField=slug (searchable fallback), got %q", lookup.LabelField)
	}
}

// TestBuildLookup_FallbackToID verifies id fallback when no candidates.
func TestBuildLookup_FallbackToID(t *testing.T) {
	target := &EntitySchema{
		QualifiedName: "mod_tag",
		Label:         "Tag",
		RoutePrefix:   "/api/v1/mod/tags",
		FieldsByName: map[string]def.FieldDef{
			"value": {Name: "value", Type: def.FieldTypeData},
		},
		SearchableFields: map[string]bool{},
	}
	lookup := buildLookup(target, false)
	if lookup.LabelField != "id" {
		t.Errorf("expected LabelField=id fallback, got %q", lookup.LabelField)
	}
}

// TestBuildLookup_Multiple verifies Multiple flag is propagated.
func TestBuildLookup_Multiple(t *testing.T) {
	target := &EntitySchema{
		QualifiedName:    "mod_tag",
		Label:            "Tag",
		RoutePrefix:      "/api/v1/mod/tags",
		FieldsByName:     map[string]def.FieldDef{},
		SearchableFields: map[string]bool{},
	}
	single := buildLookup(target, false)
	multi := buildLookup(target, true)
	if single.Multiple {
		t.Error("Multiple should be false for single lookup")
	}
	if !multi.Multiple {
		t.Error("Multiple should be true for list lookup")
	}
}

// TestEmitCapabilityGrants_AllSlots verifies grants from all PermissionSet slots.
func TestEmitCapabilityGrants_AllSlots(t *testing.T) {
	es := &EntitySchema{
		QualifiedName: "mod_item",
		Permissions: def.PermissionSet{
			Create: []string{"mod.item.create"},
			Read:   []string{"mod.item.read"},
			Write:  []string{"mod.item.write"},
			Delete: []string{"mod.item.delete"},
			Actions: map[string][]string{
				"submit": {"mod.item.submit"},
				"cancel": {"mod.item.cancel"},
			},
		},
	}
	grants := emitCapabilityGrants(es)
	// 4 CRUD + 2 action = 6
	if len(grants) != 6 {
		t.Errorf("expected 6 grants, got %d", len(grants))
	}
	found := map[string]bool{}
	for _, g := range grants {
		found[g.Action] = true
		if g.Entity != "mod_item" {
			t.Errorf("grant.Entity should be mod_item, got %q", g.Entity)
		}
	}
	for _, action := range []string{"create", "read", "write", "delete", "submit", "cancel"} {
		if !found[action] {
			t.Errorf("missing grant for action %q", action)
		}
	}
}

// TestEmitCapabilityGrants_Empty verifies no grants for empty PermissionSet.
func TestEmitCapabilityGrants_Empty(t *testing.T) {
	es := &EntitySchema{QualifiedName: "mod_x", Permissions: def.PermissionSet{}}
	grants := emitCapabilityGrants(es)
	if len(grants) != 0 {
		t.Errorf("expected 0 grants for empty PermissionSet, got %d", len(grants))
	}
}

// TestCompile_LinkTarget_Resolved verifies FieldTypeLink target is resolved.
func TestCompile_LinkTarget_Resolved(t *testing.T) {
	currency := simpleDef("mod_currency", "mod")
	invoice := &def.SystemDefinition{
		Name: "mod_invoice", Module: "mod", Label: "Invoice", LabelPlural: "Invoices",
		Fields: []def.FieldDef{
			{Name: "currency_id", Type: def.FieldTypeLink, LinkTarget: "mod_currency"},
		},
	}
	schema := buildSchema(t, []def.EntityDefinition{currency, invoice})
	es := schema.ByName["mod_invoice"]
	target, ok := es.LinkTargets["currency_id"]
	if !ok {
		t.Fatal("currency_id LinkTarget not resolved")
	}
	if target.QualifiedName != "mod_currency" {
		t.Errorf("expected mod_currency, got %q", target.QualifiedName)
	}
}

// TestCompile_EdgeTarget_Resolved verifies EdgeDef targets are resolved.
func TestCompile_EdgeTarget_Resolved(t *testing.T) {
	line := simpleDef("mod_line", "mod")
	header := &def.SystemDefinition{
		Name: "mod_header", Module: "mod", Label: "Header", LabelPlural: "Headers",
		Edges: []def.EdgeDef{
			{Name: "lines", Type: def.EdgeOneToMany, Target: "mod_line"},
		},
	}
	schema := buildSchema(t, []def.EntityDefinition{line, header})
	es := schema.ByName["mod_header"]
	target, ok := es.EdgeTargets["lines"]
	if !ok {
		t.Fatal("lines EdgeTarget not resolved")
	}
	if target.QualifiedName != "mod_line" {
		t.Errorf("expected mod_line, got %q", target.QualifiedName)
	}
}

// TestCompile_CapabilityGrants_InSchema verifies grants are in the compiled schema.
func TestCompile_CapabilityGrants_InSchema(t *testing.T) {
	d := &def.SystemDefinition{
		Name: "mod_item", Module: "mod", Label: "Item", LabelPlural: "Items",
		Permissions: def.PermissionSet{
			Create: []string{"mod.item.create"},
			Read:   []string{"mod.item.read"},
		},
	}
	schema := buildSchema(t, []def.EntityDefinition{d})
	if len(schema.CapabilityGrants) == 0 {
		t.Error("expected CapabilityGrants in compiled schema")
	}
}

// TestCompile_CustomEntity_TableName verifies custom entity uses shared table.
func TestCompile_CustomEntity_TableName(t *testing.T) {
	d := &def.CustomDefinition{
		Name: "custom_widget", Module: "custom", Label: "Widget", LabelPlural: "Widgets",
	}
	schema := buildSchema(t, []def.EntityDefinition{d})
	es := schema.ByName["custom_widget"]
	if es == nil {
		t.Fatal("custom_widget not in schema")
	}
	if es.TableName != "custom_entity_records" {
		t.Errorf("custom entity should use custom_entity_records table, got %q", es.TableName)
	}
}
