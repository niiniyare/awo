// Package metadata provides runtime schema extension — the ability for tenants
// to add custom fields to any entity without migrations or redeployment.
//
// Custom fields are stored in the custom_fields JSONB column on every system
// entity table, and in the main data column for custom entities. The field
// definitions live in the platform_custom_field entity.
//
// API, SDUI page generation, and the Filter DSL treat custom fields
// identically to system fields — the metadata service merges them into the
// resolved entity schema at request time.
package metadata

import (
	"awo.so/awo/def"
)

// CustomFieldDefinition is the platform_custom_field entity.
// One record = one tenant-defined extension field on one entity.
var CustomFieldDefinition = def.SystemDefinition{
	Name:        "custom_field",
	Module:      "platform",
	Label:       "Custom Field",
	LabelPlural: "Custom Fields",

	Fields: []def.FieldDef{
		{
			Name:      "entity_name",
			Type:      def.FieldTypeData,
			Label:     "Entity",
			Required:  true,
			Immutable: true,
			MaxLen:    100,
		},
		{
			Name:      "field_name",
			Type:      def.FieldTypeData,
			Label:     "Field Name",
			Required:  true,
			Immutable: true,
			MaxLen:    100,
			// Convention: must start with "cf_" to avoid collisions with system fields.
		},
		{
			Name:   "label",
			Type:   def.FieldTypeData,
			Label:  "Label",
			MaxLen: 255,
		},
		{
			Name:     "field_type",
			Type:     def.FieldTypeSelect,
			Label:    "Field Type",
			Required: true,
			Options: []string{
				"data", "small_text", "long_text",
				"int", "float", "currency",
				"bool", "date", "datetime", "time",
				"select", "json",
			},
		},
		{
			Name:  "options",
			Type:  def.FieldTypeJSON,
			Label: "Select Options",
			// Non-null only when field_type == "select". Array of strings.
		},
		{
			Name:    "required",
			Type:    def.FieldTypeBool,
			Label:   "Required",
			Default: func() any { return false },
		},
		{
			Name:  "default_value",
			Type:  def.FieldTypeJSON,
			Label: "Default Value",
		},
		{
			Name:    "active",
			Type:    def.FieldTypeBool,
			Label:   "Active",
			Default: func() any { return true },
		},
		{
			Name:    "sort_order",
			Type:    def.FieldTypeInt,
			Label:   "Sort Order",
			Default: func() any { return 0 },
		},
	},

	Hooks: def.HookSet{
		BeforeCreate: []def.BeforeCreateHook{&FieldNameValidator{}},
	},

	Permissions: def.PermissionSet{
		// Permission identifiers follow "platform.custom_field.{operation}".
		// Role-to-permission mappings seeded in iam_role_permissions:
		//   role:platform-admin → platform.custom_field.{create,read,update,delete}
		//   role:tenant.admin   → platform.custom_field.{create,read,update,delete}
		//   role:tenant.user    → platform.custom_field.read
		Create: []string{"platform.custom_field.create"},
		Read:   []string{"platform.custom_field.read"},
		Write:  []string{"platform.custom_field.update"},
		Delete: []string{"platform.custom_field.delete"},
	},
}

func init() {
	def.Register(&CustomFieldDefinition)
}
