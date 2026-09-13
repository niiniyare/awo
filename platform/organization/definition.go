// Package organization provides the platform Organization module.
//
// # Isolation model
//
// Awo has two independent isolation layers:
//
//  1. Tenant Isolation (framework) — enforced by PostgreSQL RLS.
//     Every row in every table carries tenant_id. The framework guarantees
//     WHERE tenant_id = current_tenant_id() and nothing more.
//
//  2. Organization Scope (application) — evaluated entirely in Go.
//     OrganizationService.ResolveScope() returns the set of org IDs
//     visible to the current user. Application services pass that set
//     as an explicit IN predicate to the repository. Organization nodes
//     are never part of RLS policies.
//
// These layers are orthogonal and must never be conflated.
//
// # Entities
//
//   - platform_organization — org tree node (arbitrary depth, materialized path)
//   - platform_org_type     — tenant-defined type registry (metadata-driven)
//   - platform_org_assignment — user ↔ organization membership with role
//
// All three entities are SystemDefinitions: they are FK targets for IAM and
// business modules, and must be accessible before per-tenant schemas load.
package organization

import "awo.so/awo/def"

// Definition is the SystemDefinition for the platform_organization entity.
//
// Organizations form an arbitrary-depth tree within a tenant. The framework
// does not assume any fixed type values (company, branch, department…) — types
// are registered by tenant admins via OrganizationTypeDefinition.
//
// Visibility is evaluated by OrganizationService.ResolveScope(), not by RLS.
var Definition = def.SystemDefinition{
	Name:        "organization",
	Module:      "platform",
	Label:       "Organization",
	LabelPlural: "Organizations",

	Fields: []def.FieldDef{
		{
			Name:       "name",
			Type:       def.FieldTypeData,
			Label:      "Name",
			Required:   true,
			Searchable: true,
			MaxLen:     255,
		},
		{
			// code is immutable: used in workflow IDs, Redis keys, audit records.
			Name:      "code",
			Type:      def.FieldTypeData,
			Label:     "Code",
			Required:  true,
			Unique:    true,
			Immutable: true,
			MaxLen:    50,
		},
		{
			// tenant_id is the infrastructure isolation boundary — always present.
			Name:       "tenant_id",
			Type:       def.FieldTypeLink,
			Label:      "Tenant",
			LinkTarget: "platform_tenant",
			Required:   true,
			Immutable:  true,
		},
		{
			// parent_id is nil for root nodes (top-level org in a tenant).
			Name:       "parent_id",
			Type:       def.FieldTypeLink,
			Label:      "Parent",
			LinkTarget: "platform_organization",
		},
		{
			// type references platform_org_type.name — not a fixed enum.
			// Extensible by tenant admins without schema migration.
			Name:   "type",
			Type:   def.FieldTypeData,
			Label:  "Type",
			MaxLen: 50,
		},
		{
			// path is a materialized path: "/root-id/parent-id/self-id/".
			// Computed by PathComputeHook on create; updated by MoveService.
			Name:      "path",
			Type:      def.FieldTypeData,
			Label:     "Path",
			Immutable: false, // updated by Move — cannot be set by API callers
			MaxLen:    4096,
		},
		{
			// depth is 0 for root nodes, n for depth-n nodes.
			// Derived from path but stored for cheap depth-limit enforcement.
			Name:    "depth",
			Type:    def.FieldTypeInt,
			Label:   "Depth",
			Default: func() any { return 0 },
		},
		{
			Name:    "active",
			Type:    def.FieldTypeBool,
			Label:   "Active",
			Default: func() any { return true },
		},
		{
			Name:  "description",
			Type:  def.FieldTypeSmallText,
			Label: "Description",
		},
	},

	Hooks: def.HookSet{
		BeforeCreate: []def.BeforeCreateHook{&CodeValidator{}, &PathComputeHook{}},
		BeforeUpdate: []def.BeforeUpdateHook{&MoveGuard{}},
	},

	Permissions: def.PermissionSet{
		// Role-to-permission mappings seeded in iam_role_permissions:
		//   role:platform-admin → platform.organization.{create,read,update,delete}
		//   role:tenant.admin   → platform.organization.{create,read,update,delete}
		//   role:tenant.user    → platform.organization.read
		Create: []string{"platform.organization.create"},
		Read:   []string{"platform.organization.read"},
		Write:  []string{"platform.organization.update"},
		Delete: []string{"platform.organization.delete"},
	},
}

// OrgTypeDefinition is the SystemDefinition for platform_org_type.
//
// Tenant admins register valid organization type names here. The framework
// does not enforce any predefined type list — types are fully metadata-driven.
// Common registrations: company, holding, subsidiary, division, department,
// region, territory, branch, team, store, warehouse, cost_centre.
var OrgTypeDefinition = def.SystemDefinition{
	Name:        "org_type",
	Module:      "platform",
	Label:       "Organization Type",
	LabelPlural: "Organization Types",

	Fields: []def.FieldDef{
		{
			// name is the machine-readable identifier stored in platform_organization.type.
			Name:      "name",
			Type:      def.FieldTypeData,
			Label:     "Name",
			Required:  true,
			Unique:    true,
			Immutable: true, // referenced in org records; rename = data migration
			MaxLen:    50,
		},
		{
			Name:     "label",
			Type:     def.FieldTypeData,
			Label:    "Label",
			Required: true,
			MaxLen:   255,
		},
		{
			Name:  "description",
			Type:  def.FieldTypeSmallText,
			Label: "Description",
		},
		{
			Name:    "sort_order",
			Type:    def.FieldTypeInt,
			Label:   "Sort Order",
			Default: func() any { return 0 },
		},
		{
			Name:    "active",
			Type:    def.FieldTypeBool,
			Label:   "Active",
			Default: func() any { return true },
		},
	},

	Permissions: def.PermissionSet{
		// Role-to-permission mappings seeded in iam_role_permissions:
		//   role:platform-admin → platform.org_type.{create,read,update,delete}
		//   role:tenant.admin   → platform.org_type.{create,read,update,delete}
		//   role:tenant.user    → platform.org_type.read
		Create: []string{"platform.org_type.create"},
		Read:   []string{"platform.org_type.read"},
		Write:  []string{"platform.org_type.update"},
		Delete: []string{"platform.org_type.delete"},
	},
}

// OrgAssignmentDefinition is the SystemDefinition for platform_org_assignment.
//
// Records which users belong to which organizations and in what role. Users
// may belong to multiple organizations simultaneously. Exactly one assignment
// per user per tenant may carry is_primary=true (enforced by unique partial index).
//
// Organization roles are distinct from IAM roles:
//   - IAM role (iam_user_role): platform-wide capabilities (e.g. tenant.admin)
//   - Org role (platform_org_assignment.role): position within an org node
//     (e.g. manager, member, viewer, auditor)
var OrgAssignmentDefinition = def.SystemDefinition{
	Name:        "org_assignment",
	Module:      "platform",
	Label:       "Organization Assignment",
	LabelPlural: "Organization Assignments",

	Fields: []def.FieldDef{
		{
			Name:       "tenant_id",
			Type:       def.FieldTypeLink,
			Label:      "Tenant",
			LinkTarget: "platform_tenant",
			Required:   true,
			Immutable:  true,
		},
		{
			Name:       "user_id",
			Type:       def.FieldTypeLink,
			Label:      "User",
			LinkTarget: "iam_user",
			Required:   true,
			Immutable:  true,
		},
		{
			Name:       "organization_id",
			Type:       def.FieldTypeLink,
			Label:      "Organization",
			LinkTarget: "platform_organization",
			Required:   true,
			Immutable:  true,
		},
		{
			// role is the user's position within this organization node.
			// Not a fixed enum — tenant-defined (e.g. manager, member, viewer).
			Name:    "role",
			Type:    def.FieldTypeData,
			Label:   "Role",
			MaxLen:  50,
			Default: func() any { return "member" },
		},
		{
			// is_primary marks the user's home organization.
			// At most one primary assignment per user per tenant (partial unique index).
			Name:    "is_primary",
			Type:    def.FieldTypeBool,
			Label:   "Primary",
			Default: func() any { return false },
		},
	},

	Permissions: def.PermissionSet{
		// Role-to-permission mappings seeded in iam_role_permissions:
		//   role:platform-admin → platform.org_assignment.{create,read,update,delete}
		//   role:tenant.admin   → platform.org_assignment.{create,read,update,delete}
		//   role:tenant.user    → platform.org_assignment.read
		Create: []string{"platform.org_assignment.create"},
		Read:   []string{"platform.org_assignment.read"},
		Write:  []string{"platform.org_assignment.update"},
		Delete: []string{"platform.org_assignment.delete"},
	},
}
