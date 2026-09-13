package layout_test

import (
	"fmt"
	"testing"

	"awo.so/awo/sdui/layout"
	"awo.so/awo/sdui/widget"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func page(children ...*widget.Node) *widget.Node {
	return &widget.Node{Kind: widget.NodePage, Children: children}
}

func form(children ...*widget.Node) *widget.Node {
	return &widget.Node{Kind: widget.NodeForm, Children: children}
}

func section(cols int, children ...*widget.Node) *widget.Node {
	n := &widget.Node{Kind: widget.NodeSection, Children: children}
	if cols > 0 {
		n.Layout = &widget.LayoutHint{ColSpan: cols}
	}
	return n
}

func tabs(panes ...*widget.Node) *widget.Node {
	return &widget.Node{Kind: widget.NodeTabs, Children: panes}
}

func tabPane(cols int, children ...*widget.Node) *widget.Node {
	n := &widget.Node{Kind: widget.NodeTabPane, Label: "Tab", Children: children}
	if cols > 0 {
		n.Layout = &widget.LayoutHint{ColSpan: cols}
	}
	return n
}

func field(name string) *widget.Node {
	return &widget.Node{Kind: widget.NodeText, Name: name}
}

func fieldSpan(name string, span int) *widget.Node {
	return &widget.Node{Kind: widget.NodeText, Name: name,
		Layout: &widget.LayoutHint{ColSpan: span}}
}

func fieldNewRow(name string) *widget.Node {
	return &widget.Node{Kind: widget.NodeText, Name: name,
		Layout: &widget.LayoutHint{NewRow: true}}
}

func textarea(name string) *widget.Node {
	return &widget.Node{Kind: widget.NodeTextArea, Name: name}
}

func kpiCard(name string) *widget.Node {
	return &widget.Node{Kind: widget.NodeKPICard, Name: name}
}

func chartPanel(name string) *widget.Node {
	return &widget.Node{Kind: widget.NodeChartPanel, Name: name}
}

func tablePanel(name string) *widget.Node {
	return &widget.Node{Kind: widget.NodeTablePanel, Name: name}
}

// ── error cases ───────────────────────────────────────────────────────────────

func TestEngine_NilRoot(t *testing.T) {
	e := layout.New()
	_, err := e.Compute(nil)
	if err == nil {
		t.Error("expected error for nil root")
	}
}

func TestEngine_NonPageRoot(t *testing.T) {
	e := layout.New()
	_, err := e.Compute(form())
	if err == nil {
		t.Error("expected error for non-page root")
	}
}

// ── flat field layout ─────────────────────────────────────────────────────────

func TestEngine_FlatFields_TwoColumns(t *testing.T) {
	// 2 fields in a 2-col form → each gets desktop span 6, same row.
	e := layout.New()
	root := page(form(field("a"), field("b")))
	cl, err := e.Compute(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(cl.Sections) == 0 {
		t.Fatal("expected at least one section")
	}
	sec := cl.Sections[0]
	if sec.Columns != layout.DefaultSectionCols {
		t.Errorf("columns = %d, want %d", sec.Columns, layout.DefaultSectionCols)
	}
	if len(sec.Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(sec.Rows))
	}
	row := sec.Rows[0]
	if len(row.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(row.Items))
	}
	for _, item := range row.Items {
		if item.Span.Desktop != layout.DesktopGridCols/layout.DefaultSectionCols {
			t.Errorf("field %q desktop span = %d, want 6", item.Node.Name, item.Span.Desktop)
		}
		if item.Span.Mobile != layout.MobileGridCols {
			t.Errorf("field %q mobile span = %d, want %d", item.Node.Name, item.Span.Mobile, layout.MobileGridCols)
		}
	}
}

func TestEngine_ThreeFields_TwoColumns_TwoRows(t *testing.T) {
	// 3 fields, 2-col → row1: [a,b], row2: [c]
	e := layout.New()
	root := page(form(field("a"), field("b"), field("c")))
	cl, err := e.Compute(root)
	if err != nil {
		t.Fatal(err)
	}
	sec := cl.Sections[0]
	if len(sec.Rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(sec.Rows))
	}
	if len(sec.Rows[0].Items) != 2 {
		t.Errorf("row1 items = %d, want 2", len(sec.Rows[0].Items))
	}
	if len(sec.Rows[1].Items) != 1 {
		t.Errorf("row2 items = %d, want 1", len(sec.Rows[1].Items))
	}
}

func TestEngine_FullWidthField_OwnRow(t *testing.T) {
	// textarea is full-width → forces its own row.
	e := layout.New()
	root := page(form(field("a"), textarea("notes"), field("b")))
	cl, err := e.Compute(root)
	if err != nil {
		t.Fatal(err)
	}
	sec := cl.Sections[0]
	// Expected rows: [a], [notes], [b]  — or [a] then notes auto-flushes
	// a fits half-row, notes is full-width so must be alone, b is next row
	if len(sec.Rows) < 2 {
		t.Errorf("expected ≥2 rows for mixed fields, got %d", len(sec.Rows))
	}
	// Find textarea row.
	found := false
	for _, row := range sec.Rows {
		for _, item := range row.Items {
			if item.Node.Kind == widget.NodeTextArea {
				if item.Span.Desktop != layout.DesktopGridCols {
					t.Errorf("textarea desktop span = %d, want %d", item.Span.Desktop, layout.DesktopGridCols)
				}
				found = true
			}
		}
	}
	if !found {
		t.Error("textarea not found in computed rows")
	}
}

func TestEngine_ExplicitColSpan(t *testing.T) {
	// Field with ColSpan=4 in 12-col desktop.
	e := layout.New()
	root := page(form(fieldSpan("f", 4)))
	cl, err := e.Compute(root)
	if err != nil {
		t.Fatal(err)
	}
	item := cl.Sections[0].Rows[0].Items[0]
	if item.Span.Desktop != 4 {
		t.Errorf("desktop span = %d, want 4", item.Span.Desktop)
	}
}

func TestEngine_NewRowHint(t *testing.T) {
	// Three fields; second has NewRow=true → row1: [a], row2: [b, c]
	e := layout.New()
	root := page(form(field("a"), fieldNewRow("b"), field("c")))
	cl, err := e.Compute(root)
	if err != nil {
		t.Fatal(err)
	}
	sec := cl.Sections[0]
	if len(sec.Rows) < 2 {
		t.Fatalf("rows = %d, want ≥2", len(sec.Rows))
	}
	if sec.Rows[0].Items[0].Node.Name != "a" {
		t.Errorf("row1 item0 = %q, want \"a\"", sec.Rows[0].Items[0].Node.Name)
	}
	if sec.Rows[1].Items[0].Node.Name != "b" {
		t.Errorf("row2 item0 = %q, want \"b\"", sec.Rows[1].Items[0].Node.Name)
	}
}

// ── section layout ────────────────────────────────────────────────────────────

func TestEngine_Section_FourColumns(t *testing.T) {
	// 4-col section: desktop span = 12/4 = 3 per field.
	e := layout.New()
	root := page(form(section(4, field("a"), field("b"), field("c"), field("d"))))
	cl, err := e.Compute(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(cl.Sections) == 0 {
		t.Fatal("no sections")
	}
	sec := cl.Sections[0]
	if sec.Columns != 4 {
		t.Errorf("columns = %d, want 4", sec.Columns)
	}
	if len(sec.Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(sec.Rows))
	}
	for _, item := range sec.Rows[0].Items {
		if item.Span.Desktop != 3 {
			t.Errorf("field %q desktop = %d, want 3", item.Node.Name, item.Span.Desktop)
		}
	}
}

// ── tab layout ────────────────────────────────────────────────────────────────

func TestEngine_TabLayout(t *testing.T) {
	e := layout.New()
	root := page(form(
		tabs(
			tabPane(2, field("x"), field("y")),
			tabPane(1, textarea("notes")),
		),
	))
	cl, err := e.Compute(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(cl.Tabs) != 1 {
		t.Fatalf("tabs = %d, want 1", len(cl.Tabs))
	}
	tab := cl.Tabs[0]
	if len(tab.Panes) != 2 {
		t.Fatalf("panes = %d, want 2", len(tab.Panes))
	}
	// Pane 0: 2-col, 2 fields → 1 row.
	if len(tab.Panes[0].Rows) != 1 {
		t.Errorf("pane0 rows = %d, want 1", len(tab.Panes[0].Rows))
	}
	if len(tab.Panes[0].Rows[0].Items) != 2 {
		t.Errorf("pane0 row0 items = %d, want 2", len(tab.Panes[0].Rows[0].Items))
	}
	// Pane 1: 1-col, textarea → 1 row, full width.
	if len(tab.Panes[1].Rows) != 1 {
		t.Errorf("pane1 rows = %d, want 1", len(tab.Panes[1].Rows))
	}
	if tab.Panes[1].Rows[0].Items[0].Span.Desktop != layout.DesktopGridCols {
		t.Errorf("textarea span = %d, want %d", tab.Panes[1].Rows[0].Items[0].Span.Desktop, layout.DesktopGridCols)
	}
}

// ── dashboard layout ──────────────────────────────────────────────────────────

func TestEngine_DashboardLayout_KPICards(t *testing.T) {
	// 4 KPI cards at span 3 → all fit in one row of 12.
	e := layout.New()
	root := page(
		kpiCard("revenue"), kpiCard("expenses"),
		kpiCard("profit"), kpiCard("invoices"),
	)
	cl, err := e.Compute(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(cl.DashboardRows) != 1 {
		t.Fatalf("dashboard rows = %d, want 1", len(cl.DashboardRows))
	}
	if len(cl.DashboardRows[0].Items) != 4 {
		t.Errorf("row items = %d, want 4", len(cl.DashboardRows[0].Items))
	}
	for _, item := range cl.DashboardRows[0].Items {
		if item.Span.Desktop != 3 {
			t.Errorf("KPI card span = %d, want 3", item.Span.Desktop)
		}
	}
}

func TestEngine_DashboardLayout_MixedPanels(t *testing.T) {
	// 2 KPI (3+3=6), 1 chart (6) → row1 full; table (12) → row2.
	e := layout.New()
	root := page(
		kpiCard("a"), kpiCard("b"),
		chartPanel("sales"),
		tablePanel("recent"),
	)
	cl, err := e.Compute(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(cl.DashboardRows) != 2 {
		t.Fatalf("dashboard rows = %d, want 2", len(cl.DashboardRows))
	}
	// Row 1: 2 KPIs + chart = 3+3+6 = 12.
	if len(cl.DashboardRows[0].Items) != 3 {
		t.Errorf("row1 items = %d, want 3", len(cl.DashboardRows[0].Items))
	}
	// Row 2: table.
	if len(cl.DashboardRows[1].Items) != 1 {
		t.Errorf("row2 items = %d, want 1", len(cl.DashboardRows[1].Items))
	}
}

func TestEngine_DashboardLayout_ExplicitSpan(t *testing.T) {
	// Panel with explicit ColSpan=4 overrides default.
	e := layout.New()
	root := page(&widget.Node{
		Kind:   widget.NodeKPICard,
		Name:   "custom",
		Layout: &widget.LayoutHint{ColSpan: 4},
	})
	cl, err := e.Compute(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(cl.DashboardRows) == 0 || len(cl.DashboardRows[0].Items) == 0 {
		t.Fatal("no items")
	}
	if cl.DashboardRows[0].Items[0].Span.Desktop != 4 {
		t.Errorf("span = %d, want 4", cl.DashboardRows[0].Items[0].Span.Desktop)
	}
}

// ── SummaryCard ───────────────────────────────────────────────────────────────

func TestEngine_SummaryCard_FullWidth(t *testing.T) {
	e := layout.New()
	card := &widget.Node{Kind: widget.NodeSummaryCard, Name: "header"}
	root := page(card, form(field("x")))
	cl, err := e.Compute(root)
	if err != nil {
		t.Fatal(err)
	}
	// First section should be the summary card at full width.
	if len(cl.Sections) == 0 {
		t.Fatal("no sections")
	}
	cardSection := cl.Sections[0]
	if len(cardSection.Rows) != 1 || len(cardSection.Rows[0].Items) != 1 {
		t.Error("summary card not in its own full-width row")
	}
	if cardSection.Rows[0].Items[0].Span.Desktop != layout.DesktopGridCols {
		t.Error("summary card must span full desktop width")
	}
}

// ── benchmarks ────────────────────────────────────────────────────────────────

func BenchmarkEngine_Compute_SmallForm(b *testing.B) {
	e := layout.New()
	root := page(form(
		field("name"), field("email"),
		field("phone"), field("company"),
		textarea("notes"),
	))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := e.Compute(root); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEngine_Compute_LargeForm(b *testing.B) {
	fields := make([]*widget.Node, 30)
	for i := range fields {
		fields[i] = field(fmt.Sprintf("f%d", i))
	}
	e := layout.New()
	root := page(form(fields...))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := e.Compute(root); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEngine_Compute_Dashboard(b *testing.B) {
	e := layout.New()
	root := page(
		kpiCard("a"), kpiCard("b"), kpiCard("c"), kpiCard("d"),
		chartPanel("sales"), chartPanel("expenses"),
		tablePanel("recent"),
	)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := e.Compute(root); err != nil {
			b.Fatal(err)
		}
	}
}
