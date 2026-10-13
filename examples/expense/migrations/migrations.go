// Package migrations registers the business_expense showcase entity's schema
// migration.
//
// Priority 40 follows migration.Source's own documented convention ("40+ —
// custom application modules") — this module is a reference business module,
// not a framework-native one (see examples/expense/definition.go's package
// doc comment).
//
// Import with a blank import to activate:
//
//	import _ "awo.so/awo/examples/expense/migrations"
package migrations

import (
	"embed"

	"awo.so/awo/migration"
)

//go:embed *.sql
var sqlFS embed.FS

func init() {
	migration.Register(migration.Source{
		Module:    "expense",
		Priority:  40,
		DependsOn: []string{"bootstrap", "tenant", "iam"},
		FS:        sqlFS,
	})
}
