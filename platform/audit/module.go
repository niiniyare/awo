// Package audit implements the Awo platform Audit Log module.
//
// The audit log is append-only and tamper-evident. Every data mutation in the
// system produces an audit entry inside the same database transaction that
// caused the change — no mutation escapes the trail.
//
// The iam_audit_log entity is a system entity stored in the global schema with
// no RLS, so compliance and investigation queries can span tenants without
// context switching. Access requires the "iam.audit_log.read" permission.
//
// Retention: entries are never deleted by the application. Archival to cold
// storage is an ops concern, not a framework concern.
package audit

import "awo.so/awo/def"

func init() {
	def.Register(&LogDefinition)
}
