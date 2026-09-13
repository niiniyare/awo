package docgen

import (
	"awo.so/awo/compiler"
	"awo.so/awo/def"
)

// entitySlugs tracks anchors already emitted for entity headings in this
// render, so two entities that happen to slug identically (e.g. "Org Type"
// and "org-type" from a different module) get disambiguated rather than
// silently colliding — see slugify's doc comment for why this must be
// shared state across the whole render, not per-module.
type entitySlugs struct {
	seen map[string]int
}

func newEntitySlugs() *entitySlugs {
	return &entitySlugs{seen: make(map[string]int)}
}

func (s *entitySlugs) anchorFor(label string) string {
	return disambiguate(slugify(label), s.seen)
}

// writeEntitySection renders one entity's full documentation section:
// identity, fields, edges, actions, workflow triggers, permissions, and
// routes, in that order. Every sub-section is optional and only appears
// when the entity actually has content for it, so a simple lookup-style
// entity (like OrgTypeDefinition) doesn't render a wall of empty headings.
func writeEntitySection(m *mdWriter, es *compiler.EntitySchema, routesByEntity map[string][]compiler.RouteDescriptor, opts Options, slugs *entitySlugs) {
	anchor := slugs.anchorFor(es.Label)
	m.headingWithAnchor(3, es.Label, anchor)

	writeIdentity(m, es)
	writeFieldsTable(m, es, opts)
	writeEdgesTable(m, es)
	writeActionsTable(m, es)
	writeWorkflowTriggersTable(m, es)
	if opts.IncludePermissions {
		writePermissionsTable(m, es)
	}
	if opts.IncludeRoutes {
		writeRoutesTable(m, es, routesByEntity)
	}
	m.rule()
}

func writeIdentity(m *mdWriter, es *compiler.EntitySchema) {
	kind := "Custom (JSONB)"
	if es.IsSystem {
		kind = "System (SQL)"
	}
	m.raw("**Entity name:** `%s`  \n", es.LocalName)
	m.raw("**Storage:** %s  \n", kind)
	m.raw("**Table:** `%s`  \n\n", es.TableName)
}

// writeFieldsTable renders the field reference table, respecting
// Options.IncludeSensitiveFields / IncludeHiddenFields. Descriptions and
// labels come from EntityDefinition authors — including tenant admins for
// custom entities — so they pass through mdWriter.table's cell escaping
// rather than being interpolated as raw Markdown.
func writeFieldsTable(m *mdWriter, es *compiler.EntitySchema, opts Options) {
	visible := make([]def.FieldDef, 0, len(es.Fields))
	for _, f := range es.Fields {
		if f.Sensitive && !opts.IncludeSensitiveFields {
			continue
		}
		if f.Hidden && !opts.IncludeHiddenFields {
			continue
		}
		visible = append(visible, f)
	}
	if len(visible) == 0 {
		return
	}

	m.heading(4, "Fields")
	rows := make([][]string, 0, len(visible))
	for _, f := range visible {
		rows = append(rows, []string{
			"`" + f.Name + "`",
			"`" + string(f.Type) + "`",
			boolMark(f.Required),
			fieldConstraints(f),
			f.Description,
		})
	}
	m.table([]string{"Name", "Type", "Required", "Constraints", "Description"}, rows)
}

func writeEdgesTable(m *mdWriter, es *compiler.EntitySchema) {
	if len(es.Edges) == 0 {
		return
	}
	m.heading(4, "Edges")
	rows := make([][]string, 0, len(es.Edges))
	for _, e := range es.Edges {
		rows = append(rows, []string{
			"`" + e.Name + "`",
			"`" + e.Target + "`",
			"`" + string(e.Type) + "`",
			boolMark(e.CascadeDelete),
		})
	}
	m.table([]string{"Name", "Target", "Type", "Cascade Delete"}, rows)
}

// writeActionsTable renders declared custom actions. Method and Permission
// defaults ("POST" / "write") are intentionally NOT re-applied here: if an
// EntityDefinition author leaves either blank, that's a compiler concern —
// the compiler should resolve defaults once when building EntitySchema, and
// docgen should only ever render already-resolved values. Rendering a
// second, independent default here would let the two silently drift.
func writeActionsTable(m *mdWriter, es *compiler.EntitySchema) {
	if len(es.Actions) == 0 {
		return
	}
	m.heading(4, "Custom Actions")
	rows := make([][]string, 0, len(es.Actions))
	for _, a := range es.Actions {
		rows = append(rows, []string{
			"`" + a.Name + "`",
			"`" + string(a.Method) + "`",
			"`" + a.Permission + "`",
			a.ConfirmMessage,
		})
	}
	m.table([]string{"Name", "Method", "Permission", "Confirmation"}, rows)
}

// writeWorkflowTriggersTable renders Temporal workflow triggers declared on
// the entity (see finance_invoice's EventOnSubmit → InvoiceApprovalWorkflow
// wiring) — surfacing these matters for anyone auditing which entity events
// kick off long-running orchestration.
func writeWorkflowTriggersTable(m *mdWriter, es *compiler.EntitySchema) {
	if len(es.WorkflowTriggers) == 0 {
		return
	}
	m.heading(4, "Workflow Triggers")
	rows := make([][]string, 0, len(es.WorkflowTriggers))
	for _, t := range es.WorkflowTriggers {
		rows = append(rows, []string{
			"`" + string(t.On) + "`",
			"`" + t.WorkflowFn + "`",
			"`" + t.TaskQueue + "`",
		})
	}
	m.table([]string{"Event", "Workflow", "Task Queue"}, rows)
}

// writePermissionsTable renders the static role/policy lists from
// PermissionSet. Note that a Policy func (row-level filtering, as used by
// finance_invoice to scope viewers to Draft-status invoices) is dynamic and
// cannot be rendered as a static table row — we surface its presence
// explicitly instead of silently omitting it, so readers aren't misled into
// thinking the static role list is the complete access story.
func writePermissionsTable(m *mdWriter, es *compiler.EntitySchema) {
	perms := es.Permissions
	if len(perms.Create)+len(perms.Read)+len(perms.Write)+len(perms.Delete) == 0 {
		return
	}
	m.heading(4, "Permissions")
	var rows [][]string
	appendPermRow := func(op string, subjects []string) {
		if len(subjects) == 0 {
			return
		}
		rows = append(rows, []string{op, "`" + joinBackticked(subjects) + "`"})
	}
	appendPermRow("create", perms.Create)
	appendPermRow("read", perms.Read)
	appendPermRow("write", perms.Write)
	appendPermRow("delete", perms.Delete)
	m.table([]string{"Operation", "Allowed Roles"}, rows)

	if perms.Policy != nil {
		m.raw("> **Note:** this entity also applies a row-level access policy " +
			"evaluated at request time (see `Permissions.Policy` in source). " +
			"The table above lists only the static role requirements; the " +
			"policy may further restrict which records a permitted role can see.\n\n")
	}
}

// writeRoutesTable renders the auto-generated REST routes for one entity.
// routesByEntity is a precomputed index (built once per WriteMarkdown call
// in render.go) rather than a linear scan of schema.Routes per entity —
// the original implementation rescanned the full route list for every
// entity, which is O(entities × routes) for no reason.
func writeRoutesTable(m *mdWriter, es *compiler.EntitySchema, routesByEntity map[string][]compiler.RouteDescriptor) {
	key := es.Module + "." + es.QualifiedName
	rows := make([][]string, 0, len(routesByEntity[key]))
	for _, r := range routesByEntity[key] {
		rows = append(rows, []string{
			"`" + r.Method + "`",
			"`" + r.Path + "`",
			"`" + r.RequiredPermission + "`",
		})
	}
	if len(rows) == 0 {
		return
	}
	m.heading(4, "API Routes")
	m.table([]string{"Method", "Path", "Permission"}, rows)
}

func joinBackticked(subjects []string) string {
	// Pre-formatted as a single already-backticked+comma-joined string so
	// escapeCell doesn't need special-case logic for lists; the backticks
	// go around the whole cell, matching the style used elsewhere.
	out := ""
	for i, s := range subjects {
		if i > 0 {
			out += "`, `"
		}
		out += s
	}
	return out
}

func fieldConstraints(f def.FieldDef) string {
	var parts []string
	if f.Unique {
		parts = append(parts, "unique")
	}
	if f.Immutable {
		parts = append(parts, "immutable")
	}
	if f.Sensitive {
		parts = append(parts, "sensitive")
	}
	if f.Searchable {
		parts = append(parts, "searchable")
	}
	if f.TenantOverridable {
		parts = append(parts, "tenant-overridable")
	}
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for _, p := range parts[1:] {
		out += ", " + p
	}
	return out
}

func boolMark(b bool) string {
	if b {
		return "✓"
	}
	return ""
}
