package generator_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"awo.so/awo/sdui/generator"
	"awo.so/awo/sdui/sduictx"
	"awo.so/awo/sdui/widget"
)

// ── Test helpers ──────────────────────────────────────────────────────────────

type stubViewer struct {
	perms map[string]bool
	admin bool
}

func (s *stubViewer) TenantID() uuid.UUID   { return uuid.New() }
func (s *stubViewer) Roles() []string       { return nil }
func (s *stubViewer) IsPlatformAdmin() bool { return s.admin }
func (s *stubViewer) HasPermission(p string) bool {
	if s.admin {
		return true
	}
	return s.perms[p]
}

func makeCtx(mode sduictx.ViewMode, viewer sduictx.ViewerContext) sduictx.GeneratorContext {
	ctx, _ := sduictx.NewGeneratorContext(
		uuid.New(), viewer, "finance_invoice", mode, "amis",
	).WithSchemaFingerprint("fp1").WithPermFingerprint("pf1").Build()
	return ctx
}

func allPermsViewer() *stubViewer {
	return &stubViewer{admin: true}
}

func noPermsViewer() *stubViewer {
	return &stubViewer{perms: map[string]bool{}}
}

func makeSchema() generator.EntitySchema {
	return generator.EntitySchema{
		Name:        "finance_invoice",
		Title:       "Invoice",
		PluralTitle: "Invoices",
		ListURL:     "/api/v1/finance/invoices",
		CreateURL:   "/api/v1/finance/invoices",
		EditURL:     "/api/v1/finance/invoices/${id}",
		DetailURL:   "/api/v1/finance/invoices/${id}",
		Permissions: map[string]string{
			"create": "finance.invoice.create",
			"read":   "finance.invoice.read",
			"update": "finance.invoice.update",
			"delete": "finance.invoice.delete",
		},
		Fields: []generator.FieldDef{
			{Name: "number", Label: "Number", FieldType: "data", InList: true, InForm: true, InDetail: true},
			{Name: "status", Label: "Status", FieldType: "select", InList: true, InForm: true, InDetail: true},
			{Name: "amount", Label: "Amount", FieldType: "currency", InList: true, InForm: true, InDetail: true},
			{Name: "date", Label: "Date", FieldType: "date", InList: true, InForm: true, InDetail: true},
			{Name: "notes", Label: "Notes", FieldType: "long_text", InForm: true, InDetail: true},
			{Name: "secret", Label: "Secret", FieldType: "data", Permission: "finance.invoice.admin",
				InList: true, InForm: true, InDetail: true},
		},
	}
}

// ── Tests ─────────────────────────────────────────────────────────────────────

func TestGenerator_ListView(t *testing.T) {
	g := generator.New()
	ctx := makeCtx(sduictx.ViewModeList, allPermsViewer())
	schema := makeSchema()

	root, err := g.Generate(schema, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if root == nil {
		t.Fatal("expected non-nil root")
	}
	if root.Kind != widget.NodePage {
		t.Errorf("expected NodePage root, got %q", root.Kind)
	}
	if len(root.Children) == 0 {
		t.Fatal("expected children on page")
	}
	listNode := root.Children[0]
	if listNode.Kind != widget.NodeList {
		t.Errorf("expected NodeList child, got %q", listNode.Kind)
	}
	// Should have 5 columns: number, status, amount, date, secret.
	// notes has InList=false (textarea fields don't appear in list view).
	if len(listNode.Children) != 5 {
		t.Errorf("expected 5 columns, got %d", len(listNode.Children))
	}
}

func TestGenerator_ListView_PermissionGating(t *testing.T) {
	g := generator.New()
	// Viewer has no permissions — secret field should be absent.
	ctx := makeCtx(sduictx.ViewModeList, noPermsViewer())
	root, err := g.Generate(makeSchema(), ctx)
	if err != nil {
		t.Fatal(err)
	}
	listNode := root.Children[0]
	// secret field is absent — 4 visible columns (number, status, amount, date).
	if len(listNode.Children) != 4 {
		t.Errorf("expected 4 columns (secret absent), got %d", len(listNode.Children))
	}
	// Verify secret is not in the list.
	for _, col := range listNode.Children {
		if col.Name == "secret" {
			t.Error("secret field should be absent for viewer without permission")
		}
	}
}

func TestGenerator_FormView_Create(t *testing.T) {
	g := generator.New()
	ctx := makeCtx(sduictx.ViewModeCreate, allPermsViewer())
	root, err := g.Generate(makeSchema(), ctx)
	if err != nil {
		t.Fatal(err)
	}
	if root.Kind != widget.NodePage {
		t.Errorf("expected NodePage, got %q", root.Kind)
	}
	formNode := root.Children[0]
	if formNode.Kind != widget.NodeForm {
		t.Errorf("expected NodeForm child, got %q", formNode.Kind)
	}
	if formNode.DataSource == nil {
		t.Fatal("expected DataSource on form")
	}
	if formNode.DataSource.Method != "POST" {
		t.Errorf("expected POST method for create, got %q", formNode.DataSource.Method)
	}
}

func TestGenerator_FormView_Edit(t *testing.T) {
	g := generator.New()
	ctx := makeCtx(sduictx.ViewModeEdit, allPermsViewer())
	root, err := g.Generate(makeSchema(), ctx)
	if err != nil {
		t.Fatal(err)
	}
	formNode := root.Children[0]
	if formNode.DataSource.Method != "PATCH" {
		t.Errorf("expected PATCH method for edit, got %q", formNode.DataSource.Method)
	}
	// Edit form should have initApi (ReadURL set).
	if formNode.DataSource.ReadURL == "" {
		t.Error("expected ReadURL set for edit form (initApi)")
	}
}

func TestGenerator_DetailView(t *testing.T) {
	g := generator.New()
	ctx := makeCtx(sduictx.ViewModeDetail, allPermsViewer())
	root, err := g.Generate(makeSchema(), ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Detail page should have NodeSummaryCard + NodeForm.
	if len(root.Children) < 2 {
		t.Fatalf("expected at least 2 children on detail page, got %d", len(root.Children))
	}
	summaryCard := root.Children[0]
	if summaryCard.Kind != widget.NodeSummaryCard {
		t.Errorf("expected NodeSummaryCard first, got %q", summaryCard.Kind)
	}
}

func TestGenerator_DetailView_ReadOnly(t *testing.T) {
	g := generator.New()
	ctx := makeCtx(sduictx.ViewModeDetail, allPermsViewer())
	root, err := g.Generate(makeSchema(), ctx)
	if err != nil {
		t.Fatal(err)
	}
	// All field nodes on detail form must be ReadOnly=true.
	formNode := root.Children[1] // [0]=summaryCard, [1]=form
	widget.WalkAll(formNode, func(n *widget.Node) {
		if n.Kind == widget.NodeText || n.Kind == widget.NodeNumber || n.Kind == widget.NodeSelect {
			if !n.ReadOnly {
				t.Errorf("field %q on detail view must be ReadOnly", n.Name)
			}
		}
	})
}

func TestGenerator_UnknownViewMode(t *testing.T) {
	g := generator.New()
	ctx, _ := sduictx.NewGeneratorContext(
		uuid.New(), allPermsViewer(), "test", sduictx.ViewMode("invalid_mode"), "amis",
	).Build()
	_, err := g.Generate(makeSchema(), ctx)
	if err == nil {
		t.Error("expected error for unknown ViewMode")
	}
}

func TestGenerator_ListActions_CreateButton(t *testing.T) {
	g := generator.New()
	// Viewer has create permission.
	viewer := &stubViewer{perms: map[string]bool{"finance.invoice.create": true}}
	ctx := makeCtx(sduictx.ViewModeList, viewer)
	root, err := g.Generate(makeSchema(), ctx)
	if err != nil {
		t.Fatal(err)
	}
	listNode := root.Children[0]
	if len(listNode.Actions) == 0 {
		t.Fatal("expected at least one action (Create button)")
	}
	createBtn := listNode.Actions[0]
	if createBtn.ID != "create" {
		t.Errorf("expected create action, got %q", createBtn.ID)
	}
}

func TestGenerator_DetailActions_NoDeleteWithoutPerm(t *testing.T) {
	g := generator.New()
	// Viewer has update but not delete permission.
	viewer := &stubViewer{perms: map[string]bool{"finance.invoice.update": true}}
	ctx := makeCtx(sduictx.ViewModeDetail, viewer)
	root, err := g.Generate(makeSchema(), ctx)
	if err != nil {
		t.Fatal(err)
	}
	summaryCard := root.Children[0]
	for _, action := range summaryCard.Actions {
		if action.ID == "delete" {
			t.Error("delete action should be absent for viewer without delete permission")
		}
	}
}

// ── Session 6 regression tests ────────────────────────────────────────────────

// TestGenerator_ListActions_RowScopedActions guards that buildListActions
// produces Scope="row" entries for View/Edit/Delete when permissions are granted.
func TestGenerator_ListActions_RowScopedActions(t *testing.T) {
	g := generator.New()
	viewer := &stubViewer{perms: map[string]bool{
		"finance.invoice.create": true,
		"finance.invoice.update": true,
		"finance.invoice.delete": true,
	}}
	schema := makeSchemaWithUIPrefix()
	ctx := makeCtx(sduictx.ViewModeList, viewer)
	root, err := g.Generate(schema, ctx)
	if err != nil {
		t.Fatal(err)
	}
	listNode := root.Children[0]

	scopeCounts := map[string]int{}
	for _, a := range listNode.Actions {
		scopeCounts[a.Scope]++
	}
	if scopeCounts["row"] < 3 {
		t.Errorf("expected at least 3 row-scoped actions (view/edit/delete), got %d", scopeCounts["row"])
	}
	if scopeCounts["bulk"] < 1 {
		t.Errorf("expected at least 1 bulk-scoped action (bulk delete), got %d", scopeCounts["bulk"])
	}
	if scopeCounts["toolbar"] < 1 {
		t.Errorf("expected at least 1 toolbar-scoped action (create), got %d", scopeCounts["toolbar"])
	}
}

// TestGenerator_ListActions_NoRowDeleteWithoutPerm guards that the Delete row
// action is absent when the viewer lacks delete permission.
func TestGenerator_ListActions_NoRowDeleteWithoutPerm(t *testing.T) {
	g := generator.New()
	// Viewer has read and update but NOT delete permission.
	viewer := &stubViewer{perms: map[string]bool{
		"finance.invoice.create": true,
		"finance.invoice.update": true,
	}}
	schema := makeSchemaWithUIPrefix()
	ctx := makeCtx(sduictx.ViewModeList, viewer)
	root, err := g.Generate(schema, ctx)
	if err != nil {
		t.Fatal(err)
	}
	listNode := root.Children[0]
	for _, a := range listNode.Actions {
		if a.ID == "delete" || a.ID == "bulk-delete" {
			t.Errorf("action %q should be absent for viewer without delete permission", a.ID)
		}
	}
}

// TestGenerator_ListActions_NoRowEditWithoutPerm guards that the Edit row action
// is absent when the viewer lacks update permission.
func TestGenerator_ListActions_NoRowEditWithoutPerm(t *testing.T) {
	g := generator.New()
	// Viewer has read and delete but NOT update permission.
	viewer := &stubViewer{perms: map[string]bool{
		"finance.invoice.create": true,
		"finance.invoice.delete": true,
	}}
	schema := makeSchemaWithUIPrefix()
	ctx := makeCtx(sduictx.ViewModeList, viewer)
	root, err := g.Generate(schema, ctx)
	if err != nil {
		t.Fatal(err)
	}
	listNode := root.Children[0]
	for _, a := range listNode.Actions {
		if a.ID == "edit" && a.Scope == "row" {
			t.Error("row edit action should be absent for viewer without update permission")
		}
	}
}

// TestGenerator_DetailActions_DeleteHasAPI guards that the delete action in
// detail view has a non-empty API URL. This prevents the AMIS ajax action
// from firing with no endpoint (silent no-op / console error).
func TestGenerator_DetailActions_DeleteHasAPI(t *testing.T) {
	g := generator.New()
	viewer := &stubViewer{perms: map[string]bool{
		"finance.invoice.delete": true,
	}}
	ctx := makeCtx(sduictx.ViewModeDetail, viewer)
	root, err := g.Generate(makeSchema(), ctx)
	if err != nil {
		t.Fatal(err)
	}
	summaryCard := root.Children[0]
	for _, action := range summaryCard.Actions {
		if action.ID == "delete" {
			if action.API == "" {
				t.Error("delete action in detail view must have non-empty API URL")
			}
			// API must use ${id} so AMIS resolves the record ID from data context.
			if !strings.Contains(action.API, "${id}") {
				t.Errorf("delete action API %q must contain ${id} for AMIS template resolution", action.API)
			}
			return
		}
	}
	// If we get here, delete action was not found — that's only OK if delete permission is absent.
	// Since we gave delete permission, it should be present.
	t.Error("expected delete action in detail view for viewer with delete permission")
}

// TestGenerator_FilterBar_FieldNames guards that filter bar field names follow
// the filter[field][op]=value convention expected by filterparse.FromQuery.
func TestGenerator_FilterBar_FieldNames(t *testing.T) {
	g := generator.New()
	ctx := makeCtx(sduictx.ViewModeList, allPermsViewer())
	schema := generator.EntitySchema{
		Name:        "test",
		Title:       "Test",
		ListURL:     "/api/v1/test",
		Permissions: map[string]string{},
		Fields: []generator.FieldDef{
			{Name: "name", Label: "Name", FieldType: "data", InList: true, InForm: true, Searchable: true},
			{Name: "status", Label: "Status", FieldType: "select", InList: true, InForm: true, Searchable: true},
			{Name: "issued_at", Label: "Issue Date", FieldType: "date", InList: true, InForm: true, Searchable: true},
		},
	}
	root, err := g.Generate(schema, ctx)
	if err != nil {
		t.Fatal(err)
	}
	listNode := root.Children[0]
	if listNode.FilterBar == nil {
		t.Fatal("expected filter bar to be present for searchable fields")
	}
	fieldNames := map[string]bool{}
	for _, child := range listNode.FilterBar.Children {
		fieldNames[child.Name] = true
	}
	// Text field → contains operator.
	if !fieldNames["filter[name][contains]"] {
		t.Errorf("text field filter name should be 'filter[name][contains]', got names: %v", fieldNames)
	}
	// Select field → eq operator.
	if !fieldNames["filter[status][eq]"] {
		t.Errorf("select field filter name should be 'filter[status][eq]', got names: %v", fieldNames)
	}
	// Date field → date range via Props (node.Name is a placeholder).
	// Check that the date node has Props with startName/endName set.
	for _, child := range listNode.FilterBar.Children {
		if child.Name == "issued_at_range" {
			// This is the date-range node — verify Props.
			if child.Props == nil {
				t.Error("date filter node must have Props set for input-date-range")
				continue
			}
			if child.Props["startName"] != "filter[issued_at][gte]" {
				t.Errorf("date filter startName=%v want filter[issued_at][gte]", child.Props["startName"])
			}
			if child.Props["endName"] != "filter[issued_at][lte]" {
				t.Errorf("date filter endName=%v want filter[issued_at][lte]", child.Props["endName"])
			}
		}
	}
}

// makeSchemaWithUIPrefix returns a test schema with UIPrefix set, required for
// row-action href generation in buildListActions.
func makeSchemaWithUIPrefix() generator.EntitySchema {
	s := makeSchema()
	s.UIPrefix = "/ui/finance/invoices"
	return s
}

func TestGenerator_FieldTypeMapping(t *testing.T) {
	g := generator.New()
	ctx := makeCtx(sduictx.ViewModeCreate, allPermsViewer())
	schema := generator.EntitySchema{
		Name:      "test",
		Title:     "Test",
		CreateURL: "/api/test",
		Fields: []generator.FieldDef{
			{Name: "f1", Label: "Text", FieldType: "data", InForm: true},
			{Name: "f2", Label: "Number", FieldType: "int", InForm: true},
			{Name: "f3", Label: "Date", FieldType: "date", InForm: true},
			{Name: "f4", Label: "Bool", FieldType: "bool", InForm: true},
			{Name: "f5", Label: "Select", FieldType: "select", InForm: true},
			{Name: "f6", Label: "Long", FieldType: "long_text", InForm: true},
		},
	}
	root, err := g.Generate(schema, ctx)
	if err != nil {
		t.Fatal(err)
	}
	formNode := root.Children[0]
	kinds := make(map[string]widget.NodeKind)
	widget.WalkAll(formNode, func(n *widget.Node) {
		if n.Name != "" {
			kinds[n.Name] = n.Kind
		}
	})
	expected := map[string]widget.NodeKind{
		"f1": widget.NodeText,
		"f2": widget.NodeNumber,
		"f3": widget.NodeDate,
		"f4": widget.NodeSwitch,
		"f5": widget.NodeSelect,
		"f6": widget.NodeTextArea,
	}
	for name, want := range expected {
		if got := kinds[name]; got != want {
			t.Errorf("field %q: kind=%q want %q", name, got, want)
		}
	}
}
