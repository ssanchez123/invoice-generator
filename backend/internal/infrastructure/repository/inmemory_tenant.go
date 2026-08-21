package repository

import (
	"context"
	"sync"

	"github.com/santiagossaa/invoice-generator/internal/domain/entity"
)

// InMemoryTenantRepository is a test double for TenantRepository.
type InMemoryTenantRepository struct {
	mu      sync.RWMutex
	tenants map[string]*entity.Tenant
}

func NewInMemoryTenantRepository() *InMemoryTenantRepository {
	return &InMemoryTenantRepository{tenants: make(map[string]*entity.Tenant)}
}

func (r *InMemoryTenantRepository) FindByID(ctx context.Context, id string) (*entity.Tenant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tenants[id]
	if !ok {
		return nil, ErrNotFound
	}
	clone := *t
	return &clone, nil
}

func (r *InMemoryTenantRepository) Save(ctx context.Context, tenant *entity.Tenant) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	clone := *tenant
	r.tenants[tenant.ID] = &clone
	return nil
}
