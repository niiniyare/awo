// Package adapt bridges the framework compiler and the SDUI generator.
//
// # Problem
//
// The compiler produces *compiler.EntitySchema from EntityDefinition metadata.
// The SDUI engine consumes generator.EntitySchema. These are different types
// in different packages to preserve dependency isolation — the generator must
// not import the compiler (that would create a cycle through the plugin system).
//
// # Solution
//
// adapt.FromCompiled translates *compiler.EntitySchema → generator.EntitySchema
// so HTTP handlers and bootstrap code can wire the two systems together without
// manual construction of generator.EntitySchema values.
//
// # Usage
//
//	// In an HTTP handler:
//	gSchema := adapt.FromCompiled(compiledSchema.ByName["finance_invoice"])
//	resp, err := eng.Handle(ctx, engine.Request{
//	    Ctx:    sduiCtx,
//	    Schema: gSchema,
//	})
//
// # Fingerprinting
//
// Cache correctness requires stable, content-derived fingerprints for the
// schema and permission dimensions of the cache key. [SchemaFingerprint]
// computes a deterministic FNV-64a hash of the fields, layout, and actions —
// fast enough to compute inline on every request.
package adapt

import (
	"context"
	"fmt"
	"hash/fnv"
	"strings"

	"github.com/google/uuid"

	"awo.so/awo/auth"
	"awo.so/awo/compiler"
	"awo.so/awo/def"
	"awo.so/awo/sdui/dashboard"
	"awo.so/awo/sdui/generator"
	"awo.so/awo/sdui/sduictx"
	"awo.so/awo/sdui/widget"
)

// FromCompiled converts a *compiler.EntitySchema into a generator.EntitySchema.
//
// The conversion is allocation-heavy by design — generator.EntitySchema is
// immutable once constructed, and per-request allocations at this layer are
// negligible relative to render cost.
//
// Callers should cache the result (e.g. once per compiled entity at startup)
// rather than calling FromCompiled on every request.
func FromCompiled(es *compiler.EntitySchema) generator.EntitySchema {
	gs := generator.EntitySchema{
		Name:        es.QualifiedName,
		Title:       es.Label,
		PluralTitle: es.LabelPlural,
		Icon:        es.Icon,
		ListURL:     es.RoutePrefix,
		CreateURL:   es.RoutePrefix,
		EditURL:     es.RoutePrefix + "/${id}",
		DetailURL:   es.RoutePrefix + "/${id}",
		UIPrefix:    "/ui/" + es.Module + "/" + es.APIResource,
		Permissions: permissionsMap(es.Permissions),
		HasWorkflow: len(es.WorkflowTriggers) > 0,
	}

	// Fields.
	for _, f := range es.Fields {
		gf := convertField(f, es)
		gs.Fields = append(gs.Fields, gf)
	}

	// Sections and Tabs from LayoutDef.
	if len(es.Layout.Tabs) > 0 {
		gs.Sections, gs.Tabs = convertTabs(es.Layout.Tabs)
	} else if len(es.Layout.Sections) > 0 {
		gs.Sections = convertSections(es.Layout.Sections)
	}

	// Actions.
	for _, a := range es.Actions {
		if !a.Hidden {
			gs.Actions = append(gs.Actions, convertAction(a))
		}
	}

	// Relations (edges).
	for _, e := range es.Edges {
		if !e.Hidden {
			gs.Relations = append(gs.Relations, convertEdge(e, es))
		}
	}

	// Dashboard panels from global registry — match by module.
	gs.DashboardPanels = collectDashboardPanels(es.Module)

	return gs
}

// SchemaFingerprint computes a stable content fingerprint for es.
// The fingerprint covers field names, field types, layout structure, and
// action names — anything that would change the generated widget tree.
// It does NOT cover permission identifiers (those are covered separately).
//
// The result is a 16-hex-character string, e.g. "a3f2e1d0c4b59817".
func SchemaFingerprint(es *compiler.EntitySchema) string {
	h := fnv.New64a()
	// Label and LabelPlural are included because they appear as page titles.
	fmt.Fprint(h, es.QualifiedName, "|", es.Icon, "|", es.Label, "|", es.LabelPlural, "|")
	for _, f := range es.Fields {
		// Label is included because a label change alters the generated widget
		// tree. Field permission identifiers are NOT included here — permission
		// gating is handled by the PermFingerprint cache dimension.
		fmt.Fprintf(h, "%s:%s:%s,", f.Name, f.Type, f.Label)
	}
	fmt.Fprint(h, "|layout:")
	for _, tab := range es.Layout.Tabs {
		fmt.Fprint(h, tab.Name, "/", tab.Permission, "[")
		for _, sec := range tab.Sections {
			fmt.Fprint(h, sec.Name, "/", sec.Permission, "{")
			for _, col := range sec.Columns {
				fmt.Fprint(h, strings.Join(col.Fields, ","), ";")
			}
			fmt.Fprint(h, "}")
		}
		fmt.Fprint(h, "]")
	}
	for _, sec := range es.Layout.Sections {
		fmt.Fprint(h, sec.Name, "/", sec.Permission, "{")
		for _, col := range sec.Columns {
			fmt.Fprint(h, strings.Join(col.Fields, ","), ";")
		}
		fmt.Fprint(h, "}")
	}
	fmt.Fprint(h, "|actions:")
	for _, a := range es.Actions {
		// Label included: a label change alters the action button text in the UI.
		fmt.Fprint(h, a.Name, ":", a.Label, ",")
	}
	return fmt.Sprintf("%016x", h.Sum64())
}

// ── viewer adapter ────────────────────────────────────────────────────────────

// ViewerAdapter wraps an auth.ViewerContext and implements sduictx.ViewerContext.
//
// The adapter uses a pre-indexed CapabilityGrant map to translate permission
// identifiers (like "finance.invoice.read_sensitive") into PolicyEvaluator
// calls. This index is built once per CompiledSchema — not per request.
//
// Construction:
//
//	grants := adapt.BuildGrantIndex(compiledSchema.CapabilityGrants)
//	adapter := adapt.NewViewerAdapter(ctx, viewer, evaluator, grants)
type ViewerAdapter struct {
	inner     auth.ViewerContext
	ctx       context.Context
	evaluator auth.PolicyEvaluator // may be nil (open = all permitted)
	grants    GrantIndex           // permID → (entity, action)
}

type grantEntry struct {
	entity string
	action string
}

// Ensure ViewerAdapter satisfies sduictx.ViewerContext at compile time.
var _ sduictx.ViewerContext = ViewerAdapter{}

// NewViewerAdapter constructs a ViewerAdapter.
//
//   - ctx is the HTTP request context, forwarded to PolicyEvaluator.
//   - viewer is the authenticated principal from auth middleware.
//   - evaluator may be nil (all field permissions granted).
//   - grants is the pre-built index from BuildGrantIndex.
func NewViewerAdapter(ctx context.Context, viewer auth.ViewerContext, evaluator auth.PolicyEvaluator, grants GrantIndex) ViewerAdapter {
	return ViewerAdapter{
		inner:     viewer,
		ctx:       ctx,
		evaluator: evaluator,
		grants:    grants,
	}
}

func (a ViewerAdapter) TenantID() uuid.UUID   { return a.inner.TenantID() }
func (a ViewerAdapter) Roles() []string       { return a.inner.Roles() }
func (a ViewerAdapter) IsPlatformAdmin() bool { return a.inner.IsPlatformAdmin() }

// HasPermission implements sduictx.ViewerContext.
// Platform admins always have all permissions. For others, the grant index is
// consulted to resolve the (entity, action) pair and PolicyEvaluator is called.
// Unknown permission identifiers are denied — unknown perms are not defined in
// the compiled schema and cannot be granted.
// Evaluator errors are treated as denial (field is absent, not broken).
func (a ViewerAdapter) HasPermission(permID string) bool {
	if a.inner.IsPlatformAdmin() {
		return true
	}
	if a.evaluator == nil {
		return true
	}
	g, ok := a.grants[permID]
	if !ok {
		return false
	}
	ok2, _ := a.evaluator.CanPerform(a.ctx, a.inner, g.entity, g.action)
	return ok2
}

// BuildGrantIndex builds an O(1) permission ID lookup from a CapabilityGrant slice.
// Call once at startup (after Compile) and pass the result to NewViewerAdapter.
// The returned map is read-only and safe for concurrent use.
func BuildGrantIndex(grants []auth.CapabilityGrant) map[string]grantEntry {
	idx := make(map[string]grantEntry, len(grants))
	for _, g := range grants {
		idx[g.Permission] = grantEntry{entity: g.Entity, action: g.Action}
	}
	return idx
}

// GrantIndex is the opaque type returned by BuildGrantIndex.
// Used in NewViewerAdapter.
type GrantIndex = map[string]grantEntry

// ── field conversion ──────────────────────────────────────────────────────────

func convertField(f def.FieldDef, es *compiler.EntitySchema) generator.FieldDef {
	gf := generator.FieldDef{
		Name:        f.Name,
		Label:       labelFor(f),
		FieldType:   string(f.Type),
		Required:    f.Required,
		ReadOnly:    f.ReadOnly || f.Immutable,
		Hidden:      f.Hidden || f.Sensitive,
		InList:      !f.Hidden && !f.Sensitive && isListable(f.Type),
		InForm:      !f.Hidden && !f.Sensitive,
		InDetail:    !f.Hidden,
		Description: f.Description,
		MaxLength:   f.MaxLen,
		Placeholder: f.Placeholder,
		Icon:        f.Icon,
		Width:       f.Width,
		Computed:    f.Computed,
		Searchable:  f.Searchable || isDefaultSearchable(f.Type),
		ClearOn:     f.ClearOn,
		VisibleOn:   f.VisibleOn,
		HiddenOn:    f.HiddenOn,
		DisabledOn:  f.DisabledOn,
		RequiredOn:  f.RequiredOn,
	}

	// Static select options.
	if len(f.Options) > 0 {
		for _, opt := range f.Options {
			gf.Options = append(gf.Options, generator.SelectOption{
				Label: optionLabel(opt),
				Value: opt,
			})
		}
	}

	// Link / LinkList: wire DataSource from compiled lookup.
	if lookup, ok := es.FieldLookups[f.Name]; ok {
		gf.DataSource = &widget.DataSource{
			URL:        lookup.SearchURL,
			ValueField: lookup.ValueField,
			LabelField: lookup.LabelField,
		}
		gf.LinkedEntity = lookup.TargetQualifiedName
	}

	return gf
}

// isListable returns true for field types that render well as list columns.
// Long-form text, JSON blobs, link lists, and multi-selects are excluded.
func isListable(t def.FieldType) bool {
	switch t {
	case def.FieldTypeLongText, def.FieldTypeJSON,
		def.FieldTypeLinkList, def.FieldTypeDynamicLink,
		def.FieldTypeMultiSelect:
		return false
	}
	return true
}

// isDefaultSearchable returns true for field types that are inherently
// filterable in an ERP list view via equality predicates (select, date, bool,
// naming_series). These types don't need a GIN index for filtering, so the
// filter bar shows them by default even when def.FieldDef.Searchable is false.
// Free-text types (data, small_text) are excluded because they require GIN
// indexes and must be explicitly marked Searchable on the field definition.
func isDefaultSearchable(t def.FieldType) bool {
	switch t {
	case def.FieldTypeSelect, def.FieldTypeMultiSelect,
		def.FieldTypeDate, def.FieldTypeDateTime,
		def.FieldTypeBool, def.FieldTypeNamingSeries:
		return true
	}
	return false
}

// labelFor returns a human-readable label: def.FieldDef.Label if set,
// otherwise derived from the field name.
func labelFor(f def.FieldDef) string {
	if f.Label != "" {
		return f.Label
	}
	return def.DeriveLabel(f.Name)
}

// optionLabel converts a snake_case option value to Title Case display text.
func optionLabel(v string) string {
	parts := strings.Split(v, "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

// ── layout conversion ─────────────────────────────────────────────────────────

// convertTabs translates a tabbed layout into generator sections + tabs.
// All sections from all tabs are flattened into the sections slice; tabs
// reference sections by ID.
func convertTabs(tabs []def.TabDef) ([]generator.SectionDef, []generator.TabDef) {
	var sections []generator.SectionDef
	var gtabs []generator.TabDef

	for _, tab := range tabs {
		var sectionIDs []string
		for _, sec := range tab.Sections {
			id := tab.Name + "." + sec.Name
			sections = append(sections, convertSection(sec, id))
			sectionIDs = append(sectionIDs, id)
		}
		gtabs = append(gtabs, generator.TabDef{
			ID:          tab.Name,
			Title:       tab.Label,
			Icon:        tab.Icon,
			Description: tab.Description,
			Permission:  tab.Permission,
			Sections:    sectionIDs,
		})
	}
	return sections, gtabs
}

// convertSections translates root (non-tabbed) sections.
func convertSections(secs []def.SectionDef) []generator.SectionDef {
	var out []generator.SectionDef
	for _, sec := range secs {
		out = append(out, convertSection(sec, sec.Name))
	}
	return out
}

// convertSection translates one def.SectionDef.
//
// Multi-column field lists are interleaved so the layout engine packs them
// correctly: given columns A=[a1,a2] and B=[b1,b2], the output field order
// is [a1,b1,a2,b2], which the layout engine places into two 6-wide cells per
// row when the section declares 2 columns.
func convertSection(sec def.SectionDef, id string) generator.SectionDef {
	cols := len(sec.Columns)
	fields := interleaveColumns(sec.Columns)

	return generator.SectionDef{
		ID:          id,
		Title:       sec.Label,
		Icon:        sec.Icon,
		Description: sec.Description,
		Permission:  sec.Permission,
		Collapsible: sec.Collapsible,
		Collapsed:   sec.Collapsed,
		Fields:      fields,
		Columns:     cols,
	}
}

// interleaveColumns merges multiple column field lists by interleaving their
// elements so the layout engine produces the correct visual column alignment.
//
//	cols = [{a,b,c}, {d,e,f}]  →  [a,d,b,e,c,f]
func interleaveColumns(cols []def.ColumnDef) []string {
	if len(cols) == 0 {
		return nil
	}
	if len(cols) == 1 {
		return cols[0].Fields
	}

	// Find max column length.
	maxLen := 0
	for _, c := range cols {
		if len(c.Fields) > maxLen {
			maxLen = len(c.Fields)
		}
	}

	out := make([]string, 0, maxLen*len(cols))
	for row := 0; row < maxLen; row++ {
		for _, col := range cols {
			if row < len(col.Fields) {
				out = append(out, col.Fields[row])
			}
		}
	}
	return out
}

// ── edge / relation conversion ────────────────────────────────────────────────

// convertEdge translates a def.EdgeDef into a generator.RelationDef.
// The DataURL is constructed from the target entity's RoutePrefix (resolved
// via es.EdgeTargets) plus a foreign key filter parameter.
func convertEdge(e def.EdgeDef, es *compiler.EntitySchema) generator.RelationDef {
	fk := e.ForeignKey
	if fk == "" {
		// Default FK convention: {parent_local_name}_id
		fk = es.LocalName + "_id"
	}

	// Resolve target entity RoutePrefix for the data URL.
	dataURL := ""
	if target, ok := es.EdgeTargets[e.Name]; ok {
		dataURL = target.RoutePrefix + "?" + fk + "=${id}"
	}

	label := e.Label
	if label == "" {
		label = def.DeriveLabel(e.Name)
	}

	return generator.RelationDef{
		Name:         e.Name,
		Label:        label,
		RelationType: string(e.Type),
		TargetEntity: e.Target,
		DataURL:      dataURL,
		ForeignKey:   fk,
		Hidden:       e.Hidden,
	}
}

// ── action conversion ─────────────────────────────────────────────────────────

func convertAction(a def.ActionDef) generator.ActionDef {
	ga := generator.ActionDef{
		ID:          a.Name,
		Label:       a.Label,
		Permission:  a.Permission,
		ConfirmText: a.ConfirmMessage,
		Icon:        a.Icon,
		ActionType:  actionType(a),
		Level:       actionLevel(a.Name),
	}
	return ga
}

func actionType(a def.ActionDef) string {
	if a.WorkflowEvent != "" {
		return "workflow"
	}
	return "ajax"
}

// actionLevel derives a visual prominence level from conventional action names.
func actionLevel(name string) string {
	switch name {
	case "submit", "approve", "confirm", "post":
		return "primary"
	case "cancel", "reject", "void":
		return "warning"
	case "delete", "discard":
		return "danger"
	default:
		return "default"
	}
}

// ── dashboard panel collection ────────────────────────────────────────────────

// collectDashboardPanels returns generator.DashboardPanel values for all
// dashboard.PanelDef entries from dashboards registered under the given module.
// Only dashboards whose Module matches are included. Returns nil when no
// dashboards are registered for the module.
func collectDashboardPanels(module string) []generator.DashboardPanel {
	var out []generator.DashboardPanel
	for _, dash := range dashboard.All() {
		if dash.Module != module {
			continue
		}
		for _, p := range dash.Panels {
			out = append(out, generator.DashboardPanel{
				ID:          p.ID,
				Title:       p.Title,
				PanelType:   string(p.Kind),
				DataURL:     p.DataSource,
				ChartType:   string(p.ChartType),
				KPIFormat:   string(p.KPIFormat),
				ValueField:  p.ValueField,
				ColSpan:     p.ColSpan,
				Permissions: p.Permissions,
			})
		}
	}
	return out
}

// ── permission fingerprint ────────────────────────────────────────────────────

// RolesFingerprint computes a stable cache-key dimension from a viewer's roles.
//
// The fingerprint is included in the SDUI cache key so that widget trees
// generated for different permission sets are stored separately. Without this,
// a cached schema produced for a platform-admin viewer (which includes all
// action buttons) could be returned to a restricted viewer.
//
// Platform admins use a dedicated fingerprint ("__admin__") — they always
// have all permissions regardless of their role list, so their cache entries
// are distinct from any role combination.
//
// The fingerprint is NOT a security gate. The generator already gates actions
// on viewer.HasPermission at generation time. The fingerprint only ensures
// that permission-differentiated trees are cached under distinct keys.
func RolesFingerprint(viewer auth.ViewerContext) string {
	if viewer.IsPlatformAdmin() {
		return "__admin__"
	}
	roles := viewer.Roles()
	if len(roles) == 0 {
		return "__noroles__"
	}
	// Sort a copy — Roles() slice order is not guaranteed stable.
	sorted := make([]string, len(roles))
	copy(sorted, roles)
	// Inline sort: roles slice is small (typically < 10 elements).
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j] < sorted[j-1]; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	h := fnv.New64a()
	for _, r := range sorted {
		fmt.Fprint(h, r, "|")
	}
	return fmt.Sprintf("%016x", h.Sum64())
}

// ── permission map ────────────────────────────────────────────────────────────

func permissionsMap(p def.PermissionSet) map[string]string {
	m := make(map[string]string, 4)
	if len(p.Create) > 0 {
		m["create"] = p.Create[0]
	}
	if len(p.Read) > 0 {
		m["read"] = p.Read[0]
	}
	if len(p.Write) > 0 {
		m["update"] = p.Write[0]
	}
	if len(p.Delete) > 0 {
		m["delete"] = p.Delete[0]
	}
	return m
}
