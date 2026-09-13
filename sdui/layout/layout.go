// Package layout computes renderer-independent layout metadata from a widget.Node tree.
//
// # Responsibility
//
// The layout engine sits between the generator and the renderer. The generator
// produces a semantic widget tree; the renderer emits target-specific output.
// The layout engine fills the gap: it resolves column spans, groups fields into
// rows, and computes breakpoint-aware grid positions — all without knowing
// anything about AMIS, Flutter, or PDF.
//
// # Grid model
//
// Layout uses a 12-column grid at desktop, 8-column at tablet, and 4-column
// at mobile. Section column counts (1–4) are declared by the generator via
// widget.Node.Layout.ColSpan on the section node. Fields without an explicit
// span receive a default span of (DesktopGridCols / sectionCols).
//
// # Row packing
//
// Fields are packed left-to-right into rows. A new row is started when:
//   - The field's desktop span + used columns exceeds DesktopGridCols, or
//   - The field's widget.LayoutHint.NewRow is true.
//
// # Usage
//
//	eng := layout.New()
//	cl, err := eng.Compute(root)
//	// cl.Sections[i].Rows[j].Items[k].Span.Desktop → computed span
//
// Renderers translate Span values to their own layout system (CSS grid classes,
// AMIS grid colspan, Flutter CrossAxisAlignment, etc.).
//
// Package layout has no AMIS-specific or renderer-specific behaviour.
package layout

import (
	"fmt"

	"awo.so/awo/sdui/widget"
)

// Grid column counts at each breakpoint.
const (
	MobileGridCols  = 4
	TabletGridCols  = 8
	DesktopGridCols = 12
)

// DefaultSectionCols is the column count used for sections that declare no explicit column count.
const DefaultSectionCols = 2

// Span holds the column span for a node at each breakpoint.
// Values are in units of that breakpoint's grid columns.
type Span struct {
	Mobile  int // 1–MobileGridCols
	Tablet  int // 1–TabletGridCols
	Desktop int // 1–DesktopGridCols
}

// Full returns a Span that fills the entire grid at every breakpoint.
func Full() Span {
	return Span{Mobile: MobileGridCols, Tablet: TabletGridCols, Desktop: DesktopGridCols}
}

// RowItem is a positioned node within a computed row.
type RowItem struct {
	// Node is the widget node at this position.
	Node *widget.Node

	// Span is the computed column span at each breakpoint.
	Span Span

	// Offset is the number of empty grid columns to leave before this item.
	// Zero means no offset. Non-zero when Node.Layout.ColOffset is set.
	Offset Span
}

// Row is a horizontal grouping of RowItems that together occupy one grid row.
// Renderers convert each Row into a flex row, CSS grid row, or equivalent.
type Row struct {
	Items []RowItem
}

// SectionLayout is the computed layout for a NodeSection, NodeTabPane, or
// the implicit flat-field area of a NodeForm.
type SectionLayout struct {
	// Node is the section or tab-pane node. Nil for the implicit flat area.
	Node *widget.Node

	// Columns is the number of form columns declared for this section (1–4).
	Columns int

	// Rows are the computed horizontal rows of field nodes.
	Rows []Row
}

// TabLayout is the computed layout for a NodeTabs subtree.
type TabLayout struct {
	// Node is the NodeTabs node.
	Node *widget.Node

	// Panes holds the computed layout for each NodeTabPane child.
	Panes []SectionLayout
}

// ComputedLayout is the complete layout result for a widget tree rooted at NodePage.
// Renderers consume this structure to emit their native layout constructs without
// re-implementing grid packing logic.
type ComputedLayout struct {
	// Root is the NodePage node.
	Root *widget.Node

	// Sections are the computed section layouts from top-level NodeForm children.
	Sections []SectionLayout

	// Tabs are the tabbed layouts from NodeTabs children of top-level forms.
	Tabs []TabLayout

	// DashboardRows are the computed panel rows for dashboard pages.
	// Non-empty only when the page contains dashboard panel nodes
	// (NodeKPICard, NodeChartPanel, NodeTablePanel, NodeFilterBar).
	DashboardRows []Row
}

// Engine computes renderer-independent layout metadata from a widget.Node tree.
// Engine is safe for concurrent use; it holds no mutable state.
type Engine struct{}

// New returns a ready-to-use Engine.
func New() *Engine { return &Engine{} }

// Compute traverses root and returns its ComputedLayout.
// root must be a non-nil NodePage node.
// Returns an error if root is nil, not a NodePage, or contains impossible layout
// constraints (e.g. ColSpan > DesktopGridCols).
func (e *Engine) Compute(root *widget.Node) (*ComputedLayout, error) {
	if root == nil {
		return nil, fmt.Errorf("layout: root node is nil")
	}
	if root.Kind != widget.NodePage {
		return nil, fmt.Errorf("layout: root must be NodePage, got %q", root.Kind)
	}

	cl := &ComputedLayout{Root: root}

	// Detect whether this is a dashboard page by checking for any dashboard panel.
	if isDashboard(root) {
		cl.DashboardRows = e.packDashboardPanels(root.Children)
		return cl, nil
	}

	for _, child := range root.Children {
		if child == nil {
			continue
		}
		switch child.Kind {
		case widget.NodeForm:
			secs, tabs := e.computeForm(child)
			cl.Sections = append(cl.Sections, secs...)
			cl.Tabs = append(cl.Tabs, tabs...)
		case widget.NodeSummaryCard:
			// Full-width header card — no grid layout needed; record it as a
			// single full-width section for renderers that need a reference.
			cl.Sections = append(cl.Sections, SectionLayout{
				Node:    child,
				Columns: 1,
				Rows:    []Row{{Items: []RowItem{{Node: child, Span: Full()}}}},
			})
		case widget.NodeList:
			// List column layout is renderer-defined; layout engine skips.
		}
	}

	return cl, nil
}

// ── form layout ──────────────────────────────────────────────────────────────

func (e *Engine) computeForm(form *widget.Node) ([]SectionLayout, []TabLayout) {
	var sections []SectionLayout
	var tabs []TabLayout
	var flat []*widget.Node // fields not inside an explicit section

	for _, child := range form.Children {
		if child == nil {
			continue
		}
		switch child.Kind {
		case widget.NodeSection:
			sections = append(sections, e.computeSection(child))
		case widget.NodeTabs:
			tabs = append(tabs, e.computeTabs(child))
		default:
			flat = append(flat, child)
		}
	}

	// Flat fields → implicit section at the top.
	if len(flat) > 0 {
		implicit := e.packFields(nil, flat, DefaultSectionCols)
		sections = append([]SectionLayout{implicit}, sections...)
	}

	return sections, tabs
}

func (e *Engine) computeSection(sec *widget.Node) SectionLayout {
	cols := sectionColumns(sec)
	return e.packFields(sec, sec.Children, cols)
}

func (e *Engine) computeTabs(tabs *widget.Node) TabLayout {
	tl := TabLayout{Node: tabs}
	for _, child := range tabs.Children {
		if child == nil {
			continue
		}
		if child.Kind == widget.NodeTabPane {
			tl.Panes = append(tl.Panes, e.computeTabPane(child))
		}
	}
	return tl
}

func (e *Engine) computeTabPane(pane *widget.Node) SectionLayout {
	cols := sectionColumns(pane)
	// Nested sections inside a tab pane are flattened to their fields.
	var fields []*widget.Node
	for _, child := range pane.Children {
		if child == nil {
			continue
		}
		if child.Kind == widget.NodeSection {
			fields = append(fields, child.Children...)
		} else {
			fields = append(fields, child)
		}
	}
	return e.packFields(pane, fields, cols)
}

// ── row packing ──────────────────────────────────────────────────────────────

// packFields groups nodes into rows according to the grid packing algorithm.
// node is the parent section/pane node (nil for implicit flat area).
// fields are the direct child nodes to pack.
// cols is the number of declared form columns (1–4).
func (e *Engine) packFields(node *widget.Node, fields []*widget.Node, cols int) SectionLayout {
	if cols < 1 {
		cols = 1
	}
	if cols > 4 {
		cols = 4
	}

	sl := SectionLayout{Node: node, Columns: cols}

	desktopColWidth := DesktopGridCols / cols
	tabletCols := cols
	if tabletCols > 2 {
		tabletCols = 2
	}
	tabletColWidth := TabletGridCols / tabletCols

	var (
		currentRow  []RowItem
		desktopUsed int
	)

	flush := func() {
		if len(currentRow) > 0 {
			sl.Rows = append(sl.Rows, Row{Items: currentRow})
			currentRow = nil
			desktopUsed = 0
		}
	}

	for _, f := range fields {
		if f == nil {
			continue
		}

		// Compute desktop span.
		desktopSpan := desktopColWidth
		if f.Layout != nil && f.Layout.ColSpan > 0 {
			desktopSpan = f.Layout.ColSpan
			if desktopSpan > DesktopGridCols {
				desktopSpan = DesktopGridCols
			}
		} else if isFullWidthKind(f.Kind) {
			desktopSpan = DesktopGridCols
		}

		// Tablet span: proportional.
		tabletSpan := tabletColWidth
		if desktopSpan >= DesktopGridCols {
			tabletSpan = TabletGridCols
		} else if desktopSpan >= desktopColWidth*2 {
			tabletSpan = min(TabletGridCols, tabletColWidth*2)
		}

		// Desktop offset from ColOffset hint.
		desktopOffset := 0
		tabletOffset := 0
		if f.Layout != nil && f.Layout.ColOffset > 0 {
			desktopOffset = f.Layout.ColOffset
			// Clamp tablet offset so it doesn't overflow.
			tabletOffset = f.Layout.ColOffset * tabletColWidth / desktopColWidth
			if tabletOffset+tabletSpan > TabletGridCols {
				tabletOffset = 0
			}
		}

		// NewRow hint or insufficient space → flush current row.
		needsNewRow := (f.Layout != nil && f.Layout.NewRow) ||
			(desktopUsed+desktopOffset+desktopSpan > DesktopGridCols && desktopUsed > 0)
		if needsNewRow {
			flush()
		}

		currentRow = append(currentRow, RowItem{
			Node: f,
			Span: Span{Mobile: MobileGridCols, Tablet: tabletSpan, Desktop: desktopSpan},
			Offset: Span{
				Mobile:  0,
				Tablet:  tabletOffset,
				Desktop: desktopOffset,
			},
		})
		desktopUsed += desktopOffset + desktopSpan

		// Row is exactly full → flush.
		if desktopUsed >= DesktopGridCols {
			flush()
		}
	}
	flush()

	return sl
}

// ── dashboard layout ──────────────────────────────────────────────────────────

// packDashboardPanels groups dashboard panel nodes into rows using default panel spans.
func (e *Engine) packDashboardPanels(panels []*widget.Node) []Row {
	var rows []Row
	var currentRow []RowItem
	used := 0

	flush := func() {
		if len(currentRow) > 0 {
			rows = append(rows, Row{Items: currentRow})
			currentRow = nil
			used = 0
		}
	}

	for _, p := range panels {
		if p == nil {
			continue
		}
		span := dashboardSpan(p)
		if used+span.Desktop > DesktopGridCols && used > 0 {
			flush()
		}
		currentRow = append(currentRow, RowItem{Node: p, Span: span})
		used += span.Desktop
		if used >= DesktopGridCols {
			flush()
		}
	}
	flush()
	return rows
}

// ── helpers ───────────────────────────────────────────────────────────────────

// isDashboard returns true if any direct child of the page is a dashboard panel node.
func isDashboard(page *widget.Node) bool {
	for _, child := range page.Children {
		if child == nil {
			continue
		}
		switch child.Kind {
		case widget.NodeKPICard, widget.NodeChartPanel, widget.NodeTablePanel, widget.NodeFilterBar:
			return true
		}
	}
	return false
}

// dashboardSpan returns the default Span for a dashboard panel node.
// Explicit Layout.ColSpan overrides the default.
func dashboardSpan(n *widget.Node) Span {
	if n.Layout != nil && n.Layout.ColSpan > 0 {
		d := n.Layout.ColSpan
		if d > DesktopGridCols {
			d = DesktopGridCols
		}
		t := d * TabletGridCols / DesktopGridCols
		if t < 1 {
			t = 1
		}
		return Span{Mobile: MobileGridCols, Tablet: t, Desktop: d}
	}
	switch n.Kind {
	case widget.NodeKPICard:
		return Span{Mobile: MobileGridCols, Tablet: 4, Desktop: 3}
	case widget.NodeChartPanel:
		return Span{Mobile: MobileGridCols, Tablet: TabletGridCols, Desktop: 6}
	case widget.NodeFilterBar:
		return Span{Mobile: MobileGridCols, Tablet: TabletGridCols, Desktop: DesktopGridCols}
	default: // NodeTablePanel and others
		return Span{Mobile: MobileGridCols, Tablet: TabletGridCols, Desktop: DesktopGridCols}
	}
}

// sectionColumns returns the declared column count for a section or tab-pane node.
// Convention: the generator stores the section column count in Layout.ColSpan
// on the section node itself (not on its children). Falls back to DefaultSectionCols.
func sectionColumns(n *widget.Node) int {
	if n != nil && n.Layout != nil && n.Layout.ColSpan >= 1 && n.Layout.ColSpan <= 4 {
		return n.Layout.ColSpan
	}
	return DefaultSectionCols
}

// isFullWidthKind returns true for NodeKinds that should always span the full row
// regardless of section column count.
func isFullWidthKind(k widget.NodeKind) bool {
	switch k {
	case widget.NodeTextArea, widget.NodeRichText, widget.NodeEditor,
		widget.NodeSection, widget.NodeGrid, widget.NodeTable,
		widget.NodeRelatedList, widget.NodeWorkflowPanel,
		widget.NodeAttachments, widget.NodeActivity, widget.NodeStaticText:
		return true
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
