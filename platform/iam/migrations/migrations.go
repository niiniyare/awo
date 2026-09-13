// Package migrations registers IAM module schema migrations.
//
// IAM migrations create the core identity tables:
//   - iam_user           (authenticated human principals)
//   - iam_user_role      (role assignments)
//   - iam_service_account (machine principals)
//   - iam_api_token      (long-lived API keys)
//   - iam_session        (durable session audit trail)
//   - iam_login_audit    (tamper-evident authentication event log)
//   - casbin_rule        (Casbin RBAC policy storage)
//
// Import with a blank import to activate:
//
//	import _ "awo.so/awo/platform/iam/migrations"
package migrations

import (
	"embed"

	"awo.so/awo/migration"
)

//go:embed *.sql
var sqlFS embed.FS

func init() {
	migration.Register(migration.Source{
		Module:    "iam",
		Priority:  20,
		DependsOn: []string{"bootstrap", "tenant"},
		FS:        sqlFS,
	})
}
