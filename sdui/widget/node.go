// Package widget defines the WidgetTree intermediate representation (IR) used
// by the SDUI framework.
//
// The WidgetTree sits between the SDUI generator and any renderer. The amis
// renderer converts a WidgetTree to amis JSON; future renderers may target
// React Native, PDF, or accessibility trees without touching the generator.
//
// ADR-006: The Node struct, NodeKind constants, DataSource, and ActionNode are
// public framework contracts frozen at v1.0. They must not be changed without
// an ADR revision.
//
// This package has no internal dependencies — only the standard library.
package widget

// Node is a single widget in the WidgetTree.
// Nodes form a tree via Children. Each Node has a semantic Kind that
// renderers translate to renderer-specific types.
//
// Normative requirements (ADR-006):
//   - NodeKind values MUST be one of the defined constants.
//   - Node.Hidden: true MUST NOT be used for permission-gating — absent nodes
//     are used instead (hidden elements can be revealed by client manipulation;
//     absent elements cannot).
//   - Node.Props MUST be used only for renderer-specific properties not
//     covered by the typed fields.
//   - The WidgetTree MUST be constructable without importing awo/sdui/amis.
//   - Expression fields (VisibleOn, HiddenOn, DisabledOn, RequiredOn) carry
//     portable ExpressionRef values, never renderer-specific strings.
type Node struct {
	// Kind is the semantic widget type (required).
	Kind NodeKind

	// ID is the optional stable identifier for this node.
	// Used by renderers for reference (e.g., dialog target IDs).
	ID string

	// Label is the human-readable display text.
	Label string

	// Name is the field binding name.
	// For form fields: the field name in EntityRecord.Data.
	// For lists: not used.
	Name string

	// Description is the help text shown beneath a form field.
	// Corresponds to FieldDef.Description. Empty means no help text.
	Description string

	// Placeholder is the input placeholder text shown when the field is empty.
	// Derived from FieldDef.MaxLen or FieldDef.Description by the generator.
	Placeholder string

	// Required marks a form field as required in the UI.
	// Does not duplicate server-side validation — both apply independently.
	Required bool

	// ReadOnly marks a form field as non-editable.
	ReadOnly bool

	// Hidden marks a node as absent from the rendered output.
	//
	// IMPORTANT: Do not use Hidden for permission-gating. Absent nodes (not
	// included in the tree) must be used instead. Hidden is reserved for
	// internal bookkeeping fields that are API-accessible but not shown in UI.
	Hidden bool

	// Collapsible makes a NodeSection collapsible (rendered as amis fieldSet
	// with a collapse toggle). Requires Label to be non-empty.
	Collapsible bool

	// Collapsed sets the initial collapsed state for Collapsible sections.
	// false (default) = expanded; true = collapsed on first render.
	Collapsed bool

	// ── Portable expression fields ────────────────────────────────────────────
	//
	// Expression values are portable ExpressionRef values evaluated by each
	// renderer in its own target language. The generator MUST NOT produce
	// renderer-specific expression strings (e.g., AMIS JS) — only ExpressionRef
	// values. The AMIS renderer translates ExpressionRef → JS string.
	// A nil ExpressionRef means the expression does not apply.

	// VisibleOn controls visibility. When non-nil and evaluates truthy, node is
	// shown; when falsy, hidden. nil means always visible.
	VisibleOn *ExpressionRef

	// HiddenOn hides the node when truthy. nil means never hidden.
	HiddenOn *ExpressionRef

	// DisabledOn disables a field when truthy. Takes precedence over ReadOnly.
	DisabledOn *ExpressionRef

	// RequiredOn makes a field required when truthy. Takes precedence over Required.
	RequiredOn *ExpressionRef

	// ── Layout ───────────────────────────────────────────────────────────────

	// Layout carries optional layout hints for this node.
	// Renderers interpret LayoutHint in their own layout system.
	// The generator sets semantic hints (column span, row, alignment);
	// the renderer converts them to CSS classes or native layout constraints.
	Layout *LayoutHint

	// ── Validation ───────────────────────────────────────────────────────────

	// Validation carries additional validation rules beyond Required.
	// Server-side validation is always authoritative; these rules provide
	// immediate client-side feedback.
	Validation []ValidationRule

	// ── Props (escape hatch) ──────────────────────────────────────────────────

	// Props contains renderer-specific properties not expressible in the typed
	// fields above. Props are merged last and override computed defaults.
	// Use sparingly — prefer typed fields where possible.
	Props map[string]any

	// ── Tree structure ────────────────────────────────────────────────────────

	// Children are the nested nodes (for page, form, section, tabs, table, grid).
	Children []*Node

	// DataSource configures remote data fetching for list, select, and lookup nodes.
	DataSource *DataSource

	// Actions are the action buttons associated with this node.
	Actions []*ActionNode

	// ── Extended metadata ─────────────────────────────────────────────────────

	// CurrencyField names the sibling field that holds the currency code for
	// NodeMoney nodes. Empty means use tenant default currency.
	CurrencyField string

	// MinValue and MaxValue bound numeric and date inputs.
	// Renderers convert these to native constraints (e.g., AMIS min/max).
	MinValue *float64
	MaxValue *float64

	// MaxLength caps text input length. Zero means no limit.
	MaxLength int

	// AcceptedTypes lists MIME types accepted by NodeFileUpload.
	// Example: ["image/png", "image/jpeg", "application/pdf"]
	// Empty means accept any file type.
	AcceptedTypes []string

	// MaxFileSize is the maximum allowed file size in bytes for NodeFileUpload.
	// Zero means no limit.
	MaxFileSize int64

	// MultiSelect enables multiple selection for NodeMultiSelect.
	MultiSelect bool

	// Searchable enables server-side search for NodeLookup and NodeSelect.
	Searchable bool

	// TreeParentField is the parent ID field name for NodeTreeSelect.
	// Defines the hierarchy structure in the referenced entity.
	TreeParentField string

	// StaticContent is display-only text for NodeStaticText nodes.
	StaticContent string

	// BadgeColor is the display color for NodeBadge nodes.
	// Value is a semantic color token: "success", "warning", "danger", "info", "default".
	BadgeColor string

	// WorkflowID references the workflow definition for NodeWorkflowPanel.
	WorkflowID string

	// ChartType specifies the chart variant for NodeChartPanel.
	// Values: "bar", "line", "pie", "area", "scatter".
	ChartType string

	// KPIFormat specifies the display format for NodeKPICard.
	// Values: "number", "currency", "percent", "duration".
	KPIFormat string

	// GridEditable marks a NodeGrid as editable (allows row add/edit/delete).
	GridEditable bool

	// GridMinRows is the minimum number of rows for a NodeGrid.
	GridMinRows int

	// Icon is the semantic icon name shown alongside the field label or input.
	// Renderers map this to their icon library (e.g. AMIS: fa-* prefix, Flutter: Material icons).
	// Use generic semantic names: "user", "calendar", "money", "tag", "lock", "search".
	// Empty means no icon.
	Icon string

	// ClearOn lists the sibling field names whose value change should reset this
	// field to its zero value. Used for cascading selects and dependent lookups.
	// Example: a "variant" field clears when "product" changes.
	// Renderers that support reactive field dependencies should implement this.
	ClearOn []string

	// Options lists static select options for NodeSelect and NodeMultiSelect.
	// When non-empty and DataSource is absent (or has no URL), the select
	// renders with these options inline — no server fetch required.
	// Mutually exclusive with DataSource: DataSource takes precedence when set.
	Options []StaticOption

	// FilterBar is an optional filter form node for NodeList nodes.
	// When non-nil, the renderer wires this as the list's search/filter bar.
	// Only meaningful for NodeList — ignored for all other NodeKind values.
	FilterBar *Node
}

// StaticOption is a label/value pair for a static select or multi-select field.
type StaticOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// NodeKind is the semantic widget type. Renderers map each NodeKind to a
// renderer-specific component type (e.g., amis component type strings).
type NodeKind string

const (
	// ── Structural ───────────────────────────────────────────────────────────

	// NodePage is the top-level page container.
	NodePage NodeKind = "page"

	// NodeForm is a form container. Children are the form fields.
	// DataSource provides the submit API endpoint.
	NodeForm NodeKind = "form"

	// NodeList is a paginated record list (amis crud2).
	// DataSource provides the data API endpoint. Children are column nodes.
	NodeList NodeKind = "list"

	// NodeSection is a visual grouping within a form (amis fieldSet or group).
	// Use NodeTabPane for tab pane containers — NodeSection is for form sections only.
	NodeSection NodeKind = "section"

	// NodeTabPane is a tab pane container within NodeTabs.
	// Distinct from NodeSection: NodeTabPane is always a child of NodeTabs;
	// NodeSection is always a child of NodeForm or another NodeSection.
	// This distinction eliminates the parent-inspection ambiguity.
	NodeTabPane NodeKind = "tab_pane"

	// NodeTabs is tabbed navigation. Each child NodeTabPane becomes one tab.
	NodeTabs NodeKind = "tabs"

	// NodeTable is an inline child record table.
	// DataSource provides the sub-list API endpoint.
	NodeTable NodeKind = "table"

	// NodeGrid is an editable line-item grid (e.g., invoice line items).
	// Children are column definition nodes. GridEditable controls mutability.
	// Distinct from NodeTable (read-only) and NodeList (paginated full-page crud).
	NodeGrid NodeKind = "grid"

	// NodeDialog is a modal dialog.
	NodeDialog NodeKind = "dialog"

	// ── Input fields ─────────────────────────────────────────────────────────

	// NodeField is a generic single-line text field (legacy; prefer NodeText).
	NodeField NodeKind = "field"

	// NodeText is a single-line text input (FieldTypeData).
	NodeText NodeKind = "text"

	// NodeTextArea is a multi-line text input (FieldTypeLongText, SmallText).
	NodeTextArea NodeKind = "textarea"

	// NodeRichText is a rich-text editor (multi-format: bold, lists, links).
	// Distinct from NodeTextArea (plain text only).
	NodeRichText NodeKind = "rich_text"

	// NodeNumber is a numeric input (FieldTypeInt, Float).
	NodeNumber NodeKind = "number"

	// NodeMoney is a currency amount input paired with a currency selector.
	// CurrencyField names the sibling field holding the currency code.
	NodeMoney NodeKind = "money"

	// NodeSelect is a dropdown (FieldTypeSelect).
	// DataSource provides remote options when options are server-side.
	NodeSelect NodeKind = "select"

	// NodeMultiSelect is a multi-value select (select many from a list).
	NodeMultiSelect NodeKind = "multi_select"

	// NodeLookup is an FK autocomplete with server-side search.
	// Searchable must be true. DataSource provides the search endpoint.
	NodeLookup NodeKind = "lookup"

	// NodeTreeSelect is a hierarchical select (e.g., chart of accounts, OU tree).
	// TreeParentField defines the hierarchy. DataSource provides the tree data.
	NodeTreeSelect NodeKind = "tree_select"

	// NodeDate is a date picker (FieldTypeDate). Format: YYYY-MM-DD.
	NodeDate NodeKind = "date"

	// NodeDateTime is a datetime picker (FieldTypeDateTime, FieldTypeTime).
	NodeDateTime NodeKind = "datetime"

	// NodeDuration is a duration input (HH:MM:SS). Used for HR timesheets.
	NodeDuration NodeKind = "duration"

	// NodeSwitch is a toggle switch (FieldTypeBool).
	NodeSwitch NodeKind = "switch"

	// NodeEditor is a JSON editor (FieldTypeJSON).
	NodeEditor NodeKind = "editor"

	// NodeColor is a color picker (e.g., category color coding).
	NodeColor NodeKind = "color"

	// NodeSignature is a signature capture pad.
	NodeSignature NodeKind = "signature"

	// NodeFileUpload is a file/image upload field.
	// AcceptedTypes and MaxFileSize constrain accepted uploads.
	NodeFileUpload NodeKind = "file_upload"

	// ── Display (non-input) ───────────────────────────────────────────────────

	// NodeStaticText is display-only text (label, description block, divider).
	// Not an input. StaticContent holds the display text.
	NodeStaticText NodeKind = "static_text"

	// NodeBadge is a status badge (colored pill).
	// BadgeColor carries the semantic color token.
	NodeBadge NodeKind = "badge"

	// NodeSummaryCard is the detail page header card (entity name, status, key metrics).
	NodeSummaryCard NodeKind = "summary_card"

	// NodeWorkflowPanel displays the workflow state machine UI on detail pages.
	// WorkflowID references the workflow definition.
	NodeWorkflowPanel NodeKind = "workflow_panel"

	// NodeAttachments is a file attachment list (view and upload).
	NodeAttachments NodeKind = "attachments"

	// NodeActivity is an activity/audit feed (chronological event list).
	NodeActivity NodeKind = "activity"

	// NodeRelatedList is an embedded child entity list within a detail page.
	// DataSource provides the filtered child list endpoint.
	NodeRelatedList NodeKind = "related_list"

	// ── Dashboard ─────────────────────────────────────────────────────────────

	// NodeKPICard is a dashboard KPI metric card.
	// KPIFormat specifies the display format.
	NodeKPICard NodeKind = "kpi_card"

	// NodeChartPanel is a dashboard chart panel.
	// ChartType specifies the chart variant.
	NodeChartPanel NodeKind = "chart_panel"

	// NodeTablePanel is a dashboard data table panel.
	NodeTablePanel NodeKind = "table_panel"

	// NodeFilterBar is a dashboard filter control bar.
	NodeFilterBar NodeKind = "filter_bar"

	// ── Interactive ──────────────────────────────────────────────────────────

	// NodeButton is an action trigger rendered as a button.
	NodeButton NodeKind = "button"
)

// ExpressionRef is a portable expression value.
// It wraps an ExpressionNode from the awo/sdui/expression package but is
// defined here as an opaque interface to avoid a circular dependency.
// The generator constructs ExpressionRef values via the expression package.
// Renderers receive ExpressionRef values and serialize them to target-specific
// strings (AMIS JS, Flutter condition, etc.).
//
// The underlying value must implement the ExpressionNode interface defined in
// awo/sdui/expression. Using any here keeps widget free of that import.
type ExpressionRef struct {
	// Expr is the portable expression AST node.
	// Concrete type is expression.ExpressionNode.
	Expr any
}

// LayoutHint carries semantic layout hints for a node.
// Renderers translate these to their own layout system (CSS grid, Flutter Column, etc.).
// The generator sets semantic values; renderers own the visual translation.
type LayoutHint struct {
	// ColSpan is the number of grid columns this node occupies (1–12).
	// Zero means the renderer chooses the default span.
	ColSpan int

	// ColOffset is the number of grid columns to skip before this node.
	// Zero means no offset.
	ColOffset int

	// NewRow forces this node to start on a new grid row.
	NewRow bool

	// Align is the horizontal alignment within the column.
	// Values: "left", "center", "right". Empty means renderer default.
	Align string

	// LabelWidth is the label column width in pixels (for form grids).
	// Zero means renderer default.
	LabelWidth int

	// Width is a semantic size hint for the input control width.
	// Valid values: "xs", "sm", "md", "lg", "xl", "full".
	// Empty means renderer default (typically "md").
	Width string
}

// ValidationRule is a single client-side validation constraint on a field node.
// Server-side validation is always authoritative; these rules provide immediate
// feedback before the form is submitted.
type ValidationRule struct {
	// Type is the validation kind.
	// Built-in values: "min", "max", "minLength", "maxLength",
	// "pattern", "email", "url", "integer".
	Type string

	// Value is the constraint parameter (e.g., "3" for minLength:3).
	Value string

	// Message is the error message shown when validation fails.
	// Should be locale-appropriate when set by the generator.
	Message string
}

// DataSource configures remote data fetching for NodeList, NodeSelect,
// NodeLookup, NodeTreeSelect, NodeRelatedList, and NodeGrid nodes.
type DataSource struct {
	// URL is the API endpoint.
	// May contain template variables: ${keywords}, ${page}, ${tenant_id}
	URL string

	// Method is the HTTP method (default: "GET").
	Method string

	// ReadURL is the GET endpoint used to pre-populate a form with existing
	// record data. Empty means no pre-population (create forms).
	// Distinct from URL to avoid conflating submit and load endpoints.
	ReadURL string

	// SendOn is a portable ExpressionRef for conditional data fetching.
	// nil means always fetch.
	SendOn *ExpressionRef

	// LabelField is the field name used as the display label in select options.
	LabelField string

	// ValueField is the field name used as the select option value (default: "id").
	ValueField string

	// ParentField is the parent ID field for tree-structured datasources.
	ParentField string

	// SearchParam is the query parameter name for keyword search (default: "keywords").
	SearchParam string
}

// ActionNode represents an action button associated with a Node (typically on
// list rows, detail views, or form toolbars).
type ActionNode struct {
	// ID is the stable action identifier used for deduplication and plugin targeting.
	ID string

	// Label is the button display text.
	Label string

	// ActionType is the semantic type: "submit", "dialog", "link", "ajax", "workflow".
	ActionType string

	// Level controls the visual prominence: "primary", "default", "warning", "danger".
	Level string

	// Href is the navigation target for "link" type actions.
	Href string

	// API is the endpoint for "ajax" type actions.
	API string

	// ConfirmText is the confirmation dialog message shown before execution.
	// Empty means no confirmation required.
	ConfirmText string

	// WorkflowID references the Temporal workflow to trigger for "workflow" type actions.
	WorkflowID string

	// DialogTarget is the ID of a NodeDialog node to open for "dialog" type actions.
	DialogTarget string

	// VisibleOn controls action visibility via a portable expression.
	// nil means always visible.
	VisibleOn *ExpressionRef

	// DisabledOn disables the action when the expression is truthy.
	// nil means never disabled.
	DisabledOn *ExpressionRef

	// Icon is the semantic icon name. Renderers map icon names to their icon set.
	// Example: "edit", "delete", "check", "x". Empty means no icon.
	Icon string

	// Scope controls where this action appears in the rendered UI.
	// "toolbar" — appears in the list header toolbar (default when empty).
	// "row"     — appears as a per-row operation button in list views.
	// "bulk"    — appears in the bulk-action bar (applies to selected rows).
	// The renderer is responsible for routing actions to their correct slot.
	Scope string
}
