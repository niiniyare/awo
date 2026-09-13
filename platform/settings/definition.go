// Package settings provides hierarchical configuration entity definitions.
//
// Settings are evaluated in order: system default → tenant override → branch override.
// The "branch" scope is used by multi-branch tenants to set location-specific config
// (e.g. different naming series prefix per branch).
//
// Typical use cases:
//   - Naming series prefix per tenant (INV- vs ZINV-)
//   - Tax rate overrides
//   - Working hours per branch
//   - Module-specific configuration (e.g. email templates)
//
// Settings are cached in Redis (5 minute TTL). Changes invalidate the cache
// for the affected scope immediately.
package settings

import (
	"awo.so/awo/def"
)

// SettingDefinition is the platform_setting entity.
// Settings are key-value pairs scoped to a namespace (module.key pattern).
var SettingDefinition = def.SystemDefinition{
	Name:        "setting",
	Module:      "platform",
	Label:       "Setting",
	LabelPlural: "Settings",

	Fields: []def.FieldDef{
		{
			Name:      "namespace",
			Type:      def.FieldTypeData,
			Label:     "Namespace",
			Required:  true,
			Immutable: true,
			MaxLen:    100,
			// Format: "module.subsystem" e.g. "finance.naming_series"
		},
		{
			Name:      "key",
			Type:      def.FieldTypeData,
			Label:     "Key",
			Required:  true,
			Immutable: true,
			MaxLen:    100,
		},
		{
			Name:      "scope",
			Type:      def.FieldTypeSelect,
			Label:     "Scope",
			Options:   []string{"system", "tenant", "branch"},
			Default:   func() any { return "tenant" },
			Immutable: true,
		},
		{
			Name:   "scope_ref",
			Type:   def.FieldTypeData,
			Label:  "Scope Reference",
			MaxLen: 36,
			// For tenant scope: tenant UUID. For branch: branch UUID. System: empty.
		},
		{
			Name:  "value",
			Type:  def.FieldTypeJSON,
			Label: "Value",
		},
		{
			Name:    "value_type",
			Type:    def.FieldTypeSelect,
			Label:   "Value Type",
			Options: []string{"string", "number", "boolean", "json"},
			Default: func() any { return "string" },
		},
		{
			Name:  "description",
			Type:  def.FieldTypeSmallText,
			Label: "Description",
		},
	},

	Permissions: def.PermissionSet{
		// Permission identifiers follow "platform.setting.{operation}".
		// Role-to-permission mappings seeded in iam_role_permissions:
		//   role:platform-admin → platform.setting.{create,read,update,delete}
		//   role:tenant.admin   → platform.setting.{create,read,update}
		//   role:tenant.user    → platform.setting.read
		Create: []string{"platform.setting.create"},
		Read:   []string{"platform.setting.read"},
		Write:  []string{"platform.setting.update"},
		Delete: []string{"platform.setting.delete"},
	},
}
