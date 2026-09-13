// Package tenant provides the platform Tenant entity definition.
//
// The Tenant entity is a mandatory system entity — it is accessible before
// per-tenant schemas load and cannot be declared as a CustomDefinition.
// Tenant records are stored in the global (non-RLS) schema and are readable
// by the platform_reader role only.
//
// platform_tenant is an infrastructure boundary only. Organisational hierarchy
// (divisions, departments, branches, teams) lives in the platform/organization
// package — see awo.so/awo/platform/organization.
//
// Lifecycle state machine:
//
//	PENDING → ACTIVE → SUSPENDED → ACTIVE   (payment resolved)
//	PENDING → ARCHIVED                        (abandoned)
//	ACTIVE  → ARCHIVED                        (account deletion)
//	SUSPENDED → ARCHIVED                      (grace period expired)
//
// ARCHIVED is terminal. Transitions are enforced by BeforeUpdate hooks.
package tenant

import (
	"awo.so/awo/def"
)

// Definition is the SystemDefinition for the platform_tenant entity.
// Registered in init() — available immediately at process start.
var Definition = def.SystemDefinition{
	Name:        "tenant",
	Module:      "platform",
	Label:       "Tenant",
	LabelPlural: "Tenants",

	Fields: []def.FieldDef{
		{
			Name:     "name",
			Type:     def.FieldTypeData,
			Label:    "Name",
			Required: true,
			MaxLen:   255,
		},
		{
			Name:      "slug",
			Type:      def.FieldTypeData,
			Label:     "Slug",
			Required:  true,
			Unique:    true,
			Immutable: true, // slug is embedded in URLs and workflow IDs
			MaxLen:    63,
		},
		{
			Name:    "status",
			Type:    def.FieldTypeSelect,
			Label:   "Status",
			Options: []string{"PENDING", "ACTIVE", "SUSPENDED", "ARCHIVED"},
			Default: func() any { return "PENDING" },
		},
		{
			Name:    "plan",
			Type:    def.FieldTypeSelect,
			Label:   "Subscription Plan",
			Options: []string{"free", "starter", "growth", "enterprise"},
			Default: func() any { return "free" },
		},
		{
			Name:   "country",
			Type:   def.FieldTypeData,
			Label:  "Country Code",
			MaxLen: 2,
		},
		{
			Name:    "locale",
			Type:    def.FieldTypeData,
			Label:   "Locale",
			MaxLen:  10,
			Default: func() any { return "en-KE" },
		},
		{
			Name:    "timezone",
			Type:    def.FieldTypeData,
			Label:   "Timezone",
			MaxLen:  64,
			Default: func() any { return "Africa/Nairobi" },
		},
		{
			Name:    "currency",
			Type:    def.FieldTypeData,
			Label:   "Currency Code",
			MaxLen:  3,
			Default: func() any { return "KES" },
		},
		{
			Name:     "contact_email",
			Type:     def.FieldTypeData,
			Label:    "Contact Email",
			Required: true,
			MaxLen:   254,
		},
		{
			Name:   "contact_phone",
			Type:   def.FieldTypeData,
			Label:  "Contact Phone",
			MaxLen: 30,
		},
		{
			Name:      "trial_ends_at",
			Type:      def.FieldTypeDateTime,
			Label:     "Trial Ends At",
			Sensitive: true,
		},
		{
			Name:      "suspended_at",
			Type:      def.FieldTypeDateTime,
			Label:     "Suspended At",
			Sensitive: true,
		},
		{
			Name:      "suspension_reason",
			Type:      def.FieldTypeSmallText,
			Label:     "Suspension Reason",
			Sensitive: true,
		},
	},

	Hooks: def.HookSet{
		BeforeCreate: []def.BeforeCreateHook{&SlugValidator{}, &StatusValidator{}},
		BeforeUpdate: []def.BeforeUpdateHook{&TransitionGuard{}},
	},

	Permissions: def.PermissionSet{
		// Permission identifiers follow the format "platform.tenant.{operation}".
		// Role-to-permission mappings are seeded in iam_role_permissions:
		//   role:platform-admin → platform.tenant.{create,read,update}
		//   role:tenant.admin   → platform.tenant.read
		Create: []string{"platform.tenant.create"},
		Read:   []string{"platform.tenant.read"},
		Write:  []string{"platform.tenant.update"},
		Delete: []string{}, // tenants are never hard-deleted — use ARCHIVED transition
	},
}
