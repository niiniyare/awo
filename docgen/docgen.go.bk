// Package docgen generates human-readable documentation from compiled schemas.
// It can produce Markdown API references, entity field tables, and permission
// matrices — all derived from EntityDefinitions without manual authoring.
package docgen

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"awo.so/awo/compiler"
	"awo.so/awo/def"
)

// Options controls what docgen renders.
type Options struct {
	// IncludeSensitiveFields includes Sensitive fields in output.
	// Default false — sensitive fields are omitted from generated docs.
	IncludeSensitiveFields bool

	// IncludeHiddenFields includes Hidden fields in output.
	IncludeHiddenFields bool

	// IncludePermissions includes the RBAC permission matrix per entity.
	IncludePermissions bool

	// IncludeRoutes includes the auto-generated API route table.
	IncludeRoutes bool
}

// DefaultOptions returns sensible defaults for public documentation.
func DefaultOptions() Options {
	return Options{
		IncludePermissions: true,
		IncludeRoutes:      true,
	}
}

// WriteMarkdown writes a full Markdown API reference for the compiled schema
// to w. Entities are grouped by module and sorted alphabetically within each group.
func WriteMarkdown(w io.Writer, s *compiler.CompiledSchema, opts Options) error {
	// Group by module.
	groups := make(map[string][]*compiler.EntitySchema)
	var moduleOrder []string
	for _, es := range s.Entities {
		if _, seen := groups[es.Module]; !seen {
			moduleOrder = append(moduleOrder, es.Module)
		}
		groups[es.Module] = append(groups[es.Module], es)
	}
	sort.Strings(moduleOrder)

	// Header.
	if _, err := fmt.Fprintf(w, "# Awo Entity Reference\n\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Generated from `%d` entities across `%d` modules.\n\n",
		len(s.Entities), len(moduleOrder)); err != nil {
		return err
	}

	// Table of contents.
	if _, err := fmt.Fprintf(w, "## Table of Contents\n\n"); err != nil {
		return err
	}
	for _, mod := range moduleOrder {
		if _, err := fmt.Fprintf(w, "- [%s](#%s)\n", toTitle(mod), strings.ToLower(mod)); err != nil {
			return err
		}
		entities := groups[mod]
		sort.Slice(entities, func(i, j int) bool {
			return entities[i].LocalName < entities[j].LocalName
		})
		for _, es := range entities {
			anchor := strings.ReplaceAll(es.LocalName, "_", "-")
			if _, err := fmt.Fprintf(w, "  - [%s](#%s)\n", es.Label, anchor); err != nil {
				return err
			}
		}
	}
	if _, err := fmt.Fprintf(w, "\n---\n\n"); err != nil {
		return err
	}

	// Entity sections.
	for _, mod := range moduleOrder {
		entities := groups[mod]
		if _, err := fmt.Fprintf(w, "## %s\n\n", toTitle(mod)); err != nil {
			return err
		}
		for _, es := range entities {
			if err := writeEntitySection(w, es, s, opts); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeEntitySection(w io.Writer, es *compiler.EntitySchema, s *compiler.CompiledSchema, opts Options) error {
	anchor := strings.ReplaceAll(es.LocalName, "_", "-")

	fmt.Fprintf(w, "### %s {#%s}\n\n", es.Label, anchor)
	fmt.Fprintf(w, "**Entity name:** `%s`  \n", es.LocalName)
	kind := "Custom (JSONB)"
	if es.IsSystem {
		kind = "System (SQL)"
	}
	fmt.Fprintf(w, "**Storage:** %s  \n", kind)
	fmt.Fprintf(w, "**Table:** `%s`  \n\n", es.TableName)

	// Fields table.
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

	if len(visible) > 0 {
		fmt.Fprintf(w, "#### Fields\n\n")
		fmt.Fprintf(w, "| Name | Type | Required | Constraints | Description |\n")
		fmt.Fprintf(w, "|------|------|----------|-------------|-------------|\n")
		for _, f := range visible {
			constraints := fieldConstraints(f)
			desc := f.Description
			if desc == "" {
				desc = "—"
			}
			fmt.Fprintf(w, "| `%s` | `%s` | %s | %s | %s |\n",
				f.Name, f.Type, boolMark(f.Required), constraints, desc)
		}
		fmt.Fprintf(w, "\n")
	}

	// Edges.
	if len(es.Edges) > 0 {
		fmt.Fprintf(w, "#### Edges\n\n")
		fmt.Fprintf(w, "| Name | Target | Type | Cascade Delete |\n")
		fmt.Fprintf(w, "|------|--------|------|----------------|\n")
		for _, e := range es.Edges {
			fmt.Fprintf(w, "| `%s` | `%s` | `%s` | %s |\n",
				e.Name, e.Target, e.Type, boolMark(e.CascadeDelete))
		}
		fmt.Fprintf(w, "\n")
	}

	// Actions.
	if len(es.Actions) > 0 {
		fmt.Fprintf(w, "#### Custom Actions\n\n")
		fmt.Fprintf(w, "| Name | Method | Permission |\n")
		fmt.Fprintf(w, "|------|--------|------------|\n")
		for _, a := range es.Actions {
			method := string(a.Method)
			if method == "" {
				method = "POST"
			}
			perm := a.Permission
			if perm == "" {
				perm = "write"
			}
			fmt.Fprintf(w, "| `%s` | `%s` | `%s` |\n", a.Name, method, perm)
		}
		fmt.Fprintf(w, "\n")
	}

	// Permissions.
	if opts.IncludePermissions {
		perms := es.Permissions
		if len(perms.Create)+len(perms.Read)+len(perms.Write)+len(perms.Delete) > 0 {
			fmt.Fprintf(w, "#### Permissions\n\n")
			fmt.Fprintf(w, "| Operation | Allowed Roles |\n")
			fmt.Fprintf(w, "|-----------|---------------|\n")
			writePermRow(w, "create", perms.Create)
			writePermRow(w, "read", perms.Read)
			writePermRow(w, "write", perms.Write)
			writePermRow(w, "delete", perms.Delete)
			fmt.Fprintf(w, "\n")
		}
	}

	// Routes.
	if opts.IncludeRoutes {
		var entityRoutes []compiler.RouteDescriptor
		for _, r := range s.Routes {
			if r.EntityQualifiedName == es.QualifiedName {
				entityRoutes = append(entityRoutes, r)
			}
		}
		if len(entityRoutes) > 0 {
			fmt.Fprintf(w, "#### API Routes\n\n")
			fmt.Fprintf(w, "| Method | Path | Permission |\n")
			fmt.Fprintf(w, "|--------|------|------------|\n")
			for _, r := range entityRoutes {
				fmt.Fprintf(w, "| `%s` | `%s` | `%s` |\n", r.Method, r.Path, r.RequiredPermission)
			}
			fmt.Fprintf(w, "\n")
		}
	}

	fmt.Fprintf(w, "---\n\n")
	return nil
}

func writePermRow(w io.Writer, op string, subjects []string) {
	if len(subjects) == 0 {
		return
	}
	quoted := make([]string, len(subjects))
	for i, s := range subjects {
		quoted[i] = "`" + s + "`"
	}
	fmt.Fprintf(w, "| %s | %s |\n", op, strings.Join(quoted, ", "))
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
	if len(parts) == 0 {
		return "—"
	}
	return strings.Join(parts, ", ")
}

func boolMark(b bool) string {
	if b {
		return "✓"
	}
	return ""
}

func toTitle(s string) string {
	parts := strings.Split(s, "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}
