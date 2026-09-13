// Package flags provides the platform feature flags entity definitions.
//
// Feature flags control behaviour at three scopes:
//
//   - system: the default value, set at deployment time
//   - tenant: tenant-level override, set via admin UI without redeploy
//   - user:   per-user override for gradual rollouts and A/B testing
//
// Evaluation order: user override → tenant override → system default.
// Results are cached in Redis for 5 minutes (key: eval:{sha256}).
// Flag changes invalidate the cache immediately via DeletePrefix.
//
// The framework gates UI elements (absent from schema if off), API
// behaviour, and module activation on feature flags.
package flags

import (
	"awo.so/awo/def"
)

// FlagDefinition is the platform_feature_flag entity.
var FlagDefinition = def.SystemDefinition{
	Name:        "feature_flag",
	Module:      "platform",
	Label:       "Feature Flag",
	LabelPlural: "Feature Flags",

	Fields: []def.FieldDef{
		{
			Name:      "key",
			Type:      def.FieldTypeData,
			Label:     "Flag Key",
			Required:  true,
			Unique:    true,
			Immutable: true, // flag keys appear in code — rename is a refactor, not a data change
			MaxLen:    100,
		},
		{
			Name:   "label",
			Type:   def.FieldTypeData,
			Label:  "Label",
			MaxLen: 255,
		},
		{
			Name:  "description",
			Type:  def.FieldTypeSmallText,
			Label: "Description",
		},
		{
			Name:    "default_enabled",
			Type:    def.FieldTypeBool,
			Label:   "Default Enabled",
			Default: func() any { return false },
		},
		{
			Name:    "enabled",
			Type:    def.FieldTypeBool,
			Label:   "System Enabled",
			Default: func() any { return false },
		},
		{
			Name:    "rollout_percentage",
			Type:    def.FieldTypeInt,
			Label:   "Rollout %",
			Default: func() any { return 0 },
		},
	},

	Permissions: def.PermissionSet{
		// Permission identifiers follow "platform.feature_flag.{operation}".
		// Role-to-permission mappings seeded in iam_role_permissions:
		//   role:platform-admin → platform.feature_flag.{create,read,update,delete}
		//   role:tenant.admin   → platform.feature_flag.read
		Create: []string{"platform.feature_flag.create"},
		Read:   []string{"platform.feature_flag.read"},
		Write:  []string{"platform.feature_flag.update"},
		Delete: []string{"platform.feature_flag.delete"},
	},
}

// TenantOverrideDefinition is the platform_flag_tenant_override entity.
// Stores per-tenant flag overrides. Absence means use the system default.
var TenantOverrideDefinition = def.SystemDefinition{
	Name:        "flag_tenant_override",
	Module:      "platform",
	Label:       "Tenant Flag Override",
	LabelPlural: "Tenant Flag Overrides",

	Fields: []def.FieldDef{
		{
			Name:       "flag_id",
			Type:       def.FieldTypeLink,
			Label:      "Feature Flag",
			LinkTarget: "platform_feature_flag",
			Required:   true,
			Immutable:  true,
		},
		{
			Name:     "enabled",
			Type:     def.FieldTypeBool,
			Label:    "Enabled",
			Required: true,
		},
	},

	Permissions: def.PermissionSet{
		// Permission identifiers follow "platform.flag_tenant_override.{operation}".
		// Role-to-permission mappings seeded in iam_role_permissions:
		//   role:platform-admin → platform.flag_tenant_override.{create,read,update,delete}
		//   role:tenant.admin   → platform.flag_tenant_override.{create,read,update,delete}
		Create: []string{"platform.flag_tenant_override.create"},
		Read:   []string{"platform.flag_tenant_override.read"},
		Write:  []string{"platform.flag_tenant_override.update"},
		Delete: []string{"platform.flag_tenant_override.delete"},
	},
}

func init() {
	def.Register(&FlagDefinition)
	def.Register(&TenantOverrideDefinition)
}
