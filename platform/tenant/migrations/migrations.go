// Package migrations registers platform_tenant schema migrations.
//
// Import with a blank import to activate:
//
//	import _ "awo.so/awo/platform/tenant/migrations"
package migrations

import (
	"embed"

	"awo.so/awo/migration"
)

//go:embed *.sql
var sqlFS embed.FS

func init() {
	migration.Register(migration.Source{
		Module:    "tenant",
		Priority:  10,
		DependsOn: []string{"bootstrap"},
		FS:        sqlFS,
	})
}
