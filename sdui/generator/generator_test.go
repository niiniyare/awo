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

// TestGenerator_ListView_MobileColumnPriority guards the mobile column
// strategy (Part 4 of the "shared SDUI record-management" mission): the
// first InList column (the row's primary identifier, by the same
// declaration-order convention EntityDefinition authors already establish —
// no new metadata) must stay always visible, and every other column must
// carry Props["breakpoint"]="xs" so sdui/amis's renderList can set the
// matching top-level "footable": true. Generic — makeSchema() is a generic
// finance_invoice fixture, not Expense.
func TestGenerator_ListView_MobileColumnPriority(t *testing.T) {
	g := generator.New()
	ctx := makeCtx(sduictx.ViewModeList, allPermsViewer())
	root, err := g.Generate(makeSchema(), ctx)
	if err != nil {
		t.Fatal(err)
	}
	listNode := root.Children[0]
	if len(listNode.Children) < 2 {
		t.Fatal("fixture must have at least 2 list columns for this test to mean anything")
	}
	first := listNode.Children[0]
	if first.Props != nil && first.Props["breakpoint"] != nil {
		t.Errorf("primary column %q must not carry a breakpoint, got %v", first.Name, first.Props["breakpoint"])
	}
	for _, col := range listNode.Children[1:] {
		if col.Props == nil || col.Props["breakpoint"] != "xs" {
			t.Errorf("column %q must carry Props[breakpoint]=xs, got %v", col.Name, col.Props)
		}
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

// TestGenerator_DetailView_SummaryCardHasContent guards against the
// summary card being built with no Children — it previously rendered as an
// empty header above the full read-only form (a confirmed, real "incomplete
// detail panel" defect). The summary card must reuse the same InList field
// set as the list view (buildColumnNode), not a new metadata concept.
func TestGenerator_DetailView_SummaryCardHasContent(t *testing.T) {
	g := generator.New()
	ctx := makeCtx(sduictx.ViewModeDetail, allPermsViewer())
	root, err := g.Generate(makeSchema(), ctx)
	if err != nil {
		t.Fatal(err)
	}
	summaryCard := root.Children[0]
	if len(summaryCard.Children) == 0 {
		t.Fatal("summary card must have Children — an empty summary card is the bug this test guards against")
	}
	schema := makeSchema()
	wantNames := map[string]bool{}
	for _, f := range schema.Fields {
		if f.InList && !f.Hidden {
			wantNames[f.Name] = true
		}
	}
	gotNames := map[string]bool{}
	for _, c := range summaryCard.Children {
		gotNames[c.Name] = true
	}
	for name := range wantNames {
		if !gotNames[name] {
			t.Errorf("summary card missing InList field %q", name)
		}
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

// makeLinkSchema returns a schema with one FieldTypeLink field ("assignee"),
// mirroring how sdui/adapt wires a real FieldTypeLink field's DataSource from
// a compiled FieldLookup (CompiledLookup.LabelField etc.) — see
// api/service.EntityService.resolveLinkLabels, which this generator behavior
// depends on: it always populates "<field>_label" for such a field,
// defaulting to the raw id string and upgrading it when a resolver is wired.
func makeLinkSchema() generator.EntitySchema {
	s := makeSchema()
	s.Fields = append(s.Fields, generator.FieldDef{
		Name:      "assignee",
		Label:     "Assignee",
		FieldType: "link",
		InList:    true,
		InForm:    true,
		InDetail:  true,
		DataSource: &widget.DataSource{
			URL:        "/api/v1/iam/users?q=${keywords}&tenant_id=${tenant_id}",
			Method:     "GET",
			LabelField: "full_name",
			ValueField: "id",
		},
	})
	return s
}

// TestGenerator_ListView_LinkField_BindsToResolvedLabel guards against the
// Expense "Submitted By" defect: a bare FieldTypeLink id has no display
// value of its own, so the list column must bind to the backend-resolved
// "<field>_label" sibling value (plain text), never to the raw id.
func TestGenerator_ListView_LinkField_BindsToResolvedLabel(t *testing.T) {
	g := generator.New()
	ctx := makeCtx(sduictx.ViewModeList, allPermsViewer())
	root, err := g.Generate(makeLinkSchema(), ctx)
	if err != nil {
		t.Fatal(err)
	}
	listNode := root.Children[0]
	var col *widget.Node
	for _, c := range listNode.Children {
		if c.Name == "assignee_label" {
			col = c
		}
		if c.Name == "assignee" {
			t.Errorf("list column must not bind to the raw link field %q directly", c.Name)
		}
	}
	if col == nil {
		t.Fatal(`expected a list column bound to "assignee_label", found none`)
	}
	if col.Kind != widget.NodeText {
		t.Errorf("link field list column should render as plain NodeText (no Options, so NodeSelect means nothing here), got %q", col.Kind)
	}
}

// TestGenerator_DetailView_LinkField_BindsToResolvedLabel guards the same
// defect in the detail view: both the summary card and the read-only form
// body must bind to "<field>_label", not show a disabled remote-select
// widget whose preset value never resolves to a label client-side.
func TestGenerator_DetailView_LinkField_BindsToResolvedLabel(t *testing.T) {
	g := generator.New()
	ctx := makeCtx(sduictx.ViewModeDetail, allPermsViewer())
	root, err := g.Generate(makeLinkSchema(), ctx)
	if err != nil {
		t.Fatal(err)
	}
	summaryCard := root.Children[0]
	formNode := root.Children[1]

	foundInSummary := false
	for _, c := range summaryCard.Children {
		if c.Name == "assignee_label" {
			foundInSummary = true
		}
	}
	if !foundInSummary {
		t.Error(`expected the detail summary card to bind to "assignee_label"`)
	}

	var formField *widget.Node
	widget.WalkAll(formNode, func(n *widget.Node) {
		if n.Name == "assignee" || n.Name == "assignee_label" {
			formField = n
		}
	})
	if formField == nil {
		t.Fatal("expected to find the assignee field somewhere in the detail form body")
	}
	if formField.Name != "assignee_label" {
		t.Errorf("detail form field must bind to %q, got %q (still showing the raw id, or a dead remote-select)", "assignee_label", formField.Name)
	}
	if formField.DataSource != nil {
		t.Error("detail form field must not carry a remote DataSource — it's read-only plain text now, nothing to search against")
	}
}

// TestGenerator_CreateView_LinkField_KeepsRemoteSelect proves the label
// substitution is detail-only: create/edit forms must keep binding to the
// raw field with its interactive remote-select widget, since that is what
// actually submits on save.
func TestGenerator_CreateView_LinkField_KeepsRemoteSelect(t *testing.T) {
	g := generator.New()
	ctx := makeCtx(sduictx.ViewModeCreate, allPermsViewer())
	root, err := g.Generate(makeLinkSchema(), ctx)
	if err != nil {
		t.Fatal(err)
	}
	var formField *widget.Node
	widget.WalkAll(root, func(n *widget.Node) {
		if n.Name == "assignee" {
			formField = n
		}
	})
	if formField == nil {
		t.Fatal(`expected the create form to bind to the raw "assignee" field`)
	}
	if formField.Kind != widget.NodeLookup {
		t.Errorf("create form link field should be NodeLookup (remote-search select), got %q", formField.Kind)
	}
	if formField.DataSource == nil {
		t.Error("create form link field must keep its remote DataSource")
	}
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
	// A real bulk-delete action is now expected: api/router registers a
	// genuine "DELETE {module}/{resource}" route (EntityHandler.DeleteBatch)
	// composed from the same per-record EntityService.Delete path, see
	// api/service/entity_batch_delete_pg_test.go for the real-Postgres
	// atomicity/authorization/tenant-isolation/lifecycle proof.
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

// TestGenerator_FilterBar_NeverRequired guards that a filter field never
// carries the entity field's own create/edit Required/validation semantics.
// buildFilterBar shares buildFieldNode with the form builder; a field that
// is Required=true for create/edit (e.g. "name" below, MaxLength 255) must
// not become an unsubmittable required filter — that would make the whole
// filter form unusable unless that one field was filled in, breaking
// "filter by any subset of fields" for every entity with a required field.
func TestGenerator_FilterBar_NeverRequired(t *testing.T) {
	g := generator.New()
	ctx := makeCtx(sduictx.ViewModeList, allPermsViewer())
	schema := generator.EntitySchema{
		Name:        "test",
		Title:       "Test",
		ListURL:     "/api/v1/test",
		Permissions: map[string]string{},
		Fields: []generator.FieldDef{
			{Name: "name", Label: "Name", FieldType: "data", InList: true, InForm: true, Searchable: true, Required: true, MaxLength: 255},
		},
	}
	root, err := g.Generate(schema, ctx)
	if err != nil {
		t.Fatal(err)
	}
	listNode := root.Children[0]
	if listNode.FilterBar == nil {
		t.Fatal("expected filter bar to be present")
	}
	if len(listNode.FilterBar.Children) != 1 {
		t.Fatalf("expected exactly 1 filter field, got %d", len(listNode.FilterBar.Children))
	}
	node := listNode.FilterBar.Children[0]
	if node.Required {
		t.Error("filter field must never be Required, even when the underlying entity field is Required for create/edit")
	}
	if node.Validation != nil {
		t.Errorf("filter field must not carry create/edit validation rules, got %v", node.Validation)
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
