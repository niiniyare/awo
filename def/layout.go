package def

// LayoutDef declares the SDUI layout for form and detail views of an entity.
//
// Zero value produces a flat layout: all fields rendered in declaration order,
// no grouping. Set Tabs or Sections to override.
//
// Layout is schema-driven and evaluated at compile time. The SDUI generator
// reads Layout from the EntitySchema and emits the corresponding WidgetTree
// nodes. No JavaScript or handwritten amis JSON is required.
//
// Hierarchy:
//
//	LayoutDef
//	  └─ Tabs []TabDef        (optional; wraps form in tabbed container)
//	       └─ Sections []SectionDef
//	            └─ Columns []ColumnDef
//	                 └─ Fields []string  (field names from FieldDef.Name)
//
// When Tabs is non-empty, Sections at the root is ignored — all sections must
// be declared inside a tab.
//
// Layout only applies to form and detail views. List columns are always
// auto-generated from field declarations and are not affected by Layout.
type LayoutDef struct {
	// Tabs, when non-empty, wraps form body in a tabbed container.
	// Each tab renders as an AMIS tab pane containing its sections.
	// When Tabs is non-empty, root Sections is ignored.
	Tabs []TabDef

	// Sections, used when Tabs is empty, groups fields into labeled or
	// collapsible sections rendered as AMIS fieldSet controls.
	Sections []SectionDef
}

// TabDef declares one tab pane in a tabbed form layout.
type TabDef struct {
	// Name is the stable identifier for this tab.
	// Convention: lowercase snake_case (e.g. "general", "line_items", "accounting").
	// Not shown in UI — used only for stable references.
	Name string

	// Label is the human-readable tab title shown in the AMIS tab bar.
	Label string

	// Icon is the semantic icon shown in the tab bar alongside the label.
	// Use generic semantic names: "info", "list", "money", "settings".
	// Empty means no icon.
	Icon string

	// Description is optional tooltip or help text for this tab.
	// Shown by renderers that support tab tooltips. Empty means no tooltip.
	Description string

	// Permission is the permission identifier required to view this tab.
	// When set, the tab is absent (not just hidden) for viewers without
	// this permission. Empty means always visible.
	Permission string

	// Sections lists the sections rendered inside this tab's body.
	// Sections are rendered top-to-bottom in declaration order.
	Sections []SectionDef
}

// SectionDef declares a named, optionally collapsible group of fields.
// Rendered as an AMIS fieldSet control inside a form.
type SectionDef struct {
	// Name is the stable identifier for this section.
	// Convention: lowercase snake_case (e.g. "header", "totals", "address").
	Name string

	// Label is the section header text. Empty means no visible header —
	// fields appear without a surrounding fieldSet title.
	Label string

	// Icon is the semantic icon shown in the section header alongside the label.
	// Empty means no icon.
	Icon string

	// Description is optional help text shown beneath the section header.
	// Useful for complex sections that benefit from a brief explanation.
	// Empty means no description.
	Description string

	// Permission is the permission identifier required to view this section.
	// When set, the section is absent (not just hidden) for viewers without
	// this permission. Empty means always visible.
	Permission string

	// Collapsible, when true, adds a collapse toggle to the section header.
	// Users can expand or collapse the section. Requires Label to be set.
	Collapsible bool

	// Collapsed sets the initial collapsed state when Collapsible is true.
	// false (default) = expanded on load; true = collapsed on load.
	Collapsed bool

	// Columns declares the column groups within this section.
	// Each column contains a list of field names rendered left-to-right.
	//
	// When Columns is empty, the generator falls back to no columns:
	// fields are rendered in a single vertical flow.
	//
	// When Columns has one entry, fields are rendered in a single column.
	// When Columns has multiple entries, fields share horizontal space using
	// AMIS grid column classes derived from ColumnDef.Span.
	Columns []ColumnDef
}

// ColumnDef declares a vertical column of fields within a section.
// Multiple ColumnDef values in a section produce a multi-column layout.
type ColumnDef struct {
	// Span is the AMIS 12-column grid width for this column.
	// Valid values: 1–12. 0 means equal distribution among all columns.
	//
	// Examples:
	//   Two equal columns:     Span: 0 (both columns get 6/12)
	//   Two-thirds + one-third: [{Span: 8}, {Span: 4}]
	//   Three equal columns:   [{Span: 4}, {Span: 4}, {Span: 4}]
	Span int

	// Fields lists the field names (FieldDef.Name) to render in this column,
	// top-to-bottom. Fields not found in the EntitySchema are silently skipped.
	// Hidden fields and "tenant_id" are always excluded regardless of layout.
	Fields []string
}
