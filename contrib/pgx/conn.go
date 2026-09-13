package pgx

import (
	"context"

	pgxlib "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"awo.so/awo/tx"
)

// pgConn wraps either a pool connection or a transaction and implements tx.Conn.
// This type is internal to the pgx driver; nothing outside contrib/pgx sees it.
type pgConn struct {
	pool *pgxpool.Pool
	txn  pgxlib.Tx // non-nil when inside a transaction
}

func (c *pgConn) InTx() bool { return c.txn != nil }

// Querier returns the underlying pgx querier — either the active transaction
// or the pool (for auto-commit queries).
func (c *pgConn) Querier() pgxlib.Row {
	panic("use QueryRow/Exec directly — see pgConn.pool or pgConn.txn")
}

// connFromContext extracts the *pgConn from ctx, or returns a pool-backed
// connection if none is present.
func connFromContext(ctx context.Context, pool *pgxpool.Pool) *pgConn {
	if c, ok := tx.ConnFromContext(ctx).(*pgConn); ok {
		return c
	}
	return &pgConn{pool: pool}
}

// execer provides a pgx-compatible Exec / QueryRow / Query interface that
// works regardless of whether we are inside a transaction.
//
// CopyFrom is included so that bulk-insert paths can use the PostgreSQL COPY
// protocol on either a pool connection or an active transaction without a type
// assertion.  Both *pgxpool.Pool and pgxlib.Tx satisfy this interface.
type execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgxlib.Row
	Query(ctx context.Context, sql string, args ...any) (pgxlib.Rows, error)
	SendBatch(ctx context.Context, b *pgxlib.Batch) pgxlib.BatchResults
	CopyFrom(ctx context.Context, tableName pgxlib.Identifier, columnNames []string, rowSrc pgxlib.CopyFromSource) (int64, error)
}

func (c *pgConn) db() execer {
	if c.txn != nil {
		return c.txn
	}
	return c.pool
}

// ExecSQL implements tx.Querier, allowing non-driver packages (e.g. awo/audit)
// to execute SQL within the active transaction without importing contrib/pgx.
func (c *pgConn) ExecSQL(ctx context.Context, sql string, args ...any) (int64, error) {
	tag, err := c.db().Exec(ctx, sql, args...)
	return tag.RowsAffected(), err
}

// NewPoolQuerier returns a tx.Querier backed directly by pool (no active
// transaction). Use this in framework bootstrap to inject a standalone executor
// into packages (e.g. awo/audit.TransactionalWriter) that must write to
// platform-level tables outside a request transaction.
//
// The returned Querier executes SQL via pool auto-commit semantics. When called
// inside a request context that already carries an active transaction, prefer
// tx.QuerierFromContext — it will return the transactional connection, which is
// what the caller usually wants.
func NewPoolQuerier(pool *pgxpool.Pool) tx.Querier {
	return &pgConn{pool: pool}
}

// setTenantContext calls the stored procedure that sets the RLS session variable.
// Must be called inside the transaction BEFORE any DML on tenant-scoped tables.
func setTenantContext(ctx context.Context, e execer, tenantID string) error {
	_, err := e.Exec(ctx, "SELECT set_tenant_context($1)", tenantID)
	return err
}

// SetOrgContext sets app.current_org_id for RLS enforcement on organization-scoped
// entities (ScopeOrganization and ScopeOrganizationTree).
// Must be called after setTenantContext and before any DML on org-scoped tables.
func SetOrgContext(ctx context.Context, e execer, orgID string) error {
	_, err := e.Exec(ctx, "SELECT set_org_context($1)", orgID)
	return err
}
