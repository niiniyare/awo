package db

import (
	"database/sql"
	// "github.com/jackc/pgx/v5/pgxpool"
)

// Store defines all functions to execute db queries and transactions
type Store interface {
	Querier
	// TransferTx(ctx context.Context, arg TransferTxParams) (TransferTxResult, error)
	// CreateUserTx(ctx context.Context, arg CreateUserTxParams) (CreateUserTxResult, error)
	// VerifyEmailTx(ctx context.Context, arg VerifyEmailTxParams) (VerifyEmailTxResult, error)
}

// SQLStore provides all functions to execute SQL queries and transactions
type SQLStore struct {
	connPool *sql.DB
	*Queries
}

// NewStore creates a new store
func NewStore(connPool *sql.DB) Store {
	return &SQLStore{
		connPool: connPool,
		Queries:  New(connPool),
	}
}
