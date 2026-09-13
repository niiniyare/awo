// Package iam — queries.go
//
// This file centralises every raw SQL statement used by AuthService.
// All SQL lives here and nowhere else in the iam package.
//
// # Why raw SQL still exists after Phase N+6C
//
// Phase N+6C migrated the following operations to EntityRepository:
//
//   - sqlInsertSessionRecord → Repos.Sessions.Create (auditLogin)
//   - sqlUpdateLastLoginAt   → Repos.Users.Update   (auditLogin)
//   - sqlRevokeSessionByHash    → Repos.Sessions.BulkUpdate (auditLogout)
//   - sqlRevokeSessionsByHashes → Repos.Sessions.BulkUpdate (auditRevoke)
//   - sqlLoadUserRoles          → Repos.UserRoles.Query     (loadUserRoles)
//
// Three statements cannot be expressed through EntityRepository and remain raw
// SQL. Each is a candidate for Phase N+6D DatabaseQuerier:
//
//  1. [sqlLookupCredentials] — Sensitive field access. [password_hash] is
//     declared Sensitive:true in UserDefinition. EntityRepository strips
//     sensitive fields from all query results. Credential verification requires
//     the raw bcrypt hash. Exposing sensitive fields through EntityRepository
//     would require a general security bypass — that is not acceptable.
//     Replacement: a dedicated DatabaseQuerier.LookupCredentials method in N+6D.
//
//  2. [sqlLookupAPIToken] — Cross-entity join. Spans iam_api_tokens and
//     iam_service_accounts. EntityRepository is scoped to a single entity type;
//     cross-entity joins are not expressible through its interface. Splitting
//     this into two sequential EntityRepository calls would introduce a TOCTOU
//     risk (service account status could change between reads) and double the
//     latency on a hot-path validation flow.
//     Replacement: DatabaseQuerier.LookupAPIToken in N+6D.
//
//  3. [sqlLoadRolePermissions] — Global bootstrap table. iam_role_permissions
//     is a global table (no tenant_id, no RLS) read at startup to initialise
//     CasbinEvaluator. Registering it as an EntityDefinition purely for
//     EntityRepository access would create unnecessary public API surface for
//     an internal bootstrap table. This table should remain outside the entity
//     lifecycle.
//     Replacement: None needed — this should stay as raw SQL permanently.
//
// [sqlSetTenantContext] is retained for use in the credential and API-token
// raw transactions (Login and ValidateAPIToken). Once those migrate to
// EntityRepository or DatabaseQuerier in N+6D, this constant can be removed.
//
// # Why set_tenant_context is called explicitly
//
// Every tenant-scoped operation must call set_tenant_context($1) at the start
// of the transaction to activate PostgreSQL Row Level Security for that
// connection. EntityRepository.WithTx calls set_tenant_context automatically.
// AuthService raw transactions (credential lookup, API-token join) must call
// it explicitly here. When those operations migrate to DatabaseQuerier in N+6D,
// these explicit calls will be eliminated.
//
// # SQL safety
//
// All statements use positional parameters ($1, $2 …). No SQL is constructed
// by string concatenation. All statements have been reviewed for injection risk.
package iam

// ── Tenant context ─────────────────────────────────────────────────────────────

// sqlSetTenantContext activates PostgreSQL Row Level Security for the current
// transaction by setting the transaction-local configuration variable
// app.current_tenant_id. Called at the start of every raw tenant-scoped
// transaction (credential lookup, API-token join).
//
// EntityRepository.WithTx calls this internally. Once the remaining raw
// transactions migrate to DatabaseQuerier in N+6D, this call will be
// eliminated from AuthService entirely.
//
// Limitation: PostgreSQL stored procedure call — no Go equivalent.
const sqlSetTenantContext = `SELECT set_tenant_context($1)`

// ── Authentication ─────────────────────────────────────────────────────────────

// sqlLookupCredentials retrieves the fields required for bcrypt password
// verification for a user identified by email within the current RLS tenant
// context.
//
// Why SQL: EntityRepository strips [password_hash] because UserDefinition
// declares it Sensitive:true. Credential verification requires the raw hash.
// Exposing sensitive fields through EntityRepository would require a general
// security bypass — that is not acceptable.
//
// N+6D replacement: DatabaseQuerier.LookupCredentials.
//
// Limitation: Security — sensitive field policy in EntityRepository.
const sqlLookupCredentials = `
	SELECT id, password_hash, status
	FROM iam_users
	WHERE email = $1
`

// ── API token validation ───────────────────────────────────────────────────────

// sqlLookupAPIToken retrieves service account identity and token metadata for
// a given token_hash. Joins iam_api_tokens and iam_service_accounts in a
// single query to avoid two round-trips on the hot-path validation flow.
//
// Why SQL: This is a cross-entity join. EntityRepository is scoped to a single
// entity type — iam_api_token or iam_service_account, not both together.
// Performing two separate EntityRepository queries and joining in Go would
// introduce a TOCTOU risk (service account status could change between reads)
// and double the latency on a hot path.
//
// N+6D replacement: DatabaseQuerier.LookupAPIToken.
//
// Limitation: Framework — EntityRepository does not support cross-entity joins.
// This is a deliberate framework boundary. The join should stay in an adapter
// (contrib/pgx) behind a DatabaseQuerier interface, not be broken into two
// sequential EntityRepository calls.
const sqlLookupAPIToken = `
	SELECT
		at.service_account_id,
		at.expires_at,
		at.is_revoked,
		sa.status
	FROM iam_api_tokens at
	JOIN iam_service_accounts sa ON sa.id = at.service_account_id
	WHERE at.token_hash = $1
`

// ── Roles ──────────────────────────────────────────────────────────────────────

// sqlLoadRolePermissions loads all role→permission bindings from the global
// iam_role_permissions table. Called at startup to initialise CasbinEvaluator
// and after any permission assignment change.
//
// iam_role_permissions is a global table — no tenant_id, no RLS. The query is
// issued directly on the pool (no transaction, no set_tenant_context).
//
// Why SQL: iam_role_permissions is not a registered EntityDefinition. It is a
// bootstrap table whose contents must be loaded before the full entity pipeline
// is available at startup. Registering it as an entity purely for EntityRepository
// access would create unnecessary public API surface for tables that are
// intentionally internal. This table should remain raw SQL permanently.
//
// N+6D replacement: None — intentionally raw SQL for a global bootstrap table.
const sqlLoadRolePermissions = `
	SELECT role_name, permission_identifier
	FROM iam_role_permissions
	ORDER BY role_name, permission_identifier
`

// ── Session recovery ────────────────────────────────────────────────────────────

// sqlLoadSessionByHash retrieves session metadata from iam_sessions for a
// given token_hash. Used to recover a valid session after Redis eviction or
// restart. Only returns sessions that are not revoked and have not expired.
//
// The query executes inside a transaction with set_tenant_context so that RLS
// on iam_sessions scopes the lookup to the correct tenant.
//
// Fields returned: user_id, service_account_id, issued_at, expires_at,
// device_id, ip_address, tenant_id.
const sqlLoadSessionByHash = `
	SELECT
		user_id,
		service_account_id,
		issued_at,
		expires_at,
		device_id,
		ip_address,
		tenant_id
	FROM iam_sessions
	WHERE token_hash = $1
	  AND revoked_at IS NULL
	  AND expires_at > NOW()
`
