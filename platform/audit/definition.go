// Package audit provides the platform audit log entity definition.
//
// The audit log is append-only and tamper-evident. Every data mutation
// in the system produces an audit entry inside the same database transaction
// that caused the change. No mutation escapes the audit trail.
//
// The iam_audit_log entity is a system entity stored in the global schema
// (no RLS) so that compliance and investigation queries can span tenants
// without context switching. Access is gated by the "iam.audit_log.read"
// permission; role-to-permission mapping is seeded in iam_role_permissions.
//
// Retention: entries are never deleted by the application. Archival to cold
// storage is an ops concern, not a framework concern.
package audit

import (
	"awo.so/awo/def"
)

// LogDefinition is the iam_audit_log entity definition.
var LogDefinition = def.SystemDefinition{
	Name:        "audit_log",
	Module:      "iam",
	Label:       "Audit Log",
	LabelPlural: "Audit Logs",

	Fields: []def.FieldDef{
		{
			Name:   "tenant_id_ref",
			Type:   def.FieldTypeData, // not a Link — audit log spans all tenants
			Label:  "Tenant ID",
			MaxLen: 36,
		},
		{
			Name:   "actor_id",
			Type:   def.FieldTypeData,
			Label:  "Actor ID",
			MaxLen: 36,
		},
		{
			Name:   "actor_email",
			Type:   def.FieldTypeData,
			Label:  "Actor Email",
			MaxLen: 254,
		},
		{
			Name:   "ip_address",
			Type:   def.FieldTypeData,
			Label:  "IP Address",
			MaxLen: 45,
		},
		{
			Name:     "operation",
			Type:     def.FieldTypeSelect,
			Label:    "Operation",
			Options:  []string{"create", "update", "delete", "action", "login", "logout"},
			Required: true,
		},
		{
			Name:     "entity_name",
			Type:     def.FieldTypeData,
			Label:    "Entity",
			Required: true,
			MaxLen:   100,
		},
		{
			Name:   "record_id",
			Type:   def.FieldTypeData,
			Label:  "Record ID",
			MaxLen: 36,
		},
		{
			Name:  "before_snapshot",
			Type:  def.FieldTypeJSON,
			Label: "Before",
		},
		{
			Name:  "after_snapshot",
			Type:  def.FieldTypeJSON,
			Label: "After",
		},
		{
			Name:  "diff",
			Type:  def.FieldTypeJSON,
			Label: "Diff",
		},
		{
			Name:   "request_id",
			Type:   def.FieldTypeData,
			Label:  "Request ID",
			MaxLen: 36,
		},
	},

	Permissions: def.PermissionSet{
		// Permission identifiers follow the format "iam.audit_log.{operation}".
		// The entity's Module is "iam", so its qualified name is "iam_audit_log".
		// Role-to-permission mappings are seeded in iam_role_permissions:
		//   role:platform-admin → iam.audit_log.read
		//   role:tenant.admin   → iam.audit_log.read  (row-level filter by tenant_id_ref applies)
		Create: []string{}, // written by framework hooks only — no API-level create
		Read:   []string{"iam.audit_log.read"},
		Write:  []string{}, // immutable
		Delete: []string{}, // never deleted
	},
}
