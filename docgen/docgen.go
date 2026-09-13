// Package docgen generates Markdown documentation from compiled entity schemas.
//
// Each entity produces one Markdown file containing:
//   - Purpose / description
//   - Entity type (system/custom) and scope
//   - Fields table (name, type, constraints)
//   - Edges table (relationships)
//   - Permissions (permission identifiers per operation)
//   - Actions (custom operations)
//   - Workflow triggers
//   - Audit behavior
//   - API endpoints
//
// Usage:
//
//	plan, err := docgen.Generate(schema)
//	for _, f := range plan.Files {
//	    os.WriteFile(outDir+"/"+f.Name+".md", []byte(f.Content), 0o644)
//	}
package docgen

import (
	"fmt"
	"strings"

	"awo.so/awo/compiler"
)

// File is one generated documentation file.
type File struct {
	// Name is the base file name without extension (e.g. "iam_user").
	Name string

	// Content is the Markdown content.
	Content string
}

// Plan is the output of Generate. Files are ordered by entity qualified name.
type Plan struct {
	// Files is the list of generated documentation files.
	Files []File
}

// Generate produces Markdown documentation for every entity in schema.
// Entities are documented in topological order (safe migration order) if a
// dependency graph is available, otherwise in declaration order.
func Generate(schema *compiler.CompiledSchema) (*Plan, error) {
	// Use topological order when available so docs follow dependency order.
	entities := schema.Entities
	if schema.Graph != nil && len(schema.Graph.TopologicalOrder) > 0 {
		ordered := make([]*compiler.EntitySchema, 0, len(entities))
		for _, name := range schema.Graph.TopologicalOrder {
			if es, ok := schema.ByName[name]; ok {
				ordered = append(ordered, es)
			}
		}
		if len(ordered) == len(entities) {
			entities = ordered
		}
	}

	plan := &Plan{Files: make([]File, 0, len(entities))}
	for _, es := range entities {
		content, err := renderEntity(es, schema)
		if err != nil {
			return nil, fmt.Errorf("docgen: entity %s: %w", es.QualifiedName, err)
		}
		plan.Files = append(plan.Files, File{
			Name:    es.QualifiedName,
			Content: content,
		})
	}
	return plan, nil
}

func renderEntity(es *compiler.EntitySchema, schema *compiler.CompiledSchema) (string, error) {
	var b strings.Builder

	// Title
	fmt.Fprintf(&b, "# %s (`%s`)\n\n", es.Label, es.QualifiedName)

	// Purpose
	if es.Description != "" {
		fmt.Fprintf(&b, "%s\n\n", es.Description)
	}

	// Entity metadata
	b.WriteString("## Overview\n\n")
	b.WriteString("| Property | Value |\n")
	b.WriteString("|---|---|\n")
	entityType := "custom"
	if es.IsSystem {
		entityType = "system"
	}
	fmt.Fprintf(&b, "| Type | %s |\n", entityType)
	fmt.Fprintf(&b, "| Module | `%s` |\n", es.Module)
	fmt.Fprintf(&b, "| Scope | `%s` |\n", string(es.Scope))
	fmt.Fprintf(&b, "| Table | `%s` |\n", es.TableName)
	fmt.Fprintf(&b, "| Audit | %s |\n", boolLabel(es.AllowAudit, "enabled", "disabled"))
	fmt.Fprintf(&b, "| API prefix | `%s` |\n", es.RoutePrefix)
	if es.Icon != "" {
		fmt.Fprintf(&b, "| Icon | `%s` |\n", es.Icon)
	}
	b.WriteString("\n")

	// Fields
	b.WriteString("## Fields\n\n")
	if len(es.Fields) == 0 {
		b.WriteString("_No fields declared._\n\n")
	} else {
		b.WriteString("| Name | Type | Required | Unique | Immutable | Sensitive | Description |\n")
		b.WriteString("|---|---|---|---|---|---|---|\n")
		for _, f := range es.Fields {
			desc := f.Description
			if f.Hidden {
				if desc != "" {
					desc = "_(hidden)_ " + desc
				} else {
					desc = "_(hidden)_"
				}
			}
			if f.ReadOnly {
				if desc != "" {
					desc = "_(read-only)_ " + desc
				} else {
					desc = "_(read-only)_"
				}
			}
			if len(f.Options) > 0 {
				optStr := "`" + strings.Join(f.Options, "`, `") + "`"
				if desc != "" {
					desc += " Options: " + optStr
				} else {
					desc = "Options: " + optStr
				}
			}
			if f.LinkTarget != "" {
				link := fmt.Sprintf("→ [`%s`](./%s.md)", f.LinkTarget, f.LinkTarget)
				if desc != "" {
					desc += " " + link
				} else {
					desc = link
				}
			}
			fmt.Fprintf(&b, "| `%s` | `%s` | %s | %s | %s | %s | %s |\n",
				f.Name, string(f.Type),
				checkMark(f.Required), checkMark(f.Unique),
				checkMark(f.Immutable), checkMark(f.Sensitive),
				mdEscape(desc),
			)
		}
		b.WriteString("\n")
	}

	// Edges
	if len(es.Edges) > 0 {
		b.WriteString("## Relationships\n\n")
		b.WriteString("| Name | Type | Target | Foreign Key |\n")
		b.WriteString("|---|---|---|---|\n")
		for _, e := range es.Edges {
			targetLink := fmt.Sprintf("[`%s`](./%s.md)", e.Target, e.Target)
			fmt.Fprintf(&b, "| `%s` | `%s` | %s | `%s` |\n",
				e.Name, string(e.Type), targetLink, e.ForeignKey)
		}
		b.WriteString("\n")
	}

	// Permissions
	b.WriteString("## Permissions\n\n")
	perms := es.Permissions
	hasPerms := len(perms.Create)+len(perms.Read)+len(perms.Write)+len(perms.Delete)+len(perms.Actions) > 0
	if !hasPerms {
		b.WriteString("_No permissions declared (internal-only entity)._\n\n")
	} else {
		b.WriteString("| Operation | Permission Identifiers |\n")
		b.WriteString("|---|---|\n")
		writePermRow(&b, "Create", perms.Create)
		writePermRow(&b, "Read", perms.Read)
		writePermRow(&b, "Write", perms.Write)
		writePermRow(&b, "Delete", perms.Delete)
		for action, ids := range perms.Actions {
			writePermRow(&b, action, ids)
		}
		b.WriteString("\n")
	}

	// Actions
	if len(es.Actions) > 0 {
		b.WriteString("## Actions\n\n")
		b.WriteString("| Name | Method | Endpoint | Permissions |\n")
		b.WriteString("|---|---|---|---|\n")
		for _, a := range es.Actions {
			method := string(a.Method)
			if method == "" {
				method = "POST"
			}
			endpoint := fmt.Sprintf("`%s/:id/%s`", es.RoutePrefix, a.Name)
			var actionPerms []string
			if p, ok := perms.Actions[a.Name]; ok {
				actionPerms = p
			}
			permStr := "_none_"
			if len(actionPerms) > 0 {
				permStr = "`" + strings.Join(actionPerms, "`, `") + "`"
			}
			fmt.Fprintf(&b, "| `%s` | %s | %s | %s |\n",
				a.Name, method, endpoint, permStr)
		}
		b.WriteString("\n")
	}

	// Workflow triggers
	if len(es.WorkflowTriggers) > 0 {
		b.WriteString("## Workflow Triggers\n\n")
		b.WriteString("| Event | Workflow |\n")
		b.WriteString("|---|---|\n")
		for _, wt := range es.WorkflowTriggers {
			fmt.Fprintf(&b, "| `%s` | `%s` |\n", string(wt.On), wt.WorkflowFn)
		}
		b.WriteString("\n")
	}

	// Audit behavior
	b.WriteString("## Audit Behavior\n\n")
	if es.AllowAudit {
		b.WriteString("Audit logging is **enabled**. Framework writes an `platform_audit_log` record for every Create, Update, and Delete operation. Sensitive fields are redacted before the audit record is written.\n\n")
	} else {
		b.WriteString("Audit logging is **disabled** (`DisableAudit: true`). This entity is either itself an audit trail or has prohibitive write volume.\n\n")
	}

	// API endpoints
	b.WriteString("## API Endpoints\n\n")
	b.WriteString("| Method | Path | Operation | Permission |\n")
	b.WriteString("|---|---|---|---|\n")
	for _, r := range schema.Routes {
		if r.EntityQualifiedName != es.QualifiedName {
			continue
		}
		fmt.Fprintf(&b, "| `%s` | `%s` | %s | `%s` |\n",
			r.Method, r.Path, r.Operation, r.RequiredPermission)
	}
	b.WriteString("\n")

	return b.String(), nil
}

func writePermRow(b *strings.Builder, op string, ids []string) {
	if len(ids) == 0 {
		return
	}
	fmt.Fprintf(b, "| %s | `%s` |\n", op, strings.Join(ids, "`, `"))
}

func checkMark(v bool) string {
	if v {
		return "✓"
	}
	return ""
}

func boolLabel(v bool, yes, no string) string {
	if v {
		return yes
	}
	return no
}

// mdEscape escapes pipe characters in Markdown table cells.
func mdEscape(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}
