// Package generator transforms entity metadata into a Widget IR tree.
//
// The generator is the second stage of the SDUI pipeline:
//
//	CompiledSchema + GeneratorContext → widget.Node tree
//
// The generator is entirely renderer-independent. It produces widget.Node
// values with semantic fields (ExpressionRef, LayoutHint, ActionNode) and
// never emits renderer-specific constructs (no AMIS types, no CSS classes,
// no JavaScript strings). The renderer receives the Node tree and translates
// it to target-specific output.
//
// # Architecture
//
// A single EntityGenerator handles all view modes (list, create, edit, detail,
// dashboard) via specialised internal builders. There is no separate ListGenerator
// or FormGenerator — the view mode is carried in GeneratorContext and selects
// which pipeline stages run.
//
// # Pipeline stages
//
//  1. Pre-generation plugin transforms (ExtPreGeneration)
//  2. Permission gating — fields without viewer permission are absent (not hidden)
//  3. View mode selection — which fields appear in list vs form vs detail
//  4. Node construction — build the widget tree from field definitions
//  5. Post-generation plugin transforms (ExtPostGeneration)
//  6. Return root node
//
// # Concurrency
//
// EntityGenerator is safe for concurrent use. GeneratorContext is a value type
// and all state is local to each Generate() call.
package generator

import (
	"fmt"

	"awo.so/awo/sdui/plugins"
	"awo.so/awo/sdui/sduictx"
	"awo.so/awo/sdui/widget"
)

// FieldDef is the generator's view of a single entity field.
// It carries the semantic metadata needed to construct a widget.Node.
// This is a simplified projection of awo/def.FieldDef — the generator does
// not import awo/def directly to preserve the dependency direction.
type FieldDef struct {
	// Name is the field binding name (e.g., "invoice_number", "status").
	Name string

	// Label is the human-readable display name.
	Label string

	// FieldType identifies the field's data type.
	// Values correspond to def.FieldType constants (passed as strings to avoid import).
	FieldType string

	// Required marks this field as required in create/edit forms.
	Required bool

	// ReadOnly marks this field as read-only in all forms.
	ReadOnly bool

	// Hidden omits this field from all views.
	Hidden bool

	// InList includes this field as a column in list view.
	InList bool

	// InForm includes this field in create/edit forms.
	InForm bool

	// InDetail includes this field in detail view.
	InDetail bool

	// Description is the help text shown beneath form fields.
	Description string

	// Placeholder is the input placeholder text.
	Placeholder string

	// MaxLength caps text input length. Zero means no limit.
	MaxLength int

	// Options lists select field options (label, value pairs).
	// Used when the select is static (not server-side).
	Options []SelectOption

	// DataSource configures remote data fetching (list, select, lookup).
	DataSource *widget.DataSource

	// LinkedEntity is the entity name this field links to (for FieldTypeLink/Lookup).
	LinkedEntity string

	// Permission is the permission identifier required to view this field.
	// Empty means no permission required (field always visible to authenticated users).
	Permission string

	// ColSpan is the grid column span for this field in form layout (1–12).
	ColSpan int

	// Section groups this field into a named form section.
	Section string

	// Tab groups this field into a named tab (within tabs layout).
	Tab string

	// CurrencyField is the sibling field name for money fields.
	CurrencyField string

	// Searchable marks this field as participating in list view search/filter.
	// When true, the generator includes this field in the filter bar.
	Searchable bool

	// Icon is the semantic icon name shown alongside the field input.
	// Renderers map this to their icon library.
	Icon string

	// Width is a semantic size hint for the input control.
	// Valid values: "xs", "sm", "md", "lg", "xl", "full". Empty = renderer default.
	Width string

	// Computed marks this field as server-computed: rendered read-only and
	// automatically refreshes when dependent fields change.
	Computed bool

	// ClearOn lists field names whose value change causes this field to reset.
	// Used for cascading selects and dependent lookups.
	ClearOn []string

	// VisibleOn is a raw expression string controlling visibility.
	// When non-empty, the renderer evaluates this as a condition.
	VisibleOn string

	// HiddenOn hides the field when the expression is truthy.
	HiddenOn string

	// DisabledOn disables the field when the expression is truthy.
	DisabledOn string

	// RequiredOn makes the field required when the expression is truthy.
	RequiredOn string
}

// SelectOption is a single option for a static select field.
type SelectOption struct {
	Label string
	Value string
}

// SectionDef describes a form section grouping.
type SectionDef struct {
	// ID is the stable section identifier.
	ID string

	// Title is the section display title. Empty = unlabeled group.
	Title string

	// Icon is the semantic icon shown in the section header.
	Icon string

	// Description is optional help text shown beneath the section header.
	Description string

	// Collapsible makes the section collapsible.
	Collapsible bool

	// Collapsed sets the initial collapsed state.
	Collapsed bool

	// Fields lists the field names in this section (in display order).
	Fields []string

	// Columns is the grid column count for this section (1–3 typical).
	// Fields are distributed across columns in order.
	Columns int

	// Permission is the permission identifier required to view this section.
	Permission string
}

// TabDef describes a tab grouping.
type TabDef struct {
	// ID is the stable tab identifier.
	ID string

	// Title is the tab display title.
	Title string

	// Icon is the semantic icon shown in the tab bar alongside the title.
	Icon string

	// Description is optional tooltip text for the tab.
	Description string

	// Sections lists the section IDs in this tab (in display order).
	Sections []string

	// Permission is the permission identifier required to view this tab.
	Permission string
}

// EntitySchema is the generator's complete view of an entity.
// It carries all metadata needed to produce widget trees for all view modes.
// This is constructed from awo/def.CompiledSchema by the SDUI handler.
type EntitySchema struct {
	// Name is the entity name (e.g., "finance_invoice").
	Name string

	// Title is the human-readable entity title (e.g., "Invoice").
	Title string

	// PluralTitle is the plural form (e.g., "Invoices").
	PluralTitle string

	// Fields are all entity fields in declaration order.
	Fields []FieldDef

	// Sections defines form section groupings. May be nil (flat form layout).
	Sections []SectionDef

	// Tabs defines tab groupings. May be nil (no tabs).
	Tabs []TabDef

	// ListURL is the API endpoint for list view (paginated entity list).
	ListURL string

	// CreateURL is the API endpoint for creating a new entity.
	CreateURL string

	// EditURL is the API endpoint for updating an existing entity.
	// May contain path parameters: "/api/v1/finance/invoices/{id}".
	EditURL string

	// DetailURL is the API endpoint for fetching a single entity.
	DetailURL string

	// Permissions carries the permission identifiers for entity-level access.
	// Keys: "create", "read", "update", "delete". Values: permission ID strings.
	Permissions map[string]string

	// Actions are entity-level actions (submit, cancel, approve, etc.).
	Actions []ActionDef

	// Icon is the semantic icon name for this entity.
	// Used in navigation menus and list headers.
	Icon string

	// Relations are the related entity edges for this entity.
	// The generator emits NodeRelatedList or NodeGrid nodes for each non-hidden
	// relation in detail and form views.
	Relations []RelationDef

	// DashboardPanels carries the panels to render in ViewModeDashboard.
	// Populated by the adapt layer from dashboard.Registry entries that match
	// this entity's module. Empty means the dashboard page is a placeholder.
	DashboardPanels []DashboardPanel

	// UIPrefix is the web UI path prefix for this entity.
	// Format: "/ui/{module}/{resource}" — e.g. "/ui/finance/invoices".
	// Used to construct create/edit/detail navigation hrefs in action buttons.
	UIPrefix string

	// HasWorkflow is true when the entity has at least one WorkflowTrigger.
	// The generator emits a NodeWorkflowPanel in detail view when true.
	// The workflow state data source URL is expected to be at {DetailURL}/workflow-state.
	HasWorkflow bool
}

// DashboardPanel carries metadata for one panel on a dashboard page.
// Derived from dashboard.PanelDef by the adapt layer; the generator converts
// each DashboardPanel into the appropriate NodeKind (NodeKPICard, NodeChartPanel,
// NodeTablePanel, NodeFilterBar).
type DashboardPanel struct {
	// ID is the stable panel identifier.
	ID string

	// Title is the panel display title.
	Title string

	// PanelType is the semantic panel type: "kpi", "chart", "table", "filter".
	PanelType string

	// DataURL is the API endpoint for this panel's data.
	DataURL string

	// ChartType specifies the chart variant for chart panels.
	// Values: "bar", "line", "pie", "area", "scatter".
	ChartType string

	// KPIFormat specifies the display format for KPI panels.
	// Values: "number", "currency", "percent", "duration".
	KPIFormat string

	// ValueField is the data field name for KPI panels.
	ValueField string

	// ColSpan is the number of grid columns this panel occupies.
	// Zero means the renderer chooses the default.
	ColSpan int

	// Permissions lists permission identifiers required to view this panel.
	// Empty means visible to all dashboard viewers.
	Permissions []string
}

// RelationDef describes a related entity edge for SDUI rendering.
// Derived from def.EdgeDef at compile time by the adapt layer.
// The generator uses RelationDef to emit NodeRelatedList and NodeGrid nodes
// in detail and edit views.
type RelationDef struct {
	// Name is the stable edge identifier.
	Name string

	// Label is the human-readable section title for this relation.
	Label string

	// RelationType is the edge cardinality: "one_to_many", "many_to_many", "one_to_one".
	RelationType string

	// TargetEntity is the qualified name of the related entity.
	// Example: "finance_invoice_line"
	TargetEntity string

	// DataURL is the API endpoint that returns related records.
	// Format: /api/v1/{module}/{resource}?{foreign_key}=${id}
	DataURL string

	// ForeignKey is the column on the target entity that references the parent.
	// Used to construct DataURL filter parameters.
	ForeignKey string

	// Inline indicates that this relation should render as an editable inline
	// grid (NodeGrid) rather than a read-only related list (NodeRelatedList).
	// Set for child tables like invoice line items.
	Inline bool

	// Hidden omits this relation from all SDUI views.
	Hidden bool
}

// ActionDef describes an entity-level action button.
type ActionDef struct {
	// ID is the stable action identifier.
	ID string

	// Label is the button display text.
	Label string

	// ActionType is the semantic type: "submit", "dialog", "link", "ajax", "workflow".
	ActionType string

	// Level controls visual prominence: "primary", "default", "warning", "danger".
	Level string

	// Permission is the permission identifier required to see this action.
	// Empty means visible to all viewers with entity read access.
	Permission string

	// ConfirmText is the confirmation prompt shown before executing the action.
	ConfirmText string

	// ViewModes lists which view modes show this action.
	// Nil means all view modes.
	ViewModes []sduictx.ViewMode

	// WorkflowID references a Temporal workflow for "workflow" type actions.
	WorkflowID string

	// Icon is the semantic icon name.
	Icon string
}

// EntityGenerator transforms EntitySchema + GeneratorContext into a widget.Node tree.
// Safe for concurrent use.
type EntityGenerator struct {
	pipeline *plugins.Pipeline // may be nil (uses global pipeline)
}

// New returns a production EntityGenerator using the global plugin pipeline.
func New() *EntityGenerator {
	return &EntityGenerator{pipeline: plugins.GlobalPipeline()}
}

// NewWithPipeline returns an EntityGenerator using the provided pipeline.
// Use in tests to inject an isolated pipeline.
func NewWithPipeline(p *plugins.Pipeline) *EntityGenerator {
	return &EntityGenerator{pipeline: p}
}

// Generate produces a widget.Node tree for the given entity schema and context.
// Returns an error if any pipeline stage fails fatally.
func (g *EntityGenerator) Generate(schema EntitySchema, ctx sduictx.GeneratorContext) (*widget.Node, error) {
	if err := ctx.Validate(); err != nil {
		return nil, fmt.Errorf("generator.Generate: %w", err)
	}

	// Stage 1: Pre-generation plugin transforms.
	// (Currently a no-op for tree transforms — pre-generation operates on schema,
	// which would require schema serialization. Deferred to post-v1.0.)

	// Stage 2–4: Build the widget tree based on view mode.
	var root *widget.Node
	var err error

	switch ctx.ViewMode {
	case sduictx.ViewModeList:
		root, err = g.buildList(schema, ctx)
	case sduictx.ViewModeCreate:
		root, err = g.buildForm(schema, ctx, false)
	case sduictx.ViewModeEdit:
		root, err = g.buildForm(schema, ctx, false)
	case sduictx.ViewModeDetail:
		root, err = g.buildDetail(schema, ctx)
	case sduictx.ViewModeDashboard:
		root, err = g.buildDashboard(schema, ctx)
	default:
		return nil, fmt.Errorf("generator.Generate: unknown ViewMode %q", ctx.ViewMode)
	}
	if err != nil {
		return nil, err
	}

	// Stage 5: Post-generation plugin transforms.
	if g.pipeline != nil {
		root, err = g.pipeline.RunTreeTransforms(plugins.ExtPostGeneration, root, ctx)
		if err != nil {
			return nil, fmt.Errorf("generator.Generate: post-generation plugin: %w", err)
		}
	}

	return root, nil
}

// ── View mode builders ────────────────────────────────────────────────────────

// buildList constructs a NodeList tree for paginated list view.
func (g *EntityGenerator) buildList(schema EntitySchema, ctx sduictx.GeneratorContext) (*widget.Node, error) {
	// Collect visible column fields (InList=true, permission gated).
	var columns []*widget.Node
	for _, f := range schema.Fields {
		if !f.InList || f.Hidden {
			continue
		}
		if !g.canViewField(f, ctx) {
			continue // absent, not hidden
		}
		col := g.buildColumnNode(f, ctx)
		col, err := g.runFieldOverrides(f.Name, col, ctx)
		if err != nil {
			return nil, err
		}
		columns = append(columns, col)
	}

	// Build list actions (Create button, row actions, etc.).
	actions := g.buildListActions(schema, ctx)

	// Filter bar — embedded in the list node so the renderer can wire it as
	// the crud filter prop. Nil when no fields are searchable.
	filterBar := g.buildFilterBar(schema, ctx)

	listNode := &widget.Node{
		Kind:      widget.NodeList,
		Label:     schema.PluralTitle,
		Children:  columns,
		Actions:   actions,
		FilterBar: filterBar,
		DataSource: &widget.DataSource{
			URL:    schema.ListURL,
			Method: "GET",
		},
	}

	return &widget.Node{
		Kind:     widget.NodePage,
		Label:    schema.PluralTitle,
		Children: []*widget.Node{listNode},
	}, nil
}

// buildFilterBar constructs a NodeFilterBar with filter fields for the list view.
// Returns nil when no fields have Searchable=true — no filter bar is emitted.
func (g *EntityGenerator) buildFilterBar(schema EntitySchema, ctx sduictx.GeneratorContext) *widget.Node {
	var filterFields []*widget.Node
	for _, f := range schema.Fields {
		if !f.InList || f.Hidden || !f.Searchable {
			continue
		}
		if !g.canViewField(f, ctx) {
			continue
		}
		node := g.buildFieldNode(f, ctx, false)
		// Override Name to match backend filter format: filter[field][op]=value.
		// The backend filterparse package reads ?filter[field][operator]=value.
		//
		// Date/DateTime: use AMIS input-date-range which posts startName and endName
		// independently, giving the backend gte + lte bounds from a single picker.
		// Non-date types: eq for exact-match types, contains for free-text.
		switch node.Kind {
		case widget.NodeText, widget.NodeTextArea:
			node.Name = "filter[" + f.Name + "][contains]"
		case widget.NodeDate, widget.NodeDateTime:
			// Date range: AMIS input-date-range posts two params independently.
			// node.Name is a placeholder (input-date-range ignores the name prop).
			node.Name = f.Name + "_range"
			if node.Props == nil {
				node.Props = make(map[string]any)
			}
			node.Props["type"] = "input-date-range"
			node.Props["startName"] = "filter[" + f.Name + "][gte]"
			node.Props["endName"] = "filter[" + f.Name + "][lte]"
			node.Props["format"] = "YYYY-MM-DD"
			node.Props["inputFormat"] = "YYYY-MM-DD"
		default:
			node.Name = "filter[" + f.Name + "][eq]"
		}
		filterFields = append(filterFields, node)
	}
	if len(filterFields) == 0 {
		return nil
	}
	return &widget.Node{
		Kind:     widget.NodeFilterBar,
		Children: filterFields,
	}
}

// buildForm constructs a NodeForm tree for create or edit view.
// isEdit: true = edit mode (initApi populated); false = create mode.
func (g *EntityGenerator) buildForm(schema EntitySchema, ctx sduictx.GeneratorContext, isEdit bool) (*widget.Node, error) {
	readOnly := ctx.ReadOnly

	var formBody []*widget.Node

	if len(schema.Tabs) > 0 {
		// Tabs layout: NodeTabs with NodeTabPane children.
		tabs, err := g.buildFormTabs(schema, ctx, readOnly)
		if err != nil {
			return nil, err
		}
		formBody = append(formBody, tabs)
	} else if len(schema.Sections) > 0 {
		// Sections layout: NodeSection children.
		sections, err := g.buildFormSections(schema, ctx, readOnly)
		if err != nil {
			return nil, err
		}
		formBody = append(formBody, sections...)
	} else {
		// Flat layout: all fields directly in the form body.
		fields, err := g.buildFlatFields(schema, ctx, readOnly)
		if err != nil {
			return nil, err
		}
		formBody = append(formBody, fields...)
	}

	// Inline grids from edge definitions (Inline=true relations).
	for _, rel := range schema.Relations {
		if rel.Hidden || !rel.Inline {
			continue
		}
		gridNode := &widget.Node{
			Kind:         widget.NodeGrid,
			Name:         rel.Name,
			Label:        rel.Label,
			GridEditable: !readOnly,
		}
		if rel.DataURL != "" {
			gridNode.DataSource = &widget.DataSource{
				URL:    rel.DataURL,
				Method: "GET",
			}
		}
		formBody = append(formBody, gridNode)
	}

	// Build form actions.
	actions := g.buildFormActions(schema, ctx)

	ds := &widget.DataSource{
		URL:    schema.CreateURL,
		Method: "POST",
	}
	if isEdit || ctx.ViewMode == sduictx.ViewModeEdit {
		ds.URL = schema.EditURL
		ds.Method = "PATCH"
		if schema.DetailURL != "" {
			ds.ReadURL = schema.DetailURL
		}
	}

	form := &widget.Node{
		Kind:       widget.NodeForm,
		Label:      schema.Title,
		Children:   formBody,
		Actions:    actions,
		DataSource: ds,
	}

	return &widget.Node{
		Kind:     widget.NodePage,
		Label:    schema.Title,
		Children: []*widget.Node{form},
	}, nil
}

// buildDetail constructs a read-only detail page.
func (g *EntityGenerator) buildDetail(schema EntitySchema, ctx sduictx.GeneratorContext) (*widget.Node, error) {
	// Detail page is a read-only form with a summary card at the top.
	detailCtx := ctx.WithReadOnly()

	var pageChildren []*widget.Node

	// Summary card (entity header).
	summaryCard := &widget.Node{
		Kind:    widget.NodeSummaryCard,
		Label:   schema.Title,
		Actions: g.buildDetailActions(schema, ctx),
	}
	pageChildren = append(pageChildren, summaryCard)

	// Workflow state panel — auto-emitted when entity has workflow triggers.
	if schema.HasWorkflow {
		pageChildren = append(pageChildren, &widget.Node{
			Kind:       widget.NodeWorkflowPanel,
			WorkflowID: schema.Name,
			DataSource: &widget.DataSource{
				URL:    schema.DetailURL + "/workflow-state",
				Method: "GET",
			},
		})
	}

	// Detail form body (read-only fields in sections/tabs).
	var formBody []*widget.Node
	if len(schema.Tabs) > 0 {
		tabs, err := g.buildFormTabs(schema, detailCtx, true)
		if err != nil {
			return nil, err
		}
		formBody = append(formBody, tabs)
	} else if len(schema.Sections) > 0 {
		sections, err := g.buildFormSections(schema, detailCtx, true)
		if err != nil {
			return nil, err
		}
		formBody = append(formBody, sections...)
	} else {
		fields, err := g.buildFlatFields(schema, detailCtx, true)
		if err != nil {
			return nil, err
		}
		formBody = append(formBody, fields...)
	}

	// Detail form: read-only, no submit action. Use ReadURL (→ initApi) so
	// AMIS loads the record on mount. Using URL (→ api) would only fire on
	// form submission, leaving all fields empty on initial render.
	detailForm := &widget.Node{
		Kind:     widget.NodeForm,
		Children: formBody,
		DataSource: &widget.DataSource{
			ReadURL: schema.DetailURL,
			// Method intentionally omitted — initApi is always GET.
		},
	}
	pageChildren = append(pageChildren, detailForm)

	// Related lists from edge definitions.
	for _, rel := range schema.Relations {
		if rel.Hidden {
			continue
		}
		relNode := &widget.Node{
			Kind:  widget.NodeRelatedList,
			Name:  rel.Name,
			Label: rel.Label,
		}
		if rel.DataURL != "" {
			relNode.DataSource = &widget.DataSource{
				URL:    rel.DataURL,
				Method: "GET",
			}
		}
		pageChildren = append(pageChildren, relNode)
	}

	return &widget.Node{
		Kind:     widget.NodePage,
		Label:    schema.Title,
		Children: pageChildren,
	}, nil
}

// buildDashboard constructs a dashboard page from schema.DashboardPanels.
// Panels are populated by the adapt layer from dashboard.Registry. When
// DashboardPanels is empty (no registered dashboard for this entity), the
// page is returned as a placeholder with no body.
func (g *EntityGenerator) buildDashboard(schema EntitySchema, ctx sduictx.GeneratorContext) (*widget.Node, error) {
	var panels []*widget.Node
	for _, p := range schema.DashboardPanels {
		// Permission gate: skip panels the viewer cannot see.
		visible := true
		for _, perm := range p.Permissions {
			if !ctx.Viewer.HasPermission(perm) {
				visible = false
				break
			}
		}
		if !visible {
			continue
		}

		var panelNode *widget.Node
		switch p.PanelType {
		case "kpi":
			panelNode = &widget.Node{
				Kind:      widget.NodeKPICard,
				ID:        p.ID,
				Label:     p.Title,
				Name:      p.ValueField,
				KPIFormat: p.KPIFormat,
			}
		case "chart":
			panelNode = &widget.Node{
				Kind:      widget.NodeChartPanel,
				ID:        p.ID,
				Label:     p.Title,
				ChartType: p.ChartType,
			}
		case "table":
			panelNode = &widget.Node{
				Kind:  widget.NodeTablePanel,
				ID:    p.ID,
				Label: p.Title,
			}
		case "filter":
			panelNode = &widget.Node{
				Kind:  widget.NodeFilterBar,
				ID:    p.ID,
				Label: p.Title,
			}
		default:
			continue // unknown panel type — skip
		}

		if p.DataURL != "" {
			panelNode.DataSource = &widget.DataSource{URL: p.DataURL, Method: "GET"}
		}
		if p.ColSpan > 0 {
			panelNode.Layout = &widget.LayoutHint{ColSpan: p.ColSpan}
		}
		panels = append(panels, panelNode)
	}

	return &widget.Node{
		Kind:     widget.NodePage,
		Label:    schema.Title,
		Children: panels,
	}, nil
}

// ── Section and tab builders ──────────────────────────────────────────────────

func (g *EntityGenerator) buildFormTabs(schema EntitySchema, ctx sduictx.GeneratorContext, readOnly bool) (*widget.Node, error) {
	var tabPanes []*widget.Node
	// Index fields by section for fast lookup.
	fieldsBySectionID := indexFieldsBySection(schema)

	for _, tab := range schema.Tabs {
		if tab.Permission != "" && !ctx.Viewer.HasPermission(tab.Permission) {
			continue // absent
		}
		var tabBody []*widget.Node
		for _, sectionID := range tab.Sections {
			section := findSection(schema.Sections, sectionID)
			if section == nil {
				continue
			}
			sec, err := g.buildSection(*section, fieldsBySectionID[sectionID], schema, ctx, readOnly)
			if err != nil {
				return nil, err
			}
			if sec != nil {
				tabBody = append(tabBody, sec)
			}
		}
		tabPanes = append(tabPanes, &widget.Node{
			Kind:     widget.NodeTabPane,
			ID:       tab.ID,
			Label:    tab.Title,
			Children: tabBody,
		})
	}
	return &widget.Node{
		Kind:     widget.NodeTabs,
		Children: tabPanes,
	}, nil
}

func (g *EntityGenerator) buildFormSections(schema EntitySchema, ctx sduictx.GeneratorContext, readOnly bool) ([]*widget.Node, error) {
	fieldsBySectionID := indexFieldsBySection(schema)
	var out []*widget.Node
	for _, section := range schema.Sections {
		sec, err := g.buildSection(section, fieldsBySectionID[section.ID], schema, ctx, readOnly)
		if err != nil {
			return nil, err
		}
		if sec != nil {
			out = append(out, sec)
		}
	}
	return out, nil
}

func (g *EntityGenerator) buildSection(section SectionDef, fields []FieldDef, schema EntitySchema, ctx sduictx.GeneratorContext, readOnly bool) (*widget.Node, error) {
	if section.Permission != "" && !ctx.Viewer.HasPermission(section.Permission) {
		return nil, nil // section absent
	}
	var children []*widget.Node
	for _, f := range fields {
		if f.Hidden || !f.InForm {
			continue
		}
		if !g.canViewField(f, ctx) {
			continue
		}
		node := g.buildFieldNode(f, ctx, readOnly)
		node, err := g.runFieldOverrides(f.Name, node, ctx)
		if err != nil {
			return nil, err
		}
		children = append(children, node)
	}
	if len(children) == 0 {
		return nil, nil
	}
	return &widget.Node{
		Kind:        widget.NodeSection,
		ID:          section.ID,
		Label:       section.Title,
		Collapsible: section.Collapsible,
		Collapsed:   section.Collapsed,
		Children:    children,
	}, nil
}

func (g *EntityGenerator) buildFlatFields(schema EntitySchema, ctx sduictx.GeneratorContext, readOnly bool) ([]*widget.Node, error) {
	var out []*widget.Node
	for _, f := range schema.Fields {
		if f.Hidden || !f.InForm {
			continue
		}
		if !g.canViewField(f, ctx) {
			continue
		}
		node := g.buildFieldNode(f, ctx, readOnly)
		node, err := g.runFieldOverrides(f.Name, node, ctx)
		if err != nil {
			return nil, err
		}
		out = append(out, node)
	}
	return out, nil
}

// ── Node builders ─────────────────────────────────────────────────────────────

// buildFieldNode creates a widget.Node for a single field.
// The node Kind is chosen based on FieldType.
func (g *EntityGenerator) buildFieldNode(f FieldDef, ctx sduictx.GeneratorContext, readOnly bool) *widget.Node {
	kind := fieldTypeToNodeKind(f.FieldType, f)
	node := &widget.Node{
		Kind:          kind,
		Name:          f.Name,
		Label:         f.Label,
		Description:   f.Description,
		Placeholder:   f.Placeholder,
		Required:      f.Required,
		ReadOnly:      readOnly || f.ReadOnly || f.Computed,
		MaxLength:     f.MaxLength,
		DataSource:    f.DataSource,
		CurrencyField: f.CurrencyField,
		Icon:          f.Icon,
		ClearOn:       f.ClearOn,
	}

	// Layout hint: ColSpan and/or Width.
	if f.ColSpan > 0 || f.Width != "" {
		node.Layout = &widget.LayoutHint{ColSpan: f.ColSpan, Width: f.Width}
	}

	// Static select options.
	for _, opt := range f.Options {
		node.Options = append(node.Options, widget.StaticOption{
			Label: opt.Label,
			Value: opt.Value,
		})
	}

	// Raw expression strings from def.FieldDef (AMIS JS).
	// Stored in ExpressionRef.Expr as plain strings. Renderers type-switch:
	//   - string → raw pass-through (AMIS renderer emits unchanged)
	//   - expression.ExpressionNode → serialize via renderer's expression serializer
	if f.VisibleOn != "" {
		node.VisibleOn = &widget.ExpressionRef{Expr: f.VisibleOn}
	}
	if f.HiddenOn != "" {
		node.HiddenOn = &widget.ExpressionRef{Expr: f.HiddenOn}
	}
	if f.DisabledOn != "" {
		node.DisabledOn = &widget.ExpressionRef{Expr: f.DisabledOn}
	}
	if f.RequiredOn != "" {
		node.RequiredOn = &widget.ExpressionRef{Expr: f.RequiredOn}
	}

	return node
}

// buildColumnNode creates a widget.Node suitable for use as a list column.
// Column nodes are lightweight — name and label only, plus type for column rendering.
func (g *EntityGenerator) buildColumnNode(f FieldDef, ctx sduictx.GeneratorContext) *widget.Node {
	kind := fieldTypeToColumnNodeKind(f.FieldType, f)
	return &widget.Node{
		Kind:  kind,
		Name:  f.Name,
		Label: f.Label,
	}
}

// ── Action builders ───────────────────────────────────────────────────────────

func (g *EntityGenerator) buildListActions(schema EntitySchema, ctx sduictx.GeneratorContext) []*widget.ActionNode {
	var out []*widget.ActionNode

	// ── Toolbar actions (Scope: "toolbar") ────────────────────────────────────

	// Create button — requires create permission.
	if perm := schema.Permissions["create"]; perm == "" || ctx.Viewer.HasPermission(perm) {
		href := schema.UIPrefix + "/create"
		if href == "/create" {
			href = "/" + schema.Name + "/create" // fallback if UIPrefix not set
		}
		out = append(out, &widget.ActionNode{
			ID:         "create",
			Label:      "New " + schema.Title,
			ActionType: "link",
			Level:      "primary",
			Href:       href,
			Icon:       "plus",
			Scope:      "toolbar",
		})
	}

	// Entity-level list toolbar actions.
	for _, a := range schema.Actions {
		if !actionInViewMode(a, sduictx.ViewModeList) {
			continue
		}
		if a.Permission != "" && !ctx.Viewer.HasPermission(a.Permission) {
			continue
		}
		out = append(out, &widget.ActionNode{
			ID:          a.ID,
			Label:       a.Label,
			ActionType:  a.ActionType,
			Level:       a.Level,
			ConfirmText: a.ConfirmText,
			WorkflowID:  a.WorkflowID,
			Icon:        a.Icon,
			API:         listActionAPI(schema, a),
			Scope:       "toolbar",
		})
	}

	// ── Row actions (Scope: "row") ─────────────────────────────────────────────
	// These appear in the per-row operation column. Permission-gated so the UI
	// matches the backend enforcement. The backend still enforces permissions
	// independently — this is defence-in-depth, not the sole gate.

	// View — always shown when viewer has read access (list already requires read).
	if schema.UIPrefix != "" {
		out = append(out, &widget.ActionNode{
			ID:         "view",
			Label:      "View",
			ActionType: "link",
			Level:      "link",
			Href:       schema.UIPrefix + "/${id}",
			Icon:       "eye",
			Scope:      "row",
		})
	}

	// Edit — requires update permission.
	if perm := schema.Permissions["update"]; perm == "" || ctx.Viewer.HasPermission(perm) {
		if schema.UIPrefix != "" {
			out = append(out, &widget.ActionNode{
				ID:         "edit",
				Label:      "Edit",
				ActionType: "link",
				Level:      "link",
				Href:       schema.UIPrefix + "/${id}/edit",
				Icon:       "edit",
				Scope:      "row",
			})
		}
	}

	// Delete — requires delete permission.
	if perm := schema.Permissions["delete"]; perm == "" || ctx.Viewer.HasPermission(perm) {
		out = append(out, &widget.ActionNode{
			ID:          "delete",
			Label:       "Delete",
			ActionType:  "ajax",
			Level:       "link",
			API:         "DELETE:" + schema.ListURL + "/${id}",
			ConfirmText: "Delete this " + schema.Title + "?",
			Icon:        "trash",
			Scope:       "row",
		})
	}

	// ── Bulk actions (Scope: "bulk") ───────────────────────────────────────────

	// Bulk delete — requires delete permission.
	if perm := schema.Permissions["delete"]; perm == "" || ctx.Viewer.HasPermission(perm) {
		out = append(out, &widget.ActionNode{
			ID:          "bulk-delete",
			Label:       "Delete selected",
			ActionType:  "ajax",
			Level:       "danger",
			API:         "DELETE:" + schema.ListURL,
			ConfirmText: "Delete all selected records? This cannot be undone.",
			Icon:        "trash",
			Scope:       "bulk",
		})
	}

	return out
}

// listActionAPI constructs the AMIS api string for a toolbar action in list view.
// Custom list actions POST to /{listURL}/actions/{actionID}.
func listActionAPI(schema EntitySchema, a ActionDef) string {
	if a.ActionType != "ajax" {
		return ""
	}
	return "POST:" + schema.ListURL + "/actions/" + a.ID
}

func (g *EntityGenerator) buildFormActions(schema EntitySchema, ctx sduictx.GeneratorContext) []*widget.ActionNode {
	var out []*widget.ActionNode
	viewMode := ctx.ViewMode

	// Submit button for create/edit.
	if viewMode == sduictx.ViewModeCreate || viewMode == sduictx.ViewModeEdit {
		if !ctx.ReadOnly {
			out = append(out, &widget.ActionNode{
				ID:         "submit",
				Label:      "Save",
				ActionType: "submit",
				Level:      "primary",
			})
		}
	}

	// Entity-level actions scoped to form view.
	for _, a := range schema.Actions {
		if !actionInViewMode(a, viewMode) {
			continue
		}
		if a.Permission != "" && !ctx.Viewer.HasPermission(a.Permission) {
			continue
		}
		out = append(out, &widget.ActionNode{
			ID:          a.ID,
			Label:       a.Label,
			ActionType:  a.ActionType,
			Level:       a.Level,
			ConfirmText: a.ConfirmText,
			WorkflowID:  a.WorkflowID,
			Icon:        a.Icon,
		})
	}
	return out
}

func (g *EntityGenerator) buildDetailActions(schema EntitySchema, ctx sduictx.GeneratorContext) []*widget.ActionNode {
	var out []*widget.ActionNode

	// Edit button — requires update permission.
	if perm := schema.Permissions["update"]; perm == "" || ctx.Viewer.HasPermission(perm) {
		editHref := schema.UIPrefix + "/${id}/edit"
		if schema.UIPrefix == "" {
			editHref = "/" + schema.Name + "/${id}/edit"
		}
		out = append(out, &widget.ActionNode{
			ID:         "edit",
			Label:      "Edit",
			ActionType: "link",
			Level:      "default",
			Href:       editHref,
			Icon:       "edit",
		})
	}

	// Delete button — requires delete permission.
	// API: DELETE /api/v1/{module}/{resource}/${id}  (EditURL == ListURL + "/${id}")
	if perm := schema.Permissions["delete"]; perm == "" || ctx.Viewer.HasPermission(perm) {
		deleteAPI := "DELETE:" + schema.EditURL
		if schema.EditURL == "" {
			deleteAPI = "DELETE:" + schema.ListURL + "/${id}"
		}
		out = append(out, &widget.ActionNode{
			ID:          "delete",
			Label:       "Delete",
			ActionType:  "ajax",
			Level:       "danger",
			API:         deleteAPI,
			ConfirmText: "Delete this " + schema.Title + "?",
			Icon:        "trash",
		})
	}

	// Workflow and custom actions for detail view.
	for _, a := range schema.Actions {
		if !actionInViewMode(a, sduictx.ViewModeDetail) {
			continue
		}
		if a.Permission != "" && !ctx.Viewer.HasPermission(a.Permission) {
			continue
		}
		// Ajax actions fire POST /api/v1/{module}/{resource}/{id}/actions/{actionID}.
		api := ""
		if a.ActionType == "ajax" {
			api = "POST:" + schema.ListURL + "/${id}/actions/" + a.ID
		}
		out = append(out, &widget.ActionNode{
			ID:          a.ID,
			Label:       a.Label,
			ActionType:  a.ActionType,
			Level:       a.Level,
			ConfirmText: a.ConfirmText,
			WorkflowID:  a.WorkflowID,
			Icon:        a.Icon,
			API:         api,
		})
	}
	return out
}

// ── Permission gating ─────────────────────────────────────────────────────────

// canViewField reports whether the viewer has permission to see this field.
// Platform admins bypass all permission checks.
func (g *EntityGenerator) canViewField(f FieldDef, ctx sduictx.GeneratorContext) bool {
	if ctx.Viewer.IsPlatformAdmin() {
		return true
	}
	if f.Permission == "" {
		return true
	}
	return ctx.Viewer.HasPermission(f.Permission)
}

// ── Plugin integration ────────────────────────────────────────────────────────

func (g *EntityGenerator) runFieldOverrides(fieldName string, node *widget.Node, ctx sduictx.GeneratorContext) (*widget.Node, error) {
	if g.pipeline == nil {
		return node, nil
	}
	result, err := g.pipeline.RunFieldNodeOverrides(fieldName, node, ctx)
	if err != nil {
		return nil, fmt.Errorf("generator: field override for %q: %w", fieldName, err)
	}
	return result, nil
}

// ── Field type mapping ────────────────────────────────────────────────────────

// fieldTypeToNodeKind maps a FieldType string to the appropriate NodeKind for form fields.
func fieldTypeToNodeKind(fieldType string, f FieldDef) widget.NodeKind {
	switch fieldType {
	case "data", "text", "small_text", "string":
		if f.MaxLength > 255 {
			return widget.NodeTextArea
		}
		return widget.NodeText
	case "long_text":
		return widget.NodeTextArea
	case "rich_text":
		return widget.NodeRichText
	case "int", "float", "decimal":
		return widget.NodeNumber
	case "currency":
		return widget.NodeMoney
	case "select":
		return widget.NodeSelect
	case "multi_select":
		return widget.NodeMultiSelect
	case "link":
		if f.DataSource != nil {
			return widget.NodeLookup
		}
		return widget.NodeSelect
	case "tree_link":
		return widget.NodeTreeSelect
	case "date":
		return widget.NodeDate
	case "datetime", "time":
		return widget.NodeDateTime
	case "duration":
		return widget.NodeDuration
	case "bool", "boolean":
		return widget.NodeSwitch
	case "json":
		return widget.NodeEditor
	case "color":
		return widget.NodeColor
	case "signature":
		return widget.NodeSignature
	case "file", "image", "attach":
		return widget.NodeFileUpload
	case "money":
		return widget.NodeMoney
	default:
		return widget.NodeText
	}
}

// fieldTypeToColumnNodeKind maps a FieldType string to the NodeKind for list columns.
func fieldTypeToColumnNodeKind(fieldType string, f FieldDef) widget.NodeKind {
	switch fieldType {
	case "int", "float", "decimal", "currency", "money":
		return widget.NodeNumber
	case "date":
		return widget.NodeDate
	case "datetime", "time":
		return widget.NodeDateTime
	case "bool", "boolean":
		return widget.NodeSwitch
	case "select", "multi_select", "link", "tree_link":
		return widget.NodeSelect
	case "file", "image", "attach":
		return widget.NodeFileUpload
	default:
		return widget.NodeText
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func indexFieldsBySection(schema EntitySchema) map[string][]FieldDef {
	idx := make(map[string][]FieldDef)
	for _, section := range schema.Sections {
		// Pre-populate to maintain order.
		idx[section.ID] = nil
	}
	for _, f := range schema.Fields {
		if f.Section != "" {
			idx[f.Section] = append(idx[f.Section], f)
		}
	}
	return idx
}

func findSection(sections []SectionDef, id string) *SectionDef {
	for i := range sections {
		if sections[i].ID == id {
			return &sections[i]
		}
	}
	return nil
}

func actionInViewMode(a ActionDef, mode sduictx.ViewMode) bool {
	if len(a.ViewModes) == 0 {
		return true // visible in all view modes
	}
	for _, m := range a.ViewModes {
		if m == mode {
			return true
		}
	}
	return false
}
