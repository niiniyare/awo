package amis_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"awo.so/awo/sdui/amis"
	"awo.so/awo/sdui/expression"
	"awo.so/awo/sdui/renderer"
	"awo.so/awo/sdui/sduictx"
	"awo.so/awo/sdui/widget"
)

// ── Test helpers ──────────────────────────────────────────────────────────────

type stubViewer struct{}

func (stubViewer) TenantID() uuid.UUID       { return uuid.New() }
func (stubViewer) Roles() []string           { return nil }
func (stubViewer) IsPlatformAdmin() bool     { return false }
func (stubViewer) HasPermission(string) bool { return true }

func makeCtx() renderer.RendererContext {
	genCtx, _ := sduictx.NewGeneratorContext(
		uuid.New(), stubViewer{}, "test_entity", sduictx.ViewModeList, amis.RendererID,
	).Build()
	return renderer.RendererContext{GenCtx: genCtx}
}

func render(t *testing.T, n *widget.Node) map[string]any {
	t.Helper()
	r := amis.New()
	out, err := r.Render(n, makeCtx())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	return out.AMISSchema
}

// ── Tests ─────────────────────────────────────────────────────────────────────

func TestRenderer_NilRoot(t *testing.T) {
	t.Parallel()
	r := amis.New()
	_, err := r.Render(nil, makeCtx())
	if err == nil {
		t.Error("Render(nil) must return error")
	}
}

func TestRenderer_UnknownNodeKind(t *testing.T) {
	t.Parallel()
	r := amis.New()
	_, err := r.Render(&widget.Node{Kind: "unknown-kind-xyz"}, makeCtx())
	if err == nil {
		t.Error("Render with unknown NodeKind must return error")
	}
}

func TestRenderer_HiddenRootStillErrors(t *testing.T) {
	// A hidden root emits nil schema — the pipeline should have removed it before calling Render.
	// This documents the behaviour: Render on a hidden root returns (empty output, nil error)
	// because hidden == omit.
	t.Parallel()
	r := amis.New()
	out, err := r.Render(&widget.Node{Kind: widget.NodeText, Name: "x", Hidden: true}, makeCtx())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.AMISSchema != nil {
		t.Errorf("expected nil AMISSchema for hidden root, got %v", out.AMISSchema)
	}
}

func TestRenderer_PageType(t *testing.T) {
	t.Parallel()
	out := render(t, &widget.Node{Kind: widget.NodePage, Label: "Invoices"})
	if out["type"] != "page" {
		t.Errorf("type=%v want page", out["type"])
	}
	if out["title"] != "Invoices" {
		t.Errorf("title=%v want Invoices", out["title"])
	}
}

func TestRenderer_FormAPI(t *testing.T) {
	t.Parallel()
	n := &widget.Node{
		Kind: widget.NodeForm,
		DataSource: &widget.DataSource{
			URL:    "/api/v1/finance/invoices",
			Method: "POST",
		},
		Children: []*widget.Node{{Kind: widget.NodeText, Name: "number", Label: "Number"}},
	}
	out := render(t, n)
	if out["type"] != "form" {
		t.Errorf("type=%v want form", out["type"])
	}
	api := out["api"].(map[string]any)
	if api["url"] != "/api/v1/finance/invoices" {
		t.Errorf("api.url=%v", api["url"])
	}
	if api["method"] != "POST" {
		t.Errorf("api.method=%v want POST", api["method"])
	}
}

func TestRenderer_ListColumns(t *testing.T) {
	t.Parallel()
	n := &widget.Node{
		Kind:       widget.NodeList,
		DataSource: &widget.DataSource{URL: "/api/v1/invoices"},
		Children: []*widget.Node{
			{Kind: widget.NodeText, Name: "number", Label: "Number"},
			{Kind: widget.NodeDate, Name: "date", Label: "Date"},
		},
	}
	out := render(t, n)
	// Must be "crud" not "crud2": crud honours toolbar string shortcuts.
	if out["type"] != "crud" {
		t.Errorf("type=%v want crud", out["type"])
	}
	cols := out["columns"].([]any)
	if len(cols) != 2 {
		t.Errorf("expected 2 columns, got %d", len(cols))
	}
}

func TestRenderer_TextFieldProperties(t *testing.T) {
	t.Parallel()
	out := render(t, &widget.Node{
		Kind:     widget.NodeText,
		Name:     "email",
		Label:    "Email",
		Required: true,
		ReadOnly: true,
	})
	if out["type"] != "input-text" {
		t.Errorf("type=%v want input-text", out["type"])
	}
	if out["name"] != "email" {
		t.Errorf("name=%v", out["name"])
	}
	if out["required"] != true {
		t.Errorf("required=%v want true", out["required"])
	}
	if out["disabled"] != true {
		t.Errorf("disabled=%v want true (ReadOnly→disabled)", out["disabled"])
	}
}

func TestRenderer_SelectValueField_Default(t *testing.T) {
	t.Parallel()
	out := render(t, &widget.Node{
		Kind: widget.NodeSelect,
		Name: "currency_id",
		DataSource: &widget.DataSource{
			URL:        "/api/v1/currencies",
			LabelField: "code",
			// ValueField empty → must default to "id"
		},
	})
	if out["valueField"] != "id" {
		t.Errorf("valueField=%v want id (default)", out["valueField"])
	}
}

func TestRenderer_PropsOverride(t *testing.T) {
	t.Parallel()
	out := render(t, &widget.Node{
		Kind:  widget.NodeText,
		Name:  "code",
		Label: "Code",
		Props: map[string]any{"label": "Custom Label"},
	})
	if out["label"] != "Custom Label" {
		t.Errorf("Props.label should override: got %v", out["label"])
	}
}

func TestRenderer_IDEmitted(t *testing.T) {
	t.Parallel()
	out := render(t, &widget.Node{Kind: widget.NodePage, ID: "invoice-page"})
	if out["id"] != "invoice-page" {
		t.Errorf("id=%v want invoice-page", out["id"])
	}
}

func TestRenderer_HiddenChildOmitted(t *testing.T) {
	t.Parallel()
	out := render(t, &widget.Node{
		Kind: widget.NodePage,
		Children: []*widget.Node{
			{Kind: widget.NodeText, Name: "visible"},
			{Kind: widget.NodeText, Name: "secret", Hidden: true},
		},
	})
	body := out["body"].([]any)
	if len(body) != 1 {
		t.Errorf("body has %d items, want 1 (hidden child must be absent)", len(body))
	}
}

func TestRenderer_XSSSanitized(t *testing.T) {
	t.Parallel()
	out := render(t, &widget.Node{Kind: widget.NodePage, Label: "<script>alert('xss')</script>"})
	title := out["title"].(string)
	if title == "<script>alert('xss')</script>" {
		t.Error("XSS: label not sanitized")
	}
}

func TestRenderer_VisibleOnExpression(t *testing.T) {
	t.Parallel()
	exprNode := expression.Eq(expression.Field("status"), expression.Lit("active"))
	out := render(t, &widget.Node{
		Kind:      widget.NodePage,
		VisibleOn: &widget.ExpressionRef{Expr: exprNode},
	})
	got := out["visibleOn"]
	if got != "data.status === 'active'" {
		t.Errorf("visibleOn=%q want data.status === 'active'", got)
	}
}

func TestRenderer_DateFormat(t *testing.T) {
	t.Parallel()
	out := render(t, &widget.Node{Kind: widget.NodeDate, Name: "date"})
	if out["format"] != "YYYY-MM-DD" {
		t.Errorf("format=%v", out["format"])
	}
}

func TestRenderer_DateTimeFormat(t *testing.T) {
	t.Parallel()
	out := render(t, &widget.Node{Kind: widget.NodeDateTime, Name: "ts"})
	if out["format"] != "YYYY-MM-DDTHH:mm:ssZ" {
		t.Errorf("format=%v", out["format"])
	}
}

func TestRenderer_PageActions(t *testing.T) {
	t.Parallel()
	out := render(t, &widget.Node{
		Kind:  widget.NodePage,
		Label: "Detail",
		Actions: []*widget.ActionNode{
			{Label: "Submit", ActionType: "ajax", Level: "primary", API: "/api/submit", ConfirmText: "Sure?"},
		},
	})
	toolbar := out["toolbar"].([]any)
	if len(toolbar) != 1 {
		t.Fatalf("expected 1 action, got %d", len(toolbar))
	}
	btn := toolbar[0].(map[string]any)
	if btn["label"] != "Submit" {
		t.Errorf("label=%v", btn["label"])
	}
	if btn["confirmText"] != "Sure?" {
		t.Errorf("confirmText=%v", btn["confirmText"])
	}
}

func TestRenderer_TabPaneInsideTabs(t *testing.T) {
	t.Parallel()
	out := render(t, &widget.Node{
		Kind: widget.NodePage,
		Children: []*widget.Node{{
			Kind: widget.NodeTabs,
			Children: []*widget.Node{{
				Kind:     widget.NodeTabPane,
				Label:    "Details",
				Children: []*widget.Node{{Kind: widget.NodeText, Name: "name"}},
			}},
		}},
	})
	body := out["body"].([]any)
	tabs := body[0].(map[string]any)
	if tabs["type"] != "tabs" {
		t.Errorf("type=%v want tabs", tabs["type"])
	}
	tabList := tabs["tabs"].([]any)
	if len(tabList) != 1 {
		t.Fatalf("expected 1 tab, got %d", len(tabList))
	}
	tab := tabList[0].(map[string]any)
	if tab["title"] != "Details" {
		t.Errorf("tab title=%v want Details", tab["title"])
	}
}

func TestRenderer_GridEditable(t *testing.T) {
	t.Parallel()
	out := render(t, &widget.Node{
		Kind:         widget.NodeGrid,
		Name:         "line_items",
		Label:        "Line Items",
		GridEditable: true,
		Children: []*widget.Node{
			{Kind: widget.NodeText, Name: "description", Label: "Description"},
			{Kind: widget.NodeNumber, Name: "quantity", Label: "Qty"},
		},
	})
	if out["type"] != "input-table" {
		t.Errorf("type=%v want input-table", out["type"])
	}
	if out["addable"] != true || out["editable"] != true || out["removable"] != true {
		t.Error("editable grid must have addable/editable/removable=true")
	}
}

func TestRenderer_AllInputKinds(t *testing.T) {
	t.Parallel()
	cases := []struct {
		kind     widget.NodeKind
		wantType string
	}{
		{widget.NodeText, "input-text"},
		{widget.NodeField, "input-text"},
		{widget.NodeTextArea, "textarea"},
		{widget.NodeRichText, "rich-text"},
		{widget.NodeNumber, "input-number"},
		{widget.NodeDate, "input-date"},
		{widget.NodeDateTime, "input-datetime"},
		{widget.NodeSwitch, "switch"},
		{widget.NodeEditor, "json-editor"},
		{widget.NodeColor, "input-color"},
		{widget.NodeFileUpload, "input-file"},
	}
	r := amis.New()
	ctx := makeCtx()
	for _, tc := range cases {
		tc := tc
		t.Run(string(tc.kind), func(t *testing.T) {
			t.Parallel()
			out, err := r.Render(&widget.Node{Kind: tc.kind, Name: "f", Label: "F"}, ctx)
			if err != nil {
				t.Fatalf("Render(%s): %v", tc.kind, err)
			}
			if out.AMISSchema["type"] != tc.wantType {
				t.Errorf("Render(%s): type=%v want %q", tc.kind, out.AMISSchema["type"], tc.wantType)
			}
		})
	}
}

// ── Session 5 regression tests ────────────────────────────────────────────────

// TestRenderer_NumberColumn_IsNumber guards against regression where NodeNumber
// and NodeMoney list columns were emitted as type "tpl" (renders blank in AMIS).
// Fix: nodeKindToColumnType must return "number" for these kinds.
func TestRenderer_NumberColumn_IsNumber(t *testing.T) {
	t.Parallel()
	for _, kind := range []widget.NodeKind{widget.NodeNumber, widget.NodeMoney} {
		kind := kind
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()
			n := &widget.Node{
				Kind:       widget.NodeList,
				DataSource: &widget.DataSource{URL: "/api/v1/items"},
				Children: []*widget.Node{
					{Kind: kind, Name: "amount", Label: "Amount"},
				},
			}
			out := render(t, n)
			cols := out["columns"].([]any)
			// First column is the amount; no row-action column (no scoped actions).
			if len(cols) < 1 {
				t.Fatal("expected at least 1 column")
			}
			col := cols[0].(map[string]any)
			if col["type"] != "number" {
				t.Errorf("column type=%v want number (kind=%s)", col["type"], kind)
			}
		})
	}
}

// TestRenderer_ListItemsKey guards that itemsKey is always "items" so that
// adaptResponse()'s { items: [...] } shape matches crud's expectations.
func TestRenderer_ListItemsKey(t *testing.T) {
	t.Parallel()
	n := &widget.Node{
		Kind:       widget.NodeList,
		DataSource: &widget.DataSource{URL: "/api/v1/items"},
	}
	out := render(t, n)
	if out["itemsKey"] != "items" {
		t.Errorf("itemsKey=%v want items", out["itemsKey"])
	}
}

// TestRenderer_DetailForm_InitApi guards that detail forms use initApi (not api)
// so AMIS loads the record on mount rather than waiting for a submit event.
func TestRenderer_DetailForm_InitApi(t *testing.T) {
	t.Parallel()
	n := &widget.Node{
		Kind: widget.NodeForm,
		Children: []*widget.Node{
			{Kind: widget.NodeText, Name: "name", Label: "Name"},
		},
		DataSource: &widget.DataSource{
			ReadURL: "/api/v1/items/${id}",
		},
	}
	out := render(t, n)
	if _, ok := out["initApi"]; !ok {
		t.Error("form with ReadURL must emit initApi (AMIS loads on mount)")
	}
	if _, ok := out["api"]; ok {
		t.Error("form with ReadURL only must NOT emit api (would fire on submit, not mount)")
	}
}

// ── Session 6 regression tests ────────────────────────────────────────────────

// TestRenderer_ListRowActions_FromScope guards that row operations come from
// Scope="row" ActionNodes rather than being hardcoded by the renderer.
// This ensures permission-gating is respected: the generator omits actions
// the viewer doesn't have, and the renderer renders only what it receives.
func TestRenderer_ListRowActions_FromScope(t *testing.T) {
	t.Parallel()
	n := &widget.Node{
		Kind:       widget.NodeList,
		DataSource: &widget.DataSource{URL: "/api/v1/items"},
		Actions: []*widget.ActionNode{
			{ID: "create", Label: "New", ActionType: "link", Href: "/ui/demo/items/create", Scope: "toolbar"},
			{ID: "view", Label: "View", ActionType: "link", Href: "/ui/demo/items/${id}", Scope: "row"},
			{ID: "edit", Label: "Edit", ActionType: "link", Href: "/ui/demo/items/${id}/edit", Scope: "row"},
			{ID: "delete", Label: "Delete", ActionType: "ajax", API: "DELETE:/api/v1/items/${id}", Scope: "row"},
		},
	}
	out := render(t, n)
	cols := out["columns"].([]any)
	// With 3 row-scoped actions, an operation column must be appended.
	var opCol map[string]any
	for _, c := range cols {
		cm := c.(map[string]any)
		if cm["type"] == "operation" {
			opCol = cm
			break
		}
	}
	if opCol == nil {
		t.Fatal("expected an operation column for row-scoped actions")
	}
	btns := opCol["buttons"].([]any)
	if len(btns) != 3 {
		t.Errorf("expected 3 row buttons (view/edit/delete), got %d", len(btns))
	}
}

// TestRenderer_ListNoRowActions_NoOpColumn guards that when no row-scoped actions
// exist (e.g. viewer has no permissions), no operation column is emitted.
func TestRenderer_ListNoRowActions_NoOpColumn(t *testing.T) {
	t.Parallel()
	n := &widget.Node{
		Kind:       widget.NodeList,
		DataSource: &widget.DataSource{URL: "/api/v1/items"},
		Actions: []*widget.ActionNode{
			// Only toolbar action — no row-scoped actions.
			{ID: "create", Label: "New", ActionType: "link", Href: "/ui/demo/items/create", Scope: "toolbar"},
		},
		Children: []*widget.Node{
			{Kind: widget.NodeText, Name: "name", Label: "Name"},
		},
	}
	out := render(t, n)
	cols := out["columns"].([]any)
	for _, c := range cols {
		cm := c.(map[string]any)
		if cm["type"] == "operation" {
			t.Error("no operation column expected when no row-scoped actions present")
		}
	}
}

// TestRenderer_ListBulkActions_EmptySlice guards that bulkActions is always a
// non-nil empty slice when no bulk actions are defined — not JSON null.
// AMIS treats null differently from [] for the bulkActions property.
func TestRenderer_ListBulkActions_EmptySlice(t *testing.T) {
	t.Parallel()
	n := &widget.Node{
		Kind:       widget.NodeList,
		DataSource: &widget.DataSource{URL: "/api/v1/items"},
		// No bulk-scoped actions.
	}
	out := render(t, n)
	bulk, ok := out["bulkActions"]
	if !ok {
		t.Fatal("bulkActions key must always be present in crud schema")
	}
	if bulk == nil {
		t.Error("bulkActions must be []any{} not nil (null vs [] differ in AMIS)")
	}
	bulkSlice, ok := bulk.([]any)
	if !ok {
		t.Errorf("bulkActions must be []any, got %T", bulk)
	}
	if len(bulkSlice) != 0 {
		t.Errorf("expected empty bulkActions, got %d entries", len(bulkSlice))
	}
}

// TestRenderer_ListBulkActions_Rendered guards that bulk-scoped actions become
// entries in the bulkActions array (enabling row checkboxes).
func TestRenderer_ListBulkActions_Rendered(t *testing.T) {
	t.Parallel()
	n := &widget.Node{
		Kind:       widget.NodeList,
		DataSource: &widget.DataSource{URL: "/api/v1/items"},
		Actions: []*widget.ActionNode{
			{
				ID:          "bulk-delete",
				Label:       "Delete selected",
				ActionType:  "ajax",
				Level:       "danger",
				API:         "DELETE:/api/v1/items",
				ConfirmText: "Delete selected?",
				Scope:       "bulk",
			},
		},
	}
	out := render(t, n)
	bulkSlice, ok := out["bulkActions"].([]any)
	if !ok || len(bulkSlice) == 0 {
		t.Fatal("expected one entry in bulkActions for bulk-scoped action")
	}
	btn := bulkSlice[0].(map[string]any)
	if btn["label"] != "Delete selected" {
		t.Errorf("bulk action label=%v want 'Delete selected'", btn["label"])
	}
}

// TestRenderer_DeleteAction_HasAPI guards that delete ActionNodes with API set
// have their api property rendered. This guards against the Session 6 bug where
// buildDetailActions emitted delete buttons with no API URL (ajax with no url = no-op).
func TestRenderer_DeleteAction_HasAPI(t *testing.T) {
	t.Parallel()
	n := &widget.Node{
		Kind: widget.NodeSummaryCard,
		Actions: []*widget.ActionNode{
			{
				ID:         "delete",
				Label:      "Delete",
				ActionType: "ajax",
				Level:      "danger",
				API:        "DELETE:/api/v1/items/${id}",
			},
		},
	}
	out := render(t, n)
	acts, ok := out["actions"].([]any)
	if !ok || len(acts) == 0 {
		t.Fatal("expected at least one action on SummaryCard")
	}
	btn := acts[0].(map[string]any)
	if btn["api"] != "DELETE:/api/v1/items/${id}" {
		t.Errorf("delete action api=%v want DELETE:/api/v1/items/${id}", btn["api"])
	}
}

// ── Session 10 security regression tests ──────────────────────────────────────

// TestRenderer_LabelNotHTMLEscaped guards that sanitizeText does NOT apply
// html.EscapeString to labels. AMIS renders labels as React text nodes (not
// innerHTML), so "&" must appear as "&" in the output — not "&amp;".
//
// This was a latent bug: html.EscapeString("R&D") = "R&amp;D", which AMIS
// would display literally as "R&amp;D" in the UI.
func TestRenderer_LabelNotHTMLEscaped(t *testing.T) {
	t.Parallel()
	out := render(t, &widget.Node{Kind: widget.NodePage, Label: "R&D Portal"})
	title, _ := out["title"].(string)
	if title != "R&D Portal" {
		t.Errorf("label must not be HTML-escaped: got %q, want %q", title, "R&D Portal")
	}
}

func TestRenderer_LabelAmpersandNotEncoded(t *testing.T) {
	t.Parallel()
	// The ampersand must NOT be HTML-encoded. AMIS renders labels as React text
	// nodes — encoding & to &amp; would display "&amp;" literally in the UI.
	for _, label := range []string{"A & B", "R&D", "Cash & Carry"} {
		label := label
		t.Run(label, func(t *testing.T) {
			t.Parallel()
			out := render(t, &widget.Node{Kind: widget.NodePage, Label: label})
			title, _ := out["title"].(string)
			if title != label {
				t.Errorf("label %q: got %q (ampersand must not be encoded)", label, title)
			}
		})
	}
}

func TestRenderer_LabelAngleBracketsStripped(t *testing.T) {
	t.Parallel()
	// Angle brackets are stripped as defense-in-depth. They never appear in
	// legitimate entity metadata and could be dangerous if AMIS ever uses innerHTML.
	out := render(t, &widget.Node{Kind: widget.NodePage, Label: "<Draft>"})
	title, _ := out["title"].(string)
	if strings.Contains(title, "<") || strings.Contains(title, ">") {
		t.Errorf("angle brackets must be stripped from labels: got %q", title)
	}
}

// Ensure _ imports compile when expression package is used in test file.
var _ = expression.Field
