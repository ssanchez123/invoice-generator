package repository

import (
	"context"
	"sort"
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

func (r *InMemoryTenantRepository) ListIDs(ctx context.Context) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tenantIDs := make([]string, 0, len(r.tenants))
	for tenantID := range r.tenants {
		tenantIDs = append(tenantIDs, tenantID)
	}
	sort.Strings(tenantIDs)
	return tenantIDs, nil
}
