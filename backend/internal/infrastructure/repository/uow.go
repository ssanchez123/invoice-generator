package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/santiagossaa/invoice-generator/internal/ctxkey"
	"github.com/santiagossaa/invoice-generator/internal/domain/port"
)

// UnitOfWork implements port.UnitOfWork using PostgreSQL.
type UnitOfWork struct {
	db *DB
}

func NewUnitOfWork(db *DB) *UnitOfWork {
	return &UnitOfWork{db: db}
}

// BeginTx starts a tenant-scoped transaction and returns a context carrying the tx.
// The tenant_id is set as a session variable for RLS policies.
func (u *UnitOfWork) BeginTx(ctx context.Context, tenantID string) (context.Context, port.Tx, error) {
	tx, err := u.db.TenantTx(ctx, tenantID)
	if err != nil {
		return nil, nil, err
	}

	// Wrap the pgx.Tx in our port.Tx interface
	wrapped := &pgxTxWrapper{tx: tx}

	// Return a context with the transaction set
	txCtx := context.WithValue(ctx, ctxkey.TenantIDKey, tenantID)
	txCtx = port.CtxWithTx(txCtx, tx)

	return txCtx, wrapped, nil
}

// pgxTxWrapper adapts pgx.Tx to the port.Tx interface.
type pgxTxWrapper struct {
	tx pgx.Tx
}

func (w *pgxTxWrapper) Commit(ctx context.Context) error {
	return w.tx.Commit(ctx)
}

func (w *pgxTxWrapper) Rollback(ctx context.Context) error {
	return w.tx.Rollback(ctx)
}

var _ port.UnitOfWork = (*UnitOfWork)(nil)
var _ port.Tx = (*pgxTxWrapper)(nil)

// Ensure pgx is used
var _ = fmt.Sprintf
