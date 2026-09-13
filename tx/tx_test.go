package tx_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"awo.so/awo/tx"
)

// stubConn is a minimal tx.Conn test double.
type stubConn struct {
	inTx bool
}

func (s *stubConn) InTx() bool { return s.inTx }

// stubQuerierConn implements both tx.Conn and tx.Querier.
type stubQuerierConn struct {
	inTx     bool
	executed string
}

func (s *stubQuerierConn) InTx() bool { return s.inTx }

func (s *stubQuerierConn) ExecSQL(_ context.Context, sql string, _ ...any) (int64, error) {
	s.executed = sql
	return 1, nil
}

// --- tests ---

// TestWithConn_ConnFromContext verifies that a connection injected via
// WithConn is retrieved by ConnFromContext with the same value.
func TestWithConn_ConnFromContext(t *testing.T) {
	conn := &stubConn{inTx: false}
	ctx := tx.WithConn(context.Background(), conn)

	got := tx.ConnFromContext(ctx)
	require.NotNil(t, got)
	assert.Equal(t, conn, got)
}

// TestConnFromContext_NoConn verifies that ConnFromContext returns nil when no
// connection has been injected.
func TestConnFromContext_NoConn(t *testing.T) {
	got := tx.ConnFromContext(context.Background())
	assert.Nil(t, got)
}

// TestInTransaction_WithTx verifies that InTransaction returns true when the
// context carries a transacted connection.
func TestInTransaction_WithTx(t *testing.T) {
	conn := &stubConn{inTx: true}
	ctx := tx.WithConn(context.Background(), conn)
	assert.True(t, tx.InTransaction(ctx))
}

// TestInTransaction_NoTx verifies that InTransaction returns false when the
// context carries a non-transacted connection.
func TestInTransaction_NoTx(t *testing.T) {
	conn := &stubConn{inTx: false}
	ctx := tx.WithConn(context.Background(), conn)
	assert.False(t, tx.InTransaction(ctx))
}

// TestInTransaction_NoConn verifies that InTransaction returns false when no
// connection is in the context.
func TestInTransaction_NoConn(t *testing.T) {
	assert.False(t, tx.InTransaction(context.Background()))
}

// TestQuerierFromContext_Found verifies that a connection implementing Querier
// is returned by QuerierFromContext.
func TestQuerierFromContext_Found(t *testing.T) {
	conn := &stubQuerierConn{inTx: true}
	ctx := tx.WithConn(context.Background(), conn)

	q, ok := tx.QuerierFromContext(ctx)
	require.True(t, ok)
	require.NotNil(t, q)

	n, err := q.ExecSQL(ctx, "SELECT 1")
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)
	assert.Equal(t, "SELECT 1", conn.executed)
}

// TestQuerierFromContext_NotFound verifies that a connection not implementing
// Querier returns (nil, false).
func TestQuerierFromContext_NotFound(t *testing.T) {
	conn := &stubConn{inTx: false} // does NOT implement Querier
	ctx := tx.WithConn(context.Background(), conn)

	q, ok := tx.QuerierFromContext(ctx)
	assert.False(t, ok)
	assert.Nil(t, q)
}

// TestQuerierFromContext_NoConn verifies that an empty context returns
// (nil, false).
func TestQuerierFromContext_NoConn(t *testing.T) {
	q, ok := tx.QuerierFromContext(context.Background())
	assert.False(t, ok)
	assert.Nil(t, q)
}
