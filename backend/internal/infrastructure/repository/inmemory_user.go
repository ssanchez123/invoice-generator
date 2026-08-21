package repository

import (
	"context"
	"sync"

	"github.com/santiagossaa/invoice-generator/internal/domain/entity"
)

// InMemoryUserRepository is a test double for UserRepository.
type InMemoryUserRepository struct {
	mu    sync.RWMutex
	users map[string]*entity.User // keyed by ID
}

func NewInMemoryUserRepository() *InMemoryUserRepository {
	return &InMemoryUserRepository{users: make(map[string]*entity.User)}
}

func (r *InMemoryUserRepository) Save(ctx context.Context, user *entity.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	clone := *user
	r.users[user.ID] = &clone
	return nil
}

func (r *InMemoryUserRepository) FindByEmail(ctx context.Context, tenantID, email string) (*entity.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, u := range r.users {
		if u.TenantID == tenantID && u.Email == email {
			clone := *u
			return &clone, nil
		}
	}
	return nil, ErrNotFound
}

func (r *InMemoryUserRepository) FindByID(ctx context.Context, id string) (*entity.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.users[id]
	if !ok {
		return nil, ErrNotFound
	}
	clone := *u
	return &clone, nil
}

func (r *InMemoryUserRepository) EmailExists(ctx context.Context, tenantID, email string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, u := range r.users {
		if u.TenantID == tenantID && u.Email == email {
			return true, nil
		}
	}
	return false, nil
}
