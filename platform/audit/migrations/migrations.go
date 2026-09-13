// Package migrations registers audit module schema migrations.
//
// Audit migrations create the unified audit event store:
//   - platform_audit_log          (partitioned append-only event log)
//   - platform_audit_checkpoint   (partition integrity checksum records)
//   - platform_audit_config       (runtime feature flags and configuration)
//   - platform_audit_migration_log (historical data migration progress)
//
// The audit_retention_role PostgreSQL role is also created here.
// It holds UPDATE + DELETE privileges on platform_audit_log for GDPR
// anonymization and archival operations.
//
// Import with a blank import to activate:
//
//	import _ "awo.so/awo/platform/audit/migrations"
package migrations

import (
	"embed"

	"awo.so/awo/migration"
)

//go:embed *.sql
var sqlFS embed.FS

func init() {
	migration.Register(migration.Source{
		Module:    "audit",
		Priority:  25,
		DependsOn: []string{"bootstrap"},
		FS:        sqlFS,
	})
}
