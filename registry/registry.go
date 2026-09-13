package registry

import (
	"fmt"
	"regexp"
	"strings"

	"awo.so/awo/def"
)

// Registry is the sealed, validated collection of EntityDefinitions. Once
// built it is read-only and safe for concurrent access.
type Registry struct {
	defs   []def.EntityDefinition
	byName map[string]def.EntityDefinition
}

// All returns all registered definitions in registration order.
func (r *Registry) All() []def.EntityDefinition {
	out := make([]def.EntityDefinition, len(r.defs))
	copy(out, r.defs)
	return out
}

// Lookup returns the EntityDefinition with the given name, or nil if not found.
func (r *Registry) Lookup(name string) def.EntityDefinition {
	return r.byName[name]
}

// Count returns the total number of registered definitions.
func (r *Registry) Count() int { return len(r.defs) }

// SystemEntities returns only SystemDefinition entries.
func (r *Registry) SystemEntities() []def.EntityDefinition {
	var out []def.EntityDefinition
	for _, d := range r.defs {
		if d.IsSystem() {
			out = append(out, d)
		}
	}
	return out
}

// CustomEntities returns only CustomDefinition entries.
func (r *Registry) CustomEntities() []def.EntityDefinition {
	var out []def.EntityDefinition
	for _, d := range r.defs {
		if !d.IsSystem() {
			out = append(out, d)
		}
	}
	return out
}

// ByModule returns all definitions that belong to the named module.
func (r *Registry) ByModule(module string) []def.EntityDefinition {
	var out []def.EntityDefinition
	for _, d := range r.defs {
		if d.EntityModule() == module {
			out = append(out, d)
		}
	}
	return out
}

// mandatorySystemEntities lists qualified entity names (module_name) that MUST
// be declared as SystemDefinition. Declaring any of these as CustomDefinition
// is a fatal validation error.
var mandatorySystemEntities = map[string]string{
	"iam_user":              "IAM data; JSONB corruption risk",
	"platform_tenant":       "Platform identity; accessible before per-tenant schemas load",
	"finance_ledger_entry":  "Double-entry accounting; SQL numeric constraints",
	"finance_journal_entry": "Double-entry; debit/credit balance enforced at DB level",
	"finance_payment":       "Financial transaction; SQL constraints + audit triggers",
	"finance_tax_entry":     "KRA eTIMS record; regulatory compliance",
	"inventory_stock_move":  "Inventory accounting; SQL quantity constraints",
}

// entityNameRe validates module-local snake_case names (no mandatory underscore —
// names like "user", "tenant", "customer" are valid; underscores are allowed
// for compound names like "org_assignment", "audit_log").
var entityNameRe = regexp.MustCompile(`^[a-z][a-z0-9]*(?:_[a-z][a-z0-9]*)*$`)

// fieldNameRe validates snake_case field names.
var fieldNameRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// Build reads all definitions registered via [def.Register], validates them,
// seals the def registry, and returns a read-only [Registry].
//
// Build is called exactly once during the Initialization Phase. It panics if
// validation fails — a process that cannot build a valid registry cannot serve
// requests.
func Build() *Registry {
	// Seal the def registry immediately: no new registrations after Build.
	def.Seal()

	defs := def.All()
	if len(defs) == 0 {
		panic("registry.Build: no EntityDefinitions registered — did module init() functions run?")
	}

	// Build qualified-name → definition map for link-target resolution.
	byQName := make(map[string]def.EntityDefinition, len(defs))
	for _, d := range defs {
		byQName[def.QualifiedName(d)] = d
	}

	var errs []string

	for _, d := range defs {
		errs = append(errs, validateDefinition(d, byQName)...)
	}

	if len(errs) > 0 {
		msg := strings.Join(errs, "\n  - ")
		panic(fmt.Sprintf("registry.Build: validation failed:\n  - %s", msg))
	}

	return &Registry{defs: defs, byName: byQName}
}

// BuildFrom constructs a Registry from an explicit list of EntityDefinitions
// without touching the global def registry. Intended for use in tests and
// tooling that needs an isolated registry.
//
// Returns an error instead of panicking so callers (e.g. test harnesses) can
// report failures cleanly via t.Fatal.
// BuildFrom constructs a Registry from an explicit list of EntityDefinitions
// without touching the global def registry. Intended for use in tests and
// tooling that needs an isolated registry.
//
// Unlike [Build], BuildFrom does not enforce mandatory-system-entity rules,
// allowing test registries to use arbitrary entity definitions. All other
// validation (name format, field names, link targets) still applies.
//
// Returns an error instead of panicking so callers (e.g. test harnesses) can
// report failures cleanly via t.Fatal.
func BuildFrom(defs []def.EntityDefinition) (*Registry, error) {
	if len(defs) == 0 {
		return nil, fmt.Errorf("registry.BuildFrom: no EntityDefinitions provided")
	}

	byQName := make(map[string]def.EntityDefinition, len(defs))
	for _, d := range defs {
		byQName[def.QualifiedName(d)] = d
	}

	var errs []string
	for _, d := range defs {
		// Skip mandatory-system-entity check in isolated test registries.
		errs = append(errs, validateDefinitionRelaxed(d, byQName)...)
	}

	if len(errs) > 0 {
		msg := strings.Join(errs, "\n  - ")
		return nil, fmt.Errorf("registry.BuildFrom: validation failed:\n  - %s", msg)
	}

	return &Registry{defs: defs, byName: byQName}, nil
}

// validateDefinitionRelaxed runs all validation rules except the
// mandatory-system-entity check. Used by BuildFrom for test/tooling registries.
func validateDefinitionRelaxed(d def.EntityDefinition, byName map[string]def.EntityDefinition) []string {
	errs := validateDefinition(d, byName)
	// Filter out mandatory-system-entity errors — they contain "must be a SystemDefinition".
	filtered := errs[:0]
	for _, e := range errs {
		if !strings.Contains(e, "must be a SystemDefinition") {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

func validateDefinition(d def.EntityDefinition, byQName map[string]def.EntityDefinition) []string {
	var errs []string
	localName := d.EntityName() // module-local name (e.g. "organization")
	module := d.EntityModule()
	qname := def.QualifiedName(d) // globally unique (e.g. "platform_organization")

	// Module must be non-empty.
	if module == "" {
		errs = append(errs, fmt.Sprintf("entity %q: Module is empty", localName))
	}

	// Local name must be valid snake_case.
	if !entityNameRe.MatchString(localName) {
		errs = append(errs, fmt.Sprintf(
			"entity %q (module %q): Name must be module-local snake_case (e.g. \"organization\", not %q)",
			qname, module, localName,
		))
	}

	// Note: module-prefixed names (e.g. "iam_user" with Module="iam") are
	// accepted during the transitional period. The compiler emits a warning
	// diagnostic for these — migrate to module-local names when convenient.

	// Label is derived from Name if empty — no error needed.

	// Mandatory system entity check (uses QualifiedName).
	if reason, mandatory := mandatorySystemEntities[qname]; mandatory && !d.IsSystem() {
		errs = append(errs, fmt.Sprintf(
			"entity %q: must be a SystemDefinition (%s)", qname, reason,
		))
	}

	// Field validation.
	fieldNames := make(map[string]bool)
	for _, f := range d.EntityFields() {
		if f.Name == "" {
			errs = append(errs, fmt.Sprintf("entity %q: field has empty Name", qname))
			continue
		}
		if !fieldNameRe.MatchString(f.Name) {
			errs = append(errs, fmt.Sprintf(
				"entity %q: field %q name must be snake_case", qname, f.Name,
			))
		}
		if fieldNames[f.Name] {
			errs = append(errs, fmt.Sprintf(
				"entity %q: duplicate field name %q", qname, f.Name,
			))
		}
		fieldNames[f.Name] = true

		// Link target must be a registered QualifiedName.
		if f.Type == def.FieldTypeLink || f.Type == def.FieldTypeLinkList {
			if f.LinkTarget == "" {
				errs = append(errs, fmt.Sprintf(
					"entity %q: field %q (Link/LinkList) has empty LinkTarget", qname, f.Name,
				))
			} else if byQName[f.LinkTarget] == nil {
				errs = append(errs, fmt.Sprintf(
					"entity %q: field %q LinkTarget %q is not registered",
					qname, f.Name, f.LinkTarget,
				))
			}
		}

		// NamingSeries requires a Series pattern.
		if f.Type == def.FieldTypeNamingSeries && f.Series == "" {
			errs = append(errs, fmt.Sprintf(
				"entity %q: field %q (NamingSeries) has empty Series pattern", qname, f.Name,
			))
		}

		// Select requires Options.
		if f.Type == def.FieldTypeSelect && len(f.Options) == 0 {
			errs = append(errs, fmt.Sprintf(
				"entity %q: field %q (Select) has no Options", qname, f.Name,
			))
		}

		// Money must never be FieldTypeFloat.
		if f.Type == def.FieldTypeFloat {
			lower := strings.ToLower(f.Name)
			for _, hint := range []string{"amount", "total", "price", "cost", "fee", "tax"} {
				if strings.Contains(lower, hint) {
					errs = append(errs, fmt.Sprintf(
						"entity %q: field %q looks monetary but uses FieldTypeFloat — "+
							"use FieldTypeCurrency (numeric(20,4)) instead",
						qname, f.Name,
					))
					break
				}
			}
		}
	}

	// Edge validation.
	edgeNames := make(map[string]bool)
	for _, e := range d.EntityEdges() {
		if e.Name == "" {
			errs = append(errs, fmt.Sprintf("entity %q: edge has empty Name", qname))
			continue
		}
		if edgeNames[e.Name] {
			errs = append(errs, fmt.Sprintf("entity %q: duplicate edge name %q", qname, e.Name))
		}
		edgeNames[e.Name] = true

		if e.Target == "" {
			errs = append(errs, fmt.Sprintf(
				"entity %q: edge %q has empty Target", qname, e.Name,
			))
		} else if byQName[e.Target] == nil {
			errs = append(errs, fmt.Sprintf(
				"entity %q: edge %q Target %q is not registered", qname, e.Name, e.Target,
			))
		}
	}

	// Action name uniqueness.
	actionNames := make(map[string]bool)
	for _, a := range d.EntityActions() {
		if a.Name == "" {
			errs = append(errs, fmt.Sprintf("entity %q: action has empty Name", qname))
			continue
		}
		if actionNames[a.Name] {
			errs = append(errs, fmt.Sprintf(
				"entity %q: duplicate action name %q", qname, a.Name,
			))
		}
		actionNames[a.Name] = true

		if a.HandlerFunc == nil {
			errs = append(errs, fmt.Sprintf(
				"entity %q: action %q has nil HandlerFunc", qname, a.Name,
			))
		}
	}

	return errs
}
