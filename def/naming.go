package def

import "strings"

// QualifiedName returns the globally unique entity identifier: module + "_" + local name.
//
// During the transitional period, if the definition's Name already contains
// the module prefix (old-style), QualifiedName returns it unchanged. This
// allows old- and new-style definitions to coexist while migrating.
//
// Examples:
//
//	Module="platform", Name="organization"  → "platform_organization"
//	Module="iam",      Name="user"          → "iam_user"
//	Module="demo",     Name="customer"      → "demo_customer"
func QualifiedName(d EntityDefinition) string {
	name := d.EntityName()
	module := d.EntityModule()
	if strings.HasPrefix(name, module+"_") {
		return name // transitional: already qualified
	}
	return module + "_" + name
}

// LocalName returns the module-local entity name, stripping any module prefix.
//
// Examples:
//
//	Module="platform", Name="platform_organization" → "organization"
//	Module="iam",      Name="user"                  → "user"
func LocalName(d EntityDefinition) string {
	name := d.EntityName()
	module := d.EntityModule()
	prefix := module + "_"
	if strings.HasPrefix(name, prefix) {
		return strings.TrimPrefix(name, prefix)
	}
	return name
}

// HasModulePrefix reports whether the definition's Name contains the module
// prefix — indicating old-style naming that should be migrated to module-local.
func HasModulePrefix(d EntityDefinition) bool {
	return strings.HasPrefix(d.EntityName(), d.EntityModule()+"_")
}

// PluralizeLocal converts a snake_case local entity name to its plural form
// for use in URL resource paths.
//
// Examples:
//
//	"organization"      → "organizations"
//	"org_assignment"    → "org_assignments"
//	"entry"             → "entries"
//	"audit_log"         → "audit_logs"
func PluralizeLocal(localName string) string {
	if localName == "" {
		return ""
	}
	parts := strings.Split(localName, "_")
	parts[len(parts)-1] = pluralizeWord(parts[len(parts)-1])
	return strings.Join(parts, "_")
}

// pluralizeWord applies basic English pluralization rules to a single word.
func pluralizeWord(w string) string {
	if w == "" {
		return w
	}
	// Irregular forms.
	irregulars := map[string]string{
		"person": "people",
		"man":    "men",
		"child":  "children",
		"entry":  "entries",
		"policy": "policies",
	}
	if plural, ok := irregulars[w]; ok {
		return plural
	}
	// Words ending in "y" preceded by a consonant → drop "y" + "ies".
	if strings.HasSuffix(w, "y") && len(w) >= 2 {
		prev := w[len(w)-2]
		if !strings.ContainsRune("aeiou", rune(prev)) {
			return w[:len(w)-1] + "ies"
		}
	}
	// Words ending in sibilants or certain digraphs → add "es".
	for _, suffix := range []string{"sh", "ch", "ss", "zz"} {
		if strings.HasSuffix(w, suffix) {
			return w + "es"
		}
	}
	if strings.HasSuffix(w, "x") || strings.HasSuffix(w, "z") {
		return w + "es"
	}
	// Default: add "s".
	return w + "s"
}

// DeriveLabel converts a snake_case local name to a human-readable label.
//
// Examples:
//
//	"organization"   → "Organization"
//	"org_assignment" → "Org Assignment"
//	"user"           → "User"
func DeriveLabel(localName string) string {
	parts := strings.Split(localName, "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

// DerivePluralLabel derives a plural label from a singular label.
// Uses the same rules as PluralizeLocal applied to the last word.
//
// Examples:
//
//	"Organization"   → "Organizations"
//	"Org Assignment" → "Org Assignments"
func DerivePluralLabel(label string) string {
	if label == "" {
		return ""
	}
	parts := strings.Fields(label)
	parts[len(parts)-1] = pluralizeWord(strings.ToLower(parts[len(parts)-1]))
	// Re-capitalize the pluralized last word.
	last := parts[len(parts)-1]
	if len(last) > 0 {
		parts[len(parts)-1] = strings.ToUpper(last[:1]) + last[1:]
	}
	return strings.Join(parts, " ")
}

// OpenAPITag derives an OpenAPI tag from a module name.
//
// Examples:
//
//	"platform" → "Platform"
//	"iam"      → "Iam"
//	"finance"  → "Finance"
func OpenAPITag(module string) string {
	if module == "" {
		return ""
	}
	return strings.ToUpper(module[:1]) + module[1:]
}
