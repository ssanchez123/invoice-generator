package port

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// txKey is the context key for the current transaction.
type txKey struct{}

// TxFromCtx extracts a pgx.Tx from the context, if one is present.
// Returns nil if no transaction is in the context.
// Note: for in-memory/test repositories, this will be nil and the repos
// will use their fallback behavior (non-transactional).
func TxFromCtx(ctx context.Context) pgx.Tx {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	return nil
}

// CtxWithTx returns a context with the given transaction set.
// The tx is stored as pgx.Tx; for in-memory tests, pass nil and use
// the non-tx fallback paths in the repositories.
func CtxWithTx(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

// UnitOfWork is the port for managing database transactions.
// The application layer uses this to execute multiple repository operations
// atomically within a single transaction.
type UnitOfWork interface {
	// BeginTx starts a tenant-scoped transaction and returns a context
	// that carries the transaction (via TxFromCtx).
	// The caller must call Commit or Rollback when done.
	BeginTx(ctx context.Context, tenantID string) (context.Context, Tx, error)
}

// Tx represents a database transaction abstraction.
type Tx interface {
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}
