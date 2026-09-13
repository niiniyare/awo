// Package bootstrap registers the AWO framework bootstrap migrations.
//
// Bootstrap migrations must run before all other module migrations. They install
// PostgreSQL extensions (uuid-ossp, pgcrypto, btree_gist) and shared utility
// functions (current_tenant_id, set_updated_at, next_naming_series) that every
// other module depends on.
//
// Import this package with a blank import to register bootstrap migrations:
//
//	import _ "awo.so/awo/migration/bootstrap"
package bootstrap

import (
	"embed"

	"awo.so/awo/migration"
)

//go:embed *.sql
var sqlFS embed.FS

func init() {
	migration.Register(migration.Source{
		Module:    "bootstrap",
		Priority:  0,
		DependsOn: nil, // bootstrap has no dependencies; it IS the foundation
		FS:        sqlFS,
	})
}
