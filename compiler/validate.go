package compiler

import (
	"fmt"
	"regexp"
	"strings"

	"awo.so/awo/def"
	"awo.so/awo/registry"
)

// permissionIdentifierRe matches the required format for permission identifiers:
// "{module}.{entity}.{operation}" — all lowercase, dot-separated, no spaces.
var permissionIdentifierRe = regexp.MustCompile(`^[a-z][a-z0-9_]*\.[a-z][a-z0-9_]*\.[a-z][a-z0-9_]*$`)

// localNameRe matches valid module-local entity names: snake_case, may be a
// single word ("user", "tenant") or compound ("org_assignment", "audit_log").
var localNameRe = regexp.MustCompile(`^[a-z][a-z0-9]*(?:_[a-z][a-z0-9]*)*$`)

// Validate runs semantic validation against the registry and returns Diagnostics.
// Called internally by Compile; also exported for tooling.
func Validate(reg *registry.Registry) Diagnostics {
	var ds Diagnostics
	defs := reg.All()

	// Build QualifiedName set for link-target resolution.
	names := make(map[string]bool, len(defs))
	for _, d := range defs {
		names[def.QualifiedName(d)] = true
	}

	for _, d := range defs {
		qname := def.QualifiedName(d)
		name := def.LocalName(d) // module-local

		// Local name format.
		if !localNameRe.MatchString(name) {
			ds = append(ds, Diagnostic{
				Severity:   SeverityError,
				EntityName: qname,
				Message:    fmt.Sprintf("local entity name %q must be snake_case (^[a-z][a-z0-9]*(?:_[a-z][a-z0-9]*)*$)", name),
			})
		}

		// Module-prefixed names are transitional — warn to migrate.
		if def.HasModulePrefix(d) {
			ds = append(ds, Diagnostic{
				Severity:   SeverityWarning,
				EntityName: qname,
				Message: fmt.Sprintf(
					"entity Name %q contains the module prefix — use %q and let the compiler derive %q",
					d.EntityName(), name, qname,
				),
			})
		}

		// Field-level checks.
		fieldNames := make(map[string]bool)
		for _, f := range d.EntityFields() {
			// Duplicate field name.
			if fieldNames[f.Name] {
				ds = append(ds, Diagnostic{
					Severity:   SeverityError,
					EntityName: qname,
					FieldName:  f.Name,
					Message:    "duplicate field name",
				})
			}
			fieldNames[f.Name] = true

			// Money must not be FieldTypeFloat or FieldTypeInt — flag when label hints money.
			// (We cannot detect by name alone perfectly, but flag explicit misuse patterns.)
			// The real rule: FieldTypeCurrency is the only correct money type.
			// Flag if a field named *_amount, *_price, *_cost, *_total uses float/int.
			if f.Type == def.FieldTypeFloat || f.Type == def.FieldTypeInt {
				lower := strings.ToLower(f.Name)
				for _, suffix := range []string{"_amount", "_price", "_cost", "_total", "_fee", "_tax"} {
					if strings.HasSuffix(lower, suffix) {
						ds = append(ds, Diagnostic{
							Severity:   SeverityWarning,
							EntityName: qname,
							FieldName:  f.Name,
							Message:    fmt.Sprintf("field %q looks like money but uses type %q — use FieldTypeCurrency instead", f.Name, f.Type),
						})
						break
					}
				}
			}

			// NamingSeries must have a non-empty pattern.
			if f.Type == def.FieldTypeNamingSeries && f.Series == "" {
				ds = append(ds, Diagnostic{
					Severity:   SeverityError,
					EntityName: qname,
					FieldName:  f.Name,
					Message:    "FieldTypeNamingSeries field must have a non-empty Series pattern",
				})
			}

			// Link targets must exist.
			if (f.Type == def.FieldTypeLink || f.Type == def.FieldTypeLinkList) && f.LinkTarget != "" {
				if !names[f.LinkTarget] {
					ds = append(ds, Diagnostic{
						Severity:   SeverityError,
						EntityName: qname,
						FieldName:  f.Name,
						Message:    fmt.Sprintf("link target %q is not registered", f.LinkTarget),
					})
				}
			}

			// Required + Default is redundant (warning only — default overrides required on omission).
			if f.Required && f.Default != nil {
				ds = append(ds, Diagnostic{
					Severity:   SeverityWarning,
					EntityName: qname,
					FieldName:  f.Name,
					Message:    "field is both Required and has a Default — the default satisfies the required constraint; consider removing Required",
				})
			}

			// Immutable without Required or Default means the field will always be zero on create.
			if f.Immutable && !f.Required && f.Default == nil {
				ds = append(ds, Diagnostic{
					Severity:   SeverityWarning,
					EntityName: qname,
					FieldName:  f.Name,
					Message:    "Immutable field has neither Required nor Default — it will always be zero-valued and cannot be changed after creation",
				})
			}
		}

		// Permission identifier format.
		// PermissionSet values MUST be "{module}.{entity}.{operation}" identifiers.
		// Role names ("role:*") are NOT allowed — they violate ADR-001/ADR-011 and
		// produce malformed CapabilityGrant records that Casbin can never satisfy.
		for _, perm := range collectPermissions(d.EntityPermissions()) {
			if strings.HasPrefix(perm, "role:") {
				// Derive a suggested permission identifier from the entity's module+name.
				module := d.EntityModule()
				local := def.LocalName(d)
				suggested := module + "." + local + ".read"
				ds = append(ds, Diagnostic{
					Severity:   SeverityError,
					EntityName: qname,
					Message: fmt.Sprintf(
						"PermissionSet entry %q is a role name, not a permission identifier; "+
							"this violates ADR-001/ADR-011. Use \"{module}.{entity}.{operation}\" format "+
							"(e.g. %q). Seed the role-to-permission mapping in iam_role_permissions instead.",
						perm, suggested,
					),
				})
			} else if !permissionIdentifierRe.MatchString(perm) {
				ds = append(ds, Diagnostic{
					Severity:   SeverityWarning,
					EntityName: qname,
					Message: fmt.Sprintf(
						"PermissionSet entry %q does not match the expected format "+
							"\"{module}.{entity}.{operation}\" (e.g. \"finance.invoice.create\")",
						perm,
					),
				})
			}
		}

		// Edge name uniqueness.
		edgeNames := make(map[string]bool)
		for _, e := range d.EntityEdges() {
			if edgeNames[e.Name] {
				ds = append(ds, Diagnostic{
					Severity:   SeverityError,
					EntityName: qname,
					Message:    fmt.Sprintf("duplicate edge name %q", e.Name),
				})
			}
			edgeNames[e.Name] = true
		}
	}

	return ds
}

// collectPermissions flattens all permission identifier strings from a PermissionSet
// into a single slice for validation purposes.
func collectPermissions(ps def.PermissionSet) []string {
	var all []string
	all = append(all, ps.Create...)
	all = append(all, ps.Read...)
	all = append(all, ps.Write...)
	all = append(all, ps.Delete...)
	for _, perms := range ps.Actions {
		all = append(all, perms...)
	}
	return all
}
