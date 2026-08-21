package repository

import (
	"context"
	"sync"

	"github.com/santiagossaa/invoice-generator/internal/domain/port"
)

// InMemoryUnitOfWork is a test double for UnitOfWork.
// It provides a simple mutex-based transaction simulation for in-memory repos.
// The mutex ensures that concurrent operations are serialized in tests.
type InMemoryUnitOfWork struct {
	mu sync.Mutex
}

func NewInMemoryUnitOfWork() *InMemoryUnitOfWork {
	return &InMemoryUnitOfWork{}
}

func (u *InMemoryUnitOfWork) BeginTx(ctx context.Context, tenantID string) (context.Context, port.Tx, error) {
	u.mu.Lock()
	// Note: we don't put a pgx.Tx in the context because in-memory repos
	// don't use TxFromCtx — they have their own fallback paths.
	// The in-memory repos' FindByIDForUpdate/SaveTx/FindByInvoiceTx/SaveTx
	// all delegate to their non-tx variants.
	txCtx := ctx // pass through, in-memory repos don't need tx in context
	return txCtx, &inMemoryTx{uow: u}, nil
}

type inMemoryTx struct {
	uow    *InMemoryUnitOfWork
	closed bool
}

func (t *inMemoryTx) Commit(ctx context.Context) error {
	if t.closed {
		return nil
	}
	t.closed = true
	t.uow.mu.Unlock()
	return nil
}

func (t *inMemoryTx) Rollback(ctx context.Context) error {
	if t.closed {
		return nil
	}
	t.closed = true
	t.uow.mu.Unlock()
	return nil
}

var _ port.UnitOfWork = (*InMemoryUnitOfWork)(nil)
var _ port.Tx = (*inMemoryTx)(nil)
