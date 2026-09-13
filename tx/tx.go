// Package tx defines the transaction abstraction used by the Awo framework.
//
// Driver implementations carry a transaction via context.Context. Framework
// code — hooks, policies, services — never opens or commits transactions
// directly; it receives an already-transacted context from the driver.
//
// The TxKey type is unexported to prevent accidental use outside the driver
// layer. Drivers embed a Tx into context via WithTx and retrieve it via
// TxFromContext. Application code calls WithTx on an EntityRepository, which
// delegates transaction management to the driver.
package tx

import "context"

// Conn is the minimal interface that a driver transaction or connection must
// satisfy to be carried through context. The framework never calls methods on
// Conn directly — it is opaque to all layers above the driver.
type Conn interface {
	// InTx reports whether this connection is inside an open transaction.
	InTx() bool
}

// Querier is an optional extension of Conn for packages that need to execute
// SQL using the current transaction connection without importing the driver
// implementation (e.g. awo/audit). Driver implementations should implement
// Querier on their Conn type so that non-driver packages can participate in
// the active transaction.
//
// The signature mirrors pgx.Tx.Exec to avoid an adapter layer, but the
// interface itself has no pgx import — only context and standard types.
type Querier interface {
	// ExecSQL executes a SQL statement using the current connection (pool or
	// active transaction). Returns the number of rows affected.
	ExecSQL(ctx context.Context, sql string, args ...any) (int64, error)
}

// QuerierFromContext returns the Querier for the connection embedded in ctx,
// or (nil, false) if no connection is present or the connection does not
// implement Querier.
func QuerierFromContext(ctx context.Context) (Querier, bool) {
	c := ConnFromContext(ctx)
	if c == nil {
		return nil, false
	}
	q, ok := c.(Querier)
	return q, ok
}

type contextKey struct{}

// WithConn embeds a driver connection (possibly transacted) into ctx.
// Called exclusively by driver implementations.
func WithConn(ctx context.Context, conn Conn) context.Context {
	return context.WithValue(ctx, contextKey{}, conn)
}

// ConnFromContext extracts the driver connection from ctx.
// Returns nil if no connection is present — callers must handle nil.
func ConnFromContext(ctx context.Context) Conn {
	c, _ := ctx.Value(contextKey{}).(Conn)
	return c
}

// InTransaction reports whether ctx carries an active transaction.
func InTransaction(ctx context.Context) bool {
	c := ConnFromContext(ctx)
	return c != nil && c.InTx()
}
