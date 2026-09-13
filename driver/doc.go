// Package driver defines the persistence driver interface that the Store
// Layer must implement.
//
// The framework works exclusively through [EntityRepository] — it never
// imports pgx, SQLC, or any ORM type. This makes the driver layer swappable
// and independently testable.
//
// # Driver contract
//
// A driver implementation must:
//   - Enforce the tenant context set by set_tenant_context() at the
//     PostgreSQL level (via PgBouncer transaction mode + RLS).
//   - Map every Filter tree to parameterised SQL without raw string
//     interpolation.
//   - Implement transactional hooks: after_save hooks receive a context
//     carrying the open transaction; the repository must use the same
//     connection.
//   - Use UUIDv7 for generated primary keys.
//   - Return typed domain errors (not raw pgx errors) from all methods.
//
// # Provided implementations
//
//   - awo.so/awo/driver/pgx — PostgreSQL driver via jackc/pgx v5
//
// # Mock implementation
//
// For unit tests, implement [EntityRepository] with a struct that satisfies
// the interface. The driver package provides a [Mock] helper that records
// calls and allows assertions without a real database.
package driver
