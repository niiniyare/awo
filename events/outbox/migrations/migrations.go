// Package migrations registers the events_outbox schema migration.
//
// events_outbox is the transactional outbox table events/outbox.OutboxWriter
// writes to (inside the caller's mutation transaction, Step 1) and
// events/outbox.Relay reads from (Step 3B). It is framework-native
// infrastructure, not tied to any single platform module, and has no
// dependency on tenant/iam data (no foreign key to platform_tenant --
// tenant_id must remain valid even after a tenant referenced by an event is
// later hard-deleted; see ADR-025 §16 row 15). It depends only on bootstrap,
// consistent with every other module's registration.
//
// Import with a blank import to activate:
//
//	import _ "awo.so/awo/events/outbox/migrations"
package migrations

import (
	"embed"

	"awo.so/awo/migration"
)

//go:embed *.sql
var sqlFS embed.FS

func init() {
	migration.Register(migration.Source{
		Module:    "outbox",
		Priority:  5,
		DependsOn: []string{"bootstrap"},
		FS:        sqlFS,
	})
}

// SQLFS exposes this module's embedded migration files for tests and tooling
// that need to apply or inspect the exact, production migration SQL directly
// -- e.g. integration tests that must exercise the real migration-defined
// events_outbox schema rather than maintaining a second, hand-written copy
// of it (see contrib/pgx/outbox_writer_transaction_test.go). This mirrors
// migration.BuildFSFrom's own stated purpose ("Intended for use in tests and
// tooling") one level down, for callers that want the raw source files
// rather than the renamed/versioned virtual FS BuildFSFrom produces.
func SQLFS() embed.FS {
	return sqlFS
}
