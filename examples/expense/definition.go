// Package expense provides the business_expense entity — Awo's first
// showcase business module.
//
// This is a REFERENCE/DEMO module, not framework-native, matching Finance's
// own documented status (see docs/00-overview/ARCH_OVERVIEW.md §10 "Platform
// Modules", which lists exactly seven framework-native modules — this is not
// one of them). It lives at the repository root under examples/ rather than
// platform/ specifically to keep that boundary visible: a real application
// repository would depend on awo.so/awo and provide its own business modules
// the same way this package does, outside the framework's own package tree.
//
// # Purpose
//
// business_expense exists to prove, end-to-end and against real PostgreSQL,
// that a small amount of declarative Go code gets a business module: tenant
// isolation (RLS), a generated CRUD API, default permission enforcement,
// mandatory audit, and a minimal explicit status-lifecycle guard — with zero
// hand-written SQL beyond the one migration this package owns and registers
// itself (see migrations/migrations.go), and zero hand-written API/routing
// code (api/router.Register's existing generic per-entity loop handles that
// automatically once this package is registered — see init() below and the
// blank import in cmd/server/main.go).
//
// # Lifecycle
//
//	draft -> submitted -> approved
//	                   \-> rejected -> draft (resubmit)
//
// Enforced by StatusTransitionGuard (hooks.go), mirroring the existing
// platform/organization.MoveGuard pattern (a BeforeUpdate hook rejecting an
// invalid field transition) — not a new workflow subsystem.
package expense

import (
	"awo.so/awo/def"
)

// Definition is the business_expense entity.
var Definition = def.SystemDefinition{
	Name:        "expense",
	Module:      "business",
	Label:       "Expense",
	LabelPlural: "Expenses",
	Description: "An employee-submitted business expense, tracked from draft through approval.",

	Fields: []def.FieldDef{
		{
			Name:       "description",
			Type:       def.FieldTypeData,
			Label:      "Description",
			Required:   true,
			Searchable: true,
			MaxLen:     255,
		},
		{
			Name:     "amount",
			Type:     def.FieldTypeCurrency,
			Label:    "Amount",
			Required: true,
		},
		{
			// category is a fixed, small set for the showcase. A real module
			// would likely make this tenant-extensible (see
			// platform/organization.OrgTypeDefinition for that pattern) —
			// deliberately not done here to keep this showcase small.
			Name:     "category",
			Type:     def.FieldTypeSelect,
			Label:    "Category",
			Options:  []string{"travel", "meals", "supplies", "other"},
			Required: true,
		},
		{
			// status drives StatusTransitionGuard (hooks.go). Not Immutable —
			// the guard, not field-level immutability, is what constrains it.
			Name:    "status",
			Type:    def.FieldTypeSelect,
			Label:   "Status",
			Options: []string{"draft", "submitted", "approved", "rejected"},
			Default: func() any { return "draft" },
		},
		{
			// submitted_by is the employee who owns this expense. Set once at
			// creation and never changed — matches iam_user/org FK convention
			// elsewhere in the codebase (e.g. platform/organization's
			// parent_id, platform/notification's recipient_id).
			Name:       "submitted_by",
			Type:       def.FieldTypeLink,
			Label:      "Submitted By",
			LinkTarget: "iam_user",
			Required:   true,
			Immutable:  true,
		},
		{
			Name:  "notes",
			Type:  def.FieldTypeLongText,
			Label: "Notes",
		},
	},

	Hooks: def.HookSet{
		BeforeUpdate: []def.BeforeUpdateHook{&StatusTransitionGuard{}},
	},

	Permissions: def.PermissionSet{
		// Role-to-permission mappings (seeded in iam_role_permissions, same
		// convention as every other platform/business entity in this repo —
		// see e.g. platform/organization/definition.go):
		//   role:tenant.user  -> business.expense.{create,read,update}
		//   role:tenant.admin -> business.expense.{create,read,update,delete}
		Create: []string{"business.expense.create"},
		Read:   []string{"business.expense.read"},
		Write:  []string{"business.expense.update"},
		Delete: []string{"business.expense.delete"},
	},
}

func init() {
	def.Register(&Definition)
}
