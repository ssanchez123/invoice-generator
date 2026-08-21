package command

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/santiagossaa/invoice-generator/internal/domain/entity"
	"github.com/santiagossaa/invoice-generator/internal/domain/port"
)

// RegisterCommand registers a new user within a tenant.
type RegisterCommand struct {
	TenantID  string
	Email     string
	Password  string
	Role      string // "admin", "member", "viewer"; defaults to "member"
	FirstName string
	LastName  string
}

type RegisterHandler struct {
	userRepo   port.UserRepository
	hasher     port.PasswordHasher
	tenantRepo port.TenantRepository
}

func NewRegisterHandler(ur port.UserRepository, h port.PasswordHasher, tr port.TenantRepository) *RegisterHandler {
	return &RegisterHandler{userRepo: ur, hasher: h, tenantRepo: tr}
}

func (h *RegisterHandler) Handle(ctx context.Context, cmd RegisterCommand) (*entity.User, error) {
	// Normalize email
	email := strings.TrimSpace(strings.ToLower(cmd.Email))
	if email == "" {
		return nil, ErrInvalidEmail
	}

	// Validate password
	if len(cmd.Password) < 8 {
		return nil, ErrPasswordTooShort
	}

	// Validate role
	role := entity.UserRole(cmd.Role)
	if role == "" {
		role = entity.UserRoleMember
	}
	if role != entity.UserRoleAdmin && role != entity.UserRoleMember && role != entity.UserRoleViewer {
		return nil, ErrInvalidRole
	}

	// Check that tenant exists
	tenant, err := h.tenantRepo.FindByID(ctx, cmd.TenantID)
	if err != nil {
		return nil, ErrTenantNotFound
	}
	_ = tenant // tenant exists, proceed

	// Check if email already exists for this tenant
	exists, err := h.userRepo.EmailExists(ctx, cmd.TenantID, email)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrEmailAlreadyExists
	}

	// Hash password
	hash, err := h.hasher.Hash(cmd.Password)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	user := &entity.User{
		ID:           uuid.NewString(),
		TenantID:     cmd.TenantID,
		Email:        email,
		PasswordHash: hash,
		Role:         role,
		FirstName:    cmd.FirstName,
		LastName:     cmd.LastName,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := h.userRepo.Save(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}
